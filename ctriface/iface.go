// MIT License
//
// Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
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
	"context"
	"fmt"
	"github.com/ease-lab/vhive/snapshotting"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/firecracker-microvm/firecracker-containerd/proto" // note: from the original repo
	"github.com/firecracker-microvm/firecracker-containerd/runtime/firecrackeroci"
	"github.com/pkg/errors"

	_ "google.golang.org/grpc/codes"  //tmp
	_ "google.golang.org/grpc/status" //tmp

	_ "github.com/davecgh/go-spew/spew" //tmp
	"github.com/ease-lab/vhive/metrics"
	"github.com/ease-lab/vhive/misc"
)

// StartVMResponse is the response returned by StartVM
// TODO: Integrate response with non-cri API
type StartVMResponse struct {
	// GuestIP is the IP of the guest MicroVM
	GuestIP string
}

const (
	testImageName = "vhiveease/helloworld:var_workload"
)

// StartVM Boots a VM if it does not exist
func (o *Orchestrator) StartVM(ctx context.Context, vmID, imageName string, memSizeMib ,vCPUCount uint32) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		startVMMetric *metrics.Metric = metrics.NewMetric()
		tStart        time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID, "image": imageName})
	logger.Debug("StartVM: Received StartVM")

	// 1. Allocate VM metadata & create vm network
	tStart = time.Now()
	vm, err := o.vmPool.Allocate(vmID)
	if err != nil {
		logger.Error("failed to allocate VM in VM pool")
		return nil, nil, err
	}
	vm.VCPUCount = vCPUCount
	vm.MemSizeMib = memSizeMib
	startVMMetric.MetricMap[metrics.AllocateVM] = metrics.ToUS(time.Since(tStart))

	defer func() {
		// Free the VM from the pool if function returns error
		if retErr != nil {
			if err := o.vmPool.Free(vmID); err != nil {
				logger.WithError(err).Errorf("failed to free VM from pool after failure")
			}
		}
	}()

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	// 2. Fetch VM image
	tStart = time.Now()
	if vm.Image, err = o.getImage(ctx, imageName); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to get/pull image")
	}
	startVMMetric.MetricMap[metrics.GetImage] = metrics.ToUS(time.Since(tStart))

	// 3. Create VM
	tStart = time.Now()
	conf := o.getVMConfig(vm)
	_, err = o.fcClient.CreateVM(ctx, conf)
	startVMMetric.MetricMap[metrics.FcCreateVM] = metrics.ToUS(time.Since(tStart))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create the microVM in firecracker-containerd")
	}

	defer func() {
		if retErr != nil {
			if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
				logger.WithError(err).Errorf("failed to stop firecracker-containerd VM after failure")
			}
		}
	}()

	// 3. Create container
	logger.Debug("StartVM: Creating a new container")
	tStart = time.Now()
	container, err := o.client.NewContainer(
		ctx,
		vm.ContainerSnapKey,
		containerd.WithSnapshotter(o.snapshotter),
		containerd.WithNewSnapshot(vm.ContainerSnapKey, *vm.Image),
		containerd.WithNewSpec(
			oci.WithImageConfig(*vm.Image),
			firecrackeroci.WithVMID(vmID),
			firecrackeroci.WithVMNetwork,
		),
		containerd.WithRuntime("aws.firecracker", nil),
	)
	startVMMetric.MetricMap[metrics.NewContainer] = metrics.ToUS(time.Since(tStart))
	vm.Container = &container
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create a container")
	}

	defer func() {
		if retErr != nil {
			if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
				logger.WithError(err).Errorf("failed to delete container after failure")
			}
		}
	}()

	// 4. Turn container into runnable process
	logger.Debug("StartVM: Creating a new task")
	tStart = time.Now()
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	startVMMetric.MetricMap[metrics.NewTask] = metrics.ToUS(time.Since(tStart))
	vm.Task = &task
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create a task")
	}

	defer func() {
		if retErr != nil {
			if _, err := task.Delete(ctx); err != nil {
				logger.WithError(err).Errorf("failed to delete task after failure")
			}
		}
	}()

	// 5. Wait for task to get ready
	logger.Debug("StartVM: Waiting for the task to get ready")
	tStart = time.Now()
	ch, err := task.Wait(ctx)
	startVMMetric.MetricMap[metrics.TaskWait] = metrics.ToUS(time.Since(tStart))
	vm.ExitStatusCh = ch
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for a task")
	}

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	// 6. Start process inside the container
	logger.Debug("StartVM: Starting the task")
	tStart = time.Now()
	if err := task.Start(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start a task")
	}
	startVMMetric.MetricMap[metrics.TaskStart] = metrics.ToUS(time.Since(tStart))

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	logger.Debug("Successfully started a VM")

	return &StartVMResponse{GuestIP: vm.NetConfig.GetCloneIP()}, startVMMetric, nil
}



