// +build linux

package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"



/**
 * A Connection for talking raw RADOS to a ceph cluster, using the standard Ceph librados native library.
 */
type RadosConnection struct {
    monitor string
    protocol ProtocolConfig
    client *rados.Conn
    ioctx *rados.IOContext  // Handle to an open pool.
}


func NewRadosConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*RadosConnection, error) {
    var conn RadosConnection
    conn.monitor = target
    conn.protocol = protocol
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
    conn.client, err = NewCephClient(conn.monitor, conn.protocol)
    if err != nil {
        return err
    }

    conn.ioctx, err = conn.client.OpenIOContext(conn.protocol["pool"])
    return err
}


func (conn *RadosConnection) WorkerClose() error {
    conn.ioctx.Destroy()
    conn.client.Shutdown()
    return nil
}


func (conn *RadosConnection) PutObject(key string, id uint64, contents []byte) error {
    logger.Tracef("Put rados object %v on %v: start\n", key, conn.monitor)
    err := conn.ioctx.WriteFull(key, contents)
    logger.Tracef("Put rados object %v on %v: end\n", key, conn.monitor)
    return err
}


func (conn *RadosConnection) GetObject(key string, id uint64) ([]byte, error) {
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


func (conn *RadosConnection) InvalidateCache() error {
    return nil
}

