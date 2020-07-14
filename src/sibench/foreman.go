package main

import "comms"
import "fmt"
import "io"
import "os"
import "runtime"
import "time"


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


type statControl int
const (
    SC_SendDetails statControl = iota
    SC_StartSummaries
    SC_StopSummaries
    SC_Terminate
)


type WorkerInfo struct {
    Worker *Worker
    OpChannel chan Opcode
}


type Foreman struct {
    order *WorkOrder
    workerInfos []*WorkerInfo
    workerResponseChannel chan *WorkerResponse
    hostname string

    /* Channel used by workers to send us stats */
    statChannel chan *Stat

    /* Channel used to send control messages to our stats procesing go-routine. */
    statControlChannel chan statControl

    /* Channel used by our stats processing go-routine to indicate that it's completed a control request */
    statResponseChannel chan statControl

    tcpControlChannel chan *comms.MessageConnection
    tcpMessageChannel chan *comms.ReceivedMessageInfo
    tcpConnection *comms.MessageConnection
    nextWorkerId uint64
    state foremanState
    pendingResponses int /* How many workers have yet to respond to the last opcode we sent them */
}


func StartForeman(listenPort uint16) error {
    var err error
    var f Foreman
    f.setState(FS_Idle)

    f.hostname, err = os.Hostname()
    if err != nil {
        return err
    }

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


func (f *Foreman) connect() {
    fmt.Printf("Connect for work order in job %v\n", f.order.JobId)

    // Create everything we need before we begin
    f.workerResponseChannel = make(chan *WorkerResponse)
    f.statChannel = make(chan *Stat, 1000)
    f.statControlChannel = make(chan statControl)
    f.statResponseChannel = make(chan statControl)

    go processStats(f.statChannel, f.statControlChannel, f.statResponseChannel, f.tcpConnection)

    // Create our workers

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
    resp.Hostname = f.hostname
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


func (f *Foreman) setState(state foremanState) {
    fmt.Printf("Foreman changing state: %v -> %v\n", foremanStateToStr(f.state), foremanStateToStr(state))
    f.state = state
}


func (f *Foreman) fail(err error) {
    fmt.Printf("Failing with error: %v\n", err)
    f.sendResponseToManager(Op_Failed, err)
    f.terminate()
}


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


func (f *Foreman) sendStatDetails() {
    // The stats processing go-routing to send details
    f.statControlChannel <- SC_SendDetails

    // Then wait for confirmation that it has been done.
    sc := <-f.statResponseChannel

    if (sc != SC_SendDetails) {
        panic("Unexpected stat channel control code")
    }
}


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



