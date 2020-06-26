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

	//csv file handling
	"bufio"
	"encoding/csv"
	"os"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo"

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

const actionCreate string = "Create"
const actionTerminate string = "Terminate"
const actionSuspend string = "Suspend"
const actionResume string = "Resume"
const actionReboot string = "Reboot"
const actionComplete string = "None"

const statusRunning string = "Running"
const statusSuspended string = "Suspended"
const statusFailed string = "Failed"
const statusTerminated string = "Terminated"
const statusCreating string = "Creating"
const statusSuspending string = "Suspending"
const statusResuming string = "Resuming"
const statusRebooting string = "Rebooting"
const statusTerminating string = "Terminating"
const statusComplete string = "None"

const milkywayPort string = ":1324/milkyway/"

const sshDefaultUserName01 string = "cb-user"
const sshDefaultUserName02 string = "ubuntu"
const sshDefaultUserName03 string = "root"
const sshDefaultUserName04 string = "ec2-user"

// Structs for REST API

// 2020-04-13 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMHandler.go
type SpiderVMReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVMInfo
}

/*
type SpiderVMReqInfo struct { // Spider
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string
	VMSpecName         string

	VMUserId     string
	VMUserPasswd string
}
*/

type SpiderVMInfo struct { // Spider
	// Fields for request
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string

	// Fields for both request and response
	VMSpecName   string //  instance type or flavour, etc... ex) t2.micro or f1.micro
	VMUserId     string // ex) user1
	VMUserPasswd string

	// Fields for response
	IId               common.IID // {NameId, SystemId}
	ImageIId          common.IID
	VpcIID            common.IID
	SubnetIID         common.IID   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIIds []common.IID // AWS, ex) sg-0b7452563e1121bb6
	KeyPairIId        common.IID
	StartTime         time.Time  // Timezone: based on cloud-barista server location.
	Region            RegionInfo //  ex) {us-east1, us-east1-c} or {ap-northeast-2}
	NetworkInterface  string     // ex) eth0
	PublicIP          string
	PublicDNS         string
	PrivateIP         string
	PrivateDNS        string
	VMBootDisk        string // ex) /dev/sda1
	VMBlockDisk       string // ex)
	KeyValueList      []common.KeyValue
}

/* Not used yet
type VMStatusInfo struct { // Spider
	//IId      IID // {NameId, SystemId}
	VmStatus VMStatus
}
*/

// GO do not support Enum. So, define like this.
type VMStatus string // Spider
//type VMOperation string // Not used yet

const ( // Spider
	Creating VMStatus = "Creating" // from launch to running
	Running  VMStatus = "Running"

	Suspending VMStatus = "Suspending" // from running to suspended
	Suspended  VMStatus = "Suspended"
	Resuming   VMStatus = "Resuming" // from suspended to running

	Rebooting VMStatus = "Rebooting" // from running to running

	Terminating VMStatus = "Terminating" // from running, suspended to terminated
	Terminated  VMStatus = "Terminated"
	NotExist    VMStatus = "NotExist" // VM does not exist

	Failed VMStatus = "Failed"
)

type RegionInfo struct { // Spider
	Region string
	Zone   string
}

/*
type TbMcisReq struct {
	//Id     string    `json:"id"`
	Name   string    `json:"name"`
	Vm_req []TbVmReq `json:"vm_req"`
	//Vm_num         string  `json:"vm_num"`
	Placement_algo string `json:"placement_algo"`
	Description    string `json:"description"`
}
*/

type TbMcisInfo struct {
	// Fields for both request and response
	Name           string     `json:"name"`
	Vm             []TbVmInfo `json:"vm"`
	Placement_algo string     `json:"placement_algo"`
	Description    string     `json:"description"`

	// Additional fields for response
	Id           string `json:"id"`
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Disabled for now
	//Vm             []vmOverview `json:"vm"`
}

type TbVmReq struct {
	//Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`

	// 1. Required by CB-Spider
	CspVmName string `json:"cspVmName"`

	CspImageName        string `json:"cspImageName"`
	CspVirtualNetworkId string `json:"cspVirtualNetworkId"`
	//CspNetworkInterfaceId string   `json:"cspNetworkInterfaceId"`
	//CspPublicIPId         string   `json:"cspPublicIPId"`
	CspSecurityGroupIds []string `json:"cspSecurityGroupIds"`
	CspSpecId           string   `json:"cspSpecId"`
	CspKeyPairName      string   `json:"cspKeyPairName"`

	CbImageId          string `json:"cbImageId"`
	CbVirtualNetworkId string `json:"cbVirtualNetworkId"`
	//CbNetworkInterfaceId string   `json:"cbNetworkInterfaceId"`
	//CbPublicIPId         string   `json:"cbPublicIPId"`
	CbSecurityGroupIds []string `json:"cbSecurityGroupIds"`
	CbSpecId           string   `json:"cbSpecId"`
	CbKeyPairId        string   `json:"cbKeyPairId"`

	VMUserId     string `json:"vmUserId"`
	VMUserPasswd string `json:"vmUserPasswd"`

	Name        string `json:"name"`
	Config_name string `json:"config_name"`
	Spec_id     string `json:"spec_id"`
	Image_id    string `json:"image_id"`
	Vnet_id     string `json:"vnet_id"`
	Subnet_id   string `json:"subnet_id"`
	//Vnic_id            string   `json:"vnic_id"`
	//Public_ip_id       string   `json:"public_ip_id"`
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`
}

type TbVmInfo struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Config_name string `json:"config_name"`
	Spec_id     string `json:"spec_id"`
	Image_id    string `json:"image_id"`
	Vnet_id     string `json:"vnet_id"`
	Subnet_id   string `json:"subnet_id"`
	//Vnic_id            string   `json:"vnic_id"`
	//Public_ip_id       string   `json:"public_ip_id"`
	Security_group_ids []string `json:"security_group_ids"`
	Ssh_key_id         string   `json:"ssh_key_id"`
	Description        string   `json:"description"`
	Vm_access_id       string   `json:"vm_access_id"`
	Vm_access_passwd   string   `json:"vm_access_passwd"`

	VmUserId     string `json:"vmUserId"`
	VmUserPasswd string `json:"vmUserPasswd"`

	Location GeoLocation `json:"location"`

	// 2. Provided by CB-Spider
	Region      RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP    string     `json:"publicIP"`
	PublicDNS   string     `json:"publicDNS"`
	PrivateIP   string     `json:"privateIP"`
	PrivateDNS  string     `json:"privateDNS"`
	VMBootDisk  string     `json:"vmBootDisk"` // ex) /dev/sda1
	VMBlockDisk string     `json:"vmBlockDisk"`

	// 3. Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	CspViewVmDetail SpiderVMInfo `json:"cspViewVmDetail"`
}

type GeoLocation struct {
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
	BriefAddr    string `json:"briefAddr"`
	CloudType    string `json:"cloudType"`
	NativeRegion string `json:"nativeRegion"`
}

type McisStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	//Vm_num string         `json:"vm_num"`
	Status       string           `json:"status"`
	TargetStatus string           `json:"targetStatus"`
	TargetAction string           `json:"targetAction"`
	Vm           []TbVmStatusInfo `json:"vm"`
}

type TbVmStatusInfo struct {
	Id            string `json:"id"`
	Csp_vm_id     string `json:"csp_vm_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	TargetStatus  string `json:"targetStatus"`
	TargetAction  string `json:"targetAction"`
	Native_status string `json:"native_status"`
	Public_ip     string `json:"public_ip"`
}

