package main


import "encoding/json"
import "fmt"
import "logger"
import "os"
import "strings"



/* 
 * A Report contains all the information we return from a run.
 *
 * It also holds a file descriptor that it uses to write out the details as JSON
 * as they are added (so that we don't have to hold ALL the stats in memory until the 
 * end).
 */
type Report struct {
    job *Job
    arguments *Arguments
    analyses []*Analysis
    errors []error
    stats []*ServerStat

    /* The file handle we use to write out a JSON version of the report. */
    jsonFile *os.File

    /* Error we maintain to avoid huge amounts of error checking everywhere */
    jsonErr error

    /* Whether or not our next stat object needs a comma. */
    jsonStatSeparator string
}


func MakeReport(job *Job, args *Arguments) (*Report, error) {
    var r Report

    r.job = job
    r.arguments = args

    logger.Infof("Creating report: %s\n", args.Output)

    r.jsonFile, r.jsonErr = os.Create(args.Output)
    if r.jsonErr != nil {
        logger.Errorf("Failure creating file: %s, %v\n", args.Output, r.jsonErr)
    }

    r.writeString("{\n  \"Arguments\": ")
    r.writeJson(args)
    r.writeString(",\n  \"Stats\": [\n")

    return &r, r.jsonErr
}


func (r *Report) Close() {
    if r.jsonErr != nil {
        return
    }

    r.writeString("\n  ],\n  \"Errors\": ")
    r.writeJson(r.errors)
    r.writeString(",\n  \"Analyses\": ")
    r.writeJson(r.analyses)
    r.writeString("\n}")

    r.jsonFile.Close()
}


func (r *Report) writeJson(val interface{}) {
    if r.jsonErr != nil {
        return
    }

    jsonVal, err := json.MarshalIndent(val, "  ", "  ")
    if err != nil {
        logger.Errorf("Failure marshalling arguments to json: %v\n", err)
        r.jsonFile.Close()
        r.jsonErr = err
        return
    }

    _, r.jsonErr = r.jsonFile.Write(jsonVal)
    if r.jsonErr != nil {
        logger.Errorf("Failure writing to file: %s, %v\n", r.arguments.Output, r.jsonErr)
        r.jsonFile.Close()
    }
}


func (r *Report) writeString(val string) {
    if r.jsonErr != nil {
        return
    }

    _, r.jsonErr = r.jsonFile.WriteString(val)
    if r.jsonErr != nil {
        logger.Errorf("Failure writing to file: %s, %v\n", r.arguments.Output, r.jsonErr)
        r.jsonFile.Close()
    }
}


func (r *Report) AddStat(s *ServerStat) {
    template := `%s    {"Start": %v, "Duration": %v, "Phase": %v, "Error": "%s", "Target": "%s", "Server": "%s"}`

    target := r.job.order.Targets[s.TargetIndex]
    server := r.job.servers[s.ServerIndex]

    val := fmt.Sprintf(template, r.jsonStatSeparator, s.TimeSincePhaseStart.Seconds(), s.Duration.Seconds(), s.Phase, s.Error.ToString(), target, server)
    r.writeString(val)

    r.jsonStatSeparator = ",\n"

    r.stats = append(r.stats, s)
}


func (r *Report) AddError(e error) {
    r.errors = append(r.errors, e)
}


/*
 * Do the maths on all the stats we are currently holding, in order to generate
 * some number of Analysis objects for the report.
 *
 * This also also us to clear out the stats we have been holding in order 
 * to save memory, as the Analyses that we have created have everything that we 
 * are still interested in keeping.
 */
func (r *Report) AnalyseStats() {
    // Start off by throwing out anything in a ramp period.
    stats := filter(r.stats, rampFilter(r.job))

    phases := []StatPhase{ SP_Write, SP_Read }

    // Produce per-target and per-server analyses for each phase
    for _, phase := range phases {

        pstats := filter(stats, phaseFilter(phase))
        if len(pstats) > 0 {
            for tIndex, t := range r.job.order.Targets {
                tstats := filter(pstats, targetFilter(uint16(tIndex)))
                a := NewAnalysis(tstats, "Target[" + limit(t, 12) + "] " + phase.ToString(), phase, false, r.job)
                r.analyses = append(r.analyses, a)
            }

            for sIndex, s := range r.job.servers {
                sstats := filter(pstats, serverFilter(uint16(sIndex)))
                a := NewAnalysis(sstats, "Server[" + limit(s, 12) + "] " + phase.ToString(), phase, false, r.job)
                r.analyses = append(r.analyses, a)
            }
        }
    }

    // End up with the most imporant stats - the overall performance for each phase.
    for _, phase := range phases {
        pstats := filter(stats, phaseFilter(phase))
        if len(pstats) > 0 {
            a := NewAnalysis(pstats, "Total " + phase.ToString(), phase, true, r.job)
            r.analyses = append(r.analyses, a)
        }
    }

    r.stats = nil
}


/*
 * Prints the analyses to stdout with some nice formatting.
 */
func (r *Report) DisplayAnalyses() {
    lineWidth := 160
    lastPhase := SP_Len // Choosing a value that will not be a real phase.

    // First print out the target and server analyses

    for _, a := range r.analyses {
        if !a.IsTotal {
            if a.Phase != lastPhase {
                lastPhase = a.Phase
                fmt.Printf("%v\n", strings.Repeat("-", lineWidth))
            }

            fmt.Printf("%v\n", a.String())
        }
    }

    // Now print the grand totals

    fmt.Printf("%v\n", strings.Repeat("=", lineWidth))

    for _, a := range r.analyses {
        if a.IsTotal {
            fmt.Printf("%v\n", a.String())
        }
    }

    fmt.Printf("%v\n", strings.Repeat("=", lineWidth))
}





