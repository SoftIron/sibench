// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "fmt"
import "logger"
import "net"
import "os"
import "path/filepath"


/* 
 * A Connection for testing CephFS
 */
type CephFSConnection struct {
    FileConnectionBase
    protocol ProtocolConfig
    worker WorkerConnectionConfig
    monitor string
    mountPoint string
}


func NewCephFSConnection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*CephFSConnection, error) {
    var conn CephFSConnection
    conn.protocol = protocol
    conn.worker = worker
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

    // This bit looks rather odd: we close our underlying CephFS connection after creating the directory,
    // rather than just waiting until ManagerClose() is called.
    // The reason we do this is to avoid maintaining multiple CephFS mounts in the kernel. 
    // (Not that there's necessarily a problem in doing that, but let's keep things as lightweight
    // as we can).

    err1 := conn.CreateDirectory()
    err2 := conn.WorkerClose(false)
    if err1 != nil {
        return err1
    }

    return err2
}


func (conn *CephFSConnection) ManagerClose(cleanup bool) error {
    err := conn.WorkerConnect()
    if err != nil {
        return err
    }

    if cleanup {
        err = conn.DeleteDirectory()
    }

    err2 := conn.WorkerClose(cleanup)
    if err != nil {
        return err2
    }

    return err
}


func (conn *CephFSConnection) WorkerConnect() error {
    logger.Infof("Creating cephfs connection to %v in %v as %v\n", conn.monitor, conn.mountPoint, conn.protocol["username"])

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

        // The Mount system call can't handle names, so do a lookup first.

        monitor_ips, err := net.LookupHost(conn.monitor)
        if err != nil {
            logger.Errorf("Failure resolving %v: %v\n", conn.monitor, err)
            mountManager.MountComplete(conn.mountPoint, false)
            return err
        }

        // Now do the actual mount

        options := fmt.Sprintf("name=%v,secret=%v", conn.protocol["username"], conn.protocol["key"])
        logger.Debugf("CephFSConnection mounting with monitor: %v, mountpoint: %v, options: %v\n", monitor_ips[0], conn.mountPoint, options)

        err = Mount(monitor_ips[0] + ":/", conn.mountPoint, "ceph", 0, options)
        if err != nil {
            logger.Errorf("Failure mounting CephFS: %v\n", err)
            mountManager.MountComplete(conn.mountPoint, false)
            return err
        }

        mountManager.MountComplete(conn.mountPoint, true)
    }

    // Tell our FileConnection delegate which directories to use for its root and its dir within that root.
    conn.InitFileConnectionBase(conn.mountPoint, conn.protocol["dir"])
    return nil
}


func (conn *CephFSConnection) WorkerClose(cleanup bool) error {
    logger.Infof("Closing cephfs connection to %v\n", conn.monitor)

    if mountManager.Release(conn.mountPoint) {
        logger.Debugf("Unmounting %v\n", conn.mountPoint)
        Unmount(conn.mountPoint, 0)
        mountManager.UnmountComplete(conn.mountPoint)
    }

    return nil
}

