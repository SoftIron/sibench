Install
=======

If you are running SoftIron Linux or Debian, we recommend you install Sibench
using the :ref:`Package install <installation:package install>` instructions.

If this is not an option, try the :ref:`Binary install <installation:binary install>`
instructions.

Package install
---------------
.. tabs::

   .. tab:: SoftIron Linux

      1. Add Sibench repository::

             echo "deb https://cdn.softiron.com/ceph/sibench buster main" | sudo tee \
             /etc/apt/sources.list.d/sibench.list > /dev/null

      2. Install Sibench::

             sudo apt-get install sibench

   .. tab:: Debian (buster)

      .. include:: installation/debian-repo-setup.txt

      4. Install Sibench::

           sudo apt-get install sibench -t buster-backports

      .. note:: This step assumes you have buster-backports enabled. Not
         recommended on environments with Ceph already installed.

   .. tab:: Debian (bullseye)

      .. include:: installation/debian-repo-setup.txt

      4. Install Sibench::

           sudo apt-get install sibench


Binary Install
--------------
.. tabs::

   .. tab:: Debian (bullseye)

      1. Install dependencies::

          sudo apt install librados2 librbd1

      .. include:: installation/binary-install.txt

   .. tab:: Ubuntu >= 20.04

      1. Install dependencies::

          sudo apt install librados2 librbd1

      .. include:: installation/binary-install.txt

   .. tab:: Fedora >= 34

      1. Install dependencies::

          sudo dnf install librados2 librbd1

      .. include:: installation/binary-install.txt
