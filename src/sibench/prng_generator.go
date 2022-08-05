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


func (pg *PrngGenerator) Generate(size uint64, id uint64, cycle uint64, buf *[]byte) {
    pos := 0

    // Write our size
    binary.LittleEndian.PutUint64((*buf)[pos:], size)
    pos += 8

    // Write our cycle
    binary.LittleEndian.PutUint64((*buf)[pos:], cycle)
    pos += 8

    // Write our seed
    binary.LittleEndian.PutUint64((*buf)[pos:], pg.seed)
    pos += 8

    // Write our id
    binary.LittleEndian.PutUint64((*buf)[pos:], id)
    pos += 8

    // Seed our prng from the global seed, and the first few fields that make us unique.
    next := pg.seed
    next = prng(next ^ size)
    next = prng(next ^ cycle)
    next = prng(next ^ id)

    remaining_buf := size - uint64(pos)
    remaining_64s := remaining_buf / 8

    for i := uint64(0); i < remaining_64s; i++ {
        binary.LittleEndian.PutUint64((*buf)[pos:], next)
        pos += 8
        next = prng(next)
    }

    // Pad with zeroes until the end
    pad_len := int(remaining_buf % 8)
    for i := 0; i < pad_len; i++ {
        (*buf)[pos] = 0
        pos += 1
    }
}


func (pg *PrngGenerator) Verify(size uint64, id uint64, buffer *[]byte, scratch *[]byte) error {
    if uint64(len(*buffer)) != size {
        return fmt.Errorf("Incorrect size: expected %v but got %v\n", size, len(*buffer))
    }

    // Read the cycle from the header of the payload: it's the only bit we don't necessarily know. 
    cycle := binary.LittleEndian.Uint64((*buffer)[8:])

    // Now we can generate the expected buffer to compare against.
    pg.Generate(size, id, cycle, scratch)

    if bytes.Compare(*buffer, *scratch) != 0 {
        for i := uint64(0); i < size; i++ {
            if (*buffer)[i] != (*scratch)[i] {
                return fmt.Errorf("Buffers do not match at position %v\n", i)
            }
        }
    }

    return nil
}

