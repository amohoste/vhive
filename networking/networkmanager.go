package networking

import (
	"bufio"
	"bytes"
	"github.com/ease-lab/vhive/metrics"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type NetworkManager struct {
	sync.Mutex

	// Each VM has a network id. Each ID gets used for the veth pair and generating IP addresses
	freeIDs        []int
	nextID          int
	netConfigs      map[string]*NetworkConfig // Maps vmIDs to their network config
	hostIfaceName   string

}

func getHostIfaceName() (string, error) {
	out, err := exec.Command(
		"route",
	).Output()
	if err != nil {
		log.Warnf("Failed to fetch host net interfaces %v\n%s\n", err, out)
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "default") {
			return line[strings.LastIndex(line, " ")+1:], nil
		}
	}
	return "", errors.New("Failed to fetch host net interface")
}

func NewNetworkManager(hostIfaceName string) (*NetworkManager, error) {
	log.Info("Creating network manager")
	manager := new(NetworkManager)

	if hostIfaceName == "" {
		hostIface, err := getHostIfaceName()
		if err != nil {
			return nil, err
		} else {
			manager.hostIfaceName = hostIface
		}
	}

	manager.netConfigs = make(map[string]*NetworkConfig)
	manager.freeIDs = make([]int, 0)

	startId, err := getNetworkStartID()
	if err == nil {
		manager.nextID = startId
	} else {
		manager.nextID = 0
	}

	return manager, nil
}

func (mgr *NetworkManager) createNetConfig(vmID string) {
	mgr.Lock()
	defer mgr.Unlock()

	var id int
	if len(mgr.freeIDs) == 0 {
		id = mgr.nextID
		mgr.nextID += 1
	} else {
		id = mgr.freeIDs[len(mgr.freeIDs)-1]
		mgr.freeIDs = mgr.freeIDs[:len(mgr.freeIDs)-1]
	}

	mgr.netConfigs[vmID] = NewNetworkConfig(id)
}

func (mgr *NetworkManager) removeNetConfig(vmID string) {
	mgr.Lock()
	defer mgr.Unlock()

	config := mgr.netConfigs[vmID]
	mgr.freeIDs = append(mgr.freeIDs, config.id)
	delete(mgr.netConfigs, vmID)
}

func (mgr *NetworkManager) CreateNetwork(vmID string, netMetric *metrics.NetMetric) error {
	var (
		tStart               time.Time
	)

	// Create network config for VM
	tStart = time.Now()
	mgr.createNetConfig(vmID)
	netCfg := mgr.GetConfig(vmID)
	netMetric.CreateConfig = metrics.ToUS(time.Since(tStart))

	// Lock the OS Thread so we don't accidentally switch namespaces
	tStart = time.Now()
	runtime.LockOSThread()
	netMetric.LockOsThread = metrics.ToUS(time.Since(tStart))

	// 0. Get host network namespace
	tStart = time.Now()
	hostNsHandle, err := netns.Get()
	defer hostNsHandle.Close()
	if err != nil {
		log.Printf("Failed to get host ns, %s\n", err)
		return err
	}
	netMetric.GetHostNs = metrics.ToUS(time.Since(tStart))

	// A. In uVM netns
	// A.1. Create network namespace for uVM & join network namespace
	tStart = time.Now()
	vmNsHandle, err := netns.NewNamed(netCfg.getNamespaceName()) // Switches namespace
	if err != nil {
		log.Println(err)
		return err
	}
	defer vmNsHandle.Close()
	netMetric.CreateVmNs = metrics.ToUS(time.Since(tStart))

	// A.2. Create tap device for uVM
	tStart = time.Now()
	if err := createTap(netCfg.containerTap, netCfg.gatewayCIDR, netCfg.getNamespaceName()); err != nil {
		return err
	}
	netMetric.CreateVmTap = metrics.ToUS(time.Since(tStart))

	// A.3. Create veth pair for uVM
	// A.3.1 Create veth pair
	tStart = time.Now()
	if err := createVethPair(netCfg.getVeth0Name(), netCfg.getVeth1Name(), vmNsHandle, hostNsHandle); err != nil {
		return err
	}
	netMetric.CreateVeth = metrics.ToUS(time.Since(tStart))

	// A.3.2 Configure uVM side veth pair
	tStart = time.Now()
	if err := configVeth(netCfg.getVeth0Name(), netCfg.getVeth0CIDR()); err != nil {
		return err
	}
	netMetric.ConfigVethVm = metrics.ToUS(time.Since(tStart))

	// A.3.3 Designate host side as default gateway for packets leaving namespace
	tStart = time.Now()
	if err := setDefaultGateway(netCfg.getVeth1CIDR()); err != nil {
		return err
	}
	netMetric.SetDefaultGw = metrics.ToUS(time.Since(tStart))

	// A.4. Setup NAT rules
	tStart = time.Now()
	if err := setupNatRules(netCfg.getVeth0Name(), netCfg.getContainerIP(), netCfg.GetCloneIP()); err != nil {
		return err
	}
	netMetric.SetupNat = metrics.ToUS(time.Since(tStart))

	// B. In host netns
	// B.1 Go back to host namespace
	tStart = time.Now()
	err = netns.Set(hostNsHandle)
	if err != nil {
		return err
	}
	netMetric.SwitchHostNs = metrics.ToUS(time.Since(tStart))

	tStart = time.Now()
	runtime.UnlockOSThread()
	netMetric.UnlockOsThread = metrics.ToUS(time.Since(tStart))

	// B.2 Configure host side veth pair
	tStart = time.Now()
	if err := configVeth(netCfg.getVeth1Name(), netCfg.getVeth1CIDR()); err != nil {
		return err
	}
	netMetric.ConfigVethHost = metrics.ToUS(time.Since(tStart))

	// B.3 Add a route on the host for the clone address
	tStart = time.Now()
	if err := addRoute(netCfg.GetCloneIP(), netCfg.getVeth0CIDR()); err != nil {
		return err
	}
	netMetric.Addroute = metrics.ToUS(time.Since(tStart))

	// B.4 Setup nat to route traffic out of veth device
	tStart = time.Now()
	if err := setupForwardRules(netCfg.getVeth1Name(), mgr.hostIfaceName); err != nil {
		return err
	}
	netMetric.SetForward = metrics.ToUS(time.Since(tStart))

	return nil
}

