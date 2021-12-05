package metrics

import (
	"fmt"
	"reflect"
)

// BootMetric is used to store a breakdown of the latency involved different stages of booting a VM.
type BootMetric struct {
	// RevisionId Revision id of the function
	RevisionId      string
	// Failed Whether the function boot has failed
	Failed          bool

	// AllocateVM Time to add a VM to the pool and setup its networking
	AllocateVM       float64
	// GetImage Time to pull docker image
	GetImage         float64
	// SnapBooted Indicates whether the VM was booted from a snapshot
	SnapBooted       bool

	// Below metrics are specific to VMs that have been booted from scratch
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

	// Below metrics are specific to VMs that have been loaded from a snapshot
	// FcLoadSnapshot Time it takes to boot the snapshot
	FcLoadSnapshot   float64
	// FcResume Time it takes to resume a VM from containerd
	FcResume         float64
}

func NewBootMetric(revisionId string) *BootMetric {
	b := new(BootMetric)
	b.RevisionId = revisionId
	b.Failed = true

	return b
}

// GetBootHeaderLine Returns a string that can be used as a CSV header for VM boot latency breakdowns.
func GetBootHeaderLine() []string {
	metric := NewBootMetric("")
	return metric.GetMetricNames()
}

// GetValueLine Returns a CSV representation of the boot latency breakdown stored in the specified BootMetric struct.
func (metric *BootMetric) GetValueLine() []string {
	v := reflect.ValueOf(*metric)
	values:= make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		values[i] = fmt.Sprintf("%v", v.Field(i).Interface())
	}
	return values
}

// GetMetricNames Returns the names of the different metrics stored in the struct.
func (metric *BootMetric) GetMetricNames() []string {
	v := reflect.ValueOf(*metric)
	typeOfS := v.Type()
	keys := make([]string, v.NumField())

	for i := 0; i< v.NumField(); i++ {
		keys[i] = fmt.Sprintf("%s", typeOfS.Field(i).Name)
	}
	return keys
}

// GetMetricValues Returns the values of the different latency metrics stored in the struct.
func (metric *BootMetric) GetMetricValues() []float64 {
	v := reflect.ValueOf(*metric)
	values:= make([]float64, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type().Name() == "float64" {
			values[i] = v.Field(i).Float()
		}
	}
	return values
}

// GetMetricMap Returns a map containing the latency breakdown values in the struct.
func (metric *BootMetric) GetMetricMap() map[string]float64 {
	v := reflect.ValueOf(*metric)
	typeOfS := v.Type()

	metricMap := make(map[string]float64)

	for i := 0; i< v.NumField(); i++ {
		if v.Field(i).Type().Name() == "float64" {
			key := fmt.Sprintf("%s", typeOfS.Field(i).Name)
			metricMap[key] = v.Field(i).Float()
		}
	}

	return metricMap
}

// Total Calculates the total time per stat
func (metric *BootMetric) Total() float64 {
	var sum float64
	for _, v := range metric.GetMetricValues() {
		sum += v
	}

	return sum
}

// PrintTotal Prints the total time
func (metric *BootMetric) PrintTotal() {
	fmt.Printf("Total: %.1f us\n", metric.Total())
}

// PrintAll Prints a breakdown of the time
func (metric *BootMetric) PrintAll() {
	for k, v := range metric.GetMetricMap() {
		fmt.Printf("%s:\t%.1f\n", k, v)
	}
	fmt.Printf("Total\t%.1f\n", metric.Total())
}