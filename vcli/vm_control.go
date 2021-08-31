package vcli

import (
	"context"
	"fmt"
	fccdcri "github.com/ease-lab/vhive/cri"
	log "github.com/sirupsen/logrus"
	"time"

	"sync"
)

type VmController struct {
	sync.Mutex
	uVms            map[string]*VmInstance

	containerId     int
	coordinator     *fccdcri.Coordinator

	safeLock        sync.Mutex
	safeUvms        map[string]*VmInstance // Updated every second so suggestions don't lock uVms & fluent
	addedUvms       []string
	deletedUvms     []string
}

func newVmController(coordinator *fccdcri.Coordinator) *VmController {
//func newVmController() *VmController {
	c := &VmController{
		uVms: make(map[string]*VmInstance),
		coordinator: coordinator,
		safeUvms: make(map[string]*VmInstance),
		containerId: 0,
		addedUvms: make([]string, 0),
		deletedUvms: make([]string, 0),
	}

	go func() {
		for range time.Tick(1 * time.Second) {
			c.copySafeUvms()
		}
	}()

	return c
}

func (c *VmController) copySafeUvms() {
	c.safeLock.Lock()
	defer c.safeLock.Unlock()
	c.Lock()
	defer c.Unlock()
	for _, k := range c.addedUvms {
		c.safeUvms[k] = c.uVms[k]
	}

	for _, k := range c.deletedUvms {
		delete(c.safeUvms, k)
	}
}

func (c *VmController) copyUvms() map[string]*VmInstance {
	uVms := make(map[string]*VmInstance)
	c.Lock()
	defer c.Unlock()
	for k, v := range c.uVms {
		uVms[k] = v
	}
	return uVms
}

func (c *VmController) delete(containerID string) {
	go func() {
		c.Lock()
		if _, present := c.uVms[containerID]; !present {
			fmt.Printf("vm with container id %s does not exist\n", containerID)
		}
		delete(c.uVms, containerID)
		c.deletedUvms = append(c.deletedUvms, containerID)
		c.Unlock()
		if err := c.coordinator.StopVM(context.Background(), containerID); err != nil {
				log.WithError(err).Error("failed to stop microVM")
		}
	}()
}


func (c *VmController) deleteAll() {
	uVms := c.copyUvms()
	for k, _ := range uVms {
		c.delete(k)
	}
}

func (c *VmController) deleteByImage(image string) {
	uVms := c.copyUvms()
	for k, uVM := range uVms {
		if uVM.image == image {
			c.delete(k)
		}
	}
}

func (c *VmController) deleteByRevision(revision string) {
	uVms := c.copyUvms()
	for k, uVM := range uVms {
		if uVM.revisionId == revision {
			c.delete(k)
		}
	}
}

func (c *VmController) list() {
	for _, uVM := range c.safeUvms {
		uVM.Print()
	}
}


func (c *VmController) create(image, revision string, memsizeMib, vCpuCount uint) {
	go func() {
		c.Lock()
		containerStr := fmt.Sprintf("%d", c.containerId)

		c.containerId += 1
		c.Unlock()

		funcInst, err := c.coordinator.StartVM(context.Background(), image, revision, uint32(memsizeMib), uint32(vCpuCount))
		if err != nil {
			fmt.Println(err)
			return
		}
		err = c.coordinator.InsertActive(containerStr, funcInst)

		c.Lock()
		fmt.Printf("%s available at %s\n", containerStr, funcInst.StartVMResponse.GuestIP)
		c.uVms[containerStr] = NewVmInstance(containerStr, image, revision, uint32(memsizeMib), uint32(vCpuCount))
		c.addedUvms = append(c.addedUvms, containerStr)
		c.Unlock()
	}()
}