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

    PutObject(key string, contents []byte) error
    GetObject(key string) ([]byte, error)
}



/*
 * Factory function that mints new connections of the appropriate type.
 *
 * The config is a string->string map that contains all the type-specific details a Connection
 * needs (such as username, key, S3 bucket, ceph poo.l..)
 */
func NewConnection(connectionType string, target string, config ConnectionConfig) (Connection, error) {
    switch connectionType {
        case "s3":      return NewS3Connection(target, config)
        case "rados":   return NewRadosConnection(target, config)
        case "cephfs":  return NewCephFSConnection(target, config)
        case "rbd":     return NewRBDConnection(target, config)
    }

    return nil, fmt.Errorf("Unknown connectionType: %v", connectionType)
}

