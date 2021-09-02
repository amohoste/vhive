package snapshotting

import (
	"encoding/gob"
	"github.com/pkg/errors"
	"github.com/ricochet2200/go-disk-usage/du"
	"os"
	"path/filepath"
)

// Snapshot identified by revision
// Only capitalized fields are serialised / deserialised
type Snapshot struct {
	revisionId             string
	snapDir                string
	Image                  string
	MemSizeMib             uint32
	VCPUCount              uint32
	usable                 bool
	sparse                 bool

	// Eviction
	numUsing               uint32
	TotalSizeMiB           int64
	freq                   int64 // TODO: across all instances & set to zero if all isntances terminated
	coldStartTimeMs        int64
	lastUsedClock          int64
	score                  int64
}

func NewSnapshot(revisionId, baseFolder, image string, sizeMiB, coldStartTimeMs, lastUsed int64, memSizeMib, vCPUCount uint32, sparse bool) *Snapshot {
	s := &Snapshot{
		revisionId:             revisionId,
		snapDir:                filepath.Join(baseFolder, revisionId),
		Image:                  image,
		MemSizeMib:             memSizeMib,
		VCPUCount:              vCPUCount,
		usable:                 false,
		numUsing:               0,
		TotalSizeMiB:           sizeMiB,
		coldStartTimeMs:        coldStartTimeMs,
		lastUsedClock:          lastUsed, // Initialize with used now to avoid immediately removing
		sparse:                 sparse,
	}

	return s
}

// Updates disk size to real used disk size
func (snp *Snapshot) UpdateDiskSize() {
	usage := du.NewDiskUsage(snp.snapDir)
	snp.TotalSizeMiB = int64(usage.Used() / (1024 * 1024))
}

func (snp *Snapshot) UpdateScore() {
	snp.score = snp.lastUsedClock + (snp.freq * snp.coldStartTimeMs) / snp.TotalSizeMiB
}

func (snp *Snapshot) GetImage() string {
	return snp.Image
}

func (snp *Snapshot) GetRevisionId() string {
	return snp.revisionId
}

func (snp *Snapshot) GetSnapFilePath() string {
	return filepath.Join(snp.snapDir, "snapfile")
}

func (snp *Snapshot) GetSnapType() string {
	var snapType string
	if snp.sparse {
		snapType = "Diff"
	} else {
		snapType = "All"
	}
	return snapType
}

func (snp *Snapshot) GetMemFilePath() string {
	return filepath.Join(snp.snapDir, "memfile")
}

func (snp *Snapshot) GetPatchFilePath() string {
	return filepath.Join(snp.snapDir, "patchfile")
}

func (snp *Snapshot) GetInfoFilePath() string {
	return filepath.Join(snp.snapDir, "infofile")
}

func (snp *Snapshot) SerializeSnapInfo() error {
	file, err := os.Create(snp.GetInfoFilePath())
	if err != nil {
		return errors.Wrapf(err, "failed to create snapinfo file")
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)

	err = encoder.Encode(*snp)
	if err != nil {
		return errors.Wrapf(err, "failed to encode snapinfo")
	}
	return nil
}

func (snp *Snapshot) LoadSnapInfo(infoPath string) error {
	file, err := os.Open(infoPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open snapinfo file")
	}
	defer file.Close()

	encoder := gob.NewDecoder(file)

	err = encoder.Decode(snp)
	if err != nil {
		return errors.Wrapf(err, "failed to decode snapinfo")
	}

	return  nil
}