package main

import "comms"
import "fmt"
import "io"
import "logger"
import "os"
import "os/signal"
import "syscall"
import "time"


type ServerDetails struct {
    Discovery
    Name string
    Index uint16
}


/*
 * A Manager handles connecting to a set of Foremen over TCP and executing
 * a benchmarking job on them.
 *
 * Currently a manager can only handle running a single job, but this would also 
 * be the right place to add queueing, or a job-listening socket, or anything 
 * else that you would need to manage multiple users with multiple (possibly
 * simultaneous requests).  
 *
 * For the moment, though, this is just brain-dead simple.
 */
type Manager struct {
    job *Job
    msgConns []*comms.MessageConnection
    msgChannel chan *comms.ReceivedMessageInfo
    connToServerDetails map[*comms.MessageConnection]*ServerDetails
    totalCoreCount uint64
    sigChan chan os.Signal
    isInterrupted bool
}


/* Creates a new manager object. */
func NewManager() *Manager{
    var m Manager
    return &m
}


/* Runs a single benchmark on the manager */
func (m *Manager) Run(j *Job) error {
    m.job = j
    o := &(m.job.order)

    // Ensure that we can connect to at least the first target ourselves.  If we can't then
    // there's no need to bother the driver nodes about this at all.
    var wcc WorkerConnectionConfig
    conn, err := NewConnection(o.ConnectionType, o.Targets[0], o.ProtocolConfig, wcc)
    if err != nil {
        logger.Errorf("Failure making new connection: %v\n", err)
        return err
    }

    err = conn.ManagerConnect()
    if err != nil {
        logger.Errorf("Failure establishing new connection: %v\n", err)
        return err
    }

    defer conn.ManagerClose()

    err = m.connectToServers()
    if err != nil {
        logger.Errorf("Failure connecting to servers: %v", err)
        return err
    }

    defer m.disconnectFromServers()

    // Ask our servers to tell us about themselves
    err = m.discoverServerCapabilities()
    if err != nil {
        logger.Errorf("Failure discovering server capabilities: %v\n", err)
        return err
    }

    err = m.sendJobToServers()
    if err != nil {
        logger.Errorf("Failure sending job to servers: %v\n", err)
        return err
    }

    // Register for interrupts before we do the actual work
    m.sigChan = make(chan os.Signal, 1)
    signal.Notify(m.sigChan, syscall.SIGINT, syscall.SIGTERM)

    phaseTime := j.runTime + j.rampUp + j.rampDown

    if j.order.ReadWriteMix == 0 {
        // Write
        logger.Infof("\n----------------------- WRITE -----------------------------\n")
        err = m.runPhase(phaseTime, OP_WriteStart, OP_WriteStop)
        if err != nil {
            logger.Errorf("Failure during write phase: %v\n", err)
            return err
        }

        // Prepare
        logger.Infof("Preparing...\n")
        m.sendOpToServers(OP_Prepare, true)
        if err != nil {
            logger.Errorf("Failure during prepare phase: %v\n", err)
            return err
        }

        // Read
        logger.Infof("\n----------------------- READ ------------------------------\n")
        err = m.runPhase(phaseTime, OP_ReadStart, OP_ReadStop)
        if err != nil {
            logger.Errorf("Failure during read phase: %v\n", err)
            return err
        }
    } else {
        // Prepare
        logger.Infof("Preparing...\n")
        m.sendOpToServers(OP_Prepare, true)
        if err != nil {
            logger.Errorf("Failure during prepare phase: %v\n", err)
            return err
        }

        // Read
        logger.Infof("\n--------------------- READ/WRITE --------------------------\n")
        err = m.runPhase(phaseTime, OP_ReadWriteStart, OP_ReadWriteStop)
        if err != nil {
            logger.Errorf("Failure during read/write phase: %v\n", err)
            return err
        }

    }
    // Process the stats.
    logger.Infof("\n")
    m.job.report.DisplayAnalyses()

    // Terminate
    logger.Infof("\n")
    err = m.sendOpToServers(OP_Terminate, true)
    if err != nil {
        logger.Errorf("Failure sending terminate message to servers: %v\n", err)
        return err
    }

    return nil
}


