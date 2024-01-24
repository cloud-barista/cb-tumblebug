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
type NetworkConfig struct {
	NetworkConfiguration Network `json:"networkConfiguration"`
}

// NetworkInterface defines the methods that both Network and NetworkDetails should implement.
type NetworkInterface interface {
	GetCIDRBlock() string
	GetName() string
	GetSubnets() []Network
}

type Network struct {
	CIDRBlock string    `json:"cidrBlock"`
	Name      string    `json:"name,omitempty"`
	Subnets   []Network `json:"subnets,omitempty"`
}

func (n *Network) GetCIDRBlock() string  { return n.CIDRBlock }
func (n *Network) GetName() string       { return n.Name }
func (n *Network) GetSubnets() []Network { return n.Subnets }

// New creates a new NetworkDetails object.
func NewNetwork(cidrBlock string) (*Network, error) {

	_, _, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}
	network := new(Network)
	network.CIDRBlock = cidrBlock

	network.Subnets = []Network{}

	return network, nil
}

type NetworkDetails struct {
	Network
	NetworkAddress   string `json:"networkAddress,omitempty"`
	BroadcastAddress string `json:"broadcastAddress,omitempty"`
	Prefix           int    `json:"prefix,omitempty"`
	Netmask          string `json:"netmask,omitempty"`
	HostCapacity     int    `json:"hostCapacity,omitempty"`
}

// Getters
func (n *NetworkDetails) GetNetworkAddress() string   { return n.NetworkAddress }
func (n *NetworkDetails) GetBroadcastAddress() string { return n.BroadcastAddress }
func (n *NetworkDetails) GetPrefix() int              { return n.Prefix }
func (n *NetworkDetails) GetNetmask() string          { return n.Netmask }
func (n *NetworkDetails) GetHostCapacity() int        { return n.HostCapacity }

// New creates a new NetworkDetails object.
func NewNetworkDetails(cidrBlock string) (*NetworkDetails, error) {

	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}
	network := new(NetworkDetails)
	network.CIDRBlock = cidrBlock

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

// GetNetworkAddr calculates the network address for a given CIDR block.
func GetNetworkAddr(cidrBlock string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", err
	}
	return CalculateNetworkAddr(ipNet)
}

// CalculateNetworkAddr calculates the network address for a given IPNet.
func CalculateNetworkAddr(ipNet *net.IPNet) (string, error) {
	ip := ipNet.IP
	networkIP := ip.Mask(ipNet.Mask)
	return networkIP.String(), nil
}

// GetBroadcastAddr calculates the broadcast address for a given CIDR block.
func GetBroadcastAddr(cidrBlock string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", err
	}

	return CalculateBroadcastAddr(ipNet)
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

// GetPrefix calculates the prefix for a given CIDR block.
func GetPrefix(cidrBlock string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return -1, err
	}

	prefix, _ := ipNet.Mask.Size()
	return prefix, nil
}

// GetNetmask calculates the netmask for a given CIDR block.
func GetNetmask(cidrBlock string) (string, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return "", err
	}
	mask := ipNet.Mask
	return net.IP(mask).String(), nil
}

// GetSizeOfHosts calculates the number of hosts that can be accommodated in a given CIDR block.
func GetSizeOfHosts(cidrBlock string) (int, error) {
	_, ipNet, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return -1, err
	}

	return CalculateHostCapacity(ipNet)
}

// CalculateHostCapacity calculates the number of hosts that can be accommodated in a given IPNet.
func CalculateHostCapacity(ipNet *net.IPNet) (int, error) {

	maskSize, bits := ipNet.Mask.Size()
	return calculateHostCapacity(maskSize, bits)
}

