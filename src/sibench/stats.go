package main


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

