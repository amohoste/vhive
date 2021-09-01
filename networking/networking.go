package networking

import (
	"fmt"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
)

func createTap(tapName, gatewayIP, netnsName string) error {
	// 1. Create tap device
	la := netlink.NewLinkAttrs()
	la.Name = tapName
	la.Namespace = netnsName
	tap0 := &netlink.Tuntap{LinkAttrs: la, Mode: netlink.TUNTAP_MODE_TAP}
	if err := netlink.LinkAdd(tap0); err != nil {
		return errors.Wrapf(err, "creating tap")
	}

	// 2. Give tap device ip address
	addr, _ := netlink.ParseAddr(gatewayIP)
	addr.Broadcast = net.IPv4(0, 0, 0, 0)
	if err := netlink.AddrAdd(tap0, addr); err != nil {
		return errors.Wrapf(err, "adding tap ip address")
	}

	// 3. Enable tap network interface
	if err := netlink.LinkSetUp(tap0); err != nil {
		return errors.Wrapf(err, "enabling tap")
	}

	return nil
}

func deleteTap(tapName string) error {
	if err := netlink.LinkDel(&netlink.Tuntap{LinkAttrs: netlink.LinkAttrs{Name: tapName}}); err != nil {
		return errors.Wrapf(err, "deleting tap %s", tapName)
	}

	return nil
}

func createVethPair(veth0Name, veth1Name string, veth0NsHandle, veth1NsHandle netns.NsHandle) error {
	veth := &netlink.Veth{netlink.LinkAttrs{Name: veth0Name, Namespace: netlink.NsFd(veth0NsHandle), TxQLen: 1000}, veth1Name, nil, netlink.NsFd(veth1NsHandle)}
	if err := netlink.LinkAdd(veth); err != nil {
		return errors.Wrapf(err, "creating veth pair")
	}

	return nil
}

func deleteVethPair(veth0Name, veth1Name string, veth0NsHandle, veth1NsHandle netns.NsHandle) error {
	if err := netlink.LinkDel(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: veth0Name, Namespace: netlink.NsFd(veth0NsHandle)}, PeerName: veth1Name, PeerNamespace: netlink.NsFd(veth1NsHandle)}); err != nil {
		return errors.Wrapf(err, "deleting veth %s", veth0Name)
	}
	return nil
}

func configVeth(linkName, vethIp string) error {
	// 1. Get link
	veth, err := netlink.LinkByName(linkName)
	if err != nil {
		return errors.Wrapf(err, "Finding veth link")
	}

	// 2. Set IP address
	addr, _ := netlink.ParseAddr(vethIp)
	addr.Broadcast = net.IPv4(0, 0, 0, 0)
	if err := netlink.AddrAdd(veth, addr); err != nil {
		return errors.Wrapf(err, "adding veth link ip address")
	}

	// 3. Enable link
	if err := netlink.LinkSetUp(veth); err != nil {
		return errors.Wrapf(err, "enabling veth link")
	}

	return nil
}

func setDefaultGateway(gatewayIp string) error {
	gw, _, err := net.ParseCIDR(gatewayIp)
	if err != nil {
		return errors.Wrapf(err, "parsing ip")
	}

	defaultRoute := &netlink.Route{
		Dst: nil,
		Gw:  gw,
	}

	if err := netlink.RouteAdd(defaultRoute); err != nil {
		return errors.Wrapf(err, "adding default route")
	}

	return nil
}

func deleteDefaultGateway(gatewayIp string) error {
	gw, _, err := net.ParseCIDR(gatewayIp)
	if err != nil {
		return errors.Wrapf(err, "parsing ip")
	}

	defaultRoute := &netlink.Route{
		Dst: nil,
		Gw:  gw,
	}

	if err := netlink.RouteDel(defaultRoute); err != nil {
		return errors.Wrapf(err, "deleting default route")
	}

	return nil
}

