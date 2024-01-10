package main

import (
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
)

func main() {
	fmt.Println("netuil example")

	cidrBlock := "192.168.0.0/16"

	fmt.Println("\nNetwork template example")
	// Define the base network
	baseNetwork := netutil.Network{
		CIDRBlock: "10.0.0.0/16",
		Subnets: []netutil.Network{
			{
				// Define a VPC Network
				CIDRBlock: "10.0.1.0/24",
				Subnets: []netutil.Network{
					{
						// Define a Subnetwork within the VPC
						CIDRBlock: "10.0.1.0/28",
					},
					{
						// Another Subnetwork within the VPC
						CIDRBlock: "10.0.1.16/28",
					},
				},
			},
			{
				// Another VPC Network
				CIDRBlock: "10.0.2.0/24",
				Subnets: []netutil.Network{
					{
						// Subnetwork within the second VPC
						CIDRBlock: "10.0.2.0/28",
					},
				},
			},
		},
	}

	fmt.Println("Base Network CIDR:", baseNetwork.CIDRBlock)
	for i, vpc := range baseNetwork.Subnets {
		fmt.Printf("VPC Network %d CIDR: %s\n", i+1, vpc.CIDRBlock)
		for j, subnet := range vpc.Subnets {
			fmt.Printf("\tSubnetwork %d CIDR: %s\n", j+1, subnet.CIDRBlock)
		}
	}

	fmt.Println("\nDivide CIDR block into subnets to accommodate at least minimum number of subnets")

	minSubnets := 4 // Minimum number of subnets required
	fmt.Printf("Minimum number of subnets: %d\n", minSubnets)

	subnets, err := netutil.SubnettingByMininumSubnetCount(cidrBlock, minSubnets)
	if err != nil {
		fmt.Println(err)
	}

	for _, subnet := range subnets {
		fmt.Println(subnet)
	}
	fmt.Println("\nDivide CIDR block by a specified number of hosts")

	hostsPerSubnet := 500 // number of hosts you want in each subnet
	fmt.Printf("Number of hosts per subnet: %d\n", hostsPerSubnet)

	subnets, err = netutil.SubnettingByHosts(cidrBlock, hostsPerSubnet)
	if err != nil {
		fmt.Println(err)
	}

	for _, subnet := range subnets {
		fmt.Println(subnet)
	}

	fmt.Println("\nGet Network Address")
	networkAddress, err := netutil.GetNetworkAddr(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Network Address: %s\n", networkAddress)

	fmt.Println("\nGet Broadcast Address")
	broadcastAddress, err := netutil.GetBroadcastAddr(cidrBlock)
	if err != nil {
		fmt.Println(err)

	}
	fmt.Printf("Broadcast Address: %s\n", broadcastAddress)

	fmt.Println("\nCalculate the number of hosts that can be accomodated in a given CIDR block")

	hosts, err := netutil.GetSizeOfHosts(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("The CIDR block %s can accommodate %d hosts.\n", cidrBlock, hosts)

	fmt.Println("\nNew network")
	baseNet, err := netutil.New(cidrBlock)
	fmt.Printf("Base Network: %+v\n", baseNet)
	fmt.Printf("GetCIDRBlock(): %s\n", baseNet.GetCIDRBlock())
	fmt.Printf("GetNetworkAddress(): %s\n", baseNet.GetNetworkAddress())
	fmt.Printf("GetBroadcastAddress(): %s\n", baseNet.GetBroadcastAddress())
	fmt.Printf("GetPrefix(): %d\n", baseNet.GetPrefix())
	fmt.Printf("GetNetmask(): %s\n", baseNet.GetNetmask())
	fmt.Printf("GetHostCapacity(): %d\n", baseNet.GetHostCapacity())
	fmt.Printf("GetSubnets(): %v\n", baseNet.GetSubnets())

}
