Sibench
=======

Sibench is a benchmarking tool for storage systems, and for Ceph in particular.

Sibench typically runs on many nodes in parallel, in order to be able to scale
up to the very high bandwidths of which modern scaleable storage systems are
capable.

It has support for file, block and object storage backends, using a variety of
different protocols, which currently include:

- Rados: Ceph's native object protocol.
- RBD: Ceph's block protocol.
- CephFS: Ceph's POSIX filesystem protocol.
- S3: Amazon's object protocol, which is always provided by Ceph's RadosGateway.
- Local block storage
- Local file storage

These last two can be used to benchmark many other protocols - iSCSI, SMB, NFS
and so on - provided that these have been manually mounted on each Sibench
node.

.. toctree::
   :maxdepth: 2
   :caption: Contents:
   :hidden:

   self
   manual
   best-practices
