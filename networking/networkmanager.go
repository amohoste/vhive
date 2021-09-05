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

	freeConfigs     []*NetworkConfig

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
	manager.freeConfigs = make([]*NetworkConfig, 0)

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

	var config *NetworkConfig
	var id int
	if len(mgr.freeConfigs) == 0 {
		id = mgr.nextID
		mgr.nextID += 1
		config = NewNetworkConfig(id, mgr.hostIfaceName)
	} else {
		config = mgr.freeConfigs[len(mgr.freeConfigs)-1]
		mgr.freeConfigs = mgr.freeConfigs[:len(mgr.freeConfigs)-1]
	}
	mgr.netConfigs[vmID] = config
}

func (mgr *NetworkManager) removeNetConfig(vmID string) {
	mgr.Lock()
	defer mgr.Unlock()

	config := mgr.netConfigs[vmID]
	mgr.freeConfigs = append(mgr.freeConfigs, config)
	delete(mgr.netConfigs, vmID)
}

func (mgr *NetworkManager) CreateNetwork(vmID string, netMetric *metrics.NetMetric) error {
	var (
		tStart               time.Time
	)

	// Create network config for VM KEEP THIS
	tStart = time.Now()
	mgr.createNetConfig(vmID)
	netCfg := mgr.GetConfig(vmID)
	netMetric.CreateConfig = metrics.ToUS(time.Since(tStart))

	if err := netCfg.CreateNetwork(netMetric); err != nil {
		return err
	}

	return nil
}

func (mgr *NetworkManager) GetConfig(vmID string) *NetworkConfig {
	mgr.Lock()
	defer mgr.Unlock()

	cfg := mgr.netConfigs[vmID]
	return cfg
}

func (mgr *NetworkManager) RemoveNetwork(vmID string) error {
	/*netCfg := mgr.GetConfig(vmID)

	if err := netCfg.RemoveNetwork(vmID); err != nil {
		return err
	}*/

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