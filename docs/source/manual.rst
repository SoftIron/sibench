Manual
======

.. include:: sibench-short-intro.txt

For more detailed information, visit https://sibench.io.

How It Works
------------

Each benchmarking node runs ``sibench`` as a server daemon, listening for incoming
connections from a ``sibench`` client.

Once the client starts a benchmark, the ``sibench`` servers send periodic summaries
back to it, so that the user can see progress.

When a benchmark completes, the ``sibench`` servers send the full results back to
the client.  A summary of the results is dislplayed by the client, but the full
results of every individual read and write operation are written out as a json
file in case the user wishes to perform their own statistical analysis.

Once a client starts running a benchmark on some set of ``sibench`` servers, those
servers will reject any other incoming connections until the benchmark completes
or is aborted.  There is no job queue or long-lived management process.

The client and server use the same binary, just with different command line
options.  The client is extremely lightweight, and may be run on one of the
server nodes without significantly impacting benchmarking performance, though
it may result in higher memory usage on that node.

Command Line
------------

The following is a list of all the commands that the sibench binary accepts:

**sibench -h | \-\-help**
  Outputs the full command line syntax.

**sibench version**
  Outputs the version number of the sibench binary.

**sibench server** [\-\-verbosity LEVEL] [\-\-port PORT] [\-\-mounts-dir DIR]
  Starts sibench as a server.

**sibench s3 run** [\-\-s3-port PORT] [\-\-s3-bucket BUCKET] (\-\-s3-access-key KEY) (\-\-s3-secret-key KEY) <target> ...
  Starts a benchmark using the S3 object protocol against the specified targets, which may S3 servers or RadosGateway nodes.

**sibench rados run** [\-\-ceph-pool POOL] [\-\-ceph-user USER] (\-\-ceph-key KEY) <target> ...
  Starts a benchmark using the Rados object protocol against the specified targets, which should be Ceph monitors.

**sibench cephfs run** [\-\-mounts-dir DIR] [\-\-ceph-dir DIR] [\-\-ceph-user USER] (\-\-ceph-key KEY) <target> ...
  Starts a benchmark using CephFS against the specified targets, which should be Ceph monitors.

**sibench rbd run** [\-\-ceph-pool POOL] [\-\-ceph-datapool POOL] [\-\-ceph-user USER] (\-\-ceph-key KEY) <target> ...
  Starts a benchmark using RBD against the specified targets, which should be Ceph monitors.

**sibench block run** [\-\-block-device DEVICE]
  Starts a benchmark using a locally mounted block device.

**sibench file run** [\-\-file-dir DIR]
  Starts a benchmark using a locally mounted filesystem.

Additional options **shared by all run commands**, omitted from above for clarity:

- [\-\-verbosity LEVEL]
- [\-\-port PORT]
- [\-\-object-size SIZE]
- [\-\-object-count COUNT]
- [\-\-ramp-up TIME]
- [\-\-run-time TIME]
- [\-\-ramp-down TIME]
- [\-\-read-write-mix MIX]
- [\-\-bandwidth BW]
- [\-\-output FILE]
- [\-\-workers FACTOR]
- [\-\-generator GEN]
- [\-\-slice-dir DIR]
- [\-\-slice-count COUNT]
- [\-\-slice-size BYTES]
- [\-\-skip-read-verification]
- [\-\-servers SERVERS]
- [\-\-use-bytes]
- [\-\-individual-stats]


Option Definitions
~~~~~~~~~~~~~~~~~~

