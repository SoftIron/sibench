Examples
========


Rados and RBD benchmarking
--------------------------

For these tests you will need to create a Pool, and a Ceph user that can
access to this pool::

    ceph osd pool create sibench.pool <pg-num>
    ceph auth get-or-create client.sibench mon 'profile rbd' \
      osd 'profile rbd pool=sibench.pool' mgr 'profile rbd pool=sibench.pool'



.. note::

    You can use an existing user and pool if needed. Additionally, you can
    clean the pool later using ``rados purge sibench.pool
    --yes-i-really-really-mean-it``

As explained on :ref:`targets section <manual:targets>` of the :doc:`manual`,
using only one Monitor is enough.

Running Sibench using the Rados object protocol
"""""""""""""""""""""""""""""""""""""""""""""""

.. code-block::

    sibench rados run --ceph-pool sibench.pool --ceph-user sibench \
      --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== <Ceph monitor address>

.. raw:: html

   <details>
   <summary>Example output</summary>


.. code-block::

    $ sibench rados run --ceph-pool sibench.pool --ceph-user sibench \
    >   --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== ceph-mon1
    Creating report: sibench.json
    Creating rados client to ceph-mon1 as user sibench
    2022-05-23 07:39:10.858 7f81428039c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    2022-05-23 07:39:10.858 7f81428039c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    2022-05-23 07:39:10.858 7f81428039c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    Connecting to sibench server at localhost:5150

    ---------- Sibench driver capabilities discovery ----------
    localhost: 4 cores, 15.5 GB of RAM

    ----------------------- WRITE -----------------------------
    0: [Write] ops: 148,  bw: 1.2 Gb/s,  ofail: 0,  vfail: 0
    1: [Write] ops: 174,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    2: [Write] ops: 195,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    3: [Write] ops: 195,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    4: [Write] ops: 180,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Write] ops: 196,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    6: [Write] ops: 190,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    7: [Write] ops: 204,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    8: [Write] ops: 184,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    9: [Write] ops: 184,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    10: [Write] ops: 196,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    11: [Write] ops: 197,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    12: [Write] ops: 190,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    13: [Write] ops: 204,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    14: [Write] ops: 184,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    15: [Write] ops: 200,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    16: [Write] ops: 192,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    17: [Write] ops: 201,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    18: [Write] ops: 197,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    19: [Write] ops: 185,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    20: [Write] ops: 196,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    21: [Write] ops: 188,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    22: [Write] ops: 191,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    23: [Write] ops: 199,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    24: [Write] ops: 195,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    25: [Write] ops: 192,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    26: [Write] ops: 198,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    27: [Write] ops: 198,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    28: [Write] ops: 204,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    29: [Write] ops: 186,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    30: [Write] ops: 197,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    31: [Write] ops: 195,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    32: [Write] ops: 201,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    33: [Write] ops: 202,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    34: [Write] ops: 193,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Write] ops: 196,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    36: [Write] ops: 197,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    7344 stats retrieved in 0.043 seconds

    ---------------------- PREPARE ----------------------------
    Retrieving stats from servers
    0 stats retrieved in 0.020 seconds

    ----------------------- READ ------------------------------
    0: [Read] ops: 229,  bw: 1.8 Gb/s,  ofail: 0,  vfail: 0
    1: [Read] ops: 294,  bw: 2.3 Gb/s,  ofail: 0,  vfail: 0
    2: [Read] ops: 312,  bw: 2.4 Gb/s,  ofail: 0,  vfail: 0
    3: [Read] ops: 665,  bw: 5.2 Gb/s,  ofail: 0,  vfail: 0
    4: [Read] ops: 1017,  bw: 7.9 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Read] ops: 1026,  bw: 8.0 Gb/s,  ofail: 0,  vfail: 0
    6: [Read] ops: 1011,  bw: 7.9 Gb/s,  ofail: 0,  vfail: 0
    7: [Read] ops: 1020,  bw: 8.0 Gb/s,  ofail: 0,  vfail: 0
    8: [Read] ops: 1011,  bw: 7.9 Gb/s,  ofail: 0,  vfail: 0
    9: [Read] ops: 1007,  bw: 7.9 Gb/s,  ofail: 0,  vfail: 0
    10: [Read] ops: 1021,  bw: 8.0 Gb/s,  ofail: 0,  vfail: 0
    11: [Read] ops: 998,  bw: 7.8 Gb/s,  ofail: 0,  vfail: 0
    12: [Read] ops: 984,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    13: [Read] ops: 997,  bw: 7.8 Gb/s,  ofail: 0,  vfail: 0
    14: [Read] ops: 996,  bw: 7.8 Gb/s,  ofail: 0,  vfail: 0
    15: [Read] ops: 984,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    16: [Read] ops: 976,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    17: [Read] ops: 997,  bw: 7.8 Gb/s,  ofail: 0,  vfail: 0
    18: [Read] ops: 987,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    19: [Read] ops: 978,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    20: [Read] ops: 976,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    21: [Read] ops: 968,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    22: [Read] ops: 982,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    23: [Read] ops: 980,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    24: [Read] ops: 976,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    25: [Read] ops: 987,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    26: [Read] ops: 977,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    27: [Read] ops: 978,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    28: [Read] ops: 967,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    29: [Read] ops: 959,  bw: 7.5 Gb/s,  ofail: 0,  vfail: 0
    30: [Read] ops: 980,  bw: 7.7 Gb/s,  ofail: 0,  vfail: 0
    31: [Read] ops: 964,  bw: 7.5 Gb/s,  ofail: 0,  vfail: 0
    32: [Read] ops: 958,  bw: 7.5 Gb/s,  ofail: 0,  vfail: 0
    33: [Read] ops: 978,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    34: [Read] ops: 932,  bw: 7.3 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Read] ops: 944,  bw: 7.4 Gb/s,  ofail: 0,  vfail: 0
    36: [Read] ops: 970,  bw: 7.6 Gb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    35163 stats retrieved in 0.092 seconds

    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-mon1] Write        bandwidth:   1.5 Gb/s,  ok:   5841,  fail:      0,  res-min:    10 ms,  res-max:   282 ms,  res-95:     29 ms, res-avg:     20 ms
    Server[localhost] Write        bandwidth:   1.5 Gb/s,  ok:   5841,  fail:      0,  res-min:    10 ms,  res-max:   282 ms,  res-95:     29 ms, res-avg:     20 ms
    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-mon1] Read         bandwidth:   7.7 Gb/s,  ok:  29540,  fail:      0,  res-min:     1 ms,  res-max:    13 ms,  res-95:      5 ms, res-avg:      3 ms
    Server[localhost] Read         bandwidth:   7.7 Gb/s,  ok:  29540,  fail:      0,  res-min:     1 ms,  res-max:    13 ms,  res-95:      5 ms, res-avg:      3 ms
    ================================================================================================================================================================
    Total Write                    bandwidth:   1.5 Gb/s,  ok:   5841,  fail:      0,  res-min:    10 ms,  res-max:   282 ms,  res-95:     29 ms, res-avg:     20 ms
    Total Read                     bandwidth:   7.7 Gb/s,  ok:  29540,  fail:      0,  res-min:     1 ms,  res-max:    13 ms,  res-95:      5 ms, res-avg:      3 ms
    ================================================================================================================================================================

    Disconnecting from servers
    Disconnected
    Done

.. raw:: html

   </details>


Using RBD protocol
""""""""""""""""""

.. code-block::

    sibench rbd run --ceph-pool sibench.pool --ceph-user sibench \
      --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== <Ceph monitor address>


.. raw:: html

   <details>
   <summary>Example output</summary>


.. code-block::

    $ sibench rbd run --ceph-pool sibench.pool --ceph-user sibench \
    >   --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== ceph-mon1
    Creating report: sibench.json
    Creating rados client to ceph-mon1 as user sibench
    2022-05-23 09:50:42.666 7fa49ca939c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    2022-05-23 09:50:42.666 7fa49ca939c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    2022-05-23 09:50:42.666 7fa49ca939c0 -1 auth: unable to find a keyring on /etc/ceph/..keyring,/etc/ceph/.keyring,/etc/ceph/keyring,/etc/ceph/keyring.bin,: (2) No such file or directory
    Connecting to sibench server at localhost:5150

    ---------- Sibench driver capabilities discovery ----------
    localhost: 4 cores, 15.5 GB of RAM

    ----------------------- WRITE -----------------------------
    0: [Write] ops: 37,  bw: 296.0 Mb/s,  ofail: 0,  vfail: 0
    1: [Write] ops: 161,  bw: 1.3 Gb/s,  ofail: 0,  vfail: 0
    2: [Write] ops: 171,  bw: 1.3 Gb/s,  ofail: 0,  vfail: 0
    3: [Write] ops: 173,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    4: [Write] ops: 170,  bw: 1.3 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Write] ops: 177,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    6: [Write] ops: 174,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    7: [Write] ops: 186,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    8: [Write] ops: 185,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    9: [Write] ops: 193,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    10: [Write] ops: 199,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    11: [Write] ops: 180,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    12: [Write] ops: 183,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    13: [Write] ops: 184,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    14: [Write] ops: 172,  bw: 1.3 Gb/s,  ofail: 0,  vfail: 0
    15: [Write] ops: 183,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    16: [Write] ops: 186,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    17: [Write] ops: 185,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    18: [Write] ops: 197,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    19: [Write] ops: 189,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    20: [Write] ops: 199,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    21: [Write] ops: 192,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    22: [Write] ops: 194,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    23: [Write] ops: 187,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    24: [Write] ops: 179,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    25: [Write] ops: 192,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    26: [Write] ops: 199,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    27: [Write] ops: 187,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    28: [Write] ops: 189,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    29: [Write] ops: 186,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    30: [Write] ops: 196,  bw: 1.5 Gb/s,  ofail: 0,  vfail: 0
    31: [Write] ops: 180,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    32: [Write] ops: 177,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    33: [Write] ops: 201,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    34: [Write] ops: 199,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Write] ops: 205,  bw: 1.6 Gb/s,  ofail: 0,  vfail: 0
    36: [Write] ops: 176,  bw: 1.4 Gb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    7041 stats retrieved in 0.021 seconds

    ---------------------- PREPARE ----------------------------
    Retrieving stats from servers
    0 stats retrieved in 0.010 seconds

    ----------------------- READ ------------------------------
    0: [Read] ops: 78,  bw: 624.0 Mb/s,  ofail: 0,  vfail: 0
    1: [Read] ops: 325,  bw: 2.5 Gb/s,  ofail: 0,  vfail: 0
    2: [Read] ops: 409,  bw: 3.2 Gb/s,  ofail: 0,  vfail: 0
    3: [Read] ops: 467,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    4: [Read] ops: 449,  bw: 3.5 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Read] ops: 459,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    6: [Read] ops: 442,  bw: 3.5 Gb/s,  ofail: 0,  vfail: 0
    7: [Read] ops: 465,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    8: [Read] ops: 490,  bw: 3.8 Gb/s,  ofail: 0,  vfail: 0
    9: [Read] ops: 496,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    10: [Read] ops: 464,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    11: [Read] ops: 412,  bw: 3.2 Gb/s,  ofail: 0,  vfail: 0
    12: [Read] ops: 491,  bw: 3.8 Gb/s,  ofail: 0,  vfail: 0
    13: [Read] ops: 449,  bw: 3.5 Gb/s,  ofail: 0,  vfail: 0
    14: [Read] ops: 509,  bw: 4.0 Gb/s,  ofail: 0,  vfail: 0
    15: [Read] ops: 425,  bw: 3.3 Gb/s,  ofail: 0,  vfail: 0
    16: [Read] ops: 488,  bw: 3.8 Gb/s,  ofail: 0,  vfail: 0
    17: [Read] ops: 475,  bw: 3.7 Gb/s,  ofail: 0,  vfail: 0
    18: [Read] ops: 528,  bw: 4.1 Gb/s,  ofail: 0,  vfail: 0
    19: [Read] ops: 433,  bw: 3.4 Gb/s,  ofail: 0,  vfail: 0
    20: [Read] ops: 497,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    21: [Read] ops: 423,  bw: 3.3 Gb/s,  ofail: 0,  vfail: 0
    22: [Read] ops: 472,  bw: 3.7 Gb/s,  ofail: 0,  vfail: 0
    23: [Read] ops: 462,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    24: [Read] ops: 466,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    25: [Read] ops: 507,  bw: 4.0 Gb/s,  ofail: 0,  vfail: 0
    26: [Read] ops: 472,  bw: 3.7 Gb/s,  ofail: 0,  vfail: 0
    27: [Read] ops: 500,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    28: [Read] ops: 496,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    29: [Read] ops: 500,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    30: [Read] ops: 474,  bw: 3.7 Gb/s,  ofail: 0,  vfail: 0
    31: [Read] ops: 514,  bw: 4.0 Gb/s,  ofail: 0,  vfail: 0
    32: [Read] ops: 461,  bw: 3.6 Gb/s,  ofail: 0,  vfail: 0
    33: [Read] ops: 505,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    34: [Read] ops: 429,  bw: 3.4 Gb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Read] ops: 504,  bw: 3.9 Gb/s,  ofail: 0,  vfail: 0
    36: [Read] ops: 418,  bw: 3.3 Gb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    17727 stats retrieved in 0.040 seconds

    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-mon1] Write        bandwidth:   1.5 Gb/s,  ok:   5653,  fail:      0,  res-min:    10 ms,  res-max:   266 ms,  res-95:     34 ms, res-avg:     20 ms
    Server[localhost] Write        bandwidth:   1.5 Gb/s,  ok:   5653,  fail:      0,  res-min:    10 ms,  res-max:   266 ms,  res-95:     34 ms, res-avg:     20 ms
    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-mon1] Read         bandwidth:   3.7 Gb/s,  ok:  14278,  fail:      0,  res-min:     2 ms,  res-max:   170 ms,  res-95:     18 ms, res-avg:      7 ms
    Server[localhost] Read         bandwidth:   3.7 Gb/s,  ok:  14278,  fail:      0,  res-min:     2 ms,  res-max:   170 ms,  res-95:     18 ms, res-avg:      7 ms
    ================================================================================================================================================================
    Total Write                    bandwidth:   1.5 Gb/s,  ok:   5653,  fail:      0,  res-min:    10 ms,  res-max:   266 ms,  res-95:     34 ms, res-avg:     20 ms
    Total Read                     bandwidth:   3.7 Gb/s,  ok:  14278,  fail:      0,  res-min:     2 ms,  res-max:   170 ms,  res-95:     18 ms, res-avg:      7 ms
    ================================================================================================================================================================

    Disconnecting from servers
    Disconnected
    Done

.. raw:: html

   </details>


Multiple Sibench servers
""""""""""""""""""""""""
To run Sibench from multiple servers you need to set the ``--servers`` option
(by default 'localhost') to select the Sibench servers to use::

    sibench rados run --ceph-pool sibench.pool --ceph-user sibench \
      --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== \
      --servers <driver1,driver2...> <Ceph monitor address>


.. raw:: html

   <details>
   <summary>Example output</summary>


.. code-block::

    $ sibench rados run --ceph-pool sibench.pool --ceph-user sibench \
    >   --ceph-key AQASFmhiN2aiCBARYd1iIdn2ntHGoFjL3QJiTA== \
    > --servers sibench-driver1,sibench-driver2 ceph-mon1

    Creating report: sibench.json
    Connecting to sibench server at sibench-driver1:5150
    Connecting to sibench server at sibench-driver2:5150

    ---------- Sibench driver capabilities discovery ----------
    sibench-driver1: 4 cores, 15.5 GB of RAM
    sibench-driver2: 4 cores, 15.5 GB of RAM

    ----------------------- WRITE -----------------------------
    0: [Write] ops: 37,  bw: 296.0 Mb/s,  ofail: 0,  vfail: 0
    1: [Write] ops: 161,  bw: 1.3 Gb/s,  ofail: 0,  vfail: 0
    ...

.. raw:: html

   </details>

S3 benchmarking
---------------

In this case you will need to create an S3 user and bucket to run Sibench::

    radosgw-admin user create --uid sibench --display-name sibench

.. note::

    The user can be removed with ``radosgw-admin user rm --uid=sibench``

.. code-block::

    sibench s3 run --s3-bucket sibench_bucket --s3-access-key <key> \
      --s3-secret-key <secret key> --servers <driver1,driver1...> \
      <List of Rados Gateway servers>

.. warning::

    Sibench will automatically create and delete the bucket for you. Avoid
    using an existing bucket.


.. raw:: html

   <details>
   <summary>Example output</summary>


.. code-block::

    $ sibench s3 run  --s3-bucket sibench_bucket --s3-access-key Q2ZUTESFMIF43V9CXOR9 \
    >   --s3-secret-key OXHtTFvLBVAoj7eyC1uZnySx0TP3c0UB2dKvjpd6  ceph-rgw1 ceph-rgw2 ceph-rgw3
    Creating report: sibench.json
    Creating S3 Connection to ceph-rgw1:7480
    Creating bucket on ceph-rgw1: sibench_bucket
    Connecting to sibench server at localhost:5150

    ---------- Sibench driver capabilities discovery ----------
    localhost: 4 cores, 15.5 GB of RAM

    ----------------------- WRITE -----------------------------
    0: [Write] ops: 35,  bw: 280.0 Mb/s,  ofail: 0,  vfail: 0
    1: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    2: [Write] ops: 44,  bw: 352.0 Mb/s,  ofail: 0,  vfail: 0
    3: [Write] ops: 47,  bw: 376.0 Mb/s,  ofail: 0,  vfail: 0
    4: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Write] ops: 48,  bw: 384.0 Mb/s,  ofail: 0,  vfail: 0
    6: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    7: [Write] ops: 44,  bw: 352.0 Mb/s,  ofail: 0,  vfail: 0
    8: [Write] ops: 46,  bw: 368.0 Mb/s,  ofail: 0,  vfail: 0
    9: [Write] ops: 47,  bw: 376.0 Mb/s,  ofail: 0,  vfail: 0
    10: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    11: [Write] ops: 46,  bw: 368.0 Mb/s,  ofail: 0,  vfail: 0
    12: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    13: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    14: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    15: [Write] ops: 44,  bw: 352.0 Mb/s,  ofail: 0,  vfail: 0
    16: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    17: [Write] ops: 44,  bw: 352.0 Mb/s,  ofail: 0,  vfail: 0
    18: [Write] ops: 46,  bw: 368.0 Mb/s,  ofail: 0,  vfail: 0
    19: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    20: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    21: [Write] ops: 46,  bw: 368.0 Mb/s,  ofail: 0,  vfail: 0
    22: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    23: [Write] ops: 47,  bw: 376.0 Mb/s,  ofail: 0,  vfail: 0
    24: [Write] ops: 48,  bw: 384.0 Mb/s,  ofail: 0,  vfail: 0
    25: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    26: [Write] ops: 45,  bw: 360.0 Mb/s,  ofail: 0,  vfail: 0
    27: [Write] ops: 41,  bw: 328.0 Mb/s,  ofail: 0,  vfail: 0
    28: [Write] ops: 43,  bw: 344.0 Mb/s,  ofail: 0,  vfail: 0
    29: [Write] ops: 39,  bw: 312.0 Mb/s,  ofail: 0,  vfail: 0
    30: [Write] ops: 42,  bw: 336.0 Mb/s,  ofail: 0,  vfail: 0
    31: [Write] ops: 42,  bw: 336.0 Mb/s,  ofail: 0,  vfail: 0
    32: [Write] ops: 40,  bw: 320.0 Mb/s,  ofail: 0,  vfail: 0
    33: [Write] ops: 42,  bw: 336.0 Mb/s,  ofail: 0,  vfail: 0
    34: [Write] ops: 42,  bw: 336.0 Mb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Write] ops: 41,  bw: 328.0 Mb/s,  ofail: 0,  vfail: 0
    36: [Write] ops: 44,  bw: 352.0 Mb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    1675 stats retrieved in 0.012 seconds

    ---------------------- PREPARE ----------------------------
    Retrieving stats from servers
    0 stats retrieved in 0.020 seconds

    ----------------------- READ ------------------------------
    0: [Read] ops: 55,  bw: 440.0 Mb/s,  ofail: 0,  vfail: 0
    1: [Read] ops: 72,  bw: 576.0 Mb/s,  ofail: 0,  vfail: 0
    2: [Read] ops: 68,  bw: 544.0 Mb/s,  ofail: 0,  vfail: 0
    3: [Read] ops: 73,  bw: 584.0 Mb/s,  ofail: 0,  vfail: 0
    4: [Read] ops: 75,  bw: 600.0 Mb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    5: [Read] ops: 73,  bw: 584.0 Mb/s,  ofail: 0,  vfail: 0
    6: [Read] ops: 69,  bw: 552.0 Mb/s,  ofail: 0,  vfail: 0
    7: [Read] ops: 71,  bw: 568.0 Mb/s,  ofail: 0,  vfail: 0
    8: [Read] ops: 69,  bw: 552.0 Mb/s,  ofail: 0,  vfail: 0
    9: [Read] ops: 63,  bw: 504.0 Mb/s,  ofail: 0,  vfail: 0
    10: [Read] ops: 65,  bw: 520.0 Mb/s,  ofail: 0,  vfail: 0
    11: [Read] ops: 62,  bw: 496.0 Mb/s,  ofail: 0,  vfail: 0
    12: [Read] ops: 61,  bw: 488.0 Mb/s,  ofail: 0,  vfail: 0
    13: [Read] ops: 63,  bw: 504.0 Mb/s,  ofail: 0,  vfail: 0
    14: [Read] ops: 64,  bw: 512.0 Mb/s,  ofail: 0,  vfail: 0
    15: [Read] ops: 76,  bw: 608.0 Mb/s,  ofail: 0,  vfail: 0
    16: [Read] ops: 83,  bw: 664.0 Mb/s,  ofail: 0,  vfail: 0
    17: [Read] ops: 84,  bw: 672.0 Mb/s,  ofail: 0,  vfail: 0
    18: [Read] ops: 87,  bw: 696.0 Mb/s,  ofail: 0,  vfail: 0
    19: [Read] ops: 91,  bw: 728.0 Mb/s,  ofail: 0,  vfail: 0
    20: [Read] ops: 91,  bw: 728.0 Mb/s,  ofail: 0,  vfail: 0
    21: [Read] ops: 89,  bw: 712.0 Mb/s,  ofail: 0,  vfail: 0
    22: [Read] ops: 85,  bw: 680.0 Mb/s,  ofail: 0,  vfail: 0
    23: [Read] ops: 95,  bw: 760.0 Mb/s,  ofail: 0,  vfail: 0
    24: [Read] ops: 79,  bw: 632.0 Mb/s,  ofail: 0,  vfail: 0
    25: [Read] ops: 76,  bw: 608.0 Mb/s,  ofail: 0,  vfail: 0
    26: [Read] ops: 80,  bw: 640.0 Mb/s,  ofail: 0,  vfail: 0
    27: [Read] ops: 81,  bw: 648.0 Mb/s,  ofail: 0,  vfail: 0
    28: [Read] ops: 78,  bw: 624.0 Mb/s,  ofail: 0,  vfail: 0
    29: [Read] ops: 78,  bw: 624.0 Mb/s,  ofail: 0,  vfail: 0
    30: [Read] ops: 78,  bw: 624.0 Mb/s,  ofail: 0,  vfail: 0
    31: [Read] ops: 78,  bw: 624.0 Mb/s,  ofail: 0,  vfail: 0
    32: [Read] ops: 81,  bw: 648.0 Mb/s,  ofail: 0,  vfail: 0
    33: [Read] ops: 81,  bw: 648.0 Mb/s,  ofail: 0,  vfail: 0
    34: [Read] ops: 80,  bw: 640.0 Mb/s,  ofail: 0,  vfail: 0
    -----------------------------------------------------------
    35: [Read] ops: 79,  bw: 632.0 Mb/s,  ofail: 0,  vfail: 0
    36: [Read] ops: 77,  bw: 616.0 Mb/s,  ofail: 0,  vfail: 0
    Retrieving stats from servers
    2908 stats retrieved in 0.045 seconds

    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-rgw1] Write        bandwidth: 117.9 Mb/s,  ok:    442,  fail:      0,  res-min:    62 ms,  res-max:   250 ms,  res-95:    111 ms, res-avg:     91 ms
    Target[ceph-rgw2] Write        bandwidth: 117.9 Mb/s,  ok:    442,  fail:      0,  res-min:    59 ms,  res-max:   265 ms,  res-95:    109 ms, res-avg:     88 ms
    Target[ceph-rgw3] Write        bandwidth: 117.6 Mb/s,  ok:    441,  fail:      0,  res-min:    61 ms,  res-max:   232 ms,  res-95:    111 ms, res-avg:     90 ms
    Server[localhost] Write        bandwidth: 353.3 Mb/s,  ok:   1325,  fail:      0,  res-min:    59 ms,  res-max:   265 ms,  res-95:    111 ms, res-avg:     90 ms
    ----------------------------------------------------------------------------------------------------------------------------------------------------------------
    Target[ceph-rgw1] Read         bandwidth: 205.6 Mb/s,  ok:    771,  fail:      0,  res-min:    28 ms,  res-max:   233 ms,  res-95:     73 ms, res-avg:     50 ms
    Target[ceph-rgw2] Read         bandwidth: 205.6 Mb/s,  ok:    771,  fail:      0,  res-min:    24 ms,  res-max:   174 ms,  res-95:     72 ms, res-avg:     51 ms
    Target[ceph-rgw3] Read         bandwidth: 205.1 Mb/s,  ok:    769,  fail:      0,  res-min:    25 ms,  res-max:   152 ms,  res-95:     72 ms, res-avg:     51 ms
    Server[localhost] Read         bandwidth: 616.3 Mb/s,  ok:   2311,  fail:      0,  res-min:    24 ms,  res-max:   233 ms,  res-95:     73 ms, res-avg:     51 ms
    ================================================================================================================================================================
    Total Write                    bandwidth: 353.3 Mb/s,  ok:   1325,  fail:      0,  res-min:    59 ms,  res-max:   265 ms,  res-95:    111 ms, res-avg:     90 ms
    Total Read                     bandwidth: 616.3 Mb/s,  ok:   2311,  fail:      0,  res-min:    24 ms,  res-max:   233 ms,  res-95:     73 ms, res-avg:     51 ms
    ================================================================================================================================================================

    Disconnecting from servers
    Disconnected
    Deleting bucket on ceph-rgw1: sibench_bucket
    Done

.. raw:: html

   </details>
