// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

/* 
 * This file defines all the TCP messages that can be sent between the manager and its foremen.
 * Some of the types here are also used to communicate between a foreman and its workers.
 */

package main


/* 
 * Opcodes used as the TCP Message type identifier for messages between the manager and its
 * Foremen.
 * Also used directly (without TCP) between a Foreman and its Workers.
 */
type Opcode uint8
const(
    // Never sent, but used as a nil value
    OP_None = iota

    // Opcodes used between Worker->Foreman and Foreman->Manager.
    OP_Fail
    OP_Hung

    // Opcodes only used between Foreman->Manager
    OP_StatSummary
    OP_Busy

    // Opcodes used between Foreman<->Manager
    OP_Discovery
    OP_StatDetails
    OP_StatDetailsDone
    OP_StatSummaryStart
    OP_StatSummaryStop

    // Opcodes used bewtween Manager<->Foreman and between Foreman<->Worker
    OP_Connect
    OP_WriteStart
    OP_WriteStop
    OP_Prepare
    OP_ReadStart
    OP_ReadStop
    OP_ReadWriteStart
    OP_ReadWriteStop
    OP_Delete
    OP_Terminate
)


func (op Opcode) ToString() string {
    switch op {
        case OP_None: return "None"
        case OP_Fail: return "Fail"
        case OP_Hung: return "Hung"
        case OP_StatSummary: return "StatSummary"
        case OP_Busy: return "Busy"
        case OP_Discovery: return "Discovery"
        case OP_StatDetails: return "StatDetails"
        case OP_StatDetailsDone: return "StatDetailsDone"
        case OP_StatSummaryStart: return "StatSummaryStart"
        case OP_StatSummaryStop: return "StatSummaryStop"
        case OP_Connect: return "Connect"
        case OP_WriteStart: return "WriteStart"
        case OP_WriteStop: return "WriteStop"
        case OP_Prepare: return "Prepare"
        case OP_ReadStart: return "ReadStart"
        case OP_ReadStop: return "ReadStop"
        case OP_ReadWriteStart: return "ReadWriteStart"
        case OP_ReadWriteStop: return "ReadWriteStop"
        case OP_Delete: return "Delete"
        case OP_Terminate: return "Terminate"
        default: return "Unknown"
    }
}


/* 
 * Standard response type for all TCP messages from the Foreman to the Manager that don't need special 
 * data (such as Stats).  
 */
type ForemanGenericResponse struct {
    Error string
}



/*
 * Enum of the different phases of a benchmark.
 */
type StatPhase uint8
const (
    SP_Write StatPhase = iota
    SP_Prepare
    SP_Read
    SP_Delete
    SP_Len // Not a phase, but a count of how many phases we have
)


func (sp StatPhase) ToString() string {
    switch sp {
        case SP_Write:    return "Write"
        case SP_Prepare:  return "Prepare"
        case SP_Read:     return "Read"
        case SP_Delete:   return "Delete"
        default:          return "Unknown"
    }
}


/* An enum of the types of errors we count for stats purposes. */
type StatError uint8
const (
    SE_None = iota
    SE_VerifyFailure    // When we read back data and get unexpected content
    SE_OperationFailure // When we hit a non-fatal error reading or writing
    SE_Len              // Not an error code, but a count of how many error codes we have
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
    Phase StatPhase
    Error StatError
    TargetIndex uint16
    TimeSincePhaseStartMillis uint32
    DurationMicros uint32
}


/*
 * A Foreman's response to a discovery request
 */
type Discovery struct {
    Cores uint64
    Ram uint64
    Version string
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
    ObjectKeyPrefix string          // A random prefix to be used for object keys to ensure uniqueness across runs
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
    CleanUpOnClose bool             // Whether we should clean up at the end of the job.
}