func (mgr *NetworkManager) GetConfig(vmID string) *NetworkConfig {
	mgr.Lock()
	defer mgr.Unlock()

	cfg := mgr.netConfigs[vmID]
	return cfg
}

func (mgr *NetworkManager) RemoveNetwork(vmID string) error {
	netCfg := mgr.GetConfig(vmID)

	// Delete nat to route traffic out of veth device
	if err := deleteForwardRules(netCfg.getVeth1Name(), mgr.hostIfaceName); err != nil {
		return err
	}

	// Delete route on the host for the clone address
	if err := deleteRoute(netCfg.GetCloneIP(), netCfg.getVeth0CIDR()); err != nil {
		return err
	}

	runtime.LockOSThread()

	hostNsHandle, err := netns.Get()
	defer hostNsHandle.Close()
	if err != nil {
		log.Printf("Failed to get host ns, %s\n", err)
		return err
	}

	// Get uVM namespace handle
	vmNsHandle, err := netns.GetFromName(netCfg.getNamespaceName())
	defer vmNsHandle.Close()
	if err != nil {
		return err
	}
	err = netns.Set(vmNsHandle)
	if err != nil {
		return err
	}

	// Delete NAT rules
	if err := deleteNatRules(netCfg.getVeth0Name(), netCfg.getContainerIP(), netCfg.GetCloneIP()); err != nil {
		return err
	}

	// Delete default gateway for packets leaving namespace
	if err := deleteDefaultGateway(netCfg.getVeth1CIDR()); err != nil {
		return err
	}

	// Delete uVM side veth pair
	if err := deleteVethPair(netCfg.getVeth0Name(), netCfg.getVeth1Name(), vmNsHandle, hostNsHandle); err != nil {
		return err
	}

	// Delete tap device for uVM
	if err := deleteTap(netCfg.containerTap); err != nil {
		return err
	}


	if err := netns.DeleteNamed(netCfg.getNamespaceName()); err != nil {
		return errors.Wrapf(err, "deleting network namespace")
	}

	err = netns.Set(hostNsHandle)
	if err != nil {
		return err
	}
	runtime.UnlockOSThread()

	mgr.removeNetConfig(vmID)

	return nil
}

func (mgr *NetworkManager) Cleanup() error {
	log.Info("Cleaning up network manager")
	mgr.Lock()
	defer mgr.Unlock()

	for vmID := range mgr.netConfigs {
		err := mgr.RemoveNetwork(vmID)
		log.Warnf("Failed to remove network for vm %s: %v\n", vmID, err)
	}

	return nil
}