type McisRecommendReq struct {
	Vm_req          []TbVmRecommendReq `json:"vm_req"`
	Placement_algo  string             `json:"placement_algo"`
	Placement_param []common.KeyValue  `json:"placement_param"`
	Max_result_num  string             `json:"max_result_num"`
}

type TbVmRecommendReq struct {
	Request_name   string `json:"request_name"`
	Max_result_num string `json:"max_result_num"`

	Vcpu_size   string `json:"vcpu_size"`
	Memory_size string `json:"memory_size"`
	Disk_size   string `json:"disk_size"`
	//Disk_type   string `json:"disk_type"`

	Placement_algo  string            `json:"placement_algo"`
	Placement_param []common.KeyValue `json:"placement_param"`
}

type McisCmdReq struct {
	Mcis_id   string `json:"mcis_id"`
	Vm_id     string `json:"vm_id"`
	Ip        string `json:"ip"`
	User_name string `json:"user_name"`
	Ssh_key   string `json:"ssh_key"`
	Command   string `json:"command"`
}

type TbVmPriority struct {
	Priority string          `json:"priority"`
	Vm_spec  mcir.TbSpecInfo `json:"vm_spec"`
}
type TbVmRecommendInfo struct {
	Vm_req          TbVmRecommendReq  `json:"vm_req"`
	Vm_priority     []TbVmPriority    `json:"vm_priority"`
	Placement_algo  string            `json:"placement_algo"`
	Placement_param []common.KeyValue `json:"placement_param"`
}

// MCIS API Proxy

func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &TbMcisInfo{}
	if err := c.Bind(req); err != nil {
		return err
	}

	key := CreateMcis(nsId, req)
	mcisId := req.Id

	keyValue, _ := store.Get(key)

	/*
		var content struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string   `json:"status"`
			TargetStatus   string   `json:"targetStatus"`
			TargetAction   string   `json:"targetAction"`
			Vm             []TbVmInfo `json:"vm"`
			Placement_algo string   `json:"placement_algo"`
			Description    string   `json:"description"`
		}
	*/
	content := TbMcisInfo{}

	json.Unmarshal([]byte(keyValue.Value), &content)

	vmList, err := ListVmId(nsId, mcisId)
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
		vmTmp := TbVmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = v
		content.Vm = append(content.Vm, vmTmp)
	}

	//mcisStatus, err := GetMcisStatus(nsId, mcisId)
	//content.Status = mcisStatus.Status

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

		err := ControlMcisAsync(nsId, mcisId, actionSuspend)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Suspending the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		err := ControlMcisAsync(nsId, mcisId, actionResume)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Resuming the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		err := ControlMcisAsync(nsId, mcisId, actionReboot)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Rebooting the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		//fmt.Println("len(vmList) %d ", len(vmList))
		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to terminate in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}

		/*
			for _, v := range vmList {
				ControlVm(nsId, mcisId, v, actionTerminate)
			}
		*/
		err = ControlMcisAsync(nsId, mcisId, actionTerminate)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Terminating the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to check in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}
		mcisStatusResponse, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			cblog.Error(err)
			return err
		}

		//fmt.Printf("%+v\n", mcisStatusResponse)
		common.PrintJsonPretty(mcisStatusResponse)

		return c.JSON(http.StatusOK, &mcisStatusResponse)

	} else {

		var content struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string     `json:"status"`
			TargetStatus   string     `json:"targetStatus"`
			TargetAction   string     `json:"targetAction"`
			Vm             []TbVmInfo `json:"vm"`
			Placement_algo string     `json:"placement_algo"`
			Description    string     `json:"description"`
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

		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		content.Status = mcisStatus.Status

		if err != nil {
			cblog.Error(err)
			return err
		}

		vmList, err := ListVmId(nsId, mcisId)
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
			vmTmp := TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v

			//get current vm status
			vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, v)
			if err != nil {
				cblog.Error(err)
			}
			vmTmp.Status = vmStatusInfoTmp.Status
			vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
			vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

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
	option := c.QueryParam("option")
	fmt.Println("[Get MCIS List requested with option: " + option)

	var content struct {
		//Name string     `json:"name"`
		Mcis []TbMcisInfo `json:"mcis"`
	}

	mcisList := ListMcisId(nsId)

	for _, v := range mcisList {

		key := common.GenMcisKey(nsId, v, "")
		//fmt.Println(key)
		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := TbMcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisId := v
		mcisTmp.Id = mcisId

		if option == "status" {
			//get current mcis status
			mcisStatus, err := GetMcisStatus(nsId, mcisId)
			if err != nil {
				cblog.Error(err)
				return err
			}
			mcisTmp.Status = mcisStatus.Status
		} else {
			//Set current mcis status with NullStr
			mcisTmp.Status = ""
		}

		vmList, err := ListVmId(nsId, mcisId)
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
			//vmTmp := vmOverview{}
			vmTmp := TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v1

			if option == "status" {
				//get current vm status
				vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, v1)
				if err != nil {
					cblog.Error(err)
				}
				vmTmp.Status = vmStatusInfoTmp.Status
			} else {
				//Set current vm status with NullStr
				vmTmp.Status = ""
			}

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

	err := DelMcis(nsId, mcisId)
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

	mcisList := ListMcisId(nsId)

	if len(mcisList) == 0 {
		mapA := map[string]string{"message": "No MCIS to delete"}
		return c.JSON(http.StatusOK, &mapA)
	}

	for _, v := range mcisList {
		err := DelMcis(nsId, v)
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

	req := &McisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	var content struct {
		//Vm_req          []TbVmRecommendReq    `json:"vm_req"`
		Vm_recommend    []TbVmRecommendInfo `json:"vm_recommend"`
		Placement_algo  string              `json:"placement_algo"`
		Placement_param []common.KeyValue   `json:"placement_param"`
	}
	//content.Vm_req = req.Vm_req
	content.Placement_algo = req.Placement_algo
	content.Placement_param = req.Placement_param

	vmList := req.Vm_req

	for i, v := range vmList {
		vmTmp := TbVmRecommendInfo{}
		//vmTmp.Request_name = v.Request_name
		vmTmp.Vm_req = req.Vm_req[i]
		vmTmp.Placement_algo = v.Placement_algo
		vmTmp.Placement_param = v.Placement_param

		var err error
		vmTmp.Vm_priority, err = GetRecommendList(nsId, v.Vcpu_size, v.Memory_size, v.Disk_size)

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

func RestPostCmdMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	req := &McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	vmIp := GetVmIp(nsId, mcisId, vmId)

	//fmt.Printf("[vmIp] " +vmIp)

	//sshKey := req.Ssh_key
	cmd := req.Command

	// find vaild username
	userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
	userNames := []string{sshDefaultUserName01, sshDefaultUserName02, sshDefaultUserName03, sshDefaultUserName04, userName, req.User_name}
	userName = VerifySshUserName(vmIp, userNames, sshKey)
	if userName == "" {
		return c.JSON(http.StatusInternalServerError, errors.New("No vaild username"))
	}

	//fmt.Printf("[userName] " +userName)

	fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
	fmt.Println("[CMD] " + cmd)

	if result, err := RunSSH(vmIp, userName, sshKey, cmd); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	} else {
		response := echo.Map{}
		response["result"] = *result
		return c.JSON(http.StatusOK, response)
	}
}

