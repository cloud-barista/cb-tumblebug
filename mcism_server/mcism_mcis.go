package main

import (
	"github.com/cloud-barista/cb-tumblebug/mcism_server/azurehandler"
	"github.com/cloud-barista/cb-tumblebug/mcism_server/ec2handler"
	"github.com/cloud-barista/cb-tumblebug/mcism_server/gcehandler"
	"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/scp"
	"github.com/cloud-barista/cb-tumblebug/mcism_server/serverhandler/sshrun"

	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/cloud-barista/cb-tumblebug/mcism_agent/grpc_def"
	"google.golang.org/grpc"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo"

	"sync"
)

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
	CbSecurityGroupIds   []string `string:"cbSecurityGroupIds"`
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
	Security_group_ids []string `string:"security_group_ids"`
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

type vmStatusInfo struct {
	Id            string `json:"id"`
	Csp_vm_id     string `json:"csp_vm_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Cpu_status    string `json:"cpu_status"`
	Memory_status string `json:"memory_status"`
	Disk_status   string `json:"disk_status"`
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

func restPostMcis(c echo.Context) error {

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

func restGetMcis(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

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
			terminateVm(nsId, mcisId, v)
		}

		mapA := map[string]string{"message": "The MCIS has been terminated"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "monitor" {

		var content struct {
			Id     string         `json:"id"`
			Name   string         `json:"name"`
			Vm_num string         `json:"vm_num"`
			Status string         `json:"status"`
			Vm     []vmStatusInfo `json:"vm"`
		}

		fmt.Println("[monitor MCIS]")

		key := genMcisKey(nsId, mcisId, "")
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}

		json.Unmarshal([]byte(keyValue.Value), &content)

		vmList, err := getVmList(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to monitor in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}

		for _, v := range vmList {
			vmKey := genMcisKey(nsId, mcisId, v)
			fmt.Println(vmKey)
			vmKeyValue, _ := store.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + vmKey}
				return c.JSON(http.StatusOK, &mapA)
			}

			fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := vmStatusInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v

			vmIp := getVmIp(nsId, mcisId, v)
			vmIpPort := vmIp + defaultMonitorPort
			//statusCpu, statusMem, statusDisk := monitorVm(vmIpPort)
			statusCpu := "0.8%"
			statusMem := "21%"
			statusDisk := "12%"

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

func restGetAllMcis(c echo.Context) error {

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

func restPutMcis(c echo.Context) error {
	return nil
}

func restDelMcis(c echo.Context) error {

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

func restDelAllMcis(c echo.Context) error {
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

func restPostMcisRecommand(c echo.Context) error {

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

func restPostMcisVm(c echo.Context) error {

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
	vmInfoData.Status = "Launching"

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

func restGetMcisVm(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")
	fmt.Println("[Get VM requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend VM]")

		mapA := map[string]string{"message": "The VM has been suspended"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		mapA := map[string]string{"message": "The VM has been resumed"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "restart" {
		fmt.Println("[restart VM]")

		mapA := map[string]string{"message": "The VM has been restarted"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		terminateVm(nsId, mcisId, vmId)

		mapA := map[string]string{"message": "The VM has been terminated"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "monitor" {

		fmt.Println("[monitor VM]")

		vmKey := genMcisKey(nsId, mcisId, vmId)
		fmt.Println(vmKey)
		vmKeyValue, _ := store.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + vmKey}
			return c.JSON(http.StatusOK, &mapA)
		}

		fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := vmStatusInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = vmId

		vmIp := getVmIp(nsId, mcisId, vmId)
		vmIpPort := vmIp + defaultMonitorPort
		//statusCpu, statusMem, statusDisk := monitorVm(vmIpPort)

		statusCpu := "0.8%"
		statusMem := "21%"
		statusDisk := "12%"
		fmt.Println("[Status for MCIS] VM:" + vmIpPort + " CPU:" + statusCpu + " MEM:" + statusMem + " DISK:" + statusDisk)

		vmTmp.Cpu_status = statusCpu
		vmTmp.Memory_status = statusMem
		vmTmp.Disk_status = statusDisk

		fmt.Printf("%+v\n", vmTmp)

		return c.JSON(http.StatusOK, &vmTmp)

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

func restPutMcisVm(c echo.Context) error {
	return nil
}

func restDelMcisVm(c echo.Context) error {

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

	// terminateMcis first
	err := terminateMcis(nsId, mcisId)
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

	// terminateVm first
	err := terminateVm(nsId, mcisId, vmId)

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

func registerRecommendList(nsId string, cpuSize string, memSize string, diskSize string, specId string, price string) error {

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
	mapA := map[string]string{"id": req.Id, "name": req.Name, "description": req.Description, "status": "launching", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
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
		vmInfoData.Status = "Launching"

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
	//goroutin
	defer wg.Done()

	key := genMcisKey(nsId, mcisId, "")
	keyValue, _ := store.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	addVmInfoToMcis(nsId, mcisId, vmInfoData)
	fmt.Printf("%+v\n", vmInfoData)

	//instanceIds, publicIPs := createVm(&vmInfoData)
	err := createVm(&vmInfoData)
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

func createVm(vmInfoData *vmInfo) error {

	fmt.Printf("createVm(vmInfoData *vmInfo)\n")
	fmt.Printf("%+v\n", vmInfoData)
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

	///userName := masterConfigInfos.AWS.USERNAME   // ec2-user
	keyName := masterConfigInfos.AWS.KEYNAME // aws.powerkim.keypair
	//keyPath := masterConfigInfos.AWS.KEYFILEPATH // /root/.aws/awspowerkimkeypair.pem

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
	/*
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
	*/
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

	//userName := masterConfigInfos.GCP.USERNAME   // byoungseob
	//keyPath := masterConfigInfos.GCP.KEYFILEPATH // /root/.gcp/gcppowerkimkeypair.pem

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
	/*
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
	*/
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
	//KeyPath := masterConfigInfos.AZURE.KEYFILEPATH
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
	/*
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
	*/
	// 1.5. add server list into etcd.
	//addServersToEtcd("azure", instanceIds, publicIPs)

	return instanceIds, publicIPs
}

func terminateMcis(nsId string, mcisId string) error {

	fmt.Println("[terminateMcis]" + mcisId)
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
		terminateVm(nsId, mcisId, v)
	}
	return nil

	//need to change status

}

func terminateVm(nsId string, mcisId string, vmId string) error {

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
	}

	fmt.Println("[terminateVm]" + vmId)
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

	//url := SPIDER_URL + "/vm?connection_name=" + temp.ConnectionName // for testapi.io
	url := SPIDER_URL + "/vm/" + temp.CspVmId + "?connection_name=" + temp.ConnectionName // for testapi.io
	fmt.Println("url: " + url)

	method := "DELETE"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
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
			terminateVmAws(content.Csp_vm_id)
		} else if strings.Compare(content.Cloud_id, "gcp") == 0 {
			terminateVmGcp(content.Csp_vm_id)
		} else if strings.Compare(content.Cloud_id, "azure") == 0 {
			terminateVmAzure(content.Csp_vm_id)
		} else {
			fmt.Println("==============ERROR=no matched provider_id=================")
		}
	*/

	return nil

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
