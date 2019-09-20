// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista

package main

import (
	"github.com/cloud-barista/poc-mcism/mcism_master/azurehandler"
	"github.com/cloud-barista/poc-mcism/mcism_master/confighandler"
	"github.com/cloud-barista/poc-mcism/mcism_master/ec2handler"
	"github.com/cloud-barista/poc-mcism/mcism_master/gcehandler"
	"github.com/cloud-barista/poc-mcism/mcism_master/serverhandler/scp"
	"github.com/cloud-barista/poc-mcism/mcism_master/serverhandler/sshrun"

	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/cloud-barista/poc-mcism/mcism_agent/grpc_def"
	"google.golang.org/grpc"

	uuid "github.com/google/uuid"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
)

// Structs for REST API
type vmReq struct {
	Csp            string `json:"csp"`
	Vm_image_name  string `json:"vm_image_name"`
	Placement_algo string `json:"placement_algo"`
	Name           string `json:"name"`
	Description    string `json:"description"`

	Location string `json:"location"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	Disk_type   string `json:"disk_type"`
}
type mcisReq struct {
	Id             string  `json:"id"`
	Name           string  `json:"name"`
	Vm_req         []vmReq `json:"vm_req"`
	Vm_num         string  `json:"vm_num"`
	Placement_algo string  `json:"placement_algo"`
	Description    string  `json:"description"`
}

type mcisInfo struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Vm_num         string `json:"vm_num"`
	Status         string `json:"status"`
	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
}
type vmInfo struct {
	Id        string `json:"id"`
	Csp_vm_id string `json:"csp_vm_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`

	Public_ip     string `json:"public_ip"`
	Domain_name   string `json:"domain_name"`
	Cloud_id      string `json:"cloud_id"`
	Location      string `json:"location"`
	Vm_image_name string `json:"vm_image_name"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	Disk_type   string `json:"disk_type"`

	Vm_image     string `json:"vm_image"`
	Vm_spec      string `json:"vm_spec"`
	Network      string `json:"network"`
	Subnet       string `json:"subnet"`
	Net_security string `json:"net_security"`

	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
}
type vmStatusInfo struct {
	Id            string `json:"id"`
	Csp_vm_id     string `json:"csp_vm_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Cpu_status    string `json:"cpu_status"`
	Memory_status string `json:"memory_status"`
	Disk_status   string `json:"disk_status"`
}

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
}

const defaultMonitorPort = ":2019"

var masterConfigInfos confighandler.MASTERCONFIGTYPE

// Main Body

func apiServer() {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World! This is cloud-barista poc-mcism")
	})

	// Route
	e.POST("/mcis", restPostMcis)
	e.GET("/mcis/:id", restGetMcis)
	e.GET("/mcis", restGetAllMcis)
	e.PUT("/mcis/:id", restPutMcis)
	e.DELETE("/mcis/:id", restDelMcis)
	e.DELETE("/mcis", restDelAllMcis)

	e.POST("/resources/image", restPostImage)
	e.GET("/resources/image/:cbImageId", restGetImage)
	e.GET("/resources/image", restGetAllImage)
	e.PUT("/resources/image/:cbImageId", restPutImage)
	e.DELETE("/resources/image/:cbImageId", restDelImage)
	e.DELETE("/resources/image", restDelAllImage)
	/*
		e.POST("/resources/spec", restPostSpec)
		e.GET("/resources/spec/:id", restGetSpec)
		e.GET("/resources/spec", restGetAllSpec)
		e.PUT("/resources/spec/:id", restPutSpec)
		e.DELETE("/resources/spec/:id", restDelSpec)
		e.DELETE("/resources/spec", restDelAllSpec)

		e.POST("/resources/network", restPostNetwork)
		e.GET("/resources/network/:id", restGetNetwork)
		e.GET("/resources/network", restGetAllNetwork)
		e.PUT("/resources/network/:id", restPutNetwork)
		e.DELETE("/resources/network/:id", restDelNetwork)
		e.DELETE("/resources/network", restDelAllNetwork)

		e.POST("/resources/subnet", restPostSubnet)
		e.GET("/resources/subnet/:id", restGetSubnet)
		e.GET("/resources/subnet", restGetAllSubnet)
		e.PUT("/resources/subnet/:id", restPutSubnet)
		e.DELETE("/resources/subnet/:id", restDelSubnet)
		e.DELETE("/resources/subnet", restDelAllSubnet)

		e.POST("/resources/securityGroup", restPostSecurityGroup)
		e.GET("/resources/securityGroup/:id", restGetSecurityGroup)
		e.GET("/resources/securityGroup", restGetAllSecurityGroup)
		e.PUT("/resources/securityGroup/:id", restPutSecurityGroup)
		e.DELETE("/resources/securityGroup/:id", restDelSecurityGroup)
		e.DELETE("/resources/securityGroup", restDelAllSecurityGroup)

		e.POST("/resources/sshKey", restPostSshKey)
		e.GET("/resources/sshKey/:id", restGetSshKey)
		e.GET("/resources/sshKey", restGetAllSshKey)
		e.PUT("/resources/sshKey/:id", restPutSshKey)
		e.DELETE("/resources/sshKey/:id", restDelSshKey)
		e.DELETE("/resources/sshKey", restDelAllSshKey)
	*/
	e.Logger.Fatal(e.Start(":1323"))

}

