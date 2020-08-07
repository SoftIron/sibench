package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"



/* apt install libcephfs-dev librbd-dev librados-dev */


type RadosConnection struct {
    monitor string
    config ConnectionConfig
    client *rados.Conn
    ioctx *rados.IOContext  // Handle to an open pool.
}


func NewRadosConnection(target string, config ConnectionConfig) (*RadosConnection, error) {
    var conn RadosConnection
    conn.monitor = target
    conn.config = config
    return &conn, nil
}


func (conn *RadosConnection) Target() string {
    return conn.monitor
}


func (conn *RadosConnection) ManagerConnect() error {
    return conn.WorkerConnect()
}


func (conn *RadosConnection) ManagerClose() error {
    return conn.WorkerClose()
}


func (conn *RadosConnection) WorkerConnect() error {
    var err error
    conn.client, err = rados.NewConnWithUser(conn.config["username"])
    if err != nil {
        return err
    }

    err = conn.client.SetConfigOption("mon_host", conn.monitor)
    if err != nil {
        return err
    }

    err = conn.client.SetConfigOption("key", conn.config["key"])
    if err != nil {
        return err
    }

    logger.Infof("Creating rados connection to %v as user %v\n", conn.monitor, conn.config["username"])

    err = conn.client.Connect()
    if err != nil {
        return err
    }

    pool := conn.config["pool"]

    // Check the pool we want exists so we can give a decent error message. 
    pools, err := conn.client.ListPools()
    found := false
    for _, p := range pools {
        if p == pool {
            found = true
        }
    }

    if !found {
        return fmt.Errorf("No such Ceph pool: %v\n", pool)
    }

    conn.ioctx, err = conn.client.OpenIOContext(pool)
    return err
}


func (conn *RadosConnection) WorkerClose() error {
    conn.client.Shutdown()
    return nil
}


func (conn *RadosConnection) PutObject(key string, contents []byte) error {
    return conn.ioctx.WriteFull(key, contents)
}


func (conn *RadosConnection) GetObject(key string) ([]byte, error) {
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

