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
	"errors"
	"fmt"
	"github.com/ease-lab/vhive/metrics"
	log "github.com/sirupsen/logrus"
	criapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"time"
)

const (
	userContainerName = "user-container"
	queueProxyName    = "queue-proxy"
	guestIPEnv        = "GUEST_ADDR"
	guestPortEnv      = "GUEST_PORT"
	guestImageEnv     = "GUEST_IMAGE"
	snapshotNameEnv   = "SNAPSHOT_NAME"
	initTypeEnv		  = "INIT_TYPE"
	blockStoreEnv	  = "BLOCKS_STORE"
	guestPortValue    = "50051"
)

// CreateContainer starts a container or a VM, depending on the name
// if the name matches "user-container", the cri plugin starts a VM, assigning it an IP,
// otherwise starts a regular container
func (s *Service) CreateContainer(ctx context.Context, r *criapi.CreateContainerRequest) (*criapi.CreateContainerResponse, error) {
	log.Debugf("CreateContainer within sandbox %q for container %+v",
		r.GetPodSandboxId(), r.GetConfig().GetMetadata())

	config := r.GetConfig()
	containerName := config.GetMetadata().GetName()

	if containerName == userContainerName {
		return s.createUserContainer(ctx, r)
	}
	if containerName == queueProxyName {
		return s.createQueueProxy(ctx, r)
	}

	// Containers relevant for control plane
	return s.stockRuntimeClient.CreateContainer(ctx, r)
}

func (s *Service) createUserContainer(ctx context.Context, r *criapi.CreateContainerRequest) (*criapi.CreateContainerResponse, error) {
	var (
		stockResp *criapi.CreateContainerResponse
		stockErr  error
		tStartStub    time.Time
		tStart    time.Time
		createContainerMetric *metrics.Metric = metrics.NewMetric()
		stockDone = make(chan struct{})
	)

	go func() {
		tStartStub = time.Now()
		defer close(stockDone)
		stockResp, stockErr = s.stockRuntimeClient.CreateContainer(ctx, r)
		createContainerMetric.MetricMap["stub"] = metrics.ToUS(time.Since(tStartStub))
	}()

	tStart = time.Now()
	config := r.GetConfig()
	guestImage, err := getGuestImage(config)
	if err != nil {
		log.WithError(err).Error()
		return nil, err
	}

	log.Info(fmt.Sprintf("Create container for %s", guestImage))

	//remoteSnapshotName, _ := getRemoteSnapshotInfo(config)
	//initType, _ := getInitInfo(config)
	//blockStoreUrl, _ := getBlockStoreUrl(config)

	// funcInst, err := s.coordinator.startVM(context.Background(), guestImage, remoteSnapshotName, initType, blockStoreUrl)
	funcInst, err := s.coordinator.startVM(context.Background(), guestImage)
	if err != nil {
		log.WithError(err).Error("failed to start VM")
		return nil, err
	}

	vmConfig := &VMConfig{guestIP: funcInst.startVMResponse.GuestIP, guestPort: guestPortValue}
	s.insertPodVMConfig(r.GetPodSandboxId(), vmConfig)
	createContainerMetric.MetricMap["function"] = metrics.ToUS(time.Since(tStart))

	// Wait for placeholder UC to be created
	<-stockDone

	containerdID := stockResp.ContainerId
	err = s.coordinator.insertActive(containerdID, funcInst)
	if err != nil {
		log.WithError(err).Error("failed to insert active VM")
		return nil, err
	}

	log.Info(fmt.Sprintf("	Total create stub container: %d", createContainerMetric.MetricMap["stub"]))
	log.Info(fmt.Sprintf("	Total create function container: %d", createContainerMetric.MetricMap["function"]))

	return stockResp, stockErr
}

func (s *Service) createQueueProxy(ctx context.Context, r *criapi.CreateContainerRequest) (*criapi.CreateContainerResponse, error) {
	vmConfig, err := s.getPodVMConfig(r.GetPodSandboxId())
	if err != nil {
		log.WithError(err).Error()
		return nil, err
	}

	s.removePodVMConfig(r.GetPodSandboxId())

	guestIPKeyVal := &criapi.KeyValue{Key: guestIPEnv, Value: vmConfig.guestIP}
	guestPortKeyVal := &criapi.KeyValue{Key: guestPortEnv, Value: vmConfig.guestPort}
	r.Config.Envs = append(r.Config.Envs, guestIPKeyVal, guestPortKeyVal)

	resp, err := s.stockRuntimeClient.CreateContainer(ctx, r)
	if err != nil {
		log.WithError(err).Error("stock containerd failed to start UC")
		return nil, err
	}

	return resp, nil
}

func getGuestImage(config *criapi.ContainerConfig) (string, error) {
	envs := config.GetEnvs()
	for _, kv := range envs {
		if kv.GetKey() == guestImageEnv {
			return kv.GetValue(), nil
		}

	}

	return "", errors.New("failed to provide non empty guest image in user container config")

}

/*func getInitInfo(config *criapi.ContainerConfig) (string, error) {
	envs := config.GetEnvs()
	for _, kv := range envs {
		if kv.GetKey() == initTypeEnv {
			return kv.GetValue(), nil
		}
	}

	return "", errors.New("failed to provide non empty guest image in user container config")
}

func getBlockStoreUrl(config *criapi.ContainerConfig) (string, error) {
	envs := config.GetEnvs()
	for _, kv := range envs {
		if kv.GetKey() == blockStoreEnv {
			return kv.GetValue(), nil
		}
	}

	return "", errors.New("failed to provide non empty guest image in user container config")
}

func getRemoteSnapshotInfo(config *criapi.ContainerConfig) (string, error) {
	envs := config.GetEnvs()
	for _, kv := range envs {
		if kv.GetKey() == snapshotNameEnv {
			return kv.GetValue(), nil
		}
	}

	return "", errors.New("failed to provide non empty guest image in user container config")
}*/
