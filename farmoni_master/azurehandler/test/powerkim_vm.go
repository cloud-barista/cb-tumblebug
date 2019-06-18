// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-06-01/compute"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/azure"
)

// inserted by pwerkim, from compute/constants.go
const (
        // used by default for VM operations
        publisher  = "Canonical"
        offer      = "UbuntuServer"
        sku        = "16.04.0-LTS"

        groupName        = "VMGroupName"
        location        = "westus2"
)

var authorizer autorest.Authorizer
var subscriptionID string
func init() {

	// get subscritionID from auth file.
        //authInfo, authErr := readJSON(os.Getenv("AZURE_AUTH_LOCATION"))
        authInfo, authErr := readJSON("/root/.azure/quickstart.auth")
        if authErr != nil {
                fmt.Println(authErr.Error())
        }
        subscriptionID = (*authInfo)["subscriptionId"].(string)

	// get autorest.Authorizer Object.
	var err error
	authorizer, err = auth.NewAuthorizerFromFile(azure.PublicCloud.ResourceManagerEndpoint)
        if err != nil {
                fmt.Println(err.Error())
        }

}

func getGroupsClient() resources.GroupsClient {
        groupsClient := resources.NewGroupsClient(subscriptionID)
	groupsClient.Authorizer = authorizer
        return groupsClient
}

func createGroup(ctx context.Context, groupName string) (resources.Group, error) {
        groupsClient := getGroupsClient()
        log.Println(fmt.Sprintf("creating resource group '%s' on location: %v", groupName, location))
        return groupsClient.CreateOrUpdate(
                ctx,
                groupName,
                resources.Group{
                        Location: to.StringPtr(location),
                })
}

func getVnetClient() network.VirtualNetworksClient {
        vnetClient := network.NewVirtualNetworksClient(subscriptionID)
	vnetClient.Authorizer = authorizer
        return vnetClient
}

func CreateVirtualNetworkAndSubnets(ctx context.Context, vnetName, subnet1Name, subnet2Name string) (vnet network.VirtualNetwork, err error) {
        vnetClient := getVnetClient()
        future, err := vnetClient.CreateOrUpdate(
                ctx,
                groupName,
                vnetName,
                network.VirtualNetwork{
                        Location: to.StringPtr(location),
                        VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
                                AddressSpace: &network.AddressSpace{
                                        AddressPrefixes: &[]string{"10.0.0.0/8"},
                                },
                                Subnets: &[]network.Subnet{
                                        {
                                                Name: to.StringPtr(subnet1Name),
                                                SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
                                                        AddressPrefix: to.StringPtr("10.0.0.0/16"),
                                                },
                                        },
                                        {
                                                Name: to.StringPtr(subnet2Name),
                                                SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
                                                        AddressPrefix: to.StringPtr("10.1.0.0/16"),
                                                },
                                        },
                                },
                        },
                })

        if err != nil {
                return vnet, fmt.Errorf("cannot create virtual network: %v", err)
        }

        err = future.WaitForCompletionRef(ctx, vnetClient.Client)
        if err != nil {
                return vnet, fmt.Errorf("cannot get the vnet create or update future response: %v", err)
        }

        return future.Result(vnetClient)
}

// Network Interfaces (NIC's)
func getNicClient() network.InterfacesClient {
        nicClient := network.NewInterfacesClient(subscriptionID)
	nicClient.Authorizer = authorizer
        return nicClient
}

// GetNic returns an existing network interface
func GetNic(ctx context.Context, nicName string) (network.Interface, error) {
        nicClient := getNicClient()
        return nicClient.Get(ctx, groupName, nicName, "")
}

// Network Security Groups
func getNsgClient() network.SecurityGroupsClient {
        nsgClient := network.NewSecurityGroupsClient(subscriptionID)
	nsgClient.Authorizer = authorizer
        return nsgClient
}

