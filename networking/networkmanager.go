package networking

import (
	"bufio"
	"bytes"
	"github.com/ease-lab/vhive/metrics"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type NetworkManager struct {
	sync.Mutex
	nextID          int
	hostIfaceName   string

	poolCond         *sync.Cond
	networkPool     []*NetworkConfig

	netConfigs      map[string]*NetworkConfig // Maps vmIDs to their network config
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

func NewNetworkManager(hostIfaceName string, poolSize int) (*NetworkManager, error) {
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
	manager.networkPool = make([]*NetworkConfig, 0)

	startId, err := getNetworkStartID()
	if err == nil {
		manager.nextID = startId
	} else {
		manager.nextID = 0
	}

	manager.initConfigPool(poolSize)
	manager.getNetCond = sync.NewCond(new(sync.Mutex))

	return manager, nil
}

func (mgr *NetworkManager) initConfigPool(poolSize int) {
	var wg sync.WaitGroup
	wg.Add(poolSize)

	for i := 0; i < poolSize; i++ {
		go func() {
			mgr.allocNetConfig()
			wg.Done()
		}()
	}
	wg.Wait()
}

// Set signal true if want to notify
func (mgr *NetworkManager) allocNetConfig() {
	mgr.Lock()
	id := mgr.nextID
	mgr.nextID += 1
	mgr.Unlock()

	netCfg := NewNetworkConfig(id, mgr.hostIfaceName)
	netMetric := metrics.NewNetMetric("")
	if err := netCfg.CreateNetwork(netMetric); err != nil {
		log.Errorf("failed to create network %s:", err)
	}

	mgr.poolCond.L.Lock()
	mgr.networkPool = append(mgr.networkPool, netCfg)
	mgr.poolCond.Signal()
	mgr.poolCond.L.Unlock()
}

func (mgr *NetworkManager) createNetConfig(vmID string) *NetworkConfig {
	go mgr.allocNetConfig() // Add netconfig to pool to keep pool to configured size

	mgr.poolCond.L.Lock()
	if len(mgr.networkPool) == 0 {
		mgr.poolCond.Wait()
	}
	config := mgr.networkPool[len(mgr.networkPool)-1]
	mgr.networkPool = mgr.networkPool[:len(mgr.networkPool)-1]
	mgr.poolCond.L.Unlock()

	mgr.Lock()
	mgr.netConfigs[vmID] = config
	mgr.Unlock()
	return config
}

func (mgr *NetworkManager) removeNetConfig(vmID string) {
	mgr.Lock()
	config := mgr.netConfigs[vmID]
	delete(mgr.netConfigs, vmID)
	mgr.Unlock()

	mgr.poolCond.L.Lock()
	mgr.networkPool = append(mgr.networkPool, config)
	mgr.poolCond.Signal()
	mgr.poolCond.L.Unlock()
}

func (mgr *NetworkManager) CreateNetwork(vmID string, netMetric *metrics.NetMetric) (*NetworkConfig, error) {
	var (
		tStart               time.Time
	)

	// Create network config for VM
	tStart = time.Now()
	netCfg := mgr.createNetConfig(vmID)
	netMetric.CreateConfig = metrics.ToUS(time.Since(tStart))

	return netCfg, nil
}

func (mgr *NetworkManager) GetConfig(vmID string) *NetworkConfig {
	mgr.Lock()
	defer mgr.Unlock()

	cfg := mgr.netConfigs[vmID]
	return cfg
}

func (mgr *NetworkManager) RemoveNetwork(vmID string) error {
	// netCfg.RemoveNetwork(vmID). Not done because keep pool
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