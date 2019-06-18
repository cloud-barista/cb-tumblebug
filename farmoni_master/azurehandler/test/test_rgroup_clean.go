// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for azurehandler.
//
// by powerkim@powerkim.co.kr, 2019.04.
package main


 import (
         "github.com/cloud-barista/poc-farmoni/farmoni_master/azurehandler"
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

    azurehandler.DeleteGroup(connInfo, groupName)

 }

