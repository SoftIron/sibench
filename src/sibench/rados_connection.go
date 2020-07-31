package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"



/* apt install libcephfs-dev librbd-dev librados-dev */


type RadosConnection struct {
    target string
    client *rados.Conn
    ioctx *rados.IOContext  // Handle to an open pool.
    pool string             // The name of the pool our IO context is bound to.
}


func NewRadosConnection(monitor string, port uint16, credentialMap map[string]string) (*RadosConnection, error) {
    client, err := rados.NewConnWithUser(credentialMap["username"])
    if err != nil {
        return nil, err
    }

    err = client.SetConfigOption("mon_host", monitor)
    if err != nil {
        return nil, err
    }

    err = client.SetConfigOption("key", credentialMap["key"])
    if err != nil {
        return nil, err
    }

    logger.Infof("Creating rados connection to %v as user %v\n", monitor, credentialMap["username"])

    err = client.Connect()
    if err != nil {
        return nil, err
    }

    var conn RadosConnection
    conn.target = monitor
    conn.client = client

    return &conn, nil
}


func (conn *RadosConnection) Target() string {
    return conn.target
}


func (conn *RadosConnection) ListBuckets() ([]string, error) {
    return conn.client.ListPools()
}


func (conn *RadosConnection) CreateBucket(bucket string) error {
    // We don't create pools in rados, since we need a lot more information about the cluster
    // to get things like pgnum correct.  Instead, just verify that the pool already exists.

    buckets, err := conn.ListBuckets()
    if err != nil {
        return err
    }

    for _, b := range buckets {
        if b == bucket {
            return nil
        }
    }

    return fmt.Errorf("Pool needs to exist: %v", bucket)
}


func (conn *RadosConnection) DeleteBucket(bucket string) error {
    // We don't create pools, so we don't delete them either.
    return nil
}


func (conn *RadosConnection) ListObjects(bucket string) ([]string, error) {
    err := conn.setPool(bucket)
    if err != nil {
        return nil, err
    }

    var objects []string
    err = conn.ioctx.ListObjects(func(obj string) {
        objects = append(objects, obj)
    })

    if err != nil {
        return nil, err
    }

    return objects, nil
}


func (conn *RadosConnection) PutObject(bucket string, key string, contents []byte) error {
    err := conn.setPool(bucket)
    if err != nil {
        return err
    }

    return conn.ioctx.WriteFull(key, contents)
}


func (conn *RadosConnection) GetObject(bucket string, key string) ([]byte, error) {
    err := conn.setPool(bucket)
    if err != nil {
        return nil, err
    }

    stat, err := conn.ioctx.Stat(key)
    if err != nil {
        return nil, err
    }

    buffer := make([]byte, stat.Size)
    var nread int

    nread, err = conn.ioctx.Read(key, buffer, 0)
    if err != nil {
        return nil, err
    }

    if uint64(nread) != stat.Size {
        return nil, fmt.Errorf("Short read: wanted %v bytes, but got %v", stat.Size, nread)
    }

    return buffer, nil
}


func (conn *RadosConnection) Close() {
    logger.Infof("Closing rados connection to %v\n", conn.Target())
    conn.client.Shutdown()
}


/* 
 * Makes sure that our IO Context points to the correct pool.
 * If it does, we do nothing.
 * If it points to a different pool, we destroy the current IO context and create a new one.
 */
func (conn *RadosConnection) setPool(pool string) error {
    if pool == conn.pool {
        // Nothing to do.  Our current IO Context points to the correct pool.
        return nil
    }

    if conn.ioctx != nil {
        // Our IO Context points to the wrong pool.  Close it first.
        conn.ioctx.Destroy()
    }

    var err error
    conn.ioctx, err = conn.client.OpenIOContext(pool)
    return err
}
