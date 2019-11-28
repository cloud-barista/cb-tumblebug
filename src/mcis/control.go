package mcis

import (
	"errors"

	//"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/scp"
	//"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/sshrun"

	"encoding/json"
	"fmt"
	"io/ioutil"
	//"log"
	"strconv"
	"strings"
	"time"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo"

	"sync"
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

const actionTerminate string = "Terminate"
const actionSuspend string = "Suspend"
const actionResume string = "Resume"
const actionReboot string = "Reboot"

const statusRunning string = "Running"
const statusSuspended string = "Suspended"
const statusFailed string = "Failed"
const statusTerminated string = "Terminated"
const statusCreating string = "Creating"
const statusSuspending string = "Suspending"
const statusResuming string = "Resuming"
const statusRebooting string = "Rebooting"
const statusTerminating string = "Terminating"

type KeyValue struct {
	Key   string
	Value string
}

// Structs for REST API

type mcisReq struct {
	Id             string  `json:"id"`
	Name           string  `json:"name"`
	Vm_req         []vmReq `json:"vm_req"`
	//Vm_num         string  `json:"vm_num"`
	Placement_algo string  `json:"placement_algo"`
	Description    string  `json:"description"`
}

type vmReq struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`

	// 1. Required by CB-Spider
	CspVmName string `json:"cspVmName"`

	CspImageName          string   `json:"cspImageName"`
	CspVirtualNetworkId   string   `json:"cspVirtualNetworkId"`
	CspNetworkInterfaceId string   `json:"cspNetworkInterfaceId"`
	CspPublicIPId         string   `json:"cspPublicIPId"`
	CspSecurityGroupIds   []string `json:"cspSecurityGroupIds"`
	CspSpecId             string   `json:"cspSpecId"`
	CspKeyPairName        string   `json:"cspKeyPairName"`

	CbImageId            string   `json:"cbImageId"`
	CbVirtualNetworkId   string   `json:"cbVirtualNetworkId"`
	CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
	CbPublicIPId         string   `json:"cbPublicIPId"`
	CbSecurityGroupIds   []string `json:"cbSecurityGroupIds"`
	CbSpecId             string   `json:"cbSpecId"`
	CbKeyPairId          string   `json:"cbKeyPairId"`

	VMUserId     string `json:"vmUserId"`
	VMUserPasswd string `json:"vmUserPasswd"`

	Name               string   `json:"name"`
	Config_name        string   `json:"config_name"`
	Spec_id            string   `json:"spec_id"`
	Image_id           string   `json:"image_id"`
	Vnet_id            string   `json:"vnet_id"`
	Vnic_id            string   `json:"vnic_id"`
	Public_ip_id       string   `json:"public_ip_id"`
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`
}

type placementKeyValue struct {
	Key   string
	Value string
}

type mcisInfo struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
	Vm             []vmOverview `json:"vm"`
}

type vmOverview struct {
	Id             string `json:"id"`
	Name               string   `json:"name"`
	Config_name        string   `json:"config_name"`
	Region       RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP     string     `json:"publicIP"`
	PublicDNS    string     `json:"publicDNS"`
	Status string `json:"status"`

}

type RegionInfo struct {
	Region string
	Zone   string
}

type vmInfo struct {
	Id             string `json:"id"`
	Name               string   `json:"name"`
	Config_name        string   `json:"config_name"`
	Spec_id            string   `json:"spec_id"`
	Image_id           string   `json:"image_id"`
	Vnet_id            string   `json:"vnet_id"`
	Vnic_id            string   `json:"vnic_id"`
	Public_ip_id       string   `json:"public_ip_id"`
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`

	VmUserId     string `json:"vmUserId"`
	VmUserPasswd string `json:"vmUserPasswd"`

	// 2. Provided by CB-Spider
	Region       RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP     string     `json:"publicIP"`
	PublicDNS    string     `json:"publicDNS"`
	PrivateIP    string     `json:"privateIP"`
	PrivateDNS   string     `json:"privateDNS"`
	VMBootDisk   string     `json:"vmBootDisk"` // ex) /dev/sda1
	VMBlockDisk  string     `json:"vmBlockDisk"`

	// 3. Required by CB-Tumblebug
	Status string `json:"status"`

	CspViewVmDetail vmCspViewInfo `json:"cspViewVmDetail"`
}


type vmCspViewInfo struct {
	Name      string    // AWS,
	Id        string    // AWS,
	StartTime time.Time // Timezone: based on cloud-barista server location.

	Region           RegionInfo // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	ImageId          string
	VMSpecId         string   // AWS, instance type or flavour, etc... ex) t2.micro or f1.micro
	VirtualNetworkId string   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIds []string // AWS, ex) sg-0b7452563e1121bb6

	NetworkInterfaceId string // ex) eth0
	PublicIP           string // ex) AWS, 13.125.43.21
	PublicDNS          string // ex) AWS, ec2-13-125-43-0.ap-northeast-2.compute.amazonaws.com
	PrivateIP          string // ex) AWS, ip-172-31-4-60.ap-northeast-2.compute.internal
	PrivateDNS         string // ex) AWS, 172.31.4.60

	KeyPairName  string // ex) AWS, powerkimKeyPair
	VMUserId     string // ex) user1
	VMUserPasswd string

	VMBootDisk  string // ex) /dev/sda1
	VMBlockDisk string // ex)

	KeyValueList []KeyValue
}


type mcisStatusInfo struct {
	Id     string         `json:"id"`
	Name   string         `json:"name"`
	//Vm_num string         `json:"vm_num"`
	Status string         `json:"status"`
	Vm     []vmStatusInfo `json:"vm"`
}

type vmStatusInfo struct {
	Id        string `json:"id"`
	Csp_vm_id string `json:"csp_vm_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Public_ip string `json:"public_ip"`
}

