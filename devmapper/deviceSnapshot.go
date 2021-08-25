package devmapper

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type DeviceSnapshot struct {
	sync.Mutex
	poolName           string
	deviceName         string
	deviceId           string
	mountDir           string
	mountedReadonly    bool
	numMounted         int
	numActivated       int
}

func (dsnp *DeviceSnapshot) GetDevicePath() string {
	return fmt.Sprintf("/dev/mapper/%s", dsnp.deviceName)
}

func (dsnp *DeviceSnapshot) getPoolPath() string {
	return fmt.Sprintf("/dev/mapper/%s", dsnp.poolName)
}

func NewDeviceSnapshot(poolName, deviceName, deviceId string) *DeviceSnapshot {
	dsnp := new(DeviceSnapshot)
	dsnp.poolName = poolName
	dsnp.deviceName = deviceName
	dsnp.deviceId = deviceId
	dsnp.mountDir = ""
	dsnp.mountedReadonly = false
	dsnp.numMounted = 0
	dsnp.numActivated = 0
	return dsnp
}

func (dsnp *DeviceSnapshot) Activate() error {
	dsnp.Lock()
	defer dsnp.Unlock()

	if dsnp.numActivated == 0 {
		tableEntry := fmt.Sprintf("0 20971520 thin %s %s", dsnp.getPoolPath(), dsnp.deviceId)

		cmd := exec.Command("sudo", "dmsetup", "create", dsnp.deviceName, "--table", fmt.Sprintf("%s", tableEntry))
		err := cmd.Run()
		if err != nil {
			return errors.Wrapf(err, "activating snapshot %s", dsnp.deviceName)
		}

	}

	dsnp.numActivated += 1

	return nil
}

func (dsnp *DeviceSnapshot) Deactivate() error {
	dsnp.Lock()
	defer dsnp.Unlock()

	if dsnp.numActivated == 1 {
		cmd := exec.Command("sudo", "dmsetup", "remove", dsnp.deviceName)
		err := cmd.Run()
		if err != nil {
			return errors.Wrapf(err, "deactivating snapshot %s", dsnp.deviceName)
		}
	}

	dsnp.numActivated -= 1
	return nil
}

func (dsnp *DeviceSnapshot) Mount(readOnly bool) (string, error) {
	dsnp.Lock()
	defer dsnp.Unlock()

	if dsnp.numActivated == 0 {
		return "", errors.New("failed to mount: snapshot not activated")
	}

	if dsnp.numMounted != 0 && (!dsnp.mountedReadonly || dsnp.mountedReadonly && !readOnly) {
		return "", errors.New("failed to mount: can't mount snapshot for both reading and writing")
	}

	if dsnp.numMounted == 0 {
		mountDir, err := ioutil.TempDir("", dsnp.deviceName)
		if err != nil {
			return "", err
		}
		mountDir = removeTrailingSlash(mountDir)

		err = mountExt4(dsnp.GetDevicePath(), mountDir, readOnly)
		if err != nil {
			return "", errors.Wrapf(err, "mounting %s at %s", dsnp.GetDevicePath(), mountDir)
		}
		dsnp.mountDir = mountDir
		dsnp.mountedReadonly = readOnly
	}

	dsnp.numMounted += 1

	return dsnp.mountDir, nil
}

func (dsnp *DeviceSnapshot) UnMount() error {
	dsnp.Lock()
	defer dsnp.Unlock()

	if dsnp.numMounted == 1 {
		err := unMountExt4(dsnp.mountDir)
		if err != nil {
			return errors.Wrapf(err, "unmounting %s", dsnp.mountDir)
		}

		err = os.RemoveAll(dsnp.mountDir)
		if err != nil {
			return errors.Wrapf(err, "removing %s", dsnp.mountDir)
		}
	}

	dsnp.numMounted -= 1
	dsnp.mountDir = ""
	return nil
}

func mountExt4(devicePath, mountPath string, readOnly bool) error {
	// Do not update access times for (all types of) files on this filesystem.
	// Do not allow access to devices (special files) on this filesystem.
	// Do not allow programs to be executed from this filesystem.
	// Do not honor set-user-ID and set-group-ID bits or file  capabilities when executing programs from this filesystem.
	// Suppress the display of certain (printk()) warning messages in the kernel log.
	var flags uintptr = syscall.MS_NOATIME | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_SILENT
	options := make([]string, 0)

	if readOnly {
		// Mount filesystem read-only.
		flags |= syscall.MS_RDONLY
		options = append(options, "noload")
	}

	return syscall.Mount(devicePath, mountPath, "ext4", flags, strings.Join(options, ","))
}

func unMountExt4(mountPath string) error {
	return syscall.Unmount(mountPath, syscall.MNT_DETACH)
}

func removeTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path[:len(path)-1]
	} else {
		return path
	}
}