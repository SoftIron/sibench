package main

import "comms"
import "fmt"
import "io"
import "logger"
import "os"
import "time"


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
    connToServerName map[*comms.MessageConnection]string
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

    // Create a connection
    var wcc WorkerConnectionConfig
    conn, err := NewConnection(o.ConnectionType, o.Targets[0], o.ProtocolConfig, wcc)
    if err != nil {
        logger.Errorf("Failure making new connection\n")
        return err
    }

    err = conn.ManagerConnect()
    if err != nil {
        logger.Errorf("Failure establishing new connection\n")
        return err
    }

    defer conn.ManagerClose()

    err = m.connectToServers()
    if err != nil {
        logger.Errorf("Failure connecting to servers")
        return err
    }

    m.sendJobToServers()
    phaseTime := j.runTime + j.rampUp + j.rampDown

    // Write
    logger.Infof("\n----------------------- WRITE -----------------------------\n")
    m.runPhase(phaseTime, Op_WriteStart, Op_WriteStop)

    // Prepare
    m.sendOpToServers(Op_Prepare, true)

    // Read
    logger.Infof("\n----------------------- READ ------------------------------\n")
    m.runPhase(phaseTime, Op_ReadStart, Op_ReadStop)

    // Fetch the fully-detailed stats.
    m.drainStats()

    // Process the stats.
    logger.Infof("\n")
    m.job.CrunchTheNumbers()

    // Terminate
    logger.Infof("\n")
    m.sendOpToServers(Op_Terminate, true)

    defer m.disconnectFromServers()

    return nil
}


/* 
 * Sends an operation request to the servers.  
 * If waitForResponse is true, then we block until all the servers have responded.
 */
func (m *Manager) sendOpToServers(op Opcode, waitForResponse bool) {
    logger.Debugf("Sending: %v\n", op)

    // Send our request.
    for _, conn := range m.msgConns {
        conn.Send(string(op), nil)
    }

    if waitForResponse {
        m.waitForResponses(op)
    }
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
func (m* Manager) drainStats() {
    m.sendOpToServers(Op_StatDetails, false)

    count := 0
    pending := len(m.msgConns)

    for {
        msgInfo := <-m.msgChannel
        if msgInfo.Error != nil {
            logger.Errorf("Failure: %v\n", msgInfo.Error)
            os.Exit(-1)
        }

        msg := msgInfo.Message
        op := Opcode(msg.ID())

        // We can ignore anything except StatDetail

        switch op {
            case Op_StatDetails:
                s := new(Stat)
                msg.Data(s)
                m.job.addStat(s)
                count++

            case Op_StatDetailsDone:
                pending--
                if pending == 0 {
                    return
                }

            case Op_StatSummary:
                // Ignore this - we just received one a bit later than expected.

            default:
                logger.Errorf("Unexpected opcode: %v\n", op)
                os.Exit(-1)
        }
    }
}


/*
 * Waits for the specified number of seconds whilst a benchmark executes.
 *
 * During this time, we accept StatSummary messages from the servers.   
 * These are aggragated, and printed out once per second so that the user can
 * see what the system is doing.
 */
func (m* Manager) runPhase(secs uint64, startOp Opcode, stopOp Opcode) {
    m.sendOpToServers(startOp, true)
    m.sendOpToServers(Op_StatSummaryStart, true)

    timer := time.NewTimer(time.Duration(secs + 1) * time.Second)
    ticker := time.NewTicker(time.Second)

    var summary StatSummary
    i := 0

    for {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    if msgInfo.Error == io.EOF {
                        logger.Errorf("Received remote close from %v\n", msgInfo.Connection.RemoteIP())
                    } else {
                        logger.Errorf("%v\n", msgInfo.Error)
                    }
                    os.Exit(-1)
                }

                msg := msgInfo.Message
                op := Opcode(msg.ID())

                if op != Op_StatSummary {
                    logger.Errorf("Unexpected opcode %v\n", op)
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
                m.sendOpToServers(Op_StatSummaryStop, true)
                m.sendOpToServers(stopOp, true)
                return
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
func (m *Manager) waitForResponses(expectedOp Opcode) {
    logger.Debugf("Waiting for %s\n", expectedOp)
    pending := len(m.msgConns)

    for {
        msgInfo := <-m.msgChannel

        if msgInfo.Error != nil {
            logger.Errorf("%v\n", msgInfo.Error)
            os.Exit(-1)
        }

        msg := msgInfo.Message
        op := Opcode(msg.ID())

        if op == expectedOp {
            pending--
            if pending == 0 {
                logger.Debugf("Finished waiting for %v\n", op)
                return
            }
        } else if op != Op_StatSummary {
            logger.Errorf("Unexpected Opcode received: expected %v but got %v\n", expectedOp, op)
            os.Exit(-1)
        }
    }
}


/*
 * Send a job to our current set of servers.
 *
 * This makes a copy of the Job's WorkOrder for each server, and adjusts the object 
 * range of each so that the range is partioned distinctly between the servers. 
 *
 * We block until all the servers have acknowledged the new job.
 */
func (m *Manager) sendJobToServers() {
    nServers := uint64(len(m.msgConns))

    order := &(m.job.order)

    rangeStart := order.RangeStart
    rangeLen := order.RangeEnd - order.RangeStart
    rangeStride := rangeLen / nServers

    // Allow for divisions that don't work out nicely by increasing the number allocated to each server
    // by one.  We'll adjust the last server to make it match what we were asked to do.
    if rangeLen % nServers != 0 {
        rangeStride++
    }

    for _, conn := range m.msgConns {
        // First make a copy of our work order and adjust it for the server.
        o := *order
        o.Bandwidth = order.Bandwidth / nServers
        o.ServerName = m.connToServerName[conn]
        o.RangeStart = rangeStart
        o.RangeEnd = rangeStart + rangeStride

        if o.RangeEnd > order.RangeEnd {
            o.RangeEnd = order.RangeEnd
        }

        rangeStart += rangeStride

        // Tell the server to connect...
        conn.Send(Op_Connect, &o)
    }

    m.waitForResponses(Op_Connect)
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
    m.connToServerName = make(map[*comms.MessageConnection]string)

    for _, s := range m.job.servers {
        endpoint := fmt.Sprintf("%v:%v", s, m.job.serverPort)
        logger.Infof("Connecting to sibench server at %v\n", endpoint)

        conn, err := comms.ConnectTCP(endpoint, comms.MakeEncoderFactory(), 0)
        if err == nil {
            conn.ReceiveToChannel(m.msgChannel)
            m.msgConns = append(m.msgConns, conn)
            m.connToServerName[conn] = s
        } else {
            logger.Warnf("Could not connect to sibench server at %v: %v\n", endpoint, err)
            os.Exit(-1)
        }
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

