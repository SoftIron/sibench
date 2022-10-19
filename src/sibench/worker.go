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
    WS_Clean
    WS_CleanDone
    WS_Terminated
)


func workerStateToStr(state workerState) string {
    switch state {
        case WS_BadTransition:  return "BadTransition"
        case WS_Init:           return "Init"
        case WS_Connect:        return "Connect"
        case WS_ConnectDone:    return "ConnectDone"
        case WS_Write:          return "Write"
        case WS_WriteDone:      return "WriteDone"
        case WS_Prepare:        return "Prepare"
        case WS_PrepareDone:    return "PrepareDone"
        case WS_Read:           return "Read"
        case WS_ReadDone:       return "ReadDone"
        case WS_ReadWrite:      return "ReadWrite"
        case WS_ReadWriteDone:  return "ReadWriteDone"
        case WS_Clean:          return "Clean"
        case WS_CleanDone:      return "CleanDone"
        case WS_Terminated:     return "Terminated"
        default:                return "Unknown WorkerState"
    }
}


/* Function type used when states change, or when we are in our event loop */
type workerStateFunction func(w *Worker)

/* Extra information associated with each state. */
type workerStateDetails struct {
    /* Whether or not this is the start of a phase. */
    isStartOfPhase bool

    /* Whether or not this is a state for which the Foreman should track our summary/heartbeat messages
       in order to handle operations which might hang. */
    canTimeout bool

    /* Opcode to send on entry to the state, or OP_None */
    opcodeOnEntry Opcode

    /* If not nil, a function tp be called when we enter this state */
    onEntry workerStateFunction;

    /* If not nil, a function to be called when we go round our event loop */
    onEventLoop workerStateFunction;
}


var wsDetails map[workerState]workerStateDetails


/** 
 * We need to initialise our table in the init() function to avoid circular references caused by the use of
 * function pointers.  (Go is strange like this: in most languages, the function pointers would be resolved
 * in a later phase by the compiler, and so this wouldn't be considered a circular reference).
 */
func init() {
    wsDetails = map[workerState]workerStateDetails {
    //  State                StartOfPhase  CanTimeout  OpcpdeOnEntry       onEntry     onEventLoop
        WS_BadTransition:  { false,        false,      OP_None,            nil,        nil              },
        WS_Init:           { false,        false,      OP_None,            nil,        nil              },
        WS_Connect:        { false,        true,       OP_None,            onConnect,  nil              },
        WS_ConnectDone:    { false,        false,      OP_Connect,         nil,        nil              },
        WS_Write:          { true,         true,       OP_WriteStart,      nil,        onWriteEvent     },
        WS_WriteDone:      { false,        false,      OP_WriteStop,       nil,        nil              },
        WS_Prepare:        { true,         true,       OP_None,            nil,        onPrepareEvent   },
        WS_PrepareDone:    { false,        false,      OP_Prepare,         nil,        nil              },
        WS_Read:           { true,         true,       OP_ReadStart,       nil,        onReadEvent      },
        WS_ReadDone:       { false,        false,      OP_ReadStop,        nil,        nil              },
        WS_ReadWrite:      { true,         true,       OP_ReadWriteStart,  nil,        onReadWriteEvent },
        WS_ReadWriteDone:  { false,        false,      OP_ReadWriteStop,   nil,        nil              },
        WS_Clean:          { true,         true,       OP_None,            onClean,    onCleanEvent     },
        WS_CleanDone:      { false,        false,      OP_Clean,           nil,        nil              },
        WS_Terminated:     { false,        false,      OP_Terminate,       nil,        nil              },
    }
}


