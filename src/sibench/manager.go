package main

import "comms"
import "fmt"
import "os"
import "time"


type Manager struct {
    job *Job
    storageConn Connection
    msgConns []*comms.MessageConnection
    msgChannel chan *comms.ReceivedMessageInfo
}



func CreateManager() *Manager{
    var m Manager
    return &m
}


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

    // Handle the full Stats
    m.drainStats()

    // Terminate
    m.sendOpToServers(Op_Terminate, true)

    defer m.disconnectFromServers()

    return nil
}


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


func (m* Manager) drainStats() {
    m.sendOpToServers(Op_StatDetails, false)

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
                var s Stat
                msg.Data(&s)
                count++

            case Op_StatDetailsDone:
                pending--
                if pending == 0 {
                    fmt.Printf("Received %v detailed stats\n", count)
                    return
                }

            case Op_StatSummary:
                // Ignore this - we just received one a bit later than expected.

            default:
                fmt.Printf("Unexpected opcode: %v\n", op)
                os.Exit(-1)
        }
    }
}


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
                fmt.Printf("%v: %v\n", i, summary.ToString(m.job.Order.ObjectSize))
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


func (m *Manager) connectToServers() error {
    // Construct our aggregated recv channel
    m.msgChannel = make(chan *comms.ReceivedMessageInfo, 1000)

    for _, s := range m.job.Servers {
        endpoint := fmt.Sprintf("%v:%v", s, m.job.ServerPort)
        fmt.Printf("Connecting to sibench server at %v\n", endpoint)

        conn, err := comms.ConnectTCP(endpoint, comms.MakeEncoderFactory(), 0)
        if err == nil {
            conn.ReceiveToChannel(m.msgChannel)
            m.msgConns = append(m.msgConns, conn)
        } else {
            fmt.Printf("Could not connect to sibench server at %v: %v\n", endpoint, err)
            os.Exit(-1)
        }
    }

    return nil
}


func (m *Manager) disconnectFromServers() {
    fmt.Printf("Disconnecting from servers\n")

    for _, c := range m.msgConns {
        c.Close()
    }

    fmt.Print("Disconnected\n")
}


func (m *Manager) createBucket() error {
    o := &(m.job.Order)

    var err error

    // Create a connection
    m.storageConn, err = CreateS3Connection(o.Targets[0], o.Port, o.Credentials)
    if err != nil {
        return err
    }

    // Create bucket
    return m.storageConn.CreateBucket(m.job.Order.Bucket)
}


func (m *Manager) deleteBucket() {
    err := m.storageConn.DeleteBucket(m.job.Order.Bucket)
    if err != nil {
        fmt.Printf("Error deleting bucket: %v\n", err)
    }

    m.storageConn.Close()
}



