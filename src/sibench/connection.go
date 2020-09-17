package main

import "fmt"


/* 
 * Connection is the abstraction of different storage backends.  
 */
type Connection interface {
    /* Return the target of this conection, as a convenience for logging an so forth */
    Target() string

    ManagerConnect() error
    ManagerClose() error

    WorkerConnect() error
    WorkerClose() error

    /* 
     * Both Key and ID uniquely identify the same object.
     *
     * Key and ID are redundant, but we provide both so that we don't need to do any string
     * operations inside Put or Get.  String ops are slow, and we don't want to compromise
     * our timing by incuding them. 
     *
     * Things like FileSystems tend to want string-based keys.  Block devices usually just use
     * offsets, which are more easily calculated directly from an ID number.
     */

    PutObject(key string, id uint64, contents []byte) error
    GetObject(key string, id uint64) ([]byte, error)
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
    switch connectionType {
        case "s3":      return NewS3Connection(target, protocolConfig, workerConfig)
        case "rados":   return NewRadosConnection(target, protocolConfig, workerConfig)
        case "cephfs":  return NewCephFSConnection(target, protocolConfig, workerConfig)
        case "rbd":     return NewRbdConnection(target, protocolConfig, workerConfig)
        case "block":   return NewBlockConnection(target, protocolConfig, workerConfig)
    }

    return nil, fmt.Errorf("Unknown connectionType: %v", connectionType)
}