func setupNatRules(vethVmName, hostIp, cloneIp string, vmNsHandle netns.NsHandle) error {
	conn := nftables.Conn{NetNS: int(vmNsHandle)}

	// 1. add table ip nat
	natTable := &nftables.Table{
		Name:   "nat",
		Family: nftables.TableFamilyIPv4,
	}

	// 2. Iptables: -t nat -A POSTROUTING -o veth1-0 -s 172.16.0.2 -j SNAT --to 192.168.0.1
	// 2.1 add chain ip nat POSTROUTING { type nat hook postrouting priority 0; policy accept; }
	polAccept := nftables.ChainPolicyAccept
	postRouteCh := &nftables.Chain{
		Name:     "POSTROUTING",
		Table:    natTable,
		Type:     nftables.ChainTypeNAT,
		Priority: 0,
		Hooknum:  nftables.ChainHookPostrouting,
		Policy:   &polAccept,
	}

	// 2.2 add rule ip nat POSTROUTING oifname veth1-0 ip saddr 172.16.0.2 counter snat to 192.168.0.1
	snatRule := &nftables.Rule{
		Table: natTable,
		Chain: postRouteCh,
		Exprs: []expr.Any{
			// Load iffname in register 1
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", vethVmName)),
			},
			// Load source IP address (offset 12 bytes network header) in register 1
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       12,
				Len:          4,
			},
			// Check source ip address == 172.16.0.2
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     net.ParseIP(hostIp).To4(),
			},
			// Load snatted address (192.168.0.1) in register 1
			&expr.Immediate{
				Register: 1,
				Data:     net.ParseIP(cloneIp).To4(),
			},
			&expr.NAT{
				Type:        expr.NATTypeSourceNAT, // Snat
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  1,
			},
		},
	}

	// 3. Iptables: -t nat -A PREROUTING -i veth1-0 -d 192.168.0.1 -j DNAT --to 172.16.0.2
	// 3.1 add chain ip nat PREROUTING { type nat hook prerouting priority 0; policy accept; }
	preRouteCh := &nftables.Chain{
		Name:     "PREROUTING",
		Table:    natTable,
		Type:     nftables.ChainTypeNAT,
		Priority: 0,
		Hooknum:  nftables.ChainHookPrerouting,
		Policy:   &polAccept,
	}

	// 3.2 add rule ip nat PREROUTING iifname veth1-0 ip daddr 192.168.0.1 counter dnat to 172.16.0.2
	dnatRule := &nftables.Rule{
		Table: natTable,
		Chain: preRouteCh,
		Exprs: []expr.Any{
			// Load iffname in register 1
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", vethVmName)),
			},
			// Load destination IP address (offset 16 bytes network header) in register 1
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       16,
				Len:          4,
			},
			// Check destination ip address == 192.168.0.1
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     net.ParseIP(cloneIp).To4(),
			},
			// Load dnatted address (172.16.0.2) in register 1
			&expr.Immediate{
				Register: 1,
				Data:     net.ParseIP(hostIp).To4(),
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT, // Dnat
				Family:      unix.NFPROTO_IPV4,
				RegAddrMin:  1,
			},
		},
	}

	// Apply
	conn.AddTable(natTable)
	conn.AddChain(postRouteCh)
	conn.AddRule(snatRule)
	conn.AddChain(preRouteCh)
	conn.AddRule(dnatRule)
	if err := conn.Flush(); err != nil {
		return errors.Wrapf(err, "creating nat rules")
	}
	return nil
}

func deleteNatRules(vmNsHandle netns.NsHandle) error {
	conn := nftables.Conn{NetNS: int(vmNsHandle)}

	natTable := &nftables.Table{
		Name:   "nat",
		Family: nftables.TableFamilyIPv4,
	}

	// Apply
	conn.DelTable(natTable)
	if err := conn.Flush(); err != nil {
		return errors.Wrapf(err, "deleting nat rules")
	}
	return nil
}

