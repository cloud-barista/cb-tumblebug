package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/spf13/cobra"
)

func main() {
	var exampleCmd = &cobra.Command{
		Use:   "./netutil",
		Short: "Example program for the netutil package",
		Long: `
This program demonstrates the usage of the netutil package.`,
		Example: `./netutil --cidr "10.0.0.0/16" --minsubnets 4 --hosts 500
or
./netutil -c "10.0.0.0/16" -s 4 -n 500`,
		Run: runExample,
	}

	// Command-line flags with shorthand
	exampleCmd.PersistentFlags().StringP("cidr", "c", "192.168.0.0/16", "Base network CIDR block")
	exampleCmd.PersistentFlags().IntP("minsubnets", "s", 4, "Minimum number of subnets required")
	exampleCmd.PersistentFlags().IntP("hosts", "n", 500, "Number of hosts per subnet")

	if err := exampleCmd.Execute(); err != nil {
		log.Fatalf("Error executing netutil-example: %s", err)
	}
}

func runExample(cmd *cobra.Command, args []string) {

	// Retrieve the flag values
	cidrBlock, _ := cmd.Flags().GetString("cidr")
	minSubnets, _ := cmd.Flags().GetInt("minsubnets")
	hostsPerSubnet, _ := cmd.Flags().GetInt("hosts")

	fmt.Println("Starting netuil example")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nDivide CIDR block into subnets to accommodate at least minimum number of subnets")

	fmt.Printf("Minimum number of subnets: %d\n", minSubnets)

	subnets, err := netutil.SubnettingByMininumSubnetCount(cidrBlock, minSubnets)
	if err != nil {
		fmt.Println(err)
	}

	for _, subnet := range subnets {
		fmt.Println(subnet)
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nDivide CIDR block by a specified number of hosts")
	fmt.Printf("Number of hosts per subnet: %d\n", hostsPerSubnet)

	subnets, err = netutil.SubnettingByHosts(cidrBlock, hostsPerSubnet)
	if err != nil {
		fmt.Println(err)
	}

	for _, subnet := range subnets {
		fmt.Println(subnet)
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nGet Network Address")
	networkAddress, err := netutil.GetNetworkAddr(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Network Address: %s\n", networkAddress)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nGet Broadcast Address")
	broadcastAddress, err := netutil.GetBroadcastAddr(cidrBlock)
	if err != nil {
		fmt.Println(err)

	}
	fmt.Printf("Broadcast Address: %s\n", broadcastAddress)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nGet Prefix")
	prefix, err := netutil.GetPrefix(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Prefix: %d\n", prefix)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nGet Netmask")
	netmask, err := netutil.GetNetmask(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Netmask: %s\n", netmask)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nCalculate the number of hosts that can be accomodated in a given CIDR block")

	hosts, err := netutil.GetSizeOfHosts(cidrBlock)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("The CIDR block %s can accommodate %d hosts.\n", cidrBlock, hosts)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nNew network")
	net, err := netutil.NewNetwork(cidrBlock)
	fmt.Printf(" Network: %+v\n", net)
	fmt.Printf(" GetCIDRBlock(): %s\n", net.GetCIDRBlock())
	fmt.Printf(" GetSubnets(): %v\n", net.GetSubnets())

	networkDetails, err := netutil.NewNetworkDetails(cidrBlock)
	fmt.Printf("\n NetworkDetails: %+v\n", networkDetails)
	fmt.Printf(" GetCIDRBlock(): %s\n", networkDetails.GetCIDRBlock())
	fmt.Printf(" GetNetworkAddress(): %s\n", networkDetails.GetNetworkAddress())
	fmt.Printf(" GetBroadcastAddress(): %s\n", networkDetails.GetBroadcastAddress())
	fmt.Printf(" GetPrefix(): %d\n", networkDetails.GetPrefix())
	fmt.Printf(" GetNetmask(): %s\n", networkDetails.GetNetmask())
	fmt.Printf(" GetHostCapacity(): %d\n", networkDetails.GetHostCapacity())
	fmt.Printf(" GetSubnets(): %v\n", networkDetails.GetSubnets())

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\n(Under development) Network template example")
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

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\n(Under development) Design multi-cloud network")
	jsonData := `{
        "baseNetwork": {
            "name": "BaseNetwork1",
            "cidrBlock": "10.0.0.0/16",
            "subnets": [
                {
                    "name": "CloudNetwork1",
                    "cidrBlock": "10.0.1.0/24",
                    "subnets": [
                        {"name": "Subnet1", "cidrBlock": "10.0.1.0/26"},
                        {"name": "Subnet2", "cidrBlock": "10.0.1.64/26"},
                        {"name": "Subnet3", "cidrBlock": "10.0.1.128/26"},
                        {"name": "Subnet4", "cidrBlock": "10.0.1.192/26"}
                    ]
                }
            ]
        }
    }`

	var config netutil.NetworkConfig
	err = json.Unmarshal([]byte(jsonData), &config)
	if err != nil {
		log.Fatalf("Error occurred during unmarshaling. Error: %s", err.Error())
	}

	prettyConfig, err := json.MarshalIndent(config, "", "   ")
	if err != nil {
		log.Fatalf("marshaling error: %s", err)
	}
	fmt.Printf("[Configuration]\n%s", string(prettyConfig))
}
