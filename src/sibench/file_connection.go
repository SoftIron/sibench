// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main


import "path/filepath"
import "fmt"
import "logger"
import "os"


/*
 * FileConnection is the connection to use when talking to a filesystem that is already locally mounted.
 *
 * FileConnectionBase is the code that is used to talk to a filesystem after it has been set up.  It exists
 * because things like CephFSConnection do the mounting automatically, and then use the FileConnectionBase
 * to do the work.
 */
type FileConnection struct {
    FileConnectionBase
}


func NewFileConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*FileConnection, error) {
    var conn FileConnection
    conn.InitFileConnectionBase(".", target)
    return &conn, nil
}


func (conn *FileConnection) Target() string {
    path := filepath.Join(conn.root, conn.dir)
    return path
}


func (conn *FileConnection) ManagerConnect() error {
    return nil
}


func (conn *FileConnection) ManagerClose() error {
    return nil
}


func (conn *FileConnection) WorkerConnect() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("Creating file connection to %v\n", path)

    // Check the directory is exists.
    info, err := os.Stat(path);
    if err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("FileConnection unable to start - directory does not exist: %v", path)
        }

        return fmt.Errorf("FileConnection Unable to start - can't stat directory %v: %v", path, err)
    }

    if !info.Mode().IsDir() {
        return fmt.Errorf("FileConnection unable to start - not a directory: %v", path)
    }

    return nil
}


func (conn *FileConnection) WorkerClose() error {
    path := filepath.Join(conn.root, conn.dir)
    logger.Infof("Closing file connection to %v\n", path)
    return nil
}