/*
 * A map of the state transitions that may be triggered by incoming opcodes.
 * Each opcode maps to a map from current state to next state. 
 *
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
    OP_Clean:           { WS_ReadDone:       WS_Clean,
                          WS_ReadWriteDone:  WS_Clean },
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
                          WS_Clean:          WS_Terminated,
                          WS_CleanDone:      WS_Terminated,
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


func (w *Worker) eventLoop() {
    for w.state != WS_Terminated {
        select {
            case op := <-w.spec.OpChannel: w.handleOpcode(op)

            default:
                fn := wsDetails[w.state].onEventLoop
                if fn != nil {
                    fn(w)
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

    w.setState(nextState)
}


func (w *Worker) setState(state workerState) {
    logger.Debugf("[worker %v] changing state: %v -> %v\n", w.spec.Id, workerStateToStr(w.state), workerStateToStr(state))
    w.state = state

    // If we have an opcode to send when we enter this state, then send it.
    op := wsDetails[state].opcodeOnEntry
    if op != OP_None {
        w.sendResponse(op, nil)
    }

    // If we have a function to call when we enter this state, then invoke it.
    fn := wsDetails[state].onEntry
    if fn != nil {
        fn(w)
    }

    // If we're starting a new phase, then clear our stats and set suitable flags.
    if wsDetails[state].isStartOfPhase {
        w.phaseFirstOp = true
        w.phaseStart = time.Now()
        w.lastSummary = w.phaseStart
        w.summary.data.Zero()
    }

    // If we're changing from a state which needs timeout monitoring from one which doesn't, or vice versa,
    // then let the foreman's stat processing routine know with a summary update.
    if w.summary.canTimeout != wsDetails[state].canTimeout {
        w.summary.canTimeout = wsDetails[state].canTimeout
        now := time.Now()
        w.sendSummary(&now, true)
    }
}


func (w *Worker) fail(err error) {
    w.state = WS_Terminated
    logger.Errorf("%v\n", err.Error())
    w.sendResponse(OP_Fail, err)
}


func onConnect(w *Worker) {
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
}


func onWriteEvent(w *Worker) {
    w.limitBandwidth()
    w.writeOrPrepare(SP_Write)
}


func onPrepareEvent(w *Worker) {
    // See if we've prepared a whole cycle of objects.
    if w.cycle > 0 {
        logger.Debugf("[worker %v] finished preparing\n", w.spec.Id)
        w.invalidateConnectionCaches()
        w.setState(WS_PrepareDone)
        return
    }

    w.writeOrPrepare(SP_Prepare)
}


func onReadEvent(w *Worker) {
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


func onReadWriteEvent(w *Worker) {
    if int(w.order.ReadWriteMix) < rand.Intn(100) {
        onWriteEvent(w)
    } else {
        onReadEvent(w)
    }
}


func onClean(w *Worker) {
    w.objectIndex = w.order.RangeStart
}


func onCleanEvent(w *Worker) {
    conn := w.connections[w.connIndex]

    var key string
    if conn.RequiresKey() {
        key = fmt.Sprintf("%v-%v", w.order.ObjectKeyPrefix, w.objectIndex)
    }

    logger.Tracef("[worker %v] starting delete for object<%v> on %v at %v\n", w.spec.Id, w.objectIndex, conn.Target(), time.Now())

    start := time.Now()
    err := conn.DeleteObject(key, w.objectIndex)
    end := time.Now()

    logger.Tracef("[worker %v] completed delete for object<%v> on %v\n", w.spec.Id, w.objectIndex, conn.Target())

    s := w.nextStat()
    s.Error = SE_None
    s.Phase = SP_Clean
    s.TimeSincePhaseStartMillis = uint32(start.Sub(w.phaseStart) / (1000 * 1000))
    s.DurationMicros = uint32(end.Sub(start) / 1000)
    s.TargetIndex = uint16(w.connIndex)

    if err != nil {
        logger.Warnf("[worker %v] failure deleting object<%v> from %v: %v\n", w.spec.Id, w.objectIndex, conn.Target(), err)
        s.Error = SE_OperationFailure
    }

    w.summary.data[SP_Clean][s.Error]++
    w.sendSummary(&end, true)

    // Advance our object ID ready for next time.
    w.objectIndex++
    if w.objectIndex >= w.order.RangeEnd {
        logger.Tracef("[worker %v] clean up completedv\n", w.spec.Id)
        w.setState(WS_CleanDone)
        return
    }

    // Advance our connection index ready for next time
    w.connIndex = (w.connIndex + 1) % uint64(len(w.connections))
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


func (w *Worker) Id() uint64 {
    return w.spec.Id
}


func (w *Worker) sendResponse(op Opcode, err error) {
    logger.Debugf("[worker %v] sending Response: %v, %v\n", w.spec.Id, op.ToString(), err)
    w.spec.ResponseChannel <- &WorkerResponse{ WorkerId: w.spec.Id, Op: op, Error: err }
}


/*
 * Tell all of our connections to invalidate their caches 
 */
func (w* Worker) invalidateConnectionCaches() {
    for _, conn := range w.connections {
        conn.InvalidateCache()
    }
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

