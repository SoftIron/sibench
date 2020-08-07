package main

import "fmt"
import "logger"
import "os"
import "path/filepath"
import "syscall"



type CephFSConnection struct {
    FileConnection
    config ConnectionConfig
    monitor string
    mountPoint string
}


func NewCephFSConnection(target string, config ConnectionConfig) (*CephFSConnection, error) {
    var conn CephFSConnection
    conn.config = config
    conn.monitor = target
    conn.mountPoint = filepath.Join(globalConfig.MountsDir, target)
    return &conn, nil
}


func (conn *CephFSConnection) Target() string {
    return conn.monitor
}


func (conn *CephFSConnection) ManagerConnect() error {
    err := conn.WorkerConnect()
    if err != nil {
        return err
    }

    // This bit looks a bit odd: we close our underlying CephFS connection after creating the directory,
    // rather than just waiting until ManagerClose() is called.
    // The reason we do this is to avoid maintaining multiple CephFS mounts in the kernel. 
    // (Not that there's necessarily a problem in doing that, but let's keep things as lightweight
    // as we can).

    err1 := conn.CreateDirectory()
    err2 := conn.WorkerClose()
    if err1 != nil {
        return err1
    }

    return err2
}


func (conn *CephFSConnection) ManagerClose() error {
    err := conn.WorkerConnect()
    if err != nil {
        return err
    }

    err1 := conn.DeleteDirectory()
    err2 := conn.WorkerClose()
    if err1 != nil {
        return err1
    }

    return err2
}


func (conn *CephFSConnection) WorkerConnect() error {
    logger.Infof("Creating cephfs connection to %v in %v as %v\n", conn.monitor, conn.mountPoint, conn.config["username"])

    if mountManager.Acquire(conn.mountPoint) {
        // The mount doesn't exist yet, and we've been told to create it.

        // First ensure our mount point exists
        _, err := os.Stat(conn.mountPoint)
	    if os.IsNotExist(err) {
		    err = os.MkdirAll(conn.mountPoint, 0755)
		    if err != nil {
                logger.Errorf("Unable to create mount point %v: %v\n", conn.mountPoint, err)
                mountManager.MountComplete(conn.mountPoint, false)
                return err
		    }
	    }

        // Now do the actual mount
        options := fmt.Sprintf("name=%v,secret=%v", conn.config["username"], conn.config["key"])
        logger.Debugf("CephFSConnection mounting: %v\n", options)

        err = syscall.Mount(conn.monitor + ":/", conn.mountPoint, "ceph", 0, options)
        if err != nil {
            mountManager.MountComplete(conn.mountPoint, false)
            return err
        }

        mountManager.MountComplete(conn.mountPoint, true)
    }

    // Tell our FileConnection delegate which directories to use for its root and its dir within that root.
    conn.InitFileConnection(conn.mountPoint, conn.config["dir"])
    return nil
}


func (conn *CephFSConnection) WorkerClose() error {
    logger.Infof("Closing cephfs connection to %v\n", conn.monitor)

    if mountManager.Release(conn.mountPoint) {
        logger.Debugf("Unmounting %v\n", conn.mountPoint)
        syscall.Unmount(conn.mountPoint, 0)
        mountManager.UnmountComplete(conn.mountPoint)
    }

    return nil
}