// StopSingleVM Shuts down a VM
// Note: VMs are not quisced before being stopped
func (o *Orchestrator) StopSingleVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received StopVM")

	fmt.Printf("Stopping vm %s\n", vmID)

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		if _, ok := err.(*misc.NonExistErr); ok {
			logger.Panic("StopVM: VM does not exist")
		}
		logger.Panic("StopVM: GetVM() failed for an unknown reason")

	}

	fmt.Printf("Found vm %s\n", vmID)

	logger = log.WithFields(log.Fields{"vmID": vmID})

	if ! vm.SnapBooted {
		fmt.Printf("Killing task %s\n", *vm.Task)
		task := *vm.Task
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			logger.WithError(err).Error("Failed to kill the task")
			return err
		}

		fmt.Printf("Waiting for task exit\n")
		<-vm.ExitStatusCh
		//FIXME: Seems like some tasks need some extra time to die Issue#15, lr_training
		time.Sleep(500 * time.Millisecond)

		fmt.Println("Deleting task")
		if _, err := task.Delete(ctx); err != nil {
			logger.WithError(err).Error("failed to delete task")
			return err
		}

		fmt.Println("Deleting container")
		container := *vm.Container
		if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			logger.WithError(err).Error("failed to delete container")
			return err
		}
	}
	fmt.Println("Stopping vm")

	if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to stop firecracker-containerd VM")
		return err
	}
	fmt.Println("Freeing vm")

	if err := o.vmPool.Free(vmID); err != nil {
		logger.Error("failed to free VM from VM pool")
		return err
	}

	if vm.SnapBooted {
		fmt.Println("Removing device snapshot")

		if err := o.devMapper.RemoveDeviceSnapshot(ctx, vm.ContainerSnapKey); err != nil {
			logger.Error("failed to deactivate container snapshot")
			return err
		}
	}

	fmt.Println("Stopped VM successfully")

	logger.Debug("Stopped VM successfully")

	return nil
}

// Checks whether a URL has a .local domain
func isLocalDomain(s string) (bool, error) {
	if ! strings.Contains(s, "://") {
		s = "dummy://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return false, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	i := strings.LastIndex(host, ".")
	tld := host[i+1:]

	return tld == "local", nil
}

// Converts an image name to a url if it is not a URL
func getImageURL(image string) string {
	// Pull from dockerhub by default if not specified (default k8s behavior)
	if strings.Contains(image, ".") {
		return image
	}
	return "docker.io/" + image
	
}

func (o *Orchestrator) getImage(ctx context.Context, imageName string) (*containerd.Image, error) {
	image, found := o.cachedImages[imageName]
	if !found {
		var err error
		log.Debug(fmt.Sprintf("Pulling image %s", imageName))

		imageURL := getImageURL(imageName)
		local, _ := isLocalDomain(imageURL)
		if local {
			// Pull local image using HTTP
			resolver := docker.NewResolver(docker.ResolverOptions{
				Client: http.DefaultClient,
				Hosts: docker.ConfigureDefaultRegistries(
					docker.WithPlainHTTP(docker.MatchAllHosts),
				),
			})
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
				containerd.WithResolver(resolver),
			)
		} else {
			// Pull remote image
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
			)
		}

		if err != nil {
			return &image, err
		}
		o.cachedImages[imageName] = image
	}

	return &image, nil
}

