package main

import "fmt"
import "logger"
import "os"
import "path/filepath"
import "syscall"



type CephFSConnection struct {
    FileConnection
    monitor string
    mountPoint string
}


func NewCephFSConnection(monitor string, port uint16, credentialMap map[string]string) (*CephFSConnection, error) {
    var conn CephFSConnection
    conn.monitor = monitor
    conn.mountPoint = filepath.Join(config.MountsDir, monitor)

    logger.Infof("Creating cephfs connection to %v in %v as %v\n", monitor, conn.mountPoint, credentialMap["username"])

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
        options := fmt.Sprintf("name=%v,secret=%v", credentialMap["username"], credentialMap["key"])
        logger.Debugf("CephFSConnection mounting: %v\n", options)

        err = syscall.Mount(monitor + ":/", conn.mountPoint, "ceph", 0, options)
        if err != nil {
            mountManager.MountComplete(conn.mountPoint, false)
            return nil, err
        }

        mountManager.MountComplete(conn.mountPoint, true)
    }

    // Tell our FileConnection delegate which directory to use as its file root.
    conn.InitFileConnection(conn.mountPoint)
    return &conn, nil
}


func (conn *CephFSConnection) Target() string {
    return conn.monitor
}



func (conn *CephFSConnection) Close() {
    logger.Infof("Closing cephfs connection to %v\n", conn.Target())

    if mountManager.Release(conn.mountPoint) {
        logger.Debugf("Unmounting %v\n", conn.mountPoint)
        syscall.Unmount(conn.mountPoint, 0)
        mountManager.UnmountComplete(conn.mountPoint)
    }
}
