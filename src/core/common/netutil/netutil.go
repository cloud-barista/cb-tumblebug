package netutil

import (
	"fmt"
	"log"
	"math"
	"net"
)

var (
	privateNetworks             []*net.IPNet
	ip10, ip172, ip192          net.IP
	ipnet10, ipnet172, ipnet192 *net.IPNet
)

func init() {
	// Initialize private networks
	for _, IPNetwork := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, privateNetwork, err := net.ParseCIDR(IPNetwork)
		if err != nil {
			log.Fatalf("parse error on %q: %v", IPNetwork, err)
		}
		privateNetworks = append(privateNetworks, privateNetwork)
	}

	// Initialize IPs and networks of each private network
	ip10, ipnet10, _ = net.ParseCIDR("10.0.0.0/8")
	ip172, ipnet172, _ = net.ParseCIDR("172.16.0.0/12")
	ip192, ipnet192, _ = net.ParseCIDR("192.168.0.0/16")
}

// Models
type Network struct {
	CIDRBlock        string    // 192.168.0.0/24
	NetworkAddress   string    // 192.168.0.0
	BroadcastAddress string    // 192.168.0.255
	Prefix           int       // 24
	Netmask          string    // 255.255.255.0
	HostCapacity     int       // 254
	Subnets          []Network // Subnets within this network
}

// New creates a new Network object.
func New(cidrBlock string) (*Network, error) {
	network := &Network{
		CIDRBlock: cidrBlock,
	}

	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}

	// Set Netmask
	mask := ipNet.Mask
	network.Netmask = net.IP(mask).String()

	// Set Prefix
	_, prefix := ipNet.Mask.Size()
	network.Prefix = prefix

	// Set NetworkAddress
	netAddr, err := CalculateNetworkAddr(ipNet)
	if err != nil {
		return nil, err
	}
	network.NetworkAddress = netAddr

	// Set BroadcastAddress
	broadcastAddr, err := CalculateBroadcastAddr(ipNet)
	if err != nil {
		return nil, err
	}
	network.BroadcastAddress = broadcastAddr

	// Set Hosts
	hosts, err := CalculateHostCapacity(ipNet)
	if err != nil {
		return nil, err
	}
	network.HostCapacity = hosts

	network.Subnets = []Network{}

	return network, nil
}

// Getters
func (n *Network) GetCIDRBlock() string        { return n.CIDRBlock }
func (n *Network) GetNetworkAddress() string   { return n.NetworkAddress }
func (n *Network) GetBroadcastAddress() string { return n.BroadcastAddress }
func (n *Network) GetPrefix() int              { return n.Prefix }
func (n *Network) GetNetmask() string          { return n.Netmask }
func (n *Network) GetHostCapacity() int        { return n.HostCapacity }
func (n *Network) GetSubnets() []Network       { return n.Subnets }

// SubnettingByMininumSubnetCount divides the CIDR block into subnets to accommodate the minimum number of subnets entered.
func SubnettingByMininumSubnetCount(cidrBlock string, minSubnets int) ([]string, error) {
	_, network, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}

	// Calculate the new subnet mask size
	maskSize, _ := network.Mask.Size()
	subnetBits := int(math.Ceil(math.Log2(float64(minSubnets))))
	newMaskSize := maskSize + subnetBits

	if newMaskSize > 32 {
		return nil, fmt.Errorf("cannot split %s to accommodate at least %d subnets", cidrBlock, minSubnets)
	}

	// Calculate the actual number of subnets that can be created with the new mask size
	numSubnets := int(math.Pow(2, float64(subnetBits)))

	var subnets []string
	for i := 0; i < numSubnets; i++ {
		ip := make(net.IP, len(network.IP))
		copy(ip, network.IP)

		// Calculate the offset to apply to the base IP address
		offset := int64(i) << (32 - newMaskSize)
		for j := 3; j >= 0; j-- {
			shift := uint((3 - j) * 8)
			ip[j] += byte((offset >> shift) & 0xff)
		}

		subnets = append(subnets, fmt.Sprintf("%s/%d", ip.String(), newMaskSize))
	}

	return subnets, nil
}

// IpToUint32 converts an IP address to a uint32.
func IpToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

// Uint32ToIP converts a uint32 to an IP address.
func Uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// SubnettingByHosts divides a CIDR block into subnets based on the number of hosts required for one subnet.
func SubnettingByHosts(cidrBlock string, hostsPerSubnet int) ([]string, error) {
	if hostsPerSubnet < 2 {
		return nil, fmt.Errorf("number of hosts per subnet should be at least 2")
	}

	_, network, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}

	maskSize, bits := network.Mask.Size()
	// Adjusting for network and broadcast addresses
	hostBits := int(math.Ceil(math.Log2(float64(hostsPerSubnet + 2))))
	newMaskSize := bits - hostBits

	if newMaskSize <= maskSize {
		return nil, fmt.Errorf("not enough room to create subnets for %d hosts in %s", hostsPerSubnet, cidrBlock)
	}

	baseIP := IpToUint32(network.IP)
	subnetMask := uint32(math.Pow(2, float64(hostBits)) - 1)
	var subnets []string

	for currentIP := baseIP; currentIP < baseIP+uint32(math.Pow(2, float64(bits-maskSize))); currentIP += subnetMask + 1 {
		subnetIP := Uint32ToIP(currentIP)
		subnets = append(subnets, fmt.Sprintf("%s/%d", subnetIP.String(), newMaskSize))
	}

	return subnets, nil
}

// CalculateNetworkAddr calculates the network address for a given IPNet.
func CalculateNetworkAddr(ipNet *net.IPNet) (string, error) {
	ip := ipNet.IP
	networkIP := ip.Mask(ipNet.Mask)
	return networkIP.String(), nil
}

// GetNetworkAddr calculates the network address for a given CIDR block.
func GetNetworkAddr(cidrBlock string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", err
	}
	return CalculateNetworkAddr(ipNet)
}

// CalculateBroadcastAddr calculates the broadcast address for a given IPNet.
func CalculateBroadcastAddr(ipNet *net.IPNet) (string, error) {
	// Calculate network and broadcast addresses
	ip := ipNet.IP
	mask := ipNet.Mask
	broadcast := make(net.IP, len(ip))
	for i := 0; i < len(ip); i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}
	broadcastAddress := broadcast.String()

	return broadcastAddress, nil
}

// GetBroadcastAddr calculates the broadcast address for a given CIDR block.
func GetBroadcastAddr(cidrBlock string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", err
	}

	return CalculateBroadcastAddr(ipNet)
}

// CalculateHostCapacity calculates the number of hosts that can be accommodated in a given IPNet.
func CalculateHostCapacity(ipNet *net.IPNet) (int, error) {

	maskSize, bits := ipNet.Mask.Size()
	switch maskSize {
	case 31:
		// Special case for /31 subnets, typically used in point-to-point links (RFC 3021)
		return 2, nil
	case 32:
		// /32 subnets represent a single host (commonly used for loopback addresses)
		return 1, nil
	default:
		hostBits := bits - maskSize
		hosts := int(math.Pow(2, float64(hostBits))) - 2
		return hosts, nil
	}
}

// GetSizeOfHosts calculates the number of hosts that can be accommodated in a given CIDR block.
func GetSizeOfHosts(cidrBlock string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return 0, err
	}

	return CalculateHostCapacity(ipNet)
}
