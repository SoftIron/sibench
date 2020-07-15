package main

import "comms"
import "fmt"
import "io"
import "runtime"
import "time"


/* 
 * All the states a Foreman can be in. 
 *
 * Note that BadTransition isn't really a state, but is the nil value.
 * This is made use of when we look up valid transitions in our state
 * tables: anything not in the table automatically maps to BadTransition.
 */
type foremanState int
const(
    FS_BadTransition foremanState = iota
    FS_Idle
    FS_Connect
    FS_ConnectDone
    FS_WriteStart
    FS_WriteStartDone
    FS_WriteStop
    FS_WriteStopDone
    FS_Prepare
    FS_PrepareDone
    FS_ReadStart
    FS_ReadStartDone
    FS_ReadStop
    FS_ReadStopDone
    FS_Terminate
)


/* Return a human-readable string for each state. */
func foremanStateToStr(state foremanState) string {
    switch state {
        case FS_BadTransition:      return "BadTranstition"
        case FS_Idle:               return "Idle"
        case FS_Connect:            return "Connect"
        case FS_ConnectDone:        return "ConnectDone"
        case FS_WriteStart:         return "WriteStart"
        case FS_WriteStartDone:     return "WriteStartDone"
        case FS_WriteStop:          return "WriteStop"
        case FS_WriteStopDone:      return "WriteStopDone"
        case FS_Prepare:            return "Prepare"
        case FS_PrepareDone:        return "PrepareDone"
        case FS_ReadStart:          return "ReadStart"
        case FS_ReadStartDone:      return "ReadStartDone"
        case FS_ReadStop:           return "ReadStop"
        case FS_ReadStopDone:       return "ReadStopDone"
        case FS_Terminate:          return "Terminate"
        default:                    return "UnknownState"
    }
}


/*
 * A map of the state transitions that may be triggered by incoming TCP opcode (as opposed to 
 * the opcodes contained in the worker responses to our own commands).
 * Each opcode maps to a map from current state to next state. 
 * If an opcode has no entry for the current state, then the output will be the zero value,
 * which in this case is BadTransition. 
 */
var validTcpTransitions = map[Opcode]map[foremanState]foremanState {
    Op_Connect:     { FS_Idle:              FS_Connect, },
    Op_WriteStart:  { FS_ConnectDone:       FS_WriteStart },
    Op_WriteStop:   { FS_WriteStartDone:    FS_WriteStop },
    Op_Prepare:     { FS_WriteStopDone:     FS_Prepare },
    Op_ReadStart:   { FS_PrepareDone:       FS_ReadStart },
    Op_ReadStop:    { FS_ReadStartDone:     FS_ReadStop },
    Op_StatDetails: { FS_WriteStopDone:     FS_WriteStopDone,
                      FS_PrepareDone:       FS_PrepareDone,
                      FS_ReadStopDone:      FS_ReadStopDone },
    Op_Terminate:   { FS_Idle:              FS_Terminate,
                      FS_Connect:           FS_Terminate,
                      FS_ConnectDone:       FS_Terminate,
                      FS_WriteStart:        FS_Terminate,
                      FS_WriteStartDone:    FS_Terminate,
                      FS_WriteStop:         FS_Terminate,
                      FS_WriteStopDone:     FS_Terminate,
                      FS_Prepare:           FS_Terminate,
                      FS_PrepareDone:       FS_Terminate,
                      FS_ReadStart:         FS_Terminate,
                      FS_ReadStartDone:     FS_Terminate,
                      FS_ReadStop:          FS_Terminate,
                      FS_ReadStopDone:      FS_Terminate },
}

/*
 * The same, but for transitions triggered by Worker responses.
 */
var validWorkerTransitions = map[Opcode]map[foremanState]foremanState {
    Op_Connect:     { FS_Connect:           FS_ConnectDone },
    Op_WriteStart:  { FS_WriteStart:        FS_WriteStartDone },
    Op_WriteStop:   { FS_WriteStop:         FS_WriteStopDone },
    Op_Prepare:     { FS_Prepare:           FS_PrepareDone },
    Op_ReadStart:   { FS_ReadStart:         FS_ReadStartDone },
    Op_ReadStop:    { FS_ReadStop:          FS_ReadStopDone },
    Op_Terminate:   { FS_Terminate:         FS_Idle },
}


/*
 * Control opcodes the Foreman can send over a channel to its stats processing go-routine
 * to tell it what to do.
 */
type statControl int
const (
    SC_SendDetails statControl = iota
    SC_StartSummaries
    SC_StopSummaries
    SC_Terminate
)