type mcisRecommendReq struct {
	Vm_req          []vmRecommendReq    `json:"vm_req"`
	Placement_algo  string              `json:"placement_algo"`
	Placement_param []placementKeyValue `json:"placement_param"`
	Max_result_num  string              `json:"max_result_num"`
}

type vmRecommendReq struct {
	Request_name   string `json:"request_name"`
	Max_result_num string `json:"max_result_num"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	//Disk_type   string `json:"disk_type"`

	Placement_algo  string              `json:"placement_algo"`
	Placement_param []placementKeyValue `json:"placement_param"`
}

type vmPriority struct {
	Priority string `json:"priority"`
	Vm_spec  mcir.SpecInfo `json:"vm_spec"`
}
type vmRecommendInfo struct {
	Vm_req          vmRecommendReq      `json:"vm_req"`
	Vm_priority     []vmPriority        `json:"vm_priority"`
	Placement_algo  string              `json:"placement_algo"`
	Placement_param []placementKeyValue `json:"placement_param"`
}

// MCIS API Proxy

func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	key := createMcis(nsId, req)
	mcisId := req.Id

	keyValue, _ := store.Get(key)

	var content struct {
		Id             string   `json:"id"`
		Name           string   `json:"name"`
		//Vm_num         string   `json:"vm_num"`
		Status         string   `json:"status"`
		Vm             []vmInfo `json:"vm"`
		Placement_algo string   `json:"placement_algo"`
		Description    string   `json:"description"`
	}

	json.Unmarshal([]byte(keyValue.Value), &content)

	vmList, err := getVmList(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return err
	}

	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
		//fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := vmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = v
		content.Vm = append(content.Vm, vmTmp)
	}

	mcisStatus, err := getMcisStatus(nsId, mcisId)
	content.Status = mcisStatus.Status

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

func RestGetMcis(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")
	fmt.Println("[Get MCIS requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend MCIS]")

		controlMcis(nsId, mcisId, actionSuspend)
		mapA := map[string]string{"message": "Suspending the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		controlMcis(nsId, mcisId, actionResume)

		mapA := map[string]string{"message": "Resuming the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		controlMcis(nsId, mcisId, actionReboot)

		mapA := map[string]string{"message": "Rebooting the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		//fmt.Println("len(vmList) %d ", len(vmList))
		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to terminate in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}

		for _, v := range vmList {
			controlVm(nsId, mcisId, v, actionTerminate)
		}

		mapA := map[string]string{"message": "Terminating the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status MCIS]")

		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to check in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}
		mcisStatusResponse, err := getMcisStatus(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		//fmt.Printf("%+v\n", mcisStatusResponse)
		common.PrintJsonPretty(mcisStatusResponse)

		return c.JSON(http.StatusOK, &mcisStatusResponse)

	} else {

		var content struct {
			Id             string   `json:"id"`
			Name           string   `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string   `json:"status"`
			Vm             []vmInfo `json:"vm"`
			Placement_algo string   `json:"placement_algo"`
			Description    string   `json:"description"`
		}

		fmt.Println("[Get MCIS for id]" + mcisId)
		key := common.GenMcisKey(nsId, mcisId, "")
		//fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		//fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)

		mcisStatus, err := getMcisStatus(nsId, mcisId)
		content.Status = mcisStatus.Status

		if err != nil {
			cblog.Error(err)
			return err
		}

		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		for _, v := range vmList {
			vmKey := common.GenMcisKey(nsId, mcisId, v)
			//fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + key}
				return c.JSON(http.StatusOK, &mapA)
			}
			//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := vmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v

			//get current vm status
			vmStatusInfoTmp, err := getVmStatus(nsId, mcisId, v)
			if err != nil {
				cblog.Error(err)
			}
			vmTmp.Status = vmStatusInfoTmp.Status

			content.Vm = append(content.Vm, vmTmp)
		}
		//fmt.Printf("%+v\n", content)
		common.PrintJsonPretty(content)
		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, &content)

	}
}

func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Mcis []mcisInfo `json:"mcis"`
	}

	mcisList := getMcisList(nsId)

	for _, v := range mcisList {

		key := common.GenMcisKey(nsId, v, "")
		//fmt.Println(key)
		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := mcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisId := v
		mcisTmp.Id = mcisId
		
		mcisStatus, err := getMcisStatus(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}
		mcisTmp.Status = mcisStatus.Status


		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		for _, v1 := range vmList {
			vmKey := common.GenMcisKey(nsId, mcisId, v1)
			//fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + key}
				return c.JSON(http.StatusOK, &mapA)
			}
			//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := vmOverview{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v1
			mcisTmp.Vm = append(mcisTmp.Vm, vmTmp)
		}


		content.Mcis = append(content.Mcis, mcisTmp)

	}
	//fmt.Printf("content %+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutMcis(c echo.Context) error {
	return nil
}

func RestDelMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	err := delMcis(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the MCIS"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the MCIS info"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllMcis(c echo.Context) error {
	nsId := c.Param("nsId")

	mcisList := getMcisList(nsId)

	if len(mcisList) == 0 {
		mapA := map[string]string{"message": "No MCIS to delete"}
		return c.JSON(http.StatusOK, &mapA)
	}

	for _, v := range mcisList {
		err := delMcis(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All MCISs"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All MCISs has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func RestPostMcisRecommand(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	var content struct {
		//Vm_req          []vmRecommendReq    `json:"vm_req"`
		Vm_recommend    []vmRecommendInfo   `json:"vm_recommend"`
		Placement_algo  string              `json:"placement_algo"`
		Placement_param []placementKeyValue `json:"placement_param"`
	}
	//content.Vm_req = req.Vm_req
	content.Placement_algo = req.Placement_algo
	content.Placement_param = req.Placement_param

	vmList := req.Vm_req

	for i, v := range vmList {
		vmTmp := vmRecommendInfo{}
		//vmTmp.Request_name = v.Request_name
		vmTmp.Vm_req = req.Vm_req[i]
		vmTmp.Placement_algo = v.Placement_algo
		vmTmp.Placement_param = v.Placement_param

		var err error
		vmTmp.Vm_priority, err = getRecommendList(nsId, v.Vcpu_size, v.Memory_size, v.Disk_size)

		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to recommend MCIS"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		content.Vm_recommend = append(content.Vm_recommend, vmTmp)
	}
	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

// VM API Proxy

func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &vmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	vmInfoData := vmInfo{}
	vmInfoData.Id = common.GenUuid()
	req.Id = vmInfoData.Id
	//vmInfoData.CspVmName = req.CspVmName

	//vmInfoData.Placement_algo = req.Placement_algo

	//vmInfoData.Location = req.Location
	//vmInfoData.Cloud_id = req.
	vmInfoData.Description = req.Description

	//vmInfoData.CspSpecId = req.CspSpecId

	//vmInfoData.Vcpu_size = req.Vcpu_size
	//vmInfoData.Memory_size = req.Memory_size
	//vmInfoData.Disk_size = req.Disk_size
	//vmInfoData.Disk_type = req.Disk_type

	//vmInfoData.CspImageName = req.CspImageName

	//vmInfoData.CspSecurityGroupIds = req.CspSecurityGroupIds
	//vmInfoData.CspVirtualNetworkId = "TBD"
	//vmInfoData.Subnet = "TBD"
	//vmInfoData.CspImageName = "TBD"
	//vmInfoData.CspSpecId = "TBD"

	//vmInfoData.PublicIP = "Not assigned yet"
	//vmInfoData.CspVmId = "Not assigned yet"
	//vmInfoData.PublicDNS = "Not assigned yet"
	vmInfoData.Status = "Creating"

	///////////
	/*
		Name              string `json:"name"`
		Config_name       string `json:"config_name"`
		Spec_id           string `json:"spec_id"`
		Image_id          string `json:"image_id"`
		Vnet_id           string `json:"vnet_id"`
		Vnic_id           string `json:"vnic_id"`
		Security_group_id string `json:"security_group_id"`
		Ssh_key_id        string `json:"ssh_key_id"`
		Description       string `json:"description"`
	*/

	vmInfoData.Name = req.Name
	vmInfoData.Config_name = req.Config_name
	vmInfoData.Spec_id = req.Spec_id
	vmInfoData.Image_id = req.Image_id
	vmInfoData.Vnet_id = req.Vnet_id
	vmInfoData.Vnic_id = req.Vnic_id
	vmInfoData.Public_ip_id = req.Public_ip_id
	vmInfoData.Security_group_ids = req.Security_group_ids
	vmInfoData.Ssh_key_id = req.Ssh_key_id
	vmInfoData.Description = req.Description

	vmInfoData.Config_name = req.Config_name

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	//createMcis(nsId, req)
	//err := addVmToMcis(nsId, mcisId, vmInfoData)
	err := addVmToMcis(&wg, nsId, mcisId, &vmInfoData)

	if err != nil {
		mapA := map[string]string{"message": "Cannot find " + common.GenMcisKey(nsId, mcisId, "")}
		return c.JSON(http.StatusOK, &mapA)
	}
	wg.Wait()

	vmStatus, err := getVmStatus(nsId, mcisId, vmInfoData.Id)
	vmInfoData.Status = vmStatus.Status


	return c.JSON(http.StatusCreated, vmInfoData)
}

func RestGetMcisVm(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")
	fmt.Println("[Get VM requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend VM]")

		controlVm(nsId, mcisId, vmId, actionSuspend)
		mapA := map[string]string{"message": "Suspending the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		controlVm(nsId, mcisId, vmId, actionResume)
		mapA := map[string]string{"message": "Resuming the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")

		controlVm(nsId, mcisId, vmId, actionReboot)
		mapA := map[string]string{"message": "Starting the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		controlVm(nsId, mcisId, vmId, actionTerminate)

		mapA := map[string]string{"message": "Terminating the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status VM]")

		vmKey := common.GenMcisKey(nsId, mcisId, vmId)
		//fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + vmKey}
			return c.JSON(http.StatusOK, &mapA)
		}

		vmStatusResponse, err := getVmStatus(nsId, mcisId, vmId)

		if err != nil {
			cblog.Error(err)
			return err
		}

		//fmt.Printf("%+v\n", vmStatusResponse)
		common.PrintJsonPretty(vmStatusResponse)

		return c.JSON(http.StatusOK, &vmStatusResponse)

	} else {

		fmt.Println("[Get MCIS-VM info for id]" + vmId)

		key := common.GenMcisKey(nsId, mcisId, "")
		//fmt.Println(key)

		vmKey := common.GenMcisKey(nsId, mcisId, vmId)
		//fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := vmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = vmId

		//get current vm status
		vmStatusInfoTmp, err := getVmStatus(nsId, mcisId, vmId)
		if err != nil {
			cblog.Error(err)
		}
		vmTmp.Status = vmStatusInfoTmp.Status


		//fmt.Printf("%+v\n", vmTmp)
		common.PrintJsonPretty(vmTmp)

		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, &vmTmp)

	}
}

func RestPutMcisVm(c echo.Context) error {
	return nil
}

func RestDelMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	err := delMcisVm(nsId, mcisId, vmId)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the VM info"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the VM info"}
	return c.JSON(http.StatusOK, &mapA)
}

