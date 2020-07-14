package main

type Job struct {
    // All the stuff we need to hand out.
    Order WorkOrder

    // The SiBench servers we should talk to.
    Servers []string
    ServerPort uint16

    // Duration paramteters (all in seconds)
    RunTime uint64
    RampUp uint64
    RampDown uint64
}


