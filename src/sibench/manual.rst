==================
Sibench User Manal
==================

Overview
========

Sibench is a benchmarking tool for storage systems, and for Ceph in particular.  

Sibench typically runs on many nodes in parallel, in order to be able to scale up to the very high bandwidths of which modern scaleable storage systems are capable.  

It has support for file, block and object storage backends, using a variety of different protocols, which currently include:

- Rados: Ceph's native object protocol.
- RBD: Ceph's block protocol.
- CephFS: Ceph's POSIX filesystem protocol.
- S3: Amazon's object protocol, which is always provided by Ceph's RadosGateway.
- Local block storage
- Local file storage

These last two can be used to benchmark many other protocols - iSCSI, SMB, NFS and so on - provided that they are already mounted on each Sibench node.


Operation
=========

Each benchmarking node runs Sibench as a server daemon, listening for incoming connections from a Sibench client.  

Once a benchmark starts, the Sibench servers send periodic summaries back to the client, so that the user can see progress.

When a benchmark completes, the Sibench servers send the full results back to the client for analysis.  The analysis is output by the client,
but the full results of every individual read and write operation are written out as a json file in case the user wishes to perform their own statistical analysis.

Once a client starts running a benchmark on some set of Sibench servers, those servers will refuse any other incoming connections until the benchmark
completes or is aborted.  There is no job queue or long-lived management process.
  
The client and server use the same binary, just with different command line options.  The client is extremely lightweight, and may be run on one of the server nodes without significantly impacting benchmarking performance.

Command Line
------------

Commands
~~~~~~

The following is a list of all the command that the sibench binary accepts:

  **sibench -h | --help**
    Outputs the full command line syntax.

  **sibench version**
    Outputs the version number of the sibench binary.

  **sibench server** [-v LEVEL] [-p PORT] [-m DIR]
     Starts sibench as a server. 

  **sibench s3 run** [--s3-port PORT] [--s3-bucket BUCKET] (--s3-access-key KEY) (--s3-secret-key KEY) <targets> ...
     Starts a benchmark run using the S3 object protocol against the specified targets, which may RadosGateway nodes.

  **sibench rados run** [--ceph-pool POOL] [--ceph-user USER] (--ceph-key KEY) <targets> ...
     Starts a benchmark run using the Rados object protocol against the specified targets, which should be Ceph monitors.

  **sibench cephfs run** [-m DIR] [--ceph-dir DIR] [--ceph-user USER] (--ceph-key KEY) <targets> ...
   Starts a benchmark run using CephFS against the specified targets, which should be Ceph monitors.

  **sibench rbd run** [--ceph-pool POOL] [--ceph-datapool POOL] [--ceph-user USER] (--ceph-key KEY) <targets> ...
   Starts a benchmark run using RBD against the specified targets, which should be Ceph monitors.

  **sibench block run** [--block-device DEVICE]
   Starts a benchmark using a locally mounted block device.

  **sibench file run** [--file-dir DIR]
   Starts a benchmark using a locally mounted filesystem.

  Additional options shared by all **run** commands, omitted from above for clarity:

  - [-v LEVEL] 
  - [-p PORT] 
  - [-o FILE]
  - [-s SIZE] 
  - [-c COUNT] 
  - [-b BW] 
  - [-x MIX] 
  - [-u TIME] 
  - [-r TIME] 
  - [-d TIME] 
  - [-w FACTOR] 
  - [-g GEN] 
  - [--slice-dir DIR] 
  - [--slice-count COUNT] 
  - [--slice-size BYTES]
  - [--skip-read-verification]
  - [--servers SERVERS] 


