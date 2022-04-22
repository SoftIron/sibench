// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "bytes"
import "encoding/binary"
import "fmt"


// Cheap hash function.
func prng(lastValue uint64) uint64 {
	x := lastValue
	x ^= x << 13;
	x ^= x >> 7;
	x ^= x << 17;
	return x;
}


/*
 * The PRNG generator is the default content generator for sibench.
 *
 * It creates test objects of the following form:
 *
 *   1. Object size (8 bytes)
 *   2. Cycle (8 bytes): every time we overwrite an object, we use different contents using the cycle 
 *      number to distinguish.
 *   3. A PRNG Seeed (8 bytes).
 *   4. Key length (8 bytes)
 *   5. Key (variable number of bytes).
 *   6. Padding to take us to an eight byte boundary.
 *   7. Random data derived from the seed in (3).  This fills in any remaining space in the object.
 *
 * We don't technically need anything other than a seed in the header, but storing the other fields 
 * allows verification that the back-end storage really is doing what we expect it to do (getting
 * keys correct and so forth).
 */
type PrngGenerator struct {
    seed uint64
}



func CreatePrngGenerator(seed uint64, config GeneratorConfig) (*PrngGenerator, error) {
    // PrngGenerator takes no configuration parameters.

    var pg PrngGenerator
    pg.seed = seed
    return &pg, nil
}


func (pg *PrngGenerator) Generate(size uint64, key string, cycle uint64) []byte {
    buf := make([]byte, 0, size)
    tmp64 := make([]byte, 8)
    zeroes := make([]byte, 8)

    // Write our size
    binary.LittleEndian.PutUint64(tmp64, size)
    buf = append(buf, tmp64...)

    // Write our cycle
    binary.LittleEndian.PutUint64(tmp64, cycle)
    buf = append(buf, tmp64...)

    // Write our seed
    binary.LittleEndian.PutUint64(tmp64, pg.seed)
    buf = append(buf, tmp64...)

    // Write our key length and key
    binary.LittleEndian.PutUint64(tmp64, uint64(len(key)))
    buf = append(buf, tmp64...)
    buf = append(buf, key...)

    // Pad to an 8-byte boundary
    pad_len := 7 - ((len(buf) + 7) % 8)
    buf = append(buf, zeroes[:pad_len]...)

    // Seed our prng from the global seed, and from the data we've marshalled so far.
    next := pg.seed
    for _, b := range buf {
        next = prng(next ^ uint64(b))
    }

    remaining_buf := size - uint64(len(buf))
    remaining_64s := remaining_buf / 8

    for i := uint64(0); i < remaining_64s; i ++ {
        binary.LittleEndian.PutUint64(tmp64, next)
        buf = append(buf, tmp64...)
        next = prng(next)
    }

    // Pad with zeroes until the end
    pad_len = int(remaining_buf % 8)
    buf = append(buf, zeroes[:pad_len]...)

    return buf
}



func (pg *PrngGenerator) Verify(size uint64, key string, contents []byte) error {
    if uint64(len(contents)) != size {
        return fmt.Errorf("Incorrect size: expected %v but got %v\n", size, len(contents))
    }

    // Read the cycle from the header of the payload: it's the only bit we don't necessarily know. 
    cycle := binary.LittleEndian.Uint64(contents[8:])

    // Now we can generate the expected buffer to compare against.
    expected := pg.Generate(size, key, cycle)

    if bytes.Compare(contents, expected) != 0 {
        return fmt.Errorf("Buffers do not match\n")
    }

    return nil
}

