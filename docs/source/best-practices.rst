Best practices
==============

Cache considerations
--------------------

When doing read operations, it is vital that your working set is large enough
that the storage backend cannot fulfil requests from cache - unless of course,
cache performance  is what you are trying to benchmark!  

The object `size` and `count` parameters determine your working set.  For 
example, if you have 10,000 objects of 1M size, then your working set will be
10 GB.

Exactly how big your working set needs to be is dependent on the storage system
under test, and may be difficult to determine.  For instance, when benchmarking
Rados, we would need to consider not only Ceph's own cache sizes, but also the
combined amount of cache built into all the drives in the system.

When in doubt, use a bigger object count.  The only downsides to using a larger
count are the possibility of running out of memory on the Sibench nodes
themselves, and the increased amount of time it will take to clean up after the
benchmark.

We regularly use working sets measured in terabytes when benchamarking medium
sized clusters.

Throughput isn't everything!
----------------------------

Storage systems are not usually run at peak throughput because it can lead to
extremely long response times.  In consequence, running without bandwidth
limiting is only giving half the story: it'll tell you what the maximum
throughput in the system might be, but it is likely to be very misleading about
the response times that the storage system is likely to give in real-world use.

More useful figures can often be obtained by *first* determining the peak
throughput of the system, and *then* re-running the benchmarks with the
bandwidth limited to 80 or 90 percent of the peak number.

Boosting throughput
-------------------

Sibench is inefficient with respect to the amount of load it puts on its own
nodes.  This is by design: we do not want to have to wait long for a thread to
be scheduled in order to read data that has become available.  Nor do we want to
be interrupted during a write. Both of these scenarios can have a huge effect on
the accuracy of our response time measurements, and may make them look much
worse than they really are.  In essence, we are trying to avoid benchmarking
the benchmarking system itself!

As a consequence, a Sibench node only starts up as many workers as it has cores.
This is adjustable using the ``--workers`` option.  (A factor of 2.0 will have
twice as many workers as cores).  This may be useful if we want to determine
absolute maximum throughput, provided we don't care about the accuracy of the
response times.

*Note that sibench considers hyperthreaded cores as real cores for the purposes
of determining core counts.*

Alternatively, you may also be able to boost read throughput from the Sibench
nodes by using the ``--skip-read-verification`` option, which does exactly what
it suggests.

In general though, neither of these two options are recommended except for one
particular use case: if disabling read verification or increasing the worker
count boosts your throughput numbers, then that is an indication that more
Sibench nodes should be added in order to benchmark at those rates whilst still
giving accurate timings.

Response times
--------------

Whilst Sibench will output the maximum, minimum and average response times, in
practice it is the 95%-response time - the time in which 95% of requests
complete - that is likely to be the most informative.  Maximum response times
can be thrown out by one outlier result, which in turn poisons the average.  The
95% figure (or the 99% figure if you wish to perform your own analysis) is a
better indicator of a system's behaviour.

Memory considerations
---------------------

Sibench is written to use as little memory as possible.  The generators
algorithmically create each object to be written or read-and-verified on the
fly, and so objects do not need to be held in memory for longer than a single
read or write operation as they can be recreated at will.

The one part of Sibench that can take a *lot* of memory is the stats gathering,
as stats are held in memory by each driver node until the completion of each
phase of a run.  At the end of each phase, the manager process collects the
stats from all the nodes and merges them.  This can be a lot of data if, say,
you are running 30 driver nodes against an NVMe cluster for a long run time.

A consequence of this approach is that the manager node may need a lot more 
memory than the driver nodes, because it has to hold the stats of *all* of the
driver nodes in memory in order to do the merge.

Unfortunately, some of the Ceph native libraries used by Sibench appear to
hold on to data for longer than would seem necessary.  This can result in large
amounts of memory being used, which can result in two undesirable outcomes:

* Swapping: if the benchmarking process needs to swap, then performance figures
  are likely to be wildly wrong.

* Process death: on Linux, the OOM Killer in the kernel will terminate processes
  that take too much memory with a SIGKILL.  Since this is not a signal that can
  be caught, there is no warning or error when it occurs.  (The systemd script
  should start a new copy of the server immediately though, so the Sibench node
  will be usable for a new benchmark run with no further action.

At the start of each run, Sibench determines how much physical memory each node
has, and does some back-of-the-envelope maths to determine how much memory a
benchmark may consume in the worst case.  If the latter is within about 80% of
the former, it outputs a warning message to alert the user of possible
consequences.  However, the benchmark will try to run (and because this assumes
worst-case ceph library behaviour, it may well succeed).

Homogeneous cores
-----------------

Sibench divides its workload between nodes, with each taking responsibility for
reading and writing some number of objects.  The division of labour is done
purely according to how many cores each node has.  It does not attempt to
measure the performance of each server node, nor does it use some artificial
measure of performance such as BogoMIPS.  Because of this, it is important that
the nodes used as Sibench servers be of roughly equivalent speed, at least on a
per-core basis.

The reason for this is that if one Sibench server is far quicker than its peers,
then when it finishes reading its share of the objects and loops round to start
at the beginning again, the data may still be in the storage system's caches.
