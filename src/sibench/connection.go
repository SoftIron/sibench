package main

import "fmt"


/* 
 * Connection is the abstraction of different storage backends.  
 */
type Connection interface {
    /* Return the target of this conection, as a convenience */
    Target() string

    ListBuckets() ([]string, error)
    CreateBucket(bucket string) error
    DeleteBucket(bucket string) error

    ListObjects(bucket string) ([]string, error)
    PutObject(bucket string, key string, contents []byte) error
    GetObject(bucket string, key string) ([]byte, error)

    /* Close the connection */
    Close()
}



/*
 * Factory function that mints new connections of the appropriate type.
 */
func NewConnection(connectionType string, target string, port uint16, credentials map[string]string) (Connection, error) {
    switch connectionType {
        case "s3":    return NewS3Connection(target, port, credentials)
        case "rados": return NewRadosConnection(target, port, credentials)
    }

    return nil, fmt.Errorf("Unknown connectionType: %v", connectionType)
}

