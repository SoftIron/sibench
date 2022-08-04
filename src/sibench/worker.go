// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main


import "fmt"
import "logger"
import "math/rand"
import "time"



/* The set of states in which the worker can reside. */
type workerState int
const (
    WS_BadTransition workerState = iota // This is not an actual state, but exists to be a zero value in a map
    WS_Init
    WS_Connect
    WS_ConnectDone
    WS_Write
    WS_WriteDone
    WS_Prepare
    WS_PrepareDone
    WS_Read
    WS_ReadDone
    WS_ReadWrite
    WS_ReadWriteDone
    WS_Terminated
)


func workerStateToStr(state workerState) string {
    switch state {
        case WS_BadTransition:  return "BadTranstition"
        case WS_Init:           return "Init"
        case WS_Connect:        return "Connecting"
        case WS_ConnectDone:    return "ConnectingDone"
        case WS_Write:          return "Writing"
        case WS_WriteDone:      return "WritingDone"
        case WS_Prepare:        return "Prepare"
        case WS_PrepareDone:    return "PrepareDone"
        case WS_Read:           return "Reading"
        case WS_ReadDone:       return "ReadingDone"
        case WS_ReadWrite:      return "ReadWrite"
        case WS_ReadWriteDone:  return "ReadWriteDone"
        case WS_Terminated:     return "Terminated"
        default:                return "UnknownState"
    }
}


/*
 * A map of the state transitions that may be triggered by incoming opcodes.
 * Each opcode maps to a map from current state to next state. 
 * If an opcode has no entry for the current state, then the output will be the zero value,
 * which in this case is BadTransition. 
 */
var validWSTransitions = map[Opcode]map[workerState]workerState {
    OP_Connect:         { WS_Init:           WS_Connect },
    OP_WriteStart:      { WS_ConnectDone:    WS_Write },
    OP_WriteStop:       { WS_Write:          WS_WriteDone },
    OP_Prepare:         { WS_ConnectDone:    WS_Prepare,
                          WS_WriteDone:      WS_Prepare },
    OP_ReadStart:       { WS_PrepareDone:    WS_Read },
    OP_ReadStop:        { WS_Read:           WS_ReadDone },
    OP_ReadWriteStart:  { WS_PrepareDone:    WS_ReadWrite },
    OP_ReadWriteStop:   { WS_ReadWrite:      WS_ReadWriteDone },
    OP_Terminate:       { WS_Init:           WS_Terminated,
                          WS_Connect:        WS_Terminated,
                          WS_ConnectDone:    WS_Terminated,
                          WS_Write:          WS_Terminated,
                          WS_WriteDone:      WS_Terminated,
                          WS_Prepare:        WS_Terminated,
                          WS_PrepareDone:    WS_Terminated,
                          WS_Read:           WS_Terminated,
                          WS_ReadDone:       WS_Terminated,
                          WS_ReadWrite:      WS_Terminated,
                          WS_ReadWriteDone:  WS_Terminated,
                          WS_Terminated:     WS_Terminated },
}


/*
 * When reporting how long each read, write or prepare operation took, we can encode
 * any errors we encounter as negative durations.
 */
const (
    Stat_VerifyFailure time.Duration = -1
    Stat_OperationFailure            = -2
)



const HangTimeoutSecs = 90



/*
 * WorkerResponse reports the error from an opcode, which is nil if the opcode succeeded. 
 */
type WorkerResponse struct {
    WorkerId uint64
    Op Opcode
    Error error
}


/* The arguments used to construct a worker.  They have been bundled into a struct purely for readability. */
type WorkerSpec struct {
    Id uint64
    ConnConfig WorkerConnectionConfig
    OpChannel <-chan Opcode
    ResponseChannel chan<- *WorkerResponse
    StatChannel chan<- *Stat
}


