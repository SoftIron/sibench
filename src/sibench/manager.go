// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import (
	"comms"
	"fmt"
	"io"
	"logger"
	"os"
    "os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/exp/slices"
)


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
    report *Report
    msgConns []*comms.MessageConnection
    msgChannel chan *comms.ReceivedMessageInfo
    connToServerDetails map[*comms.MessageConnection]*ServerDetails
    totalCoreCount uint64
    sigChan chan os.Signal
    isInterrupted bool

    /* Most operations will be skipped after the first time we encounter an error */
    err error
}


/* Runs a single benchmark */
func RunBenchmark(j *Job) error {
    var m Manager;
    m.job = j
    m.report, m.err = MakeReport(j)

    // Pull out the order, just to make the code more clear.
    o := &(j.order)

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

    defer conn.ManagerClose(j.order.CleanUpOnClose)

    m.connectToServers()
    defer m.disconnectFromServers()

    m.discoverServerCapabilities()
    m.sendJobToServers()

    // Register for interrupts before we do the actual work
    m.sigChan = make(chan os.Signal, 1)
    signal.Notify(m.sigChan, syscall.SIGINT, syscall.SIGTERM)

    phaseTime := j.runTime + j.rampUp + j.rampDown

    if j.order.ReadWriteMix == 0 {
        // Write/Prepare/Read
        m.runPhaseForTime("WRITE", phaseTime, OP_WriteStart, OP_WriteStop)
        m.runPhaseToCompletion("PREPARE", OP_Prepare)
        m.runPhaseForTime("READ", phaseTime, OP_ReadStart, OP_ReadStop)
    } else {
        // Prepare/Read-Write-Mix
        m.runPhaseToCompletion("PREPARE", OP_Prepare)
        m.runPhaseForTime("READ/WRITE", phaseTime, OP_ReadWriteStart, OP_ReadWriteStop)
    }

    if (conn.CanDelete() && j.order.CleanUpOnClose) {
        m.runPhaseToCompletion("DELETE", OP_Delete)
    }

    // Process the stats.
    if m.err == nil {
        logger.Infof("\n")
        m.report.DisplayAnalyses(m.job.useBytes)
    }

    // Terminate
    logger.Infof("\n")
    m.terminate()

    if m.err != nil {
        m.report.AddError(m.err)
        logger.Errorf("%v\n", m.err)
    }

    m.report.Close()
    return m.err
}


/*
 * Returns a string containing a banner with a centred message.
 */
func banner(msg string, padChar byte) string {
    msgLen := len(msg)
    padStr := string(padChar)
    prepadLen := (78 - msgLen) / 2
    postpadLen := (79 - msgLen) / 2

    return "\n" + strings.Repeat(padStr, prepadLen) + " " + msg + " " + strings.Repeat(padStr, postpadLen) + "\n"
}


/**
 * Runs a script, if we have one, at key points in the run.
 */
func (m *Manager) runScript(phase string, event string) {
    if m.job.script == "" {
        return
    }

    logger.Debugf("Running phase script: '%s %s %s'\n", m.job.script, phase, event)

    cmd := exec.Command(m.job.script, phase, event)
    err := cmd.Run()

    if err != nil {
        logger.Errorf("Failure running phase script: '%s %s %ws' - %v\n", m.job.script, phase, event, err)
    }
}


/*
 * Sends an operation request to the servers.
 * If waitForResponse is true, then we block until all the servers have responded.
 */
func (m *Manager) sendOpToServers(op Opcode, waitForResponse bool) {
    if op != OP_Terminate {
        if m.err != nil { return }
        if m.isInterrupted { return }
    }

    logger.Debugf("Sending: %v\n", op.ToString())

    // Send our request.
    for _, conn := range m.msgConns {
        conn.Send(uint8(op), nil)
    }

    if waitForResponse {
        m.waitForResponses(op)
    }
}