func VerifySshUserName(vmIp string, userNames []string, privateKey string) string {
	theUserName := ""
	cmd := "ls"
	for _, v := range userNames {
		fmt.Println("[SSH] " + "(" + vmIp + ")" + "with userName:" + v)
		fmt.Println("[CMD] " + cmd)
		if v != "" {
			if result, err := RunSSH(vmIp, v, privateKey, cmd); err == nil {
				theUserName = v
				fmt.Println("[RST] " + *result + "[Username] " + v)
				break
			}
		}
	}
	return theUserName
}

type SshCmdResult struct {
	Mcis_id string `json:"mcis_id"`
	Vm_id   string `json:"vm_id"`
	Vm_ip   string `json:"vm_ip"`
	Result  string `json:"result"`
	Err     error  `json:"err"`
}

func RestPostCmdMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	type contentSub struct {
		Mcis_id string `json:"mcis_id"`
		Vm_id   string `json:"vm_id"`
		Vm_ip   string `json:"vm_ip"`
		Result  string `json:"result"`
	}
	var content struct {
		Result_array []contentSub `json:"result_array"`
	}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return err
	}

	//goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp := GetVmIp(nsId, mcisId, vmId)

		cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.User_name
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }
		// find vaild username
		userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		userNames := []string{sshDefaultUserName01, sshDefaultUserName02, sshDefaultUserName03, sshDefaultUserName04, userName, req.User_name}
		userName = VerifySshUserName(vmIp, userNames, sshKey)

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		go RunSSHAsync(&wg, vmId, vmIp, userName, sshKey, cmd, &resultArray)

	}
	wg.Wait() //goroutine sync wg

	for _, v := range resultArray {

		resultTmp := contentSub{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, content)

}

type AgentInstallContentWrapper struct {
	Result_array []AgentInstallContent `json:"result_array"`
}
type AgentInstallContent struct {
	Mcis_id string `json:"mcis_id"`
	Vm_id   string `json:"vm_id"`
	Vm_ip   string `json:"vm_ip"`
	Result  string `json:"result"`
}

func RestPostInstallAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := InstallAgentToMcis(nsId, mcisId, req)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}

func InstallAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	content := AgentInstallContentWrapper{}

	//install script
	cmd := "wget https://github.com/cloud-barista/cb-milkyway/raw/master/src/milkyway -O ~/milkyway; chmod +x ~/milkyway; ~/milkyway > /dev/null 2>&1 & netstat -tulpn | grep milkyway"

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp := GetVmIp(nsId, mcisId, vmId)

		//cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.User_name
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }

		// find vaild username
		userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		userNames := []string{sshDefaultUserName01, sshDefaultUserName02, sshDefaultUserName03, sshDefaultUserName04, userName, req.User_name}
		userName = VerifySshUserName(vmIp, userNames, sshKey)

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		go RunSSHAsync(&wg, vmId, vmIp, userName, sshKey, cmd, &resultArray)

	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return content, nil

}

func RestGetBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	type bmReq struct {
		Host string `json:"host"`
	}
	req := &bmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}
	target := req.Host

	action := c.QueryParam("action")
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	var err error
	content := BenchmarkInfoArray{}

	vaildActions := "install init cpus cpum memR memW fioR fioW dbR dbW rtt mrtt clean"

	fmt.Println("[Benchmark] " + action)
	if strings.Contains(vaildActions, action) {
		content, err = BenchmarkAction(nsId, mcisId, action, option)
	} else {
		mapA := map[string]string{"message": "Not available action"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	if err != nil {
		mapError := map[string]string{"message": "Benchmark Error"}
		return c.JSON(http.StatusFailedDependency, &mapError)
	}
	common.PrintJsonPretty(content)
	return c.JSON(http.StatusOK, content)
}

type SpecBenchmarkInfo struct {
	SpecId     string `json:"specid"`
	Cpus       string `json:"cpus"`
	Cpum       string `json:"cpum"`
	MemR       string `json:"memR"`
	MemW       string `json:"memW"`
	FioR       string `json:"fioR"`
	FioW       string `json:"fioW"`
	DbR        string `json:"dbR"`
	DbW        string `json:"dbW"`
	Rtt        string `json:"rtt"`
	EvaledTime string `json:"evaledTime"`
}

func RestGetAllBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	type bmReq struct {
		Host string `json:"host"`
	}
	req := &bmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}
	target := req.Host

	action := "all"
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	var err error

	content := BenchmarkInfoArray{}

	allBenchCmd := []string{"cpus", "cpum", "memR", "memW", "fioR", "fioW", "dbR", "dbW", "rtt"}

	resultMap := make(map[string]SpecBenchmarkInfo)

	for i, v := range allBenchCmd {
		fmt.Println("[Benchmark] " + v)
		content, err = BenchmarkAction(nsId, mcisId, v, option)
		for _, k := range content.ResultArray {
			SpecId := k.SpecId
			Result := k.Result
			specBenchInfoTmp := SpecBenchmarkInfo{}

			val, exist := resultMap[SpecId]
			if exist {
				specBenchInfoTmp = val
			} else {
				specBenchInfoTmp.SpecId = SpecId
			}

			switch i {
			case 0:
				specBenchInfoTmp.Cpus = Result
			case 1:
				specBenchInfoTmp.Cpum = Result
			case 2:
				specBenchInfoTmp.MemR = Result
			case 3:
				specBenchInfoTmp.MemW = Result
			case 4:
				specBenchInfoTmp.FioR = Result
			case 5:
				specBenchInfoTmp.FioW = Result
			case 6:
				specBenchInfoTmp.DbR = Result
			case 7:
				specBenchInfoTmp.DbW = Result
			case 8:
				specBenchInfoTmp.Rtt = Result
			}

			resultMap[SpecId] = specBenchInfoTmp

		}
	}

	file, err := os.OpenFile("benchmarking.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	strsTmp := []string{}
	for key, val := range resultMap {
		strsTmp = nil
		fmt.Println(key, val)
		strsTmp = append(strsTmp, val.SpecId)
		strsTmp = append(strsTmp, val.Cpus)
		strsTmp = append(strsTmp, val.Cpum)
		strsTmp = append(strsTmp, val.MemR)
		strsTmp = append(strsTmp, val.MemW)
		strsTmp = append(strsTmp, val.FioR)
		strsTmp = append(strsTmp, val.FioW)
		strsTmp = append(strsTmp, val.DbR)
		strsTmp = append(strsTmp, val.DbW)
		strsTmp = append(strsTmp, val.Rtt)
		csvWriter.Write(strsTmp)
		csvWriter.Flush()
	}

	file2, err := os.OpenFile("rttmap.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file2.Close()
	csvWriter2 := csv.NewWriter(file2)

	const mrttArrayXMax = 50
	const mrttArrayYMax = 50
	mrttArray := make([][]string, mrttArrayXMax)
	for i := 0; i < mrttArrayXMax; i++ {
		mrttArray[i] = make([]string, mrttArrayYMax)
		for j := 0; j < mrttArrayYMax; j++ {
			mrttArray[i][j] = "0"
		}
	}

	rttIndexMapX := make(map[string]int)
	cntTargetX := 1
	rttIndexMapY := make(map[string]int)
	cntTargetY := 1

	action = "mrtt"
	fmt.Println("[Benchmark] " + action)
	content, err = BenchmarkAction(nsId, mcisId, action, option)
	for _, k := range content.ResultArray {
		SpecId := k.SpecId
		iX, exist := rttIndexMapX[SpecId]
		if !exist {
			rttIndexMapX[SpecId] = cntTargetX
			iX = cntTargetX
			mrttArray[iX][0] = SpecId
			cntTargetX++
		}
		for _, m := range k.ResultArray {
			tagetSpecId := m.SpecId
			tagetRtt := m.Result
			iY, exist2 := rttIndexMapY[tagetSpecId]
			if !exist2 {
				rttIndexMapY[tagetSpecId] = cntTargetY
				iY = cntTargetY
				mrttArray[0][iY] = tagetSpecId
				cntTargetY++
			}
			mrttArray[iX][iY] = tagetRtt
		}
	}

	csvWriter2.WriteAll(mrttArray)
	csvWriter2.Flush()

	if err != nil {
		mapError := map[string]string{"message": "Benchmark Error"}
		return c.JSON(http.StatusFailedDependency, &mapError)
	}
	common.PrintJsonPretty(content)
	return c.JSON(http.StatusOK, content)
}

type BenchmarkInfo struct {
	Result      string          `json:"result"`
	Unit        string          `json:"unit"`
	Desc        string          `json:"desc"`
	Elapsed     string          `json:"elapsed"`
	SpecId      string          `json:"specid"`
	ResultArray []BenchmarkInfo `json:"resultarray"` // struct-element cycle ?
}

type BenchmarkInfoArray struct {
	ResultArray []BenchmarkInfo `json:"resultarray"`
}

type BenchmarkReq struct {
	Host string `json:"host"`
	Spec string `json:"spec"`
}

type MultihostBenchmarkReq struct {
	Multihost []BenchmarkReq `json:"multihost"`
}

func CallMilkyway(wg *sync.WaitGroup, vmList []string, nsId string, mcisId string, vmId string, vmIp string, action string, option string, results *BenchmarkInfoArray) {
	defer wg.Done() //goroutine sync done

	url := "http://" + vmIp + milkywayPort + action
	method := "GET"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Create Req body
	type JsonTemplate struct {
		Host string `json:"host"`
	}
	tempReq := JsonTemplate{}
	tempReq.Host = option
	payload, _ := json.MarshalIndent(tempReq, "", "  ")

	if action == "mrtt" {
		reqTmp := MultihostBenchmarkReq{}
		for _, vm := range vmList {
			vmIdTmp := vm
			vmIpTmp := GetVmIp(nsId, mcisId, vmIdTmp)
			fmt.Println("[Test for vmList " + vmIdTmp + ", " + vmIpTmp + "]")

			hostTmp := BenchmarkReq{}
			hostTmp.Host = vmIpTmp
			hostTmp.Spec = GetVmSpecId(nsId, mcisId, vmIdTmp)
			reqTmp.Multihost = append(reqTmp.Multihost, hostTmp)
		}
		common.PrintJsonPretty(reqTmp)
		payload, _ = json.MarshalIndent(reqTmp, "", "  ")
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		fmt.Println(err)
	}
	errStr := ""
	res, err := client.Do(req)
	if err != nil {
		cblog.Error(err)
		errStr = err.Error()
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cblog.Error(err)
		errStr = err.Error()
	}
	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		errStr = err.Error()
	}

	if action == "mrtt" {
		//benchInfoTmp := BenchmarkInfo{}
		resultTmp := BenchmarkInfo{}
		err2 := json.Unmarshal(body, &resultTmp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		//benchInfoTmp.ResultArray =  resultTmp.ResultArray
		if errStr != "" {
			resultTmp.Result = errStr
		}
		resultTmp.SpecId = GetVmSpecId(nsId, mcisId, vmId)
		results.ResultArray = append(results.ResultArray, resultTmp)

	} else {
		resultTmp := BenchmarkInfo{}
		err2 := json.Unmarshal(body, &resultTmp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		if errStr != "" {
			resultTmp.Result = errStr
		}
		resultTmp.SpecId = GetVmSpecId(nsId, mcisId, vmId)
		results.ResultArray = append(results.ResultArray, resultTmp)
	}

}

func BenchmarkAction(nsId string, mcisId string, action string, option string) (BenchmarkInfoArray, error) {

	var results BenchmarkInfoArray

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return BenchmarkInfoArray{}, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp := GetVmIp(nsId, mcisId, vmId)

		go CallMilkyway(&wg, vmList, nsId, mcisId, vmId, vmIp, action, option, &results)
	}
	wg.Wait() //goroutine sync wg

	return results, nil

}

/*
func BenchmarkAction(nsId string, mcisId string, action string, option string) (BenchmarkInfoArray, error) {


	var results BenchmarkInfoArray

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		cblog.Error(err)
		return BenchmarkInfoArray{}, err
	}

	for _, v := range vmList {

		vmId := v
		vmIp := GetVmIp(nsId, mcisId, vmId)

		url := "http://"+ vmIp + milkywayPort + action
		method := "GET"

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}



		// Create Req body
		type JsonTemplate struct {
			Host string `json:"host"`
		}
		tempReq := JsonTemplate{}
		tempReq.Host = option
		payload, _ := json.MarshalIndent(tempReq, "", "  ")

		if action == "mrtt" {
			reqTmp := MultihostBenchmarkReq{}
			for _, vm := range vmList {
				vmIdTmp := vm
				vmIpTmp := GetVmIp(nsId, mcisId, vmIdTmp)
				fmt.Println("[Test for vmList " + vmIdTmp + ", " +vmIpTmp + "]")

				hostTmp := BenchmarkReq{}
				hostTmp.Host = vmIpTmp
				reqTmp.Multihost = append(reqTmp.Multihost, hostTmp)
			}
			common.PrintJsonPretty(reqTmp)
			payload, _ = json.MarshalIndent(reqTmp, "", "  ")
		}

		req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
		req.Header.Add("Content-Type", "application/json")
		if err != nil {
			fmt.Println(err)
		}

		res, err := client.Do(req)
		if err != nil {
			cblog.Error(err)
			return BenchmarkInfoArray{}, err
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			cblog.Error(err)
			return BenchmarkInfoArray{}, err
		}
		fmt.Println(string(body))

		fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			cblog.Error(err)
			return BenchmarkInfoArray{}, err
		}

		if action == "mrtt" {
			//benchInfoTmp := BenchmarkInfo{}
			resultTmp := BenchmarkInfo{}
			err2 := json.Unmarshal(body, &resultTmp)
			if err2 != nil {
				fmt.Println("whoops:", err2)
			}
			//benchInfoTmp.ResultArray =  resultTmp.ResultArray
			results.ResultArray = append(results.ResultArray, resultTmp)

		} else{
			resultTmp := BenchmarkInfo{}
			err2 := json.Unmarshal(body, &resultTmp)
			if err2 != nil {
				fmt.Println("whoops:", err2)
			}
			results.ResultArray = append(results.ResultArray, resultTmp)
		}

	}

	return results, nil

}
*/

// VM API Proxy

func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &TbVmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	vmInfoData := TbVmInfo{}
	//vmInfoData.Id = common.GenUuid()
	vmInfoData.Id = common.GenId(req.Name)
	//req.Id = vmInfoData.Id
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
	vmInfoData.Subnet_id = req.Subnet_id
	//vmInfoData.Vnic_id = req.Vnic_id
	//vmInfoData.Public_ip_id = req.Public_ip_id
	vmInfoData.Security_group_ids = req.Security_group_ids
	vmInfoData.Ssh_key_id = req.Ssh_key_id
	vmInfoData.Description = req.Description

	vmInfoData.Config_name = req.Config_name

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	//CreateMcis(nsId, req)
	//err := AddVmToMcis(nsId, mcisId, vmInfoData)
	err := AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)

	if err != nil {
		mapA := map[string]string{"message": "Cannot find " + common.GenMcisKey(nsId, mcisId, "")}
		return c.JSON(http.StatusOK, &mapA)
	}
	wg.Wait()

	vmStatus, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

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

		ControlVm(nsId, mcisId, vmId, actionSuspend)
		mapA := map[string]string{"message": "Suspending the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		ControlVm(nsId, mcisId, vmId, actionResume)
		mapA := map[string]string{"message": "Resuming the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")

		ControlVm(nsId, mcisId, vmId, actionReboot)
		mapA := map[string]string{"message": "Rebooting the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		ControlVm(nsId, mcisId, vmId, actionTerminate)

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

		vmStatusResponse, err := GetVmStatus(nsId, mcisId, vmId)

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
		vmTmp := TbVmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = vmId

		//get current vm status
		vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmId)
		if err != nil {
			cblog.Error(err)
		}

		vmTmp.Status = vmStatusInfoTmp.Status
		vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
		vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

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

	err := DelMcisVm(nsId, mcisId, vmId)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the VM info"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the VM info"}
	return c.JSON(http.StatusOK, &mapA)
}

