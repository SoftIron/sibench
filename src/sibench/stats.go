package main

import "fmt"
import "sort"
import "strings"


/* Reset all off the summary's counters to 0. */
func (s *StatSummary) Zero() {
    for phase := 0; phase < int(SP_Len); phase++ {
        for err :=0; err < int(SE_Len); err++ {
            s[phase][err] = 0
        }
    }
}


/* Add one summary to another. */
func (s *StatSummary) Add(other *StatSummary) {
    for phase := 0; phase < int(SP_Len); phase++ {
        for err :=0; err < int(SE_Len); err++ {
            s[phase][err] += other[phase][err]
        }
    }
}


/* Helper to convert values into to K, G, M etc. units */
func toUnits(val uint64) string {
    const unit = 1024

    if val < unit {
        return fmt.Sprintf("%d", val)
    }

    div, exp := uint64(unit), 0

    for n := val / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }

    return fmt.Sprintf("%.1f %c", float64(val) / float64(div), "KMGTPE"[exp])
}


/* Produce a human readable string from a StatSummary object */
func (s *StatSummary) String(objectSize uint64) string {
    result := ""

    for i := StatPhase(0); i < SP_Len; i++ {
        data := false
        for j := StatError(0); j < SE_Len; j++ {
            if s[i][j] > 0 {
                data = true
            }
        }

        if data {
            phase := i.ToString()
            ops := s[i][SE_None]
            ofail := s[i][SE_OperationFailure]
            vfail := s[i][SE_VerifyFailure]
            bw := toUnits(ops * objectSize * 8)

            result += fmt.Sprintf("[%v] ops: %v,  bw: %vb/s,  ofail: %v,  vfail: %v ", phase, ops, bw, ofail, vfail)
        }
    }

    return result
}


/*
 * We will use a filter function to include/exclude stats from a slice
 * We could use anonymous functions to do the exact same thing with less code, but
 * by giving explicit names to things we can make it a lot more readable. 
 */
type filterFunc func(*Stat) bool


/* Filter based on phase */
func phaseFilter(phase StatPhase) filterFunc {
    return func(s *Stat) bool {
        return s.Phase == phase
    }
}


/* Filter based on error type */
func errorFilter(err StatError) filterFunc {
    return func(s *Stat) bool {
        return s.Error == err
    }
}


/* Filter out stats that are not in the relevant time period */
func rampFilter(job *Job) filterFunc {

    // Durations are in ns, so convert our values from seconds
    up := job.RampUp * 1000 * 1000 * 1000
    time := job.RunTime * 1000 * 1000 * 1000

    return func(s *Stat) bool {
        start := uint64(s.TimeSincePhaseStart)
        return (start > up) && (start <= up + time)
    }
}


/* Filter on target */
func targetFilter(target string) filterFunc {
    return func(s *Stat) bool {
        return s.Target == target
    }
}


/* Filter on server */
func serverFilter(server string) filterFunc {
    return func(s *Stat) bool {
        return s.Server == server
    }
}


/* Inverts the sense of a filter function */
func invertFilter(fn filterFunc) filterFunc{
    return func(s *Stat) bool {
        return !fn(s)
    }
}


/* Apply filters to a slice of stats, returning a new slice that contains only the matching values */
func filter(stats []*Stat, fns ...filterFunc) []*Stat {
    var results []*Stat

    for _, s:= range stats {
        include := true
        for i := 0; (i < len(fns)) && include; i++ {
            include = include && fns[i](s)
        }

        if include {
            results = append(results, s)
        }
    }

    return results
}


/* Sort a slice of stats to fastest first, slowest last. */
func sortByDuration(stats []*Stat) {
    sort.Slice(stats, func(i, j int) bool {
        return stats[i].Duration < stats[j].Duration
    })
}


/*
 * An Analysis object holds all the statistics we have computed on some particular set of Stats objects.  
 *
 * There may be a quite a few different Analyses, on different subsets of our overall pool of Stats.
 * For instance, we might have one Analsysis of the read performance of just one of our targets, and
 * another describing the write performance of the system as a whole.
 *
 * Each Analysis is named.
 */
