package main

import "fmt"


/* 
 * Generators create the contents for Objects writes, and can verify the contents
 * from object reads.
 * 
 * Currently we have only the PRNG Generator, but will probably add a Zero Generator
 * which sets data to be all zeroes (as it's cheap and allows us to get more bandwidth
 * from a single Sibench server, when correctness testing isn't required).
 *
 * We may also end up creating a set of generators for different file types: for instance
 * jpeg, mpeg and others, so that we can see how compression performs with different content.
 * (The current PRNG generator will be particularly hard on compression, and a Zero
 * generator would be unreasonably trivial).
 */
type Generator interface {
    /* 
     * Generate creates a payload for an object.
     * size is the size of the payload in bytes.
     * key is the object name.
     * cycle is a counter that should be incremented if overwriting an object, so that the contents will not be the same as before. 
     */
    Generate(size uint64, key string, cycle uint64) []byte

    /*
     * Verify checks if the contents of a payload are well-formed. 
     * Returns nil on success, or an error on failure.
     */
    Verify(size uint64, key string, contents []byte) error
}


/* 
 * Factory function that mints new generators.
 */
func CreateGenerator(generatorType string, seed uint64) (Generator, error) {
    switch generatorType {
        case "prng": return CreatePrngGenerator(seed), nil
    }

    return nil, fmt.Errorf("Unknown generatorType: %v", generatorType)
}