/* 
 * Sends an operation request to the servers.  
 * If waitForResponse is true, then we block until all the servers have responded.
 */
func (m *Manager) sendOpToServers(op Opcode, waitForResponse bool) error {
    if m.isInterrupted && (op != OP_Terminate) {
        return nil
    }

    logger.Debugf("Sending: %v\n", op)

    // Send our request.
    for _, conn := range m.msgConns {
        conn.Send(string(op), nil)
    }

    if waitForResponse {
        return m.waitForResponses(op)
    }

    return nil
}


/*
 * Check if an incoming message is an error type, and convert it to error if so.
 */
func (m *Manager) checkError(msgInfo *comms.ReceivedMessageInfo) error {
    msg := msgInfo.Message
    op := Opcode(msg.ID())

    if (op != OP_Fail) && (op != OP_Hung) {
        return nil
    }

    var resp ForemanGenericResponse
    msg.Data(&resp)

    details := m.connToServerDetails[msgInfo.Connection]
    return fmt.Errorf("%v:%v", details.Name, resp.Error)
}


/* 
 * When we have complete a phase (or the whole run!) we can ask the servers to 
 * send us all the detailed stats that they have been collecting (and to then
 * forget about them themselves).  
 * 
 * (The detailed  stats are NOT sent during the benchmark's execution as it may be a 
 * lot of traffic, though once-per-second summaries are sent during that time).
 *
 * We return the stats we obtain this way.
 */
func (m* Manager) drainStats() error {
    if m.isInterrupted {
        return nil
    }

    err := m.sendOpToServers(OP_StatDetails, false)
    if err != nil {
        return err
    }

    count := 0
    pending := len(m.msgConns)

    for pending > 0 {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    return fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
                }

                err := m.checkError(msgInfo)
                if err != nil {
                    return err
                }

                msg := msgInfo.Message
                op := Opcode(msg.ID())

                // We can ignore anything except StatDetail

                switch op {
                    case OP_StatDetails:
                        var s Stat
                        msg.Data(&s)

                        details := m.connToServerDetails[msgInfo.Connection]

                        ss := new(ServerStat)
                        ss.ServerIndex = details.Index
                        ss.Stat = s

                        m.job.report.AddStat(ss)
                        count++

                    case OP_StatDetailsDone:
                        pending--

                    case OP_StatSummary:
                        // Ignore this - we just received one a bit later than expected.

                    default:
                        return fmt.Errorf("Unexpected opcode: %v\n", op)
                }

            case <-m.sigChan:
                logger.Infof("Interrupting stats collection and waiting to shut down\n")
                m.isInterrupted = true
                return nil
        }
    }

    m.job.report.AnalyseStats()
    return nil
}


/*
 * Waits for the specified number of seconds whilst a benchmark executes.
 *
 * During this time, we accept StatSummary messages from the servers.   
 * These are aggragated, and printed out once per second so that the user can
 * see what the system is doing.
 */
