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
 *   2. Cycle (8 bytes): every time we overwrite an object, we use different buffer using the cycle 
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


func (pg *PrngGenerator) Generate(size uint64, key string, cycle uint64, buf *[]byte) {
    i := 0

    // Write our size
    binary.LittleEndian.PutUint64((*buf)[i:], size)
    i += 8

    // Write our cycle
    binary.LittleEndian.PutUint64((*buf)[i:], cycle)
    i += 8

    // Write our seed
    binary.LittleEndian.PutUint64((*buf)[i:], pg.seed)
    i += 8

    // Write our key length
    binary.LittleEndian.PutUint64((*buf)[i:], uint64(len(key)))
    i += 8

    // Write our key
    i += copy((*buf)[i:], key)

    // Pad to an 8-byte boundary
    pad_len := 7 - ((i + 7) % 8)
    for j := 0; j < pad_len; j++ {
        (*buf)[i] = 0
        i += 1
    }

    // Seed our prng from the global seed, and from the data we've marshalled so far.
    next := pg.seed
    for _, b := range *buf {
        next = prng(next ^ uint64(b))
    }

    remaining_buf := size - uint64(len(*buf))
    remaining_64s := remaining_buf / 8

    for i := uint64(0); i < remaining_64s; i ++ {
        binary.LittleEndian.PutUint64((*buf)[i:], next)
        i += 8
        next = prng(next)
    }

    // Pad with zeroes until the end
    pad_len = int(remaining_buf % 8)
    for j := 0; j < pad_len; j++ {
        (*buf)[i] = 0
        i += 1
    }
}



func (pg *PrngGenerator) Verify(size uint64, key string, buffer *[]byte, scratch *[]byte) error {
    if uint64(len(*buffer)) != size {
        return fmt.Errorf("Incorrect size: expected %v but got %v\n", size, len(*buffer))
    }

    // Read the cycle from the header of the payload: it's the only bit we don't necessarily know. 
    cycle := binary.LittleEndian.Uint64((*buffer)[8:])

    // Now we can generate the expected buffer to compare against.
    pg.Generate(size, key, cycle, scratch)

    if bytes.Compare(*buffer, *scratch) != 0 {
        return fmt.Errorf("Buffers do not match\n")
    }

    return nil
}

