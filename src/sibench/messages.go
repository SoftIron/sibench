/* 
 * This file defines all the TCP messages that can be send between the manager and its foremen.
 * Some of the types here are also used to communicate between a foreman and its workers.
 */

package main

import "time"


/* 
 * Opcodes used as the TCP Message type identifier for messages between the manager and its
 * Foremen.
 * Also used directly (without TCP) between a Foreman and its Workers.
 */
type Opcode string
const(
    // Opcodes only used between Foreman->Manager
    Op_StatSummary = "StatSummary"
    Op_Busy = "Busy"
    Op_Failed = "Failed"

    // Opcodes used between Foreman<->Manager
    Op_StatDetails = "StatDetails"
    Op_StatDetailsDone = "StatDetailsDone"

    // Opcodes used bewtween Manager<->Foreman and between Foreman<->Worker
    Op_Connect = "Connect"
    Op_WriteStart = "WriteStart"
    Op_WriteStop = "WriteStop"
    Op_Prepare = "Prepare"
    Op_ReadStart = "ReadStart"
    Op_ReadStop = "ReadStop"
    Op_Terminate = "Terminate"
)



/* 
 * Standard response type for all TCP messages from the Foreman to the Manager that don't need special 
 * data (such as Stats).  It is combined with an Opcode to identify which message this is a response too. 
 */
type ForemanGenericResponse struct {
    Hostname string
    Error error
}



type StatPhase uint8
const (
    SP_Write StatPhase = iota
    SP_Prepare
    SP_Read
    SP_Len // Not a phase, but a count of how many phases we have
)


func (sp StatPhase) ToString() string {
    switch sp {
        case SP_Write:    return "Write"
        case SP_Prepare:  return "Prepare"
        case SP_Read:     return "Read"
        default:          return "Unkown"
    }
}


type StatError uint8
const (
    SE_None = iota
    SE_VerifyFailure
    SE_OperationFailure
    SE_Len // Not an error code, but a count of how many error codes we have
)


/*
 * A summary of the stats that we send periodically when doing a phase
 */
type StatSummary [SP_Len][SE_Len] uint64


/*
 * The fully-detailed stats that we send on Job completion.
 */
type Stat struct {
    TimeSincePhaseStart time.Duration
    Duration time.Duration
    Phase StatPhase
    Error StatError
    Server string
    Target string
}


/* 
 * A WorkOrder contains everything that the foremen needs to do their part of a Job.
 * It is sent as the data for the Connect message.
 */
type WorkOrder struct {
    JobId uint64                    // Which job this WorkOrder is part of
    ServerName string               // The name we wish the server processing the order to use in stats

    // Object parameters
    Bucket string                   // The storage bucket into which we will write
    ObjectSize uint64               // The size of the objects we read and write
    Seed uint64                     // A seed for any PRNGs in use. 
    GeneratorType string            // Which type of Generator we will use to create and verify object data.
    RangeStart uint64               // Start of the object range to be used.
    RangeEnd uint64                 // End of the object range, not inclusive.

    // Connection parameters
    ConnectionType string           // The type of connection: s3, librados etc... 
    Targets []string                // The set of gateways, monitors, metadata servers or whatever we connect to. 
    Port uint16                     // The port on which we connect to the storage servers.
    Credentials map[string]string   // ConnectionType-specific key/value pairs for credential info for connecting.
}

