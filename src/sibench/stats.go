package main

import "fmt"


/* 
 * Reset all off the summary's counters to 0. 
 */
func (s *StatSummary) Zero() {
    for phase := 0; phase < int(SP_Len); phase++ {
        for err :=0; err < int(SE_Len); err++ {
            s[phase][err] = 0
        }
    }
}


/*
 * Add one summary to another
 */
func (s *StatSummary) Add(other *StatSummary) {
    for phase := 0; phase < int(SP_Len); phase++ {
        for err :=0; err < int(SE_Len); err++ {
            s[phase][err] += other[phase][err]
        }
    }
}


/* Convert to K, G, M etc. units */
func units(val uint64) string {
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


func (s *StatSummary) ToString(objectSize uint64) string {
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
            bw := units(ops * objectSize * 8)

            result += fmt.Sprintf("[%v] ops: %v,  bw: %vb/s,  ofail: %v,  vfail: %v ", phase, ops, bw, ofail, vfail)
        }
    }

    return result
}


/*
 * Do the maths for a slice full of detailed stats.
 */
func CrunchTheNumbers(stats []Stat, job *Job) {

}



