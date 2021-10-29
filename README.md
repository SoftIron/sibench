# sibench

Sibench is a tool for benchmarking ceph clusters (and other file systems).  It was written as an alternative to Intel's Cosbench, after
experiencing numerous difficulties with that software.

Currently sibench supports the following protocols:
* S3
* Rados
* RBD
* CephFS
* Block devices
* Filesystems - anything locally mounted on the drivers

It has the scaffolding in place for filesystem-based protocols, so we should soon be adding support for:
* Samba
* NFS

Sibench is designed to run as a set of server daemons, running on separate host machines, which are controlled by a single manager process.  The
manager requires very few resources to run, and so is usually run on the same box as one of the servers.

The manager process is not a daemon: it is a simple process that is started, runs a benchmark and then quits.  It's possible that we may extend it
to run as a daemon with a job queue (in the manner of Cosbench), but that depends on how, or if, we dedide to make it part of the management console.

Whilst sibench can be run directly, it is more usual to use the benchmaster application to drive it.  (See https://git.softiron.com:9987/benchmarking/benchmaster for more details), since that adds the ability to push results to google sheets, run sweeps across parameter ranges and so forth.  (It also allows the use of different backends, and can drive Cosbench too).  It also manages creation of users, keys and so forth.

## ISCSI

ISCSI can be benchmarked using the block device option.  Each server will need to have its own ISCSI image mounted as block device, each using the same name (most easily accomplished using a link from, say, /tmp/sibench-iscsi to /dev/dm-0, or wherever it was mounted by multipathd).  

The easiest way to accomplish this is to use benchmaster, since that can setup and teardown iscsi mounts (including creating the RBD images to back them, and handling the multipath stuff for you).

## Starting the servers

The servers are started by systemd, but can be manually run with `sibench server`

## Start the manager

As mentioned above, the manager is usually invoked by benchmaster, rather than running it directly.

## Tracking down problems

If things aren't working, and it's not immediately obvious why, stop the daemon on one of the servers by running `systemctl stop sibench` and then start the server manually with `sibench server -v trace` which will generate a LOT of debug output.  The `-v trace` option can also be added to the mananger command line in case the issue is there.

(Note that the manager often does a connect to the cluster before the servers - to do things like create an S3 bucket - and so it is quite likely that things like authentication errors show up there first).

## Generators

Generators are responsible for generating the workloads for benchmarking.  There are currently two of them:

### PRNG Generator

This is the default option.  The PRNG generator creates objects with random content (based on a very cheap PRNG).

### Slice Geneerator

The PRNG generator is fine for just about all circumstances except when testing compression; random data is not compressible!  If we want to test compression, then we need to use the slice generator.

The SliceGenerator builds workloads from existing files.  It aims to reproduce the compressibility of those files, whilst still creating an effectively
infinite supply of different objects.

It works by taking a directory of files (which should all be of the same type: source code,
VM images, movies, or whatever), and then loading fixed sized slices of bytes
from random positions within those files.  The end result is that we have a library of (say) 1000
slices, each containing (say) 4Kb of data.  (Both of those values are run-time arguments).

When the slice generator is asked for a workload object of a given size, it starts concatenating randomly chosen slices together until it reaches the correct size.  If all the files in the directory are of the same type, This tends to give a very close approximation - in terms of compressibility - to the much larger set of suitable files we would need to ensure object uniqueness if we just used the files directly.  This also means we do not need to worry about holding a massive workload in memory.

Note that all of the sibench driver nodes need to have this directory in the same location.

Also, the drivers do NOT need to have the same files in their slice directory, though it's likely they will.  (In fact, you could mount the same NFS dir on all teh drivers.  Performance isn't really a consideration, since loading the slices is done before the benchmark proper begins, and so will not affect the numbers).
