// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"
import "io"
import "logger"
import "syscall"



/**
 * BlockConnection is for testing generic block performance.
 *
 * This is for things like iSCSI, where you do a kernel mount of a block device and then use like a local device.
 *
 * This is NOT for things that have their own libraries to access them (like RBD.  You *could* mount RBD with a
 * kernel driver, but you'll get better functionality using Ceph's RBD go package).
 */
type BlockConnection struct {
    device string
    protocol ProtocolConfig
    worker WorkerConnectionConfig

    /* either a unix file descriptor int or a windows Handle. */
    fd FileDescriptor
}


func NewBlockConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*BlockConnection, error) {
    var conn BlockConnection
    conn.device = target
    conn.protocol = protocol
    conn.worker = worker
    return &conn, nil
}


func (conn *BlockConnection) Target() string {
    return conn.device
}


func (conn *BlockConnection) ManagerConnect() error {
    return nil
}


func (conn *BlockConnection) ManagerClose() error {
    return nil
}


func (conn *BlockConnection) WorkerConnect() error {
    var err error

    conn.fd, err = Open(conn.device, syscall.O_RDWR, 0644)
    if err != nil {
        conn.fd = 0
        return err
    }

    offset, err := conn.fd.Seek(0, io.SeekEnd)
    if err != nil {
        return err
    }

    minSize := (conn.worker.ForemanRangeEnd - conn.worker.ForemanRangeStart) * conn.worker.ObjectSize
    if offset < int64(minSize) {
        return fmt.Errorf("Block device %v too small: only %v bytes when we need %v", conn.device, offset, minSize)
    }

    return nil
}


func (conn *BlockConnection) WorkerClose() error {
    return conn.fd.Close()
}


/* 
 * Helper function to determine an object's offset into the image from an object key 
 */
func (conn *BlockConnection) objectOffset(id uint64) int64 {
    return int64((id - conn.worker.ForemanRangeStart) * conn.worker.ObjectSize)
}


func (conn *BlockConnection) PutObject(key string, id uint64, buffer []byte) error {
    offset := conn.objectOffset(id)
    logger.Tracef("Put block object %v on %v with size %v and offset %v\n", id, conn.device, len(buffer), offset)

    for len(buffer) > 0 {
        n, err := conn.fd.Pwrite(buffer, offset)
        if err == nil {
            return err
        }

        buffer = buffer[n:]
        offset += int64(n)
    }

    return nil
}


func (conn *BlockConnection) GetObject(key string, id uint64, buffer []byte) error {
    offset := conn.objectOffset(id)
    logger.Tracef("Get block object %v on %v with size %v and offset %v\n", key, conn.device, conn.worker.ObjectSize, offset)

    remaining := conn.worker.ObjectSize
    start := 0

    for remaining > 0 {
        n, err := conn.fd.Pread(buffer[start:], offset)
        if err != nil {
            return err
        }

        start += n
        offset += int64(n)
        remaining -= uint64(n)
    }

    return nil
}


func (conn *BlockConnection) InvalidateCache() error {
    return nil
}
