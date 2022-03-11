package main


import "encoding/json"
import "fmt"
import "logger"
import "os"
import "strings"



/* 
 * A Report contains all the information about a run.  This includes:
 *
 *    The job object we were executing
 *    The errors encountered
 *    The details from every single operation performed by the system.
 *    An analysis of the results, both as summaries, and broken down by sibench node and
 *    by target node/
 *
 * The report is written as a JSON file.  It is continually added to as we progress 
 * through the phases of a benchmark.
 *
 * We do our best to hold as little data in memory as possible, but it can still end up
 * being pretty large.
 */
type Report struct {
    job *Job
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


/*
 * Create a new Report object.
 *
 * This will also create a new output file for the JSON results, with the filename
 * being set with the --output argumenmt.
 */
func MakeReport(job *Job) (*Report, error) {
    var r Report
    r.job = job

    logger.Infof("Creating report: %s\n", job.arguments.Output)

    r.jsonFile, r.jsonErr = os.Create(job.arguments.Output)
    if r.jsonErr != nil {
        logger.Errorf("Failure creating file: %s, %v\n", job.arguments.Output, r.jsonErr)
    }

    r.writeString("{\n  \"Arguments\": ")
    r.writeJson(job.arguments)
    r.writeString(",\n  \"Stats\": [\n")

    return &r, r.jsonErr
}


/*
 * Closes the File object which we are using to write the JSON, having first added 
 * any last sections to it.
 */
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


/* 
 * Writes an object as JSON using whatever marshalling is default in the encoding/json
 * package.
 *
 * This method will do nothing if we have previously encountered an error.
 */
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
        logger.Errorf("Failure writing to file: %s, %v\n", r.job.arguments.Output, r.jsonErr)
        r.jsonFile.Close()
    }
}


/* 
 * Writes a string into the JSON, so that we can add data to it without the overhead of
 * marshalling.
 *
 * This method will do nothing if we have previously encountered an error.
 */
func (r *Report) writeString(val string) {
    if r.jsonErr != nil {
        return
    }

    _, r.jsonErr = r.jsonFile.WriteString(val)
    if r.jsonErr != nil {
        logger.Errorf("Failure writing to file: %s, %v\n", r.job.arguments.Output, r.jsonErr)
        r.jsonFile.Close()
    }
}


/**
 * Adds a Stat to the report.  It will be written into the JSON immediately.
 * The Stat will be held on to in memory until AnalyseStats is next called.
 */
func (r *Report) AddStat(s *ServerStat) {
    template := `%s    {"Start": %v, "Duration": %v, "Phase": %v, "Error": "%s", "Target": "%s", "Server": "%s"}`

    target := r.job.order.Targets[s.TargetIndex]
    server := r.job.servers[s.ServerIndex]

    val := fmt.Sprintf(
            template,
            r.jsonStatSeparator,
            s.TimeSincePhaseStart.Seconds(),
            s.Duration.Seconds(),
            s.Phase,
            s.Error.ToString(),
            target,
            server)

    r.writeString(val)
    r.jsonStatSeparator = ",\n"
    r.stats = append(r.stats, s)
}


/*
 * Adds an error to the Report.
 */
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
