// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"
import "sort"



/* 
 * A ServerStat wraps a Stat to add a field for Server ID. 
 */
type ServerStat struct {
    Stat
    ServerIndex uint16
}



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


func (s *StatSummary) Total() uint64 {
    total := uint64(0)

    for phase := 0; phase < int(SP_Len); phase++ {
        for err :=0; err < int(SE_Len); err++ {
            total += s[phase][err]
        }
    }

    return total
}


/* Produce a human readable string from a StatSummary object */
func (s *StatSummary) String(objectSize uint64, useBytes bool) string {
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
            bwb := ToUnits(ops * objectSize)
            bw := ToUnits(ops * objectSize * 8)
            bwstr := ""
            if useBytes {
                bwstr = fmt.Sprintf("%vB/s", bwb)
            } else {
                bwstr = fmt.Sprintf("%vb/s", bw)
            }
            result += fmt.Sprintf("[%v] ops: %v,  bw: %v,  ofail: %v,  vfail: %v ", phase, ops, bwstr, ofail, vfail)
        }
    }

    if result == "" {
        result = "No operations completed"
    }

    return result
}


/*
 * We will use a filter function to include/exclude stats from a slice
 * We could use anonymous functions to do the exact same thing with less code, but
 * by giving explicit names to things we can make it a lot more readable. 
 */
type filterFunc func(*ServerStat) bool


/* Filter based on phase */
func phaseFilter(phase StatPhase) filterFunc {
    return func(s *ServerStat) bool {
        return s.Phase == phase
    }
}


/* Filter based on error type */
func errorFilter(err StatError) filterFunc {
    return func(s *ServerStat) bool {
        return s.Error == err
    }
}


/* Filter out stats that are not in the relevant time period */
func rampFilter(job *Job) filterFunc {

    // Convert seonds to milliseconds
    up := uint32(job.rampUp * 1000)
    time := uint32(job.runTime * 1000)

    return func(s *ServerStat) bool {
        start := uint32(s.TimeSincePhaseStartMillis)
        return (start > up) && (start <= up + time)
    }
}


/* Filter on target */
func targetFilter(targetIndex uint16) filterFunc {
    return func(s *ServerStat) bool {
        return s.TargetIndex == targetIndex
    }
}


/* Filter on server */
func serverFilter(serverIndex uint16) filterFunc {
    return func(s *ServerStat) bool {
        return s.ServerIndex == serverIndex
    }
}


/* Inverts the sense of a filter function */
func invertFilter(fn filterFunc) filterFunc {
    return func(s *ServerStat) bool {
        return !fn(s)
    }
}


/* Apply filters to a slice of stats, returning a new slice that contains only the matching values */
func filter(stats []*ServerStat, fns ...filterFunc) []*ServerStat {
    var results []*ServerStat

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
func sortByDuration(stats []*ServerStat) {
    sort.Slice(stats, func(i, j int) bool {
        return stats[i].DurationMicros < stats[j].DurationMicros
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
    Phase string
    IsTotal bool

    /* All response times in ms */
    ResTimeMin uint64   // The fastest reponse we had for a successful operation
    ResTimeMax uint64   // The slowest response we had for a successful operation
    ResTime95  uint64   // The response time by which 95% of our successful operations completed
    ResTimeAvg uint64   // The average response time for a successful operation

    /* Bandwidth is in bits per seconds */
    Bandwidth uint64
    BandwidthBytes uint64

    /* Counts */
    Successes uint64
    Failures uint64
}


/*
 * Produce a human-readable string from an Analysis.
 * This is intended to be used to dump tables of Analyses, and aligns fields nicely for that purpose.
 */
func (a *Analysis) String(useBytes bool) string {
    bwstr := ""
    if useBytes {
        bwstr = fmt.Sprintf("%vB/s", ToUnits(a.BandwidthBytes))
    } else {
        bwstr = fmt.Sprintf("%vb/s", ToUnits(a.Bandwidth))
    }

    return fmt.Sprintf("%-28v   bandwidth: %7v,  ok: %6v,  fail: %6v,  res-min: %5v ms,  res-max: %5v ms,  res-95: %6v ms, res-avg: %6v ms",
        a.Name,
        bwstr,
        a.Successes,
        a.Failures,
        a.ResTimeMin / 1000,
        a.ResTimeMax / 1000,
        a.ResTime95  / 1000,
        a.ResTimeAvg / 1000)
}


/* 
 * Create an Analysis object describing a slice of stats.
 * We pass in the name that we wish to give the Analysis.
 * The job is needed so that we can pul run times and object size from it.
 */
func NewAnalysis(stats []*ServerStat, name string, phase StatPhase, isTotal bool, job *Job) *Analysis {
    var result Analysis
    result.Name =name
    result.Phase = phase.ToString()
    result.IsTotal = isTotal

    good := filter(stats, errorFilter(SE_None))
    result.Successes = uint64(len(good))
    result.Failures = uint64(len(stats) - len(good))

    if len(good) > 0 {
        sortByDuration(good)

        // Would like to use Duration.Milliseconds, but it doesn't exist in our go version.
        result.ResTimeMin = uint64(good[0].DurationMicros)
        result.ResTimeMax = uint64(good[len(good) - 1].DurationMicros)
        result.ResTime95  = uint64(good[int(float64(len(good)) * 0.95)].DurationMicros)
        result.Bandwidth  = uint64(8 * len(good)) * job.order.ObjectSize / job.runTime
        result.BandwidthBytes  = uint64(len(good)) * job.order.ObjectSize / job.runTime


        total := uint64(0)
        for i, _ := range(good) {
            total += uint64(good[i].DurationMicros)
        }

        result.ResTimeAvg = total / uint64(len(good))
    }

    return &result
}


/*
 * Limit a string to a particular length.  Longer strings will be truncated and '...' appended to them
 * to indiate that the truncation has taken place.
 */
func limit(s string, length int) string {
    if len(s) <= length {
        return s
    }

    return string(s[:length - 1]) + "..."
}