/* A Worker is does the actual benchmarking work: it performs the Puts and Gets, and times them. */
type Worker struct {
    spec WorkerSpec
    order WorkOrder
    state workerState
    cycle uint64
    objectIndex uint64
    generator Generator
    connections []Connection
    connIndex uint64
    phaseStart time.Time
    objectBuffer []byte
    verifyBuffer []byte

    /* These fields are used for the bandwidth-limiting delays code */

    phaseFirstOp bool           // Whether this is the first op since we started a phase.
    lastOpStart time.Time       // The start time of our last read or write
    avgElapsed time.Duration    // Our running average operation time.
    postDelay time.Duration     // A delay we need to insert after the next op completes.
}


func NewWorker(spec *WorkerSpec, order *WorkOrder) (*Worker, error) {
    logger.Debugf("[worker %v] creating worker with range %v to %v\n", spec.Id, order.RangeStart, order.RangeEnd)

    var w Worker
    w.spec = *spec
    w.order = *order
    w.objectIndex = order.RangeStart
    w.setState(WS_Init)

    w.objectBuffer = make([]byte, w.order.ObjectSize)
    w.verifyBuffer = make([]byte, w.order.ObjectSize)

    var err error
    w.generator, err = CreateGenerator(order.GeneratorType, order.Seed, order.GeneratorConfig)
    if err != nil {
        logger.Errorf("[worker %v] failure during creation: %v\n", spec.Id, err)
        return nil, err
    }

    // Start the worker's event loop
    go w.eventLoop()

    return &w, nil
}


func (w *Worker) Id() uint64 {
    return w.spec.Id
}


func (w *Worker) eventLoop() {
    for w.state != WS_Terminated {
        select {
            case op := <-w.spec.OpChannel: w.handleOpcode(op)

            default: switch w.state {
                case WS_Connect:    w.connect()
                case WS_Write:      w.write()
                case WS_Prepare:    w.prepare()
                case WS_Read:       w.read()
                case WS_ReadWrite:  w.readWrite()
                default:
            }
        }
    }

    logger.Debugf("[worker %v] shutting down\n", w.spec.Id)

    for _, conn := range w.connections {
        conn.WorkerClose()
    }
}


func (w *Worker) handleOpcode(op Opcode) {
    logger.Debugf("[worker %v] handleOpcode: %v\n", w.spec.Id, op.ToString())

    // See if the Opcode is valid in our current state.
    nextState := validWSTransitions[op][w.state]
    if nextState == WS_BadTransition {
        w.fail(fmt.Errorf("[worker %v] handleOpcode: bad transition from state %v on opcode %v", w.spec.Id, workerStateToStr(w.state), op.ToString()))
        return
    }

    // Connect and Prepare are the two opcodes that take time to complete, and so we send responses for 
    // them when they have actually completed their work.
    // The rest of the opcodes take place immediately, so we can respond with acknowledgement here.

    if (op != OP_Connect) && (op != OP_Prepare) {
        w.sendResponse(op, nil)
    }

    w.setState(nextState)
    w.phaseStart = time.Now()
    w.phaseFirstOp = true
}


func (w *Worker) sendResponse(op Opcode, err error) {
    logger.Debugf("[worker %v] sending Response: %v, %v\n", w.spec.Id, op.ToString(), err)
    w.spec.ResponseChannel <- &WorkerResponse{ WorkerId: w.spec.Id, Op: op, Error: err }
}


func (w *Worker) fail(err error) {
    w.state = WS_Terminated
    logger.Errorf("%v\n", err.Error())
    w.sendResponse(OP_Fail, err)
}


func (w *Worker) hung(err error) {
    w.state = WS_Terminated
    logger.Errorf("%v\n", err.Error())
    w.sendResponse(OP_Hung, err)
}


