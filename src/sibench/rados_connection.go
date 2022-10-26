// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

// +build linux

package main

import "fmt"
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


func (conn *RadosConnection) ManagerClose(cleanup bool) error {
    return conn.WorkerClose(cleanup)
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


func (conn *RadosConnection) WorkerClose(cleanup bool) error {
    conn.ioctx.Destroy()
    conn.client.Shutdown()
    return nil
}


func (conn *RadosConnection) RequiresKey() bool {
    return true
}


func (conn *RadosConnection) CanDelete() bool {
    return true
}


func (conn *RadosConnection) PutObject(key string, id uint64, buffer []byte) error {
    err := conn.ioctx.WriteFull(key, buffer)
    return err
}


func (conn *RadosConnection) GetObject(key string, id uint64, buffer []byte) error {
    stat, err := conn.ioctx.Stat(key)
    if err != nil {
        return err
    }

    if uint64(cap(buffer)) != stat.Size {
        return fmt.Errorf("Object has wrong size: expected v, but got %v", cap(buffer), stat.Size)
    }

    var nread int

    nread, err = conn.ioctx.Read(key, buffer, 0)
    if err != nil {
        return err
    }

    if uint64(nread) != stat.Size {
        return fmt.Errorf("Short read: wanted %v bytes, but got %v", stat.Size, nread)
    }

    return nil
}


func (conn *RadosConnection) DeleteObject(key string, id uint64) error {
    err := conn.ioctx.Delete(key)
    return err
}


func (conn *RadosConnection) InvalidateCache() error {
    return nil
}

