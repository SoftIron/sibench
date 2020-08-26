package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"
import "github.com/ceph/go-ceph/rbd"
import "strconv"


type RbdConnection struct {
    monitor string
    config ConnectionConfig
    client *rados.Conn
    ioctx *rados.IOContext
    image *rbd.Image
    objectSize int64
    objectMap map[string]int64
    nextIndex int64
}


func NewRbdConnection(target string, config ConnectionConfig) (*RbdConnection, error) {
    var conn RbdConnection
    conn.monitor = target
    conn.config = config
    conn.objectMap = make(map[string]int64)
    return &conn, nil
}


func (conn *RbdConnection) Target() string {
    return conn.monitor
}


func (conn *RbdConnection) ManagerConnect() error {
    var err error
    conn.client, err = NewCephClient(conn.monitor, conn.config)
    if err != nil {
        return err
    }

    conn.ioctx, err = conn.client.OpenIOContext(conn.config["pool"])
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
    // use.  The connection config map know how much data we will be managing.

    conn.objectSize, err = strconv.ParseInt(conn.config["object_size"], 10, 64)
    if err != nil {
        return nil
    }

    dataSize, err := strconv.ParseInt(conn.config["total_data_size"], 10, 64)
    if err != nil {
        return err
    }

    imageSize := uint64(dataSize)
    imageName := fmt.Sprintf("sibench-%v-%v", conn.config["hostname"], conn.config["worker_id"])
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
func (conn *RbdConnection) objectOffset(key string) int64 {
    index, ok := conn.objectMap[key]
    if !ok {
        index = conn.nextIndex
        conn.objectMap[key] = index
        conn.nextIndex++
    }

    return index * conn.objectSize
}


func (conn *RbdConnection) PutObject(key string, contents []byte) error {
    logger.Tracef("Put rados object %v on %v: start\n", key, conn.monitor)

    offset := conn.objectOffset(key)
    _, err := conn.image.Seek(offset, rbd.SeekSet)
    if err != nil {
        return err
    }

    nwrite, err := conn.image.Write(contents)

    logger.Tracef("Put rados object %v on %v: end\n", key, conn.monitor)

    if err != nil {
        return err
    }

    if int64(nwrite) != conn.objectSize {
        return fmt.Errorf("Short write: expected %v bytes, but got %v", conn.objectSize, nwrite)
    }

    err = conn.image.Flush()
    return err
}



func (conn *RbdConnection) GetObject(key string) ([]byte, error) {
    offset := conn.objectOffset(key)
    _, err := conn.image.Seek(offset, rbd.SeekSet)
    if err != nil {
        return nil, err
    }

    buffer := make([]byte, conn.objectSize)
    nread, err := conn.image.Read(buffer)


    if err != nil {
        return nil, err
    }

    if int64(nread) != conn.objectSize {
        return nil, fmt.Errorf("Short read: wanted %v bytes, but got %v", conn.objectSize, nread)
    }

    return buffer, nil
}


