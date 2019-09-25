// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// AZURE COMPUTE Hander (AZURE-SDK-FOR-GO:COMPUTE Version 28.0.0, Thanks AZURE.)
//
// by powerkim@powerkim.co.kr, 2019.04.
package azurehandler

import (
        "context"
        "fmt"
        "io/ioutil"
        "log"
        "os"
        "time"
        "encoding/json"
        "strconv"

        "github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
        "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2017-09-01/network"
        "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-06-01/compute"

        "github.com/Azure/go-autorest/autorest"
        "github.com/Azure/go-autorest/autorest/to"
        "github.com/Azure/go-autorest/autorest/azure/auth"
        "github.com/Azure/go-autorest/autorest/azure"
)
// Information for connection
type ConnectionInfo struct {
	context context.Context
        subscriptionID string
	authorizer autorest.Authorizer
}

type ImageInfo struct {
	Publisher string
	Offer     string
	Sku       string
	Version   string
}

type VMInfo struct {
	UserName string
	Password string
	SshPublicKeyPath string
}

type NICInfo struct {
        VirtualNetworkName string
        SubnetName string
        NetworkSecurityGroup string
}

func Connect(credentialFilePath string) ConnectionInfo {
	var connInfo ConnectionInfo

	//ctx, cancel := context.WithTimeout(context.Background(), 6000*time.Second)
	//defer cancel()
	ctx, _ := context.WithTimeout(context.Background(), 6000*time.Second)
	connInfo.context = ctx

        // get subscritionID from auth file.
        authInfo, authErr := readJSON(credentialFilePath)
        if authErr != nil {
                log.Fatal(authErr)
        }
        connInfo.subscriptionID = (*authInfo)["subscriptionId"].(string)

        // get autorest.Authorizer Object.
        var err error
        connInfo.authorizer, err = auth.NewAuthorizerFromFile(azure.PublicCloud.ResourceManagerEndpoint)
        if err != nil {
                log.Fatal(err)
        }

	return connInfo
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



func getGroupsClient(connInfo ConnectionInfo) resources.GroupsClient {
        groupsClient := resources.NewGroupsClient(connInfo.subscriptionID)
        groupsClient.Authorizer = connInfo.authorizer
        return groupsClient
}

func CreateGroup(connInfo ConnectionInfo, groupName string, location string) (resources.Group, error) {
        groupsClient := getGroupsClient(connInfo)
        log.Println(fmt.Sprintf("creating resource group '%s' on location: %v", groupName, location))
        return groupsClient.CreateOrUpdate(
                connInfo.context,
                groupName,
                resources.Group{
                        Location: to.StringPtr(location),
                })
}

func DeleteGroup(connInfo ConnectionInfo, groupName string) (result resources.GroupsDeleteFuture, err error) {
        groupsClient := getGroupsClient(connInfo)
        log.Println(fmt.Sprintf("deleting resource group '%s'", groupName))
	return groupsClient.Delete(connInfo.context, groupName)
}

func getVnetClient(connInfo ConnectionInfo) network.VirtualNetworksClient {
        vnetClient := network.NewVirtualNetworksClient(connInfo.subscriptionID)
        vnetClient.Authorizer = connInfo.authorizer
        return vnetClient
}

func CreateVirtualNetworkAndSubnets(connInfo ConnectionInfo, groupName, location, vnetName, subnet1Name, subnet2Name string) (vnet network.VirtualNetwork, err error) {
        vnetClient := getVnetClient(connInfo)
        future, err := vnetClient.CreateOrUpdate(
                connInfo.context,
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

        err = future.WaitForCompletionRef(connInfo.context, vnetClient.Client)
        if err != nil {
                return vnet, fmt.Errorf("cannot get the vnet create or update future response: %v", err)
        }

        return future.Result(vnetClient)
}

// Network Interfaces (NIC's)
func getNicClient(connInfo ConnectionInfo) network.InterfacesClient {
        nicClient := network.NewInterfacesClient(connInfo.subscriptionID)
        nicClient.Authorizer = connInfo.authorizer
        return nicClient
}

// GetNic returns an existing network interface
func GetNic(connInfo ConnectionInfo, groupName string, nicName string) (network.Interface, error) {
        nicClient := getNicClient(connInfo)
        return nicClient.Get(connInfo.context, groupName, nicName, "")
}

// Network Security Groups
func getNsgClient(connInfo ConnectionInfo) network.SecurityGroupsClient {
        nsgClient := network.NewSecurityGroupsClient(connInfo.subscriptionID)
        nsgClient.Authorizer = connInfo.authorizer
        return nsgClient
}

// CreateNetworkSecurityGroup creates a new network security group with rules set for allowing SSH and HTTPS use
func CreateNetworkSecurityGroup(connInfo ConnectionInfo, groupName, location, nsgName string) (nsg network.SecurityGroup, err error) {
        nsgClient := getNsgClient(connInfo)
        future, err := nsgClient.CreateOrUpdate(
                connInfo.context,
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
                                                Name: to.StringPtr("allow_agent"), // added by powekrim. 2019.05.12, for agent port
                                                SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
                                                        Protocol:                 network.SecurityRuleProtocolTCP,
                                                        SourceAddressPrefix:      to.StringPtr("0.0.0.0/0"),
                                                        SourcePortRange:          to.StringPtr("1-65535"),
                                                        DestinationAddressPrefix: to.StringPtr("0.0.0.0/0"),
                                                        DestinationPortRange:     to.StringPtr("2019"),
                                                        Access:                   network.SecurityRuleAccessAllow,
                                                        Direction:                network.SecurityRuleDirectionInbound,
                                                        Priority:                 to.Int32Ptr(150),
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

        err = future.WaitForCompletionRef(connInfo.context, nsgClient.Client)
        if err != nil {
                return nsg, fmt.Errorf("cannot get nsg create or update future response: %v", err)
        }

        return future.Result(nsgClient)
}

// VNet Subnets
func getSubnetsClient(connInfo ConnectionInfo) network.SubnetsClient {
        subnetsClient := network.NewSubnetsClient(connInfo.subscriptionID)
        subnetsClient.Authorizer = connInfo.authorizer
        return subnetsClient
}

// GetVirtualNetworkSubnet returns an existing subnet from a virtual network
func GetVirtualNetworkSubnet(connInfo ConnectionInfo, groupName, vnetName string, subnetName string) (network.Subnet, error) {
        subnetsClient := getSubnetsClient(connInfo)
        return subnetsClient.Get(connInfo.context, groupName, vnetName, subnetName, "")
}

// Public IP Addresses
func getIPClient(connInfo ConnectionInfo) network.PublicIPAddressesClient {
        ipClient := network.NewPublicIPAddressesClient(connInfo.subscriptionID)
        ipClient.Authorizer = connInfo.authorizer
        return ipClient
}

// CreatePublicIP creates a new public IP
func CreatePublicIP(connInfo ConnectionInfo, groupName, location, ipName string) (ip network.PublicIPAddress, err error) {
        ipClient := getIPClient(connInfo)
        future, err := ipClient.CreateOrUpdate(
                connInfo.context,
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

        err = future.WaitForCompletionRef(connInfo.context, ipClient.Client)
        if err != nil {
                return ip, fmt.Errorf("cannot get public ip address create or update future response: %v", err)
        }

        return future.Result(ipClient)
}

// CreateNIC creates a new network interface. The Network Security Group is not a required parameter
func CreateNIC(connInfo ConnectionInfo, groupName, location, vnetName, subnetName, nsgName, ipName, nicName string) (nic network.Interface, err error) {
        subnet, err := GetVirtualNetworkSubnet(connInfo, groupName, vnetName, subnetName)
        if err != nil {
                log.Fatalf("failed to get subnet: %v", err)
        }

        ip, err := GetPublicIP(connInfo, groupName, ipName)
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
                nsg, err := GetNetworkSecurityGroup(connInfo, groupName, nsgName)
                if err != nil {
                        log.Fatalf("failed to get nsg: %v", err)
                }
                nicParams.NetworkSecurityGroup = &nsg
        }

        nicClient := getNicClient(connInfo)
        future, err := nicClient.CreateOrUpdate(connInfo.context, groupName, nicName, nicParams)
        if err != nil {
                return nic, fmt.Errorf("cannot create nic: %v", err)
        }

        err = future.WaitForCompletionRef(connInfo.context, nicClient.Client)
        if err != nil {
                return nic, fmt.Errorf("cannot get nic create or update future response: %v", err)
        }

        return future.Result(nicClient)
}

// GetPublicIP returns an existing public IP
func GetPublicIP(connInfo ConnectionInfo, groupName, ipName string) (network.PublicIPAddress, error) {
        ipClient := getIPClient(connInfo)
        return ipClient.Get(connInfo.context, groupName, ipName, "")
}

// GetNetworkSecurityGroup returns an existing network security group
func GetNetworkSecurityGroup(connInfo ConnectionInfo, groupName, nsgName string) (network.SecurityGroup, error) {
        nsgClient := getNsgClient(connInfo)
        return nsgClient.Get(connInfo.context, groupName, nsgName, "")
}

func getVMClient(connInfo ConnectionInfo) compute.VirtualMachinesClient {
        vmClient := compute.NewVirtualMachinesClient(connInfo.subscriptionID)
        vmClient.Authorizer = connInfo.authorizer
        return vmClient
}

// CreateVM creates a new virtual machine with the specified name using the specified NIC.
// Username, password, and sshPublicKeyPath determine logon credentials.
func CreateInstances(connInfo ConnectionInfo, groupName, location, baseName string, nicInfo NICInfo, imageInfo ImageInfo, vmInfo VMInfo, maxCount int) []*string {

var future compute.VirtualMachinesCreateOrUpdateFuture
vmClient := getVMClient(connInfo)

instanceIds :=  make([]*string, maxCount)
for i:=0; i<maxCount; i++ {
vmName := baseName + strconv.Itoa(i)

        // create PublicIP
	ipName := baseName + "IP" + strconv.Itoa(i)
	CreatePublicIP(connInfo, groupName, location, ipName)
	fmt.Printf("created public IP: %s\n", ipName)

        // create NIC
	nicName := baseName + "NIC" + strconv.Itoa(i)
        nic, _ := CreateNIC(connInfo, groupName, location, nicInfo.VirtualNetworkName, nicInfo.SubnetName, nicInfo.NetworkSecurityGroup, ipName, nicName)
	fmt.Printf("created nic: %s\n", nicName)

        var sshKeyData string
        if _, err := os.Stat(vmInfo.SshPublicKeyPath); err == nil {
                sshBytes, err := ioutil.ReadFile(vmInfo.SshPublicKeyPath)
                if err != nil {
                        log.Fatalf("failed to read SSH key data: %v", err)
                }
                sshKeyData = string(sshBytes)
        }
        //vmClient := getVMClient(connInfo)
	var err error
        future, err = vmClient.CreateOrUpdate(
                connInfo.context,
                groupName,
                vmName,
                compute.VirtualMachine{
                        Location: to.StringPtr(location),
                        VirtualMachineProperties: &compute.VirtualMachineProperties{
                                HardwareProfile: &compute.HardwareProfile{
                                        VMSize: compute.VirtualMachineSizeTypesBasicA0,
                                },
                                StorageProfile: &compute.StorageProfile{
                                        ImageReference: &compute.ImageReference{
                                                Publisher: to.StringPtr(imageInfo.Publisher),
                                                Offer:     to.StringPtr(imageInfo.Offer),
                                                Sku:       to.StringPtr(imageInfo.Sku),
                                                Version:   to.StringPtr(imageInfo.Version),
                                        },
                                },
                                OsProfile: &compute.OSProfile{
                                        ComputerName:  to.StringPtr(vmName),
                                        AdminUsername: to.StringPtr(vmInfo.UserName),
                                        AdminPassword: to.StringPtr(vmInfo.Password),
                                        LinuxConfiguration: &compute.LinuxConfiguration{
                                                SSH: &compute.SSHConfiguration{
                                                        PublicKeys: &[]compute.SSHPublicKey{
                                                                {
                                                                        Path: to.StringPtr(
                                                                                fmt.Sprintf("/home/%s/.ssh/authorized_keys",
                                                                                        vmInfo.UserName)),
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
		log.Fatal(err)
                return nil
        }
/*
        err = future.WaitForCompletionRef(connInfo.context, vmClient.Client)
        if err != nil {
                log.Fatal(err)
                return nil
        }
*/
	instanceIds[i] = &vmName

fmt.Printf("======= accepted: %s\n", vmName)

} // enf of for until maxcount
        err := future.WaitForCompletionRef(connInfo.context, vmClient.Client)
        if err != nil {
		log.Fatal(err)
                return nil
        }

        future.Result(vmClient)

        return instanceIds
}



func DestroyInstances(connInfo ConnectionInfo, groupName string, instanceNames []*string) {

	vmClient := getVMClient(connInfo)

	for _, instanceName := range instanceNames {
		_, err := vmClient.Delete(connInfo.context, groupName, *instanceName)
		if err != nil {
			log.Fatal(err)
		}
	}
}

