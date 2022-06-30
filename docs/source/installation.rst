Install
=======

Debian
------

1. Update ``apt`` package index::

       sudo apt-get update
       sudo apt-get install ca-certificates curl gnupg lsb-release


2. Add Softiron's official GPG key::

       curl -fsSL https://cdn.softiron.com/linux/debian/gpg | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/softiron-archive.gpg

3. Setup the repository::

       echo  "https://cdn.softiron.com/distro/packages $(lsb_release -cs)-sibench main" | sudo tee \
       /etc/apt/sources.list.d/sibench.list > /dev/null

4. Install Sibench::

       sudo apt-get install sibench

Other Linux systems
-------------------

1. Install dependencies:

   - On Debian/Ubuntu::

       sudo apt install librados2 librbd1

   - On Fedora::

       sudo dnf install librados-devel librbd-devel

2. Download a Sibench release from https://github.com/softiron/sibench/releases, for example::

       wget https://github.com/home-assistant/operating-system/releases/latest/download/sibench-amd64 \
       -o sibench

3. Copy the binary somewhere in your ``$PATH``, for example::

       sudo cp sibench /usr/local/bin/


You will not have a Sibench sever running if only installing the binary. You
can run it manually using ``sibench server`` command, or you can create a
systemd unit like `this one. <https://github.com/SoftIron/sibench/blob/master/lib/systemd/system/sibench.service>`__