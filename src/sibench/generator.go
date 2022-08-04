// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"


/* 
 * Generators create the contents for Objects writes, and can verify the contents
 * from object reads.
 *
 * Generators should be written so that the data returned by a connection read can 
 * be verified WITHOUT storing the expected data in memory for comparison.  Typically
 * this means that the data should be generated algorithmically from a seed written
 * into (or derivable from) the header of the object.
 */
type Generator interface {
    /* 
     * Generate creates a payload for an object.
     * size is the size of the payload in bytes.
     * key is the object name.
     * cycle is a counter that should be incremented if overwriting an object, so that the contents will not be the same as before. 
     * buffer is the buffer into which we will write the object.  It must be at least as big as size.
     */
    Generate(size uint64, key string, cycle uint64, buffer *[]byte)

    /*
     * Verify checks if the contents of a payload are well-formed.
     * buffer is the actual contents of the object.
     * Scratch is a scratch buffer, and should be at least as big as the expected object.
     * Returns nil on success, or an error on failure.
     */
    Verify(size uint64, key string, buffer *[]byte, scratch *[]byte) error
}


/* 
 * Factory function that mints new generators.
 */
func CreateGenerator(generatorType string, seed uint64, config GeneratorConfig) (Generator, error) {
    switch generatorType {
        case "prng": return CreatePrngGenerator(seed, config)
        case "slice": return CreateSliceGenerator(seed, config)
    }

    return nil, fmt.Errorf("Unknown generatorType: %v", generatorType)
}

