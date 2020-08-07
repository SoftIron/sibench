package main

import "fmt"
import "logger"
import "os"
import "path/filepath"
import "strconv"
import "sync"
import "syscall"




var rbdIndex uint
var rbdLock sync.Mutex


/*
 * Most file-based protocols can share the same mountpoint between multiple connections, but 
 * RBD uses a different mount for each.  In consequence, we need a unique ID for each connection
 * so that we can make each mountpoint unique.
 */
func getNextIndex() uint {
    rbdLock.Lock()
    defer rbdLock.Unlock()
    rbdIndex++
    return rbdIndex
}



type RBDConnection struct {
    FileConnection
    monitor string
    mountPoint string
}


func NewRBDConnection(monitor string, config ConnectionConfig) (*RBDConnection, error) {
    var conn RBDConnection
    conn.monitor = monitor
    conn.mountPoint = filepath.Join(globalConfig.MountsDir, strconv.Itoa(int(getNextIndex())), monitor)

    logger.Infof("Creating RBD connection to %v in %v as %v\n", monitor, conn.mountPoint, config["username"])

    if mountManager.Acquire(conn.mountPoint) {
        // The mount doesn't exist yet, and we've been told to create it.

        // First ensure our mount point exists
        _, err := os.Stat(conn.mountPoint)
	    if os.IsNotExist(err) {
		    err = os.MkdirAll(conn.mountPoint, 0755)
		    if err != nil {
                logger.Errorf("Unable to create mount point %v: %v\n", conn.mountPoint, err)
                mountManager.MountComplete(conn.mountPoint, false)
                return nil, err
		    }
	    }

        // Now do the actual mount
        options := fmt.Sprintf("name=%v,secret=%v", config["username"], config["key"])
        logger.Debugf("RBDConnection mounting: %v\n", options)

        err = syscall.Mount(monitor + ":/", conn.mountPoint, "ceph", 0, options)
        if err != nil {
            mountManager.MountComplete(conn.mountPoint, false)
            return nil, err
        }

        mountManager.MountComplete(conn.mountPoint, true)
    }

    // Tell our FileConnection delegate which directory to use as its file root.
    conn.InitFileConnection(conn.mountPoint, config["dir"])
    return &conn, nil
}


func (conn *RBDConnection) Target() string {
    return conn.monitor
}


func (conn *RBDConnection) ManagerConnect() error {
    return nil
}


func (conn *RBDConnection) ManagerClose() error {
    return nil
}


func (conn *RBDConnection) WorkerConnect() error {
    return nil
}


func (conn *RBDConnection) WorkerClose() error {
    return nil
}
