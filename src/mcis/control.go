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

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

const ActionCreate string = "Create"
const ActionTerminate string = "Terminate"
const ActionSuspend string = "Suspend"
const ActionResume string = "Resume"
const ActionReboot string = "Reboot"
const ActionComplete string = "None"

const StatusRunning string = "Running"
const StatusSuspended string = "Suspended"
const StatusFailed string = "Failed"
const StatusTerminated string = "Terminated"
const StatusCreating string = "Creating"
const StatusSuspending string = "Suspending"
const StatusResuming string = "Resuming"
const StatusRebooting string = "Rebooting"
const StatusTerminating string = "Terminating"
const StatusComplete string = "None"

const milkywayPort string = ":1324/milkyway/"

const SshDefaultUserName01 string = "cb-user"
const SshDefaultUserName02 string = "ubuntu"
const SshDefaultUserName03 string = "root"
const SshDefaultUserName04 string = "ec2-user"

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

type TbMcisReq struct {
	Name           string    `json:"name"`
	Vm             []TbVmReq `json:"vm"`
	Placement_algo string    `json:"placement_algo"`
	Description    string    `json:"description"`
}

type TbMcisInfo struct {
	Id             string     `json:"id"`
	Name           string     `json:"name"`
	Vm             []TbVmInfo `json:"vm"`
	Placement_algo string     `json:"placement_algo"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	TargetStatus   string     `json:"targetStatus"`
	TargetAction   string     `json:"targetAction"`

	// Disabled for now
	//Vm             []vmOverview `json:"vm"`
}

type TbVmReq struct {
	Name             string   `json:"name"`
	ConnectionName   string   `json:"connectionName"`
	SpecId           string   `json:"specId"`
	ImageId          string   `json:"imageId"`
	VNetId           string   `json:"vNetId"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`
	Description      string   `json:"description"`

	/*
		//Id             string `json:"id"`
		//ConnectionName string `json:"connectionName"`

		// 1. Required by CB-Spider
		//CspVmName string `json:"cspVmName"` // will be deprecated

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

		Name           string `json:"name"`
		ConnectionName string `json:"connectionName"`
		SpecId         string `json:"specId"`
		ImageId        string `json:"imageId"`
		VNetId         string `json:"vNetId"`
		SubnetId       string `json:"subnetId"`
		//Vnic_id            string   `json:"vnic_id"`
		//Public_ip_id       string   `json:"public_ip_id"`
		SecurityGroupIds []string `json:"securityGroupIds"`
		SshKeyId         string   `json:"sshKeyId"`
		Description      string   `json:"description"`
		VmUserAccount    string   `json:"vmUserAccount"`
		VmUserPassword   string   `json:"vmUserPassword"`
	*/
}

type TbVmInfo struct {
	Id               string   `json:"id"`
	Name             string   `json:"name"`
	ConnectionName   string   `json:"connectionName"`
	SpecId           string   `json:"specId"`
	ImageId          string   `json:"imageId"`
	VNetId           string   `json:"vNetId"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`
	Description      string   `json:"description"`
	//Vnic_id            string   `json:"vnic_id"`
	//Public_ip_id       string   `json:"public_ip_id"`

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

type SshCmdResult struct { // Tumblebug
	Mcis_id string `json:"mcis_id"`
	Vm_id   string `json:"vm_id"`
	Vm_ip   string `json:"vm_ip"`
	Result  string `json:"result"`
	Err     error  `json:"err"`
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

func InstallAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	content := AgentInstallContentWrapper{}

	//install script
	cmd := "wget https://github.com/cloud-barista/cb-milkyway/raw/master/src/milkyway -O ~/milkyway; chmod +x ~/milkyway; ~/milkyway > /dev/null 2>&1 & netstat -tulpn | grep milkyway"

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
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
		userNames := []string{SshDefaultUserName01, SshDefaultUserName02, SshDefaultUserName03, SshDefaultUserName04, userName, req.User_name}
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
		common.CBLog.Error(err)
		errStr = err.Error()
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}
	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
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
			common.CBLog.Error(err)
			return BenchmarkInfoArray{}, err
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			common.CBLog.Error(err)
			return BenchmarkInfoArray{}, err
		}
		fmt.Println(string(body))

		fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			common.CBLog.Error(err)
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

// MCIS Information Managemenet

/*
func AddVmInfoToMcis(nsId string, mcisId string, vmInfoData TbVmInfo) {

	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")

}
*/

func UpdateMcisInfo(nsId string, mcisInfoData TbMcisInfo) {
	key := common.GenMcisKey(nsId, mcisInfoData.Id, "")
	val, _ := json.Marshal(mcisInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

func UpdateVmInfo(nsId string, mcisId string, vmInfoData TbVmInfo) {
	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

func ListMcisId(nsId string) []string {

	fmt.Println("[Get MCISs")
	key := "/ns/" + nsId + "/mcis"
	//fmt.Println(key)

	keyValue, _ := common.CBStore.GetList(key, true)
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

	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
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
	err := ControlMcis(nsId, mcisId, ActionTerminate)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	// delete vms info
	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
		fmt.Println(vmKey)
		err := common.CBStore.Delete(vmKey)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}
	}
	// delete mcis info
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil
}

func DelMcisVm(nsId string, mcisId string, vmId string) error {

	fmt.Println("[Delete VM] " + vmId)

	// ControlVm first
	err := ControlVm(nsId, mcisId, vmId, ActionTerminate)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished

	// delete vms info
	key := common.GenMcisKey(nsId, mcisId, vmId)
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
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
	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
		return []TbVmPriority{}, err
	}

	var vmPriorityList []TbVmPriority

	for cnt, v := range keyValue {
		fmt.Println("getRecommendList1: " + v.Key)
		err = json.Unmarshal([]byte(v.Value), &content)
		if err != nil {
			common.CBLog.Error(err)
			return []TbVmPriority{}, err
		}

		content2 := mcir.TbSpecInfo{}
		key2 := common.GenResourceKey(nsId, "spec", content.Id)

		keyValue2, err := common.CBStore.Get(key2)
		if err != nil {
			common.CBLog.Error(err)
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

func CreateMcis(nsId string, req *TbMcisReq) string {
	/*
		check, _ := CheckMcis(nsId, req.Name)

		if check {
			//temp := TbMcisInfo{}
			//err := fmt.Errorf("The mcis " + req.Name + " already exists.")
			return ""
		}
	*/

	targetAction := ActionCreate
	targetStatus := StatusRunning

	//req.Id = common.GenUuid()
	//req.Id = common.GenId(req.Name)
	mcisId := common.GenId(req.Name)
	vmRequest := req.Vm

	fmt.Println("=========================== Put createSvc")
	key := common.GenMcisKey(nsId, mcisId, "")
	//mapA := map[string]string{"name": req.Name, "description": req.Description, "status": "launching", "vm_num": req.Vm_num, "placement_algo": req.Placement_algo}
	mapA := map[string]string{
		"id":           mcisId,
		"name":         req.Name,
		"description":  req.Description,
		"status":       StatusCreating,
		"targetAction": targetAction,
		"targetStatus": targetStatus,
	}
	val, _ := json.Marshal(mapA)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	keyValue, _ := common.CBStore.Get(string(key))
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

		vmInfoData.Status = StatusCreating
		vmInfoData.TargetAction = targetAction
		vmInfoData.TargetStatus = targetStatus

		///////////
		/*
			Name              string `json:"name"`
			ConnectionName       string `json:"connectionName"`
			SpecId           string `json:"specId"`
			ImageId          string `json:"imageId"`
			VNetId           string `json:"vNetId"`
			Vnic_id           string `json:"vnic_id"`
			Security_group_id string `json:"security_group_id"`
			SshKeyId        string `json:"sshKeyId"`
			Description       string `json:"description"`
		*/

		vmInfoData.Name = k.Name
		vmInfoData.ConnectionName = k.ConnectionName
		vmInfoData.SpecId = k.SpecId
		vmInfoData.ImageId = k.ImageId
		vmInfoData.VNetId = k.VNetId
		vmInfoData.SubnetId = k.SubnetId
		//vmInfoData.Vnic_id = k.Vnic_id
		//vmInfoData.Public_ip_id = k.Public_ip_id
		vmInfoData.SecurityGroupIds = k.SecurityGroupIds
		vmInfoData.SshKeyId = k.SshKeyId
		vmInfoData.Description = k.Description

		vmInfoData.ConnectionName = k.ConnectionName

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
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	return key
}

func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo) error {
	fmt.Printf("\n[AddVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, _ := common.CBStore.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	//AddVmInfoToMcis(nsId, mcisId, *vmInfoData)
	key = common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	fmt.Printf("\n[vmInfoData]\n %+v\n", vmInfoData)

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mcisId, vmInfoData)
	if err != nil {
		vmInfoData.Status = StatusFailed
		UpdateVmInfo(nsId, mcisId, *vmInfoData)
		common.CBLog.Error(err)
		return err
	}

	//vmInfoData.PublicIP = string(*publicIPs[0])
	//vmInfoData.CspVmId = string(*instanceIds[0])
	vmInfoData.Status = StatusRunning
	vmInfoData.TargetAction = ActionComplete
	vmInfoData.TargetStatus = StatusComplete
	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

func CreateVm(nsId string, mcisId string, vmInfoData *TbVmInfo) error {

	fmt.Printf("\n\n[CreateVm(vmInfoData *TbVmInfo)]\n\n")

	switch {
	case vmInfoData.Name == "":
		err := fmt.Errorf("vmInfoData.Name is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.ImageId == "":
		err := fmt.Errorf("vmInfoData.ImageId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.ConnectionName == "":
		err := fmt.Errorf("vmInfoData.ConnectionName is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SshKeyId == "":
		err := fmt.Errorf("vmInfoData.SshKeyId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SpecId == "":
		err := fmt.Errorf("vmInfoData.SpecId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SecurityGroupIds == nil:
		err := fmt.Errorf("vmInfoData.SecurityGroupIds is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.VNetId == "":
		err := fmt.Errorf("vmInfoData.VNetId is empty")
		common.CBLog.Error(err)
		return err
	case vmInfoData.SubnetId == "":
		err := fmt.Errorf("vmInfoData.SubnetId is empty")
		common.CBLog.Error(err)
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
	tempReq.ConnectionName = vmInfoData.ConnectionName

	tempReq.ReqInfo.Name = vmInfoData.Name

	err := fmt.Errorf("")

	tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, "image", vmInfoData.ImageId)
	if tempReq.ReqInfo.ImageName == "" || err != nil {
		common.CBLog.Error(err)
		return err
	}

	tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, "spec", vmInfoData.SpecId)
	if tempReq.ReqInfo.VMSpecName == "" || err != nil {
		common.CBLog.Error(err)
		return err
	}

	tempReq.ReqInfo.VPCName = vmInfoData.VNetId //common.GetCspResourceId(nsId, "vNet", vmInfoData.VNetId)
	if tempReq.ReqInfo.VPCName == "" {
		common.CBLog.Error(err)
		return err
	}

	tempReq.ReqInfo.SubnetName = vmInfoData.SubnetId //common.GetCspResourceId(nsId, "vNet", vmInfoData.SubnetId)
	if tempReq.ReqInfo.SubnetName == "" {
		common.CBLog.Error(err)
		return err
	}

	var SecurityGroupIdsTmp []string
	for _, v := range vmInfoData.SecurityGroupIds {
		CspSgId := v //common.GetCspResourceId(nsId, "securityGroup", v)
		if CspSgId == "" {
			common.CBLog.Error(err)
			return err
		}

		SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
	}
	tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

	tempReq.ReqInfo.KeyPairName = vmInfoData.SshKeyId //common.GetCspResourceId(nsId, "sshKey", vmInfoData.SshKeyId)
	if tempReq.ReqInfo.KeyPairName == "" {
		common.CBLog.Error(err)
		return err
	}

	tempReq.ReqInfo.VMUserId = vmInfoData.VmUserAccount
	tempReq.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword

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
		common.CBLog.Error(err)
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	//reqBody, _ := ioutil.ReadAll(req.Body)
	//fmt.Println(string(reqBody))

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
		return err
	}

	vmInfoData.CspViewVmDetail = temp

	vmInfoData.VmUserAccount = temp.VMUserId
	vmInfoData.VmUserPassword = temp.VMUserPasswd

	//vmInfoData.Location = vmInfoData.Location

	//vmInfoData.Vcpu_size = vmInfoData.Vcpu_size
	//vmInfoData.Memory_size = vmInfoData.Memory_size
	//vmInfoData.Disk_size = vmInfoData.Disk_size
	//vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.Placement_algo = vmInfoData.Placement_algo

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

	configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
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
		err := common.CBStore.Put(string(Key), string(Val))
		if err != nil {
			common.CBLog.Error(err)
			return nil, nil
		}
		keyValue, _ := common.CBStore.Get(string(Key))
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
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	vmList, err := ListVmId(nsId, mcisId)
	fmt.Println("=============================================== %#v", vmList)
	if err != nil {
		common.CBLog.Error(err)
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
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	mcisTmp := TbMcisInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	UpdateMcisInfo(nsId, mcisTmp)

	if mcisStatusTmp.Status == StatusTerminating || mcisStatusTmp.Status == StatusResuming || mcisStatusTmp.Status == StatusSuspending || mcisStatusTmp.Status == StatusCreating || mcisStatusTmp.Status == StatusRebooting {
		return errors.New(action + " is not allowed for MCIS under " + mcisStatusTmp.Status)
	}
	if mcisStatusTmp.Status == StatusTerminated {
		return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
	}
	if mcisStatusTmp.Status == StatusSuspended {
		if action == ActionResume || action == ActionTerminate {
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
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return nil
	}

	switch action {
	case ActionTerminate:

		mcisTmp.TargetAction = ActionTerminate
		mcisTmp.TargetStatus = StatusTerminated
		mcisTmp.Status = StatusTerminating

	case ActionReboot:

		mcisTmp.TargetAction = ActionReboot
		mcisTmp.TargetStatus = StatusRunning
		mcisTmp.Status = StatusRebooting

	case ActionSuspend:

		mcisTmp.TargetAction = ActionSuspend
		mcisTmp.TargetStatus = StatusSuspended
		mcisTmp.Status = StatusSuspending

	case ActionResume:

		mcisTmp.TargetAction = ActionResume
		mcisTmp.TargetStatus = StatusRunning
		mcisTmp.Status = StatusResuming

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

	keyValue, _ := common.CBStore.Get(key)
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
	case ActionTerminate:

		temp.TargetAction = ActionTerminate
		temp.TargetStatus = StatusTerminated
		temp.Status = StatusTerminating

		//url = common.SPIDER_URL + "/vm/" + cspVmId + "?connection_name=" + temp.ConnectionName
		url = common.SPIDER_URL + "/vm/" + cspVmId
		method = "DELETE"
	case ActionReboot:

		temp.TargetAction = ActionReboot
		temp.TargetStatus = StatusRunning
		temp.Status = StatusRebooting

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=reboot"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=reboot"
		method = "GET"
	case ActionSuspend:

		temp.TargetAction = ActionSuspend
		temp.TargetStatus = StatusSuspended
		temp.Status = StatusSuspending

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=suspend"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=suspend"
		method = "GET"
	case ActionResume:

		temp.TargetAction = ActionResume
		temp.TargetStatus = StatusRunning
		temp.Status = StatusResuming

		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
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
	tempReq.ConnectionName = temp.ConnectionName
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
		temp.Status = StatusFailed
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
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
		errTmp = err
	}
	if errTmp != nil {
		resultTmp.Error = errTmp

		temp.Status = StatusFailed
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

	keyValue, _ := common.CBStore.Get(key)
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
	case ActionTerminate:
		//url = common.SPIDER_URL + "/vm/" + cspVmId + "?connection_name=" + temp.ConnectionName
		url = common.SPIDER_URL + "/vm/" + cspVmId
		method = "DELETE"
	case ActionReboot:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=reboot"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=reboot"
		method = "GET"
	case ActionSuspend:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=suspend"
		url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?action=suspend"
		method = "GET"
	case ActionResume:
		//url = common.SPIDER_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
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
	tempReq.ConnectionName = temp.ConnectionName
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
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
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
		common.CBLog.Error(err)
		return McisStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return McisStatusInfo{}, nil
	}

	for _, v := range vmList {
		vmStatusTmp, err := GetVmStatus(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
			vmStatusTmp.Status = StatusFailed
			return mcisStatus, err
		}

		mcisStatus.Vm = append(mcisStatus.Vm, vmStatusTmp)
	}

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{StatusFailed, StatusSuspended, StatusRunning, StatusTerminated, StatusCreating, StatusSuspending, StatusResuming, StatusRebooting, StatusTerminating, "Include-NotDefinedStatus"}
	for _, v := range mcisStatus.Vm {

		switch v.Status {
		case StatusFailed:
			statusFlag[0]++
		case StatusSuspended:
			statusFlag[1]++
		case StatusRunning:
			statusFlag[2]++
		case StatusTerminated:
			statusFlag[3]++
		case StatusCreating:
			statusFlag[4]++
		case StatusSuspending:
			statusFlag[5]++
		case StatusResuming:
			statusFlag[6]++
		case StatusRebooting:
			statusFlag[7]++
		case StatusTerminating:
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
		if v.TargetStatus != StatusComplete {
			isDone = false
		}
	}
	if isDone {
		mcisStatus.TargetAction = ActionComplete
		mcisStatus.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
		mcisTmp.TargetStatus = StatusComplete
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
			CspVmName string // will be deprecated
			PublicIP  string
		}
	*/

	fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
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
	keyValue, _ = common.CBStore.Get(key)
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

	url := common.SPIDER_URL + "/vmstatus/" + cspVmId // + "?connection_name=" + temp.ConnectionName
	method := "GET"

	//fmt.Println("url: " + url)

	type VMStatusReqInfo struct {
		ConnectionName string
	}
	tempReq := VMStatusReqInfo{}
	tempReq.ConnectionName = temp.ConnectionName
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	errorInfo := TbVmStatusInfo{}
	errorInfo.Status = StatusFailed

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
		statusResponseTmp.Status = StatusCreating
	} else if statusResponseTmp.Status == "Running" {
		statusResponseTmp.Status = StatusRunning
	} else if statusResponseTmp.Status == "Suspending" {
		statusResponseTmp.Status = StatusSuspending
	} else if statusResponseTmp.Status == "Suspended" {
		statusResponseTmp.Status = StatusSuspended
	} else if statusResponseTmp.Status == "Resuming" {
		statusResponseTmp.Status = StatusResuming
	} else if statusResponseTmp.Status == "Rebooting" {
		statusResponseTmp.Status = StatusRebooting
	} else if statusResponseTmp.Status == "Terminating" {
		statusResponseTmp.Status = StatusTerminating
	} else if statusResponseTmp.Status == "Terminated" {
		statusResponseTmp.Status = StatusTerminated
	} else {
		statusResponseTmp.Status = "statusUndefined"
	}

	//Correct undefined status using TargetAction
	if vmStatusTmp.TargetAction == ActionCreate {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = StatusCreating
		}
	}
	if vmStatusTmp.TargetAction == ActionTerminate {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = StatusTerminated
		}
		if statusResponseTmp.Status == StatusSuspending {
			statusResponseTmp.Status = StatusTerminated
		}
	}
	if vmStatusTmp.TargetAction == ActionResume {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = StatusResuming
		}
		if statusResponseTmp.Status == StatusCreating {
			statusResponseTmp.Status = StatusResuming
		}

	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if vmStatusTmp.TargetAction == ActionReboot {
		if statusResponseTmp.Status == "statusUndefined" {
			statusResponseTmp.Status = StatusRebooting
		}
		if statusResponseTmp.Status == StatusSuspending || statusResponseTmp.Status == StatusSuspended || statusResponseTmp.Status == StatusCreating || statusResponseTmp.Status == StatusResuming {
			statusResponseTmp.Status = StatusRebooting
		}
	}

	// End of Temporal CODE.

	vmStatusTmp.Status = statusResponseTmp.Status
	if err != nil {
		common.CBLog.Error(err)
		vmStatusTmp.Status = StatusFailed
	}

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		vmStatusTmp.TargetStatus = StatusComplete
		vmStatusTmp.TargetAction = ActionComplete
	}

	return vmStatusTmp, nil

}

func UpdateVmPublicIp(nsId string, mcisId string, vmInfoData TbVmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, vmInfoData.Id)

	if err != nil {
		common.CBLog.Error(err)
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

	keyValue, _ := common.CBStore.Get(key)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("\n\n[Calling SPIDER]START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	cspVmId := temp.CspViewVmDetail.IId.NameId

	url := common.SPIDER_URL + "/vm/" + cspVmId // + "?connection_name=" + temp.ConnectionName
	method := "GET"

	type VMStatusReqInfo struct {
		ConnectionName string
	}
	tempReq := VMStatusReqInfo{}
	tempReq.ConnectionName = temp.ConnectionName
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	errorInfo := TbVmStatusInfo{}
	errorInfo.Status = StatusFailed

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
		SshKeyId string `json:"sshKeyId"`
	}

	fmt.Println("[GetVmIp]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SshKeyId)

	sshKey := common.GenResourceKey(nsId, "sshKey", content.SshKeyId)
	keyValue, _ = common.CBStore.Get(sshKey)
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

	keyValue, _ := common.CBStore.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.PublicIP)

	return content.PublicIP
}

func GetVmSpecId(nsId string, mcisId string, vmId string) string {

	var content struct {
		SpecId string `json:"specId"`
	}

	fmt.Println("[getVmSpecID]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)

	keyValue, _ := common.CBStore.Get(key)

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SpecId)

	return content.SpecId
}

func GetCloudLocation(cloudType string, nativeRegion string) GeoLocation {

	location := GeoLocation{}

	key := "/cloudtype/" + cloudType + "/region/" + nativeRegion

	fmt.Printf("[GetCloudLocation] KEY: %+v\n", key)

	keyValue, err := common.CBStore.Get(key)

	if err != nil {
		common.CBLog.Error(err)
		return location
	}

	if keyValue == nil {
		file, fileErr := os.Open("./resource/cloudlocation.csv")
		defer file.Close()
		if fileErr != nil {
			common.CBLog.Error(fileErr)
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
			dbErr := common.CBStore.Put(string(keyLoc), string(valLoc))
			if dbErr != nil {
				common.CBLog.Error(dbErr)
				return location
			}
			for j := range row {
				fmt.Printf("%s ", rows[i][j])
			}
			fmt.Println()
		}
	}
	keyValue, err = common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return location
	}

	if keyValue != nil {
		fmt.Printf("[GetCloudLocation] %+v %+v\n", keyValue.Key, keyValue.Value)
		err = json.Unmarshal([]byte(keyValue.Value), &location)
		if err != nil {
			common.CBLog.Error(err)
			return location
		}
	}

	return location
}

/*
type vmOverview struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	ConnectionName string      `json:"connectionName"`
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
