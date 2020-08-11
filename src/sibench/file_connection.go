package main


import "path/filepath"
import "logger"
import "os"
import "syscall"


/* 
 * An abstract connection backed by a local file system.  
 *
 * It is initialised with a root: a directory under which all operations take place.  This
 * is likely to be the directory where a remote filesystem has been mounted (where the 
 * remote filesystem is backed by the cluster under test), but it could be any dir really.
 *
 * FileConnection is not intented to be used directly, but wrapped in a parent Connection
 * that know how to create and tear-down the mount (such as CephFSConnection).   As such
 * it doesn't have the ususal connection constructor, or a Target() function.
 */
type FileConnection struct {
    root string
    dir string
}


func (conn *FileConnection) InitFileConnection(root string, dir string) {
    logger.Debugf("Initialising file connection on %v with dir %v\n", root, dir)
    conn.root = root
    conn.dir = dir
}


func (conn *FileConnection) CreateDirectory() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("FileConnection creating directory: %v\n", path)
    return os.MkdirAll(path, 0644)
}


func (conn *FileConnection) DeleteDirectory() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("FileConnection deleting directory: %v\n", path)
    return os.RemoveAll(path)
}


func (conn *FileConnection) PutObject(key string, contents []byte) error {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := syscall.Open(filename, syscall.O_WRONLY | syscall.O_CREAT | syscall.O_TRUNC | syscall.O_DIRECT | syscall.O_SYNC, 0644)
    if err != nil {
        return err
    }

    defer syscall.Close(fd)

    for len(contents) > 0 {
        n, err := syscall.Write(fd, contents)
        if err == nil {
            return err
        }

        contents = contents[n:]
    }

    return nil
}


func (conn *FileConnection) GetObject(key string) ([]byte, error) {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := syscall.Open(filename, syscall.O_RDONLY | syscall.O_DIRECT | syscall.O_SYNC, 0644)
    if err != nil {
        return nil, err
    }

    defer syscall.Close(fd)

    var stat syscall.Stat_t
    err = syscall.Fstat(fd, &stat)
    if err != nil {
        return nil, err
    }

    contents := make([]byte, stat.Size)
    remaining := stat.Size
    start := 0

    for remaining > 0 {
        n, err := syscall.Read(fd, contents[start:])
        if err != nil {
            return nil, err
        }

        start += n
        remaining -= int64(n)
    }

    return contents, err
}
