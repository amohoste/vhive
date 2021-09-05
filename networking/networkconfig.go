package networking

import (
	"fmt"
	"github.com/ease-lab/vhive/metrics"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
	"net"
	"runtime"
	"time"
)

// Can at most have 2^14 VMs at the same time
type NetworkConfig struct {
	id             int
	initialized    bool
	containerCIDR  string // Container IP address (CIDR notation)
	gatewayCIDR    string // Container gateway IP address
	containerTap   string // Container tap name
	hostIfaceName  string
}

func NewNetworkConfig(id int, hostIfaceName string) *NetworkConfig {
	return &NetworkConfig{id: id, containerCIDR: "172.16.0.2/24", gatewayCIDR: "172.16.0.1/24", containerTap: "tap0", initialized: false, hostIfaceName: hostIfaceName}
}

func (cfg *NetworkConfig) GetMacAddress() string {
	return "AA:FC:00:00:00:01"
}

func (cfg *NetworkConfig) GetHostDevName() string {
	return "tap0"
}

func (cfg *NetworkConfig) getVeth0Name() string {
	return fmt.Sprintf("veth%d-0", cfg.id)
}

func (cfg *NetworkConfig) getVeth0CIDR() string {
	return fmt.Sprintf("172.17.%d.%d/30", (4 * cfg.id) / 256, ((4 * cfg.id) + 2) % 256)
}

func (cfg *NetworkConfig) getVeth1Name() string {
	return fmt.Sprintf("veth%d-1", cfg.id)
}

func (cfg *NetworkConfig) getVeth1CIDR() string {
	return fmt.Sprintf("172.17.%d.%d/30", (4 * cfg.id) / 256, ((4 * cfg.id) + 1) % 256)
}

// External IP
func (cfg *NetworkConfig) GetCloneIP() string {
	return fmt.Sprintf("172.18.%d.%d", cfg.id / 254, 1 + (cfg.id % 254))
}

func (cfg *NetworkConfig) getNamespaceName() string {
	return fmt.Sprintf("uvmns%d", cfg.id)
}
func (cfg *NetworkConfig) getContainerIP() string {
	ip, _, _ := net.ParseCIDR(cfg.containerCIDR)
	return ip.String()
}

func (cfg *NetworkConfig) GetGatewayIP() string {
	ip, _, _ := net.ParseCIDR(cfg.gatewayCIDR)
	return ip.String()
}

func (cfg *NetworkConfig) GetNamespacePath() string {
	return fmt.Sprintf("/var/run/netns/%s", cfg.getNamespaceName())
}

func (cfg *NetworkConfig) GetContainerCIDR() string {
	return cfg.containerCIDR
}

func (cfg *NetworkConfig) CreateNetwork(netMetric *metrics.NetMetric) error {
	var (
		tStart               time.Time
	)

	if !cfg.initialized {
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
		vmNsHandle, err := netns.NewNamed(cfg.getNamespaceName()) // Switches namespace
		if err != nil {
			log.Println(err)
			return err
		}
		defer vmNsHandle.Close()
		netMetric.CreateVmNs = metrics.ToUS(time.Since(tStart))

		// A.2. Create tap device for uVM
		tStart = time.Now()
		if err := createTap(cfg.containerTap, cfg.gatewayCIDR, cfg.getNamespaceName()); err != nil {
			return err
		}
		netMetric.CreateVmTap = metrics.ToUS(time.Since(tStart))

		// A.3. Create veth pair for uVM
		// A.3.1 Create veth pair
		tStart = time.Now()
		if err := createVethPair(cfg.getVeth0Name(), cfg.getVeth1Name(), vmNsHandle, hostNsHandle); err != nil {
			return err
		}
		netMetric.CreateVeth = metrics.ToUS(time.Since(tStart))

		// A.3.2 Configure uVM side veth pair
		tStart = time.Now()
		if err := configVeth(cfg.getVeth0Name(), cfg.getVeth0CIDR()); err != nil {
			return err
		}
		netMetric.ConfigVethVm = metrics.ToUS(time.Since(tStart))

		// A.3.3 Designate host side as default gateway for packets leaving namespace
		tStart = time.Now()
		if err := setDefaultGateway(cfg.getVeth1CIDR()); err != nil {
			return err
		}
		netMetric.SetDefaultGw = metrics.ToUS(time.Since(tStart))

		// A.4. Setup NAT rules
		tStart = time.Now()
		if err := setupNatRules(cfg.getVeth0Name(), cfg.getContainerIP(), cfg.GetCloneIP(), vmNsHandle); err != nil {
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
		if err := configVeth(cfg.getVeth1Name(), cfg.getVeth1CIDR()); err != nil {
			return err
		}
		netMetric.ConfigVethHost = metrics.ToUS(time.Since(tStart))

		// B.3 Add a route on the host for the clone address
		tStart = time.Now()
		if err := addRoute(cfg.GetCloneIP(), cfg.getVeth0CIDR()); err != nil {
			return err
		}
		netMetric.Addroute = metrics.ToUS(time.Since(tStart))

		// B.4 Setup nat to route traffic out of veth device
		tStart = time.Now()
		if err := setupForwardRules(cfg.getVeth1Name(), cfg.hostIfaceName); err != nil {
			return err
		}
		netMetric.SetForward = metrics.ToUS(time.Since(tStart))
	}

	return nil
}

func (cfg *NetworkConfig) RemoveNetwork(vmID string) error {
	// Delete nat to route traffic out of veth device
	if err := deleteForwardRules(cfg.getVeth1Name()); err != nil {
		return err
	}

	// Delete route on the host for the clone address
	if err := deleteRoute(cfg.GetCloneIP(), cfg.getVeth0CIDR()); err != nil {
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
	vmNsHandle, err := netns.GetFromName(cfg.getNamespaceName())
	defer vmNsHandle.Close()
	if err != nil {
		return err
	}
	err = netns.Set(vmNsHandle)
	if err != nil {
		return err
	}

	// Delete NAT rules
	if err := deleteNatRules(vmNsHandle); err != nil {
		return err
	}

	// Delete default gateway for packets leaving namespace
	if err := deleteDefaultGateway(cfg.getVeth1CIDR()); err != nil {
		return err
	}

	// Delete uVM side veth pair
	if err := deleteVethPair(cfg.getVeth0Name(), cfg.getVeth1Name(), vmNsHandle, hostNsHandle); err != nil {
		return err
	}

	// Delete tap device for uVM
	if err := deleteTap(cfg.containerTap); err != nil {
		return err
	}


	if err := netns.DeleteNamed(cfg.getNamespaceName()); err != nil {
		return errors.Wrapf(err, "deleting network namespace")
	}

	err = netns.Set(hostNsHandle)
	if err != nil {
		return err
	}
	runtime.UnlockOSThread()

	return nil
}