func (w *Worker) connect() {
    w.setState(WS_ConnectDone)

    for _, t := range w.order.Targets {
        conn, err := NewConnection(w.order.ConnectionType, t, w.order.ProtocolConfig, w.spec.ConnConfig)
        if err == nil {
            done := make(chan bool, 1)

            go func() {
                err = conn.WorkerConnect()
                done <- true
            }()

            // See which happens first: a result, or a timeout.
            select {
                case <-time.After(HangTimeoutSecs * time.Second):
                    // We can't tell if this is a slow operation or a hang in a ceph library, so assume the worst.
                    w.hung(fmt.Errorf("[worker %v] Timeout on write to %v", w.spec.Id, conn.Target()))
                    return

                case <-done:
                    logger.Tracef("[worker %v] completed connect to %v\n", w.spec.Id, t)
            }
        }

        if err != nil {
            w.fail(fmt.Errorf("[worker %v] failure during connect to %v: %v", w.spec.Id, t, err))
            return
        }

        w.connections = append(w.connections, conn)
    }

    logger.Debugf("[worker %v] successfully connected\n", w.spec.Id)
    w.sendResponse(OP_Connect, nil)
}


func (w *Worker) writeOrPrepare(phase StatPhase) {
    key := fmt.Sprintf("%v-%v", w.order.ObjectKeyPrefix, w.objectIndex)
    w.generator.Generate(w.order.ObjectSize, key, w.cycle, &w.objectBuffer)
    conn := w.connections[w.connIndex]

    var err error
    var start time.Time
    var end time.Time
    done := make(chan bool, 1)

    go func() {
        start = time.Now()
        err = conn.PutObject(key, w.objectIndex, w.objectBuffer)
        end = time.Now()
        done <- true
    }()

    // See which happens first: a result, or a timeout.
    select {
        case <-time.After(HangTimeoutSecs * time.Second):
            // We can't tell if this is a slow operation or a hang in a ceph library, so assume the worst.
            w.hung(fmt.Errorf("[worker %v] Timeout on write to %v", w.spec.Id, conn.Target()))
            return

        case <-done:
            logger.Tracef("[worker %v] completed write\n", w.spec.Id)

    }

    var s Stat
    s.Error = SE_None
    s.Phase = phase
    s.TimeSincePhaseStart = end.Sub(w.phaseStart)
    s.Duration = end.Sub(start)
    s.TargetIndex = uint16(w.connIndex)

    if err != nil {
        logger.Warnf("[worker %v] failure putting object<%v> to %v: %v\n", w.spec.Id, key, conn.Target(), err)
        s.Error = SE_OperationFailure
    }

    // Submit a stat
    w.spec.StatChannel <- &s

    // Advance our object ID ready for next time.
    w.objectIndex++
    if w.objectIndex >= w.order.RangeEnd {
        w.objectIndex = w.order.RangeStart
        w.cycle++
        logger.Tracef("[worker %v] advancing cycle to %v\n", w.spec.Id, w.cycle)
    }

    // Advance our connection index ready for next time
    w.connIndex = (w.connIndex + 1) % uint64(len(w.connections))
}


func (w *Worker) write() {
    w.limitBandwidth()
    w.writeOrPrepare(SP_Write)
}


func (w *Worker) prepare() {
    // See if we've prepared a whole cycle of objects.
    if w.cycle > 0 {
        w.invalidateConnectionCaches()
        w.setState(WS_PrepareDone)
        w.sendResponse(OP_Prepare, nil)
        return
    }

    w.writeOrPrepare(SP_Prepare)
}