// CreateNetworkSecurityGroup creates a new network security group with rules set for allowing SSH and HTTPS use
func CreateNetworkSecurityGroup(ctx context.Context, nsgName string) (nsg network.SecurityGroup, err error) {
        nsgClient := getNsgClient()
        future, err := nsgClient.CreateOrUpdate(
                ctx,
                groupName,
                nsgName,
                network.SecurityGroup{
                        Location: to.StringPtr(location),
                        SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
                                SecurityRules: &[]network.SecurityRule{
                                        {
                                                Name: to.StringPtr("allow_ssh"),
                                                SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
                                                        Protocol:                 network.SecurityRuleProtocolTCP,
                                                        SourceAddressPrefix:      to.StringPtr("0.0.0.0/0"),
                                                        SourcePortRange:          to.StringPtr("1-65535"),
                                                        DestinationAddressPrefix: to.StringPtr("0.0.0.0/0"),
                                                        DestinationPortRange:     to.StringPtr("22"),
                                                        Access:                   network.SecurityRuleAccessAllow,
                                                        Direction:                network.SecurityRuleDirectionInbound,
                                                        Priority:                 to.Int32Ptr(100),
                                                },
                                        },
                                        {
                                                Name: to.StringPtr("allow_https"),
                                                SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
                                                        Protocol:                 network.SecurityRuleProtocolTCP,
                                                        SourceAddressPrefix:      to.StringPtr("0.0.0.0/0"),
                                                        SourcePortRange:          to.StringPtr("1-65535"),
                                                        DestinationAddressPrefix: to.StringPtr("0.0.0.0/0"),
                                                        DestinationPortRange:     to.StringPtr("443"),
                                                        Access:                   network.SecurityRuleAccessAllow,
                                                        Direction:                network.SecurityRuleDirectionInbound,
                                                        Priority:                 to.Int32Ptr(200),
                                                },
                                        },
                                },
                        },
                },
        )

        if err != nil {
                return nsg, fmt.Errorf("cannot create nsg: %v", err)
        }

        err = future.WaitForCompletionRef(ctx, nsgClient.Client)
        if err != nil {
                return nsg, fmt.Errorf("cannot get nsg create or update future response: %v", err)
        }

        return future.Result(nsgClient)
}

// VNet Subnets
func getSubnetsClient() network.SubnetsClient {
        subnetsClient := network.NewSubnetsClient(subscriptionID)
	subnetsClient.Authorizer = authorizer
        return subnetsClient
}

// GetVirtualNetworkSubnet returns an existing subnet from a virtual network
func GetVirtualNetworkSubnet(ctx context.Context, vnetName string, subnetName string) (network.Subnet, error) {
        subnetsClient := getSubnetsClient()
        return subnetsClient.Get(ctx, groupName, vnetName, subnetName, "")
}

// Public IP Addresses
func getIPClient() network.PublicIPAddressesClient {
        ipClient := network.NewPublicIPAddressesClient(subscriptionID)
	ipClient.Authorizer = authorizer
        return ipClient
}

// CreatePublicIP creates a new public IP
func CreatePublicIP(ctx context.Context, ipName string) (ip network.PublicIPAddress, err error) {
        ipClient := getIPClient()
        future, err := ipClient.CreateOrUpdate(
                ctx,
                groupName,
                ipName,
                network.PublicIPAddress{
                        Name:     to.StringPtr(ipName),
                        Location: to.StringPtr(location),
                        PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
                                PublicIPAddressVersion:   network.IPv4,
                                PublicIPAllocationMethod: network.Static,
                        },
                },
        )

        if err != nil {
                return ip, fmt.Errorf("cannot create public ip address: %v", err)
        }

        err = future.WaitForCompletionRef(ctx, ipClient.Client)
        if err != nil {
                return ip, fmt.Errorf("cannot get public ip address create or update future response: %v", err)
        }

        return future.Result(ipClient)
}

// CreateNIC creates a new network interface. The Network Security Group is not a required parameter
func CreateNIC(ctx context.Context, vnetName, subnetName, nsgName, ipName, nicName string) (nic network.Interface, err error) {
        subnet, err := GetVirtualNetworkSubnet(ctx, vnetName, subnetName)
        if err != nil {
                log.Fatalf("failed to get subnet: %v", err)
        }

        ip, err := GetPublicIP(ctx, ipName)
        if err != nil {
                log.Fatalf("failed to get ip address: %v", err)
        }

        nicParams := network.Interface{
                Name:     to.StringPtr(nicName),
                Location: to.StringPtr(location),
                InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
                        IPConfigurations: &[]network.InterfaceIPConfiguration{
                                {
                                        Name: to.StringPtr("ipConfig1"),
                                        InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
                                                Subnet: &subnet,
                                                PrivateIPAllocationMethod: network.Dynamic,
                                                PublicIPAddress:           &ip,
                                        },
                                },
                        },
                },
        }

        if nsgName != "" {
                nsg, err := GetNetworkSecurityGroup(ctx, nsgName)
                if err != nil {
                        log.Fatalf("failed to get nsg: %v", err)
                }
                nicParams.NetworkSecurityGroup = &nsg
        }

        nicClient := getNicClient()
        future, err := nicClient.CreateOrUpdate(ctx, groupName, nicName, nicParams)
        if err != nil {
                return nic, fmt.Errorf("cannot create nic: %v", err)
        }

        err = future.WaitForCompletionRef(ctx, nicClient.Client)
        if err != nil {
                return nic, fmt.Errorf("cannot get nic create or update future response: %v", err)
        }

        return future.Result(nicClient)
}

