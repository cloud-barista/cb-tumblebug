package main

import (
	//"encoding/json"
	//"os"
	"fmt"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/mcis"
)

/*
func getInteractiveRequest() {
	command := -1
	fmt.Println("[Select opt (0:API-server, 1:create-vm, 2:delete-vm, 3:list-vm, 4:monitor-vm]")
	fmt.Print("Your section : ")
	fmt.Scanln(&command)
	fmt.Println(command)
*/

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
}

var SPIDER_URL string

func main() {
	//SPIDER_URL = os.Getenv("SPIDER_URL")
	SPIDER_URL = "http://localhost:1024"

	/*
	// Step 1. Register Cloud Driver info
	// Option 1
	
	// aws_driver01 := cloudDriverRegisterRequestInfo{
	// 	DriverName:        "aws-driver01",
	// 	ProviderName:      "AWS",
	// 	DriverLibFileName: "aws-driver-v1.0.so",
	// }
	

	// Option 2
	myCloudDriverRegisterRequestInfo := `{"DriverName":"aws-driver01","ProviderName":"AWS", "DriverLibFileName":"aws-driver-v1.0.so"}`
	aws_driver01 := cloudDriverRegisterRequestInfo{}
	json.Unmarshal([]byte(myCloudDriverRegisterRequestInfo), &aws_driver01)

	err := registerCloudInfo("driver", aws_driver01)
	if err != nil {
		cblog.Error(err)
		os.Exit(1)
	}

	// Step 2. Register Cloud Credential info

	// Step 3. Register Cloud Region info

	// Cloud Region Info for Shooter
	// Option 1
	region_aws_canada_central := cloudRegionRegisterRequestInfo{
		RegionName:   "aws-canada-central",
		ProviderName: "AWS",
		KeyValueInfoList: []KeyValue{
			{
				Key:   "Region",
				Value: "ca-central-1",
			},
		},
	}

	// for test service
	// Option 1
	region_aws_california_north := cloudRegionRegisterRequestInfo{
		RegionName:   "aws-california-north",
		ProviderName: "AWS",
		KeyValueInfoList: []KeyValue{
			{
				Key:   "Region",
				Value: "us-west-1",
			},
		},
	}

	err = registerCloudInfo("region", region_aws_canada_central)
	if err != nil {
		cblog.Error(err)
		os.Exit(1)
	}

	err = registerCloudInfo("region", region_aws_california_north)
	if err != nil {
		cblog.Error(err)
		os.Exit(1)
	}

	// Step 4. Create Cloud connection config
	// Cloud Connection Config Info for Shooter
	aws_canada_central_config := cloudConnectionConfigCreateRequestInfo{
		ConfigName:     "aws-canada-central-config",
		ProviderName:   "AWS",
		DriverName:     "aws-driver01",
		CredentialName: "aws-credential01",
		RegionName:     "aws-canada-central",
	}

	// for test service
	aws_california_north_config := cloudConnectionConfigCreateRequestInfo{
		ConfigName:     "aws-california-north-config",
		ProviderName:   "AWS",
		DriverName:     "aws-driver01",
		CredentialName: "aws-credential01",
		RegionName:     "aws-california-north",
	}

	err = registerCloudInfo("connectionconfig", aws_canada_central_config)
	if err != nil {
		cblog.Error(err)
		os.Exit(1)
	}

	err = registerCloudInfo("connectionconfig", aws_california_north_config)
	if err != nil {
		cblog.Error(err)
		os.Exit(1)
	}
	*/

	fmt.Println("Listing all namespaces")
	common.RestGetAllNs
}
