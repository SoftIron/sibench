// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "comms"
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


/* 
 * Extra information associated with each state.
 */
type workerStateDetails struct {
    /* Human readable name for debug statements. */
    name string

    /* Whether or not this is a state for which the Foreman should track our summary/heartbeat messages
       in order to handle hung operations */
    canTimeout bool
}


var wsDetails = map[workerState]workerStateDetails {
    WS_BadTransition:  { "BadTranstition",  false },
    WS_Init:           { "Init",            false },
    WS_Connect:        { "Connecting",      true },
    WS_ConnectDone:    { "ConnectingDone",  false },
    WS_Write:          { "Writing",         true },
    WS_WriteDone:      { "WritingDone",     false },
    WS_Prepare:        { "Prepare",         true },
    WS_PrepareDone:    { "PrepareDone",     false },
    WS_Read:           { "Reading",         true },
    WS_ReadDone:       { "ReadingDone",     false },
    WS_ReadWrite:      { "ReadWrite",       true },
    WS_ReadWriteDone:  { "ReadWriteDone",   false },
    WS_Terminated:     { "Terminated",      false },
}


func workerStateToStr(state workerState) string {
    return wsDetails[state].name
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



/*
 * WorkerResponse reports the error from an opcode, which is nil if the opcode succeeded. 
 */
type WorkerResponse struct {
    WorkerId uint64
    Op Opcode
    Error error
}


/**
 * Bundles a StatSummary together with a worker ID so that we can stick them in a channel.
 */
type WorkerSummary struct {
    data StatSummary
    workerId uint64
    canTimeout bool // Indicates whether or not the worker is running in a phase.
}


/* The arguments used to construct a worker.  They have been bundled into a struct purely for readability. */
type WorkerSpec struct {
    Id uint64
    ConnConfig WorkerConnectionConfig
    OpChannel <-chan Opcode
    ResponseChannel chan<- *WorkerResponse
    SummaryChannel chan<- WorkerSummary
    StatPreallocationCount uint64
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
    lastSummary time.Time
    summary WorkerSummary
    stats [][]Stat
    nextStatIndex int
    statSliceIndex int
    statLastSliceIndex int

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
    w.summary.workerId = spec.Id

    w.stats = make([][]Stat, 0, 100)
    w.stats = append(w.stats, make([]Stat, w.spec.StatPreallocationCount))
    w.clearStats()

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


/**
 * Clears our stats (but does not free them).
 */
func (w *Worker) clearStats() {
    w.nextStatIndex = 0
    w.statLastSliceIndex = 0
    w.statSliceIndex = 0
}


/**
 * Returns a pointer to the next Stat object to fill in when we complete an op.
 *
 * This will allocate a new slice of Stats whenever our current slice fills up. 
 * (We don't append to slices, so thee isn't any grow-then-copy or GC.
 */
func (w *Worker) nextStat() *Stat {
    result := &(w.stats[w.statSliceIndex][w.nextStatIndex])

    w.nextStatIndex++
    if w.nextStatIndex == len(w.stats[w.statSliceIndex]) {
        w.nextStatIndex = 0
        w.statSliceIndex++
        if w.statSliceIndex >= w.statLastSliceIndex {
            w.statLastSliceIndex++
            w.stats = append(w.stats, make([]Stat, w.spec.StatPreallocationCount))
        }
    }

    return result
}


/**
 * At the end of a phase, the Foreman asks each worker in turn to send their Stats back to the 
 * manager, using a TCP connection that the Foreman provides.
 *
 * When we're done, we clear our stats so we can reuse them.
 */
func (w *Worker) UploadStats(tcpConnection *comms.MessageConnection) {
    for i := 0; i <= w.statSliceIndex; i++ {
        if i != w.statSliceIndex {
            logger.Debugf("[worker %v] sending complete stats buffer: %v entries\n", w.spec.Id, len(w.stats[i]))
            tcpConnection.Send(OP_StatDetails, w.stats[i])
        } else {
            logger.Debugf("[worker %v] sending partial stats buffer: %v entries\n", w.spec.Id, w.nextStatIndex)
            tcpConnection.Send(OP_StatDetails, w.stats[i][:w.nextStatIndex])
        }
    }

    w.clearStats()
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
    w.phaseFirstOp = true
    w.phaseStart = time.Now()
    w.lastSummary = w.phaseStart
    w.summary.data.Zero()
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
    for _, t := range w.order.Targets {
        conn, err := NewConnection(w.order.ConnectionType, t, w.order.ProtocolConfig, w.spec.ConnConfig)
        if err == nil {
            err = conn.WorkerConnect()
        }

        if err != nil {
            w.fail(fmt.Errorf("[worker %v] failure during connect to %v: %v", w.spec.Id, t, err))
            return
        }

        logger.Tracef("[worker %v] completed connect to %v\n", w.spec.Id, t)
        w.connections = append(w.connections, conn)
    }

    logger.Debugf("[worker %v] successfully connected\n", w.spec.Id)

    w.setState(WS_ConnectDone)
    w.sendResponse(OP_Connect, nil)
}


/* 
 * Sends a summary of our stats to our foreman, and then clears our summary data.
 *
 * This only does anything if either it's been at least 250ms since our last time,
 * or if force is set true.
 */
func (w *Worker) sendSummary(t *time.Time, force bool) {
    if force || ((*t).Sub(w.lastSummary) > (250 * time.Millisecond)) {
        w.lastSummary = *t
        w.spec.SummaryChannel <- w.summary
        w.summary.data.Zero()
    }
}


func (w *Worker) writeOrPrepare(phase StatPhase) {
    w.generator.Generate(w.order.ObjectSize, w.objectIndex, w.cycle, &w.objectBuffer)
    conn := w.connections[w.connIndex]

    var key string
    if conn.RequiresKey() {
        key = fmt.Sprintf("%v-%v", w.order.ObjectKeyPrefix, w.objectIndex)
    }

    logger.Tracef("[worker %v] starting put for object<%v> on %v at %v\n", w.spec.Id, w.objectIndex, conn.Target(), time.Now())

    start := time.Now()
    err := conn.PutObject(key, w.objectIndex, w.objectBuffer)
    end := time.Now()

    logger.Tracef("[worker %v] completed put for object<%v> on %v\n", w.spec.Id, w.objectIndex, conn.Target())

    s := w.nextStat()
    s.Error = SE_None
    s.Phase = phase
    s.TimeSincePhaseStartMillis = uint32(start.Sub(w.phaseStart) / (1000 * 1000))
    s.DurationMicros = uint32(end.Sub(start) / 1000)
    s.TargetIndex = uint16(w.connIndex)

    if err != nil {
        logger.Warnf("[worker %v] failure putting object<%v> to %v: %v\n", w.spec.Id, w.objectIndex, conn.Target(), err)
        s.Error = SE_OperationFailure
    }

    w.summary.data[phase][s.Error]++
    w.sendSummary(&end, true)

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
        logger.Debugf("[worker %v] finished preparing\n", w.spec.Id)

        w.invalidateConnectionCaches()
        w.setState(WS_PrepareDone)
        w.sendResponse(OP_Prepare, nil)
        return
    }

    w.writeOrPrepare(SP_Prepare)
}


func (w *Worker) read() {
    w.limitBandwidth()

    conn := w.connections[w.connIndex]

    var key string
    if conn.RequiresKey() {
        key = fmt.Sprintf("%v-%v", w.order.ObjectKeyPrefix, w.objectIndex)
    }

    logger.Tracef("[worker %v] starting get for object<%v> on %v\n", w.spec.Id, w.objectIndex, conn.Target())

    start := time.Now()
    err := conn.GetObject(key, w.objectIndex, w.objectBuffer)
    end := time.Now()

    logger.Tracef("[worker %v] completed get for object<%v> on %v\n", w.spec.Id, w.objectIndex, conn.Target())

    s := w.nextStat()
    s.Error = SE_None
    s.Phase = SP_Read
    s.TimeSincePhaseStartMillis = uint32(start.Sub(w.phaseStart) / (1000 * 1000))
    s.DurationMicros = uint32(end.Sub(start) / 1000)
    s.TargetIndex = uint16(w.connIndex)

    if err != nil {
        logger.Warnf("[worker %v] failure getting object<%v> to %v: %v\n", w.spec.Id, w.objectIndex, conn.Target(), err)
        s.Error = SE_OperationFailure
    } else {
        if !w.order.SkipReadValidation {
            err = w.generator.Verify(w.order.ObjectSize, w.objectIndex, &w.objectBuffer, &w.verifyBuffer)
            if err != nil {
                logger.Warnf("[worker %v] failure verfiying object<%v> to %v: %v\n", w.spec.Id, w.objectIndex, conn.Target(), err)
                s.Error = SE_VerifyFailure
            }
        }
    }

    w.summary.data[SP_Read][s.Error]++
    w.sendSummary(&end, true)

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
    logger.Debugf("[worker %v] changing state: %v -> %v\n", w.spec.Id, workerStateToStr(w.state), workerStateToStr(state))
    w.state = state

    // If we're changing from a state which needs timeout monitoring from one which doesn't, or vice versa,
    // then let the foreman's stat processing routine know with a summary update.

    if w.summary.canTimeout != wsDetails[state].canTimeout {
        w.summary.canTimeout = wsDetails[state].canTimeout
        now := time.Now()
        w.sendSummary(&now, true)
    }
}

/*
 * Tell all of our connections to invalidate their caches 
 */
func (w* Worker) invalidateConnectionCaches() {
    for _, conn := range w.connections {
        conn.InvalidateCache()
    }
}