func (m* Manager) runPhase(secs uint64, startOp Opcode, stopOp Opcode) error {
    if m.isInterrupted {
        return nil
    }

    m.sendOpToServers(startOp, true)
    m.sendOpToServers(OP_StatSummaryStart, true)

    timer := time.NewTimer(time.Duration(secs + 1) * time.Second)
    ticker := time.NewTicker(time.Second)

    var summary StatSummary
    i := 0

    for {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    if msgInfo.Error == io.EOF {
                        return fmt.Errorf("Received remote close from %v\n", msgInfo.Connection.RemoteIP())
                    }
                    return fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
                }

                msg := msgInfo.Message
                err := m.checkError(msgInfo)
                if err != nil {
                    return err
                }

                op := Opcode(msg.ID())
                if op != OP_StatSummary {
                    return fmt.Errorf("Unexpected opcode %v\n", op)
                }

                var s StatSummary
                msg.Data(&s)
                summary.Add(&s)

            case <-ticker.C:
                logger.Infof("%v: %v\n", i, summary.String(m.job.order.ObjectSize))
                i++

                // Draw some lines to indicate the ramp-up/ramp-down demarcation.
                if (uint64(i) == m.job.rampUp) || (uint64(i) == m.job.rampUp + m.job.runTime) {
                    logger.Infof("-----------------------------------------------------------\n")
                }

                summary.Zero()

            case <-timer.C:
                ticker.Stop()
                err := m.sendOpToServers(OP_StatSummaryStop, true)
                if err != nil {
                    return err
                }

                err = m.sendOpToServers(stopOp, true)
                if err != nil {
                    return err
                }

                return m.drainStats()

            case <-m.sigChan:
                logger.Infof("Interrupting job and waiting to shut down\n")
                ticker.Stop()
                m.isInterrupted = true
                return nil
        }
    }
}


/* 
 * Blocks until all the servers have responded with the specified opcode.
 *
 * Any unexpected opcodes recieved from the servers will cause us to error out.
 * The exception to that is StatSummary messages, which can be received at any
 * time, and which are just ignored here.
 */
func (m *Manager) waitForResponses(expectedOp Opcode) error {
    logger.Debugf("Waiting for %s\n", expectedOp)
    pending := len(m.msgConns)

    for {
        msgInfo := <-m.msgChannel

        if msgInfo.Error != nil {
            logger.Errorf("%v\n", msgInfo.Error)
            os.Exit(-1)
        }

        err := m.checkError(msgInfo)
        if err != nil {
            return err
        }

        msg := msgInfo.Message
        op := Opcode(msg.ID())

        if op == expectedOp {
            var resp ForemanGenericResponse
            msg.Data(&resp)

            pending--
            if pending == 0 {
                logger.Debugf("Received %v, finished waiting\n", op)
                return nil
            }

            logger.Debugf("Received %v, still waiting for %v more\n", op, pending)
        } else if op != OP_StatSummary {
            // Stat Summary messages can arrive later than expected because they're asynchronous.  
            // If we see one when we don't want one, we just drop it.  
            // All other unexpected opcodes are an error.
            return fmt.Errorf("Unexpected Opcode received: expected %v but got %v\n", expectedOp, op)
        }
    }
}


/*
 * Send a job to our current set of servers.
 *
 * This makes a copy of the Job's WorkOrder for each server, and adjusts the object 
 * range of each so that the range is partioned distinctly between the servers. 
 *
 * Each server is allocated a section proportional to the number of cores it has.
 *
 * We block until all the servers have acknowledged the new job.
 */