func (w *Worker) read() {
    w.limitBandwidth()

    key := fmt.Sprintf("%v-%v", w.order.ObjectKeyPrefix, w.objectIndex)
    conn := w.connections[w.connIndex]

    var err error
    var start time.Time
    var end time.Time
    done := make(chan bool, 1)

    go func() {
        start = time.Now()
        err = conn.GetObject(key, w.objectIndex, w.objectBuffer)
        end = time.Now()
        done <- true
    }()

    // See which happens first: a result, or a timeout.
    select {
        case <-time.After(HangTimeoutSecs * time.Second):
            // We can't tell if this is a slow operation or a hang in a ceph library, so assume the worst.
            w.hung(fmt.Errorf("[worker %v] Timeout on read from %v", w.spec.Id, conn.Target()))
            return

        case <-done:
            logger.Tracef("[worker %v] completed read\n", w.spec.Id)

    }

    var s Stat
    s.Error = SE_None
    s.Phase = SP_Read
    s.TimeSincePhaseStart = end.Sub(w.phaseStart)
    s.Duration = end.Sub(start)
    s.TargetIndex = uint16(w.connIndex)

    if err != nil {
        logger.Warnf("[worker %v] failure getting object<%v> to %v: %v\n", w.spec.Id, key, conn.Target(), err)
        s.Error = SE_OperationFailure
    } else {
        if !w.order.SkipReadValidation {
            err = w.generator.Verify(w.order.ObjectSize, key, &w.objectBuffer, &w.verifyBuffer)
            if err != nil {
                logger.Warnf("[worker %v] failure verfiying object<%v> to %v: %v\n", w.spec.Id, key, conn.Target(), err)
                s.Error = SE_VerifyFailure
            }
        }
    }

    // Submit the stat
    w.spec.StatChannel <- &s

    // Advance our object ID ready for next time.
    w.objectIndex++
    if w.objectIndex >= w.order.RangeEnd {
        w.objectIndex = w.order.RangeStart
        w.invalidateConnectionCaches()
    }

    // Advance our connection index ready for next time
    w.connIndex = (w.connIndex + 1) % uint64(len(w.connections))
}


func (w *Worker) readWrite() {
    if int(w.order.ReadWriteMix) < rand.Intn(100) {
        w.write()
    } else {
        w.read()
    }
}


/* 
 * Sleep in order to limit bandwidth 
 */
func (w *Worker) limitBandwidth() {
    // See if we need to do anything in the first place.
    if w.order.Bandwidth == 0 {
        return
    }

    if w.phaseFirstOp {
        // Random delay to smooth out between workers.  
        // This is only really important for large object sizes (where traffic is 'lumpy').
        time.Sleep(time.Duration(rand.Intn(1000 * 1000 * 10)))

        w.phaseFirstOp = false
        w.lastOpStart = time.Now()
        w.avgElapsed = 0
        w.postDelay = 0
        return
    }

    elapsed := time.Now().Sub(w.lastOpStart)
    time.Sleep(w.postDelay)

    // Compute our rolling average.
    if w.avgElapsed == 0 {
        // This is our first completed op, so start the rolling average with this value.
        w.avgElapsed = elapsed
    } else {
        // This is not our first complete op: merge in the new value to our rolling average
        w.avgElapsed = ((w.avgElapsed * 7) + elapsed) / 8
    }

    // Compute how log we would like an op to take to maintain our limited bandwidth.
    desired := time.Duration(1000 * 1000 * 1000 * w.order.ObjectSize / w.order.Bandwidth)

    // If the desired value is slower than the average value, sleep for a bit.
    if desired > w.avgElapsed {
        // Compute the total delay we want
        totalDelay := desired - w.avgElapsed

        // Then split it into two parts: a pre-delay that we'll do before the operation, 
        // and a post-delay that we'll do afterwards (in the interests of traffic smoothing).
        preDelay := time.Duration(rand.Intn(int(totalDelay)))
        w.postDelay = totalDelay - preDelay
        time.Sleep(preDelay)
    }

    w.lastOpStart = time.Now()
}


func (w *Worker) setState(state workerState) {
    logger.Debugf("[worker %v] Worker changing state: %v -> %v\n", w.spec.Id, workerStateToStr(w.state), workerStateToStr(state))
    w.state = state
}


/*
 * Tell all of our connections to invalidate their caches 
 */
func (w* Worker) invalidateConnectionCaches() {
    for _, conn := range w.connections {
        conn.InvalidateCache()
    }
}