+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| Long                           | Short  | Value     | Description                                                                             | Default            |
+================================+========+===========+=========================================================================================+====================+
| **\-\-help**                   | **-h** | \-        | Show full usage.                                                                        | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-verbosity**              | **-v** | *LEVEL*   | Set debugging output at level "off", "debug" or "trace".  The "trace" level may         |                    |
|                                |        |           | generate enough output to affect benchmark performance, and should only be used when    |                    |
|                                |        |           | trying to track down issues.                                                            | off                |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-port**                   | **-p** | *PORT*    | The port on which ``sibench`` communicates.                                             | 5150               |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-object-size**            | **-s** | *SIZE*    | Object size to test, in units of K or M.                                                | 1M                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-object-count**           | **-c** | *COUNT*   | The total number of objects to use as our working set.                                  | 1000               |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ramp-up**                | **-u** | *TIME*    | The number of seconds at the start of each phase where we don't record data (to         | 5                  |
|                                |        |           | discount edge effects caused by new connections).                                       |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-run-time**               | **-r** | *TIME*    | The number of seconds in the middle on each phase of the benchmark where we             | 30                 |
|                                |        |           | do record the data.                                                                     |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ramp-down**              | **-d** | *TIME*    | The number of seconds at the end of each phase where we don't record data.              | 2                  |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-read-write-mix**         | **-x** | *MIX*     | The ratio between read and writes, specified as the percentage of reads.                | 0                  |
|                                |        |           | A value of zero indicates that reads and writes should be done in separate passes,      |                    |
|                                |        |           | rather than being combined.                                                             |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-bandwidth**              | **-b** | *BW*      | Benchmark at a fixed bandwidth, in units of K, M or G bits/s                            | 0                  |
|                                |        |           | A value of zero indicates no limit.                                                     |                    |
|                                |        |           | When the read/write mix is not zero - that is, when we are not doing separate passes    |                    |
|                                |        |           | for read and write - then this is the bandwidth of the combined operations.             |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-output**                 | **-o** | *FILE*    | The file to which we write our json results.                                            |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-workers**                | **-w** | *FACTOR*  | Number of worker threads per server as a factor x number of CPU cores.                  | 1.0                |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-mounts-dir**             | **-m** | *DIR*     | The directory in which we should create any filesystem mounts that are performed by     | /tmp/sibench_mnt   |
|                                |        |           | ``sibench`` itself, such as when using CephFS.  It is not needed for running generic    |                    |
|                                |        |           | filesystem benchmarks, because those must be mounted outside of ``sibench``.            |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-generator**              | **-g** | *GEN*     | Which object generator to use: "prng" or "slice".                                       | prng               |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-skip-read-verification** |        | \-        | Disable validation on reads.  This should only be used to check if the number of nodes  | \-                 |
|                                |        |           | in the ``sibench`` cluster is a limiting factor when benchmarking read performance.     |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-servers**                |        | *SERVERS* | A comma-separated list of ``sibench`` servers to connect to.                            | localhost          |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-s3-port**                |        | *PORT*    | The port on which to connect to S3.                                                     | 7480               |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-s3-bucket**              |        | *BUCKET*  | The name of the bucket we wish to use for S3 operations.                                | sibench            |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-s3-access-key**          |        | *KEY*     | S3 access key.                                                                          | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-s3-secret-key**          |        | *KEY*     | S3 secret key.                                                                          | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ceph-pool**              |        | *POOL*    | The pool we use for benchmarking.                                                       | sibench            |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ceph-datapool**          |        | *POOL*    | Optional pool used for RBD.  If set, ceph-pool is used only for metadata.               | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ceph-user**              |        | *USER*    | The Ceph username we wish to use.                                                       | admin              |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ceph-key**               |        | *KEY*     | The CephX secret key belonging to the ceph user.                                        | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-ceph-dir**               |        | *DIR*     | The directory within CephFS that we should use for a benchmark.    This will be created | sibench            |
|                                |        |           | by ``sibench`` if it does not already exist.                                            |                    |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-block-device**           |        | *DEVICE*  | The local block device to use for a benchmark.                                          | /tmp/sibench_block |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-file-dir**               |        | *DIR*     | The local directory to use for file operations.  The directory must already exist.      | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-slice-dir**              |        | *DIR*     | The directory of files to be sliced up to form new workload objects.                    | \-                 |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-slice-count**            |        | *COUNT*   | The number of slices to construct for workload generation.                              | 10000              |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-slice-size**             |        | *BYTES*   | The size of each slice in bytes.                                                        | 4096               |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-use-bytes**              |        | \-        | Show bandwidth in Bytes                                                                 | off                |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-individual-stats**       |        | \-        | Record the individual stats in the output file.  This may be VERY big                   | off                |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+
| **\-\-clean-up**               |        | \-        | Delete the data at the end of the benchmark run                                         | off                |
+--------------------------------+--------+-----------+-----------------------------------------------------------------------------------------+--------------------+


Targets
~~~~~~~

The targets are the nodes to which the worker threads connect.  Each worker
opens a connection to each target and round-robins their reads and writes across
those connections.

For most Ceph operations, the targets are monitors, and there is no advantage to
specifying more than one.  All the monitors do is provide the
state-of-the-cluster map so that the workers can connect to the OSDs directly.

For RGW/S3, however, you should *definitely* list all of the storage cluster's
RGW nodes as targets, since those nodes are doing real work, and it needs to be
balanced.

RBD
~~~

RBD behaviour is a little different than you might expect: each worker creates
an RBD image per target, just big enough to hold that worker's share of the
'objects' for the benchmark.  All reads and writes that the worker then does are
within the RBD image.

For example, if you have the following:

1. 10 ``sibench`` nodes, each with 16 cores
2. A single target monitor
3. And object count of 1600 and an object size of 1MB

Then ``sibench`` will create 160 workers (by default, it is one per core), each of
which will create a single 10MB RBD image, and then it will proceed to read and
write 1 MB at a time to parts of that image.

Generators
~~~~~~~~~~

Generators create the data that ``sibench`` uses as workloads for the storage
system.  There are currently two of them, selectable with the ``--generator``
option.

PRNG Generator
""""""""""""""

The PRNG generator creates data which is entirely pseudorandom.  It requires no
configuration, and is the default choice.  However, it has one shortcoming:
because it creates pseudorandom data, it is not compressible.  If you wish to
test compression in your storage system, then you will need need to create a
compressible workload.  The same restriction applies to de-duplication
technologies.

