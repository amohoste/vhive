package devmapper

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/snapshots"
	"github.com/opencontainers/image-spec/identity"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type DeviceMapper struct {
	sync.Mutex
	poolName           string
	snapDevices        map[string]*DeviceSnapshot // maps revision snapkey to snapshot device
	snapshotService    snapshots.Snapshotter

	// Need to create leases to avoid garbage collecting snapshots manually created through containerd.
	// Done already if using normal containerd functions (eg. container.create)
	leaseManager      leases.Manager
	leases            map[string]*leases.Lease
}

func NewDeviceMapper(client *containerd.Client, poolName string) *DeviceMapper {
	devMapper := new(DeviceMapper)
	devMapper.poolName = poolName
	devMapper.snapDevices = make(map[string]*DeviceSnapshot)
	devMapper.snapshotService = client.SnapshotService("devmapper")
	devMapper.leaseManager = client.LeasesService()
	devMapper.leases = make(map[string]*leases.Lease)
	return devMapper
}

func getImageKey(image containerd.Image, ctx context.Context) (string, error) {
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return "", err
	}
	return identity.ChainID(diffIDs).String(), nil
}

func (dmpr *DeviceMapper) CreateDeviceSnapshotFromImage(ctx context.Context, snapshotKey string, image containerd.Image) error {
	parent, err := getImageKey(image, ctx)
	if err != nil {
		return err
	}

	return dmpr.CreateDeviceSnapshot(ctx, snapshotKey, parent)
}

func (dmpr *DeviceMapper) CreateDeviceSnapshot(ctx context.Context, snapKey, parentKey string) error {
	lease, err := dmpr.leaseManager.Create(ctx, leases.WithID(snapKey))
	if err != nil {
		return err
	}

	leasedCtx := leases.WithLease(ctx, lease.ID)
	mounts, err := dmpr.snapshotService.Prepare(leasedCtx, snapKey, parentKey)
	if err != nil {
		return err
	}

	// Devmapper always only has a single mount /dev/mapper/fc-thinpool-snap-x
	deviceName := filepath.Base(mounts[0].Source)
	info, err := dmpr.snapshotService.Stat(ctx, snapKey)
	if err != nil {
		return err
	}

	dmpr.Lock()
	dsnp := NewDeviceSnapshot(dmpr.poolName, deviceName, info.SnapshotDev)
	dsnp.numActivated = 1
	dmpr.snapDevices[snapKey] = dsnp
	dmpr.leases[snapKey] = &lease
	dmpr.Unlock()
	return nil
}

// Only do for container snapshots, else locking not correct
func (dmpr *DeviceMapper) RemoveDeviceSnapshot(ctx context.Context, snapKey string) error {
	dmpr.Lock()

	lease, present := dmpr.leases[snapKey]
	if ! present {
		return errors.New(fmt.Sprintf("Delete device snapshot: lease for key %s does not exist", snapKey))
	}

	if _, present := dmpr.snapDevices[snapKey]; !present {
		return errors.New(fmt.Sprintf("Delete device snapshot: device for key %s does not exist", snapKey))
	}
	delete(dmpr.snapDevices, snapKey)
	delete(dmpr.leases, snapKey)
	dmpr.Unlock()

	// Not only deactivates but also deletes device
	err := dmpr.snapshotService.Remove(ctx, snapKey)
	if err != nil {
		return err
	}

	if err := dmpr.leaseManager.Delete(ctx, *lease); err != nil {
		return err
	}

	return nil
}

func (dmpr *DeviceMapper) GetImageSnapshot(ctx context.Context, image containerd.Image) (*DeviceSnapshot, error) {
	imageSnapKey, err := getImageKey(image, ctx)
	if err != nil {
		return nil, err
	}

	return dmpr.GetDeviceSnapshot(ctx, imageSnapKey)
}

// TODO: Could do locking more efficiently
func (dmpr *DeviceMapper) GetDeviceSnapshot(ctx context.Context, snapKey string) (*DeviceSnapshot, error) {
	dmpr.Lock()
	defer dmpr.Unlock()
	_, present := dmpr.snapDevices[snapKey]

	if !present {
		info, err := dmpr.snapshotService.Stat(ctx, snapKey)
		if err != nil {
			return nil, err
		}
		deviceName := getDeviceName(dmpr.poolName, info.SnapshotId)

		dsnp := NewDeviceSnapshot(dmpr.poolName, deviceName, info.SnapshotDev)
		if _, err := os.Stat(dsnp.GetDevicePath()); err == nil {
			// Snapshot already activated
			dsnp.numActivated = 1
		}

		dmpr.snapDevices[snapKey] = dsnp
	}

	return dmpr.snapDevices[snapKey], nil
}

func addTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	} else {
		return path + "/"
	}
}

func extractPatch(imageMountPath, containerMountPath, patchPath string) error {
	patchArg := fmt.Sprintf("--only-write-batch=%s", patchPath)
	cmd := exec.Command("sudo", "rsync", "-ar", patchArg, addTrailingSlash(imageMountPath), addTrailingSlash(containerMountPath))
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "creating patch between %s and %s at %s", imageMountPath, containerMountPath, patchPath)
	}

	err = os.Remove(patchPath + ".sh") // Remove unnecessary script output
	if err!= nil {
		return errors.Wrapf(err, "removing %s", patchPath + ".sh")
	}
	return nil
}

// CreatePatch creates a patch file storing the difference between an image and the container filesystem
func (dmpr *DeviceMapper) CreatePatch(ctx context.Context, patchPath, containerSnapKey string, image containerd.Image) error {
	containerSnap, err := dmpr.GetDeviceSnapshot(ctx, containerSnapKey)
	if err != nil {
		return err
	}

	imageSnap, err := dmpr.GetImageSnapshot(ctx, image)
	if err != nil {
		return err
	}

	// 1. Activate image snapshot
	err = imageSnap.Activate()
	if err != nil {
		return errors.Wrapf(err, "failed to activate image snapshot")
	}
	defer imageSnap.Deactivate()

	// 2. Mount original and snapshot image
	imageMountPath, err := imageSnap.Mount(true)
	if err != nil {
		return err
	}
	defer imageSnap.UnMount()

	containerMountPath, err := containerSnap.Mount(true)
	if err != nil {
		return err
	}
	defer containerSnap.UnMount()

	// 3. Save changes to file
	return extractPatch(imageMountPath, containerMountPath, patchPath)
}

func applyPatch(containerMountPath, patchPath string) error {
	patchArg := fmt.Sprintf("--read-batch=%s", patchPath)
	cmd := exec.Command("sudo", "rsync", "-ar", patchArg, addTrailingSlash(containerMountPath))
	err := cmd.Run()
	if err!= nil {
		return errors.Wrapf(err, "applying %s at %s", patchPath, containerMountPath)
	}
	return nil
}

// Apply changes on top of container layer
func (dmpr *DeviceMapper) RestorePatch(ctx context.Context, containerSnapKey, patchPath string) error {
	containerSnap, err := dmpr.GetDeviceSnapshot(ctx, containerSnapKey)
	if err != nil {
		return err
	}

	// 1. Mount container snapshot device
	containerMountPath, err := containerSnap.Mount(false)
	if err != nil {
		return err
	}
	defer containerSnap.UnMount()

	// 2. Apply changes to container mounted file system
	return applyPatch(containerMountPath, patchPath)
}

func getDeviceName(poolName, snapshotId string) string {
	return fmt.Sprintf("%s-snap-%s", poolName, snapshotId)
}