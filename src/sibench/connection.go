package main

import "io"
import "fmt"


/* 
 * Connection is the abstraction of different storage backends.  
 *
 * Currently we ony have an S3 connection, but we should add librados, CephFS and
 * others too.
 */
type Connection interface {
    /* Return the target of this conection, as a convenience */
    Target() string

    ListBuckets() ([]string, error)
    CreateBucket(bucket string) error
    DeleteBucket(bucket string) error

    ListObjects(bucket string) ([]string, error)
    PutObject(bucket string, key string, contents io.ReadSeeker) error
    GetObject(bucket string, key string) (io.Reader, error)

    /* Close the connection */
    Close()
}



/*
 * Factory function that mints new connections of the appropriate type.
 */
func CreateConnection(connectionType string, target string, port uint16, credentials map[string]string) (Connection, error) {
    switch connectionType {
        case "s3": return CreateS3Connection(target, port, credentials)
    }

    return nil, fmt.Errorf("Unknown connectionType: %v", connectionType)
}

