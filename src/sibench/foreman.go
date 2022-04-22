// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "comms"
import "fmt"
import "io"
import "logger"
import "os"
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
    FS_ReadWriteStart
    FS_ReadWriteStartDone
    FS_ReadWriteStop
    FS_ReadWriteStopDone
    FS_Terminate
    FS_Hung
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
        case FS_ReadWriteStart:     return "ReadWriteStart"
        case FS_ReadWriteStartDone: return "ReadWriteStartDone"
        case FS_ReadWriteStop:      return "ReadWriteStop"
        case FS_ReadWriteStopDone:  return "ReadWriteStopDone"
        case FS_Terminate:          return "Terminate"
        case FS_Hung:               return "Hung"
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
    OP_Discovery:           { FS_Idle:                  FS_Idle },
    OP_Connect:             { FS_Idle:                  FS_Connect },
    OP_WriteStart:          { FS_ConnectDone:           FS_WriteStart },
    OP_WriteStop:           { FS_WriteStartDone:        FS_WriteStop },
    OP_Prepare:             { FS_ConnectDone:           FS_Prepare,
                              FS_WriteStopDone:         FS_Prepare },
    OP_ReadStart:           { FS_PrepareDone:           FS_ReadStart },
    OP_ReadStop:            { FS_ReadStartDone:         FS_ReadStop },
    OP_ReadWriteStart:      { FS_PrepareDone:           FS_ReadWriteStart },
    OP_ReadWriteStop:       { FS_ReadWriteStartDone:    FS_ReadWriteStop },
    OP_StatDetails:         { FS_WriteStopDone:         FS_WriteStopDone,
                              FS_PrepareDone:           FS_PrepareDone,
                              FS_ReadStopDone:          FS_ReadStopDone,
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone },
    OP_StatSummaryStart:    { FS_WriteStart:            FS_WriteStart,
                              FS_WriteStartDone:        FS_WriteStartDone,
                              FS_WriteStop:             FS_WriteStop,
                              FS_WriteStopDone:         FS_WriteStopDone,
                              FS_Prepare:               FS_Prepare,
                              FS_PrepareDone:           FS_PrepareDone,
                              FS_ReadStart:             FS_ReadStart,
                              FS_ReadStartDone:         FS_ReadStartDone,
                              FS_ReadStop:              FS_ReadStop,
                              FS_ReadStopDone:          FS_ReadStopDone,
                              FS_ReadWriteStart:        FS_ReadWriteStart,
                              FS_ReadWriteStartDone:    FS_ReadWriteStartDone,
                              FS_ReadWriteStop:         FS_ReadWriteStop,
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone },
    OP_StatSummaryStop:     { FS_WriteStart:            FS_WriteStart,
                              FS_WriteStartDone:        FS_WriteStartDone,
                              FS_WriteStop:             FS_WriteStop,
                              FS_WriteStopDone:         FS_WriteStopDone,
                              FS_Prepare:               FS_Prepare,
                              FS_PrepareDone:           FS_PrepareDone,
                              FS_ReadStart:             FS_ReadStart,
                              FS_ReadStartDone:         FS_ReadStartDone,
                              FS_ReadStop:              FS_ReadStop,
                              FS_ReadStopDone:          FS_ReadStopDone,
                              FS_ReadWriteStart:        FS_ReadWriteStart,
                              FS_ReadWriteStartDone:    FS_ReadWriteStartDone,
                              FS_ReadWriteStop:         FS_ReadWriteStop,
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone },
    OP_Terminate:           { FS_Idle:                  FS_Terminate,
                              FS_Connect:               FS_Terminate,
                              FS_ConnectDone:           FS_Terminate,
                              FS_WriteStart:            FS_Terminate,
                              FS_WriteStartDone:        FS_Terminate,
                              FS_WriteStop:             FS_Terminate,
                              FS_WriteStopDone:         FS_Terminate,
                              FS_Prepare:               FS_Terminate,
                              FS_PrepareDone:           FS_Terminate,
                              FS_ReadStart:             FS_Terminate,
                              FS_ReadStartDone:         FS_Terminate,
                              FS_ReadStop:              FS_Terminate,
                              FS_ReadStopDone:          FS_Terminate,
                              FS_ReadWriteStart:        FS_Terminate,
                              FS_ReadWriteStartDone:    FS_Terminate,
                              FS_ReadWriteStop:         FS_Terminate,
                              FS_ReadWriteStopDone:     FS_Terminate,
                              FS_Terminate:             FS_Terminate,
                              FS_Hung:                  FS_Hung },
}