/* Simple type to bundle up a Worker with the channel through which we control it */
type WorkerInfo struct {
    Worker *Worker
    OpChannel chan Opcode
}


/* 
 * A Foreman is a server that listens on TCP for incoming commands from a Manager and 
 * executes them by farming our the actual work to a horde of Workers.
 *
 * Foremen only work on one TCP connection at a time.  If any other managers attempt
 * to connect, then they will be handed a Op_Busy message and then connection will be 
 * closed.
 *
 * When a Foreman accepts a new WorkOrder from a Manager (sent as part of an Op_Connect
 * message), it spins up a set of Workers.  We have choose the number of workers based
 * on the number of CPU cores of the box that the Foreman is running on.  Typically we
 * do not create more than a dozen or two workers; we do not attempt to get maximum
 * bandwidth from a given machine, as that means that some of workers operations would
 * spend a lot of time blocking.  In effect this would mean that we are benchmarking
 * the local machine, and not the storage cluster!
 *
 * The short version: if you want higher bandwidth from sibench, add more nodes.
 */
type Foreman struct {
    /* Our current WorkOrder, or nil if we are idle. */
    order *WorkOrder

    /* Our current set of workers, or nil if we are idle. */
    workerInfos []*WorkerInfo

    /* A channel on which workers can send their response to us.  
     * They all share the same channel. */
    workerResponseChannel chan *WorkerResponse

    /* Channel used by workers to send us stats */
    statChannel chan *Stat

    /* Channel used to send control messages to our stats procesing go-routine. */
    statControlChannel chan statControl

    /* Channel used by our stats processing go-routine to indicate that it's completed a control request */
    statResponseChannel chan statControl

    /* The channel on which new TCP connections are given to us by our listening socket. */
    tcpControlChannel chan *comms.MessageConnection

    /* The channel on which we are currently sending and receiving with a Manager. */
    tcpMessageChannel chan *comms.ReceivedMessageInfo

    /* The TCP connection we are currently using to talk to a Manager. */
    tcpConnection *comms.MessageConnection

    /* We give each new worker a unique ID, which is not reused when we start a new WorkOrder. */
    nextWorkerId uint64

    /* How many workers have yet to respond to the last opcode we sent them */
    pendingResponses int

    /* Our current state. */
    state foremanState
}


/* 
 * Creates a new Foreman which starts a TCP listening socket and waits for connections.  
 *
 * If we have an error establishing a listening socket, then we return the error.
 *
 * Otherwise, we will start out event-loop.  We will not return from that, so this function
 * should be run as a new go-routine if you need to continue to do things in your current 
 * go-routine.
 */
func StartForeman(listenPort uint16) error {
    var err error
    var f Foreman
    f.setState(FS_Idle)

    endpoint := fmt.Sprintf(":%v", listenPort)
    f.tcpControlChannel = make(chan *comms.MessageConnection, 100)
    _, err = comms.ListenTCP(endpoint, comms.MakeEncoderFactory(), f.tcpControlChannel)
    if err != nil {
        return err
    }

    // Start our event loop in the current goroutine
    f.eventLoop()

    return nil
}


/* Event-loop that endlessly polls for new messages or connections */
func (f *Foreman) eventLoop() {
    for {
        select {
            case conn := <-f.tcpControlChannel:
                f.handleNewTcpConnection(conn)

            case msg := <-f.tcpMessageChannel:
                f.handleTcpMsg(msg)

            case resp := <-f.workerResponseChannel:
                f.handleWorkerResponse(resp)
        }
    }
}


/* Handle a new incoming TCP Connection */
func (f *Foreman) handleNewTcpConnection(conn *comms.MessageConnection) {
    fmt.Printf("Connection from %v\n", conn.RemoteIP())

    // If we aready already have a connection then tell the new one we're busy.
    if f.tcpConnection != nil {
        fmt.Printf("Rejecting connection: already busy\n");
        conn.Send(Op_Busy, nil)
        conn.Close()
        return
    }

    // We're not busy - tell the connection to deliver messages to us over a channel.
    f.tcpConnection = conn
    f.tcpMessageChannel = make(chan *comms.ReceivedMessageInfo)
    conn.ReceiveToChannel(f.tcpMessageChannel)
}


