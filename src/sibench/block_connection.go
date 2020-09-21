package main

import "fmt"
import "io"
import "logger"
import "syscall"



type BlockConnection struct {
    device string
    protocol ProtocolConfig
    worker WorkerConnectionConfig
    fd int
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

    conn.fd, err = syscall.Open(conn.device, syscall.O_RDWR | syscall.O_DIRECT | syscall.O_SYNC, 0644)
    if err != nil {
        conn.fd = 0
        return err
    }

    offset, err := syscall.Seek(conn.fd, 0, io.SeekEnd)
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
    if conn.fd != 0 {
        return syscall.Close(conn.fd)
    }

    return nil
}


/* 
 * Helper function to determine an object's offset into the image from an object key 
 */
func (conn *BlockConnection) objectOffset(id uint64) int64 {
    return int64((id - conn.worker.ForemanRangeStart) * conn.worker.ObjectSize)
}


func (conn *BlockConnection) PutObject(key string, id uint64, contents []byte) error {
    offset := conn.objectOffset(id)
    logger.Tracef("Put block object %v on %v with size %v and offset %v\n", id, conn.device, len(contents), offset)

    for len(contents) > 0 {
        n, err := syscall.Pwrite(conn.fd, contents, offset)
        if err == nil {
            return err
        }

        contents = contents[n:]
        offset += int64(n)
    }

    return nil
}



func (conn *BlockConnection) GetObject(key string, id uint64) ([]byte, error) {
    offset := conn.objectOffset(id)
    logger.Tracef("Get block object %v on %v with size %v and offset %v\n", key, conn.device, conn.worker.ObjectSize, offset)

    contents := make([]byte, conn.worker.ObjectSize)
    remaining := conn.worker.ObjectSize
    start := 0

    for remaining > 0 {
        n, err := syscall.Pread(conn.fd, contents[start:], offset)
        if err != nil {
            return nil, err
        }

        start += n
        offset += int64(n)
        remaining -= uint64(n)
    }

    return contents, nil
}