type Analysis struct {
    Name string

    /* All response times in ms */
    ResTimeMin uint64   // The fastest reponse we had for a successful operation
    ResTimeMax uint64   // The slowest response we had for a successful operation
    ResTime95  uint64   // The reponse time by which 95% of our successful operations completed

    /* Bandwidth is in bits per seconds */
    Bandwidth uint64

    /* Counts */
    Successes uint64
    Failures uint64
}


/*
 * Produce a human-readable string from an Analysis.
 * This is intended to be used to dump tables of Analyses, and aligns fields nicely for that purpose.
 */
func (a *Analysis) String() string {
    return fmt.Sprintf("%-28v   bandwidth: %7vb/s,  successes: %6v,  failures: %6v,  res-min: %5v ms,  res-max: %5v ms,  res-95: %6v ms",
        a.Name,
        toUnits(a.Bandwidth),
        a.Successes,
        a.Failures,
        a.ResTimeMin,
        a.ResTimeMax,
        a.ResTime95)
}


/* 
 * Create an Analysis object describing a slice of stats.
 * We pass in the name that we wish to give the Analysis.
 * The job is needed so that we can pul run times and object size from it.
 */
func CreateAnalysis(stats []*Stat, name string, job *Job) *Analysis {
    var result Analysis
    result.Name =name

    good := filter(stats, errorFilter(SE_None))
    result.Successes = uint64(len(good))
    result.Failures = uint64(len(stats) - len(good))

    if len(good) > 0 {
        sortByDuration(good)

        // Would like to use Duration.Milliseconds, but it doesn't exist in our go version.
        result.ResTimeMin = uint64(good[0].Duration) / (1000 * 1000)
        result.ResTimeMax = uint64(good[len(good) - 1].Duration) / (1000 * 1000)
        result.ResTime95  = uint64(good[int(float64(len(good)) * 0.95)].Duration) / (1000 * 1000)
        result.Bandwidth  = uint64(8 * len(good)) * job.Order.ObjectSize / job.RunTime
    }

    return &result
}


/* Process each phase that we are interested in, one by one */
func crunchPhases(stats []*Stat, job *Job, prefix string) {
    phases := []StatPhase{ SP_Write, SP_Read }
    for _, phase := range phases {
        pstats := filter(stats, phaseFilter(phase))
        fmt.Printf("%v\n", CreateAnalysis(pstats, prefix + " " + phase.ToString(), job).String())
    }
}



/*
 * Do the maths for a slice full of detailed stats.
 * We return a slice of various different Analyses that we create.
 * As a side-effect, we also currently print the Analyses to the console.
 */
func CrunchTheNumbers(stats []*Stat, job *Job) []*Analysis {
    var results []*Analysis

    // Start off by throwing out anything in a ramp period.
    stats = filter(stats, rampFilter(job))

    phases := []StatPhase{ SP_Write, SP_Read }
    lineWidth := 151

    // Produce per-target and per-server analyses
    for _, phase := range phases {
        fmt.Printf("%v\n", strings.Repeat("-", lineWidth))
        pstats := filter(stats, phaseFilter(phase))

        for _, t := range job.Order.Targets {
            tstats := filter(pstats, targetFilter(t))
            a := CreateAnalysis(tstats, "Target[" + t + "] " + phase.ToString(), job)
            results = append(results, a)
            fmt.Printf("%v\n", a.String())
        }

        for _, s := range job.Servers {
            sstats := filter(pstats, serverFilter(s))
            a := CreateAnalysis(sstats, "Server[" + s + "] " + phase.ToString(), job)
            results = append(results, a)
            fmt.Printf("%v\n", a.String())
        }
    }

    fmt.Printf("%v\n", strings.Repeat("=", lineWidth))

    // End up with the most imporant stats - the overall performance.
    for _, phase := range phases {
        pstats := filter(stats, phaseFilter(phase))
        a := CreateAnalysis(pstats, "Total " + phase.ToString(), job)
        results = append(results, a)
        fmt.Printf("%v\n", a.String())
    }

    fmt.Printf("%v\n", strings.Repeat("=", lineWidth))
    return results
}



