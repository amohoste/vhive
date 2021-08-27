package snapshotting

import (
	"container/heap"
	"fmt"
	"github.com/pkg/errors"
	"math"
	"os"
	"sync"
)

// TODO: fetch snapshot from remote  if needed and viable. Keep global snapshot store with cold start time
// so can approximately calculate if worth to fetch now, and if worth to return but still fetch in background.
// also set usable flag for remote snapshots.

type SnapshotManager struct {
	sync.Mutex
	snapshots          map[string]*Snapshot // maps revision revisionId to snapshot
	freeSnaps          SnapHeap
	baseFolder         string

	// Eviction
	clock       int64 	// When container last used. Increased to priority terminated container on termination
	capacityMib int64
	usedMib     int64
}

func NewSnapshotManager(baseFolder string, capacityMib int64) *SnapshotManager {
	manager := new(SnapshotManager)
	manager.snapshots = make(map[string]*Snapshot)
	heap.Init(&manager.freeSnaps)
	manager.baseFolder = baseFolder
	manager.clock = 0
	manager.capacityMib = capacityMib
	manager.usedMib = 0
	return manager
}

func (mgr *SnapshotManager) AcquireSnapshot(revision string) (*Snapshot, error) {
	mgr.Lock()
	defer mgr.Unlock()

	snap, present := mgr.snapshots[revision]
	if !present {
		return nil, errors.New(fmt.Sprintf("Get: Snapshot for revision %s does not exist", revision))
	}

	if ! snap.usable {
		return nil, errors.New(fmt.Sprintf("Snapshot is not usable"))
	}

	if snap.numUsing == 0 {
		// Remove from free snaps (could be done more efficiently)
		heapIdx := 0
		for i, heapSnap := range mgr.freeSnaps {
			if heapSnap.revisionId == revision {
				heapIdx = i
				break
			}
		}
		heap.Remove(&mgr.freeSnaps, heapIdx)
	}

	snap.numUsing += 1
	snap.freq += 1
	snap.lastUsedClock = mgr.clock

	return snap, nil
}

func (mgr *SnapshotManager) ReleaseSnapshot(revision string) error {
	mgr.Lock()
	defer mgr.Unlock()

	snap, present := mgr.snapshots[revision]
	if !present {
		return errors.New(fmt.Sprintf("Get: Snapshot for revision %s does not exist", revision))
	}

	snap.numUsing -= 1

	if snap.numUsing == 0 {
		// Add to freesnaps
		snap.UpdateScore()
		heap.Push(&mgr.freeSnaps, snap)
	}

	return nil
}

// TODO: could check if want to add snapshot
// TODO: also check if want to upload to remote storage once done
// Check error to see if we should create snapshot
func (mgr *SnapshotManager) InitSnapshot(revision, image string, coldStartTimeMs int64, memSizeMib, vCPUCount uint32) (*Snapshot, error) {
	mgr.Lock()

	var estimatedSnapSizeMib = int64(math.Round(float64(memSizeMib) * 1.25))

	if _, present := mgr.snapshots[revision]; present {
		return nil, errors.New(fmt.Sprintf("Add: Snapshot for revision %s already exists", revision))
	}

	availableMib := mgr.capacityMib - mgr.usedMib
	if estimatedSnapSizeMib > availableMib {
		if err := mgr.freeSpace(estimatedSnapSizeMib - availableMib); err != nil {
			mgr.Unlock()
			return nil, err
		}
	}
	mgr.usedMib += estimatedSnapSizeMib

	snap := NewSnapshot(revision, mgr.baseFolder, image, estimatedSnapSizeMib, coldStartTimeMs, mgr.clock, memSizeMib, vCPUCount)
	mgr.snapshots[revision] = snap
	mgr.Unlock()

	err := os.Mkdir(snap.snapDir, 0755)
	if err != nil {
		return nil, errors.Wrapf(err, "creating snapDir for snapshots %s", revision)
	}

	return snap, nil
}

func (mgr *SnapshotManager) CommitSnapshot(revision string) error {
	mgr.Lock()
	snap, present := mgr.snapshots[revision]
	if !present {
		return errors.New(fmt.Sprintf("Snapshot for revision %s to commit does not exist", revision))
	}
	mgr.Unlock()

	// Calculate actual disk size used
	var sizeIncrement int64 = 0
	oldSize := snap.TotalSizeMiB
	snap.UpdateDiskSize() // Should always result in a decrease or equal!
	sizeIncrement = snap.TotalSizeMiB - oldSize

	mgr.Lock()
	defer mgr.Unlock()
	mgr.usedMib += sizeIncrement
	snap.usable = true
	snap.UpdateScore()
	heap.Push(&mgr.freeSnaps, snap)

	return nil
}

// Make sure to have lock when calling!
// TODO: might have to lock more efficiently so not locked when deleting folders
func (mgr *SnapshotManager) freeSpace(neededMib int64) error {
	var toDelete []string
	var freedMib int64 = 0

	for freedMib < neededMib && len(mgr.freeSnaps) > 0 {
		snap :=  heap.Pop(&mgr.freeSnaps).(*Snapshot)
		snap.usable = false
		toDelete = append(toDelete, snap.revisionId)
		freedMib += snap.TotalSizeMiB
	}

	// Delete snapshots resources
	for _, revisionId := range toDelete {
		snapDir := mgr.snapshots[revisionId].snapDir
		if err := os.RemoveAll(snapDir); err != nil {
			return errors.Wrapf(err, "removing snapshot snapDir %s", snapDir)
		}
	}

	// Delete snapshot map entry and update clock
	for _, revisionId := range toDelete {
		snap := mgr.snapshots[revisionId]
		snap.UpdateScore()
		if snap.score > mgr.clock {
			mgr.clock = snap.score
		}
		delete(mgr.snapshots, revisionId)
	}
	mgr.usedMib -= freedMib

	if freedMib < neededMib {
		return errors.New("There is not enough free space available")
	}

	return nil
}


/*
TODO: can use for remote snaps
func filesExist(filePaths []string) bool {
	for _, filepath := range filePaths {
		if _, err := os.Stat(filepath); os.IsNotExist(err) || err != nil {
			return false
		}
	}
	return true
}

func (mgr *SnapshotManager) RecoverSnapshots() error {
	mgr.Lock()
	defer mgr.Unlock()

	files, err := ioutil.ReadDir(mgr.baseFolder)
	if err != nil {
		return errors.Wrapf(err, "reading folders in %s", mgr.baseFolder)
	}

	for _, f := range files {
		if f.IsDir() {
			revision := f.Name()
			snapshot := NewSnapshot(revision, mgr.baseFolder, "")

			if filesExist([]string{snapshot.GetSnapFilePath(), snapshot.GetPatchFilePath(), snapshot.GetInfoFilePath(), snapshot.GetMemFilePath()}) {
				err = snapshot.LoadSnapInfo(snapshot.GetInfoFilePath())
				if err != nil {
					return errors.Wrapf(err, "recovering snapshot %s", f.Name())
				}
				mgr.snapshots[revision] = snapshot
			}
		}
	}
	return nil
}*/