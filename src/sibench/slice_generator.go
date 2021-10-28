package main

import "bytes"
import "encoding/binary"
import "fmt"
import "io"
import "io/fs"
import "math/rand"
import "os"
import "path"
import "strconv"

/*
 * SliceGenerator is a generator which biulds workloads from existing files.  It aims to reproduce
 * the compressibility of those files, whilst still creating an effectively infinite supply of 
 * different objects.
 *
 * It works by taking a directory of files (which will usually be of the same type: source code,
 * VM images, movies, or whatever), and then loading fixed sized slices of bytes
 * from random positions within those files.  The end result is that we have a library of (say) 1000
 * slices, each containing (say) 4Kb of data.  (Both of those values are run-time arguments).
 *
 * When asked to generate a new workload object we do the following:
 *   1,  Create a seed (chosen from our master PRNG).
 *   2.  Write the seed into the start of the workload object.
 *   3.  Use the seed to create a PRNG just for this workload object.
 *   4.  Use that prng to select slices from our library, which are concatenated onto the object
 *       until we have as many bytes as we were asked for.
 *
 * This approach means that we do not need to ever store the objects themselves: we can verify a 
 * read operation by reading the seed from the first few bytes, and then recreating the object we
 * would expect.
 */


type SliceGenerator struct {
    prng *rand.Rand
    sliceCount int
    sliceSize int
    slices [][]byte
}


func CreateSliceGenerator(seed uint64, config GeneratorConfig) (*SliceGenerator, error) {
    var sg SliceGenerator

    // No need to check for conversion errors here: these are the result of Itoa calls anyway.
    sg.sliceSize, _ = strconv.Atoi(config["size"])
    sg.sliceCount, _ = strconv.Atoi(config["count"])
    sg.prng = rand.New(rand.NewSource(int64(seed)))
    sg.slices = make([][]byte, sg.sliceCount)

    dirname := config["dir"]
	entries, err := os.ReadDir(dirname)

	if err != nil {
        return nil, fmt.Errorf("Unable to read slice directory %v: %v", dirname, err)
    }

    /* Build a array of file info objects for all the regular files in our directory.
     * Also, compute the total number of bytes in those files. */

    var totalBytes uint64 = 0
    infos := make([]fs.FileInfo, 0)

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
            return nil, fmt.Errorf("Unable to stat %v/%v: %v", dirname, e.Name(), err)
		}

        if info.Mode().IsRegular() {
            infos = append(infos, info)
            totalBytes += uint64(info.Size())
        }
	}

    /* If our total number of bytes is less than our requested slice sliceSize, then we can't make slices! */
    if uint64(sg.sliceSize) > totalBytes {
        return nil, fmt.Errorf("Not enough bytes in the files in %v - need at least %v", dirname, sg.sliceSize)
    }

    for i :=0; i < sg.sliceCount; i++ {
        sg.slices[i], err = sg.loadSlice(totalBytes, dirname, infos)
        if err != nil {
            return nil, err
        }
    }

    return &sg, nil
}



/*
 * Load a slice if data at random from the contents of the files in our slice directory.
 * The slice can span across multiple files: in effect we are concatenating the contents
 * of all the files in the directory into a buffer, and then picking a random position 
 * in that buffer to read as many bytes as we need for our slice.
 * (We don't actually do it like, obviously, but the effect is the same).
 */
func (sg *SliceGenerator) loadSlice(totalBytes uint64, dirname string, infos []fs.FileInfo) ([]byte, error) {
    lastStart := totalBytes - uint64(sg.sliceSize)
    start := sg.prng.Int63n(int64(lastStart))
    var pos int64 = 0
    read := 0

    result := make([]byte, sg.sliceSize)

    for _, info := range infos {
        if pos + info.Size() >= start {
            filename := path.Join(dirname, info.Name())
            f, err := os.Open(filename)
            if err != nil {
                return nil, fmt.Errorf("Unable to open %v: %v", filename, err)
            }

            defer f.Close()

            offset := start - pos
            if offset < 0 {
                offset = 0
            }

            n, err := f.ReadAt(result[read:], offset)
            read += n

            if read == sg.sliceSize {
                return result, nil
            }

            if err != io.EOF {
                return nil, fmt.Errorf("Unable read %v bytes from offset %v in %v: %v", sg.sliceSize - read, offset, filename, err)
            }
        }

        pos += info.Size()
    }

    return nil, fmt.Errorf("Should never happen!")
}



func (sg *SliceGenerator) Generate(size uint64, key string, cycle uint64) []byte {
    seed := uint32(sg.prng.Int())
    return sg.generateFromSeed(size, seed)
}



func (sg *SliceGenerator) generateFromSeed(size uint64, seed uint32) []byte {
    result := make([]byte, size)
    binary.LittleEndian.PutUint32(result, seed)
    tmp_prng := rand.New(rand.NewSource(int64(seed)))

    for start := uint64(4); start < size; start += uint64(sg.sliceSize) {
        /* Copy does the computation of min( len(src), len(dst) ) for us, so we don't need to worry */
        copy(result[start:], sg.slices[tmp_prng.Int63n(int64(sg.sliceCount))])
    }

    return result
}



func (sg *SliceGenerator) Verify(size uint64, key string, contents []byte) error {
    if uint64(len(contents)) != size {
        return fmt.Errorf("Incorrect size: expected %v but got %v\n", size, len(contents))
    }

    // Read the seed from the header of the payload
    seed := binary.LittleEndian.Uint32(contents)

    // Now we can generate the expected buffer to compare against.
    expected := sg.generateFromSeed(size, seed)

    if bytes.Compare(contents, expected) != 0 {
        return fmt.Errorf("Buffers do not match\n")
    }

    return nil
}