func setupForwardRules(vethHostName, hostIface string, outForwardHandle, inForwardHandle uint64) error {
	conn := nftables.Conn{}

	// 1. add table ip filter
	filterTable := &nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyIPv4,
	}

	// 2. add chain ip filter FORWARD { type filter hook forward priority 0; policy accept; }
	polAccept := nftables.ChainPolicyAccept
	fwdCh := &nftables.Chain{
		Name:     "FORWARD",
		Table:    filterTable,
		Type:     nftables.ChainTypeFilter,
		Priority: 0,
		Hooknum:  nftables.ChainHookForward,
		Policy:   &polAccept,
	}

	// 3. Iptables: -A FORWARD -i veth1-1 -o eno49 -j ACCEPT
	// 3.1 add rule ip filter FORWARD iifname veth1-1 oifname eno49 counter accept
	outRule := &nftables.Rule{
		Table: filterTable,
		Chain: fwdCh,
		Exprs: []expr.Any{
			// Load iffname in register 1
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", vethHostName)),
			},
			// Load oif in register 1
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", hostIface)),
			},
			&expr.Verdict{
				Kind: expr.VerdictDrop,
			},
		},
		Handle: outForwardHandle,
	}

	// 4. Iptables: -A FORWARD -o veth1-1 -i eno49 -j ACCEPT
	// 4.1 add rule ip filter FORWARD iifname eno49 oifname veth1-1 counter accept
	inRule := &nftables.Rule{
		Table: filterTable,
		Chain: fwdCh,
		Exprs: []expr.Any{
			// Load oifname in register 1
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", vethHostName)),
			},
			// Load oif in register 1
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			// Check iifname == veth1-0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(fmt.Sprintf("%s\x00", hostIface)),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
		Handle: inForwardHandle,
	}
	conn.AddTable(filterTable)
	conn.AddChain(fwdCh)
	conn.AddRule(outRule)
	conn.AddRule(inRule)
	if err := conn.Flush(); err != nil {
		return errors.Wrapf(err, "creating forward rules")
	}
	return nil
}

func deleteForwardRules(outForwardHandle, inForwardHandle uint64) error {
	conn := nftables.Conn{}

	// 1. add table ip filter
	filterTable := &nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyIPv4,
	}

	// 2. add chain ip filter FORWARD { type filter hook forward priority 0; policy accept; }
	polAccept := nftables.ChainPolicyAccept
	fwdCh := &nftables.Chain{
		Name:     "FORWARD",
		Table:    filterTable,
		Type:     nftables.ChainTypeFilter,
		Priority: 0,
		Hooknum:  nftables.ChainHookForward,
		Policy:   &polAccept,
	}

	if err := conn.DelRule(&nftables.Rule{
		Table:  filterTable,
		Chain:  fwdCh,
		Handle: outForwardHandle,
	}); err != nil {
		return errors.Wrapf(err, "deleting out forward rule")
	}

	if err := conn.DelRule(&nftables.Rule{
		Table:  filterTable,
		Chain:  fwdCh,
		Handle: inForwardHandle,
	}); err != nil {
		return errors.Wrapf(err, "deleting in forward rule")
	}

	if err := conn.Flush(); err != nil {
		return errors.Wrapf(err, "deleting forward rules")
	}
	return nil
}

func addRoute(destIp, gatewayIp string) error {
	_, dstNet, err := net.ParseCIDR(fmt.Sprintf("%s/32", destIp))
	if err != nil {
		return errors.Wrapf(err, "parsing route destination ip")
	}

	gwAddr, _, err := net.ParseCIDR(gatewayIp)
	if err != nil {
		return errors.Wrapf(err, "parsing route gateway ip")
	}

	route := &netlink.Route{
		Dst: dstNet,
		Gw:  gwAddr,
	}

	if err := netlink.RouteAdd(route); err != nil {
		return errors.Wrapf(err, "adding route")
	}
	return nil
}

func deleteRoute(destIp, gatewayIp string) error {
	_, dstNet, err := net.ParseCIDR(fmt.Sprintf("%s/32", destIp))
	if err != nil {
		return errors.Wrapf(err, "parsing route destination ip")
	}

	gwAddr, _, err := net.ParseCIDR(gatewayIp)
	if err != nil {
		return errors.Wrapf(err, "parsing route gateway ip")
	}

	route := &netlink.Route{
		Dst: dstNet,
		Gw:  gwAddr,
	}

	if err := netlink.RouteDel(route); err != nil {
		return errors.Wrapf(err, "deleting route")
	}
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getNetworkStartID() (int, error) {
	files, err := ioutil.ReadDir("/run/netns")
	if err != nil {
		return 0, errors.Wrapf(err,"Couldn't read network namespace dir")
	}

	maxId := 0
	for _, f := range files {
		if ! f.IsDir() {
			netnsName := f.Name()

			re := regexp.MustCompile(`^uvmns([0-9]+)$`)
			regres := re.FindStringSubmatch(netnsName)

			if len(regres) > 1 {
				id, err := strconv.Atoi(regres[1])
				if err == nil {
					maxId = max(id, maxId)
				}
			}
		}
	}

	return maxId + 1, nil
}