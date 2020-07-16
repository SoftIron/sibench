package main

import "comms"
import "fmt"
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
    storageConn Connection
    msgConns []*comms.MessageConnection
    msgChannel chan *comms.ReceivedMessageInfo
    connToServerName map[*comms.MessageConnection]string
}


/* Creates a new manager object. */
func CreateManager() *Manager{
    var m Manager
    return &m
}


/* Runs a single benchmark on the manager */
func (m *Manager) Run(j *Job) error {
    m.job = j

    err := m.createBucket()
    if err != nil {
        return err
    }

    defer m.deleteBucket()

    err = m.connectToServers()
    if err != nil {
        return err
    }

    m.sendJobToServers()
    phaseTime := j.RunTime + j.RampUp + j.RampDown

    // Write
    m.sendOpToServers(Op_WriteStart, true)
    m.runPhase(phaseTime)
    m.sendOpToServers(Op_WriteStop, true)

    // Prepare
    m.sendOpToServers(Op_Prepare, true)

    // Read
    m.sendOpToServers(Op_ReadStart, true)
    m.runPhase(phaseTime)
    m.sendOpToServers(Op_ReadStop, true)

    // Fetch the fully-detailed stats, and then process them.
    stats := m.drainStats()
    CrunchTheNumbers(stats, m.job)

    // Terminate
    m.sendOpToServers(Op_Terminate, true)

    defer m.disconnectFromServers()

    return nil
}


/* 
 * Sends an operation request to the servers.  
 * If waitForResponse is true, then we block until all the servers have responded.
 */
func (m *Manager) sendOpToServers(op Opcode, waitForResponse bool) {
    fmt.Printf("Sending: %v\n", op)

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
func (m* Manager) drainStats() []*Stat {
    m.sendOpToServers(Op_StatDetails, false)

    var stats []*Stat
    count := 0
    pending := len(m.msgConns)

    for {
        msgInfo := <-m.msgChannel
        if msgInfo.Error != nil {
            fmt.Printf("Failure: %v\n", msgInfo.Error)
            os.Exit(-1)
        }

        msg := msgInfo.Message
        op := Opcode(msg.ID())

        // We can ignore anything except StatDetail

        switch op {
            case Op_StatDetails:
                s := new(Stat)
                msg.Data(s)
                stats = append(stats, s)
                count++

            case Op_StatDetailsDone:
                pending--
                if pending == 0 {
                    return stats
                }

            case Op_StatSummary:
                // Ignore this - we just received one a bit later than expected.

            default:
                fmt.Printf("Unexpected opcode: %v\n", op)
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
func (m* Manager) runPhase(secs uint64) {
    timer := time.NewTimer(time.Duration(secs + 1) * time.Second)
    ticker := time.NewTicker(time.Second)

    var summary StatSummary
    i := 0

    for {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    fmt.Printf("Failure: %v\n", msgInfo.Error)
                    os.Exit(-1)
                }

                msg := msgInfo.Message
                op := Opcode(msg.ID())

                if op != Op_StatSummary {
                    fmt.Printf("Failure: unexpected opcode %v\n", op)
                }

                var s StatSummary
                msg.Data(&s)
                summary.Add(&s)

            case <-ticker.C:
                fmt.Printf("%v: %v\n", i, summary.String(m.job.Order.ObjectSize))
                i++

                // Draw some lines to indicate the ramp-up/ramp-down demarcation.
                if (uint64(i) == m.job.RampUp) || (uint64(i) == m.job.RampUp + m.job.RunTime) {
                    fmt.Printf("-----------------------------------------------------------\n")
                }

                summary.Zero()

            case <-timer.C:
                ticker.Stop()
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
    fmt.Printf("Waiting for %s\n", expectedOp)
    pending := len(m.msgConns)

    for {
        msgInfo := <-m.msgChannel

        if msgInfo.Error != nil {
            fmt.Printf("Failure: %v\n", msgInfo.Error)
            os.Exit(-1)
        }

        msg := msgInfo.Message
        op := Opcode(msg.ID())

        if op == expectedOp {
            pending--
            if pending == 0 {
                fmt.Printf("Finished waiting\n")
                return
            }
        } else if op != Op_StatSummary {
            fmt.Printf("Unexpected Opcode received: expected %v but got %v\n", expectedOp, op)
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

    order := &(m.job.Order)

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
 * change this to log the errors but continue with whatever servers we could 
 * successfully talk to.
 */
func (m *Manager) connectToServers() error {
    // Construct our aggregated recv channel
    m.msgChannel = make(chan *comms.ReceivedMessageInfo, 1000)
    m.connToServerName = make(map[*comms.MessageConnection]string)

    for _, s := range m.job.Servers {
        endpoint := fmt.Sprintf("%v:%v", s, m.job.ServerPort)
        fmt.Printf("Connecting to sibench server at %v\n", endpoint)

        conn, err := comms.ConnectTCP(endpoint, comms.MakeEncoderFactory(), 0)
        if err == nil {
            conn.ReceiveToChannel(m.msgChannel)
            m.msgConns = append(m.msgConns, conn)
            m.connToServerName[conn] = s
        } else {
            fmt.Printf("Could not connect to sibench server at %v: %v\n", endpoint, err)
            os.Exit(-1)
        }
    }

    return nil
}


/* Disconnects from all the Foremen that we are successfully connected to. */
func (m *Manager) disconnectFromServers() {
    fmt.Printf("Disconnecting from servers\n")

    for _, c := range m.msgConns {
        c.Close()
    }

    fmt.Print("Disconnected\n")
}


/* Create a bucket in which to put/get our benchmark objects. */
func (m *Manager) createBucket() error {
    o := &(m.job.Order)

    var err error

    // Create a connection
    m.storageConn, err = CreateConnection(o.ConnectionType, o.Targets[0], o.Port, o.Credentials)

    if err != nil {
        return err
    }

    // Create bucket
    return m.storageConn.CreateBucket(m.job.Order.Bucket)
}


/* Delete the bucket we use for our benchmark objects */
func (m *Manager) deleteBucket() {
    err := m.storageConn.DeleteBucket(m.job.Order.Bucket)
    if err != nil {
        fmt.Printf("Error deleting bucket: %v\n", err)
    }

    m.storageConn.Close()
}