func (o *Orchestrator) getVMConfig(vm *misc.VM) *proto.CreateVMRequest {
	kernelArgs := "ro noapic reboot=k panic=1 pci=off nomodules systemd.log_color=false systemd.unit=firecracker.target init=/sbin/overlay-init tsc=reliable quiet 8250.nr_uarts=0 ipv6.disable=1"

	return &proto.CreateVMRequest{
		VMID:           vm.ID,
		TimeoutSeconds: 100,
		KernelArgs:     kernelArgs,
		MachineCfg: &proto.FirecrackerMachineConfiguration{
			VcpuCount:  vm.VCPUCount,
			MemSizeMib: vm.MemSizeMib,
		},
		NetworkInterfaces: []*proto.FirecrackerNetworkInterface{{
			StaticConfig: &proto.StaticNetworkConfiguration{
				MacAddress:  vm.NetConfig.GetMacAddress(),
				HostDevName: vm.NetConfig.GetHostDevName(),
				IPConfig: &proto.IPConfiguration{
					PrimaryAddr: vm.NetConfig.GetContainerCIDR(),
					GatewayAddr: vm.NetConfig.GetGatewayIP(),
					Nameservers: []string{"8.8.8.8"},
				},
			},
		}},
		NetworkNamespace: vm.NetConfig.GetNamespacePath(),
	}
}

// StopActiveVMs Shuts down all active VMs
func (o *Orchestrator) StopActiveVMs() error {
	var vmGroup sync.WaitGroup
	for vmID, vm := range o.vmPool.GetVMMap() {
		vmGroup.Add(1)
		logger := log.WithFields(log.Fields{"vmID": vmID})
		go func(vmID string, vm *misc.VM) {
			defer vmGroup.Done()
			err := o.StopSingleVM(context.Background(), vmID)
			if err != nil {
				logger.Warn(err)
			}
		}(vmID, vm)
	}

	log.Info("waiting for goroutines")
	vmGroup.Wait()
	log.Info("waiting done")

	log.Info("Closing fcClient")
	o.fcClient.Close()
	log.Info("Closing containerd client")
	o.client.Close()

	return nil
}

// PauseVM Pauses a VM
func (o *Orchestrator) PauseVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received PauseVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	if _, err := o.fcClient.PauseVM(ctx, &proto.PauseVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to pause the VM")
		return err
	}

	return nil
}

// ResumeVM Resumes a VM
func (o *Orchestrator) ResumeVM(ctx context.Context, vmID string) (*metrics.Metric, error) {
	var (
		resumeVMMetric *metrics.Metric = metrics.NewMetric()
		tStart         time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received ResumeVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	tStart = time.Now()
	if _, err := o.fcClient.ResumeVM(ctx, &proto.ResumeVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to resume the VM")
		return nil, err
	}
	resumeVMMetric.MetricMap[metrics.FcResume] = metrics.ToUS(time.Since(tStart))

	return resumeVMMetric, nil
}

// CreateSnapshot Creates a snapshot of a VM
func (o *Orchestrator) CreateSnapshot(ctx context.Context, vmID string, snap *snapshotting.Snapshot, sparse bool) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received CreateSnapshot")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	// 1. Get VM metadata
	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		return err
	}

	// 2. Create VM & VM memory state snapshot
	req := &proto.CreateSnapshotRequest{
		VMID:             vmID,
		SnapshotFilePath: snap.GetSnapFilePath(),
		MemFilePath:      snap.GetMemFilePath(),
	}

	if _, err := o.fcClient.CreateSnapshot(ctx, req); err != nil {
		logger.WithError(err).Error("failed to create snapshot of the VM")
		return err
	}

	// 3. Backup disk state difference
	if err := o.devMapper.CreatePatch(ctx, snap.GetPatchFilePath(), vm.ContainerSnapKey, *vm.Image); err != nil {
		logger.WithError(err).Error("failed to create container patch file")
		return err
	}

	// 4. Serialize snapshot info
	if err := snap.SerializeSnapInfo(); err != nil {
		logger.WithError(err).Error("failed to serialize snapshot info")
		return err
	}

	// 5. Make snapshot memory file sparse
	if sparse {
		if err = snap.SparsifyMemfile(); err != nil {
			logger.WithError(err).Error("failed to make memfile sparse")
			return err
		}
	}

	return nil
}