// MCIS Information Managemenet

func addVmInfoToMcis(nsId string, mcisId string, vmInfoData vmInfo) {

	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := store.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")

}

func updateMcisInfo(nsId string, mcisInfoData mcisInfo) {
	key := common.GenMcisKey(nsId, mcisInfoData.Id, "")
	val, _ := json.Marshal(mcisInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := store.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

func updateVmInfo(nsId string, mcisId string, vmInfoData vmInfo) {
	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := store.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

func getMcisList(nsId string) []string {

	fmt.Println("[Get MCISs")
	key := "/ns/" + nsId + "/mcis"
	//fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mcisList = append(mcisList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/mcis/"))
		}
	}
	/*
	for _, v := range mcisList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	*/
	return mcisList

}

func getVmList(nsId string, mcisId string) ([]string, error) {

	fmt.Println("[getVmList]")
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, err := store.GetList(key, true)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	var vmList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vm/") {
			vmList = append(vmList, strings.TrimPrefix(v.Key, (key+"/vm/")))
		}
	}
	/*
	for _, v := range vmList {
		fmt.Println("<" + v + ">")
	}
	fmt.Println("===============================================")
	*/
	return vmList, nil

}

func delMcis(nsId string, mcisId string) error {

	fmt.Println("[Delete MCIS] " + mcisId)

	// controlMcis first
	err := controlMcis(nsId, mcisId, actionTerminate)
	if err != nil {
		cblog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := getVmList(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
		fmt.Println(vmKey)
		err := store.Delete(vmKey)
		if err != nil {
			cblog.Error(err)
			return err
		}
	}
	// delete mcis info
	err = store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}

func delMcisVm(nsId string, mcisId string, vmId string) error {

	fmt.Println("[Delete VM] " + vmId)

	// controlVm first
	err := controlVm(nsId, mcisId, vmId, actionTerminate)

	if err != nil {
		cblog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished

	// delete vms info
	key := common.GenMcisKey(nsId, mcisId, vmId)
	err = store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}

//// Info manage for MCIS recommandation
func getRecommendList(nsId string, cpuSize string, memSize string, diskSize string) ([]vmPriority, error) {

	fmt.Println("getRecommendList")
	
	var content struct {
		Id					string 
		Price        		string 
		ConnectionName     string  
	}
	//fmt.Println("[Get MCISs")
	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize
	fmt.Println(key)
	keyValue, err := store.GetList(key, true)
	if err != nil {
		cblog.Error(err)
		return []vmPriority{}, err
	}

	var vmPriorityList []vmPriority

	for cnt, v := range keyValue {
		fmt.Println("getRecommendList1: "+v.Key)
		err = json.Unmarshal([]byte(v.Value), &content)
		if err != nil {
			cblog.Error(err)
			return []vmPriority{}, err
		}

		content2 := mcir.SpecInfo{}
		key2 := common.GenResourceKey(nsId, "spec", content.Id)

		keyValue2, err := store.Get(key2)
		if err != nil {
			cblog.Error(err)
			return []vmPriority{}, err
		}
		json.Unmarshal([]byte(keyValue2.Value), &content2)
		content2.Id = content.Id

		vmPriorityTmp := vmPriority{}
		vmPriorityTmp.Priority = strconv.Itoa(cnt)
		vmPriorityTmp.Vm_spec = content2
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList, err

	//requires error handling

}



// MCIS Control

func createMcis(nsId string, req *mcisReq) string {

	req.Id = common.GenUuid()
	vmRequest := req.Vm_req

	fmt.Println("=========================== Put createSvc")
	key := common.GenMcisKey(nsId, req.Id, "")
	//mapA := map[string]string{"name": req.Name, "description": req.Description, "status": "launching", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
	mapA := map[string]string{"id": req.Id, "name": req.Name, "description": req.Description, "status": "CREATING"}
	val, _ := json.Marshal(mapA)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	//goroutin
	var wg sync.WaitGroup
	wg.Add(len(vmRequest))

	for _, k := range vmRequest {

		//vmInfoData vmInfo
		vmInfoData := vmInfo{}
		vmInfoData.Id = common.GenUuid()
		//vmInfoData.CspVmName = k.CspVmName

		//vmInfoData.Placement_algo = k.Placement_algo

		//vmInfoData.Location = k.Location
		//vmInfoData.Cloud_id = k.Csp
		vmInfoData.Description = k.Description

		//vmInfoData.CspSpecId = k.CspSpecId

		//vmInfoData.Vcpu_size = k.Vcpu_size
		//vmInfoData.Memory_size = k.Memory_size
		//vmInfoData.Disk_size = k.Disk_size
		//vmInfoData.Disk_type = k.Disk_type

		//vmInfoData.CspImageName = k.CspImageName

		//vmInfoData.CspSecurityGroupIds = ["TBD"]
		//vmInfoData.CspVirtualNetworkId = "TBD"
		//vmInfoData.Subnet = "TBD"

		vmInfoData.PublicIP = "Not assigned yet"
		//vmInfoData.CspVmId = "Not assigned yet"
		vmInfoData.PublicDNS = "Not assigned yet"
		vmInfoData.Status = "Creating"

		///////////
		/*
			Name              string `json:"name"`
			Config_name       string `json:"config_name"`
			Spec_id           string `json:"spec_id"`
			Image_id          string `json:"image_id"`
			Vnet_id           string `json:"vnet_id"`
			Vnic_id           string `json:"vnic_id"`
			Security_group_id string `json:"security_group_id"`
			Ssh_key_id        string `json:"ssh_key_id"`
			Description       string `json:"description"`
		*/

		vmInfoData.Name = k.Name
		vmInfoData.Config_name = k.Config_name
		vmInfoData.Spec_id = k.Spec_id
		vmInfoData.Image_id = k.Image_id
		vmInfoData.Vnet_id = k.Vnet_id
		vmInfoData.Vnic_id = k.Vnic_id
		vmInfoData.Public_ip_id = k.Public_ip_id
		vmInfoData.Security_group_ids = k.Security_group_ids
		vmInfoData.Ssh_key_id = k.Ssh_key_id
		vmInfoData.Description = k.Description

		vmInfoData.Config_name = k.Config_name

		/////////

		go addVmToMcis(&wg, nsId, req.Id, &vmInfoData)
		//addVmToMcis(nsId, req.Id, vmInfoData)
	}
	wg.Wait()


	return key
}

func addVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *vmInfo) error {
	fmt.Printf("\n[addVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, _ := store.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	addVmInfoToMcis(nsId, mcisId, *vmInfoData)
	fmt.Printf("\n[vmInfoData]\n %+v\n", vmInfoData)

	//instanceIds, publicIPs := createVm(&vmInfoData)
	err := createVm(nsId, mcisId, vmInfoData)
	if err != nil {
		cblog.Error(err)
		return err
	}

	//vmInfoData.PublicIP = string(*publicIPs[0])
	//vmInfoData.CspVmId = string(*instanceIds[0])
	vmInfoData.Status = "Running"
	updateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

func createVm(nsId string, mcisId string, vmInfoData *vmInfo) error {

	fmt.Printf("\n\n[createVm(vmInfoData *vmInfo)]\n\n")

	//prettyJSON, err := json.MarshalIndent(vmInfoData, "", "    ")
	//if err != nil {
	//log.Fatal("Failed to generate json")
	//}
	//fmt.Printf("%s\n", string(prettyJSON))

	//common.PrintJsonPretty(vmInfoData)

	//fmt.Printf("%+v\n", vmInfoData.CspVmId)

	/*
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
	*/

	/* FYI
			type vmReq struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`

		// 1. Required by CB-Spider
		CspVmName string `json:"cspVmName"`

		CspImageName          string   `json:"cspImageName"`
		CspVirtualNetworkId   string   `json:"cspVirtualNetworkId"`
		CspNetworkInterfaceId string   `json:"cspNetworkInterfaceId"`
		CspPublicIPId         string   `json:"cspPublicIPId"`
		CspSecurityGroupIds   []string `json:"cspSecurityGroupIds"`
		CspSpecId             string   `json:"cspSpecId"`
		CspKeyPairName        string   `json:"cspKeyPairName"`

		CbImageId            string   `json:"cbImageId"`
		CbVirtualNetworkId   string   `json:"cbVirtualNetworkId"`
		CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
		CbPublicIPId         string   `json:"cbPublicIPId"`
		CbSecurityGroupIds   []string `json:"cbSecurityGroupIds"`
		CbSpecId             string   `json:"cbSpecId"`
		CbKeyPairId          string   `json:"cbKeyPairId"`

		VMUserId     string `json:"vmUserId"`
		VMUserPasswd string `json:"vmUserPasswd"`

		// 2. Required by CB-Tumblebug
		Location string `json:"location"`

		Vcpu_size   string `json:"vcpu_size"`
		Memory_size string `json:"memory_size"`
		Disk_size   string `json:"disk_size"`
		Disk_type   string `json:"disk_type"`

		Placement_algo string `json:"placement_algo"`
		//Description    string `json:"description"`

	Name               string   `json:"name"`
	Config_name        string   `json:"config_name"`
	Spec_id            string   `json:"spec_id"`
	Image_id           string   `json:"image_id"`
	Vnet_id            string   `json:"vnet_id"`
	Vnic_id            string   `json:"vnic_id"`
	Public_ip_id       string   `json:"public_ip_id"`
	Security_group_ids []string `json:"security_group_id"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	}
	*/
	


	url := SPIDER_URL + "/vm?connection_name=" + vmInfoData.Config_name

	method := "POST"

	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("url: " + url + " method: " + method)


	//payload := strings.NewReader("{ \"Name\": \"" + u.CspSshKeyName + "\"}")

	/* Mark 1
	type VNicReqInfo struct {
		Name             string
		VNetName         string
		SecurityGroupIds []string
		PublicIPid       string
	}
	tempReq := VNicReqInfo{}
	tempReq.Name = u.CspVNicName
	tempReq.VNetName = u.CspVNetName
	//tempReq.SecurityGroupIds =
	tempReq.PublicIPid = u.PublicIpId
	*/

	/* Mark 2
	tempReq := map[string]string{
		"Name":     u.CspVNicName,
		"VNetName": u.CspVNetName,
		//"SecurityGroupIds":    content.Fingerprint,
		"PublicIPid": u.PublicIpId}
	*/

	// Mark 3
	type VMReqInfo struct {
		VMName string

		ImageId            string
		VirtualNetworkId   string
		NetworkInterfaceId string
		PublicIPId         string
		SecurityGroupIds   []string

		VMSpecId string

		KeyPairName  string
		VMUserId     string
		VMUserPasswd string
	}

	/* VM creation requtest with csp resource ids
	tempReq := VMReqInfo{}
	tempReq.VMName = vmInfoData.Name

	tempReq.ImageId = vmInfoData.Image_id
	tempReq.VirtualNetworkId = vmInfoData.Vnet_id
	tempReq.NetworkInterfaceId = vmInfoData.Vnic_id
	tempReq.PublicIPId = vmInfoData.Public_ip_id
	tempReq.SecurityGroupIds = vmInfoData.Security_group_ids

	tempReq.VMSpecId = vmInfoData.Spec_id

	tempReq.KeyPairName = vmInfoData.Ssh_key_id

	tempReq.VMUserId = vmInfoData.Vm_access_id
	tempReq.VMUserPasswd = vmInfoData.Vm_access_passwd
	*/

	tempReq := VMReqInfo{}
	tempReq.VMName = vmInfoData.Name

	tempReq.ImageId = common.GetCspResourceId(nsId, "image", vmInfoData.Image_id)
	tempReq.VirtualNetworkId = common.GetCspResourceId(nsId, "network", vmInfoData.Vnet_id)
	tempReq.NetworkInterfaceId = "" //common.GetCspResourceId(nsId, "vNic", vmInfoData.Vnic_id)
	tempReq.PublicIPId = common.GetCspResourceId(nsId, "publicIp", vmInfoData.Public_ip_id)

	var SecurityGroupIdsTmp []string
	for _, v := range vmInfoData.Security_group_ids {
		SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, common.GetCspResourceId(nsId, "securityGroup", v))
	}
	tempReq.SecurityGroupIds = SecurityGroupIdsTmp

	tempReq.VMSpecId = common.GetCspResourceId(nsId, "spec", vmInfoData.Spec_id)

	tempReq.KeyPairName = common.GetCspResourceId(nsId, "sshKey", vmInfoData.Ssh_key_id)

	tempReq.VMUserId = vmInfoData.Vm_access_id
	tempReq.VMUserPasswd = vmInfoData.Vm_access_passwd

	fmt.Printf("\n[Request body to CB-SPIDER for Creating VM]\n")
	common.PrintJsonPretty(tempReq)

	payload, _ := json.Marshal(tempReq)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
		cblog.Error(err)
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	//reqBody, _ := ioutil.ReadAll(req.Body)
	//fmt.Println(string(reqBody))

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		cblog.Error(err)
		return err
	}

	fmt.Println("Called cb-spider API.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	//fmt.Println(string(body))

	// jhseo 191016
	//var s = new(imageInfo)
	//s := imageInfo{}

	temp := vmCspViewInfo{}
	err2 := json.Unmarshal(body, &temp)

	if err2 != nil {
		fmt.Println("whoops:", err2)
		fmt.Println(err)
		cblog.Error(err)
		return err
	}

	fmt.Println("[Response from SPIDER]")
	common.PrintJsonPretty(temp)
	fmt.Println("[Calling SPIDER]END\n\n")

	/* FYI
	type vmInfo struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`

		// 1. Variables in vmReq
		CspVmName string `json:"cspVmName"`

		CspImageName          string   `json:"cspImageName"`
		CspVirtualNetworkId   string   `json:"cspVirtualNetworkId"`
		CspNetworkInterfaceId string   `json:"cspNetworkInterfaceId"`
		CspPublicIPId         string   `json:"cspPublicIPId"`
		CspSecurityGroupIds   []string `json:"cspSecurityGroupIds"`
		CspSpecId             string   `json:"cspSpecId"`
		CspKeyPairName        string   `json:"cspKeyPairName"`

		CbImageId          string   `json:"cbImageId"`
		CbVirtualNetworkId   string   `json:"cbVirtualNetworkId"`
		CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
		CbPublicIPId         string   `json:"cbPublicIPId"`
		CbSecurityGroupIds   []string `json:"cbSecurityGroupIds"`
		CbSpecId             string   `json:"cbSpecId"`
		CbKeyPairId        string   `json:"cbKeyPairId"`

		VMUserId     string `json:"vmUserId"`
		VMUserPasswd string `json:"vmUserPasswd"`

		Location string `json:"location"`

		Vcpu_size   string `json:"vcpu_size"`
		Memory_size string `json:"memory_size"`
		Disk_size   string `json:"disk_size"`
		Disk_type   string `json:"disk_type"`

		Placement_algo string `json:"placement_algo"`
		Description    string `json:"description"`

		// 2. Provided by CB-Spider
		CspVmId      string     `json:"cspVmId"`
		StartTime    time.Time  `json:"startTime"`
		Region       RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
		PublicIP     string     `json:"publicIP"`
		PublicDNS    string     `json:"publicDNS"`
		PrivateIP    string     `json:"privateIP"`
		PrivateDNS   string     `json:"privateDNS"`
		VMBootDisk   string     `json:"vmBootDisk"` // ex) /dev/sda1
		VMBlockDisk  string     `json:"vmBlockDisk"`
		KeyValueList []KeyValue `json:"keyValueList"`

		// 3. Required by CB-Tumblebug
		Status string `json:"status"`

		//Public_ip     string `json:"public_ip"`
		//Domain_name   string `json:"domain_name"`
		Cloud_id string `json:"cloud_id"`
	}
	*/

	//content := vmInfo{}
	//content.Id = genUuid()
	//vmInfoData.Config_name = vmInfoData.Config_name

	// 1. Variables in vmReq
	/*
	vmInfoData.CspVmName = temp.Name // = u.CspVmName

	vmInfoData.CspImageName = temp.ImageId
	vmInfoData.CspVirtualNetworkId = temp.VirtualNetworkId
	vmInfoData.CspNetworkInterfaceId = temp.NetworkInterfaceId
	//vmInfoData.CspPublicIPId = temp..CspPublicIPId
	vmInfoData.CspSecurityGroupIds = temp.SecurityGroupIds
	vmInfoData.CspSpecId = temp.VMSpecId
	vmInfoData.CspKeyPairName = temp.KeyPairName

	vmInfoData.CbImageId = vmInfoData.Image_id
	vmInfoData.CbVirtualNetworkId = vmInfoData.Vnet_id
	vmInfoData.CbNetworkInterfaceId = vmInfoData.Vnic_id
	vmInfoData.CbPublicIPId = vmInfoData.Public_ip_id
	vmInfoData.CbSecurityGroupIds = vmInfoData.Security_group_ids
	vmInfoData.CbSpecId = vmInfoData.Spec_id
	vmInfoData.CbKeyPairId = vmInfoData.Ssh_key_id

	vmInfoData.Vm_access_id = temp.VMUserId
	vmInfoData.Vm_access_passwd = temp.VMUserPasswd

	*/
	
	vmInfoData.CspViewVmDetail = temp

	vmInfoData.Vm_access_id = temp.VMUserId
	vmInfoData.Vm_access_passwd = temp.VMUserPasswd

	//vmInfoData.Location = vmInfoData.Location

	//vmInfoData.Vcpu_size = vmInfoData.Vcpu_size
	//vmInfoData.Memory_size = vmInfoData.Memory_size
	//vmInfoData.Disk_size = vmInfoData.Disk_size
	//vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.Placement_algo = vmInfoData.Placement_algo
	vmInfoData.Description = vmInfoData.Description

	// 2. Provided by CB-Spider
	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = temp.Region
	vmInfoData.PublicIP = temp.PublicIP
	vmInfoData.PublicDNS = temp.PublicDNS
	vmInfoData.PrivateIP = temp.PrivateIP
	vmInfoData.PrivateDNS = temp.PrivateDNS
	vmInfoData.VMBootDisk = temp.VMBootDisk
	vmInfoData.VMBlockDisk = temp.VMBlockDisk
	//vmInfoData.KeyValueList = temp.KeyValueList

	//content.Status = temp.
	//content.Cloud_id = temp.

	// cb-store
	//fmt.Println("=========================== PUT createVM")
	/*
		Key := genResourceKey(nsId, "vm", content.Id)

		Val, _ := json.Marshal(content)
		fmt.Println("Key: ", Key)
		fmt.Println("Val: ", Val)
		cbStorePutErr := store.Put(string(Key), string(Val))
		if cbStorePutErr != nil {
			cblog.Error(cbStorePutErr)
			return nil, nil
		}
		keyValue, _ := store.Get(string(Key))
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===========================")
		return content, nil
	*/

	//instanceIds := make([]*string, 1)
	//publicIPs := make([]*string, 1)
	//instanceIds[0] = &content.CspVmId
	//publicIPs[0] = &content.PublicIP
	return nil
}

func controlMcis(nsId string, mcisId string, action string) error {

	fmt.Println("[controlMcis]" + mcisId + " to " + action)
	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	vmList, err := getVmList(nsId, mcisId)
	fmt.Println("=============================================== %#v", vmList)
	if err != nil {
		cblog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return nil
	}

	// delete vms info
	for _, v := range vmList {
		controlVm(nsId, mcisId, v, action)
	}
	return nil

	//need to change status

}

func controlVm(nsId string, mcisId string, vmId string, action string) error {

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
	}

	fmt.Println("[controlVm]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.Cloud_id)
	//fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := vmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}


	fmt.Println("\n\n[Calling SPIDER]START vmControl")

	fmt.Println("temp.CspVmId: " + temp.CspViewVmDetail.Id)
	
	/*
	cspType := getVMsCspType(nsId, mcisId, vmId)
	var cspVmId string
	if cspType == "AWS" {
		cspVmId = temp.CspViewVmDetail.Id
	} else {
		*/
	cspVmId := temp.CspViewVmDetail.Name
	common.PrintJsonPretty(temp.CspViewVmDetail)

	url := ""
	method := ""
	switch action {
	case actionTerminate:
		url = SPIDER_URL + "/vm/" + cspVmId + "?connection_name=" + temp.Config_name
		method = "DELETE"
	case actionReboot:
		url = SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=reboot"
		method = "GET"
	case actionSuspend:
		url = SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=suspend"
		method = "GET"
	case actionResume:
		url = SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=resume"
		method = "GET"
	default:
		return errors.New(action + "is invalid actionType")
	}
	fmt.Println("url: " + url + " method: " + method)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
		return err
	}

	res, err := client.Do(req)
	//fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	fmt.Println("[Calling SPIDER]END vmControl\n\n")
	/*
		if strings.Compare(content.Csp_vm_id, "Not assigned yet") == 0 {
			return nil
		}
		if strings.Compare(content.Cloud_id, "aws") == 0 {
			controlVmAws(content.Csp_vm_id)
		} else if strings.Compare(content.Cloud_id, "gcp") == 0 {
			controlVmGcp(content.Csp_vm_id)
		} else if strings.Compare(content.Cloud_id, "azure") == 0 {
			controlVmAzure(content.Csp_vm_id)
		} else {
			fmt.Println("==============ERROR=no matched provider_id=================")
		}
	*/

	return nil

}

func getMcisStatus(nsId string, mcisId string) (mcisStatusInfo, error) {

	fmt.Println("[getMcisStatus]" + mcisId)
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return mcisStatusInfo{}, err
	}

	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	mcisStatus := mcisStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisStatus)

	vmList, err := getVmList(nsId, mcisId)
	//fmt.Println("=============================================== %#v", vmList)
	if err != nil {
		cblog.Error(err)
		return mcisStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return mcisStatusInfo{}, nil
	}

	for _, v := range vmList {
		vmStatusTmp, err := getVmStatus(nsId, mcisId, v)
		if err != nil {
			cblog.Error(err)
			vmStatusTmp.Status = "FAILED"
			return mcisStatus, err
		}
		mcisStatus.Vm = append(mcisStatus.Vm, vmStatusTmp)
	}

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{statusFailed, statusSuspended, statusRunning, statusTerminated, statusCreating, statusSuspending, statusResuming, statusRebooting, statusTerminating, "Include-NotDefinedStatus"}
	for _, v := range mcisStatus.Vm {

		switch v.Status {
		case statusFailed:
			statusFlag[0]++
		case statusSuspended:
			statusFlag[1]++
		case statusRunning:
			statusFlag[2]++
		case statusTerminated:
			statusFlag[3]++
		case statusCreating:
			statusFlag[4]++
		case statusSuspending:
			statusFlag[5]++
		case statusResuming:
			statusFlag[6]++
		case statusRebooting:
			statusFlag[7]++
		case statusTerminating:
			statusFlag[8]++
		default:
			statusFlag[9]++
		}
	}

	tmpMax := 0
	tmpMaxIndex := 0
	for i, v := range statusFlag {
		if v > tmpMax {
			tmpMax = v
			tmpMaxIndex = i
		}
	}

	if tmpMax == len(mcisStatus.Vm) {
		mcisStatus.Status = statusFlagStr[tmpMaxIndex]
	} else if tmpMax < len(mcisStatus.Vm) {
		mcisStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex]
	} else {
		mcisStatus.Status = statusFlagStr[9]
	}
	if statusFlag[0] > 0 {
		mcisStatus.Status = statusFlagStr[0]
	}
	if statusFlag[9] > 0 {
		mcisStatus.Status = statusFlagStr[9]
	}

	return mcisStatus, nil

	//need to change status

}

