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
    // Opcodes used between Worker->Foreman and Foreman->Manager.
    OP_Fail = "Fail"
    OP_Hung = "Hung"

    // Opcodes only used between Foreman->Manager
    OP_StatSummary = "StatSummary"
    OP_Busy = "Busy"

    // Opcodes used between Foreman<->Manager
    OP_Discovery = "Discovery"
    OP_StatDetails = "StatDetails"
    OP_StatDetailsDone = "StatDetailsDone"
    OP_StatSummaryStart = "StatSummaryStart"
    OP_StatSummaryStop = "StatSummaryStop"

    // Opcodes used bewtween Manager<->Foreman and between Foreman<->Worker
    OP_Connect = "Connect"
    OP_WriteStart = "WriteStart"
    OP_WriteStop = "WriteStop"
    OP_Prepare = "Prepare"
    OP_ReadStart = "ReadStart"
    OP_ReadStop = "ReadStop"
    OP_ReadWriteStart = "ReadWriteStart"
    OP_ReadWriteStop = "ReadWriteStop"
    OP_Terminate = "Terminate"
)


/* 
 * Standard response type for all TCP messages from the Foreman to the Manager that don't need special 
 * data (such as Stats).  
 */
type ForemanGenericResponse struct {
    Error string
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


/* An enum of the types of errors we count for stats purposes. */
type StatError uint8
const (
    SE_None = iota
    SE_VerifyFailure
    SE_OperationFailure
    SE_Len // Not an error code, but a count of how many error codes we have
)


func (se StatError) ToString() string {
    switch se {
        case SE_None:               return "None"
        case SE_VerifyFailure:      return "Verify"
        case SE_OperationFailure:   return "Operation"
        default:                    return "Unknown"
    }
}


/*
 * A summary of the stats that we send periodically when doing a phase
 */
type StatSummary [SP_Len][SE_Len] uint64


/*
 * The fully-detailed stats that we send on Job completion.
 * Each stat describes a single operation (such as a single object read or write).
 */
type Stat struct {
    TimeSincePhaseStart time.Duration
    Duration time.Duration
    Phase StatPhase
    Error StatError
    TargetIndex uint16
}


/*
 * A Foreman's response to a discovery request
 */
type Discovery struct {
    Cores uint64
    Ram uint64
}


type ProtocolConfig map[string]string
type GeneratorConfig map[string]string

/* 
 * A WorkOrder contains everything that the foremen needs to do their part of a Job.
 * It is sent as the data for the Connect message.
 */
type WorkOrder struct {
    JobId uint64                    // Which job this WorkOrder is part of
    Bandwidth uint64                // Bytes/s limit, or zero for no limit.
    WorkerFactor float64            // Number of workers to create for each core on a server.
    SkipReadValidation bool         // Whether to skip the validation step when we read objects.
    ReadWriteMix uint64             // Give the percentage of reads vs writes for combined ops. 

    // Object parameters
    ObjectSize uint64               // The size of the objects we read and write
    Seed uint64                     // A seed for any PRNGs in use. 
    GeneratorType string            // Which type of Generator we will use to create and verify object data.
    RangeStart uint64               // Start of the object range to be used.
    RangeEnd uint64                 // End of the object range, not inclusive.

    // Connection parameters
    ConnectionType string           // The type of connection: s3, librados etc... 
    Targets []string                // The set of gateways, monitors, metadata servers or whatever we connect to. 
    ProtocolConfig ProtocolConfig   // Protocol-specific key/value pairs for credential info for making new connection.
    GeneratorConfig GeneratorConfig // Generator-specific key/value pairs.
}

