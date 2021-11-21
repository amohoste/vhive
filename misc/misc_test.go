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

package misc

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	ctrdlog "github.com/containerd/containerd/log"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	os.Exit(m.Run())
}

func TestAllocateFreeVMs(t *testing.T) {
	vmPool := NewVMPool("", 10)

	vmIDs := [2]string{"test1", "test2"}

	for _, vmID := range vmIDs {
		_, err := vmPool.Allocate(vmID)
		require.NoError(t, err, "Failed to allocate VM")
	}

	for _, vmID := range vmIDs {
		err := vmPool.Free(vmID)
		require.NoError(t, err, "Failed to free a VM")
	}

	vmPool.CleanupNetwork()
}

func TestAllocateFreeVMsParallel(t *testing.T) {
	vmNum := 100

	vmPool := NewVMPool("", 10)

	var vmGroup sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroup.Add(1)
		go func(i int) {
			defer vmGroup.Done()
			vmID := fmt.Sprintf("test_%d", i)
			_, err := vmPool.Allocate(vmID)
			require.NoError(t, err, "Failed to allocate VM")
		}(i)
	}
	vmGroup.Wait()

	var vmGroupFree sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroupFree.Add(1)
		go func(i int) {
			defer vmGroupFree.Done()
			vmID := fmt.Sprintf("test_%d", i)
			err := vmPool.Free(vmID)
			require.NoError(t, err, "Failed to free a VM")
		}(i)
	}
	vmGroupFree.Wait()

	vmPool.CleanupNetwork()
}

func TestReuseTaps(t *testing.T) {
	vmNum := 100

	vmPool := NewVMPool("", 10)

	var vmGroup sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroup.Add(1)
		go func(i int) {
			defer vmGroup.Done()
			vmID := fmt.Sprintf("test_%d", i)
			_, err := vmPool.Allocate(vmID)
			require.NoError(t, err, "Failed to allocate VM")
		}(i)
	}
	vmGroup.Wait()

	var vmGroupFree sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroupFree.Add(1)
		go func(i int) {
			defer vmGroupFree.Done()
			vmID := fmt.Sprintf("test_%d", i)
			err := vmPool.Free(vmID)
			require.NoError(t, err, "Failed to free a VM")
		}(i)
	}
	vmGroupFree.Wait()

	var vmGroupRecreate sync.WaitGroup

	tStart := time.Now()

	for i := 0; i < vmNum; i++ {
		vmGroupRecreate.Add(1)
		go func(i int) {
			defer vmGroupRecreate.Done()
			vmID := fmt.Sprintf("test_%d", i)
			err := vmPool.Free(vmID)
			require.NoError(t, err, "Failed to recreate tap")
		}(i)
	}
	vmGroupRecreate.Wait()

	tElapsed := time.Since(tStart)
	log.Infof("Recreated %d taps in %d ms", vmNum, tElapsed.Milliseconds())

	var vmGroupCleanup sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroupCleanup.Add(1)
		go func(i int) {
			defer vmGroupCleanup.Done()
			vmID := fmt.Sprintf("test_%d", i)
			err := vmPool.Free(vmID)
			require.NoError(t, err, "Failed to free a VM")
		}(i)
	}
	vmGroupCleanup.Wait()

	vmPool.CleanupNetwork()
}