/*
 * Check if an incoming message is an error type, and convert it to error if so.
 */
func (m *Manager) checkError(msgInfo *comms.ReceivedMessageInfo) {
    msg := msgInfo.Message
    op := Opcode(msg.ID())

    // If it's not a failure, then we're done.
    if (op != OP_Fail) && (op != OP_Hung) { return }

    var resp ForemanGenericResponse
    msg.Data(&resp)

    details := m.connToServerDetails[msgInfo.Connection]
    err := fmt.Errorf("%v:%v", details.Name, resp.Error)

    // First check if this is a hung server, in which case we drop all knowledge of it
    // so that we don't try to shut it down cleanly later.
    if op == OP_Hung {
        logger.Infof("Server %v has hung\n", m.connToServerDetails[msgInfo.Connection].Name)

        index := slices.Index(m.msgConns, msgInfo.Connection)
        if index >= 0 {
            m.msgConns = slices.Delete(m.msgConns, index, index + 1)
            delete(m.connToServerDetails, msgInfo.Connection)
        }
    }

    // If we've already seen an error, then we don't need to record this one.
    if (m.err != nil) || m.isInterrupted { return }

    m.err = err
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
    if (m.err != nil) || m.isInterrupted { return }

    logger.Infof("Retrieving stats from servers\n")

    m.sendOpToServers(OP_StatDetails, false)

    count := 0
    pending := len(m.msgConns)
    start := time.Now()

    for pending > 0 {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    m.err = fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
                    return
                }

                m.checkError(msgInfo)
                if m.err != nil { return }

                msg := msgInfo.Message
                op := Opcode(msg.ID())

                // We can ignore anything except StatDetail

                switch op {
                    case OP_StatDetails:
                        var stats []Stat
                        msg.Data(&stats)
                        details := m.connToServerDetails[msgInfo.Connection]

                        for _, s := range(stats) {
                            ss := new(ServerStat)
                            ss.ServerIndex = details.Index
                            ss.Stat = s

                            m.report.AddStat(ss)
                            count++
                        }

                    case OP_StatDetailsDone:
                        pending--

                    case OP_StatSummary:
                        // Ignore this - we just received one a bit later than expected.

                    default:
                        m.err = fmt.Errorf("Unexpected opcode: %v\n", op.ToString())
                        return
                }

            case <-m.sigChan:
                logger.Infof("Interrupting stats collection and waiting to shut down\n")
                m.isInterrupted = true
                return
        }
    }

    end := time.Now()
    logger.Infof("%v stats retrieved in %.3f seconds\n", len(m.report.stats), end.Sub(start).Seconds())

    start = time.Now()
    m.report.AnalyseStats()
    end = time.Now()
    logger.Infof("Stats merged and analysed in %.3f seconds\n", end.Sub(start).Seconds())

    return
}


/*
 * Works very much like runPhaseForTime, but this time we wait for the servers to tell us the're done,
 * rather the running for a specifed length of time.
 *
 * This is used for the Prepare and CleanUp phases.
 */
func (m *Manager) runPhaseToCompletion(msg string, phaseOp Opcode) {
    if (m.err != nil) || m.isInterrupted { return }

    logger.Infof(banner(msg, '-'))

    m.sendOpToServers(OP_StatSummaryStart, true)
    m.sendOpToServers(phaseOp, false)

    ticker := time.NewTicker(time.Second)

    var summary StatSummary
    pending := len(m.msgConns)
    i := 0

    for {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    if msgInfo.Error == io.EOF {
                        m.err = fmt.Errorf("Received remote close from %v\n", msgInfo.Connection.RemoteIP())
                        return
                    }

                    m.err = fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
                    return
                }

                msg := msgInfo.Message
                m.checkError(msgInfo)
                if m.err != nil { return }

                op := Opcode(msg.ID())
                switch op {
                    case phaseOp:
                        pending--
                        if pending == 0 {
                            m.sendOpToServers(OP_StatSummaryStop, true)
                            m.drainStats()
                            return
                        }

                    case OP_StatSummary:
                        var s StatSummary
                        msg.Data(&s)
                        summary.Add(&s)

                    default:
                        m.err = fmt.Errorf("Unexpected opcode %v\n", op.ToString())
                        return
                }

            case <-ticker.C:
                logger.Infof("%v: %v\n", i, summary.String(m.job.order.ObjectSize, m.job.useBytes))
                i++
                summary.Zero()

            case <-m.sigChan:
                logger.Infof("Interrupting job and waiting to shut down\n")
                ticker.Stop()
                m.isInterrupted = true
                return
        }
    }
}



