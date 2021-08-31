package main

import "fmt"

type VmInstance struct {
	containerID     string
	image           string
	revisionId      string
	coldStartTimeMs int64
	memSizeMib      uint32
	vCPUCount       uint32
}

func NewVmInstance(containerID, image, revisionId string, memSizeMib, vCPUCount uint32,) *VmInstance {
	f := &VmInstance{
		containerID: containerID,
		image:       image,
		revisionId:  revisionId,
		memSizeMib:  memSizeMib,
		vCPUCount:   vCPUCount,
	}

	return f
}

func (i *VmInstance) Print() {
	fmt.Printf("%+v\n",*i)
}