/* Handle a close event on our TCP connection */
func (f *Foreman) handleTcpConnectionClose(msgInfo *comms.ReceivedMessageInfo) {
    conn := msgInfo.Connection

    if msgInfo.Error == io.EOF {
        fmt.Printf("Received remote close from %v\n", conn.RemoteIP())
    } else {
        fmt.Printf("TCP Connection failed from %v: %v\n", conn.RemoteIP(), msgInfo.Error)
    }

    if f.tcpConnection != conn {
        // Not our active connection - just move on...
        return
    }

    // This is our active connection - terminate the job and then wait for a new connection.
    f.tcpConnection = nil

    // If we're in any other state except Idle, then we have stuff to shut down.
    if f.state != FS_Idle {
        f.terminate()
    }
}


/* Handle a TCP message over our existing connection */
func (f *Foreman) handleTcpMsg(msgInfo *comms.ReceivedMessageInfo) {
    if msgInfo.Error != nil {
        f.handleTcpConnectionClose(msgInfo)
        return
    }

    msg := msgInfo.Message
    op := Opcode(msg.ID())

    fmt.Printf("Received message from %v: %v\n", msgInfo.Connection.RemoteIP(), op)

    // See if the Opcode is valid in our current state.
    nextState := validTcpTransitions[op][f.state]
    if nextState == FS_BadTransition {
        f.fail(fmt.Errorf("Bad TCP state transition: %v, %v", foremanStateToStr(f.state), op))
        return
    }

    switch op {
        case Op_Connect:
            // Type assertion without checking since this is generated by the TCP unmarshaller, which already checks.
            msg.Data(&f.order)
            f.connect()

        case Op_StatDetails:
            f.sendStatDetails()

        default:
            f.setState(nextState)
            f.sendOpcodeToWorkers(op)
    }
}


/*
 * Handle a response from a worker, after we asked it to perform some operation.
 *
 * We check that the response is one we were expecting, and that it is legal in our current state.
 *
 * If this is the last worker to respond, then we notify our Manager that we have completed the
 * operation and change our state.
 */
func (f *Foreman) handleWorkerResponse(resp *WorkerResponse) {
    if (f.state == FS_Terminate) && (resp.Op != Op_Terminate) {
        fmt.Printf("Ignoring worker response (%v) as we are terminating\n", resp.Op)
    }

    // Check if this is a bad message.
    nextState := validWorkerTransitions[resp.Op][f.state]
    if nextState == FS_BadTransition {
        f.fail(fmt.Errorf("Bad Worker state transition: %v, %v", foremanStateToStr(f.state), resp.Op))
        return
    }

    f.pendingResponses--

    if f.pendingResponses == 0 {
        // This was the last worker to complete the op.
        f.setState(nextState)
        f.sendResponseToManager(resp.Op, nil)
    }
}


/* Handle a new connection (with its attendant WorkOrder). */
func (f *Foreman) connect() {
    fmt.Printf("Connect for work order in job %v\n", f.order.JobId)

    // Create everything we need before we begin
    f.workerResponseChannel = make(chan *WorkerResponse)
    f.statChannel = make(chan *Stat, 1000)
    f.statControlChannel = make(chan statControl)
    f.statResponseChannel = make(chan statControl)

    go processStats(f.statChannel, f.statControlChannel, f.statResponseChannel, f.tcpConnection)

    // Create our workers.
    // We divvy up our object range between them.

    nWorkers := uint64(runtime.NumCPU())
    f.workerInfos = make([]*WorkerInfo, 0, nWorkers)

    rangeStart := f.order.RangeStart
    rangeLen := f.order.RangeEnd - f.order.RangeStart
    rangeStride := rangeLen / nWorkers

    // Allow for divisions that don't work out nicely by increasing the number allocated to each worker
    // by one.  We'll adjust the last worker to make it match what we were asked to do.
    if rangeLen % nWorkers != 0 {
        rangeStride++
    }

    var err error

    for i := uint64(0); (i < nWorkers) && (err == nil); i++ {
        opChannel := make(chan Opcode, 10)

        s := &WorkerSpec {
            Id: f.nextWorkerId,
            OpChannel: opChannel,
            ResponseChannel: f.workerResponseChannel,
            StatChannel: f.statChannel,
        }

        o := *(f.order)
        o.RangeStart = rangeStart
        o.RangeEnd = rangeStart + rangeStride

        if o.RangeEnd > f.order.RangeEnd {
            o.RangeEnd = f.order.RangeEnd
        }

        w, err := CreateWorker(s, &o)
        if err == nil {
            info := WorkerInfo{OpChannel: opChannel, Worker: w}
            f.workerInfos = append(f.workerInfos, &info)
        }

        rangeStart += rangeStride
        f.nextWorkerId++
    }

    if err != nil {
        f.fail(err)
        return
    }

    // We're all good.  Time to connect.
    f.setState(FS_Connect)
    f.sendOpcodeToWorkers(Op_Connect)
}