/*
 * Waits for the specified number of seconds whilst a benchmark executes.
 *
 * During this time, we accept StatSummary messages from the servers.
 * These are aggragated, and printed out once per second so that the user can
 * see what the system is doing.
 *
 * This is used for Read, Write and Read/Write phases.
 */
func (m *Manager) runPhaseForTime(msg string, secs uint64, startOp Opcode, stopOp Opcode) {
    if (m.err != nil) || m.isInterrupted { return }

    logger.Infof(banner(msg, '-'))

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
                        m.err = fmt.Errorf("Received remote close from %v\n", msgInfo.Connection.RemoteIP())
                        return
                    }

                    m.err = fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
                    return
                }

                msg := msgInfo.Message
                m.checkError(msgInfo)
                if m.err != nil { return }

                op := Opcode(msg.ID())
                if op != OP_StatSummary {
                    m.err = fmt.Errorf("Unexpected opcode %v\n", op.ToString())
                    return
                }

                var s StatSummary
                msg.Data(&s)
                summary.Add(&s)

            case <-ticker.C:
                logger.Infof("%v: %v\n", i, summary.String(m.job.order.ObjectSize, m.job.useBytes))
                i++

                isRampUp := (uint64(i) == m.job.rampUp)
                isRampDown := (uint64(i) == m.job.rampUp + m.job.runTime)

                if isRampUp || isRampDown {
                    // Draw some lines to indicate the ramp-up/ramp-down demarcation.
                    logger.Infof("-----------------------------------------------------------\n")

                    // Run the script (if we have one) with suitable args.
                    if isRampUp {
                        go m.runScript(msg, "UP")
                    } else {
                        go m.runScript(msg, "DOWN")
                    }
                }

                summary.Zero()

            case <-timer.C:
                ticker.Stop()
                m.sendOpToServers(OP_StatSummaryStop, true)
                logger.Infof("Waiting for all workers to complete their current operation\n");
                m.sendOpToServers(stopOp, true)
                m.drainStats()
                return

            case <-m.sigChan:
                logger.Infof("Interrupting job and waiting to shut down\n")
                ticker.Stop()
                m.isInterrupted = true
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
    if (m.err != nil) || m.isInterrupted { return }

    logger.Debugf("Waiting for %s\n", expectedOp.ToString())
    pending := len(m.msgConns)

    for {
        select {
            case msgInfo := <-m.msgChannel:
                if msgInfo.Error != nil {
                    logger.Errorf("%v\n", msgInfo.Error)
                    os.Exit(-1)
                }

                m.checkError(msgInfo)
                if m.err != nil { return }

                msg := msgInfo.Message
                op := Opcode(msg.ID())

                if op == expectedOp {
                    var resp ForemanGenericResponse
                    msg.Data(&resp)

                    pending--
                    if pending == 0 {
                        logger.Debugf("Received %v, finished waiting\n", op.ToString())
                        return
                    }

                    logger.Debugf("Received %v, still waiting for %v more\n", op.ToString(), pending)
                } else if op != OP_StatSummary {
                    // Stat Summary messages can arrive later than expected because they're asynchronous.
                    // If we see one when we don't want one, we just drop it.
                    // All other unexpected opcodes are an error.
                    m.err = fmt.Errorf("Unexpected Opcode received: expected %v but got %v\n", expectedOp.ToString(), op.ToString())
                    return
                }

            case <-m.sigChan:
                logger.Infof("Interrupting job and waiting to shut down\n")
                m.isInterrupted = true
                return
        }
    }
}


