// MIT License
//
// Copyright (c) 2020 Plamen Petrov and EASE lab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ctriface

import (
	"github.com/ease-lab/vhive/devmapper"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"

	fcclient "github.com/firecracker-microvm/firecracker-containerd/firecracker-control/client"
	// note: from the original repo

	_ "google.golang.org/grpc/codes"  //tmp
	_ "google.golang.org/grpc/status" //tmp

	"github.com/ease-lab/vhive/memory/manager"
	"github.com/ease-lab/vhive/metrics"
	"github.com/ease-lab/vhive/misc"

	_ "github.com/davecgh/go-spew/spew" //tmp
)

const (
	containerdAddress      = "/run/firecracker-containerd/containerd.sock" // Containerd client
	containerdTTRPCAddress = containerdAddress + ".ttrpc"				   // Firecracker client
	namespaceName          = "firecracker-containerd"
)

// Orchestrator Drives all VMs
type Orchestrator struct {
	vmPool       *misc.VMPool				 // Pool of active VMs. Map of vmid to vms and tapmanager
	cachedImages map[string]containerd.Image // Cached container images
	snapshotter  string						 // image snapshotter
	client       *containerd.Client			 // containerd client
	fcClient     *fcclient.Client			 // firecrackercontainerd client
	devMapper    *devmapper.DeviceMapper
	// store *skv.KVStore
	snapshotsEnabled bool					 // VM snapshots enabled
	isUPFEnabled     bool
	isLazyMode       bool
	snapshotsDir     string					 // Dir for VM snapshots
	isMetricsMode    bool
	hostIface        string

	memoryManager *manager.MemoryManager	// Memory manager to handle UPF
}

// NewOrchestrator Initializes a new orchestrator
func NewOrchestrator(hostIface string, opts ...OrchestratorOption) *Orchestrator {
	var err error

	o := new(Orchestrator)
	o.vmPool = misc.NewVMPool(hostIface)
	o.cachedImages = make(map[string]containerd.Image)
	o.snapshotter = "devmapper"
	o.snapshotsDir = "/fccd/snapshots"
	o.hostIface = hostIface

	for _, opt := range opts {
		opt(o)
	}

	if _, err := os.Stat(o.snapshotsDir); err != nil {
		if !os.IsNotExist(err) {
			log.Panicf("Snapshot dir %s exists", o.snapshotsDir)
		}
	}

	if err := os.MkdirAll(o.snapshotsDir, 0777); err != nil {
		log.Panicf("Failed to create snapshots dir %s", o.snapshotsDir)
	}

	log.Info("Creating containerd client")
	o.client, err = containerd.New(containerdAddress)
	if err != nil {
		log.Fatal("Failed to start containerd client", err)
	}
	log.Info("Created containerd client")

	log.Info("Creating firecracker client")
	o.fcClient, err = fcclient.New(containerdTTRPCAddress)
	if err != nil {
		log.Fatal("Failed to start firecracker client", err)
	}
	log.Info("Created firecracker client")
	o.devMapper = devmapper.NewDeviceMapper(o.client, "fc-dev-thinpool")
	return o
}

func (o *Orchestrator) setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("\r- Ctrl+C pressed in Terminal")
		_ = o.StopActiveVMs()
		o.Cleanup()
		os.Exit(0)
	}()
}

// Cleanup Removes the bridges created by the VM pool's tap manager
// Cleans up snapshots directory
func (o *Orchestrator) Cleanup() {
	o.vmPool.CleanupNetwork()
	if err := os.RemoveAll(o.snapshotsDir); err != nil {
		log.Panic("failed to delete snapshots dir", err)
	}
}

// GetSnapshotsEnabled Returns the snapshots mode of the orchestrator
func (o *Orchestrator) GetSnapshotsEnabled() bool {
	return o.snapshotsEnabled
}

// DumpUPFPageStats Dumps the memory manager's stats about the number of
// the unique pages and the number of the pages that are reused across invocations
func (o *Orchestrator) DumpUPFPageStats(vmID, functionName, metricsOutFilePath string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received DumpUPFPageStats")

	return o.memoryManager.DumpUPFPageStats(vmID, functionName, metricsOutFilePath)
}

// DumpUPFLatencyStats Dumps the memory manager's latency stats
func (o *Orchestrator) DumpUPFLatencyStats(vmID, functionName, latencyOutFilePath string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received DumpUPFPageStats")

	return o.memoryManager.DumpUPFLatencyStats(vmID, functionName, latencyOutFilePath)
}

// GetUPFLatencyStats Returns the memory manager's latency stats
func (o *Orchestrator) GetUPFLatencyStats(vmID string) ([]*metrics.Metric, error) {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received DumpUPFPageStats")

	return o.memoryManager.GetUPFLatencyStats(vmID)
}

func (o *Orchestrator) setupHeartbeat() {
	heartbeat := time.NewTicker(60 * time.Second)

	go func() {
		for {
			<-heartbeat.C
			log.Info("HEARTBEAT: number of active VMs: ", len(o.vmPool.GetVMMap()))
		} // for
	}() // go func
}
