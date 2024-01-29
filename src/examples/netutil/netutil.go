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
	fmt.Printf("Divide CIDR block by a specified number of hosts\n")

	fmt.Println("[Usecase] Get superneted VPCs and its subnets inside")
	fmt.Printf("- Base network: %v\n", cidrBlock)
	fmt.Printf("- Minimum number of VPCs (subnets of the base network): %d\n", minSubnets)
	fmt.Printf("- Subnets in a VPC base on the number of hosts per subnet: %d\n", hostsPerSubnet)

	subnets, err := netutil.SubnettingByMinimumSubnetCount(cidrBlock, minSubnets)
	if err != nil {
		fmt.Println(err)
	}

	for i, vpc := range subnets {
		fmt.Printf("\nVPC[%03d]:\t%v\nSubnets:\t", i+1, vpc)
		vpcsubnets, err := netutil.SubnettingByMinimumHosts(vpc, hostsPerSubnet)
		if err != nil {
			fmt.Println(err)
		}
		for j, subnet := range vpcsubnets {
			fmt.Printf("%v", subnet)
			if j < len(vpcsubnets)-1 {
				fmt.Print(", ")
			}
		}
		fmt.Println("")
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// fmt.Println("\nDivide CIDR block by a specified number of hosts")
	// fmt.Printf("Number of hosts per subnet: %d\n", hostsPerSubnet)

	// subnets, err = netutil.SubnettingByMinimumHosts(cidrBlock, hostsPerSubnet)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// for _, subnet := range subnets {
	// 	fmt.Printf("%v, ", subnet)
	// }

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\n\nGet Network Address")
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
	fmt.Println("\nValidate a network configuration")

	expectedInput := `{
        "networkConfiguration": {
            "name": "BaseNetwork (note - a CIDR block of logical global mutli-cloud network)",
            "cidrBlock": "10.0.0.0/16",
            "subnets": [
                {
                    "name": "CloudNetwork1 (note - a CIDR block to be assigned to cloud network such as VPC network)",
                    "cidrBlock": "10.0.1.0/24",
                    "subnets": [
                        {"name": "Subnet1", "cidrBlock": "10.0.1.0/26"},
                        {"name": "Subnet2", "cidrBlock": "10.0.1.64/26"},
                        {"name": "Subnet3", "cidrBlock": "10.0.1.128/26"},
                        {"name": "Subnet4", "cidrBlock": "10.0.1.192/26"}
                    ]
                },
				{
                    "name": "CloudNetwork2 (note - a CIDR block to be assigned to cloud network such as VPC network)",
                    "cidrBlock": "10.0.2.0/24",
                    "subnets": [
                        {"name": "Subnet1", "cidrBlock": "10.0.2.0/26"},
                        {"name": "Subnet2", "cidrBlock": "10.0.2.64/26"},
                        {"name": "Subnet3", "cidrBlock": "10.0.2.128/26"},
                        {"name": "Subnet4", "cidrBlock": "10.0.2.192/26"}
                    ]
                }
            ]
        }
    }`
	fmt.Printf("[Expected input]\n%s\n", expectedInput)

	var netConf netutil.NetworkConfig
	err = json.Unmarshal([]byte(expectedInput), &netConf)
	if err != nil {
		fmt.Printf("Error occurred during unmarshaling. Error: %s\n", err.Error())
	}

	network := netConf.NetworkConfiguration
	pretty, err := json.MarshalIndent(network, "", "   ")
	if err != nil {
		fmt.Printf("marshaling error: %s\n", err)
	}
	fmt.Printf("[Network configuration to validate]\n%s\n", string(pretty))

	if err := netutil.ValidateNetwork(network); err != nil {
		fmt.Println("Network configuration is invalid.")
	}

	fmt.Println("Network configuration is valid.")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\nSubnetting a CIDR block by requests")
	request := netutil.SubnettingRequest{
		CIDRBlock: cidrBlock,
		SubnettingRules: []netutil.SubnettingRule{
			{Type: netutil.SubnettingRuleTypeMinSubnets, Value: minSubnets},
			{Type: netutil.SubnettingRuleTypeMinHosts, Value: hostsPerSubnet},
		},
	}

	pretty, err = json.MarshalIndent(request, "", "   ")
	if err != nil {
		fmt.Printf("marshaling error: %s\n", err)
	}
	fmt.Printf("[Subnetting request]\n%s\n", string(pretty))

	// Subnetting by requests
	networkConfig, err := netutil.SubnettingBy(request)
	if err != nil {
		fmt.Println("Error subnetting network:", err)
		return
	}

	pretty, err = json.MarshalIndent(networkConfig, "", "   ")
	if err != nil {
		fmt.Printf("marshaling error: %s\n", err)
	}
	fmt.Printf("[Subnetting result]\n%s\n", string(pretty))

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\n\nNextSubnet() test example")

	baseNetwork := "10.0.0.0/16"
	currentSubnet0 := "10.0.0.0/18"
	currentSubnet1 := "10.0.64.0/18"
	currentSubnet2 := "10.0.128.0/18"
	currentSubnet3 := "10.0.192.0/18"

	fmt.Printf("[NextSubnet() Case1] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet0)
	nextSubnet, err := netutil.NextSubnet(currentSubnet0, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Next Subnet:", nextSubnet)
	}

	fmt.Printf("[NextSubnet() Case2] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet1)
	nextSubnet, err = netutil.NextSubnet(currentSubnet1, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Next Subnet:", nextSubnet)
	}

	fmt.Printf("[NextSubnet() Case3] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet2)
	nextSubnet, err = netutil.NextSubnet(currentSubnet2, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Next Subnet:", nextSubnet)
	}

	fmt.Printf("[NextSubnet() Case4] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet3)
	nextSubnet, err = netutil.NextSubnet(currentSubnet3, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Next Subnet:", nextSubnet)
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	fmt.Println("\n\nPreviousSubnet() test example")

	fmt.Printf("[PreviousSubnet() Case1] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet0)
	previousSubnet, err := netutil.PreviousSubnet(currentSubnet0, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Previous Subnet:", previousSubnet)
	}

	fmt.Printf("[PreviousSubnet() Case2] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet1)
	previousSubnet, err = netutil.PreviousSubnet(currentSubnet1, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Previous Subnet:", previousSubnet)
	}

	fmt.Printf("[PreviousSubnet() Case3] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet2)
	previousSubnet, err = netutil.PreviousSubnet(currentSubnet2, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Previous Subnet:", previousSubnet)
	}

	fmt.Printf("[PreviousSubnet() Case4] Base Network CIDR: %s, Current Subnet CIDR: %s\n", baseNetwork, currentSubnet3)
	previousSubnet, err = netutil.PreviousSubnet(currentSubnet3, baseNetwork)
	if err != nil {
		fmt.Println(" Error:", err)
	} else {
		fmt.Println(" Previous Subnet:", previousSubnet)
	}

}