/*
 * The same, but for transitions triggered by Worker responses.
 */
var validWorkerTransitions = map[Opcode]map[foremanState]foremanState {
    OP_Connect:         { FS_Connect:           FS_ConnectDone },
    OP_WriteStart:      { FS_WriteStart:        FS_WriteStartDone },
    OP_WriteStop:       { FS_WriteStop:         FS_WriteStopDone },
    OP_Prepare:         { FS_Prepare:           FS_PrepareDone },
    OP_ReadStart:       { FS_ReadStart:         FS_ReadStartDone },
    OP_ReadStop:        { FS_ReadStop:          FS_ReadStopDone },
    OP_ReadWriteStart:  { FS_ReadWriteStart:    FS_ReadWriteStartDone },
    OP_ReadWriteStop:   { FS_ReadWriteStop:     FS_ReadWriteStopDone },
    OP_Terminate:       { FS_Terminate:         FS_Idle },
    OP_Fail:            { FS_Connect:           FS_Terminate,
                          FS_WriteStart:        FS_Terminate,
                          FS_WriteStop:         FS_Terminate,
                          FS_Prepare:           FS_Terminate,
                          FS_ReadStart:         FS_Terminate,
                          FS_ReadStop:          FS_Terminate,
                          FS_ReadWriteStart:    FS_Terminate,
                          FS_ReadWriteStop:     FS_Terminate,
                          FS_Terminate:         FS_Terminate },
    OP_Hung:            { FS_Connect:           FS_Hung,
                          FS_WriteStart:        FS_Hung,
                          FS_WriteStop:         FS_Hung,
                          FS_Prepare:           FS_Hung,
                          FS_ReadStart:         FS_Hung,
                          FS_ReadStop:          FS_Hung,
                          FS_ReadWriteStart:    FS_Hung,
                          FS_ReadWriteStop:     FS_Hung,
                          FS_Terminate:         FS_Hung },
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
 * to connect, then they will be handed a OP_Busy message and then connection will be 
 * closed.
 *
 * When a Foreman accepts a new WorkOrder from a Manager (sent as part of an OP_Connect
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
    responsePending int

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
func StartForeman() error {
    var err error
    var f Foreman
    f.setState(FS_Idle)

    endpoint := fmt.Sprintf(":%v", globalConfig.ListenPort)
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
    logger.Infof("Connection from %v\n", conn.RemoteIP())

    // If we aready already have a connection then tell the new one we're busy.
    if f.tcpConnection != nil {
        logger.Warnf("Rejecting connection: already busy\n");
        conn.Send(OP_Busy, nil)
        conn.Close()
        return
    }

    // We're not busy - tell the connection to deliver messages to us over a channel.
    f.tcpConnection = conn
    f.tcpMessageChannel = make(chan *comms.ReceivedMessageInfo, 2)
    conn.ReceiveToChannel(f.tcpMessageChannel)
}


/* Handle a close event on our TCP connection */
func (f *Foreman) handleTcpConnectionClose(msgInfo *comms.ReceivedMessageInfo) {
    conn := msgInfo.Connection

    if msgInfo.Error == io.EOF {
        logger.Infof("Received remote close from %v\n", conn.RemoteIP())
    } else {
        logger.Warnf("TCP Connection failed from %v: %v\n", conn.RemoteIP(), msgInfo.Error)
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

    logger.Debugf("Received message from %v: %v\n", msgInfo.Connection.RemoteIP(), op.ToString())

    // See if the Opcode is valid in our current state.
    nextState := validTcpTransitions[op][f.state]
    if nextState == FS_BadTransition {
        f.fail(fmt.Errorf("Bad TCP state transition: %v, %v", foremanStateToStr(f.state), op.ToString()))
        return
    }

    // Type assertions on msg.Data are done without checking since it is generated by the TCP unmarshaller, which already checks.

    switch op {
        case OP_Discovery:
            var d Discovery
            msg.Data(&d)
            d.Cores = uint64(runtime.NumCPU())
            d.Ram = GetPhysicalMemorySize()
            f.tcpConnection.Send(OP_Discovery, d)

        case OP_Connect:
            msg.Data(&f.order)
            f.connect()

        case OP_StatDetails:       f.setStatControl(SC_SendDetails)
        case OP_StatSummaryStart:  f.setStatControl(SC_StartSummaries)
        case OP_StatSummaryStop:   f.setStatControl(SC_StopSummaries)

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
    if resp.Op == OP_Fail {
        f.fail(resp.Error)
        return
    }

    if resp.Op == OP_Hung {
        f.hung(resp.Error)
        return
    }

    if (f.state == FS_Terminate) && (resp.Op != OP_Terminate) {
        logger.Debugf("Ignoring worker response (%v) as we are terminating\n", resp.Op.ToString())
        return
    }

    // Check if this is a bad message.
    nextState := validWorkerTransitions[resp.Op][f.state]
    if nextState == FS_BadTransition {
        f.fail(fmt.Errorf("Bad Worker state transition: %v, %v", foremanStateToStr(f.state), resp.Op.ToString()))
        return
    }

    f.responsePending--

    if f.responsePending == 0 {
        f.setState(nextState)
        f.sendOpcodeToManager(resp.Op, nil)
    }
}


/* Handle a new connection (with its attendant WorkOrder). */
func (f *Foreman) connect() {
    logger.Infof("Connect for work order in job %v\n", f.order.JobId)

    // Create everything we need before we begin
    f.workerResponseChannel = make(chan *WorkerResponse)
    f.statChannel = make(chan *Stat, 1000)
    f.statControlChannel = make(chan statControl)
    f.statResponseChannel = make(chan statControl)

    go processStats(f.statChannel, f.statControlChannel, f.statResponseChannel, f.tcpConnection)

    // Create our workers.
    // We divvy up our object range between them.

    nWorkers := uint64(float64(runtime.NumCPU()) * f.order.WorkerFactor)
    f.workerInfos = make([]*WorkerInfo, 0, nWorkers)

    rangeStart := float32(f.order.RangeStart)
    rangeLen := f.order.RangeEnd - f.order.RangeStart
    rangeStride := float32(rangeLen) / float32(nWorkers)

    hostname, err := os.Hostname()
    if err != nil {
        logger.Errorf("Unable to obtain hostname: %v\n", err)
        f.fail(err)
        return
    }

    for i := uint64(0); (i < nWorkers) && (err == nil); i++ {
        opChannel := make(chan Opcode, 10)

        s := &WorkerSpec {
            Id: f.nextWorkerId,
            OpChannel: opChannel,
            ResponseChannel: f.workerResponseChannel,
            StatChannel: f.statChannel,
        }

        rangeEnd := rangeStart + rangeStride

        o := *(f.order)
        o.Bandwidth = f.order.Bandwidth / nWorkers
        o.RangeStart = uint64(rangeStart)
        o.RangeEnd = uint64(rangeEnd)

        rangeStart = rangeEnd

        s.ConnConfig = WorkerConnectionConfig {
            Hostname: hostname,
            WorkerId: s.Id,
            ObjectSize: o.ObjectSize,
            ForemanRangeStart: f.order.RangeStart,
            ForemanRangeEnd: f.order.RangeEnd,
            WorkerRangeStart: o.RangeStart,
            WorkerRangeEnd: o.RangeEnd }

        w, err := NewWorker(s, &o)
        if err == nil {
            info := WorkerInfo{OpChannel: opChannel, Worker: w}
            f.workerInfos = append(f.workerInfos, &info)
        }

        f.nextWorkerId++
    }

    if err != nil {
        f.fail(err)
        return
    }

    // We're all good.  Time to connect.
    f.setState(FS_Connect)
    f.sendOpcodeToWorkers(OP_Connect)
}


/* Send a response to an opcode back to our manager */
func (f *Foreman) sendOpcodeToManager(op Opcode, err error) {
    // If our connection died, then don't bother trying to send anything.
    if f.tcpConnection == nil {
        logger.Debugf("No connection: not sending response for %v\n", op.ToString())
        return
    }

    var resp ForemanGenericResponse

    if err != nil {
        resp.Error = err.Error()
    }

    logger.Debugf("Send response to manager: %v, %v\n", op.ToString(), err)

    f.tcpConnection.Send(uint8(op), &resp)
}


/* Helper function to terminate the current WorkOrder when we hit a failure */
func (f *Foreman) fail(err error) {
    logger.Errorf("Failing with error: %v\n", err)
    f.sendOpcodeToManager(OP_Fail,  err)
    f.terminate()
}

func (f *Foreman) hung(err error) {
    logger.Errorf("Hung with error: %v\n", err)
    f.sendOpcodeToManager(OP_Hung,  err)
    f.terminate()

    logger.Infof("Exiting process to allow daemon to restart\n")
    os.Exit(-1)
}


/* Send an opcode to all our workers */
func (f *Foreman) sendOpcodeToWorkers(op Opcode) {
    logger.Debugf("Sending op to workers: %v\n", op.ToString())

    // When we send out this message, we expect to see each of our workers acknowledge it.
    f.responsePending = len(f.workerInfos)

    for _, wi := range f.workerInfos {
        wi.OpChannel <- op
    }
}


/* Set state.  Mostly a place to put a loggerging statement. */
func (f *Foreman) setState(state foremanState) {
    logger.Debugf("Foreman changing state: %v -> %v\n", foremanStateToStr(f.state), foremanStateToStr(state))
    f.state = state
}


/* Terminate the current WorkOrder, kill our workers and return to the Idle state ready for the 
 * next connection. */
func (f *Foreman) terminate() {
    f.setState(FS_Terminate)
    f.sendOpcodeToWorkers(OP_Terminate)

    // Tell the stats channel to terminate
    logger.Debugf("Waiting for Stats termination\n")
    f.statControlChannel <- SC_Terminate

    // Then wait for it to finish sending stats back to the Manager on our TCP connection
    sc := <-f.statResponseChannel
    if (sc != SC_Terminate) {
        panic("Unexpected stat channel control code")
    }

    logger.Infof("Stats terminated\n")

    if f.tcpConnection != nil {
        f.tcpConnection.Close()
        f.tcpConnection = nil
        logger.Infof("TCP connction closed\n")
    }

    logger.Infof("WorkOrder Terminated\n")
}


/* Tells our Stat-processing go-routine to send all its stored details back to the Manager */
func (f *Foreman) setStatControl(sc statControl) {
    f.statControlChannel <- sc

    // Wait for confirmation that it has been done.
    if sc != <-f.statResponseChannel {
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
    sendSummaries := false

    for {
        select {
            case stat := <-statChannel:
                summary[stat.Phase][stat.Error]++
                stats = append(stats, stat)

            case <-ticker.C:
                if sendSummaries {
                    tcpConnection.Send(OP_StatSummary, summary)
                    summary = new(StatSummary)
                }

            case ctl := <-controlChannel:
                switch ctl {
                    case SC_SendDetails:
                        // Send Stats 64 at a time, if we have that many.

                        step := 64
                        count := len(stats) - (len(stats) % step)
                        var i int
                        for i = 0; i < count; i += step {
                            tcpConnection.Send(OP_StatDetails, stats[i:i + step])
                        }

                        if i < len(stats) {
                            tcpConnection.Send(OP_StatDetails, stats[i:])
                        }

                        logger.Debugf("Sent %v detailed stats\n", len(stats))
                        stats = nil
                        tcpConnection.Send(OP_StatDetailsDone, nil)

                    case SC_StartSummaries:
                        logger.Debugf("Enabling summaries\n")
                        summary = new(StatSummary)
                        sendSummaries = true
                        tcpConnection.Send(OP_StatSummaryStart, nil)

                    case SC_StopSummaries:
                        logger.Debugf("Disabling summaries\n")
                        sendSummaries = false
                        tcpConnection.Send(OP_StatSummaryStop, nil)

                    case SC_Terminate:
                        ticker.Stop()
                        responseChannel <- SC_Terminate
                        return
                }

                responseChannel <- ctl
        }
    }
}



