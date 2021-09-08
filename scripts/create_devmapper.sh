#!/bin/bash

sudo mkdir -p /fccd/firecracker-containerd/snapshotter/devmapper

pushd /fccd/firecracker-containerd/snapshotter/devmapper > /dev/null
DIR=/fccd/firecracker-containerd/snapshotter/devmapper
POOL=fc-dev-thinpool

# Create thinpool devices
DATADEV=/dev/VG1/dmdata
if [[ ! -f "${DATADEV}" ]]; then
    sudo lvcreate --wipesignatures y -n dmdata --size 100G VG1
fi

METADEV=/dev/VG1/dmmeta
if [[ ! -f "${METADEV}" ]]; then
    sudo lvcreate --wipesignatures y -n dmmeta --size 2G VG1
fi

# Create thinpool
SECTORSIZE=512
DATASIZE="$(sudo blockdev --getsize64 -q ${DATADEV})"
LENGTH_SECTORS=$(bc <<< "${DATASIZE}/${SECTORSIZE}")
DATA_BLOCK_SIZE=128
LOW_WATER_MARK=32768
THINP_TABLE="0 ${LENGTH_SECTORS} thin-pool ${METADEV} ${DATADEV} ${DATA_BLOCK_SIZE} ${LOW_WATER_MARK} 1 skip_block_zeroing"
echo "${THINP_TABLE}"

if ! $(sudo dmsetup reload "${POOL}" --table "${THINP_TABLE}"); then
    sudo dmsetup create "${POOL}" --table "${THINP_TABLE}"
fi

popd > /dev/null