/* Send a response to an opcode back to our manager */
func (f *Foreman) sendResponseToManager(op Opcode, err error) {

    // If our connection died, then don't bother trying to send anything.
    if f.tcpConnection == nil {
        fmt.Printf("No connection: not sending response for %v\n", op)
        return
    }

    fmt.Printf("Send response to manager: %v, %v\n", op, err)

    var resp ForemanGenericResponse
    resp.Hostname = f.order.ServerName
    resp.Error = err

    f.tcpConnection.Send(string(op), &resp)
}


/* Send an opcode to all our workers */
func (f *Foreman) sendOpcodeToWorkers(op Opcode) {
    fmt.Printf("Sending op to workers: %v\n", op)

    // When we send out this message, we expect to see each of our workers acknowledge it.
    f.pendingResponses = len(f.workerInfos)

    for _, wi := range f.workerInfos {
        wi.OpChannel <- op
    }
}


/* Set state.  Mostly a place to put a logging statement. */
func (f *Foreman) setState(state foremanState) {
    fmt.Printf("Foreman changing state: %v -> %v\n", foremanStateToStr(f.state), foremanStateToStr(state))
    f.state = state
}


/* Helper function to terminate the current WorkOrder when we hit a failure */
func (f *Foreman) fail(err error) {
    fmt.Printf("Failing with error: %v\n", err)
    f.sendResponseToManager(Op_Failed, err)
    f.terminate()
}


/* Terminate the current WorkOrder, kill our workers and return to the Idle state ready for the 
 * next connection. */
func (f *Foreman) terminate() {
    f.setState(FS_Terminate)
    f.sendOpcodeToWorkers(Op_Terminate)

    // Tell the stats channel to terminate
    fmt.Printf("Waiting for Stats termination\n")
    f.statControlChannel <- SC_Terminate

    // Then wait for it to finish sending stats back to the Manager on our TCP connection
    sc := <-f.statResponseChannel
    if (sc != SC_Terminate) {
        panic("Unexpected stat channel control code")
    }

    fmt.Printf("Stats terminated\n")

    if f.tcpConnection != nil {
        f.tcpConnection.Close()
        f.tcpConnection = nil
        fmt.Print("TCP connction closed\n")
    }
}


/* Tells our Stat-processing go-routine to send all its stored details back to the Manager */
func (f *Foreman) sendStatDetails() {
    f.statControlChannel <- SC_SendDetails

    // Wait for confirmation that it has been done.
    sc := <-f.statResponseChannel

    if (sc != SC_SendDetails) {
        panic("Unexpected stat channel control code")
    }
}


/* 
 * Stats handling function, to be run as its own go-routine.
 *
 * statChannel is a channel to receive the stats which the workers are generating.
 * controlChannel is a channel to receive incoming state changes (including 'shut down please').
 * repsponseChannel is a channel on which we acknowledge receipt of control requests.
 * tcpConnection is a connection back to the Manager so that we can send it stats and summaries 
 * when appropriate. 
 *
 * We start a Ticker to trigger sending a summary back to the Manager once per second.  
 * This can be enabled and disabled by using the controlChannel.
 */
func processStats(statChannel chan *Stat, controlChannel chan statControl, responseChannel chan statControl, tcpConnection *comms.MessageConnection) {
    ticker := time.NewTicker(1 * time.Second)
    var summary = new(StatSummary)
    var stats []*Stat
    sendSummaries := true

    for {
        select {
            case stat := <-statChannel:
                summary[stat.Phase][stat.Error]++
                stats = append(stats, stat)

            case <-ticker.C:
                if sendSummaries {
                    tcpConnection.Send(Op_StatSummary, summary)
                    summary = new(StatSummary)
                }

            case ctl := <-controlChannel:
                switch ctl {
                    case SC_SendDetails:
                        for _, s := range stats {
                            tcpConnection.Send(Op_StatDetails, s)
                        }

                        fmt.Printf("Sent %v detailed stats\n", len(stats))
                        stats = nil
                        tcpConnection.Send(Op_StatDetailsDone, nil)

                    case SC_StartSummaries:
                        sendSummaries = true

                    case SC_StopSummaries:
                        sendSummaries = false

                    case SC_Terminate:
                        ticker.Stop()
                        responseChannel <- SC_Terminate
                        return
                }

                responseChannel <- ctl
        }
    }
}



