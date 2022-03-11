package main

/*
 * A job is all the data needed by the Manager to describe a single run.
 *
 * Most of the data is contained within the WorkOrder object (which contains the 
 * data the we pass along to Foremen).
 *
 * Note that we do not hand the Job's WorkOrder directly to the Formen: we make
 * a separate copy for each one that we mututate to divide up the object range.
 *
 * When doing a run, for each phase we ignore the results for the first few 
 * seconds (called the RampUp period) to give the storage backend time to reach a 
 * steady state.
 *
 * Then we have a period (the RunTime) where we do gather the results.
 *
 * Finally, we have a period at the end (the RampDown) where we again discard the 
 * results.
 *
 * Depending on the storage backend, you may need to use different values (they can
 * be specified on the command line).  Typically RampUp is set to around 5-10 seconds,
 * the RunTime can be as long you want to smooth out any bumps in the numbers - say
 * 30 seconds to 10 minutes, and the RampDown is short - perhaps 5 secs maximum.
 */
type Job struct {
    /* The command line arguments with which we were created */
    arguments *Arguments

    /* All the stuff we need to hand out to our Foremen. */
    order WorkOrder

    /* The SiBench servers we should talk to. */
    servers []string    // The sibench servers we will try to use to do the work
    serverPort uint16   // The port we use to connect to those servers.

    /* Duration paramteters (all in seconds) */
    rampUp uint64       // Time given to settle down before we start recording results
    runTime uint64      // The length of the main part of the run where we record results.
    rampDown uint64     // Time at the end of the run where we throw away the results again.
}