Slice Generator
"""""""""""""""

The Slice generator builds workloads from existing files.  It aims to reproduce
the compressibility characteristics of those files, whilst still creating an
effectively infinite supply of different objects.

It works by taking a directory of files (which will usually be of the same type:
source code, VM images, movies, or whatever), and then loading fixed sized
slices of bytes from random positions within those files.  The end result is
that we have a library of (say) 1000 slices, each containing (say) 4Kb of data.
Both of those values may be set with command line options.

When asked to generate a new workload object the slice generator does the
following:

1.  Creates a random seed.
2.  Writes the seed into the start of the workload object.
3.  Uses the seed to create a random number generator  just for this workload object.
4.  Uses that random number generator  to select slices from our library, which are concatenated
    onto the object until we have as many bytes as we were asked for.

This approach means that we do not need to ever store the objects themselves: we
can verify a read operation by reading the seed from the first few bytes, and
then recreating the object we would expect.

Note that the directory of data to be sliced needs to be in the same location on
each of the ``sibench`` server nodes.

The drivers do *not* need to have the same files in their slice directories,
though it's likely that they will.  One option would be to mount the same NFS
share on all the drivers as a repository for the slice data.  Performance when
loading the slices is not a consideration, since it is done before the benchmark
begins, and so will not affect the numbers.

Write Cycles
~~~~~~~~~~~~

The ``count`` parameter determines how many objects we create.  However, for long
benchmarks runs, or for small counts or object sizes, we are likely to wrap
around and start writing from the first object again.  If this happens, ``sibench``
internally increments a cycle counter, which it uses to ensure that objects
written in different cycles will have different contents, even though the object
will still use the same key as previously.

The Prepare Phase
~~~~~~~~~~~~~~~~~

``sibench`` either benchmarks write operations first and then read operations, or
else it benchmarks a mixture of the too (depending on the ``--read-write-mix``
option.  When benchmarking reads, or a read-write mix, it must first ensure that
there are enough objects there to read before it can start work.  This is the
*prepare* phase, and that is what is happening when you see messages about
'Preparing'.

It also happens if we are doing separate writes and reads and we did not have a
long enough run time for ``sibench`` to write all of the objects specified by the
``object-count`` option.  In this case, the prepare phase will keep writing until
all the objects are ready for reading.


The Delete Phase
~~~~~~~~~~~~~~~~

``sibench`` does not clean up after itself by default, since Ceph can be very
slow at deleting objects.  However, if you wish to execute multiple runs over a 
weekend (perhaps by using Benchmaster to control sibench), then you may run the
risk of running out of storage space on the Ceph cluster.  In such cases, 
deleting the objects at the end of the run may be necessary.  You can enable
this by using the ``--clean-up`` flag.

Setting ``--clean-up`` behaves differently depending on the protocol, but in 
essence there are two parts to it: deleting the individual objects, and cleaning
up other resources.  Protocols may do either, neither or both.

In addition, the cleanup may be synchonous or not.  This is best illustrated
by comparing the behaviour or RADOS and RBD.

With RADOS, we can delete the individual objects, and we can do it synchronously
 - meaning that when sibench completes the run, Ceph will have deleted the objects
and will have no pending workload.

With RBD, we delete the RBD image synchronously, but under the hood, that image
is comprised of multiple objects, and Ceph does not delete them at once, but
adds them to a queue for later deletion.

Clealy asynchonous deletes are bad if we wish to run a set of benchmarks: when
the benchmark terminates, the Ceph cluster under test may still be deleting in
background, and thus degrading the performance of subsequent runs.

Sadly, there's nothing sibench can do to determine completion in such cases.

+----------+---------------+--------------------------------------------------+------------------------------------+
| Protocol | Object Delete | End Of Run Clean-up                              | Synchronous                        |
+==========+===============+==================================================+====================================+
| s3       | yes           | Deletes the bucket, but only if we created it    | yes                                |
+----------+---------------+--------------------------------------------------+------------------------------------+
| rados    | yes           | no                                               | yes                                |
+----------+---------------+--------------------------------------------------+------------------------------------+
| cephfs   | yes           | Deletes the directories only if we created them  | yes                                |
+----------+---------------+--------------------------------------------------+------------------------------------+
| rbd      | no            | Deletes the images                               | no                                 |
+----------+---------------+--------------------------------------------------+------------------------------------+
| block    | no            | no                                               | n/a                                |
+----------+---------------+--------------------------------------------------+------------------------------------+
| file     | yes           | no                                               | dependent on underlying filesystem |
+----------+---------------+--------------------------------------------------+------------------------------------+

Lastly, if you're not running a production cluster, then you can tell Ceph to 
delete more quickly (or more accurately, to insert smaller delays between delete
operations) by adding the following to your ceph config (and then restarting the
osd daemons).

::
osd_delete_sleep_hybrid = 0.001
osd_delete_sleep_hdd = 0.001
osd_delete_sleep_ssd = 0.001
