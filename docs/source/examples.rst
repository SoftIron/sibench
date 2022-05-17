Examples
========


Rados and RBD benchmarking
--------------------------

For these tests you will need to create a Pool, and a Ceph user that can
access to this pool::

    ceph osd pool create sibench.pool <pg-num>
    ceph auth get-or-create client.sibench mon 'profile rbd' osd 'profile rbd pool=sibench.pool' mgr 'profile rbd pool=sibench.pool'

.. note::
    You can use an exsiting pool if wanted

Running Sibench from a single node, using the Rados object protocol::

    sibench rados  run --ceph-pool sibench.pool --ceph-user sibench --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== <Ceph monitor address>

Using RBD::

    sibench rbd  run --ceph-pool sibench.pool --ceph-user sibench --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== <Ceph monitor address>

To run Sibench from multiple servers you need to set the ``--servers`` option
(by default 'localhost') to select the Sibench servers to use::

    sibench rados  run --ceph-pool sibench.pool --ceph-user sibench --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== --servers <List of Sibench servers> <Ceph monitor address>


S3 benchmarking
---------------

In this case you will need to create an S3 user and bucket to run Sibench::

    sibench s3 run  --s3-bucket sibench_bucket --s3-access-key <key> --s3-secret-key <secret key> --servers <List of Sibench servers> <List of Rados Gatway servers>
