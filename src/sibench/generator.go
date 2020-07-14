package main

import "fmt"
import "io"


type Generator interface {
    /* 
     * Generate creates a payload for an object.
     * size is the size of the payload in bytes.
     * key is the object name.
     * cycle is a counter that should be incremented if overwriting an object, so that the contents will not be the same as before. 
     */
    Generate(size uint64, key string, cycle uint64) io.ReadSeeker

    /*
     * Verify checks if the contents of a payload are well-formed. 
     * Returns nil on success, or an error on failure.
     */
    Verify(size uint64, key string, contents io.Reader) error
}


func CreateGenerator(generatorType string, seed uint64) (Generator, error) {
    switch generatorType {
        case "prng": return CreatePrngGenerator(seed), nil
    }

    return nil, fmt.Errorf("Unknown generatorType: %v", generatorType)
}

