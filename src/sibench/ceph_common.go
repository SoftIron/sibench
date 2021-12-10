// +build linux

package main

import "fmt"
import "logger"
import "github.com/ceph/go-ceph/rados"



/*
 * Helper function to open a new low-level Ceph connection used for both rados and rbd.
 * Pulls information from the ConnectionConfig about username, key, pool and so forth.
 * Enables ceph debug logging if our logger is set to trace mode.
 * 
 * Note that this is NOT a connection in the sibench sense of the term.  RadosConnection
 * and RbdConnection both use this low-level connection to provide the sibench connection
 * functionality.
 */
func NewCephClient(monitor string, config ProtocolConfig) (*rados.Conn, error) {
    client, err := rados.NewConnWithUser(config["username"])
    if err != nil {
        return nil, err
    }

    err = client.SetConfigOption("mon_host", monitor)
    if err != nil {
        return nil, err
    }

    err = client.SetConfigOption("key", config["key"])
    if err != nil {
        return nil, err
    }

    if logger.IsTrace() {
        err = client.SetConfigOption("debug_rados", "20")
        if err != nil {
            return nil, err
        }

        err = client.SetConfigOption("debug_objecter", "20")
        if err != nil {
            return nil, err
        }

        err = client.SetConfigOption("log_to_stderr", "true")
        if err != nil {
            return nil, err
        }
    }

    logger.Infof("Creating rados client to %v as user %v\n", monitor, config["username"])

    err = client.Connect()
    if err != nil {
        return nil, err
    }

    pool := config["pool"]

    // Check the pool we want exists so we can give a decent error message. 
    pools, err := client.ListPools()
    found := false
    for _, p := range pools {
        if p == pool {
            found = true
        }
    }

    if !found {
        client.Shutdown()
        return nil, fmt.Errorf("No such Ceph pool: %v\n", pool)
    }

    return client, nil
}