func getVmStatus(nsId string, mcisId string, vmId string) (vmStatusInfo, error) {

	/*
	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
		CspVmId   string
		CspVmName string
		PublicIP  string
	}
	*/

	fmt.Println("[getVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	//json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.Cloud_id)
	//fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := vmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.Id)
	/*
	var cspVmId string
	cspType := getVMsCspType(nsId, mcisId, vmId)
	if cspType == "AWS" {
		cspVmId = temp.CspViewVmDetail.Id
	} else {
		*/
	cspVmId := temp.CspViewVmDetail.Name
	
	url := SPIDER_URL + "/vmstatus/" + cspVmId + "?connection_name=" + temp.Config_name 
	method := "GET"

	fmt.Println("url: " + url)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)

	errorInfo := vmStatusInfo{}
	errorInfo.Status = "FAILED"

	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}

	res, err := client.Do(req)
	//fmt.Println("Called CB-Spider API.")
	
	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	type statusResponse struct {
		Status string
	}
	statusResponseTmp := statusResponse{}

	

	err2 := json.Unmarshal(body, &statusResponseTmp)
	if err2 != nil {
		fmt.Println(err2)
		return errorInfo, err2
	}

	common.PrintJsonPretty(statusResponseTmp)
	fmt.Println("[Calling SPIDER]END\n\n")

	// Temporal CODE. This should be changed after CB-Spider fixes status types and strings/
	if statusResponseTmp.Status == "Creating" {
		statusResponseTmp.Status = statusCreating
	} else if statusResponseTmp.Status == "Running" {
		statusResponseTmp.Status = statusRunning
	} else if statusResponseTmp.Status == "Suspending" {
		statusResponseTmp.Status = statusSuspending
	} else if statusResponseTmp.Status == "Suspended" {
		statusResponseTmp.Status = statusSuspended
	} else if statusResponseTmp.Status == "Resuming" {
		statusResponseTmp.Status = statusResuming
	} else if statusResponseTmp.Status == "Rebooting" {
		statusResponseTmp.Status = statusRebooting
	} else if statusResponseTmp.Status == "Terminating" {
		statusResponseTmp.Status = statusTerminating
	} else if statusResponseTmp.Status == "Terminated" {
		statusResponseTmp.Status = statusTerminated
	} else {
		statusResponseTmp.Status = "statusUndefined"
	}

	// End of Temporal CODE.

	vmStatusTmp := vmStatusInfo{}
	vmStatusTmp.Id = vmId
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.Csp_vm_id = temp.CspViewVmDetail.Id
	vmStatusTmp.Public_ip = temp.PublicIP
	vmStatusTmp.Status = statusResponseTmp.Status
	if err != nil {
		cblog.Error(err)
		vmStatusTmp.Status = "FAILED"
	}

	return vmStatusTmp, nil

}

func getVmIp(nsId string, mcisId string, vmId string) string {

	var content struct {
		Public_ip string `json:"public_ip"`
	}

	fmt.Println("[getVmIp]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Public_ip)

	return content.Public_ip
}

/*
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
	//sourceFile := "/root/go/src/github.com/cloud-barista/cb-tumblebug/mcism_agent/mcism_agent"
	homePath := os.Getenv("HOME")
	sourceFile := homePath + "/go/src/github.com/cloud-barista/cb-tumblebug/mcism_agent/mcism_agent"
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
*/

/*
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
*/