func (m *Manager) sendJobToServers() error {
    order := &(m.job.order)

    rangeStart := float32(order.RangeStart)
    rangeLen := order.RangeEnd - order.RangeStart
    rangeStridePerCore := float32(rangeLen) / float32(m.totalCoreCount)

    hostsWithLowRam := make([]string, 0, 16)

    for _, conn := range m.msgConns {
        details := m.connToServerDetails[conn]

        // First make a copy of our work order and adjust it for the server.
        o := *order

        rangeEnd := rangeStart + (rangeStridePerCore * float32(details.Cores))

        o.Bandwidth = (order.Bandwidth * details.Cores) / m.totalCoreCount
        o.RangeStart = uint64(rangeStart)
        o.RangeEnd = uint64(rangeEnd)

        rangeStart = rangeEnd

        // Check if we should warn about memory usage for this server
        if ((o.RangeEnd - o.RangeStart) * o.ObjectSize) * 10 > (details.Ram * 8) {
            hostsWithLowRam = append(hostsWithLowRam, details.Name)
        }

        // Tell the server to connect...
        logger.Debugf("Sending job to %s with start: %v, end: %v, bandwidth: %v\n", details.Name, o.RangeStart, o.RangeEnd, o.Bandwidth)
        conn.Send(OP_Connect, &o)
    }

    // Check if we should warn about hosts with low RAM
    if len(hostsWithLowRam) > 0 {
        logger.Warnf("--------------------------------------------------------------------\n")
        logger.Warnf("\n")
        logger.Warnf("The job may take large proportion of the RAM on the following hosts:\n")

        for _, host := range(hostsWithLowRam) {
            logger.Warnf("    %v\n", host)
        }

        logger.Warnf("\n")
        logger.Warnf("This may result in swapping (which will make the benchmarks invalid),\n")
        logger.Warnf("Or the OS may choose to kill the sibench daemon without warning.\n")
        logger.Warnf("\n")
        logger.Warnf("--------------------------------------------------------------------\n")
    }

    return m.waitForResponses(OP_Connect)
}


/*
 * Interogates each sibench server for information about core count, RAM size and 
 * so forth, so that we can allocate the workloads appropriately later.
 */
func (m *Manager) discoverServerCapabilities() error {
    logger.Debugf("Sending Server Capability Discovery requests\n")
    for _, conn := range m.msgConns {
        conn.Send(OP_Discovery, nil)
    }

    m.totalCoreCount = 0

    logger.Infof("\n---------- Sibench driver capabilities discovery ----------\n")
    pending := len(m.msgConns)

    for pending > 0 {
        msgInfo := <-m.msgChannel

        if msgInfo.Error != nil {
            return fmt.Errorf("%v\n", msgInfo.Error)
        }

        msg := msgInfo.Message

        op := Opcode(msg.ID())
        if op != OP_Discovery {
            return fmt.Errorf("Unexpected Opcode received: expected Discovery but got %v\n", op)
        }

        d := m.connToServerDetails[msgInfo.Connection]
        msg.Data(&d.Discovery)

        // Find our details object

        logger.Infof("%s: %v cores, %vB of RAM\n", d.Name, d.Cores, ToUnits(d.Ram))
        m.totalCoreCount += d.Cores

        pending--
    }

    logger.Debugf("Discovery complete\n\n")
    return nil
}


/* 
 * Attempts to connect to a set of servers (as specified in our current Job).
 *
 * Currently we exit with a non-zero error code if we can't connect to all of them.  
 *
 * In future (if we add job queuing, and the Manager becomes a daemon) then we could
 * change this to logger the errors but continue with whatever servers we could 
 * successfully talk to.
 */
func (m *Manager) connectToServers() error {
    // Construct our aggregated recv channel
    m.msgChannel = make(chan *comms.ReceivedMessageInfo, 1000)
    m.connToServerDetails = make(map[*comms.MessageConnection]*ServerDetails)

    for i, s := range m.job.servers {
        endpoint := fmt.Sprintf("%v:%v", s, m.job.serverPort)
        logger.Infof("Connecting to sibench server at %v\n", endpoint)

        conn, err := comms.ConnectTCP(endpoint, comms.MakeEncoderFactory(), 0)
        if err != nil {
            return fmt.Errorf("Could not connect to sibench server at %v: %v\n", endpoint, err)
        }

        conn.ReceiveToChannel(m.msgChannel)
        m.msgConns = append(m.msgConns, conn)

        details := new(ServerDetails)
        details.Name = s
        details.Index = uint16(i)

        m.connToServerDetails[conn] = details
    }

    return nil
}


/* Disconnects from all the Foremen that we are successfully connected to. */
func (m *Manager) disconnectFromServers() {
    logger.Infof("Disconnecting from servers\n")

    for _, c := range m.msgConns {
        c.Close()
    }

    logger.Infof("Disconnected\n")
}

