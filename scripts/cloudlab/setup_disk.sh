#!/bin/bash
# Initialize partition for use by lvm
sudo pvcreate /dev/sda4
sudo vgcreate VG1 /dev/sda4

# Create fccd device & mount
sudo lvcreate --size 200G -n fccd VG1
sudo mkfs.ext4 /dev/VG1/fccd
sudo mkdir /fccd
sudo mount /dev/VG1/fccd /fccd
sudo chmod 0777 /fccd