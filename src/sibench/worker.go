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
    WS_Failed
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
        case WS_Failed:         return "Failed"
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
    Op_Connect:     { WS_Init:           WS_Connect },
    Op_WriteStart:  { WS_ConnectDone:    WS_Write },
    Op_WriteStop:   { WS_Write:          WS_WriteDone },
    Op_Prepare:     { WS_WriteDone:      WS_Prepare },
    Op_ReadStart:   { WS_PrepareDone:    WS_Read },
    Op_ReadStop:    { WS_Read:           WS_ReadDone },
    Op_Terminate:   { WS_Init:           WS_Terminated,
                      WS_Connect:        WS_Terminated,
                      WS_ConnectDone:    WS_Terminated,
                      WS_Write:          WS_Terminated,
                      WS_WriteDone:      WS_Terminated,
                      WS_Prepare:        WS_Terminated,
                      WS_PrepareDone:    WS_Terminated,
                      WS_Read:           WS_Terminated,
                      WS_ReadDone:       WS_Terminated,
                      WS_Failed:         WS_Terminated },
}


/*
 * When reporting how long each read, write or prepare operation took, we can encode
 * any errors we encounter as negative durations.
 */
const (
    Stat_VerifyFailure time.Duration = -1
    Stat_OperationFailure           = -2
)



/* A WorkerResponse reports the error from an opcode, which is nil if the opcode succeeded. */
type WorkerResponse struct {
    WorkerId uint64
    Op Opcode
    Error  error
}


/* The arguments used to construct a worker.  They have been bundled into a struct purely for readability. */
type WorkerSpec struct {
    Id uint64
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
    lastDelay time.Time
}


func NewWorker(spec *WorkerSpec, order *WorkOrder) (*Worker, error) {
    logger.Debugf("[worker %v] creating worker\n", spec.Id)

    var w Worker
    w.spec = *spec
    w.order = *order
    w.objectIndex = order.RangeStart
    w.setState(WS_Init)

    var err error
    w.generator, err = CreateGenerator(order.GeneratorType, order.Seed)
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


func (w *Worker) sendResponse(op Opcode, err error) {
    logger.Debugf("[worker %v] sending Response: %v, %v\n", w.spec.Id, op, err)
    w.spec.ResponseChannel <- &WorkerResponse{ WorkerId: w.spec.Id, Op: op, Error: err }
}



/* Insert delays in order to limit bandwitdh */
func (w *Worker) bandwidthDelay() {
    // See if we need to do anything in the first place.
    if ((w.state != WS_Write) && (w.state != WS_Read)) || (w.order.Bandwidth == 0) {
        return
    }

    if w.lastDelay.Unix() == 0 {
        // Random delay to smooth out between workers.
        time.Sleep(time.Duration(rand.Intn(1000 * 1000 * 10)))
        w.lastDelay = time.Now()
        return
    }

    now := time.Now()
    elapsed := now.Sub(w.lastDelay)

    desired := time.Duration(1000 * 1000 * 1000 * w.order.ObjectSize / w.order.Bandwidth)

    if w.spec.Id == 0 {
        fmt.Printf("BW: %v, Elapsed: %v, Desired: %v\n", w.order.Bandwidth, elapsed, desired)
    }

    if desired > elapsed {
        noise := (0.05 * rand.NormFloat64() * 2.0) + 1.0
        time.Sleep(time.Duration(float64(desired - elapsed) * noise))
    }

    w.lastDelay = time.Now()
}


func (w *Worker) eventLoop() {
    for w.state != WS_Terminated {
        select {
            case op := <-w.spec.OpChannel: w.handleOpcode(op)
            default: // default is needed to prevent blocking when there's no opcode to process.
        }

        w.bandwidthDelay()

        switch w.state {
            case WS_Connect:  w.connect()
            case WS_Write:    w.write()
            case WS_Prepare:  w.prepare()
            case WS_Read:     w.read()
            default:
        }
    }

    logger.Debugf("[worker %v] shutting down\n", w.spec.Id)

    for _, conn := range w.connections {
        conn.Close()
    }
}


func (w *Worker) handleOpcode(op Opcode) {
    logger.Debugf("[worker %v] handleOpcode: %v\n", w.spec.Id, op)

    // See if the Opcode is valid in our current state.
    nextState := validWSTransitions[op][w.state]
    if nextState == WS_BadTransition {
        logger.Errorf("[worker %v] handleOpcode: bad transition from state %v\n", w.spec.Id, workerStateToStr(w.state))
        w.sendResponse(op, fmt.Errorf("Bad state transition"))
        w.setState(WS_Failed)
        return
    }

    // Connect and Prepare are the two opcodes that take time to complete, and so we send responses for 
    // them when they have actually completed their work.
    // The rest of the opcodes take place immediately, so we can respond with acknowledgement here.

    if (op != Op_Connect) && (op != Op_Prepare) {
        w.sendResponse(op, nil)
    }

    w.setState(nextState)
    w.phaseStart = time.Now()
}


func (w *Worker) connect() {
    for _, t := range w.order.Targets {
        conn, err := NewConnection(w.order.ConnectionType, t, w.order.Port, w.order.Credentials)
        if err != nil {
            logger.Errorf("[worker %v] failure during connect to %v: %v\n", w.spec.Id, t, err)
            w.setState(WS_Failed)
            w.sendResponse(Op_Connect, err)
            return
        }

        w.connections = append(w.connections, conn)
    }

    logger.Debugf("[worker %v] successfully connected\n", w.spec.Id)
    w.setState(WS_ConnectDone)
    w.sendResponse(Op_Connect, nil)
}


func (w *Worker) writeOrPrepare(phase StatPhase) {
    key := fmt.Sprintf("obj_%v", w.objectIndex)
    contents := w.generator.Generate(w.order.ObjectSize, key, w.cycle)
    conn := w.connections[w.connIndex]

    // Actually do the PUT
    start := time.Now()
    err := conn.PutObject(w.order.Bucket, key, contents)
    end := time.Now()

    var s Stat
    s.Error = SE_None
    s.Phase = phase
    s.TimeSincePhaseStart = end.Sub(w.phaseStart)
    s.Duration = end.Sub(start)
    s.Target = conn.Target()
    s.Server = w.order.ServerName

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
        logger.Debugf("[worker %v] advancing cycle to %v\n", w.spec.Id, w.cycle)
    }

    // Advance our connection index ready for next time
    w.connIndex = (w.connIndex + 1) % uint64(len(w.connections))
}


