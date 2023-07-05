// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import (
	"comms"
	"fmt"
	"io"
	"logger"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
	"unsafe"
)

// Initial value for how long we will wait for a worker to report a summary
// before we conclude that it has hung.
// Note that we will dynamically adjust the actual value in response to incoming
// summaries.
const InitialHangTimeoutSecs = 90

// The value below which our dynamically adjusted hang timeout will not drop.
const MinHangTimeoutSecs = 60


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
    FS_Delete
    FS_DeleteDone
    FS_Terminate
    FS_Hung
)


/*
 * Extra information associated with each state.
 */
type foremanStateDetails struct {
    /* Human readable name for debug statements. */
    name string

    /* If set, on entering this state we should clear out worker timeouts */
    clearTimeouts bool

    /* If set, on entering this state we should start a new CPU profile, using the suffix with the filename. */
    profileStartSuffix string

    /* If set, on entering this state we should finish our CPU profiling, then write a Heap profile, using the
       suffix with the filename. */
    profileStopSuffix string
}


var stateDetails = map[foremanState]foremanStateDetails {
    FS_BadTransition:      { "BadTranstition",      false,  "",             "" },
    FS_Idle:               { "Idle",                false,  "",             "" },
    FS_Connect:            { "Connect",             false,  "",             "" },
    FS_ConnectDone:        { "ConnectDone",         false,  "",             "" },
    FS_WriteStart:         { "WriteStart",          true,   "write",        "" },
    FS_WriteStartDone:     { "WriteStartDone",      false,  "",             "" },
    FS_WriteStop:          { "WriteStop",           false,  "",             "write" },
    FS_WriteStopDone:      { "WriteStopDone",       false,  "",             "" },
    FS_Prepare:            { "Prepare",             true,   "",             "" },
    FS_PrepareDone:        { "PrepareDone",         false,  "",             "" },
    FS_ReadStart:          { "ReadStart",           true,   "read",         "" },
    FS_ReadStartDone:      { "ReadStartDone",       false,  "",             "" },
    FS_ReadStop:           { "ReadStop",            false,  "",             "read" },
    FS_ReadStopDone:       { "ReadStopDone",        false,  "",             "" },
    FS_ReadWriteStart:     { "ReadWriteStart",      true,   "read_write",   "" },
    FS_ReadWriteStartDone: { "ReadWriteStartDone",  false,  "",             "" },
    FS_ReadWriteStop:      { "ReadWriteStop",       false,  "",             "read_write" },
    FS_ReadWriteStopDone:  { "ReadWriteStopDone",   false,  "",             "" },
    FS_Delete:             { "Delete",              true,   "",             "" },
    FS_DeleteDone:         { "DeleteDone",          false,  "",             "" },
    FS_Terminate:          { "Terminate",           false,  "",             "" },
    FS_Hung:               { "Hung",                false,  "",             "" },
}


/* Return a human-readable string for each state. */
func foremanStateToStr(state foremanState) string {
    return stateDetails[state].name;
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
    OP_Delete:              { FS_ReadStopDone:          FS_Delete,
                              FS_ReadWriteStopDone:     FS_Delete },
    OP_StatDetails:         { FS_WriteStopDone:         FS_WriteStopDone,
                              FS_PrepareDone:           FS_PrepareDone,
                              FS_ReadStopDone:          FS_ReadStopDone,
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone,
                              FS_DeleteDone:            FS_DeleteDone },
    OP_StatSummaryStart:    { FS_ConnectDone:           FS_ConnectDone,
                              FS_WriteStart:            FS_WriteStart,
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
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone,
                              FS_Delete:                FS_Delete,
                              FS_DeleteDone:            FS_DeleteDone },
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
                              FS_ReadWriteStopDone:     FS_ReadWriteStopDone,
                              FS_Delete:                FS_Delete,
                              FS_DeleteDone:            FS_DeleteDone },
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
                              FS_Delete:                FS_Terminate,
                              FS_DeleteDone:            FS_Terminate,
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
    OP_Delete:          { FS_Delete:            FS_DeleteDone },
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
    SC_ClearTimeouts
    SC_Terminate
)