// GetPublicIP returns an existing public IP
func GetPublicIP(ctx context.Context, ipName string) (network.PublicIPAddress, error) {
        ipClient := getIPClient()
        return ipClient.Get(ctx, groupName, ipName, "")
}

// GetNetworkSecurityGroup returns an existing network security group
func GetNetworkSecurityGroup(ctx context.Context, nsgName string) (network.SecurityGroup, error) {
        nsgClient := getNsgClient()
        return nsgClient.Get(ctx, groupName, nsgName, "")
}

func getVMClient() compute.VirtualMachinesClient {
        vmClient := compute.NewVirtualMachinesClient(subscriptionID)
	vmClient.Authorizer = authorizer
        return vmClient
}

// CreateVM creates a new virtual machine with the specified name using the specified NIC.
// Username, password, and sshPublicKeyPath determine logon credentials.
func CreateVM(ctx context.Context, vmName, nicName, username, password, sshPublicKeyPath string) (vm compute.VirtualMachine, err error) {

	// see the network samples for how to create and get a NIC resource
	nic, _ := GetNic(ctx, nicName)

	var sshKeyData string
	if _, err = os.Stat(sshPublicKeyPath); err == nil {
		sshBytes, err := ioutil.ReadFile(sshPublicKeyPath)
		if err != nil {
			log.Fatalf("failed to read SSH key data: %v", err)
		}
		sshKeyData = string(sshBytes)
	} 
	vmClient := getVMClient()
	future, err := vmClient.CreateOrUpdate(
		ctx,
		//config.GroupName(),
		groupName,
		vmName,
		compute.VirtualMachine{
			//Location: to.StringPtr(config.Location()),
			Location: to.StringPtr(location),
			VirtualMachineProperties: &compute.VirtualMachineProperties{
				HardwareProfile: &compute.HardwareProfile{
					VMSize: compute.VirtualMachineSizeTypesBasicA0,
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr(publisher),
						Offer:     to.StringPtr(offer),
						Sku:       to.StringPtr(sku),
						Version:   to.StringPtr("latest"),
					},
				},
				OsProfile: &compute.OSProfile{
					ComputerName:  to.StringPtr(vmName),
					AdminUsername: to.StringPtr(username),
					AdminPassword: to.StringPtr(password),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path: to.StringPtr(
										fmt.Sprintf("/home/%s/.ssh/authorized_keys",
											username)),
									KeyData: to.StringPtr(sshKeyData),
								},
							},
						},
					},
				},
				NetworkProfile: &compute.NetworkProfile{
					NetworkInterfaces: &[]compute.NetworkInterfaceReference{
						{
							ID: nic.ID,
							NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
								Primary: to.BoolPtr(true),
							},
						},
					},
				},
			},
		},
	)
	if err != nil {
		return vm, fmt.Errorf("cannot create vm: %v", err)
	}

	err = future.WaitForCompletionRef(ctx, vmClient.Client)
	if err != nil {
		return vm, fmt.Errorf("cannot get the vm create or update future response: %v", err)
	}
	return future.Result(vmClient)
}


func main() {

// added by powerkim
	virtualNetworkName := "virtualNetworkName"
	subnet1Name := "subnet1Name"
	subnet2Name := "subnet2Name"
	nsgName := "nsgName"
	ipName := "ipName"
	nicName := "nicName"
	vmName := "vmName"
	username := "powerkim"
	password := "powerkim"
	sshPublicKeyPath := "/root/.azure/azurepublickey.pem"

        ctx, cancel := context.WithTimeout(context.Background(), 6000*time.Second)
        defer cancel()
        // by powerkim, defer resources.Cleanup(ctx)

        _, err := createGroup(ctx, groupName)
        if err != nil {
                fmt.Println(err.Error())
        }

        _, err = CreateVirtualNetworkAndSubnets(ctx, virtualNetworkName, subnet1Name, subnet2Name)

        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created vnet and 2 subnets")

        _, err = CreateNetworkSecurityGroup(ctx, nsgName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created network security group")

        _, err = CreatePublicIP(ctx, ipName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created public IP")

        _, err = CreateNIC(ctx, virtualNetworkName, subnet1Name, nsgName, ipName, nicName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created nic")

        _, err = CreateVM(ctx, vmName, nicName, username, password, sshPublicKeyPath)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created VM")

}

func readJSON(path string) (*map[string]interface{}, error) {
        data, err := ioutil.ReadFile(path)
        if err != nil {
                log.Fatalf("failed to read file: %v", err)
        }
        contents := make(map[string]interface{})
        json.Unmarshal(data, &contents)
        return &contents, nil
}