func main() {

	fmt.Println("\n[PoC-MCISM (Multi-Cloud Infra Service Management Framework)]")
	fmt.Println("\nInitiating REST API Server ...")
	fmt.Println("\n[REST API call examples]")

	fmt.Println("[List MCISs]:\t\t curl <ServerIP>:1323/mcis")
	fmt.Println("[Create MCIS]:\t\t curl -X POST <ServerIP>:1323/mcis  -H 'Content-Type: application/json' -d '{<MCIS_REQ_JSON>}'")
	fmt.Println("[Get MCIS Info]:\t curl <ServerIP>:1323/mcis/<McisID>")
	fmt.Println("[Get MCIS status]:\t curl <ServerIP>:1323/mcis/<McisID>?action=monitor")
	fmt.Println("[Terminate MCIS]:\t curl <ServerIP>:1323/mcis/<McisID>?action=terminate")
	fmt.Println("[Del MCIS Info]:\t curl -X DELETE <ServerIP>:1323/mcis/<McisID>")
	fmt.Println("[Del MCISs Info]:\t curl -X DELETE <ServerIP>:1323/mcis")

	fmt.Println("\n")
	fmt.Println("[List Images]:\t\t curl <ServerIP>:1323/image")
	fmt.Println("[Create Image]:\t\t curl -X POST <ServerIP>:1323/image?action=create -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
	fmt.Println("[Register Image]:\t\t curl -X POST <ServerIP>:1323/image?action=register -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
	fmt.Println("[Get Image Info]:\t curl <ServerIP>:1323/image/<imageID>")
	fmt.Println("[Del Image Info]:\t curl -X DELETE <ServerIP>:1323/image/<imageID>")
	fmt.Println("[Del Images Info]:\t curl -X DELETE <ServerIP>:1323/image")

	// load config
	masterConfigInfos = confighandler.GetMasterConfigInfos()

	// Run API Server
	apiServer()

}

// MCIS API Proxy
func restPostMcis(c echo.Context) error {

	u := &mcisReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	createMcis(u)

	return c.JSON(http.StatusCreated, u)
}

func restGetMcis(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	mcisId := c.Param("id")

	action := c.QueryParam("action")
	fmt.Println("[Get MCIS requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend MCIS]")

		mapA := map[string]string{"message": "The MCIS has been suspended"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		mapA := map[string]string{"message": "The MCIS has been resumed"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "restart" {
		fmt.Println("[restart MCIS]")

		mapA := map[string]string{"message": "The MCIS has been restarted"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList := getVmList(mcisId)

		for _, v := range vmList {
			terminateVm(mcisId, v)
		}

		mapA := map[string]string{"message": "The MCIS has been terminated"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "monitor" {

		var content struct {
			Name   string         `json:"name"`
			Vm_num string         `json:"vm_num"`
			Status string         `json:"status"`
			Vm     []vmStatusInfo `json:"vm"`
		}

		fmt.Println("[monitor MCIS]")

		key := "/mcis/" + mcisId
		fmt.Println(key)
		keyValue, _ := store.Get(key)

		json.Unmarshal([]byte(keyValue.Value), &content)

		vmList := getVmList(mcisId)

		for _, v := range vmList {
			vmKey := "/mcis/" + mcisId + "/vm/" + v
			fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := vmStatusInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v

			vmIp := getVmIp(mcisId, v)
			vmIpPort := vmIp + defaultMonitorPort
			statusCpu, statusMem, statusDisk := monitorVm(vmIpPort)
			fmt.Println("[Status for MCIS] VM:" + vmIpPort + " CPU:" + statusCpu + " MEM:" + statusMem + " DISK:" + statusDisk)

			vmTmp.Cpu_status = statusCpu
			vmTmp.Memory_status = statusMem
			vmTmp.Disk_status = statusDisk

			content.Vm = append(content.Vm, vmTmp)
		}
		fmt.Printf("%+v\n", content)

		return c.JSON(http.StatusOK, &content)

	} else {

		var content struct {
			Name           string   `json:"name"`
			Vm_num         string   `json:"vm_num"`
			Status         string   `json:"status"`
			Vm             []vmInfo `json:"vm"`
			Placement_algo string   `json:"placement_algo"`
			Description    string   `json:"description"`
		}

		fmt.Println("[Get MCIS for id]" + mcisId)
		key := "/mcis/" + mcisId
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)

		vmList := getVmList(mcisId)

		for _, v := range vmList {
			vmKey := "/mcis/" + mcisId + "/vm/" + v
			fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := vmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v
			content.Vm = append(content.Vm, vmTmp)
		}
		fmt.Printf("%+v\n", content)

		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, &content)

	}
}

func restGetAllMcis(c echo.Context) error {

	var content struct {
		//Name string     `json:"name"`
		Mcis []mcisInfo `json:"mcis"`
	}

	mcisList := getMcisList()

	for _, v := range mcisList {

		key := "/mcis/" + v
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := mcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisTmp.Id = v
		content.Mcis = append(content.Mcis, mcisTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutMcis(c echo.Context) error {
	return nil
}

func restDelMcis(c echo.Context) error {

	mcisId := c.Param("id")

	err := delMcis(mcisId)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the MCIS"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The MCIS has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllMcis(c echo.Context) error {

	mcisList := getMcisList()

	for _, v := range mcisList {
		err := delMcis(v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All MCISs"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All MCISs has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

// MCIS Information Managemenet

func addVmToMcis(mcisId string, vmInfoData vmInfo) {

	key := "/mcis/" + mcisId + "/vm/" + vmInfoData.Id
	val, _ := json.Marshal(vmInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	fmt.Println("===========================")
	vmkeyValue, _ := store.Get(string(key))
	fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	fmt.Println("===========================")

}

func updateVmInfo(mcisId string, vmInfoData vmInfo) {
	key := "/mcis/" + mcisId + "/vm/" + vmInfoData.Id
	val, _ := json.Marshal(vmInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	fmt.Println("===========================")
	vmkeyValue, _ := store.Get(string(key))
	fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	fmt.Println("===========================")
}

func getMcisList() []string {

	fmt.Println("[Get MCISs")
	key := "/mcis"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mcisList = append(mcisList, strings.TrimPrefix(v.Key, "/mcis/"))
		}
	}
	for _, v := range mcisList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return mcisList

}

func getVmList(mcisId string) []string {

	fmt.Println("[getVmList]")
	key := "/mcis/" + mcisId
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var vmList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vm/") {
			vmList = append(vmList, strings.TrimPrefix(v.Key, (key+"/vm/")))
		}
	}
	for _, v := range vmList {
		fmt.Println("<" + v + ">")
	}
	fmt.Println("===============================================")
	return vmList

}

func delMcis(mcisId string) error {

	fmt.Println("[Delete MCIS] " + mcisId)

	// terminateMcis first
	terminateMcis(mcisId)
	// for deletion, need to wait untill termination is finished

	key := "/mcis/" + mcisId
	fmt.Println(key)

	vmList := getVmList(mcisId)

	// delete vms info
	for _, v := range vmList {
		vmKey := "/mcis/" + mcisId + "/vm/" + v
		fmt.Println(vmKey)
		err := store.Delete(vmKey)
		if err != nil {
			cblog.Error(err)
			return err
		}
	}
	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}

// MCIS Control

func createMcis(u *mcisReq) {

	u.Id = genUuid()
	vmRequest := u.Vm_req

	// cb-store
	fmt.Println("=========================== Put createSvc")
	Key := "/mcis/" + u.Id
	mapA := map[string]string{"name": u.Name, "description": u.Description, "status": "launching", "vm_num": u.Vm_num, "placement_algo": u.Placement_algo}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	for _, k := range vmRequest {

		//vmInfoData vmInfo
		vmInfoData := vmInfo{}
		vmInfoData.Id = uuid.New().String()
		vmInfoData.Name = k.Name

		vmInfoData.Placement_algo = k.Placement_algo

		vmInfoData.Location = k.Location
		vmInfoData.Cloud_id = k.Csp
		vmInfoData.Description = k.Description

		vmInfoData.Vcpu_size = k.Vcpu_size
		vmInfoData.Memory_size = k.Memory_size
		vmInfoData.Disk_size = k.Disk_size
		vmInfoData.Disk_type = k.Disk_type

		vmInfoData.Vm_image_name = k.Vm_image_name

		vmInfoData.Net_security = "TBD"
		vmInfoData.Network = "TBD"
		vmInfoData.Subnet = "TBD"
		vmInfoData.Vm_image = "TBD"
		vmInfoData.Vm_spec = "TBD"

		vmInfoData.Public_ip = "Not assigned yet"
		vmInfoData.Csp_vm_id = "Not assigned yet"
		vmInfoData.Domain_name = "Not assigned yet"
		vmInfoData.Status = "Launching"

		addVmToMcis(u.Id, vmInfoData)

		instanceIds, publicIPs := createVm(vmInfoData)

		vmInfoData.Public_ip = string(*publicIPs[0])
		vmInfoData.Csp_vm_id = string(*instanceIds[0])
		vmInfoData.Status = "Running"
		updateVmInfo(u.Id, vmInfoData)

	}

}

func createVm(vmInfoData vmInfo) ([]*string, []*string) {

	fmt.Printf("%+v\n", vmInfoData.Cloud_id)
	fmt.Printf("%+v\n", vmInfoData.Csp_vm_id)

	if strings.Compare(vmInfoData.Cloud_id, "aws") == 0 {
		return createVmAws(1)
	} else if strings.Compare(vmInfoData.Cloud_id, "gcp") == 0 {
		return createVmGcp(1)
	} else if strings.Compare(vmInfoData.Cloud_id, "azure") == 0 {
		return createVmAzure(1)
	} else {
		fmt.Println("==============ERROR=no matched provider_id=================")
		return nil, nil
	}

}

func createVmAws(count int) ([]*string, []*string) {

	// 1.1. create Servers(VM).
	// 1.2. get servers' public IP.
	// 1.3. insert MCISM Agent into Servers.
	// 1.4. execute Servers' Agent.
	// 1.5. add server list into etcd.

	// ==> AWS-EC2
	//region := "ap-northeast-2" // seoul region.
	region := masterConfigInfos.AWS.REGION // seoul region.

	svc := ec2handler.Connect(region)

	// 1.1. create Servers(VM).
	// some options are static for simple PoC.
	// These must be prepared before.

	imageId := masterConfigInfos.AWS.IMAGEID                       // ami-047f7b46bd6dd5d84
	instanceType := masterConfigInfos.AWS.INSTANCETYPE             // t2.micro
	securityGroupId := masterConfigInfos.AWS.SECURITYGROUPID       // sg-2334584f
	subnetid := masterConfigInfos.AWS.SUBNETID                     // subnet-8c4a53e4
	instanceNamePrefix := masterConfigInfos.AWS.INSTANCENAMEPREFIX // powerkimInstance_

	userName := masterConfigInfos.AWS.USERNAME   // ec2-user
	keyName := masterConfigInfos.AWS.KEYNAME     // aws.powerkim.keypair
	keyPath := masterConfigInfos.AWS.KEYFILEPATH // /root/.aws/awspowerkimkeypair.pem

	//instanceIds := ec2handler.CreateInstances(svc, "ami-047f7b46bd6dd5d84", "t2.micro", 1, count,
	//   "aws.powerkim.keypair", "sg-2334584f", "subnet-8c4a53e4", "powerkimInstance_")
	instanceIds := ec2handler.CreateInstances(svc, imageId, instanceType, 1, count,
		keyName, securityGroupId, subnetid, instanceNamePrefix)

	publicIPs := make([]*string, len(instanceIds))

	// 1.2. get servers' public IP.
	// waiting for completion of new instance running.
	// after then, can get publicIP.
	for k, v := range instanceIds {
		// wait until running status
		ec2handler.WaitForRun(svc, *v)
		// get public IP
		publicIP, err := ec2handler.GetPublicIP(svc, *v)
		if err != nil {
			fmt.Println("Error", err)
			return nil, nil
		}
		fmt.Println("==============> " + publicIP)
		publicIPs[k] = &publicIP
	}

	// 1.3. insert MCISM Agent into Servers.
	// 1.4. execute Servers' Agent.
	for _, v := range publicIPs {
		for i := 0; ; i++ {
			err := insertAgent(*v, userName, keyPath)
			if i == 30 {
				os.Exit(3)
			}
			if err == nil {
				break
			}
			// need to load SSH Service on the VM
			time.Sleep(time.Second * 3)
		} // end of for
	} // end of for

	// 1.5. add server list into etcd.
	//addServersToEtcd("aws", instanceIds, publicIPs)

	return instanceIds, publicIPs
}

func createVmGcp(count int) ([]*string, []*string) {
	// ==> GCP-GCE

	/*
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

		userName := "byoungseob"
		keyPath := "/root/.gcp/gcppowerkimkeypair.pem"
	*/

	credentialFile := masterConfigInfos.GCP.CREDENTIALFILE
	svc := gcehandler.Connect(credentialFile)

	// 1.1. create Servers(VM).
	// some options are static for simple PoC.
	// These must be prepared before.
	region := masterConfigInfos.GCP.REGION
	zone := masterConfigInfos.GCP.ZONE
	projectID := masterConfigInfos.GCP.PROJECTID
	//prefix := masterConfigInfos.GCP.PREFIX
	imageURL := masterConfigInfos.GCP.IMAGEID
	machineType := masterConfigInfos.GCP.INSTANCETYPE
	subNetwork := masterConfigInfos.GCP.SUBNETID
	networkName := masterConfigInfos.GCP.NETWORKNAME
	serviceAccoutsMail := masterConfigInfos.GCP.SERVICEACCOUTSMAIL
	baseName := masterConfigInfos.GCP.INSTANCENAMEPREFIX

	userName := masterConfigInfos.GCP.USERNAME   // byoungseob
	keyPath := masterConfigInfos.GCP.KEYFILEPATH // /root/.gcp/gcppowerkimkeypair.pem

	instanceIds := gcehandler.CreateInstances(svc, region, zone, projectID, imageURL, machineType, 1, count,
		subNetwork, networkName, serviceAccoutsMail, baseName)

	for _, v := range instanceIds {
		fmt.Println("\tInstanceName: ", *v)
	}

	publicIPs := make([]*string, len(instanceIds))
	// 1.2. get servers' public IP.
	// waiting for completion of new instance running.
	// after then, can get publicIP.
	for k, v := range instanceIds {
		// wait until running status

		fmt.Println("===========> ", svc, zone, projectID, *v)
		gcehandler.WaitForRun(svc, zone, projectID, *v)

		// get public IP
		publicIP := gcehandler.GetPublicIP(svc, zone, projectID, *v)
		fmt.Println("==============> " + publicIP)
		publicIPs[k] = &publicIP
	}

	// 1.3. insert MCISM Agent into Servers.
	// 1.4. execute Servers' Agent.
	for _, v := range publicIPs {
		for i := 0; ; i++ {
			err := insertAgent(*v, userName, keyPath)
			if i == 30 {
				os.Exit(3)
			}
			if err == nil {
				break
			}
			// need to load SSH Service on the VM
			time.Sleep(time.Second * 3)
		} // end of for
	} // end of for

	// 1.5. add server list into etcd.
	//addServersToEtcd("gcp", instanceIds, publicIPs)

	return instanceIds, publicIPs
}

func createVmAzure(count int) ([]*string, []*string) {
	// ==> AZURE-Compute

	/*
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
			keyPath := "/root/.azure/azurepowerkimkeypair.pem"
			sshPublicKeyPath = "/root/.azure/azurepublickey.pem"
		)
	*/

	credentialFile := masterConfigInfos.AZURE.CREDENTIALFILE
	connInfo := azurehandler.Connect(credentialFile)

	// 1.1. create Servers(VM).
	// some options are static for simple PoC.
	// These must be prepared before.
	groupName := masterConfigInfos.AZURE.GROUPNAME
	location := masterConfigInfos.AZURE.LOCATION
	virtualNetworkName := masterConfigInfos.AZURE.VIRTUALNETWORKNAME
	subnet1Name := masterConfigInfos.AZURE.SUBNET1NAME
	subnet2Name := masterConfigInfos.AZURE.SUBNET2NAME
	nsgName := masterConfigInfos.AZURE.NETWORKSECURITYGROUPNAME
	//        ipName := masterConfigInfos.AZURE.IPNAME
	//        nicName := masterConfigInfos.AZURE.NICNAME

	baseName := masterConfigInfos.AZURE.BASENAME
	vmUserName := masterConfigInfos.AZURE.USERNAME
	vmPassword := masterConfigInfos.AZURE.PASSWORD
	KeyPath := masterConfigInfos.AZURE.KEYFILEPATH
	sshPublicKeyPath := masterConfigInfos.AZURE.PUBLICKEYFILEPATH

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

	instanceIds := azurehandler.CreateInstances(connInfo, groupName, location, baseName, nicInfo, imageInfo, vmInfo, count)

	for _, v := range instanceIds {
		fmt.Println("\tInstanceName: ", *v)
	}

	publicIPs := make([]*string, len(instanceIds))
	// 1.2. get servers' public IP.
	// waiting for completion of new instance running.
	// after then, can get publicIP.
	for i, _ := range instanceIds {
		ipName := baseName + "IP" + strconv.Itoa(i)

		// get public IP
		publicIP, err := azurehandler.GetPublicIP(connInfo, groupName, ipName)
		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("==============> " + *publicIP.PublicIPAddressPropertiesFormat.IPAddress)
		publicIPs[i] = publicIP.PublicIPAddressPropertiesFormat.IPAddress

		//          fmt.Printf("[PublicIP] %#v", publicIP);
		//            fmt.Printf("[PublicIP] %s", *publicIP.PublicIPAddressPropertiesFormat.IPAddress);
	}

	// 1.3. insert MCISM Agent into Servers.
	// 1.4. execute Servers' Agent.
	for _, v := range publicIPs {
		for i := 0; ; i++ {
			err := insertAgent(*v, vmUserName, KeyPath)
			if i == 30 {
				os.Exit(3)
			}
			if err == nil {
				break
			}
			// need to load SSH Service on the VM
			time.Sleep(time.Second * 3)
		} // end of for
	} // end of for

	// 1.5. add server list into etcd.
	//addServersToEtcd("azure", instanceIds, publicIPs)

	return instanceIds, publicIPs
}

func terminateMcis(mcisId string) {

	fmt.Println("[terminateMcis]" + mcisId)
	key := "/mcis/" + mcisId
	fmt.Println(key)
	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	vmList := getVmList(mcisId)

	// delete vms info
	for _, v := range vmList {
		terminateVm(mcisId, v)
	}

	//need to change status

}

func terminateVm(mcisId string, vmId string) {

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
	}

	fmt.Println("[terminateVm]" + vmId)
	key := "/mcis/" + mcisId + "/vm/" + vmId
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Cloud_id)
	fmt.Printf("%+v\n", content.Csp_vm_id)

	if strings.Compare(content.Cloud_id, "aws") == 0 {
		terminateVmAws(content.Csp_vm_id)
	} else if strings.Compare(content.Cloud_id, "gcp") == 0 {
		terminateVmGcp(content.Csp_vm_id)
	} else if strings.Compare(content.Cloud_id, "azure") == 0 {
		terminateVmAzure(content.Csp_vm_id)
	} else {
		fmt.Println("==============ERROR=no matched provider_id=================")
	}

}

func terminateVmAws(cspVmId string) {

	//idList := make([]*string, 1)
	//idList = append(idList, &cspVmId)
	idList := []*string{&cspVmId}
	fmt.Println("<terminateVmAws cspVmId : " + *idList[0] + ">" + strconv.Itoa(len(idList)))

	// (2) terminate AWS server
	//region := "ap-northeast-2"
	region := masterConfigInfos.AWS.REGION
	svc := ec2handler.Connect(region)
	//  destroy Servers(VMs).
	ec2handler.DestroyInstances(svc, idList)

}

func terminateVmGcp(cspVmId string) {

	idList := []*string{&cspVmId}
	fmt.Println("<terminateVmGcp cspVmId : " + *idList[0] + ">")

	// (2) terminate all GCP servers
	credentialFile := masterConfigInfos.GCP.CREDENTIALFILE
	svc := gcehandler.Connect(credentialFile)

	//  destroy Servers(VM).
	zone := masterConfigInfos.GCP.ZONE
	projectID := masterConfigInfos.GCP.PROJECTID
	gcehandler.DestroyInstances(svc, zone, projectID, idList)

}

func terminateVmAzure(cspVmId string) {

	idList := []*string{&cspVmId}
	fmt.Println("<terminateVmAzure cspVmId : " + *idList[0] + ">")

	// (2) terminate all AZURE servers
	credentialFile := masterConfigInfos.AZURE.CREDENTIALFILE
	connInfo := azurehandler.Connect(credentialFile)

	//  destroy Servers(VMs).
	groupName := masterConfigInfos.AZURE.GROUPNAME
	//    azurehandler.DestroyInstances(connInfo, groupName, idList)  @todo now, just delete target Group for convenience.
	azurehandler.DeleteGroup(connInfo, groupName)

}

// MCIS utilities

func genUuid() string {
	return uuid.New().String()
}

func insertAgent(serverIP string, userName string, keyPath string) error {

	// server connection info
	// some options are static for simple PoC.// some options are static for simple PoC.
	// These must be prepared before.
	//userName := "ec2-user"
	port := ":22"
	serverPort := serverIP + port

	//keyPath := "/root/.aws/awspowerkimkeypair.pem"
	//keyPath := masterConfigInfos.AWS.KEYFILEPATH

	// file info to copy
	//sourceFile := "/root/go/src/mcism/mcism_agent/mcism_agent"
	//sourceFile := "/root/go/src/github.com/cloud-barista/poc-mcism/mcism_agent/mcism_agent"
	homePath := os.Getenv("HOME")
	sourceFile := homePath + "/go/src/github.com/cloud-barista/poc-mcism/mcism_agent/mcism_agent"
	targetFile := "/tmp/mcism_agent"

	// command for ssh run
	cmd := "/tmp/mcism_agent &"

	// Connect to the server for scp
	scpCli, err := scp.Connect(userName, keyPath, serverPort)
	if err != nil {
		fmt.Println("Couldn't establisch a connection to the remote server ", err)
		return err
	}

	// copy agent into the server.
	if err := scp.Copy(scpCli, sourceFile, targetFile); err != nil {
		fmt.Println("Error while copying file ", err)
		return err
	}

	// close the session
	scp.Close(scpCli)

	// Connect to the server for ssh
	sshCli, err := sshrun.Connect(userName, keyPath, serverPort)
	if err != nil {
		fmt.Println("Couldn't establisch a connection to the remote server ", err)
		return err
	}

	if err := sshrun.RunCommand(sshCli, cmd); err != nil {
		fmt.Println("Error while running cmd: "+cmd, err)
		return err
	}

	sshrun.Close(sshCli)

	return err
}

func getVmIp(mcisId string, vmId string) string {

	var content struct {
		Public_ip string `json:"public_ip"`
	}

	fmt.Println("[getVmIp]" + vmId)
	key := "/mcis/" + mcisId + "/vm/" + vmId
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Public_ip)

	return content.Public_ip
}

func monitorVm(vmIpPort string) (string, string, string) {

	// Set up a connection to the server.
	conn, err := grpc.Dial(vmIpPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewResourceStatClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Hour)
	defer cancel()

	r, err := c.GetResourceStat(ctx, &pb.ResourceStatRequest{})
	if err != nil {
		log.Fatalf("could not Fetch Resource Status Information: %v", err)
	}
	println("[" + r.Servername + "]")
	log.Printf("%s", r.Cpu)
	log.Printf("%s", r.Mem)
	log.Printf("%s", r.Dsk)

	return r.Cpu, r.Mem, r.Dsk

}