// LoadSnapshot Loads a snapshot of a VM TODO: correct defer to undo stuff
// TODO: could do stuf in parallel as in vhive, also more generally maybe
func (o *Orchestrator) LoadSnapshot(ctx context.Context, vmID string, snap *snapshotting.Snapshot) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		loadSnapshotMetric   *metrics.Metric = metrics.NewMetric() // TODO: update metrics
		tStart               time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID, "image": snap.GetImage()})
	logger.Debug("StartVM: Received StartVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	// 1. Allocate VM metadata & create vm network
	tStart = time.Now()
	vm, err := o.vmPool.Allocate(vmID)
	if err != nil {
		logger.Error("failed to allocate VM in VM pool")
		return nil, nil, err
	}
	loadSnapshotMetric.MetricMap[metrics.AllocateVM] = metrics.ToUS(time.Since(tStart))

	defer func() {
		// Free the VM from the pool if function returns error
		if retErr != nil {
			if err := o.vmPool.Free(vmID); err != nil {
				logger.WithError(err).Errorf("failed to free VM from pool after failure")
			}
		}
	}()

	// 2. Fetch image for VM
	tStart = time.Now()
	if vm.Image, err = o.getImage(ctx, snap.GetImage()); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to get/pull image")
	}
	loadSnapshotMetric.MetricMap[metrics.GetImage] = metrics.ToUS(time.Since(tStart))

	// 3. Create snapshot for container to run
	tStart = time.Now()
	if err := o.devMapper.CreateDeviceSnapshotFromImage(ctx, vm.ContainerSnapKey, *vm.Image); err != nil {
		return nil, nil, errors.Wrapf(err, "creating container snapshot")
	}
	loadSnapshotMetric.MetricMap[metrics.CreateDeviceSnap] = metrics.ToUS(time.Since(tStart))

	containerSnap, err := o.devMapper.GetDeviceSnapshot(ctx, vm.ContainerSnapKey)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "previously created container device does not exist")
	}

	// 5. Unpack patch into container snapshot
	tStart = time.Now()
	if err := o.devMapper.RestorePatch(ctx, vm.ContainerSnapKey, snap.GetPatchFilePath()); err != nil {
		return nil, nil, errors.Wrapf(err, "unpacking patch into container snapshot")
	}
	loadSnapshotMetric.MetricMap[metrics.RestorePatch] = metrics.ToUS(time.Since(tStart))

	// 6. Load VM from snapshot
	req := &proto.LoadSnapshotRequest{
		VMID:             vmID,
		SnapshotFilePath: snap.GetSnapFilePath(),
		MemFilePath:      snap.GetMemFilePath(),
		EnableUserPF:     false,
		NetworkNamespace: vm.NetConfig.GetNamespacePath(),
		NewSnapshotPath:  containerSnap.GetDevicePath(),
	}

	tStart = time.Now()
	_, err = o.fcClient.LoadSnapshot(ctx, req)
	if err != nil {
		logger.Error("Failed to load snapshot of the VM: ", err)
		return nil, nil, err
	}
	loadSnapshotMetric.MetricMap[metrics.FcLoadSnapshot] = metrics.ToUS(time.Since(tStart))

	vm.SnapBooted = true

	return &StartVMResponse{GuestIP: vm.NetConfig.GetCloneIP()}, loadSnapshotMetric, nil
}
