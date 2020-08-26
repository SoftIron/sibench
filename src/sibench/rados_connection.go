package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"




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
    conn.client, err = NewCephClient(conn.monitor, conn.config)
    if err != nil {
        return err
    }

    conn.ioctx, err = conn.client.OpenIOContext(conn.config["pool"])
    return err
}


func (conn *RadosConnection) WorkerClose() error {
    conn.ioctx.Destroy()
    conn.client.Shutdown()
    return nil
}


func (conn *RadosConnection) PutObject(key string, contents []byte) error {
    logger.Tracef("Put rados object %v on %v: start\n", key, conn.monitor)
    err := conn.ioctx.WriteFull(key, contents)
    logger.Tracef("Put rados object %v on %v: end\n", key, conn.monitor)
    return err
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

