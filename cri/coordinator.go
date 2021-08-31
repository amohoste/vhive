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

package cri

import (
	"context"
	"github.com/ease-lab/vhive/metrics"
	"github.com/ease-lab/vhive/snapshotting"
	"github.com/pkg/errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ease-lab/vhive/ctriface"
	log "github.com/sirupsen/logrus"
)

const snapshotsDir = "/fccd/snapshots"

type Coordinator struct {
	sync.Mutex
	orch   *ctriface.Orchestrator
	nextID uint64
	isSparseSnaps bool
	isMetricMode bool

	activeInstances     map[string]*FuncInstance
	withoutOrchestrator bool
	snapshotManager     *snapshotting.SnapshotManager
	metricsManager      *metrics.MetricsManager
}

type coordinatorOption func(*Coordinator)

// withoutOrchestrator is used for testing the Coordinator without calling the orchestrator
func withoutOrchestrator() coordinatorOption {
	return func(c *Coordinator) {
		c.withoutOrchestrator = true
	}
}

func newCoordinator(orch *ctriface.Orchestrator, snapsCapacityMiB int64, isSparseSnaps bool, isMetricsMode bool, opts ...coordinatorOption) *Coordinator {
	c := &Coordinator{
		activeInstances: make(map[string]*FuncInstance),
		orch:            orch,
		snapshotManager: snapshotting.NewSnapshotManager(snapshotsDir, snapsCapacityMiB),
		isSparseSnaps:   isSparseSnaps,
		metricsManager: metrics.NewMetricsManager("/fccd/metrics"),
		isMetricMode: isMetricsMode,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Called upon createcontainer CRI request
func (c *Coordinator) StartVM(ctx context.Context, image, revision string, memSizeMib, vCPUCount uint32) (*FuncInstance, error) {
	if c.orch != nil && c.orch.GetSnapshotsEnabled()  {
		if snap, err := c.snapshotManager.AcquireSnapshot(revision); err == nil {
			if snap.MemSizeMib != memSizeMib || snap.VCPUCount != vCPUCount {
				return nil, errors.New("Please create a new revision when updating uVM memory size or vCPU count")
			} else {
				return c.orchStartVMSnapshot(ctx, snap, memSizeMib, vCPUCount)
			}
		} else {
			return c.orchStartVM(ctx, image, revision, memSizeMib, vCPUCount)
		}
	}

	return c.orchStartVM(ctx, image, revision, memSizeMib, vCPUCount)
}

// Called upon removecontainer CRI request
func (c *Coordinator) StopVM(ctx context.Context, containerID string) error {
	c.Lock()

	fi, present := c.activeInstances[containerID]
	if present {
		delete(c.activeInstances, containerID)
	}

	c.Unlock()

	// Not a request to remove vm container
	if !present {
		return nil
	}

	if fi.snapBooted {
		defer c.snapshotManager.ReleaseSnapshot(fi.revisionId)
	} else if c.orch != nil && c.orch.GetSnapshotsEnabled() {
		err := c.orchCreateSnapshot(ctx, fi)
		if err != nil {
			log.Printf("Err creating snapshot %s\n", err)
		}
	}

	return c.orchStopVM(ctx, fi)
}

// for testing
func (c *Coordinator) isActive(containerID string) bool {
	c.Lock()
	defer c.Unlock()

	_, ok := c.activeInstances[containerID]
	return ok
}

// Adds an active function instance
func (c *Coordinator) InsertActive(containerID string, fi *FuncInstance) error {
	c.Lock()
	defer c.Unlock()

	logger := log.WithFields(log.Fields{"containerID": containerID, "vmID": fi.vmID})

	if fi, present := c.activeInstances[containerID]; present {
		logger.Errorf("entry for container already exists with vmID %s" + fi.vmID)
		return errors.New("entry for container already exists")
	}

	c.activeInstances[containerID] = fi

	return nil
}

func (c *Coordinator) orchStartVM(ctx context.Context, image, revision string, memSizeMib, vCPUCount uint32) (*FuncInstance, error) {
	tStartCold := time.Now()
	vmID := strconv.Itoa(int(atomic.AddUint64(&c.nextID, 1)))
	logger := log.WithFields(
		log.Fields{
			"vmID":  vmID,
			"image": image,
		},
	)

	logger.Debug("creating fresh instance")

	var (
		resp *ctriface.StartVMResponse
		err  error
	)

	bootMetric := metrics.NewBootMetric(revision)
	netMetric := metrics.NewNetMetric(revision)

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*40)
	defer cancel()

	if !c.withoutOrchestrator {
		resp, err = c.orch.StartVM(ctxTimeout, vmID, image, memSizeMib, vCPUCount, bootMetric, netMetric)
		if err != nil {
			logger.WithError(err).Error("Coordinator failed to start VM")
		}
	}

	coldStartTimeMs := metrics.ToMs(time.Since(tStartCold))
	fi := NewFuncInstance(vmID, image, revision, resp, false, memSizeMib, vCPUCount, coldStartTimeMs)
	logger.Debug("successfully created fresh instance")

	if c.isMetricMode {
		bootMetric.SnapBooted = false
		bootMetric.Failed = false
		go c.metricsManager.AddBootMetric(bootMetric)

		netMetric.Failed = false
		go c.metricsManager.AddNetMetric(netMetric)
	}

	return fi, err
}

func (c *Coordinator) orchStartVMSnapshot(ctx context.Context, snap *snapshotting.Snapshot, memSizeMib, vCPUCount uint32) (*FuncInstance, error) {
	tStartCold := time.Now()
	vmID := strconv.Itoa(int(atomic.AddUint64(&c.nextID, 1)))
	logger := log.WithFields(
		log.Fields{
			"vmID":  vmID,
			"image": snap.GetImage(),
		},
	)

	logger.Debug("loading instance from snapshot")

	var (
		resp *ctriface.StartVMResponse
		err  error
	)

	bootMetric := metrics.NewBootMetric(snap.GetRevisionId())
	netMetric := metrics.NewNetMetric(snap.GetRevisionId())

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	resp, err = c.orch.LoadSnapshot(ctxTimeout, vmID, snap, bootMetric, netMetric)
	if err != nil {
		logger.WithError(err).Error("failed to load VM")
		return nil, err
	}

	if err := c.orch.ResumeVM(ctxTimeout, vmID, bootMetric); err != nil {
		logger.WithError(err).Error("failed to load VM")
		return nil, err
	}

	coldStartTimeMs := metrics.ToMs(time.Since(tStartCold))
	fi := NewFuncInstance(vmID, snap.GetImage(), snap.GetRevisionId(), resp, true, memSizeMib, vCPUCount, coldStartTimeMs)
	logger.Debug("successfully loaded instance from snapshot")

	if c.isMetricMode {
		bootMetric.SnapBooted = true
		bootMetric.Failed = false
		go c.metricsManager.AddBootMetric(bootMetric)

		netMetric.Failed = false
		go c.metricsManager.AddNetMetric(netMetric)
	}

	return fi, err
}

func (c *Coordinator) orchCreateSnapshot(ctx context.Context, fi *FuncInstance) error {
	logger := log.WithFields(
		log.Fields{
			"vmID":  fi.vmID,
			"image": fi.image,
		},
	)

	if snap, err := c.snapshotManager.InitSnapshot(fi.revisionId, fi.image, fi.coldStartTimeMs, fi.memSizeMib, fi.vCPUCount); err == nil {
		// TODO: maybe needs to be longer
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*60)
		defer cancel()

		logger.Debug("creating instance snapshot before stopping")

		snapMetric := metrics.NewSnapMetric(fi.revisionId)

		tStart := time.Now()
		err = c.orch.PauseVM(ctxTimeout, fi.vmID)
		if err != nil {
			logger.WithError(err).Error("failed to pause VM")
			return nil
		}
		snapMetric.PauseVm = metrics.ToUS(time.Since(tStart))

		err = c.orch.CreateSnapshot(ctxTimeout, fi.vmID, snap, c.isSparseSnaps, snapMetric)
		if err != nil {
			fi.logger.WithError(err).Error("failed to create snapshot")
			return nil
		}

		if err := c.snapshotManager.CommitSnapshot(fi.revisionId); err != nil {
			fi.logger.WithError(err).Error("failed to commit snapshot")
			return err
		}

		if c.isMetricMode {
			snapMetric.Failed = false
			go c.metricsManager.AddSnapMetric(snapMetric)
		}
	} else {
		fi.logger.Warn("Not enough space for snapshot")
		return nil
	}

	return nil
}


func (c *Coordinator) orchStopVM(ctx context.Context, fi *FuncInstance) error {
	if c.withoutOrchestrator {
		return nil
	}

	if err := c.orch.StopSingleVM(ctx, fi.vmID); err != nil {
		fi.logger.WithError(err).Error("failed to stop VM for instance")
		return err
	}

	return nil
}
