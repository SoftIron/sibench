package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"
import "github.com/ceph/go-ceph/rbd"


type RbdConnection struct {
    monitor string
    protocol ProtocolConfig
    worker WorkerConnectionConfig
    client *rados.Conn
    ioctx *rados.IOContext
    image *rbd.Image
}


func NewRbdConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*RbdConnection, error) {
    var conn RbdConnection
    conn.monitor = target
    conn.protocol = protocol
    conn.worker = worker
    return &conn, nil
}


func (conn *RbdConnection) Target() string {
    return conn.monitor
}


func (conn *RbdConnection) ManagerConnect() error {
    var err error
    conn.client, err = NewCephClient(conn.monitor, conn.protocol)
    if err != nil {
        return err
    }

    conn.ioctx, err = conn.client.OpenIOContext(conn.protocol["pool"])
    return err
}


func (conn *RbdConnection) ManagerClose() error {
    conn.ioctx.Destroy()
    conn.client.Shutdown()
    return nil
}


func (conn *RbdConnection) WorkerConnect() error {
    err := conn.ManagerConnect()
    if err != nil {
        return err
    }

    // The Manager just tested to make sure it could connect, and that the pool exists (so 
    // we can fail fast if there's a problem).  The workers have to create an RBD image to
    // use.  The connection protocol map know how much data we will be managing.

    imageSize := uint64((conn.worker.WorkerRangeEnd - conn.worker.WorkerRangeStart) * conn.worker.ObjectSize)
    imageName := fmt.Sprintf("sibench-%v-%v", conn.worker.Hostname, conn.worker.WorkerId)
    imageOrder := 22 // 1 << 22 gives a 4MB object size

    logger.Infof("Creating rbd image - name: %v, size: %v, order: %v\n", imageName, imageSize, imageOrder)

    conn.image, err = rbd.Create(conn.ioctx, imageName, imageSize, imageOrder)
    if err != nil {
        return err
    }

    openImage, err := rbd.OpenImage(conn.ioctx, imageName, "")
    if err != nil {
        conn.image.Remove()
        conn.image = nil
        return err
    }

    conn.image = openImage
    return nil
}


func (conn *RbdConnection) WorkerClose() error {
    if conn.image != nil {
        conn.image.Close()
        conn.image.Remove()
    }

    return conn.ManagerClose()
}


/* 
 * Helper function to determine an object's offset into the image from an object key 
 */
func (conn *RbdConnection) objectOffset(id uint64) int64 {
    return int64((id - conn.worker.WorkerRangeStart) * conn.worker.ObjectSize)
}


func (conn *RbdConnection) PutObject(key string, id uint64, contents []byte) error {
    logger.Tracef("Put rados object %v on %v: start\n", key, conn.monitor)

    offset := conn.objectOffset(id)
    _, err := conn.image.Seek(offset, rbd.SeekSet)
    if err != nil {
        return err
    }

    nwrite, err := conn.image.Write(contents)

    logger.Tracef("Put rados object %v on %v: end\n", key, conn.monitor)

    if err != nil {
        return err
    }

    if uint64(nwrite) != conn.worker.ObjectSize {
        return fmt.Errorf("Short write: expected %v bytes, but got %v", conn.worker.ObjectSize, nwrite)
    }

    err = conn.image.Flush()
    return err
}



func (conn *RbdConnection) GetObject(key string, id uint64) ([]byte, error) {
    offset := conn.objectOffset(id)
    _, err := conn.image.Seek(offset, rbd.SeekSet)
    if err != nil {
        return nil, err
    }

    buffer := make([]byte, conn.worker.ObjectSize)
    nread, err := conn.image.Read2(buffer, rbd.LIBRADOS_OP_FLAG_FADVISE_NOCACHE)


    if err != nil {
        return nil, err
    }

    if uint64(nread) != conn.worker.ObjectSize {
        return nil, fmt.Errorf("Short read: wanted %v bytes, but got %v", conn.worker.ObjectSize, nread)
    }

    return buffer, nil
}


