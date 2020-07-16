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


type PrngGenerator struct {
    seed uint64
}



func CreatePrngGenerator(seed uint64) *PrngGenerator {
    var pg PrngGenerator
    pg.seed = seed
    return &pg
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

