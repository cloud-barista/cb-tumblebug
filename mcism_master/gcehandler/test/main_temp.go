package main

import (
	"context"
	"fmt"
	"log"
	_ "os"
	_ "strings"
	 "io/ioutil"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	_ "golang.org/x/oauth2/jwt"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func main() {
	ctx := context.Background()

	// create a Service Account and download PrivateKey file: https://console.cloud.google.com/iam-admin/serviceaccounts?project=kubernetes-207013&authuser=0&organizationId=797100582747

	// Use the downloaded JSON file in its entirety
	data, err := ioutil.ReadFile("/root/.gcp/credentials")
	if err != nil {
		log.Fatal(err)
	}

	conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/compute")
	if err != nil {
		log.Fatal(err)
	}

	/* Use the email & privateKey from the JSON file (good for ENV vars & CircleCI ;)
	email := os.Getenv("GCE_EMAIL")
	privateKey := os.Getenv("GCE_PRIVATE_KEY")
	privateKey = strings.Replace(privateKey, "\\n", "\n", -1)

	// this key will have a bunch of '\n's which must be removed and replaced with hard returns.
	// paste result into CircleCI env var

	conf := &jwt.Config{
		Email:      email,
		PrivateKey: []byte(privateKey),
		Scopes: []string{
			"https://www.googleapis.com/auth/compute",
		},
		TokenURL: google.JWTTokenURL,
	}*/

	client := conf.Client(oauth2.NoContext)
	computeService, err := compute.New(client)
	if err != nil {
		log.Fatal(err)
	}

	setFalse := false

	//projectID := "kubernetes-207013"
	//projectID := "GOOGLE-API-GO"
	projectID := "ornate-course-236606"
	prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID
	//imageURL := "projects/coreos-cloud/global/images/coreos-stable-1745-7-0-v20180614"
	imageURL := "projects/gce-uefi-images/global/images/centos-7-v20190326"
	zone := "us-east1-b"
	//zone := "asia-northeast1-c"

	instanceName := "gcepowerkim1"

	rb := &compute.Instance{
		MachineType:        prefix + "/zones/" + zone + "/machineTypes/f1-micro",
		Name:               instanceName,
		CanIpForward:       false,
		DeletionProtection: false,
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				Mode:       "READ_WRITE",
				DeviceName: "instance-1",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "my-root-pd1",
					// powerkim DiskType:    prefix + "/zones/" + zone + "/diskTypes/pd-ssd",
					SourceImage: imageURL,
					// powerkim DiskSizeGb:  9,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				Subnetwork: prefix + "/regions/us-east1/subnetworks/default",
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: prefix + "/global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				//Email: email,
				Email: "default",
				Scopes: []string{
					compute.ComputeScope,
				},
			},
		},
		Scheduling: &compute.Scheduling{
			Preemptible:       true,
			OnHostMaintenance: "TERMINATE",
			AutomaticRestart:  &setFalse,
		},
	}


// for debugging without Insert
	ctx=ctx
	rb=rb
	computeService=computeService

///*
	resp, err := computeService.Instances.Insert(projectID, zone, rb).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n\n%#v\n", resp)
	//fmt.Printf("\n\n%v\n", resp)
//*/
	inst, err := computeService.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	fmt.Printf("\n\n%#v\n", inst)
	// for printing public IP

	if inst.NetworkInterfaces[0].AccessConfigs[0].NatIP != "" {
		fmt.Printf("\n%#v\n", inst.NetworkInterfaces[0].AccessConfigs[0].NatIP)
	} else {
		fmt.Println("Public IP is not avalable!!")
	}

//	log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}

/* second insert continously
	instanceName = "gcepowerkim2"
	rb.Name = instanceName
	rb.Disks[0].InitializeParams.DiskName = "my-root-pd2"

	resp, err = computeService.Instances.Insert(projectID, zone, rb).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", resp)

        inst, err = computeService.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
        log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
        if googleapi.IsNotModified(err) {
                log.Printf("Instance not modified since insert.")
        } else {
                log.Printf("Instance modified since insert.")
        }
*/

}