/* Simple type to bundle up a Worker with the extra information we need about it. */
type WorkerInfo struct {
    Worker *Worker

    /* The channel we use to control the worker. */
    OpChannel chan Opcode

    /* The last time we saw a summary/heartbeat message from the woker. */
    lastSummary time.Time

    /* Whether the worker is currently running benchmark ops. */
    canTimeout bool
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

    /* Channel used by workers to send us stat summaries */
    summaryChannel chan WorkerSummary

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

    /* How many workers have yet to respond to the last opcode we sent them */
    responsePending int

    /* Our current state. */
    state foremanState

    /* Filename prefix for our profile output (or empty). */
    profilePrefix string

    /* Suffix we'll put on to our profile filename, incremented for each benchmark */
    profileIndex int

    /* Current profiling file (or nil) */
    profileFile *os.File

    /* The dynamically adjusted timeout value for workers */
    hangTimeout time.Duration
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
func StartForeman(profileFilename string) error {
    var err error
    var f Foreman
    f.setState(FS_Idle)
    f.profilePrefix = profileFilename

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
            d.Version = fmt.Sprintf("%s - %s", Version, BuildDate)
            f.tcpConnection.Send(OP_Discovery, d)

        case OP_Connect:
            msg.Data(&f.order)
            f.connect()

        case OP_StatDetails:       f.setStatControl(SC_SendDetails)
        case OP_StatSummaryStart:  f.setStatControl(SC_StartSummaries)
        case OP_StatSummaryStop:   f.setStatControl(SC_StopSummaries)

        case OP_Terminate:         f.terminate()

        default:
            f.setState(nextState)
            f.sendOpcodeToWorkers(op)
    }
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


/*
 * Handle a response from a worker, after we asked it to perform some operation.
 *
 * We check that the response is one we were expecting, and that it is legal in our current state.
 *
 * If this is the last worker to respond, then we notify our Manager that we have completed the
 * operation and change our state.
 */
func (f *Foreman) handleWorkerResponse(resp *WorkerResponse) {
    switch resp.Op {
        // Handle the special failure cases first.
        case OP_Fail: f.fail(resp.Error)
        case OP_Hung: f.hung(resp.Error)

        // Everything wlse is handled the same way.
        default:
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


/* Handle a new connection (with its attendant WorkOrder). */
func (f *Foreman) connect() {
    logger.Infof("Connect for work order in job %v for range %v:%v\n", f.order.JobId, f.order.RangeStart, f.order.RangeEnd)

    // Create everything we need before we begin
    f.workerResponseChannel = make(chan *WorkerResponse)
    f.summaryChannel = make(chan WorkerSummary, 1000)
    f.statControlChannel = make(chan statControl)
    f.statResponseChannel = make(chan statControl)

    // Work out how many workers we need to create.

    nWorkers := uint64(float64(runtime.NumCPU()) * f.order.WorkerFactor)
    rangeStart := float32(f.order.RangeStart)
    rangeLen := f.order.RangeEnd - f.order.RangeStart

    if nWorkers > rangeLen {
        nWorkers = rangeLen
    }

    // Determine how much memory each worker should pre-allocate for stats.
    // (They can allocate more than this, but we'll pick something usable to start with).
    // We'll take a quarter of the physical memory on the box and then divide it between the
    // workers. Then we round down to the nearest power of two.  Finally, we limit it to a
    // million stats.
    var stat Stat
    statPreallocationCount := previousPowerOfTwo(GetPhysicalMemorySize() / (4 * uint64(unsafe.Sizeof(stat)) * nWorkers))
    if statPreallocationCount > (1024 * 1024) {
        statPreallocationCount = 1024 * 1024
    }

    logger.Infof("Pre-allocating %v stats entries per worker\n", ToUnits(statPreallocationCount))

    // Create our workers.
    // We divvy up our object range between them.

    f.workerInfos = make([]*WorkerInfo, 0, nWorkers)
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
            Id: i,
            OpChannel: opChannel,
            ResponseChannel: f.workerResponseChannel,
            SummaryChannel: f.summaryChannel,
            StatPreallocationCount: statPreallocationCount,
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
            info := WorkerInfo{OpChannel: opChannel, Worker: w, lastSummary: time.Now()}
            f.workerInfos = append(f.workerInfos, &info)
        }
    }

    if err != nil {
        f.fail(err)
        return
    }

    go f.processStats()

    // We're all good.  Time to connect.
    f.setState(FS_Connect)
    f.sendOpcodeToWorkers(OP_Connect)
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
    time.Sleep(10)
    os.Exit(-1)
}


/* Set state.  Mostly a place to put a loggerging statement. */
func (f *Foreman) setState(state foremanState) {
    logger.Debugf("Foreman changing state: %v -> %v\n", foremanStateToStr(f.state), foremanStateToStr(state))
    f.state = state

    details := stateDetails[state]

    if details.clearTimeouts {
        f.setStatControl(SC_ClearTimeouts)
    }

    // If profiling is enabled and we're entering the start of a Read or Write phase, then start capturing...
    // Conversely, if profiling is enabled and we're leaving a Read or Write phase, then stop capturing!

    if f.profilePrefix != "" {
        var err error

        if details.profileStartSuffix != "" {
            f.profileIndex++;
            filename := fmt.Sprintf("%v-cpu-%v.%v", f.profilePrefix, details.profileStartSuffix, f.profileIndex)
            logger.Infof("Creating profile output in %v\n", filename)

            f.profileFile, err = os.Create(filename)
            if err != nil {
                f.fail(fmt.Errorf("Unable to create CPU profile results file %v: %v", filename, err))
                return
            }

            pprof.StartCPUProfile(f.profileFile)
        }

        if details.profileStopSuffix != "" {
            logger.Infof("Closing profile output\n")
            pprof.StopCPUProfile()
            f.profileFile.Close()

            filename := fmt.Sprintf("%v-heap-%v.%v", f.profilePrefix, details.profileStopSuffix, f.profileIndex)
            mf, err2 := os.Create(filename)
            if err2 != nil {
                f.fail(fmt.Errorf("Unable to create heap profile results file %v: %v", filename, err2))
                return
            }

            pprof.WriteHeapProfile(mf)
            mf.Close()
        }
    }
}


func (f *Foreman) terminateTCP() {
    if f.tcpConnection != nil {
        f.sendOpcodeToManager(OP_Terminate, nil);
        f.tcpConnection.Close()
        f.tcpConnection = nil
        f.tcpMessageChannel = nil
        logger.Infof("TCP connection closed\n")
    }
}


/* Terminate the current WorkOrder, kill our workers and return to the Idle state ready for the
 * next connection. */
func (f *Foreman) terminate() {
    f.setState(FS_Terminate)
    f.sendOpcodeToWorkers(OP_Terminate)

    timeout := time.NewTimer(f.hangTimeout)
	defer timeout.Stop()

    // And wait for acknowledgment
    for pending := len(f.workerInfos); pending > 0;  {
        select {
            case resp := <-f.workerResponseChannel:
                if resp.Op == OP_Terminate {
                    pending--
                }

			case <- timeout.C:
				logger.Infof("Timing out on worker clean-up in terminate")
                pending = 0
        }
    }

    // Tell the stats channel to terminate
    logger.Debugf("Waiting for Stats termination\n")
    f.statControlChannel <- SC_Terminate

    // Then wait for it to finish sending stats back to the Manager on our TCP connection
    sc := <-f.statResponseChannel
    if (sc != SC_Terminate) {
        panic("Unexpected stat channel control code")
    }

    logger.Infof("Stats terminated\n")

    f.terminateTCP()
    logger.Infof("WorkOrder Terminated\n")

    f.setState(FS_Idle)
}


/* Tells our Stat-processing go-routine to send all its stored details back to the Manager */
func (f *Foreman) setStatControl(sc statControl) {
    f.statControlChannel <- sc

    // Wait for confirmation that it has been done.
    if sc != <-f.statResponseChannel {
        panic("Unexpected stat channel control code")
    }
}


/* Reset the worker timeouts.  Only called from the processStats go routine */
func (f *Foreman) clearHangTimeouts() {
    logger.Debugf("Clearing hang timeout value\n");

    now := time.Now()
    for i, _  := range f.workerInfos {
        f.workerInfos[i].lastSummary = now
    }

    f.hangTimeout = InitialHangTimeoutSecs * time.Second
}


/*
 * Stats handling function, to be run as its own go-routine.
 *
 * Workers send us stats summaries periodically.  These also function as heartbeats, and their absence
 * is used to determine if a worker has hung.  (Note that we only check for this if the worker is in
 * the middle of benchmark phase and thus is running operations on its connections.
 *
 * We start a Ticker to trigger sending a summary back to the Manager once per second.
 * This can be enabled and disabled by using the controlChannel.
 */
func (f *Foreman) processStats() {
    ticker := time.NewTicker(1 * time.Second)
    var summary = new(StatSummary)
    sendSummaries := false

    for {
        select {
            case s := <-f.summaryChannel:
                summary.Add(&s.data)

                now := time.Now()
                wi := f.workerInfos[s.workerId]
                wi.lastSummary = now

                // Adjust our rolling average for operarion duration, so that we can dynamically adjust our timeout.

                ops := s.data.Total()

                if ops > 0 {
                    time_per_op := now.Sub(wi.lastSummary) / time.Duration(ops)
                    f.hangTimeout = ((7 * f.hangTimeout) + (8 * time_per_op)) / 8
                    if (f.hangTimeout < MinHangTimeoutSecs * time.Second) {
                        f.hangTimeout = MinHangTimeoutSecs * time.Second
                    }

                    logger.Tracef("Update from [worker %v] at %v - setting foreman timeout to %0.2f\n", s.workerId, now, f.hangTimeout.Seconds())
                }

                if wi.canTimeout != s.canTimeout {
                    wi.canTimeout = s.canTimeout
                    if s.canTimeout {
                        logger.Debugf("Enabling timeout monitoring for [worker %v]\n", s.workerId)
                    } else {
                        logger.Debugf("Disabling timeout monitoring for [worker %v]\n", s.workerId)
                    }
                }

            case <-ticker.C:
                if sendSummaries {
                    f.tcpConnection.Send(OP_StatSummary, summary)
                    summary = new(StatSummary)

                    // And check for hung workers (defined as any worker that has not send a summary in the
                    // last 90 or so seconds, provided that it should be in the middle of running benchmark ops).

                    now := time.Now()
                    for i, wi  := range f.workerInfos {
                        if wi.canTimeout {
                            if now.Sub(wi.lastSummary) > f.hangTimeout {
                                err := fmt.Errorf("No update from [worker %v] in %0.2f seconds at %v\n", i, f.hangTimeout.Seconds(), now)
                                f.workerResponseChannel <- &WorkerResponse{ WorkerId: uint64(i), Op: OP_Hung, Error: err }
                            }
                        }
                    }
                }

            case ctl := <-f.statControlChannel:
                switch ctl {
                    case SC_ClearTimeouts:
                        f.clearHangTimeouts()

                    case SC_SendDetails:
                        // Tell each worker to send its stats back to the manager.
                        for i, _  := range f.workerInfos {
                            f.workerInfos[i].Worker.UploadStats(f.tcpConnection)
                        }

                        f.tcpConnection.Send(OP_StatDetailsDone, nil)

                    case SC_StartSummaries:
                        logger.Debugf("Enabling summaries\n")
                        summary = new(StatSummary)
                        sendSummaries = true
                        f.tcpConnection.Send(OP_StatSummaryStart, nil)

                    case SC_StopSummaries:
                        logger.Debugf("Disabling summaries\n")
                        sendSummaries = false
                        f.tcpConnection.Send(OP_StatSummaryStop, nil)

                    case SC_Terminate:
                        ticker.Stop()
                        f.statResponseChannel <- SC_Terminate
                        return
                }

                f.statResponseChannel <- ctl
        }
    }
}



