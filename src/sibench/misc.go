// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"


/**
 * Find the greatest power-of-two number that is less than or equal to x.
 * (Credit to Hacker's Delight for the cunning branchless way of doing this!  A very cool trick...)
 */
func previousPowerOfTwo(x uint64) uint64 {
    x = x | (x >> 1);
    x = x | (x >> 2);
    x = x | (x >> 4);
    x = x | (x >> 8);
    x = x | (x >> 16);
    x = x | (x >> 32);

    return x - (x >> 1);
}


/* Convert values into to K, G, M etc. units */
func ToUnits(val uint64) string {
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