func (m *Manager) terminate() {
    logger.Infof("Terminating\n")

    m.sendOpToServers(OP_Terminate, false)

    // We don't do our usual wait-for-response thing here because we may have done this from
    // an interrupt, and so there could be spurious incoming message that we have to ignore.

    for pending := len(m.msgConns); pending > 0; {
        msgInfo := <-m.msgChannel

        switch msgInfo.Error {
            case nil:
                if Opcode(msgInfo.Message.ID()) == OP_Terminate {
                     pending--
                }

            case io.EOF:
                // Ignore: the foreman has just closed the connection.

            default:
                m.err = fmt.Errorf("Transport failure: %v\n", msgInfo.Error)
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
func (m *Manager) sendJobToServers() {
    if (m.err != nil) || m.isInterrupted { return }

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

    m.waitForResponses(OP_Connect)
}


/*
 * Interogates each sibench server for information about core count, RAM size and
 * so forth, so that we can allocate the workloads appropriately later.
 */
func (m *Manager) discoverServerCapabilities() {
    if (m.err != nil) || m.isInterrupted { return }

    logger.Debugf("Sending Server Capability Discovery requests\n")
    for _, conn := range m.msgConns {
        conn.Send(OP_Discovery, nil)
    }

    if m.err != nil { return }
    m.totalCoreCount = 0

    logger.Infof("\n---------- Sibench driver capabilities discovery ----------\n")
    pending := len(m.msgConns)

    for pending > 0 {
        msgInfo := <-m.msgChannel

        if msgInfo.Error != nil {
            m.err = fmt.Errorf("Failure in driver discovery: %v\n", msgInfo.Error)
            return
        }

        msg := msgInfo.Message

        op := Opcode(msg.ID())
        if op != OP_Discovery {
            m.err = fmt.Errorf("Unexpected Opcode received: expected Discovery but got %v\n", op.ToString())
            return
        }

        d := m.connToServerDetails[msgInfo.Connection]
        msg.Data(&d.Discovery)

        // Find our details object

        logger.Infof("%s: %v cores, %vB of RAM, sibench build %s\n", d.Name, d.Cores, ToUnits(d.Ram), d.Version)
        m.totalCoreCount += d.Cores

        pending--
    }

    logger.Debugf("Discovery complete\n\n")
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
func (m *Manager) connectToServers() {
    if (m.err != nil) || m.isInterrupted { return }

    // Construct our aggregated recv channel
    m.msgChannel = make(chan *comms.ReceivedMessageInfo, 1000)
    m.connToServerDetails = make(map[*comms.MessageConnection]*ServerDetails)

    for i, s := range m.job.servers {
        endpoint := fmt.Sprintf("%v:%v", s, m.job.serverPort)
        logger.Infof("Connecting to sibench server at %v\n", endpoint)

        conn, err := comms.ConnectTCP(endpoint, comms.MakeEncoderFactory(), 0)
        if err != nil {
            m.err = fmt.Errorf("Could not connect to sibench server at %v: %v\n", endpoint, err)
            return
        }

        conn.ReceiveToChannel(m.msgChannel)
        m.msgConns = append(m.msgConns, conn)

        details := new(ServerDetails)
        details.Name = s
        details.Index = uint16(i)

        m.connToServerDetails[conn] = details
    }
}


/* Disconnects from all the Foremen that we are successfully connected to. */
func (m *Manager) disconnectFromServers() {
    logger.Infof("Disconnecting from servers\n")

    for _, c := range m.msgConns {
        c.Close()
    }

    logger.Infof("Disconnected\n")
}

