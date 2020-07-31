package main

import "logger"
import "sync"



/* Possible states for a mount to be in */
type mountState int
const (
    MS_Init mountState = iota
    MS_Mounting
    MS_Mounted
    MS_Unmounting
)


/* State relating to one particular mount. */
type mountInfo struct {
    state mountState
    count int
    cond *sync.Cond
}


/*
 * The MountManager is a singleton used to manage access to mounts.
 * It does not do the work of actually mounting and unmounting.  That is 
 * done by Connection objects, usually controlled by Workers, since those
 * are the things that know how to mount CephFS, iSCSI or whatever.
 *
 * Instead, the MountManager's job is to provide synchronisation between those 
 * Connections so that a particular mount is only created once.
 */
type MountManager struct {
    /* Mutex for protecting access to our mount map */
    mutex sync.Mutex
    mounts map[string]*mountInfo
}


/* Singleton instance that everyone can share. */
var mountManager MountManager


/* 
 * Helper function to manage locking the mount info map. 
 */
func (m *MountManager) getMountInfo(mountPoint string) *mountInfo {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    if m.mounts == nil {
        m.mounts = make(map[string]*mountInfo)
    }

    mi, ok := m.mounts[mountPoint]
    if ok {
        return mi
    }

    // We've never been asked about this mount point before, so create it.

    lock := sync.Mutex{}
    cond := sync.NewCond(&lock)
    mi = &mountInfo{state: MS_Init, count: 0, cond: cond}
    m.mounts[mountPoint] = mi
    return mi
}


/* 
 * Tell the mount manager that we want to use a mount point.
 * 
 * If the mount point does not yet exist, then we return true.  The caller
 * should then create the mount point (in whatever way it wants) and then
 * call mountComplete to unblock anyone that is waiting for it to be come
 * available.
 *
 * If the mount point alreadys exists, then we return false.  The caller 
 * should just go ahead and use it.
 *
 * If the mount point has been acquired before, but is not yet ready, then
 * we block until it is either ready or failed.
 */
func (m *MountManager) Acquire(mountPoint string) bool {
    mi := m.getMountInfo(mountPoint)

    mi.cond.L.Lock()
    defer mi.cond.L.Unlock()

    for {
        switch mi.state {
            case MS_Init:
                mi.count++
                mi.state = MS_Mounting
                logger.Debugf("MountManager: mountpoint %v moving from Init to Mounting\n", mountPoint)
                return true

            case MS_Mounted:
                mi.count++
                logger.Debugf("MountManager: reusing %v with count %v\n", mountPoint, mi.count)
                return false

            default:
                mi.cond.Wait()
        }
    }
}


/*
 * Tell the manager that we have tried to mount, and whether we succeeded or not.
 */
func (m *MountManager) MountComplete(mountPoint string, success bool) {
    logger.Debugf("MountManager: mount completed: %v, success: %v\n", mountPoint, success)

    mi := m.getMountInfo(mountPoint)
    mi.cond.L.Lock()

    if success {
        mi.state = MS_Mounted
    } else {
        mi.count--;
        mi.state = MS_Init
    }

    mi.cond.L.Unlock()
    mi.cond.Broadcast()
}


/*
 * Tell the mount manager that we are done with a mount point.
 *
 * If this is the last caller to release it, then we return true, and 
 * the caller should unmount it and then call UnmountComplete.
 *
 * If there are other callers still using it then we return false.
 */
func (m *MountManager) Release(mountPoint string) bool {
    mi := m.getMountInfo(mountPoint)

    mi.cond.L.Lock()
    defer mi.cond.L.Unlock()

    if mi.state != MS_Mounted {
        panic("Bad state for Release")
    }

    mi.count--

    if mi.count == 0 {
        logger.Debugf("MountManager: mount released: %v, now unmounting\n", mountPoint)
        mi.state = MS_Unmounting
        return true
    }

    logger.Debugf("MountManager: mount released: %v, count still %v\n", mountPoint, mi.count)
    return false
}


/*
 * Tell the maanager that we have unmounted.
 */
func (m *MountManager) UnmountComplete(mountPoint string) {
    mi := m.getMountInfo(mountPoint)
    mi.cond.L.Lock()

    if mi.state != MS_Unmounting {
        panic("Bad state for UnmountComplete")
    }

    mi.state = MS_Init
    mi.cond.L.Unlock()
    mi.cond.Broadcast()
}
