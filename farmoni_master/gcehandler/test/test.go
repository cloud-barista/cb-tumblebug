// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for gcehandler.
//
// by powerkim@powerkim.co.kr, 2019.04.
package main


 import (
         "github.com/cloud-barista/poc-farmoni/farmoni_master/gcehandler"
	 "fmt"
 )

 func main() {

    credentialFile := "/root/.gcp/credentials"
    svc := gcehandler.Connect(credentialFile)

    region := "us-east1"
    zone := "us-east1-c"
    projectID := "ornate-course-236606"
    prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID
    imageURL := "projects/gce-uefi-images/global/images/centos-7-v20190326"
    machineType := prefix + "/zones/" + zone + "/machineTypes/f1-micro"
    subNetwork := prefix + "/regions/us-east1/subnetworks/default"
    networkName := prefix + "/global/networks/default"
    serviceAccoutsMail := "default"
    //baseName := "powerkimInstance"
    baseName := "gcepowerkim"

    instanceIds := gcehandler.CreateInstances(svc, region, zone, projectID, imageURL, machineType, 1, 2,
        subNetwork, networkName, serviceAccoutsMail, baseName)

    for _, v := range instanceIds {
	fmt.Println("\tInstanceName: ", *v)
    }

    // waiting for completion of new instance running.
    for _, v := range instanceIds {
	    // wait until running status
	    gcehandler.WaitForRun(svc, zone, projectID, *v)

	    // get public IP
	    publicIP := gcehandler.GetPublicIP(svc, zone, projectID, *v)
	    fmt.Println(publicIP);
    }

 }

