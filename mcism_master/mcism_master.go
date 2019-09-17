// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista

package main

import (
	"flag"

	"github.com/cloud-barista/poc-mcism/mcism_master/azurehandler"
	"github.com/cloud-barista/poc-mcism/mcism_master/confighandler"
	"github.com/cloud-barista/poc-mcism/mcism_master/ec2handler"
	"github.com/cloud-barista/poc-mcism/mcism_master/etcdhandler"
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

	pb "github.com/cloud-barista/poc-mcism/grpc_def"
	"google.golang.org/grpc"

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
type vmreq struct {
	CSP string `json:"csp"`
	IMG string `json:"img"`
}
type (
	svc struct {
		ID    int     `json:"id"`
		NAME  string  `json:"name"`
		VMREQ []vmreq `json:"vmreq"`
		VMNUM int     `json:"vmnum"`
		CSP   string  `json:"csp"`
		NUM   int     `json:"num"`
	}
)

var (
	svcs   = map[int]*svc{}
	seqSvc = 1
)

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

	Public_ip    string `json:"public_ip"`
	Domain_name  string `json:"domain_name"`
	Cloud_id     string `json:"cloud_id"`
	Location     string `json:"location"`
	Vmimage_name string `json:"vmimage_name"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	Disk_type   string `json:"disk_type"`

	Vmimage      string `json:"vmimage"`
	Vmspec       string `json:"vmspec"`
	Network      string `json:"network"`
	Subnet       string `json:"subnet"`
	Net_security string `json:"net_security"`

	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
}

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
}

const (
	defaultName        = ""
	defaultMonitorPort = ":2019"
)

var masterConfigInfos confighandler.MASTERCONFIGTYPE

var etcdServerPort *string
var fetchType *string

var addServer *string
var delServer *string

var addVMNumAWS *int
var delServerNumAWS *int

var addVMNumGCP *int
var delServerNumGCP *int

var addVMNumAZURE *int

var listvm *bool
var monitoring *bool
var delVMAWS *bool
var delVMGCP *bool
var delVMAZURE *bool

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

	e.Logger.Fatal(e.Start(":1323"))

}

func main() {

	fmt.Println("[ PoC-MCISM (Multi-Cloud Infra Service Management Framework) ]")
	fmt.Println("// Initiating API Server ...")

	fmt.Println("Ex) curl <ServerIP>:1323/mcis | json_pp")
	fmt.Println("Ex) curl -X POST <ServerIP>:1323/mcis \n -H 'Content-Type: application/json' \n -d '{\"name\":\"mcis-3-t002\",\"vmnum\":3,\"vmreq\":[{\"csp\":\"aws\",\"img\":\"aws-ec2-ubuntu-image\"},{\"csp\":\"azure\",\"img\":\"azure-ubuntu-image\"},{\"csp\":\"gcp\",\"img\":\"gcp-ubuntu-image\"}]}' | json_pp")
	fmt.Println("Ex) curl <ServerIP>:1323/mcis/<McisID> | json_pp")
	fmt.Println("Ex) curl -X DELETE <ServerIP>:1323/mcis | json_pp")

	// load config
	// you can see the details of masterConfigInfos
	// at confighander/confighandler.go:MASTERCONFIGTYPE.
	masterConfigInfos = confighandler.GetMasterConfigInfos()

	// dedicated option for PoC
	// 1. parsing user's request.
	//parseRequest()

	// Get interactive command request
	//getInteractiveRequest()
	// Run API Server
	apiServer()

	/*
		//<add servers in AWS/GCP/AZURE>
		// 1.1. create Servers(VM).
		if *addVMNumAWS != 0 {
			fmt.Println("######### addVMaws....")
			addVMaws(*addVMNumAWS)
		}
		if *addVMNumGCP != 0 {
			fmt.Println("######### addVMgcp....")
			addVMgcp(*addVMNumGCP)
		}
		if *addVMNumAZURE != 0 {
			fmt.Println("######### addVMazure....")
			addVMazure(*addVMNumAZURE)
		}

		//<get all server list>
		if *listvm != false {
			//fmt.Println("######### list of all servers....")
			serverList()
		}
		// 2.2. fetch all agent's monitoring info.
		if *monitoring != false {
			fmt.Println("######### monitoring all servers....")
			monitoringAll()
		}

		//<delete all servers inAWS/GCP/AZURE>
		if *delVMAWS != false {
			fmt.Println("######### delete all servers in AWS....")
			delAllVMaws()
		}
		if *delVMGCP != false {
			fmt.Println("######### delete all servers in GCP....")
			delAllVMgcp()
		}
		if *delVMAZURE != false {
			fmt.Println("######### delete all servers in AZURE....")
			delAllVMazure()
		}
	*/

}

// MCIS API Proxy
func restPostMcis(c echo.Context) error {
	u := &svc{
		ID: int(time.Now().UnixNano() / 1e6),
	}
	if err := c.Bind(u); err != nil {
		return err
	}
	svcs[u.ID] = u
	//fmt.Print("VMREQ: "+u.VMREQ)
	//fmt.Print("VMNUM: "+u.VMNUM)

	vmRequest := u.VMREQ

	// cb-store
	fmt.Println("=========================== Put createSvc")
	Key := "/mcis/" + strconv.Itoa(u.ID)
	mapA := map[string]string{"name": u.NAME, "description": "the description", "status": "launching", "vm_num": "TBD", "placement_algo": "TBD"}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	for _, k := range vmRequest {
		fmt.Print(k.CSP)

		if k.CSP == "aws" {
			fmt.Println("######### addVMaws....")
			instanceIds, publicIPs := launchVmAws(1)

			for i := 0; i < len(instanceIds) && i < len(publicIPs); i++ {
				fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" + string(*publicIPs[i]))
				addVmToMcis(strconv.Itoa(u.ID), u.NAME, "aws", instanceIds, publicIPs)
			}

		}
		if k.CSP == "gcp" {
			fmt.Println("######### addVMgcp....")
			instanceIds, publicIPs := launchVmGcp(1)

			for i := 0; i < len(instanceIds) && i < len(publicIPs); i++ {
				fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" + string(*publicIPs[i]))
				addVmToMcis(strconv.Itoa(u.ID), u.NAME, "gcp", instanceIds, publicIPs)
			}

		}
		if k.CSP == "azure" {
			fmt.Println("######### addVMazure....")
			instanceIds, publicIPs := launchVmAzure(1)

			for i := 0; i < len(instanceIds) && i < len(publicIPs); i++ {
				fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" + string(*publicIPs[i]))
				addVmToMcis(strconv.Itoa(u.ID), u.NAME, "azure", instanceIds, publicIPs)
			}
		}
	}
	/*
			for _, v := range instanceIds {
			//vs := strings.Split(string(*v), "/")

			fmt.Println("[instanceIds=] " + v )
		}
	*/

	seqSvc++
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
		fmt.Println("[monitor MCIS]")

		vmList := getVmList(mcisId)

		for _, v := range vmList {
			vmIp := getVmIp(mcisId, v)
			vmIpPort := vmIp + defaultMonitorPort

			statusCpu, statusMem, statusDisk := monitorVm(vmIpPort)
			fmt.Println("[Status for MCIS] VM:" + vmIpPort + " CPU:" + statusCpu + " MEM:" + statusMem + " DISK:" + statusDisk)
		}

		mapA := map[string]string{"message": "The MCIS monitoring"}
		return c.JSON(http.StatusOK, &mapA)

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
	u := new(svc)
	if err := c.Bind(u); err != nil {
		return err
	}
	id, _ := strconv.Atoi(c.Param("id"))
	svcs[id].NAME = u.NAME
	return c.JSON(http.StatusOK, svcs[id])
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

func addVmToMcis(svcId string, svcName string, provider string, instanceIds []*string, serverIPs []*string) {

	/*
		etcdcli, err := etcdhandler.Connect(etcdServerPort)
		if err != nil {
			panic(err)
		}

		defer etcdhandler.Close(etcdcli)

	*/
	//ctx := context.Background()
	//ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	for i, v := range serverIPs {
		serverPort := *v + ":2019" // 2019 Port is dedicated value for PoC.
		fmt.Println("######### addServer...." + serverPort)
		// /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
		//etcdhandler.AddServerToService(ctx, etcdcli, &svcId, &svcName, &provider, instanceIds[i], &serverPort, fetchType)

		// cb-store
		vmId := strconv.Itoa(int(time.Now().UnixNano() / 1e6))

		fmt.Println("=========================== Put(...)" + *instanceIds[i])
		Key := "/mcis/" + svcId + "/vm/" + vmId
		//svcName + "/server/"+ provider + "/" + *instanceId + "/" + addserver
		mapA := map[string]string{"csp_vm_id": *instanceIds[i], "name": "vmName", "description": "the description", "status": "running", "public_ip": *v, "domain_name": "TBD", "cloud_id": provider, "location": "TBD", "placement_algo": "TBD", "vmimage_name": "TBD", "vcpu_size": "TBD", "memory_size": "TBD", "disk_size": "TBD", "disk_type": "TBD", "vmimage": "TBD", "vmspec": "TBD", "network": "TBD", "subnet": "TBD", "net_security": "TBD"}
		Val, _ := json.Marshal(mapA)
		err := store.Put(string(Key), string(Val))
		if err != nil {
			cblog.Error(err)
		}
		fmt.Println("===========================")
		vmkeyValue, _ := store.Get(string(Key))
		fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
		fmt.Println("===========================")

	}

}

func getMcisList() []string {

	// cb-store
	fmt.Println("[Get MCISs")
	key := "/mcis"
	fmt.Println(key)

	/*
		keyValue, _ := store.GetList(key, true)
		//mcisList := make([]icbs.KeyValue, len(keyValue))
		var mcisList []icbs.KeyValue

		for _, v := range keyValue {
			if !strings.Contains(v.Key, "vm") {
				mcisList = append(mcisList, *v)
				fmt.Println("<" + v.Key + "> \n")
			}
		}
		for _, v := range mcisList {
			fmt.Println("<" + v.Key + "> \n" + v.Value)
		}
		fmt.Println("===============================================")
	*/

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

	//json.Unmarshal([]byte(keyValue.Value), &content)
	//fmt.Printf("%+v\n", content)

	//return etcdhandler.ServiceList(ctx, etcdcli)
	/*
		etcdcli, err := etcdhandler.Connect(etcdServerPort)
		if err != nil {
			panic(err)
		}

		defer etcdhandler.Close(etcdcli)

		//ctx := context.Background()
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

		return etcdhandler.ServiceList(ctx, etcdcli)
	*/
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

func launchVmAws(count int) ([]*string, []*string) {

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
			err := copyAndPlayAgent(*v, userName, keyPath)
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

func launchVmGcp(count int) ([]*string, []*string) {
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
			err := copyAndPlayAgent(*v, userName, keyPath)
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

func launchVmAzure(count int) ([]*string, []*string) {
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
			err := copyAndPlayAgent(*v, vmUserName, KeyPath)
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

func copyAndPlayAgent(serverIP string, userName string, keyPath string) error {

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

// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.
// Appendix. Will be deplicated soon.

func parseRequest() {

	etcdServerPort = &masterConfigInfos.ETCDSERVERPORT

	//etcdServerPort = flag.String("etcdserver", "129.254.175.43:2379", "etcdserver=129.254.175.43:2379")
	fetchType = flag.String("fetchtype", "PULL", "fetch type: -fetchtype=PUSH")
	/*
		addServer = flag.String("addserver", "none", "add a server: -addserver=192.168.0.10:5000")
		delServer = flag.String("delserver", "none", "delete a server: -delserver=192.168.0.10")
	*/
	addVMNumAWS = flag.Int("addvm-aws", 0, "add servers in AWS: -addvm-aws=10")
	delVMAWS = flag.Bool("delvm-aws", false, "delete all servers in AWS: -delvm-aws")

	addVMNumGCP = flag.Int("addvm-gcp", 0, "add servers in GCP: -addvm-gcp=10")
	delVMGCP = flag.Bool("delvm-gcp", false, "delete all servers in GCP: -delvm-gcp")

	addVMNumAZURE = flag.Int("addvm-azure", 0, "add servers in AZURE: -addvm-azure=10")
	delVMAZURE = flag.Bool("delvm-azure", false, "delete all servers in AZURE: -delvm-azure")

	listvm = flag.Bool("listvm", false, "report server list: -listvm")
	monitoring = flag.Bool("monitor", false, "report all server' resources status: -monitor")

	flag.Parse()
}

func getInteractiveRequest() {

	command := -1
	fmt.Println("[Select opt (0:API-server, 1:create-vm, 2:delete-vm, 3:list-vm, 4:monitor-vm]")
	fmt.Print("Your section : ")

	fmt.Scanln(&command)
	fmt.Println(command)

	switch {
	case command == 0:
		apiServer()
	case command == 1:
		selCsp := 0
		fmt.Println("[Select cloud service provider (1:aws, 2:gcp, 3:azure, 4:TBD]")
		fmt.Print("Your section : ")
		fmt.Scanln(&selCsp)

		selVmNum := 1
		fmt.Println("[Provide the number of VM to create (e.g., 5)")
		fmt.Print("Your section : ")
		fmt.Scanln(&selVmNum)

		switch {
		case selCsp == 0:
			fmt.Println("nothing was selected")
		case selCsp == 1:
			fmt.Println("Create VM(s) in aws")
			*addVMNumAWS = selVmNum
		case selCsp == 2:
			fmt.Println("Create VM(s) in gcp")
			*addVMNumGCP = selVmNum
		case selCsp == 3:
			fmt.Println("Create VM(s) in azure")
			*addVMNumAZURE = selVmNum
		case selCsp == 4:
			fmt.Println("not implemented yet. will be provided soon")
		default:
			fmt.Println("select within 1-4")
		}
	case command == 2:
		selCsp := -1
		fmt.Println("[Select cloud service provider (0: all, 1:aws, 2:gcp, 3:azure, 4:TBD]")
		fmt.Print("Your section : ")
		fmt.Scanln(&selCsp)

		switch {
		case selCsp == -1:
			fmt.Println("nothing was selected")
		case selCsp == 0:
			fmt.Println("Delete all VMs for all CPSs")
			*delVMAWS = true
			*delVMGCP = true
			*delVMAZURE = true
		case selCsp == 1:
			fmt.Println("Delete all VMs in aws")
			*delVMAWS = true
		case selCsp == 2:
			fmt.Println("Delete all VMs in gcp")
			*delVMGCP = true
		case selCsp == 3:
			fmt.Println("Delete all VMs in azure")
			*delVMAZURE = true
		case selCsp == 4:
			fmt.Println("not implemented yet. will be provided soon")
		default:
			fmt.Println("select within 1-4")
		}
	case command == 3:
		*listvm = true
	case command == 4:
		*monitoring = true
	default:
		fmt.Println("select within 1-4")
	}
}

func delProviderAllServersFromEtcd(provider string) {
	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	ctx := context.Background()

	fmt.Println("######### delete " + provider + " all Server....")
	etcdhandler.DelProviderAllServers(ctx, etcdcli, &provider)
}

func delAllVMaws() {

	// (1) get all AWS server id list from etcd
	idList := getInstanceIdListAWS()

	// (2) terminate all AWS servers
	//region := "ap-northeast-2"
	region := masterConfigInfos.AWS.REGION

	svc := ec2handler.Connect(region)

	//  destroy Servers(VMs).
	ec2handler.DestroyInstances(svc, idList)

	// (3) remove all aws server list from etcd
	delProviderAllServersFromEtcd(string("aws"))
}

// (1) get all GCP server id list from etcd
// (2) terminate all GCP servers
// (3) remove server list from etcd
func delAllVMgcp() {

	// (1) get all GCP server id list from etcd
	idList := getInstanceIdListGCP()

	// (2) terminate all GCP servers
	credentialFile := masterConfigInfos.GCP.CREDENTIALFILE
	svc := gcehandler.Connect(credentialFile)

	//  destroy all Servers(VMs).
	zone := masterConfigInfos.GCP.ZONE
	projectID := masterConfigInfos.GCP.PROJECTID
	gcehandler.DestroyInstances(svc, zone, projectID, idList)

	// (3) remove all aws server list from etcd
	delProviderAllServersFromEtcd(string("gcp"))
}

// (1) get all AZURE server id list from etcd
// (2) terminate all AZURE servers
// (3) remove server list from etcd
func delAllVMazure() {

	// (1) get all AZURE server id list from etcd
	// idList := getInstanceIdListAZURE()

	// (2) terminate all AZURE servers
	credentialFile := masterConfigInfos.AZURE.CREDENTIALFILE
	connInfo := azurehandler.Connect(credentialFile)

	//  destroy all Servers(VMs).
	groupName := masterConfigInfos.AZURE.GROUPNAME
	//    azurehandler.DestroyInstances(connInfo, groupName, idList)  @todo now, just delete target Group for convenience.
	azurehandler.DeleteGroup(connInfo, groupName)

	// (3) remove all aws server list from etcd
	delProviderAllServersFromEtcd(string("azure"))
}

func getInstanceIdListAWS() []*string {
	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	ctx := context.Background()
	return etcdhandler.InstanceIDListAWS(ctx, etcdcli)
}

func getInstanceIdListGCP() []*string {
	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	ctx := context.Background()
	return etcdhandler.InstanceIDListGCP(ctx, etcdcli)
}

func getInstanceIdListAZURE() []*string {
	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	ctx := context.Background()
	return etcdhandler.InstanceIDListAZURE(ctx, etcdcli)
}

func addServersToEtcd(provider string, instanceIds []*string, serverIPs []*string) {

	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	//ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	for i, v := range serverIPs {
		serverPort := *v + ":2019" // 2019 Port is dedicated value for PoC.
		fmt.Println("######### addServer...." + serverPort)
		// /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
		etcdhandler.AddServer(ctx, etcdcli, &provider, instanceIds[i], &serverPort, fetchType)
	}

}

func serverList() {

	list := getServerList()
	fmt.Print("######### all server list....(" + strconv.Itoa(len(list)) + ")\n")

	for _, v := range list {
		vs := strings.Split(string(*v), "/")
		fmt.Println("[CSP] " + vs[0] + "\t/ [VmID] " + vs[1] + "\t/ [IP] " + vs[2])
	}
}

func getServerList() []*string {

	etcdcli, err := etcdhandler.Connect(etcdServerPort)
	if err != nil {
		panic(err)
	}

	defer etcdhandler.Close(etcdcli)

	//ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	return etcdhandler.ServerList(ctx, etcdcli)
}

func monitoringAll() {

	for {
		list := getServerList()
		for _, v := range list {
			vs := strings.Split(string(*v), "/")
			println("-----monitoiring for------")
			fmt.Println("[CSP] " + vs[0] + "\t/ [VmID] " + vs[1] + "\t/ [IP] " + vs[2])

			monitorVm(vs[2])
			println("-----------")
		}
		println("==============================")
		time.Sleep(time.Second)
	} // end of for
}
