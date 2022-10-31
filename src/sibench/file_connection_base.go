// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main


import "path/filepath"
import "fmt"
import "logger"
import "os"
import "strings"
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
    dirsCreated []string
}


func (conn *FileConnectionBase) InitFileConnectionBase(root string, dir string) {
    logger.Debugf("Initialising file connection on %v with dir %v\n", root, dir)
    conn.root = root
    conn.dir = dir
}


func dirExists(path string) (bool, error) {
    fi, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) { return false, nil }
        return true, err
    }

    if !fi.IsDir() {
        return true, fmt.Errorf("% already exists, but is not a directory", path)
    }

    return true, nil
}


/*
 * Create all the directories in the path (in case we have been given a nested dir)
 * We remember which directories we have created in case we are asked to clean them
 * up again.
 */
func (conn *FileConnectionBase) CreateDirectories() error {
    dirs := strings.Split(filepath.Clean(conn.dir), string(os.PathSeparator))
    path := conn.root

    for _, d := range dirs {
        path = filepath.Join(path, d)
        exists, err := dirExists(path)
        if err != nil { return err }

        if !exists {
            err = os.Mkdir(path, 0644)
            if err != nil { return err }

            logger.Infof("Created dir: %v\n", path)
            conn.dirsCreated = append([]string{path}, conn.dirsCreated...)
        }
    }

    return nil
}


func (conn *FileConnectionBase) DeleteDirectories() error {

    for _, d := range conn.dirsCreated {
        logger.Infof("FileConnectionBase deleting directory: %v\n", d)
        err := os.Remove(d)
        if err != nil { return err }
    }

    return nil
}


func (conn *FileConnectionBase) RequiresKey() bool {
    return true
}


func (conn *FileConnectionBase) CanDelete() bool {
    return true
}


func (conn *FileConnectionBase) PutObject(key string, id uint64, buffer []byte) error {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := Open(filename, syscall.O_WRONLY | syscall.O_CREAT | syscall.O_TRUNC, 0644)
    if err != nil {
        return err
    }

    defer fd.Close()

    for len(buffer) > 0 {
        n, err := fd.Write(buffer)
        if err == nil {
            return err
        }

        buffer = buffer[n:]
    }

    return nil
}


func (conn *FileConnectionBase) GetObject(key string, id uint64, buffer []byte) error {
    filename := filepath.Join(conn.root, conn.dir, key)

    fd, err := Open(filename, syscall.O_RDONLY, 0644)
    if err != nil {
        return err
    }

    defer fd.Close()

    remaining, err := fd.Size()
    if err != nil {
        return err
    }

    if int64(cap(buffer)) != remaining {
        return fmt.Errorf("File has wrong size: expected %v, but got %v", cap(buffer), remaining)
    }

    start := 0

    for remaining > 0 {
        n, err := fd.Read(buffer[start:])
        if err != nil {
            return err
        }

        start += n
        remaining -= int64(n)
    }

    return nil
}


func (conn *FileConnectionBase) DeleteObject(key string, id uint64) error {
    filename := filepath.Join(conn.root, conn.dir, key)
    return os.Remove(filename)
}


func (conn *FileConnectionBase) InvalidateCache() error {
    return nil
}