func calculateHostCapacity(maskSize, bits int) (int, error) {
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

// ///////////////////////////////////////////////////////////////////
// SubnettingRuleType defines the type for subnetting rules.
type SubnettingRuleType string

// SubnettingRuleType constants.
const (
	SubnettingRuleTypeMinSubnets SubnettingRuleType = "minSubnets"
	SubnettingRuleTypeMinHosts   SubnettingRuleType = "minHosts"
)

// Models for subnetting
type SubnettingRequest struct {
	CIDRBlock       string           `json:"cidrBlock" example:"192.168.0.0/16"`
	SubnettingRules []SubnettingRule `json:"subnettingRules"`
}

type SubnettingRule struct {
	Type  SubnettingRuleType `json:"type" example:"minSubnets" enum:"minSubnets,minHosts"`
	Value int                `json:"value" example:"2"`
}

// Functions for subnetting
// SubnettingBy divides a CIDR block into subnets based on the given rules.
func SubnettingBy(request SubnettingRequest) (Network, error) {
	network, err := NewNetwork(request.CIDRBlock)
	if err != nil {
		return Network{}, fmt.Errorf("error creating base network: %w", err)
	}

	return subnetting(*network, request.SubnettingRules)
}

// subnetting recursivly divides a CIDR block into subnets according to the subnetting rules.
func subnetting(network Network, rules []SubnettingRule) (Network, error) {
	// return the network if there are no more rule
	if len(rules) == 0 {
		return network, nil
	}

	rule := rules[0]
	remainingRules := rules[1:]

	var subnetsStr []string
	var err error
	var subnets []Network

	// Subnetting by the given rule
	switch rule.Type {
	case SubnettingRuleTypeMinSubnets:
		subnetsStr, err = SubnettingByMinimumSubnetCount(network.CIDRBlock, rule.Value)
	case SubnettingRuleTypeMinHosts:
		subnetsStr, err = SubnettingByMinimumHosts(network.CIDRBlock, rule.Value)
	default:
		return network, fmt.Errorf("unknown rule type: %s", rule.Type)
	}

	if err != nil {
		return network, err
	}

	// Recursively subnetting again for each subnet
	for _, cidr := range subnetsStr {
		// a subnet without subnets
		subnetWithoutSubnets, err := NewNetwork(cidr)
		if err != nil {
			return network, err
		}
		// subnetting this subnet recursively
		subnetWithSubnets, err := subnetting(*subnetWithoutSubnets, remainingRules)
		if err != nil {
			return network, err
		}
		subnets = append(subnets, subnetWithSubnets)
	}

	network.Subnets = subnets
	return network, nil
}

// SubnettingByMinimumSubnetCount divides the CIDR block into subnets to accommodate the minimum number of subnets entered.
func SubnettingByMinimumSubnetCount(cidrBlock string, minSubnets int) ([]string, error) {
	_, network, err := net.ParseCIDR(cidrBlock)
	if err != nil {
		return nil, err
	}

	// Calculate the new subnet mask size
	maskSize, _ := network.Mask.Size()
	subnetBits := int(math.Ceil(math.Log2(float64(minSubnets))))
	newMaskSize := maskSize + subnetBits

	if newMaskSize > 32 {
		return nil, fmt.Errorf("cannot split '%s' to accommodate at least %d subnets", cidrBlock, minSubnets)
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

// SubnettingByMinimumHosts divides a CIDR block into subnets based on the number of hosts required for one subnet.
func SubnettingByMinimumHosts(cidrBlock string, hostsPerSubnet int) ([]string, error) {
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
		capa, _ := calculateHostCapacity(newMaskSize, bits)
		return nil, fmt.Errorf("cannot split '%s' (host capacity: %d) into multiple subnets, each containing at least %d hosts", cidrBlock, capa, hostsPerSubnet)
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

// ///////////////////////////////////////////////////////////////////
// ValidateNetwork recursively validates the network and its subnets.
func ValidateNetwork(network Network) error {
	// Check if the CIDR block is valid
	if _, _, err := net.ParseCIDR(network.CIDRBlock); err != nil {
		return fmt.Errorf("invalid CIDR block '%s': %w", network.CIDRBlock, err)
	}

	// Check for overlapping subnets within the same network
	if err := hasOverlappingSubnets(network.Subnets); err != nil {
		return fmt.Errorf("in network '%s': %w", network.CIDRBlock, err)
	}

	// Recursively validate each subnet
	for _, subnet := range network.Subnets {
		if !isSubnetOf(network.CIDRBlock, subnet.CIDRBlock) {
			return fmt.Errorf("subnet '%s' is not a valid subnet of '%s'", subnet.CIDRBlock, network.CIDRBlock)
		}
		if err := ValidateNetwork(subnet); err != nil {
			return err
		}
	}
	return nil
}

// isSubnetOf checks if childCIDR is a subnet of parentCIDR.
func isSubnetOf(parentCIDR, childCIDR string) bool {
	_, parentNet, _ := net.ParseCIDR(parentCIDR)
	_, childNet, _ := net.ParseCIDR(childCIDR)
	return parentNet.Contains(childNet.IP)
}

// hasOverlappingSubnets checks if there are overlapping subnets within the same network.
func hasOverlappingSubnets(subnets []Network) error {
	for i := 0; i < len(subnets); i++ {
		for j := i + 1; j < len(subnets); j++ {
			if cidrOverlap(subnets[i].CIDRBlock, subnets[j].CIDRBlock) {
				return fmt.Errorf("overlapping subnets found: '%s' and '%s'", subnets[i].CIDRBlock, subnets[j].CIDRBlock)
			}
		}
	}
	return nil
}

// cidrOverlap checks if two CIDR blocks overlap.
func cidrOverlap(cidr1, cidr2 string) bool {
	_, net1, _ := net.ParseCIDR(cidr1)
	_, net2, _ := net.ParseCIDR(cidr2)
	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
}

// ///////////////////////////////////////////////////////////////////////////////////
// NextSubnet find and check the next subnet based on the base/parent network.
func NextSubnet(currentSubnetCIDR string, baseNetworkCIDR string) (string, error) {
	// Parse the current subnet
	_, currentNet, err := net.ParseCIDR(currentSubnetCIDR)
	if err != nil {
		return "", err
	}

	// Parse the base network
	_, baseNet, err := net.ParseCIDR(baseNetworkCIDR)
	if err != nil {
		return "", err
	}

	// Convert the current subnet's IP to uint32
	currentIPInt := IpToUint32(currentNet.IP)

	// Calculate the size of the current subnet
	maskSize, _ := currentNet.Mask.Size()
	subnetSize := uint32(1 << (32 - maskSize))

	// Calculate the next subnet's starting IP
	nextIPInt := currentIPInt + subnetSize

	// Convert the next IP to net.IP
	nextIP := Uint32ToIP(nextIPInt)

	// Check if the next subnet is within the base network range
	if !baseNet.Contains(nextIP) {
		return "", fmt.Errorf("the next subnet is outside the base network range")
	}

	return fmt.Sprintf("%s/%d", nextIP.String(), maskSize), nil
}

// PreviousSubnet find and check the previous subnet based on the base/parent network.
func PreviousSubnet(currentSubnet string, baseNetworkCIDR string) (string, error) {
	// Parse the current subnet
	_, currentNet, err := net.ParseCIDR(currentSubnet)
	if err != nil {
		return "", err
	}

	// Parse the base network
	_, baseNet, err := net.ParseCIDR(baseNetworkCIDR)
	if err != nil {
		return "", err
	}

	// Convert the current subnet's IP to uint32
	currentIPInt := IpToUint32(currentNet.IP)

	// Calculate the size of the current subnet
	maskSize, _ := currentNet.Mask.Size()
	subnetSize := uint32(1 << (32 - maskSize))

	// Calculate the previous subnet's starting IP
	previousIPInt := currentIPInt - subnetSize

	// Convert the previous IP to net.IP
	previousIP := Uint32ToIP(previousIPInt)

	// Check if the previous subnet is within the base network range
	if !baseNet.Contains(previousIP) {
		return "", fmt.Errorf("the previous subnet is outside the base network range")
	}

	return fmt.Sprintf("%s/%d", previousIP.String(), maskSize), nil
}