// MCIS Information Managemenet

/*
func AddVmInfoToMcis(nsId string, mcisId string, vmInfoData TbVmInfo) {

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
*/

func UpdateMcisInfo(nsId string, mcisInfoData TbMcisInfo) {
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

func UpdateVmInfo(nsId string, mcisId string, vmInfoData TbVmInfo) {
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

func ListMcisId(nsId string) []string {

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

func ListVmId(nsId string, mcisId string) ([]string, error) {

	fmt.Println("[ListVmId]")
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

func DelMcis(nsId string, mcisId string) error {

	fmt.Println("[Delete MCIS] " + mcisId)

	// ControlMcis first
	err := ControlMcis(nsId, mcisId, actionTerminate)
	if err != nil {
		cblog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := ListVmId(nsId, mcisId)
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

func DelMcisVm(nsId string, mcisId string, vmId string) error {

	fmt.Println("[Delete VM] " + vmId)

	// ControlVm first
	err := ControlVm(nsId, mcisId, vmId, actionTerminate)

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
func GetRecommendList(nsId string, cpuSize string, memSize string, diskSize string) ([]TbVmPriority, error) {

	fmt.Println("GetRecommendList")

	var content struct {
		Id             string
		Price          string
		ConnectionName string
	}
	//fmt.Println("[Get MCISs")
	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize
	fmt.Println(key)
	keyValue, err := store.GetList(key, true)
	if err != nil {
		cblog.Error(err)
		return []TbVmPriority{}, err
	}

	var vmPriorityList []TbVmPriority

	for cnt, v := range keyValue {
		fmt.Println("getRecommendList1: " + v.Key)
		err = json.Unmarshal([]byte(v.Value), &content)
		if err != nil {
			cblog.Error(err)
			return []TbVmPriority{}, err
		}

		content2 := mcir.TbSpecInfo{}
		key2 := common.GenResourceKey(nsId, "spec", content.Id)

		keyValue2, err := store.Get(key2)
		if err != nil {
			cblog.Error(err)
			return []TbVmPriority{}, err
		}
		json.Unmarshal([]byte(keyValue2.Value), &content2)
		content2.Id = content.Id

		vmPriorityTmp := TbVmPriority{}
		vmPriorityTmp.Priority = strconv.Itoa(cnt)
		vmPriorityTmp.Vm_spec = content2
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList, err

	//requires error handling

}

// MCIS Control

func CreateMcis(nsId string, req *TbMcisInfo) string {
	/*
		check, _ := checkMcis(nsId, req.Name)

		if check {
			//temp := TbMcisInfo{}
			//err := fmt.Errorf("The mcis " + req.Name + " already exists.")
			return ""
		}
	*/

	targetAction := actionCreate
	targetStatus := statusRunning

	//req.Id = common.GenUuid()
	req.Id = common.GenId(req.Name)
	vmRequest := req.Vm
	mcisId := req.Id

	fmt.Println("=========================== Put createSvc")
	key := common.GenMcisKey(nsId, mcisId, "")
	//mapA := map[string]string{"name": req.Name, "description": req.Description, "status": "launching", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
	mapA := map[string]string{"id": mcisId, "name": req.Name, "description": req.Description, "status": statusCreating, "targetAction": targetAction, "targetStatus": targetStatus}
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

		vmInfoData := TbVmInfo{}
		//vmInfoData.Id = common.GenUuid()
		vmInfoData.Id = common.GenId(k.Name)
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

		vmInfoData.Status = statusCreating
		vmInfoData.TargetAction = targetAction
		vmInfoData.TargetStatus = targetStatus

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
		vmInfoData.Subnet_id = k.Subnet_id
		//vmInfoData.Vnic_id = k.Vnic_id
		//vmInfoData.Public_ip_id = k.Public_ip_id
		vmInfoData.Security_group_ids = k.Security_group_ids
		vmInfoData.Ssh_key_id = k.Ssh_key_id
		vmInfoData.Description = k.Description

		vmInfoData.Config_name = k.Config_name

		/////////

		go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)
		//AddVmToMcis(nsId, req.Id, vmInfoData)

		if err != nil {
			errMsg := "Failed to add VM " + vmInfoData.Name + " to MCIS " + req.Name
			return errMsg
		}
	}
	wg.Wait()

	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	mcisTmp.Status = mcisStatusTmp.Status

	if mcisTmp.TargetStatus == mcisTmp.Status {
		mcisTmp.TargetStatus = statusComplete
		mcisTmp.TargetAction = actionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	return key
}

func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo) error {
	fmt.Printf("\n[AddVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, _ := store.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	//AddVmInfoToMcis(nsId, mcisId, *vmInfoData)
	key = common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := store.Put(string(key), string(val))
	if err != nil {
		cblog.Error(err)
	}
	fmt.Printf("\n[vmInfoData]\n %+v\n", vmInfoData)

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mcisId, vmInfoData)
	if err != nil {
		vmInfoData.Status = statusFailed
		UpdateVmInfo(nsId, mcisId, *vmInfoData)
		cblog.Error(err)
		return err
	}

	//vmInfoData.PublicIP = string(*publicIPs[0])
	//vmInfoData.CspVmId = string(*instanceIds[0])
	vmInfoData.Status = statusRunning
	vmInfoData.TargetAction = actionComplete
	vmInfoData.TargetStatus = statusComplete
	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

func CreateVm(nsId string, mcisId string, vmInfoData *TbVmInfo) error {

	fmt.Printf("\n\n[CreateVm(vmInfoData *TbVmInfo)]\n\n")

	switch {
	case vmInfoData.Name == "":
		err := fmt.Errorf("vmInfoData.Name is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Image_id == "":
		err := fmt.Errorf("vmInfoData.Image_id is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Config_name == "":
		err := fmt.Errorf("vmInfoData.Config_name is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Ssh_key_id == "":
		err := fmt.Errorf("vmInfoData.Ssh_key_id is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Spec_id == "":
		err := fmt.Errorf("vmInfoData.Spec_id is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Security_group_ids == nil:
		err := fmt.Errorf("vmInfoData.Security_group_ids is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Vnet_id == "":
		err := fmt.Errorf("vmInfoData.Vnet_id is empty")
		cblog.Error(err)
		return err
	case vmInfoData.Subnet_id == "":
		err := fmt.Errorf("vmInfoData.Subnet_id is empty")
		cblog.Error(err)
		return err
	default:

	}

	//prettyJSON, err := json.MarshalIndent(vmInfoData, "", "    ")
	//if err != nil {
	//log.Fatal("Failed to generate json")
	//}
	//fmt.Printf("%s\n", string(prettyJSON))

	//common.PrintJsonPretty(vmInfoData)

	//fmt.Printf("%+v\n", vmInfoData.CspVmId)

	url := common.SPIDER_URL + "/vm"

	method := "POST"

	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("url: " + url + " method: " + method)

	tempReq := SpiderVMReqInfoWrapper{}
	tempReq.ConnectionName = vmInfoData.Config_name

	tempReq.ReqInfo.Name = vmInfoData.Name

	err := fmt.Errorf("")

	tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, "image", vmInfoData.Image_id)
	if tempReq.ReqInfo.ImageName == "" || err != nil {
		cblog.Error(err)
		return err
	}

	tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, "spec", vmInfoData.Spec_id)
	if tempReq.ReqInfo.VMSpecName == "" || err != nil {
		cblog.Error(err)
		return err
	}

	tempReq.ReqInfo.VPCName = vmInfoData.Vnet_id //common.GetCspResourceId(nsId, "vNet", vmInfoData.Vnet_id)
	if tempReq.ReqInfo.VPCName == "" {
		cblog.Error(err)
		return err
	}

	tempReq.ReqInfo.SubnetName = vmInfoData.Subnet_id //common.GetCspResourceId(nsId, "vNet", vmInfoData.Subnet_id)
	if tempReq.ReqInfo.SubnetName == "" {
		cblog.Error(err)
		return err
	}

	var SecurityGroupIdsTmp []string
	for _, v := range vmInfoData.Security_group_ids {
		CspSgId := v //common.GetCspResourceId(nsId, "securityGroup", v)
		if CspSgId == "" {
			cblog.Error(err)
			return err
		}

		SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
	}
	tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

	tempReq.ReqInfo.KeyPairName = vmInfoData.Ssh_key_id //common.GetCspResourceId(nsId, "sshKey", vmInfoData.Ssh_key_id)
	if tempReq.ReqInfo.KeyPairName == "" {
		cblog.Error(err)
		return err
	}

	tempReq.ReqInfo.VMUserId = vmInfoData.Vm_access_id
	tempReq.ReqInfo.VMUserPasswd = vmInfoData.Vm_access_passwd

	fmt.Printf("\n[Request body to CB-SPIDER for Creating VM]\n")
	common.PrintJsonPretty(tempReq)

	payload, _ := json.Marshal(tempReq)
	fmt.Println("payload: " + string(payload))

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

	fmt.Println("Called CB-Spider API.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	//fmt.Println(string(body))

	temp := SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
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

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		return err
	}

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

	configTmp, _ := common.GetConnConfig(vmInfoData.Config_name)
	vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(temp.Region.Region))

	//content.Status = temp.
	//content.Cloud_id = temp.

	// cb-store
	//fmt.Println("=========================== PUT createVM")
	/*
		Key := genResourceKey(nsId, "vm", content.Id)

		Val, _ := json.Marshal(content)
		fmt.Println("Key: ", Key)
		fmt.Println("Val: ", Val)
		err := store.Put(string(Key), string(Val))
		if err != nil {
			cblog.Error(err)
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

func ControlMcis(nsId string, mcisId string, action string) error {

	fmt.Println("[ControlMcis]" + mcisId + " to " + action)
	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	vmList, err := ListVmId(nsId, mcisId)
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
		ControlVm(nsId, mcisId, v, action)
	}
	return nil

	//need to change status

}

func CheckAllowedTransition(nsId string, mcisId string, action string) error {

	fmt.Println("[CheckAllowedTransition]" + mcisId + " to " + action)
	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	mcisTmp := TbMcisInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	UpdateMcisInfo(nsId, mcisTmp)

	if mcisStatusTmp.Status == statusTerminating || mcisStatusTmp.Status == statusResuming || mcisStatusTmp.Status == statusSuspending || mcisStatusTmp.Status == statusCreating || mcisStatusTmp.Status == statusRebooting {
		return errors.New(action + " is not allowed for MCIS under " + mcisStatusTmp.Status)
	}
	if mcisStatusTmp.Status == statusTerminated {
		return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
	}
	if mcisStatusTmp.Status == statusSuspended {
		if action == actionResume || action == actionTerminate {
			return nil
		} else {
			return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
		}
	}
	return nil
}

func ControlMcisAsync(nsId string, mcisId string, action string) error {

	checkError := CheckAllowedTransition(nsId, mcisId, action)
	if checkError != nil {
		return checkError
	}

	fmt.Println("[ControlMcis]" + mcisId + " to " + action)
	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	mcisTmp := TbMcisInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	vmList, err := ListVmId(nsId, mcisId)
	fmt.Println("=============================================== %#v", vmList)
	if err != nil {
		cblog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return nil
	}

	switch action {
	case actionTerminate:

		mcisTmp.TargetAction = actionTerminate
		mcisTmp.TargetStatus = statusTerminated
		mcisTmp.Status = statusTerminating

	case actionReboot:

		mcisTmp.TargetAction = actionReboot
		mcisTmp.TargetStatus = statusRunning
		mcisTmp.Status = statusRebooting

	case actionSuspend:

		mcisTmp.TargetAction = actionSuspend
		mcisTmp.TargetStatus = statusSuspended
		mcisTmp.Status = statusSuspending

	case actionResume:

		mcisTmp.TargetAction = actionResume
		mcisTmp.TargetStatus = statusRunning
		mcisTmp.Status = statusResuming

	default:
		return errors.New(action + "is invalid actionType")
	}
	UpdateMcisInfo(nsId, mcisTmp)

	//goroutin sync wg
	var wg sync.WaitGroup
	var results ControlVmResultWrapper
	// delete vms info
	for _, v := range vmList {
		wg.Add(1)

		go ControlVmAsync(&wg, nsId, mcisId, v, action, &results)
	}
	wg.Wait() //goroutine sync wg

	return nil

	//need to change status

}

type ControlVmResult struct {
	VmId   string `json:"vm_id"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}
type ControlVmResultWrapper struct {
	ResultArray []ControlVmResult `json:"resultarray"`
}

func ControlVmAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, action string, results *ControlVmResultWrapper) error {
	defer wg.Done() //goroutine sync done

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
	}

	fmt.Println("[ControlVm]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.Cloud_id)
	//fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Println("\n\n[Calling SPIDER]START vmControl")

	fmt.Println("temp.CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	/*
		cspType := getVMsCspType(nsId, mcisId, vmId)
		var cspVmId string
		if cspType == "AWS" {
			cspVmId = temp.CspViewVmDetail.Id
		} else {
	*/
	cspVmId := temp.CspViewVmDetail.IId.NameId
	common.PrintJsonPretty(temp.CspViewVmDetail)

	url := ""
	method := ""
	switch action {
	case actionTerminate:

		temp.TargetAction = actionTerminate
		temp.TargetStatus = statusTerminated
		temp.Status = statusTerminating

		//url = common.SPIDER_URL + "/vm/" + cspVmId + "?connection_name=" + temp.Config_name
		url = common.SPIDER_URL + "/vm/" + cspVmId
		method = "DELETE"
	case actionReboot:

		temp.TargetAction = actionReboot
		temp.TargetStatus = statusRunning
		temp.Status = statusRebooting

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=reboot"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=reboot"
		method = "GET"
	case actionSuspend:

		temp.TargetAction = actionSuspend
		temp.TargetStatus = statusSuspended
		temp.Status = statusSuspending

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=suspend"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=suspend"
		method = "GET"
	case actionResume:

		temp.TargetAction = actionResume
		temp.TargetStatus = statusRunning
		temp.Status = statusResuming

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=resume"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=resume"
		method = "GET"
	default:
		return errors.New(action + "is invalid actionType")
	}

	UpdateVmInfo(nsId, mcisId, temp)
	//fmt.Println("url: " + url + " method: " + method)

	type ControlVMReqInfo struct {
		ConnectionName string
	}
	tempReq := ControlVMReqInfo{}
	tempReq.ConnectionName = temp.Config_name
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
		temp.Status = statusFailed
		UpdateVmInfo(nsId, mcisId, temp)
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	//fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	//var resBodyTmp struct {
	//	Status string `json:"Status"`
	//}

	var errTmp error
	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		errTmp = err
	}

	//err2 := json.Unmarshal(body, &resBodyTmp)
	//if err2 != nil {
	//	fmt.Println("whoops:", err2)
	//	return errors.New("whoops: "+ err2.Error())
	//}

	resultTmp := ControlVmResult{}
	err2 := json.Unmarshal(body, &resultTmp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
		cblog.Error(err)
		errTmp = err
	}
	if errTmp != nil {
		resultTmp.Error = errTmp

		temp.Status = statusFailed
		UpdateVmInfo(nsId, mcisId, temp)
	}
	results.ResultArray = append(results.ResultArray, resultTmp)

	common.PrintJsonPretty(resultTmp)

	fmt.Println("[Calling SPIDER]END vmControl\n\n")

	UpdateVmPublicIp(nsId, mcisId, temp)

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

func ControlVm(nsId string, mcisId string, vmId string, action string) error {

	var content struct {
		Cloud_id  string `json:"cloud_id"`
		Csp_vm_id string `json:"csp_vm_id"`
	}

	fmt.Println("[ControlVm]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.Cloud_id)
	//fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Println("\n\n[Calling SPIDER]START vmControl")

	fmt.Println("temp.CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	/*
		cspType := getVMsCspType(nsId, mcisId, vmId)
		var cspVmId string
		if cspType == "AWS" {
			cspVmId = temp.CspViewVmDetail.Id
		} else {
	*/
	cspVmId := temp.CspViewVmDetail.IId.NameId
	common.PrintJsonPretty(temp.CspViewVmDetail)

	url := ""
	method := ""
	switch action {
	case actionTerminate:
		//url = common.SPIDER_URL + "/vm/" + cspVmId + "?connection_name=" + temp.Config_name
		url = common.SPIDER_URL + "/vm/" + cspVmId
		method = "DELETE"
	case actionReboot:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=reboot"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=reboot"
		method = "GET"
	case actionSuspend:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=suspend"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=suspend"
		method = "GET"
	case actionResume:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.Config_name + "&action=resume"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=resume"
		method = "GET"
	default:
		return errors.New(action + "is invalid actionType")
	}
	//fmt.Println("url: " + url + " method: " + method)

	type ControlVMReqInfo struct {
		ConnectionName string
	}
	tempReq := ControlVMReqInfo{}
	tempReq.ConnectionName = temp.Config_name
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")

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

func GetMcisStatus(nsId string, mcisId string) (McisStatusInfo, error) {

	fmt.Println("[GetMcisStatus]" + mcisId)
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return McisStatusInfo{}, err
	}

	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	mcisStatus := McisStatusInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisStatus)

	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	vmList, err := ListVmId(nsId, mcisId)
	//fmt.Println("=============================================== %#v", vmList)
	if err != nil {
		cblog.Error(err)
		return McisStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return McisStatusInfo{}, nil
	}

	for _, v := range vmList {
		vmStatusTmp, err := GetVmStatus(nsId, mcisId, v)
		if err != nil {
			cblog.Error(err)
			vmStatusTmp.Status = statusFailed
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

	numVm := len(mcisStatus.Vm)
	proportionStr := "-(" + strconv.Itoa(tmpMax) + "/" + strconv.Itoa(numVm) + ")"
	if tmpMax == numVm {
		mcisStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numVm {
		mcisStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		mcisStatus.Status = statusFlagStr[9] + proportionStr
	}
	proportionStr = "-(" + strconv.Itoa(statusFlag[0]) + "/" + strconv.Itoa(numVm) + ")"
	if statusFlag[0] > 0 {
		mcisStatus.Status = statusFlagStr[0] + proportionStr
	}
	proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numVm) + ")"
	if statusFlag[9] > 0 {
		mcisStatus.Status = statusFlagStr[9] + proportionStr
	}

	var isDone bool
	isDone = true
	for _, v := range mcisStatus.Vm {
		if v.TargetStatus != statusComplete {
			isDone = false
		}
	}
	if isDone {
		mcisStatus.TargetAction = actionComplete
		mcisStatus.TargetStatus = statusComplete
		mcisTmp.TargetAction = actionComplete
		mcisTmp.TargetStatus = statusComplete
		UpdateMcisInfo(nsId, mcisTmp)
	}

	return mcisStatus, nil

	//need to change status

}

func GetVmStatus(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	/*
		var content struct {
			Cloud_id  string `json:"cloud_id"`
			Csp_vm_id string `json:"csp_vm_id"`
			CspVmId   string
			CspVmName string
			PublicIP  string
		}
	*/

	fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	//json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.Cloud_id)
	//fmt.Printf("%+v\n", content.Csp_vm_id)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	//UpdateVmPublicIp. update temp TbVmInfo{} with changed IP
	UpdateVmPublicIp(nsId, mcisId, temp)
	keyValue, _ = store.Get(key)
	unmarshalErr = json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)
	/*
		var cspVmId string
		cspType := getVMsCspType(nsId, mcisId, vmId)
		if cspType == "AWS" {
			cspVmId = temp.CspViewVmDetail.Id
		} else {
	*/
	cspVmId := temp.CspViewVmDetail.IId.NameId

	url := common.SPIDER_URL + "/vmstatus/" + cspVmId // + "?connection_name=" + temp.Config_name
	method := "GET"

	//fmt.Println("url: " + url)

	type VMStatusReqInfo struct {
		ConnectionName string
	}
	tempReq := VMStatusReqInfo{}
	tempReq.ConnectionName = temp.Config_name
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	errorInfo := TbVmStatusInfo{}
	errorInfo.Status = statusFailed

	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}
	req.Header.Add("Content-Type", "application/json")

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

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Id = vmId
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.Csp_vm_id = temp.CspViewVmDetail.IId.NameId
	vmStatusTmp.Public_ip = temp.PublicIP
	vmStatusTmp.Native_status = statusResponseTmp.Status

	vmStatusTmp.TargetAction = temp.TargetAction
	vmStatusTmp.TargetStatus = temp.TargetStatus

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

	//Correct undefined status using TargetAction
	if vmStatusTmp.TargetAction == actionCreate {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = statusCreating
		}
	}
	if vmStatusTmp.TargetAction == actionTerminate {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = statusTerminated
		}
		if statusResponseTmp.Status == statusSuspending {
			statusResponseTmp.Status = statusTerminated
		}
	}
	if vmStatusTmp.TargetAction == actionResume {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = statusResuming
		}
		if statusResponseTmp.Status == statusCreating {
			statusResponseTmp.Status = statusResuming
		}

	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if vmStatusTmp.TargetAction == actionReboot {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = statusRebooting
		}
		if statusResponseTmp.Status == statusSuspending || statusResponseTmp.Status == statusSuspended || statusResponseTmp.Status == statusCreating || statusResponseTmp.Status == statusResuming {
			statusResponseTmp.Status = statusRebooting
		}
	}

	// End of Temporal CODE.

	vmStatusTmp.Status = statusResponseTmp.Status
	if err != nil {
		cblog.Error(err)
		vmStatusTmp.Status = statusFailed
	}

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		vmStatusTmp.TargetStatus = statusComplete
		vmStatusTmp.TargetAction = actionComplete
	}

	return vmStatusTmp, nil

}

func UpdateVmPublicIp(nsId string, mcisId string, vmInfoData TbVmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, vmInfoData.Id)

	if err != nil {
		cblog.Error(err)
		return err
	}

	vmInfoData.PublicIP = vmInfoTmp.Public_ip

	UpdateVmInfo(nsId, mcisId, vmInfoData)

	return nil

}

func GetVmCurrentPublicIp(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	cspVmId := temp.CspViewVmDetail.IId.NameId

	url := common.SPIDER_URL + "/vm/" + cspVmId // + "?connection_name=" + temp.Config_name
	method := "GET"

	type VMStatusReqInfo struct {
		ConnectionName string
	}
	tempReq := VMStatusReqInfo{}
	tempReq.ConnectionName = temp.Config_name
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	errorInfo := TbVmStatusInfo{}
	errorInfo.Status = statusFailed

	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	//fmt.Println("Called CB-Spider API.")

	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	type statusResponse struct {
		Status   string
		PublicIP string
	}
	statusResponseTmp := statusResponse{}

	err2 := json.Unmarshal(body, &statusResponseTmp)
	if err2 != nil {
		fmt.Println(err2)
		return errorInfo, err2
	}

	common.PrintJsonPretty(statusResponseTmp)
	fmt.Println("[Calling SPIDER]END\n\n")

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Public_ip = statusResponseTmp.PublicIP

	return vmStatusTmp, nil

}

func ValidateStatus() {

	nsList := common.ListNsId()

	for _, v := range nsList {
		fmt.Println("validateStatus: NS[" + v + "]")
	}

}

func GetVmSshKey(nsId string, mcisId string, vmId string) (string, string) {

	var content struct {
		Ssh_key_id string `json:"ssh_key_id"`
	}

	fmt.Println("[GetVmIp]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Ssh_key_id)

	sshKey := common.GenResourceKey(nsId, "sshKey", content.Ssh_key_id)
	keyValue, _ = store.Get(sshKey)
	var keyContent struct {
		Username   string `json:"username"`
		PrivateKey string `json:"privateKey"`
	}
	json.Unmarshal([]byte(keyValue.Value), &keyContent)

	return keyContent.Username, keyContent.PrivateKey
}

func GetVmIp(nsId string, mcisId string, vmId string) string {

	var content struct {
		PublicIP string `json:"publicIP"`
	}

	fmt.Println("[GetVmIp]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := store.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.PublicIP)

	return content.PublicIP
}

func GetVmSpecId(nsId string, mcisId string, vmId string) string {

	var content struct {
		Spec_id string `json:"spec_id"`
	}

	fmt.Println("[getVmSpecID]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, _ := store.Get(key)

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.Spec_id)

	return content.Spec_id
}

func GetCloudLocation(cloudType string, nativeRegion string) GeoLocation {

	location := GeoLocation{}

	key := "/cloudtype/" + cloudType + "/region/" + nativeRegion

	fmt.Printf("[GetCloudLocation] KEY: %+v\n", key)

	keyValue, err := store.Get(key)

	if err != nil {
		cblog.Error(err)
		return location
	}

	if keyValue == nil {
		file, fileErr := os.Open("./resource/cloudlocation.csv")
		defer file.Close()
		if fileErr != nil {
			cblog.Error(fileErr)
			return location
		}

		rdr := csv.NewReader(bufio.NewReader(file))
		rows, _ := rdr.ReadAll()
		for i, row := range rows {
			keyLoc := "/cloudtype/" + rows[i][0] + "/region/" + rows[i][1]
			location.CloudType = rows[i][0]
			location.NativeRegion = rows[i][1]
			location.BriefAddr = rows[i][2]
			location.Latitude = rows[i][3]
			location.Longitude = rows[i][4]
			valLoc, _ := json.Marshal(location)
			dbErr := store.Put(string(keyLoc), string(valLoc))
			if dbErr != nil {
				cblog.Error(dbErr)
				return location
			}
			for j := range row {
				fmt.Printf("%s ", rows[i][j])
			}
			fmt.Println()
		}
	}
	keyValue, err = store.Get(key)
	if err != nil {
		cblog.Error(err)
		return location
	}

	if keyValue != nil {
		fmt.Printf("[GetCloudLocation] %+v %+v\n", keyValue.Key, keyValue.Value)
		err = json.Unmarshal([]byte(keyValue.Value), &location)
		if err != nil {
			cblog.Error(err)
			return location
		}
	}

	return location
}

/*
type vmOverview struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Config_name string      `json:"config_name"`
	Region      RegionInfo  `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	Location    GeoLocation `json:"location"`
	PublicIP    string      `json:"publicIP"`
	PublicDNS   string      `json:"publicDNS"`
	Status      string      `json:"status"`
}
*/

/* Use "SpiderVMInfo" (from Spider), instead of this.
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

	KeyValueList []common.KeyValue
}
*/