func (w *Worker) write() {
    w.writeOrPrepare(SP_Write)
}


func (w *Worker) prepare() {
    // See if we've prepared a whole cycle of objects.
    if w.cycle > 0 {
        w.setState(WS_PrepareDone)
        w.sendResponse(Op_Prepare, nil)
        return
    }

    w.writeOrPrepare(SP_Prepare)
}


func (w *Worker) read() {
    key := fmt.Sprintf("obj_%v", w.objectIndex)
    conn := w.connections[w.connIndex]

    // logger.Debugf(" [worker %v] getting object<%v> from %v\n", w.spec.Id, key, conn.Target())

    // Actually do the GET
    start := time.Now()
    contents, err := conn.GetObject(w.order.Bucket, key)
    end := time.Now()

    var s Stat
    s.Error = SE_None
    s.Phase = SP_Read
    s.TimeSincePhaseStart = end.Sub(w.phaseStart)
    s.Duration = end.Sub(start)
    s.Target = conn.Target()
    s.Server = w.order.ServerName

    if err != nil {
        logger.Warnf("[worker %v] failure getting object<%v> to %v: %v\n", w.spec.Id, key, conn.Target(), err)
        s.Error = SE_OperationFailure
    } else {
        err = w.generator.Verify(w.order.ObjectSize, key, contents)
        if err != nil {
            logger.Warnf("[worker %v] failure verfiying object<%v> to %v: %v\n", w.spec.Id, key, conn.Target(), err)
            s.Error = SE_VerifyFailure
        }
    }

    // Submit the stat
    w.spec.StatChannel <- &s

    // Advance our object ID ready for next time.
    w.objectIndex++
    if w.objectIndex >= w.order.RangeEnd {
        w.objectIndex = w.order.RangeStart
    }

    // Advance our connection index ready for next time
    w.connIndex = (w.connIndex + 1) % uint64(len(w.connections))
}


func (w *Worker) setState(state workerState) {
    logger.Debugf("[worker %v] Worker changing state: %v -> %v\n", w.spec.Id, workerStateToStr(w.state), workerStateToStr(state))
    w.state = state
}

