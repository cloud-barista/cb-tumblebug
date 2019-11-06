package mcis

import (
	"errors"

	//"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/scp"
	//"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/sshrun"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	// REST API (echo)
	"net/http"

	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/scp"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/sshrun"
	"github.com/labstack/echo"

	"sync"

	//"github.com/cloud-barista/cb-tumblebug/src/common"
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
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`
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

type RegionInfo struct {
	Region string
	Zone   string
}

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

	CbImageId            string   `json:"cbImageId"`
	CbVirtualNetworkId   string   `json:"cbVirtualNetworkId"`
	CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
	CbPublicIPId         string   `json:"cbPublicIPId"`
	CbSecurityGroupIds   []string `json:"cbSecurityGroupIds"`
	CbSpecId             string   `json:"cbSpecId"`
	CbKeyPairId          string   `json:"cbKeyPairId"`

	VMUserId     string `json:"vmUserId"`
	VMUserPasswd string `json:"vmUserPasswd"`

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
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`

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

type mcisStatusInfo struct {
	Id     string         `json:"id"`
	Name   string         `json:"name"`
	Vm_num string         `json:"vm_num"`
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
type vmPriority struct {
	Priority string `json:"priority"`
	Vm_spec  string `json:"vm_spec"`
}
type vmRecommendInfo struct {
	Name           string       `json:"name"`
	Vm_priority    []vmPriority `json:"vm_priority"`
	Placement_algo string       `json:"placement_algo"`
	Description    string       `json:"description"`
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
		Vm_num         string   `json:"vm_num"`
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
		vmKey := genMcisKey(nsId, mcisId, v)
		fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := vmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = v
		content.Vm = append(content.Vm, vmTmp)
	}
	fmt.Printf("%+v\n", content)

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
		mapA := map[string]string{"message": "The MCIS has been suspended"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		controlMcis(nsId, mcisId, actionResume)

		mapA := map[string]string{"message": "The MCIS has been resumed"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		controlMcis(nsId, mcisId, actionReboot)

		mapA := map[string]string{"message": "The MCIS has been rebooted"}
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

		mapA := map[string]string{"message": "The MCIS has been terminated"}
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

		fmt.Printf("%+v\n", mcisStatusResponse)

		return c.JSON(http.StatusOK, &mcisStatusResponse)

	} else {

		var content struct {
			Id             string   `json:"id"`
			Name           string   `json:"name"`
			Vm_num         string   `json:"vm_num"`
			Status         string   `json:"status"`
			Vm             []vmInfo `json:"vm"`
			Placement_algo string   `json:"placement_algo"`
			Description    string   `json:"description"`
		}

		fmt.Println("[Get MCIS for id]" + mcisId)
		key := genMcisKey(nsId, mcisId, "")
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)

		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		for _, v := range vmList {
			vmKey := genMcisKey(nsId, mcisId, v)
			fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + key}
				return c.JSON(http.StatusOK, &mapA)
			}
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

func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Mcis []mcisInfo `json:"mcis"`
	}

	mcisList := getMcisList(nsId)

	for _, v := range mcisList {

		key := genMcisKey(nsId, v, "")
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := mcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisTmp.Id = v
		content.Mcis = append(content.Mcis, mcisTmp)

	}
	fmt.Printf("content %+v\n", content)

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

	mapA := map[string]string{"message": "The MCIS has been deleted"}
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

	req := &mcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	var content struct {
		Vm_recommend   []vmRecommendInfo `json:"vm_recommend"`
		Placement_algo string            `json:"placement_algo"`
		Description    string            `json:"description"`
	}
	content.Placement_algo = req.Placement_algo
	content.Description = req.Description
	vmList := req.Vm_req

	for _, v := range vmList {
		vmTmp := vmRecommendInfo{}
		vmTmp.Placement_algo = v.Placement_algo
		vmTmp.Name = v.CspVmName

		vmTmp.Vm_priority = getRecommendList(nsId, v.Vcpu_size, v.Memory_size, v.Disk_size)

		content.Vm_recommend = append(content.Vm_recommend, vmTmp)
	}
	fmt.Printf("%+v\n", content)

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
	vmInfoData.Id = genUuid()
	req.Id = vmInfoData.Id
	vmInfoData.CspVmName = req.CspVmName

	vmInfoData.Placement_algo = req.Placement_algo

	vmInfoData.Location = req.Location
	//vmInfoData.Cloud_id = req.
	vmInfoData.Description = req.Description

	vmInfoData.CspSpecId = req.CspSpecId

	vmInfoData.Vcpu_size = req.Vcpu_size
	vmInfoData.Memory_size = req.Memory_size
	vmInfoData.Disk_size = req.Disk_size
	vmInfoData.Disk_type = req.Disk_type

	vmInfoData.CspImageName = req.CspImageName

	vmInfoData.CspSecurityGroupIds = req.CspSecurityGroupIds
	vmInfoData.CspVirtualNetworkId = "TBD"
	//vmInfoData.Subnet = "TBD"
	vmInfoData.CspImageName = "TBD"
	vmInfoData.CspSpecId = "TBD"

	vmInfoData.PublicIP = "Not assigned yet"
	vmInfoData.CspVmId = "Not assigned yet"
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

	vmInfoData.ConnectionName = req.Config_name

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	//createMcis(nsId, req)
	//err := addVmToMcis(nsId, mcisId, vmInfoData)
	err := addVmToMcis(&wg, nsId, mcisId, vmInfoData)

	if err != nil {
		mapA := map[string]string{"message": "Cannot find " + genMcisKey(nsId, mcisId, "")}
		return c.JSON(http.StatusOK, &mapA)
	}
	wg.Wait()

	return c.JSON(http.StatusCreated, req)
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
		mapA := map[string]string{"message": "The VM has been suspended"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		controlVm(nsId, mcisId, vmId, actionResume)
		mapA := map[string]string{"message": "The VM has been resumed"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")

		controlVm(nsId, mcisId, vmId, actionReboot)
		mapA := map[string]string{"message": "The VM has been restarted"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		controlVm(nsId, mcisId, vmId, actionTerminate)

		mapA := map[string]string{"message": "The VM has been terminated"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status VM]")

		vmKey := genMcisKey(nsId, mcisId, vmId)
		fmt.Println(vmKey)
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

		fmt.Printf("%+v\n", vmStatusResponse)

		return c.JSON(http.StatusOK, &vmStatusResponse)

	} else {

		fmt.Println("[Get MCIS for id]" + mcisId)
		key := genMcisKey(nsId, mcisId, "")
		fmt.Println(key)

		vmKey := genMcisKey(nsId, mcisId, vmId)
		fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := vmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = vmId

		fmt.Printf("%+v\n", vmTmp)

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
		mapA := map[string]string{"message": "Failed to delete the VM"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The VM has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

// MCIS Information Managemenet

func addVmInfoToMcis(nsId string, mcisId string, vmInfoData vmInfo) {

	key := genMcisKey(nsId, mcisId, vmInfoData.Id)
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

func updateVmInfo(nsId string, mcisId string, vmInfoData vmInfo) {
	key := genMcisKey(nsId, mcisId, vmInfoData.Id)
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

func getMcisList(nsId string) []string {

	fmt.Println("[Get MCISs")
	key := "/ns/" + nsId + "/mcis"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mcisList = append(mcisList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/mcis/"))
		}
	}
	for _, v := range mcisList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return mcisList

}

func getVmList(nsId string, mcisId string) ([]string, error) {

	fmt.Println("[getVmList]")
	key := genMcisKey(nsId, mcisId, "")
	fmt.Println(key)

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
	for _, v := range vmList {
		fmt.Println("<" + v + ">")
	}
	fmt.Println("===============================================")
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

	key := genMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := getVmList(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := genMcisKey(nsId, mcisId, v)
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
	key := genMcisKey(nsId, mcisId, vmId)
	err = store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}

//// Info manage for MCIS recommandation
func getRecommendList(nsId string, cpuSize string, memSize string, diskSize string) []vmPriority {

	//fmt.Println("[Get MCISs")
	key := genMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var vmPriorityList []vmPriority
	for cnt, v := range keyValue {
		vmPriorityTmp := vmPriority{}
		vmPriorityTmp.Priority = strconv.Itoa(cnt)
		vmPriorityTmp.Vm_spec = v.Key
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList

}

func RegisterRecommendList(nsId string, cpuSize string, memSize string, diskSize string, specId string, price string) error {

	//fmt.Println("[Get MCISs")
	key := genMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize + "/specId/" + specId
	fmt.Println(key)

	err := store.Put(string(key), string(price))
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("===============================================")
	return nil

}

// MCIS Control

func createMcis(nsId string, req *mcisReq) string {

	req.Id = genUuid()
	vmRequest := req.Vm_req

	fmt.Println("=========================== Put createSvc")
	key := genMcisKey(nsId, req.Id, "")
	//mapA := map[string]string{"name": req.Name, "description": req.Description, "status": "launching", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
	mapA := map[string]string{"id": req.Id, "name": req.Name, "description": req.Description, "status": "CREATING", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
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
		vmInfoData.Id = genUuid()
		vmInfoData.CspVmName = k.CspVmName

		vmInfoData.Placement_algo = k.Placement_algo

		vmInfoData.Location = k.Location
		//vmInfoData.Cloud_id = k.Csp
		vmInfoData.Description = k.Description

		vmInfoData.CspSpecId = k.CspSpecId

		vmInfoData.Vcpu_size = k.Vcpu_size
		vmInfoData.Memory_size = k.Memory_size
		vmInfoData.Disk_size = k.Disk_size
		vmInfoData.Disk_type = k.Disk_type

		vmInfoData.CspImageName = k.CspImageName

		//vmInfoData.CspSecurityGroupIds = ["TBD"]
		vmInfoData.CspVirtualNetworkId = "TBD"
		//vmInfoData.Subnet = "TBD"

		vmInfoData.PublicIP = "Not assigned yet"
		vmInfoData.CspVmId = "Not assigned yet"
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

		vmInfoData.ConnectionName = k.Config_name

		/////////

		go addVmToMcis(&wg, nsId, req.Id, vmInfoData)
		//addVmToMcis(nsId, req.Id, vmInfoData)
	}
	wg.Wait()

	return key
}

func addVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData vmInfo) error {
	fmt.Printf("\n[addVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := genMcisKey(nsId, mcisId, "")
	keyValue, _ := store.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	addVmInfoToMcis(nsId, mcisId, vmInfoData)
	fmt.Printf("\n[vmInfoData]\n %+v\n", vmInfoData)

	//instanceIds, publicIPs := createVm(&vmInfoData)
	err := createVm(nsId, mcisId, &vmInfoData)
	if err != nil {
		cblog.Error(err)
		return err
	}

	//vmInfoData.PublicIP = string(*publicIPs[0])
	//vmInfoData.CspVmId = string(*instanceIds[0])
	vmInfoData.Status = "Running"
	updateVmInfo(nsId, mcisId, vmInfoData)

	return nil

}

func createVm(nsId string, mcisId string, vmInfoData *vmInfo) error {

	fmt.Printf("\n\n[createVm(vmInfoData *vmInfo)]\n\n")

	prettyJSON, err := json.MarshalIndent(vmInfoData, "", "    ")
	if err != nil {
		log.Fatal("Failed to generate json", err)
	}
	fmt.Printf("%s\n", string(prettyJSON))

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

	url := SPIDER_URL + "/vm?connection_name=" + vmInfoData.ConnectionName

	method := "POST"

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

	tempReq.ImageId = getCspResourceId(nsId, "image", vmInfoData.Image_id)
	tempReq.VirtualNetworkId = getCspResourceId(nsId, "network", vmInfoData.Vnet_id)
	tempReq.NetworkInterfaceId = getCspResourceId(nsId, "vNic", vmInfoData.Vnic_id)
	tempReq.PublicIPId = getCspResourceId(nsId, "publicIp", vmInfoData.Public_ip_id)

	var SecurityGroupIdsTmp []string
	for _, v := range vmInfoData.Security_group_ids {
		SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, getCspResourceId(nsId, "securityGroup", v))
	}
	tempReq.SecurityGroupIds = SecurityGroupIdsTmp

	tempReq.VMSpecId = getCspResourceId(nsId, "spec", vmInfoData.Spec_id)

	tempReq.KeyPairName = getCspResourceId(nsId, "sshKey", vmInfoData.Ssh_key_id)

	tempReq.VMUserId = vmInfoData.Vm_access_id
	tempReq.VMUserPasswd = vmInfoData.Vm_access_passwd

	fmt.Printf("%+v\n", tempReq)

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

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		cblog.Error(err)
		return err
	}

	fmt.Println("Called cb-spider API.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	// jhseo 191016
	//var s = new(imageInfo)
	//s := imageInfo{}
	type VMInfo struct {
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
	temp := VMInfo{}
	err2 := json.Unmarshal(body, &temp)

	if err2 != nil {
		fmt.Println("whoops:", err2)
		fmt.Println(err)
		cblog.Error(err)
		return err
	}

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
	vmInfoData.ConnectionName = vmInfoData.ConnectionName

	// 1. Variables in vmReq
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

	//vmInfoData.Location = vmInfoData.Location

	vmInfoData.Vcpu_size = vmInfoData.Vcpu_size
	vmInfoData.Memory_size = vmInfoData.Memory_size
	vmInfoData.Disk_size = vmInfoData.Disk_size
	vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.Placement_algo = vmInfoData.Placement_algo
	vmInfoData.Description = vmInfoData.Description

	// 2. Provided by CB-Spider
	vmInfoData.CspVmId = temp.Id
	vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = temp.Region
	vmInfoData.PublicIP = temp.PublicIP
	vmInfoData.PublicDNS = temp.PublicDNS
	vmInfoData.PrivateIP = temp.PrivateIP
	vmInfoData.PrivateDNS = temp.PrivateDNS
	vmInfoData.VMBootDisk = temp.VMBootDisk
	vmInfoData.VMBlockDisk = temp.VMBlockDisk
	vmInfoData.KeyValueList = temp.KeyValueList

	//content.Status = temp.
	//content.Cloud_id = temp.

	// cb-store
	fmt.Println("=========================== PUT createVM")
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
	key := genMcisKey(nsId, mcisId, "")
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
	key := genMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Cloud_id)
	fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := vmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspVmId: " + temp.CspVmId)

	url := ""
	method := ""
	switch action {
	case actionTerminate:
		url = SPIDER_URL + "/vm/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName
		method = "DELETE"
	case actionReboot:
		url = SPIDER_URL + "/controlvm/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName + "&action=reboot"
		method = "GET"
	case actionSuspend:
		url = SPIDER_URL + "/controlvm/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName + "&action=suspend"
		method = "GET"
	case actionResume:
		url = SPIDER_URL + "/controlvm/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
		method = "GET"
	default:
		return errors.New(action + "is invalid actionType")
	}
	fmt.Println("url: " + url)

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
	fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

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
	key := genMcisKey(nsId, mcisId, "")
	fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return mcisStatusInfo{}, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	mcisStatus := mcisStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisStatus)

	vmList, err := getVmList(nsId, mcisId)
	fmt.Println("=============================================== %#v", vmList)
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

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
		CspVmId   string
		CspVmName string
		PublicIP  string
	}

	fmt.Println("[getVmStatus]" + vmId)
	key := genMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Cloud_id)
	fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := vmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspVmId: " + temp.CspVmId)

	url := SPIDER_URL + "/vmstatus/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
	method := "GET"

	fmt.Println("url: " + url)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
		return vmStatusInfo{}, err
	}

	res, err := client.Do(req)
	fmt.Println("Called CB-Spider API.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	type statusResponse struct {
		Status string
	}
	statusResponseTmp := statusResponse{}
	err2 := json.Unmarshal(body, &statusResponseTmp)
	if err2 != nil {
		fmt.Println(err2)
		return vmStatusInfo{}, err2
	}

	// Temporal CODE. This should be changed after CB-Spider fixes status types and strings/
	if statusResponseTmp.Status == "PENDING" {
		statusResponseTmp.Status = statusCreating
	} else if statusResponseTmp.Status == "RUNNING" {
		statusResponseTmp.Status = statusRunning
	} else if statusResponseTmp.Status == "STOPPING" {
		statusResponseTmp.Status = statusSuspending
	} else if statusResponseTmp.Status == "STOPPED" {
		statusResponseTmp.Status = statusSuspended
	} else if statusResponseTmp.Status == "REBOOTING" {
		statusResponseTmp.Status = statusRebooting
	} else if statusResponseTmp.Status == "SHUTTING-DOWN" {
		statusResponseTmp.Status = statusTerminating
	} else if statusResponseTmp.Status == "TERMINATED" {
		statusResponseTmp.Status = statusTerminated
	} else {
		statusResponseTmp.Status = "statusUndefined"
	}

	// End of Temporal CODE.

	vmStatusTmp := vmStatusInfo{}
	vmStatusTmp.Id = vmId
	vmStatusTmp.Name = content.CspVmName
	vmStatusTmp.Csp_vm_id = content.CspVmId
	vmStatusTmp.Public_ip = content.PublicIP
	vmStatusTmp.Status = statusResponseTmp.Status
	if err != nil {
		cblog.Error(err)
		vmStatusTmp.Status = "FAILED"
	}

	return vmStatusTmp, nil

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

func getVmIp(nsId string, mcisId string, vmId string) string {

	var content struct {
		Public_ip string `json:"public_ip"`
	}

	fmt.Println("[getVmIp]" + vmId)
	key := genMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Public_ip)

	return content.Public_ip
}

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
