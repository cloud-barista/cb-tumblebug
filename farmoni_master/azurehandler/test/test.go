// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for azurehandler.
//
// by powerkim@powerkim.co.kr, 2019.04.
package main


 import (
         "github.com/cloud-barista/poc-farmoni/farmoni_master/azurehandler"
	 "fmt"
	 "strconv"
 )

 func main() {


const (
	groupName = "VMGroupName"
	location = "westus2"
        virtualNetworkName = "virtualNetworkName"
        subnet1Name = "subnet1Name"
        subnet2Name = "subnet2Name"
        nsgName = "nsgName"
        ipName = "ipName"
        nicName = "nicName"

        baseName = "azurepowerkim"
        vmUserName = "powerkim"
        vmPassword = "powerkim"
        sshPublicKeyPath = "/root/.azure/azurepublickey.pem"
)



    credentialFile := "/root/.azure/credentials"
    connInfo := azurehandler.Connect(credentialFile)

        // by powerkim, defer resources.Cleanup(ctx)

        _, err := azurehandler.CreateGroup(connInfo, groupName, location)
        if err != nil {
                fmt.Println(err.Error())
        }

        _, err = azurehandler.CreateVirtualNetworkAndSubnets(connInfo, groupName, location, virtualNetworkName, subnet1Name, subnet2Name)

        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created vnet and 2 subnets")

        _, err = azurehandler.CreateNetworkSecurityGroup(connInfo, groupName, location, nsgName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created network security group")

/* PublicIP & NIC is made in CreateInstnaces()
        _, err = azurehandler.CreatePublicIP(connInfo, groupName, location, ipName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created public IP")
        _, err = azurehandler.CreateNIC(connInfo, groupName, location, virtualNetworkName, subnet1Name, nsgName, ipName, nicName)
        if err != nil {
                fmt.Println(err.Error())
        }
	fmt.Println("created nic")
*/


/*
type ImageInfo struct {
        Publisher string
        Offer     string
        Sku       string
        Version   string
}
*/
	imageInfo := azurehandler.ImageInfo{"Canonical", "UbuntuServer", "16.04.0-LTS", "latest"}

/*
type VMInfo struct {
        UserName string
        Password string
        SshPublicKeyPath string
}
*/
    vmInfo := azurehandler.VMInfo{vmUserName, vmPassword, sshPublicKeyPath}

/*
type NICInfo struct {
	VirtualNetworkName string
	SubnetName string
	NetworkSecurityGroup string
}
*/

    nicInfo := azurehandler.NICInfo{virtualNetworkName, subnet1Name, nsgName}

    //instanceIds := azurehandler.CreateInstances(connInfo, groupName, location, baseName, nicInfo, imageInfo, vmInfo, 2)
    instanceIds := azurehandler.CreateInstances(connInfo, groupName, location, baseName, nicInfo, imageInfo, vmInfo, 1)

    for _, v := range instanceIds {
	fmt.Println("\tInstanceName: ", *v)
    }

    // waiting for completion of new instance running.
    for i, _ := range instanceIds {
	    ipName := baseName + "IP" + strconv.Itoa(i)

	    // get public IP
	    publicIP, err := azurehandler.GetPublicIP(connInfo, groupName, ipName)
	    if(err != nil) {
                fmt.Println(err.Error())
	    }

//	    fmt.Printf("[PublicIP] %#v", publicIP);
	    fmt.Printf("[PublicIP] %s", *publicIP.PublicIPAddressPropertiesFormat.IPAddress);
    }

 }

