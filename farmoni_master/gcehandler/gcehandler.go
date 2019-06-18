// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// GCE Hander (GOOGLE-API-GO-CLIENT:COMPUTE Version 1.0, Thanks GCP.)
//
// by powerkim@powerkim.co.kr, 2019.04.
package gcehandler

import (
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/compute/v1"
    "google.golang.org/api/googleapi"
    "golang.org/x/oauth2/jwt"

    "fmt"
    "os"
    "log"
    "strings"
    "io/ioutil"
    "context"
    "strconv"
    "time"
)

func Connect(credentialFilePath string) *compute.Service {

        // Use the downloaded JSON file in its entirety
        //data, err := ioutil.ReadFile("/root/.gcp/credentials")
        data, err := ioutil.ReadFile(credentialFilePath)
        if err != nil {
                log.Fatal(err)
        }

        conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/compute")
        if err != nil {
                log.Fatal(err)
        }

        client := conf.Client(oauth2.NoContext)

	// Create GCE service
        computeService, err := compute.New(client)

        if err != nil {
                log.Fatal(err)
        }

	return computeService
}

func ConnectByEnv() *compute.Service {

        // Use the email & privateKey from the JSON file (good for ENV vars & CircleCI ;)
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
        }
	
        client := conf.Client(oauth2.NoContext)

        // Create GCE service
        computeService, err := compute.New(client)
        if err != nil {
                log.Fatal(err)
        }

        return computeService
}

/*
func Close() {
}
*/

func getRequestBody(instanceName string, region string, zone string, projectID string, imageURL string, machineType string,
                diskName string, minCount int, maxCount int, subNetwork string, networkName string, serviceAccoutsMail string) *compute.Instance {

        setFalse := false

        rb := &compute.Instance{
                MachineType:        machineType, 
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
                                        DiskName:    diskName, 
                                        SourceImage: imageURL,
                                },
                        },
                },
                NetworkInterfaces: []*compute.NetworkInterface{
                        &compute.NetworkInterface{
                                Subnetwork: subNetwork,
                                AccessConfigs: []*compute.AccessConfig{
                                        &compute.AccessConfig{
                                                Type: "ONE_TO_ONE_NAT",
                                                Name: "External NAT",
                                        },
                                },
                                Network: networkName,
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
	return rb
}


func CreateInstances(computeService *compute.Service, region string, zone string, projectID string, 
		imageURL string, machineType string, minCount int, maxCount int, subNetwork string, 
		networkName string, serviceAccoutsMail string, baseName string) []*string {

	ctx := context.Background()

	instanceIds :=  make([]*string, maxCount)
	for i:=0; i<maxCount; i++ {
		instanceName := baseName + strconv.Itoa(i)
		diskName := "my-root-pd" + strconv.Itoa(i)
		rb := getRequestBody(instanceName, region, zone, projectID, imageURL, machineType,
			diskName, minCount, maxCount, subNetwork, networkName, serviceAccoutsMail)

		resp, err := computeService.Instances.Insert(projectID, zone, rb).Context(ctx).Do()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%#v\n", resp)
		instanceIds[i] = &instanceName

		inst, err := computeService.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
		log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
		if googleapi.IsNotModified(err) {
			log.Printf("Instance not modified since insert.")
		} else {
			log.Printf("Instance modified since insert.")
		}

	}
	
	return instanceIds 
}

func GetPublicIP(computeService *compute.Service, zone string, projectID string, instanceName string) string {

	ctx := context.Background()

	inst, err := computeService.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		log.Fatal(err)
	}

	 //log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}

	// @TODO now, fetch just first Network Interface.
	publicIP := inst.NetworkInterfaces[0].AccessConfigs[0].NatIP

    return publicIP
}

// now, use GetPublicIP() for fast develop.
func WaitForRun(computeService *compute.Service, zone string, projectID string, instanceName string) {

    for i:=0; ; i++ {
	publicIP := GetPublicIP(computeService, zone, projectID, instanceName)
	if(i==30) { os.Exit(3) }
	    if publicIP != "" {
		break;
	    }
	    // Let's give some time for Gooble to attach Public IP to the VM
	    time.Sleep(time.Second*3)
    } // end of for
}

func DestroyInstances(computeService *compute.Service, zone string, projectID string, instanceNames []*string) {

	ctx := context.Background()

	for _, instanceName := range instanceNames {
		_, err := computeService.Instances.Delete(projectID, zone, *instanceName).Context(ctx).Do()
		if err != nil {
			log.Fatal(err)
		}
	}
}

