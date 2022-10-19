// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"
import "runtime"


/* 
 * Connection is the abstraction of different storage backends.  
 */
type Connection interface {
    /* Return the target of this conection, as a convenience for logging an so forth */
    Target() string

    /* The manager will typically open a connection to the backend to test it, and will then
     * close it before firing up the workers to do their thing. */
    ManagerConnect() error
    ManagerClose() error

    WorkerConnect() error
    WorkerClose() error

    /* 
     * Returns true if the Put/Get methods make use of a string key, or if we can get away with
     * just an integer id.
     *
     * We do this because:
     *   (a) generating the string key can be expensive, and we don't want to do it INSIDE our
     *       timing code.
     *   (b) We don't want to do it all if we don't need to.  This lets a worker decide...
     */
    RequiresKey() bool

    /* 
     * Both Key and ID uniquely identify the same object.
     *
     * FileSystems tend to want string-based keys.  Block devices usually want to use
     * offsets, which are more easily calculated directly from an integer ID number.
     *
     * Key and ID are redundant, but we provide both so that we don't need to do any string
     * operations inside Put or Get (ie, not while we're inside the timed section of the code).
     *
     * Both methods take a slice with exactly the capacity we expect to read or write.
     */

    PutObject(key string, id uint64, buffer []byte) error
    GetObject(key string, id uint64, buffer []byte) error
    DeleteObject(key string, id uint64) error

    InvalidateCache() error
}


/* 
 * WorkerConnectionConfig is all the non-protocol specific information that a particular worker
 * knows that might be useful when constructing a new connection.
 */
type WorkerConnectionConfig struct {
    Hostname string
    WorkerId uint64
    ObjectSize uint64
    ForemanRangeStart uint64
    ForemanRangeEnd uint64
    WorkerRangeStart uint64
    WorkerRangeEnd uint64
}


/*
 * Factory function that mints new connections of the appropriate type.
 *
 * config is a string->string map that contains all the protocol-specific details a Connection
 * needs (such as username, key, S3 bucket, ceph pool..)
 *
 * workerConfig is all the protocol-independent information that a worker knows that might be needed
 * for to make a new connection.
 */
func NewConnection(connectionType string, target string, protocolConfig ProtocolConfig, workerConfig WorkerConnectionConfig) (Connection, error) {
    if runtime.GOOS == "linux" {
        switch connectionType {
            case "rados":   return NewRadosConnection(target, protocolConfig, workerConfig)
            case "cephfs":  return NewCephFSConnection(target, protocolConfig, workerConfig)
            case "rbd":     return NewRbdConnection(target, protocolConfig, workerConfig)
        }
    }

    switch connectionType {
        case "s3":      return NewS3Connection(target, protocolConfig, workerConfig)
        case "block":   return NewBlockConnection(target, protocolConfig, workerConfig)
        case "file":    return NewFileConnection(target, protocolConfig, workerConfig)
    }

    return nil, fmt.Errorf("Unknown connectionType: %v", connectionType)
}