Option Definitions
~~~~~~~~~~~~~~~~

  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | Long                         | Short  | Value     | Description                                                                             | Default            |
  +==============================+========+===========+=========================================================================================+====================+
  | **--help**                   | **-h** | \-        | Show full usage.                                                                        | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--verbosity**              | **-v** | *LEVEL*   | Set debugging output at level "off", "debug" or "trace".  The "trace" level may         |                    |
  |                              |        |           | generate enough output to affect benchamrk performance, and should only be used when    |                    |
  |                              |        |           | trying to track down issues.                                                            | off                |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--port**                   | **-p** | *PORT*    | The port on which sibench communicates.                                                 |  5150              |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--mounts-dir**             | **-m** | *DIR*     | The directory in which we should create any filesystem mounts that are performed by     | /tmp/sibench_mnt   |
  |                              |        |           | Sibench itself, such as when using CephFS.  It is not needed for running generic        |                    |
  |                              |        |           | filesystem benchmarks, because those must be mounted outside of sibench.                |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--size**                   | **-s** | *SIZE*    | Object size to test, in units of K or M.                                                | 1M                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--count**                  | **-c** | *COUNT*   | The total number of objects to use as our working set.                                  | 1000               |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ramp-up**                | **-u** | *TIME*    | The number of seconds at the start of each phase where we don't record data (to         | 5                  |
  |                              |        |           | discount edge effects caused by new connections).                                       |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--run-time**               | **-r** | *TIME*    | The number of seconds in the middle on each phase of the benchmark where we             | 30                 |
  |                              |        |           | do record the data.                                                                     |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ramp-down**              | **-d** | *TIME*    | The number of seconds at the end of each phase where we don't record data.              | 2                  |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--output**                 | **-o** | *FILE*    | The file to which we write our json results.                                            | sibench.json       |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--workers**                | **-w** | *FACTOR*  | Number of worker threads per server as a factor x number of CPU cores.                  | 1.0                |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--read-write-mix**         | **-x** | *MIX*     | The ratio between read and writes, specified as the percentage of reads.                | 0                  |
  |                              |        |           | A value of zero indicates that reads and writes should be done in separate passes,      |                    |
  |                              |        |           | rather than being combined.                                                             |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--bandwidth**              | **-b** | *BW*      | Benchmark at a fixed bandwidth, in units of K, M or G bits/s                            | 0                  |
  |                              |        |           | A value of zero indicates no limit.                                                     |                    |
  |                              |        |           | When the read/write mix is not zero - that is, when we are not doing separate passes    |                    |
  |                              |        |           | for read and write - then this is the bandwidth of the combined operations.             |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--generator**              | **-g** | *GEN*     | Which object generator to use: "prng" or "slice".                                       | prng               |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--skip-read-verification** |        | \-        | Disable validation on reads.  Should only be used to check if the number of nodes in    | \-                 |
  |                              |        |           | the Sibench cluster is a limiting factor in benchmark performance.                      |                    |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--servers**                |        | *SERVERS* | A comma-separated list of sibench servers to connect to.                                | localhost          |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--s3-port**                |        | *PORT*    | The port on which to connect to S3.                                                     | 7480               |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--s3-bucket**              |        | *BUCKET*  | The name of the bucket we wish to use for S3 operations.                                | sibench            |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--s3-access-key**          |        | *KEY*     | S3 access key.                                                                          | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--s3-secret-key**          |        | *KEY*     | S3 secret key.                                                                          | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ceph-pool**              |        | *POOL*    | The pool we use for benchmarking.                                                       | sibench            |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ceph-datapoolv**         |        | *POOL*    | Optional pool used for RBD.  If set, ceph-pool is for metadata.                         | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ceph-user**              |        | *USER*    | The ceph username we wish to use.                                                       | admin              |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ceph-key**               |        | *KEY*     | The secret key belonging to the ceph user.                                              | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--ceph-dir**               |        | *DIR*     | The directory within CephFS that we should use for a benchmark.                         | sibench            |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--block-device**           |        | *DEVICE*  | The block device to use for a benchmark.                                                | /tmp/sibench_block |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--file-dir**               |        | *DIR*     | The directory to use for file operations.  The directory must already exist.            | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--slice-dir**              |        | *DIR*     | The directory of files to be sliced up to form new workload objects.                    | \-                 |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--slice-count**            |        | *COUNT*   | The number of slices to construct for workload generation.                              | 10000              |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
  | **--slice-size**             |        | *BYTES*   | The size of each slice in bytes.                                                        | 4096               |
  +------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+

Slow Shutdown

There are times when sibench can take a long time when cleaning up after a benchmark run.  This is due to Ceph being extremely slow at deleting objects.



Best practices for benchmarking
===============================

Boosting Throughput
-------------------

Sibench is inefficient with respect to the amount of load it puts on its own nodes.  This is by design: we do not want to have to wait long for a thread to be scheduled in order to read data that has become available.  Nor do we want to be interrupted during a write. Both of these scenarios that can have a huge effect on the accuracy of our response time measurements, and may make them look much worse than they really are.  

As a consequence, a sibench node only starts up as many workers as we have cores.  This is adjustable using the ``--worker-factor`` option.  (A factor of 2.0 will have twice as many workers as cores).  This may be useful if we want to determine absolute maximum throughput, provided we don't care about response times.

Alternatively, you may also be able boost read throughput from the sibench nodes by using the ``--skip-read-verification`` option, which does exactly what it suggests.

In general though, neither of these two options are recommended except for one particular use case: if disabling read verification or increasing the worker count boosts your throughput numbers, then that is telling you that you need more sibench nodes in your cluster in order to benchmark at those rates whilst still giving accurate timings.


Memory Considerations
---------------------

Sibench is written to use as little memory as possible.  The generators algorithmically create each object to be written or read-and-verified on the fly, so objects do not need to be held in memory for longer than a single read or write operation.

Unfortunately, some of the ceph native libraries used by sibench do appear to hold on to data for longer periods of time.  This can result in large amounts of memory being used, which can result in two undesirable outcomes:

* Swapping: the benchmarking process needs to swap, then performance figures are likely to be wildly wrong.

* Process death: on Linux, the OOM Killer in the kernel will terminate sibench with a SIGKILL.  Since this is not a signal that sibench can catch, there is no warning or error when it occurs.  (The systemd script should start a new copy of the server immediately though, so the sibench cluster will be usable for a new benchmark run with no further action.

At the start of each run, Sibench determines how much physical memory each node has, and does some back-of-the-envelope maths to determine how much memory a benchmark may consume in the worst case.  If the latter is within about 80% of the former, it output a warning message to alert the user of possible consequences.   |


Cache Considerations
--------------------

