// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

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
 * FileConnectionBase is not intented to be used directly, but wrapped in a parent Connection
 * that knows how to create and tear-down the mount (such as CephFSConnection).   As such
 * it doesn't have the ususal connection constructor, or a Target() function.
 */
type FileConnectionBase struct {
    root string
    dir string
}


func (conn *FileConnectionBase) InitFileConnectionBase(root string, dir string) {
    logger.Debugf("Initialising file connection on %v with dir %v\n", root, dir)
    conn.root = root
    conn.dir = dir
}


func (conn *FileConnectionBase) CreateDirectory() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("FileConnectionBase creating directory: %v\n", path)
    return os.MkdirAll(path, 0644)
}


func (conn *FileConnectionBase) DeleteDirectory() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("FileConnectionBase deleting directory: %v\n", path)
    return os.RemoveAll(path)
}


func (conn *FileConnectionBase) PutObject(key string, id uint64, contents []byte) error {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := Open(filename, syscall.O_WRONLY | syscall.O_CREAT | syscall.O_TRUNC, 0644)
    if err != nil {
        return err
    }

    defer fd.Close()

    for len(contents) > 0 {
        n, err := fd.Write(contents)
        if err == nil {
            return err
        }

        contents = contents[n:]
    }

    return nil
}


func (conn *FileConnectionBase) GetObject(key string, id uint64) ([]byte, error) {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := Open(filename, syscall.O_RDONLY, 0644)
    if err != nil {
        return nil, err
    }

    defer fd.Close()

    remaining, err := fd.Size()
    if err != nil {
        return nil, err
    }

    contents := make([]byte, remaining)
    start := 0

    for remaining > 0 {
        n, err := fd.Read(contents[start:])
        if err != nil {
            return nil, err
        }

        start += n
        remaining -= int64(n)
    }

    return contents, err
}


func (conn *FileConnectionBase) InvalidateCache() error {
    return nil
}

