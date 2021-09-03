package metrics

import (
	"fmt"
	"reflect"
)

// If value is 0: not exist. Make sure all fields capitalized. Fields added to csv in order in struct
type BootMetric struct {
	RevisionId      string
	Failed          bool

	// 1. General metrics
	// AllocateVM Time to add a VM to the pool and setup its networking
	AllocateVM       float64
	// GetImage Time to pull docker image
	GetImage         float64

	// 2. Scratch metrics
	// FcCreateVM Time to create VM
	FcCreateVM      float64
	// NewContainer Time to create new container
	NewContainer    float64
	// NewTask Time to create new task
	NewTask         float64
	// TaskWait Time to wait for task to be ready
	TaskWait        float64
	// TaskStart Time to start task
	TaskStart       float64

	// 3. (remote) snapshot metrics
	SnapBooted       bool
	// CreateDeviceSnap Time to create a snapshot from the container from an image
	CreateDeviceSnap float64
	// RestorePatch Time to apply a patch to the container snapshot
	RestorePatch     float64
	// FcLoadSnapshot Time it takes to boot the snapshot
	FcLoadSnapshot   float64
	// FcResume Time it takes to resume a VM from containerd
	FcResume         float64

	// 4. Remote snapshot metrics
	RemoteBooted     bool
	// FcFetchSnapshot Time it takes to check if the snapshot is in remote storage
	FcCheckSnapshot  float64
	// FcFetchSnapshot Time it takes to fetch the snapshot from remote storage
	FcFetchSnapshot  float64
}

func NewBootMetric(revisionId string) *BootMetric {
	b := new(BootMetric)
	b.RevisionId = revisionId
	b.Failed = true
	b.RemoteBooted = false

	return b
}

func GetBootHeaderLine() []string {
	metric := NewBootMetric("")
	v := reflect.ValueOf(*metric)
	typeOfS := v.Type()
	keys := make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		keys[i] = fmt.Sprintf("%s", typeOfS.Field(i).Name)
	}
	return keys
}

func (metric *BootMetric) GetValueLine() []string {
	v := reflect.ValueOf(*metric)
	values:= make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		values[i] = fmt.Sprintf("%v", v.Field(i).Interface())
	}
	return values
}

// If value is 0: not exist. Make sure all fields capitalized. Fields added to csv in order in struct
type SnapMetric struct {
	RevisionId        string
	Failed          bool

	// GetVM Time to pause the vm
	PauseVm             float64
	// GetVM Time to get the vm from the pool
	GetVM             float64
	// FcCreateSnapshot Time it takes to create the snapshot in firecracker
	FcCreateSnapshot  float64
	// CreatePatch Time to create a patch for the container snapshot
	CreatePatch       float64 // TODO: old. Only for remote
	// ForkContainerSnap Time to create a copy of the container snapshot
	ForkContainerSnap float64
	// SerializeSnapInfo Time to serialize the snapshot info
	SerializeSnapInfo float64
	// SerializeSnapInfo Time to make the memfile sparse
	SparsifyMemfile   float64
	// FcResume Time to resume vm
	FcResume   float64

}

func NewSnapMetric(revisionId string) *SnapMetric {
	s := new(SnapMetric)
	s.RevisionId = revisionId
	s.Failed = true
	return s
}

func GetSnapHeaderLine() []string {
	metric := NewSnapMetric("")
	v := reflect.ValueOf(*metric)
	typeOfS := v.Type()
	keys := make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		keys[i] = fmt.Sprintf("%s", typeOfS.Field(i).Name)
	}
	return keys
}

func (metric *SnapMetric) GetValueLine() []string {
	v := reflect.ValueOf(*metric)
	values:= make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		values[i] = fmt.Sprintf("%v", v.Field(i).Interface())
	}
	return values
}

// If value is 0: not exist. Make sure all fields capitalized. Fields added to csv in order in struct
type NetMetric struct {
	RevisionId        string
	Failed          bool

	CreateConfig       float64
	LockOsThread       float64
	GetHostNs          float64
	CreateVmNs         float64
	CreateVmTap        float64
	CreateVeth         float64
	ConfigVethVm       float64
	SetDefaultGw       float64
	SetupNat           float64
	SwitchHostNs       float64
	UnlockOsThread     float64
	ConfigVethHost     float64
	Addroute           float64
	SetForward         float64
}

func NewNetMetric(revisionId string) *NetMetric {
	s := new(NetMetric)
	s.RevisionId = revisionId
	s.Failed = true
	return s
}

func GetNetHeaderLine() []string {
	metric := NewNetMetric("")
	v := reflect.ValueOf(*metric)
	typeOfS := v.Type()
	keys := make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		keys[i] = fmt.Sprintf("%s", typeOfS.Field(i).Name)
	}
	return keys
}

func (metric *NetMetric) GetValueLine() []string {
	v := reflect.ValueOf(*metric)
	values:= make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		values[i] = fmt.Sprintf("%v", v.Field(i).Interface())
	}
	return values
}