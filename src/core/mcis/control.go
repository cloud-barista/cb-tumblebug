package mcis

import (
	"errors"

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

	"math/rand"
	"reflect"

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
)

const (
	// ActionCreate is const for Create
	ActionCreate string = "Create"

	// ActionTerminate is const for Terminate
	ActionTerminate string = "Terminate"

	// ActionSuspend is const for Suspend
	ActionSuspend string = "Suspend"

	// ActionResume is const for Resume
	ActionResume string = "Resume"

	// ActionReboot is const for Reboot
	ActionReboot string = "Reboot"

	// ActionComplete is const for Complete
	ActionComplete string = "None"
)
const (
	// StatusRunning is const for Running
	StatusRunning string = "Running"

	// StatusSuspended is const for Suspended
	StatusSuspended string = "Suspended"

	// StatusFailed is const for Failed
	StatusFailed string = "Failed"

	// StatusTerminated is const for Terminated
	StatusTerminated string = "Terminated"

	// StatusCreating is const for Creating
	StatusCreating string = "Creating"

	// StatusSuspending is const for Suspending
	StatusSuspending string = "Suspending"

	// StatusResuming is const for Resuming
	StatusResuming string = "Resuming"

	// StatusRebooting is const for Rebooting
	StatusRebooting string = "Rebooting"

	// StatusTerminating is const for Terminating
	StatusTerminating string = "Terminating"

	// StatusUndefined is const for Undefined
	StatusUndefined string = "Undefined"

	// StatusComplete is const for Complete
	StatusComplete string = "None"
)

const milkywayPort string = ":1324/milkyway/"

const labelAutoGen string = "AutoGen"

// sshDefaultUserName is array for temporal constants
var sshDefaultUserName = []string{"cb-user", "ubuntu", "root", "ec2-user"}

// SpiderVMReqInfoWrapper is struct from CB-Spider (VMHandler.go) for wrapping SpiderVMInfo
type SpiderVMReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVMInfo
}

// SpiderVMInfo is struct from CB-Spider for VM information
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
	SSHAccessPoint    string
	KeyValueList      []common.KeyValue
}

// RegionInfo is struct from CB-Spider for region information
type RegionInfo struct {
	Region string
	Zone   string
}

// TbMcisReq is sturct for requirements to create MCIS
type TbMcisReq struct {
	Name string `json:"name" validate:"required"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"yes" default:"yes" enums:"yes,no"` // yes or no

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label" example:"custom tag" default:"no"`

	PlacementAlgo string `json:"placementAlgo"`
	Description   string `json:"description"`

	Vm []TbVmReq `json:"vm" validate:"required"`
}

// TbMcisReqStructLevelValidation is func to validate fields in TbMcisReqStruct
func TbMcisReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbMcisReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", "NotObeyingNamingConvention", "")
	}
}

// TbMcisInfo is struct for MCIS info
type TbMcisInfo struct {
	Id           string          `json:"id"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"yes" default:"yes" enums:"yes,no"` // yes or no

	// Label is for describing the mcis in a keyword (any string can be used)
	Label string `json:"label"`

	PlacementAlgo string     `json:"placementAlgo"`
	Description   string     `json:"description"`
	Vm            []TbVmInfo `json:"vm"`
}

// TbVmReq is struct to get requirements to create a new server instance
type TbVmReq struct {
	// VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name" validate:"required"`

	// if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
	VmGroupSize string `json:"vmGroupSize" example:"3" default:""`

	Label string `json:"label"`

	Description string `json:"description"`

	ConnectionName   string   `json:"connectionName" validate:"required"`
	SpecId           string   `json:"specId" validate:"required"`
	ImageId          string   `json:"imageId" validate:"required"`
	VNetId           string   `json:"vNetId" validate:"required"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds" validate:"required"`
	SshKeyId         string   `json:"sshKeyId" validate:"required"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`
}

// TbVmReqStructLevelValidation is func to validate fields in TbVmReqStruct
func TbVmReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbVmReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", "NotObeyingNamingConvention", "")
	}
}

// TbVmGroupInfo is struct to define an object that includes homogeneous VMs
type TbVmGroupInfo struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	VmId        []string `json:"vmId"`
	VmGroupSize string   `json:"vmGroupSize"`
}

// TbVmInfo is struct to define a server instance object
type TbVmInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	// defined if the VM is in a group
	VmGroupId string `json:"vmGroupId"`

	Location GeoLocation `json:"location"`

	// Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	Label       string `json:"label"`
	Description string `json:"description"`

	Region      RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP    string     `json:"publicIP"`
	SSHPort     string     `json:"sshPort"`
	PublicDNS   string     `json:"publicDNS"`
	PrivateIP   string     `json:"privateIP"`
	PrivateDNS  string     `json:"privateDNS"`
	VMBootDisk  string     `json:"vmBootDisk"` // ex) /dev/sda1
	VMBlockDisk string     `json:"vmBlockDisk"`

	ConnectionName   string   `json:"connectionName"`
	SpecId           string   `json:"specId"`
	ImageId          string   `json:"imageId"`
	VNetId           string   `json:"vNetId"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`

	CspViewVmDetail SpiderVMInfo `json:"cspViewVmDetail"`
}

// GeoLocation is struct for geographical location
type GeoLocation struct {
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
	BriefAddr    string `json:"briefAddr"`
	CloudType    string `json:"cloudType"`
	NativeRegion string `json:"nativeRegion"`
}

// StatusCountInfo is struct to count the number of VMs in each status. ex: Running=4, Suspended=8.
type StatusCountInfo struct {

	// CountTotal is for Total VMs
	CountTotal int `json:"countTotal"`

	// CountCreating is for counting Creating
	CountCreating int `json:"countCreating"`

	// CountRunning is for counting Running
	CountRunning int `json:"countRunning"`

	// CountFailed is for counting Failed
	CountFailed int `json:"countFailed"`

	// CountSuspended is for counting Suspended
	CountSuspended int `json:"countSuspended"`

	// CountRebooting is for counting Rebooting
	CountRebooting int `json:"countRebooting"`

	// CountTerminated is for counting Terminated
	CountTerminated int `json:"countTerminated"`

	// CountSuspending is for counting Suspending
	CountSuspending int `json:"countSuspending"`

	// CountResuming is for counting Resuming
	CountResuming int `json:"countResuming"`

	// CountTerminating is for counting Terminating
	CountTerminating int `json:"countTerminating"`

	// CountUndefined is for counting Undefined
	CountUndefined int `json:"countUndefined"`
}

// McisStatusInfo is struct to define simple information of MCIS with updated status of all VMs
type McisStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no

	MasterVmId    string `json:"masterVmId" example:"vm-asiaeast1-cb-01"`
	MasterIp      string `json:"masterIp" example:"32.201.134.113"`
	MasterSSHPort string `json:"masterSSHPort"`

	Vm []TbVmStatusInfo `json:"vm"`
}

// TbVmStatusInfo is to define simple information of VM with updated status
type TbVmStatusInfo struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	CspVmId string `json:"cspVmId"`

	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`
	NativeStatus string `json:"nativeStatus"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	PublicIp  string `json:"publicIp"`
	PrivateIp string `json:"privateIp"`
	SSHPort   string `json:"sshPort"`

	Location GeoLocation `json:"location"`
}

// McisCmdReq is struct for remote command
type McisCmdReq struct {
	UserName string `json:"userName" example:"cb-user" default:""`
	Command  string `json:"command" validate:"required" example:"sudo apt-get install ..."`
}

// TbMcisCmdReqStructLevelValidation is func to validate fields in McisCmdReq
func TbMcisCmdReqStructLevelValidation(sl validator.StructLevel) {

	// u := sl.Current().Interface().(McisCmdReq)

	// err := common.CheckString(u.Command)
	// if err != nil {
	// 	// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
	// 	sl.ReportError(u.Command, "command", "Command", "NotObeyingNamingConvention", "")
	// }
}

// McisRecommendReq is struct for McisRecommendReq
type McisRecommendReq struct {
	VmReq          []TbVmRecommendReq `json:"vmReq"`
	PlacementAlgo  string             `json:"placementAlgo"`
	PlacementParam []common.KeyValue  `json:"placementParam"`
	MaxResultNum   string             `json:"maxResultNum"`
}

// TbVmRecommendReq is struct for TbVmRecommendReq
type TbVmRecommendReq struct {
	RequestName  string `json:"requestName"`
	MaxResultNum string `json:"maxResultNum"`

	VcpuSize   string `json:"vcpuSize"`
	MemorySize string `json:"memorySize"`
	DiskSize   string `json:"diskSize"`
	//Disk_type   string `json:"disk_type"`

	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

// TbVmPriority is struct for TbVmPriority
type TbVmPriority struct {
	Priority string          `json:"priority"`
	VmSpec   mcir.TbSpecInfo `json:"vmSpec"`
}

// TbVmRecommendInfo is struct for TbVmRecommendInfo
type TbVmRecommendInfo struct {
	VmReq          TbVmRecommendReq  `json:"vmReq"`
	VmPriority     []TbVmPriority    `json:"vmPriority"`
	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

// SshCmdResult is struct for SshCmd Result
type SshCmdResult struct { // Tumblebug
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
	Err    error  `json:"err"`
}

// AgentInstallContentWrapper ...
type AgentInstallContentWrapper struct {
	ResultArray []AgentInstallContent `json:"resultArray"`
}

// AgentInstallContent ...
type AgentInstallContent struct {
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
}

// InstallAgentToMcis is func to install milkyway agents in MCIS
func InstallAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := AgentInstallContentWrapper{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

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

		vmId := v
		vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

		//cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.UserName
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }

		// find vaild username
		userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)

		fmt.Println("")
		fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName:" + userName)
		fmt.Println("[CMD] " + cmd)
		fmt.Println("")

		// Avoid RunSSH to not ready VM
		if err != nil {
			wg.Add(1)
			go RunSSHAsync(&wg, vmId, vmIp, sshPort, userName, sshKey, cmd, &resultArray)
		} else {
			common.CBLog.Error(err)
			sshResultTmp := SshCmdResult{}
			sshResultTmp.McisId = mcisId
			sshResultTmp.VmId = vmId
			sshResultTmp.VmIp = vmIp
			sshResultTmp.Result = err.Error()
			sshResultTmp.Err = err
		}

	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.McisId = mcisId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Result
		content.ResultArray = append(content.ResultArray, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return content, nil

}

// SpecBenchmarkInfo is struct for SpecBenchmarkInfo
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

// BenchmarkInfo is struct for BenchmarkInfo
type BenchmarkInfo struct {
	Result      string          `json:"result"`
	Unit        string          `json:"unit"`
	Desc        string          `json:"desc"`
	Elapsed     string          `json:"elapsed"`
	SpecId      string          `json:"specid"`
	ResultArray []BenchmarkInfo `json:"resultarray"` // struct-element cycle ?
}

// BenchmarkInfoArray is struct for BenchmarkInfoArray
type BenchmarkInfoArray struct {
	ResultArray []BenchmarkInfo `json:"resultarray"`
}

// BenchmarkReq is struct for BenchmarkReq
type BenchmarkReq struct {
	Host string `json:"host"`
	Spec string `json:"spec"`
}

// MultihostBenchmarkReq is struct for MultihostBenchmarkReq
type MultihostBenchmarkReq struct {
	Multihost []BenchmarkReq `json:"multihost"`
}

// CallMilkyway is func to call milkyway agents
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
			vmIpTmp, _ := GetVmIp(nsId, mcisId, vmIdTmp)
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

	fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	//benchInfoTmp := BenchmarkInfo{}
	resultTmp := BenchmarkInfo{}
	err2 := json.Unmarshal(body, &resultTmp)
	if err2 != nil {
		common.CBLog.Error(err2)
		errStr = err2.Error()
	}
	//benchInfoTmp.ResultArray =  resultTmp.ResultArray
	if errStr != "" {
		resultTmp.Result = errStr
	}
	resultTmp.SpecId = GetVmSpecId(nsId, mcisId, vmId)
	results.ResultArray = append(results.ResultArray, resultTmp)
}

// CoreGetAllBenchmark is func to get alls Benchmarks
func CoreGetAllBenchmark(nsId string, mcisId string, host string) (*BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &BenchmarkInfoArray{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	target := host

	action := "all"
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

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
		//mapError := map[string]string{"message": "Benchmark Error"}
		//return c.JSON(http.StatusFailedDependency, &mapError)
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// CoreGetBenchmark is func to get Benchmark
func CoreGetBenchmark(nsId string, mcisId string, action string, host string) (*BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &BenchmarkInfoArray{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	target := host

	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	content := BenchmarkInfoArray{}

	vaildActions := "install init cpus cpum memR memW fioR fioW dbR dbW rtt mrtt clean"

	fmt.Println("[Benchmark] " + action)
	if strings.Contains(vaildActions, action) {
		content, err = BenchmarkAction(nsId, mcisId, action, option)
	} else {
		//mapA := map[string]string{"message": "Not available action"}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return nil, fmt.Errorf("Not available action")
	}

	if err != nil {
		//mapError := map[string]string{"message": "Benchmark Error"}
		//return c.JSON(http.StatusFailedDependency, &mapError)
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// BenchmarkAction is func to action Benchmark
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
		vmIp, _ := GetVmIp(nsId, mcisId, vmId)

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

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
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
				common.CBLog.Error(err2)
				return BenchmarkInfoArray{}, err2
			}
			//benchInfoTmp.ResultArray =  resultTmp.ResultArray
			results.ResultArray = append(results.ResultArray, resultTmp)

		} else{
			resultTmp := BenchmarkInfo{}
			err2 := json.Unmarshal(body, &resultTmp)
			if err2 != nil {
				common.CBLog.Error(err2)
				return BenchmarkInfoArray{}, err2
			}
			results.ResultArray = append(results.ResultArray, resultTmp)
		}

	}

	return results, nil

}
*/

// MCIS Information Managemenet

// UpdateMcisInfo is func to update MCIS Info (without VM info in MCIS)
func UpdateMcisInfo(nsId string, mcisInfoData TbMcisInfo) {

	mcisInfoData.Vm = nil

	key := common.GenMcisKey(nsId, mcisInfoData.Id, "")

	// Check existence of the key. If no key, no update.
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		return
	}

	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	if !reflect.DeepEqual(mcisTmp, mcisInfoData) {
		val, _ := json.Marshal(mcisInfoData)
		err = common.CBStore.Put(string(key), string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

// UpdateVmInfo is func to update VM Info
func UpdateVmInfo(nsId string, mcisId string, vmInfoData TbVmInfo) {
	key := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)

	// Check existence of the key. If no key, no update.
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		return
	}

	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)

	if !reflect.DeepEqual(vmTmp, vmInfoData) {
		val, _ := json.Marshal(vmInfoData)
		err = common.CBStore.Put(string(key), string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
	}

	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}

// ListMcisId is func to list MCIS ID
func ListMcisId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	// fmt.Println("[ListMcisId]")
	var mcisList []string

	// Check MCIS exists
	key := common.GenMcisKey(nsId, "", "")
	key += "/"

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/mcis/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "mcis/"))
			// prevent malformed key (if key for mcis id includes '/', the key does not represent MCIS ID)
			if !strings.Contains(trimmedString, "/") {
				mcisList = append(mcisList, trimmedString)
			}
		}
	}

	return mcisList, nil
}

// ListVmId is func to list VM IDs
func ListVmId(nsId string, mcisId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	// fmt.Println("[ListVmId]")
	var vmList []string

	// Check MCIS exists
	key := common.GenMcisKey(nsId, mcisId, "")
	key += "/"

	_, err = common.CBStore.Get(key)
	if err != nil {
		fmt.Println("[Not found] " + mcisId)
		common.CBLog.Error(err)
		return vmList, err
	}

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vm/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "vm/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				vmList = append(vmList, trimmedString)
			}
		}
	}

	return vmList, nil

}

// ListVmGroupId is func to return list of VmGroups in a given MCIS
func ListVmGroupId(nsId string, mcisId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[ListVmGroupId]")
	key := common.GenMcisKey(nsId, mcisId, "")
	key += "/"

	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	var vmGroupList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vmgroup/") {
			trimmedString := strings.TrimPrefix(v.Key, (key + "vmgroup/"))
			// prevent malformed key (if key for vm id includes '/', the key does not represent VM ID)
			if !strings.Contains(trimmedString, "/") {
				vmGroupList = append(vmGroupList, trimmedString)
			}
		}
	}
	return vmGroupList, nil
}

// DelMcis is func to delete MCIS object
func DelMcis(nsId string, mcisId string, option string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err
	}

	fmt.Println("[Delete MCIS] " + mcisId)

	// // ControlMcis first
	// err = ControlMcisAsync(nsId, mcisId, ActionTerminate)
	// if err != nil {
	// 	common.CBLog.Error(err)
	// 	if option != "force" {
	// 		return err
	// 	}
	// }
	// // for deletion, need to wait until termination is finished
	// // Sleep for 5 seconds
	// fmt.Printf("\n\n[Info] Sleep for 5 seconds for safe MCIS-VMs termination.\n\n")
	// time.Sleep(5 * time.Second)

	// Check MCIS status is Terminated so that approve deletion
	mcisStatus, _ := GetMcisStatus(nsId, mcisId)
	if mcisStatus == nil {
		err := fmt.Errorf("MCIS " + mcisId + " status nil, Deletion is not allowed (use option=force for force deletion)")
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}
	// Check MCIS status is Terminated (not Partial)
	if !(!strings.Contains(mcisStatus.Status, "Partial-") && (strings.Contains(mcisStatus.Status, StatusTerminated) || strings.Contains(mcisStatus.Status, StatusUndefined) || strings.Contains(mcisStatus.Status, StatusFailed))) {
		err := fmt.Errorf("MCIS " + mcisId + " is " + mcisStatus.Status + " and not " + StatusTerminated + "/" + StatusUndefined + "/" + StatusFailed + ", Deletion is not allowed (use option=force for force deletion)")
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}

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

		// get vm info
		vmInfo, err := GetVmObject(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		err = common.CBStore.Delete(vmKey)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, vmKey)
		mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, vmKey)

		for _, v2 := range vmInfo.SecurityGroupIds {
			mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrDelete, vmKey)
		}
	}

	// delete vm group info
	vmGroupList, err := ListVmGroupId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	for _, v := range vmGroupList {
		vmGroupKey := common.GenMcisVmGroupKey(nsId, mcisId, v)
		fmt.Println(vmGroupKey)
		err := common.CBStore.Delete(vmGroupKey)
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

// DelMcisVm is func to delete VM object
func DelMcisVm(nsId string, mcisId string, vmId string, option string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err
	}

	fmt.Println("[Delete VM] " + vmId)

	// ControlVm first
	err = ControlVm(nsId, mcisId, vmId, ActionTerminate)

	if err != nil {
		common.CBLog.Error(err)
		if option != "force" {
			return err
		}
	}
	// for deletion, need to wait until termination is finished
	// Sleep for 5 seconds
	fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe VM termination.\n\n")
	time.Sleep(5 * time.Second)

	// get vm info
	vmInfo, _ := GetVmObject(nsId, mcisId, vmId)

	// delete vms info
	key := common.GenMcisKey(nsId, mcisId, vmId)
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfo.ImageId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfo.SpecId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfo.SshKeyId, common.StrDelete, key)
	mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfo.VNetId, common.StrDelete, key)

	for _, v2 := range vmInfo.SecurityGroupIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrDelete, key)
	}

	return nil
}

// GetRecommendList is func to get recommendation list
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
	keyValue = cbstore_utils.GetChildList(keyValue, key)
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
		key2 := common.GenResourceKey(nsId, common.StrSpec, content.Id)

		keyValue2, err := common.CBStore.Get(key2)
		if err != nil {
			common.CBLog.Error(err)
			return []TbVmPriority{}, err
		}
		json.Unmarshal([]byte(keyValue2.Value), &content2)
		content2.Id = content.Id

		vmPriorityTmp := TbVmPriority{}
		vmPriorityTmp.Priority = strconv.Itoa(cnt)
		vmPriorityTmp.VmSpec = content2
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList, err

	//requires error handling

}

// MCIS Control

// HandleMcisAction is func to handle actions to MCIS
func HandleMcisAction(nsId string, mcisId string, action string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err.Error(), err
	}

	fmt.Println("[Get MCIS requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionSuspend)
		if err != nil {
			//mapA := map[string]string{"message": err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", err
		}

		//mapA := map[string]string{"message": "Suspending the MCIS"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Suspending the MCIS", nil

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionResume)
		if err != nil {
			//mapA := map[string]string{"message": err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", err
		}

		//mapA := map[string]string{"message": "Resuming the MCIS"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Resuming the MCIS", nil

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		err := ControlMcisAsync(nsId, mcisId, ActionReboot)
		if err != nil {
			//mapA := map[string]string{"message": err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", err
		}

		//mapA := map[string]string{"message": "Rebooting the MCIS"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Rebooting the MCIS", nil

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		//fmt.Println("len(vmList) %d ", len(vmList))
		if len(vmList) == 0 {
			//mapA := map[string]string{"message": "No VM to terminate in the MCIS"}
			//return c.JSON(http.StatusOK, &mapA)
			return "No VM to terminate in the MCIS", nil
		}

		/*
			for _, v := range vmList {
				ControlVm(nsId, mcisId, v, ActionTerminate)
			}
		*/
		err = ControlMcisAsync(nsId, mcisId, ActionTerminate)
		if err != nil {
			return "", err
		}

		return "Terminating the MCIS", nil

	} else if action == "refine" { //refine delete VMs in StatusFailed or StatusUndefined
		fmt.Println("[terminate MCIS]")

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		if len(vmList) == 0 {
			return "No VM in the MCIS", nil
		}

		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return "", err
		}

		for _, v := range mcisStatus.Vm {

			// Remove VMs in StatusFailed or StatusUndefined
			fmt.Println("[vmInfo.Status]", v.Status)
			if v.Status == StatusFailed || v.Status == StatusUndefined {
				// Delete VM sequentially for safety (for performance, need to use goroutine)
				err := DelMcisVm(nsId, mcisId, v.Id, "force")
				if err != nil {
					common.CBLog.Error(err)
					return "", err
				}
			}
		}

		return "Refined the MCIS", nil

	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// GetMcisInfo is func to return MCIS information with the current status update
func GetMcisInfo(nsId string, mcisId string) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &TbMcisInfo{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	mcisObj, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	// common.PrintJsonPretty(mcisObj)

	mcisStatus, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	// common.PrintJsonPretty(mcisStatus)

	mcisObj.Status = mcisStatus.Status
	mcisObj.StatusCount = mcisStatus.StatusCount

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	for num := range vmList {
		//fmt.Println("[GetMcisInfo compare two VMs]")
		//common.PrintJsonPretty(mcisObj.Vm[num])
		//common.PrintJsonPretty(mcisStatus.Vm[num])

		mcisObj.Vm[num].Status = mcisStatus.Vm[num].Status
		mcisObj.Vm[num].TargetStatus = mcisStatus.Vm[num].TargetStatus
		mcisObj.Vm[num].TargetAction = mcisStatus.Vm[num].TargetAction
	}

	return &mcisObj, nil
}

// CoreGetAllMcis is func to get all MCIS objects
func CoreGetAllMcis(nsId string, option string) ([]TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/*
		var content struct {
			//Name string     `json:"name"`
			Mcis []mcis.TbMcisInfo `json:"mcis"`
		}
	*/
	// content := RestGetAllMcisResponse{}

	Mcis := []TbMcisInfo{}

	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range mcisList {

		key := common.GenMcisKey(nsId, v, "")
		//fmt.Println(key)
		keyValue, _ := common.CBStore.Get(key)
		if keyValue == nil {
			//mapA := map[string]string{"message": "Cannot find " + key}
			//return c.JSON(http.StatusOK, &mapA)
			return nil, fmt.Errorf("in CoreGetAllMcis() mcis loop; Cannot find " + key)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := TbMcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisId := v
		mcisTmp.Id = mcisId

		if option == "status" || option == "simple" {
			//get current mcis status
			mcisStatus, err := GetMcisStatus(nsId, mcisId)
			if err != nil {
				common.CBLog.Error(err)
				return nil, err
			}
			mcisTmp.Status = mcisStatus.Status
		} else {
			//Set current mcis status with NullStr
			mcisTmp.Status = ""
		}

		// The cases with id, status, or others. except simple
		if option != "simple" {
			vmList, err := ListVmId(nsId, mcisId)
			if err != nil {
				common.CBLog.Error(err)
				return nil, err
			}

			for _, v1 := range vmList {
				vmKey := common.GenMcisKey(nsId, mcisId, v1)
				//fmt.Println(vmKey)
				vmKeyValue, _ := common.CBStore.Get(vmKey)
				if vmKeyValue == nil {
					//mapA := map[string]string{"message": "Cannot find " + key}
					//return c.JSON(http.StatusOK, &mapA)
					return nil, fmt.Errorf("in CoreGetAllMcis() vm loop; Cannot find " + vmKey)
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
						common.CBLog.Error(err)
					}
					vmTmp.Status = vmStatusInfoTmp.Status
				} else {
					//Set current vm status with NullStr
					vmTmp.Status = ""
				}

				mcisTmp.Vm = append(mcisTmp.Vm, vmTmp)
			}
		}

		Mcis = append(Mcis, mcisTmp)
	}

	return Mcis, nil
}

// CoreDelAllMcis is func to delete all MCIS objects
func CoreDelAllMcis(nsId string, option string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	if len(mcisList) == 0 {
		//mapA := map[string]string{"message": "No MCIS to delete"}
		//return c.JSON(http.StatusOK, &mapA)
		return "No MCIS to delete", nil
	}

	for _, v := range mcisList {
		err := DelMcis(nsId, v, option)
		if err != nil {
			common.CBLog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete All MCISs"}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", fmt.Errorf("Failed to delete All MCISs")
		}
	}

	return "All MCISs has been deleted", nil
}

// CorePostMcisRecommend is func to command to all VMs in MCIS with SSH
func CorePostMcisRecommend(nsId string, req *McisRecommendReq) ([]TbVmRecommendInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/*
		var content struct {
			//VmReq          []TbVmRecommendReq    `json:"vmReq"`
			VmRecommend    []mcis.TbVmRecommendInfo `json:"vmRecommend"`
			PlacementAlgo  string                   `json:"placementAlgo"`
			PlacementParam []common.KeyValue        `json:"placementParam"`
		}
	*/
	//content := RestPostMcisRecommendResponse{}
	//content.VmReq = req.VmReq
	//content.PlacementAlgo = req.PlacementAlgo
	//content.PlacementParam = req.PlacementParam

	VmRecommend := []TbVmRecommendInfo{}

	vmList := req.VmReq

	for i, v := range vmList {
		vmTmp := TbVmRecommendInfo{}
		//vmTmp.RequestName = v.RequestName
		vmTmp.VmReq = req.VmReq[i]
		vmTmp.PlacementAlgo = v.PlacementAlgo
		vmTmp.PlacementParam = v.PlacementParam

		var err error
		vmTmp.VmPriority, err = GetRecommendList(nsId, v.VcpuSize, v.MemorySize, v.DiskSize)

		if err != nil {
			common.CBLog.Error(err)
			//mapA := map[string]string{"message": "Failed to recommend MCIS"}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return nil, fmt.Errorf("Failed to recommend MCIS")
		}

		VmRecommend = append(VmRecommend, vmTmp)
	}

	return VmRecommend, nil
}

// RemoteCommandToMcisVm is func to command to a VM in MCIS by SSH
func RemoteCommandToMcisVm(nsId string, mcisId string, vmId string, req *McisCmdReq) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return "", err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return "", err
	}

	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

	//fmt.Printf("[vmIp] " +vmIp)

	//sshKey := req.SshKey
	cmd := req.Command

	// find vaild username
	userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)

	if err != nil || userName == "" {
		return "", fmt.Errorf("Not found: valid ssh username, " + err.Error())
	}

	fmt.Println("")
	fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName:" + userName)
	fmt.Println("[CMD] " + cmd)
	fmt.Println("")

	if result, err := RunSSH(vmIp, sshPort, userName, sshKey, cmd); err != nil {
		//return c.JSON(http.StatusInternalServerError, err)
		return "", err
	} else {
		//response := echo.Map{}
		//response["result"] = *result
		//response := RestPostCmdMcisVmResponse{Result: *result}
		//return c.JSON(http.StatusOK, response)
		return *result, nil
	}
}

// RemoteCommandToMcis is func to command to all VMs in MCIS by SSH
func RemoteCommandToMcis(nsId string, mcisId string, req *McisCmdReq) ([]SshCmdResult, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			temp := []SshCmdResult{}
			return temp, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		temp := []SshCmdResult{}
		return temp, err
	}

	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := []SshCmdResult{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	/*
		type contentSub struct {
			McisId string `json:"mcisId"`
			VmId   string `json:"vmId"`
			VmIp   string `json:"vmIp"`
			Result  string `json:"result"`
		}
		var content struct {
			ResultArray []contentSub `json:"resultArray"`
		}
	*/
	//content := RestPostCmdMcisResponseWrapper{}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	//goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	for _, v := range vmList {

		vmId := v
		vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

		cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.UserName
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }
		// find vaild username
		userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)
		// Eventhough VerifySshUserName is not complete, Try RunSSH
		// With RunSSH, error will be checked again
		if err == nil {
			// Just logging the error (but it is net a faultal )
			common.CBLog.Info(err)
		}
		fmt.Println("")
		fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName:" + userName)
		fmt.Println("[CMD] " + cmd)
		fmt.Println("")

		wg.Add(1)
		go RunSSHAsync(&wg, vmId, vmIp, sshPort, userName, sshKey, cmd, &resultArray)

	}
	wg.Wait() //goroutine sync wg

	return resultArray, nil
}

// CorePostMcisVm is func to post (create) McisVm
func CorePostMcisVm(nsId string, mcisId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	err = common.CheckString(vmInfoData.Name)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckVm(nsId, mcisId, vmInfoData.Name)

	if check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmInfoData.Name + " already exists.")
		return temp, err
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	vmInfoData.Id = vmInfoData.Name
	vmInfoData.PublicIP = "Not assigned yet"
	vmInfoData.PublicDNS = "Not assigned yet"
	vmInfoData.TargetAction = targetAction
	vmInfoData.TargetStatus = targetStatus
	vmInfoData.Status = StatusCreating

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	go AddVmToMcis(&wg, nsId, mcisId, vmInfoData)

	wg.Wait()

	vmStatus, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)
	if err != nil {
		//mapA := map[string]string{"message": "Cannot find " + common.GenMcisKey(nsId, mcisId, "")}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, fmt.Errorf("Cannot find " + common.GenMcisKey(nsId, mcisId, vmInfoData.Id))
	}

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

	// Install CB-Dragonfly monitoring agent

	mcisTmp, _ := GetMcisObject(nsId, mcisId)

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisId, mcisTmp.InstallMonAgent)

	if mcisTmp.InstallMonAgent != "no" {

		// Sleep for 20 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(20 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "ubuntu" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}

	return vmInfoData, nil
}

// CorePostMcisGroupVm is func for a wrapper for CreateMcisGroupVm
func CorePostMcisGroupVm(nsId string, mcisId string, vmReq *TbVmReq) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(vmReq)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return nil, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return nil, err
	}

	content, err := CreateMcisGroupVm(nsId, mcisId, vmReq)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	return content, nil
}

// CreateMcisGroupVm is func to create MCIS groupVM
func CreateMcisGroupVm(nsId string, mcisId string, vmRequest *TbVmReq) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(vmRequest)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return nil, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return nil, err
	}

	mcisTmp, err := GetMcisObject(nsId, mcisId)

	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}

	//vmRequest := req

	targetAction := ActionCreate
	targetStatus := StatusRunning

	//goroutin
	var wg sync.WaitGroup

	// VM Group handling
	vmGroupSize, _ := strconv.Atoi(vmRequest.VmGroupSize)
	fmt.Printf("vmGroupSize: %v\n", vmGroupSize)

	if vmGroupSize > 0 {

		fmt.Println("=========================== Create MCIS VM Group object")
		key := common.GenMcisVmGroupKey(nsId, mcisId, vmRequest.Name)

		// TODO: Enhancement Required. Need to check existing VM Group. Need to update it if exist.
		vmGroupInfoData := TbVmGroupInfo{}
		vmGroupInfoData.Id = vmRequest.Name
		vmGroupInfoData.Name = vmRequest.Name
		vmGroupInfoData.VmGroupSize = vmRequest.VmGroupSize

		for i := 0; i < vmGroupSize; i++ {
			vmGroupInfoData.VmId = append(vmGroupInfoData.VmId, vmGroupInfoData.Id+"-"+strconv.Itoa(i))
		}

		val, _ := json.Marshal(vmGroupInfoData)
		err := common.CBStore.Put(string(key), string(val))
		if err != nil {
			common.CBLog.Error(err)
		}
		keyValue, _ := common.CBStore.Get(string(key))
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===========================")

	}

	for i := 0; i <= vmGroupSize; i++ {
		vmInfoData := TbVmInfo{}

		if vmGroupSize == 0 { // for VM (not in a group)
			vmInfoData.Name = vmRequest.Name
		} else { // for VM (in a group)
			if i == vmGroupSize {
				break // if vmGroupSize != 0 && vmGroupSize == i, skip the final loop
			}
			vmInfoData.VmGroupId = vmRequest.Name
			// TODO: Enhancement Required. Need to check existing VM Group. Need to update it if exist.
			vmInfoData.Name = vmRequest.Name + "-" + strconv.Itoa(i)
			fmt.Println("===========================")
			fmt.Println("vmInfoData.Name: " + vmInfoData.Name)
			fmt.Println("===========================")

		}
		vmInfoData.Id = vmInfoData.Name

		vmInfoData.Description = vmRequest.Description
		vmInfoData.PublicIP = "Not assigned yet"
		vmInfoData.PublicDNS = "Not assigned yet"

		vmInfoData.Status = StatusCreating
		vmInfoData.TargetAction = targetAction
		vmInfoData.TargetStatus = targetStatus

		vmInfoData.ConnectionName = vmRequest.ConnectionName
		vmInfoData.SpecId = vmRequest.SpecId
		vmInfoData.ImageId = vmRequest.ImageId
		vmInfoData.VNetId = vmRequest.VNetId
		vmInfoData.SubnetId = vmRequest.SubnetId
		//vmInfoData.VnicId = vmRequest.VnicId
		//vmInfoData.PublicIpId = vmRequest.PublicIpId
		vmInfoData.SecurityGroupIds = vmRequest.SecurityGroupIds
		vmInfoData.SshKeyId = vmRequest.SshKeyId
		vmInfoData.Description = vmRequest.Description

		vmInfoData.VmUserAccount = vmRequest.VmUserAccount
		vmInfoData.VmUserPassword = vmRequest.VmUserPassword

		wg.Add(1)
		go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)

	}

	wg.Wait()

	//Update MCIS status

	mcisTmp, err = GetMcisObject(nsId, mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		return temp, err
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	mcisTmp.Status = mcisStatusTmp.Status

	if mcisTmp.TargetStatus == mcisTmp.Status {
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisId, mcisTmp.InstallMonAgent)
	if mcisTmp.InstallMonAgent != "no" {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "ubuntu" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}
	return &mcisTmp, nil

}

// CoreGetMcisVmAction is func to Get McisVm Action
func CoreGetMcisVmAction(nsId string, mcisId string, vmId string, action string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	fmt.Println("[Get VM requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend VM]")

		ControlVm(nsId, mcisId, vmId, ActionSuspend)
		//mapA := map[string]string{"message": "Suspending the VM"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Suspending the VM", nil

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		ControlVm(nsId, mcisId, vmId, ActionResume)
		//mapA := map[string]string{"message": "Resuming the VM"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Resuming the VM", nil

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")

		ControlVm(nsId, mcisId, vmId, ActionReboot)
		//mapA := map[string]string{"message": "Rebooting the VM"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Rebooting the VM", nil

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		ControlVm(nsId, mcisId, vmId, ActionTerminate)

		//mapA := map[string]string{"message": "Terminating the VM"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Terminating the VM", nil
	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

// CoreGetMcisVmStatus is func to Get McisVm Status
func CoreGetMcisVmStatus(nsId string, mcisId string, vmId string) (*TbVmStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		temp := &TbVmStatusInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	fmt.Println("[status VM]")

	vmKey := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(vmKey)
	vmKeyValue, _ := common.CBStore.Get(vmKey)
	if vmKeyValue == nil {
		//mapA := map[string]string{"message": "Cannot find " + vmKey}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, fmt.Errorf("Cannot find " + vmKey)
	}

	vmStatusResponse, err := GetVmStatus(nsId, mcisId, vmId)

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	return &vmStatusResponse, nil
}

// CoreGetMcisVmInfo is func to Get McisVm Info
func CoreGetMcisVmInfo(nsId string, mcisId string, vmId string) (*TbVmInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(vmId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return temp, err
	}

	fmt.Println("[Get MCIS-VM info for id]" + vmId)

	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	vmKey := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(vmKey)
	vmKeyValue, _ := common.CBStore.Get(vmKey)
	if vmKeyValue == nil {
		//mapA := map[string]string{"message": "Cannot find " + key}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, fmt.Errorf("Cannot find " + key)
	}
	//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
	vmTmp.Id = vmId

	//get current vm status
	vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
	}

	vmTmp.Status = vmStatusInfoTmp.Status
	vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
	vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

	return &vmTmp, nil
}

// CreateMcis is func to create MCIS obeject and deploy requested VMs
func CreateMcis(nsId string, req *TbMcisReq) (*TbMcisInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	// returns InvalidValidationError for bad validation input, nil or ValidationErrors ( []FieldError )
	err = validate.Struct(req)
	if err != nil {

		// this check is only needed when your code could produce
		// an invalid value for validation such as interface with nil
		// value most including myself do not usually have code like this.
		if _, ok := err.(*validator.InvalidValidationError); ok {
			fmt.Println(err)
			return nil, err
		}

		// for _, err := range err.(validator.ValidationErrors) {

		// 	fmt.Println(err.Namespace()) // can differ when a custom TagNameFunc is registered or
		// 	fmt.Println(err.Field())     // by passing alt name to ReportError like below
		// 	fmt.Println(err.StructNamespace())
		// 	fmt.Println(err.StructField())
		// 	fmt.Println(err.Tag())
		// 	fmt.Println(err.ActualTag())
		// 	fmt.Println(err.Kind())
		// 	fmt.Println(err.Type())
		// 	fmt.Println(err.Value())
		// 	fmt.Println(err.Param())
		// 	fmt.Println()
		// }

		return nil, err
	}

	check, _ := CheckMcis(nsId, req.Name)
	if check {
		err := fmt.Errorf("The mcis " + req.Name + " already exists.")
		return nil, err
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	mcisId := req.Name
	vmRequest := req.Vm

	fmt.Println("=========================== Create MCIS object")
	key := common.GenMcisKey(nsId, mcisId, "")
	mapA := map[string]string{
		"id":              mcisId,
		"name":            mcisId,
		"description":     req.Description,
		"status":          StatusCreating,
		"targetAction":    targetAction,
		"targetStatus":    targetStatus,
		"installMonAgent": req.InstallMonAgent,
		"label":           req.Label,
	}
	val, err := json.Marshal(mapA)
	if err != nil {
		err := fmt.Errorf("System Error: CreateMcis json.Marshal(mapA) Error")
		common.CBLog.Error(err)
		return nil, err
	}

	err = common.CBStore.Put(string(key), string(val))
	if err != nil {
		err := fmt.Errorf("System Error: CreateMcis CBStore.Put Error")
		common.CBLog.Error(err)
		return nil, err
	}

	keyValue, _ := common.CBStore.Get(string(key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// Check whether VM names meet requirement.
	for _, k := range vmRequest {
		err = common.CheckString(k.Name)
		if err != nil {
			temp := &TbMcisInfo{}
			common.CBLog.Error(err)
			return temp, err
		}
	}

	//goroutin
	var wg sync.WaitGroup

	for _, k := range vmRequest {

		// VM Group handling
		vmGroupSize, _ := strconv.Atoi(k.VmGroupSize)
		fmt.Printf("vmGroupSize: %v\n", vmGroupSize)

		if vmGroupSize > 0 {

			fmt.Println("=========================== Create MCIS VM Group object")
			key := common.GenMcisVmGroupKey(nsId, mcisId, k.Name)

			vmGroupInfoData := TbVmGroupInfo{}
			vmGroupInfoData.Id = common.ToLower(k.Name)
			vmGroupInfoData.Name = common.ToLower(k.Name)
			vmGroupInfoData.VmGroupSize = k.VmGroupSize

			for i := 0; i < vmGroupSize; i++ {
				vmGroupInfoData.VmId = append(vmGroupInfoData.VmId, vmGroupInfoData.Id+"-"+strconv.Itoa(i))
			}

			val, _ := json.Marshal(vmGroupInfoData)
			err := common.CBStore.Put(string(key), string(val))
			if err != nil {
				common.CBLog.Error(err)
			}
			keyValue, _ := common.CBStore.Get(string(key))
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			fmt.Println("===========================")

		}

		for i := 0; i <= vmGroupSize; i++ {
			vmInfoData := TbVmInfo{}

			if vmGroupSize == 0 { // for VM (not in a group)
				vmInfoData.Name = common.ToLower(k.Name)
			} else { // for VM (in a group)
				if i == vmGroupSize {
					break // if vmGroupSize != 0 && vmGroupSize == i, skip the final loop
				}
				vmInfoData.VmGroupId = common.ToLower(k.Name)
				vmInfoData.Name = common.ToLower(k.Name) + "-" + strconv.Itoa(i)
				fmt.Println("===========================")
				fmt.Println("vmInfoData.Name: " + vmInfoData.Name)
				fmt.Println("===========================")

			}
			vmInfoData.Id = vmInfoData.Name

			vmInfoData.PublicIP = "Not assigned yet"
			vmInfoData.PublicDNS = "Not assigned yet"

			vmInfoData.Status = StatusCreating
			vmInfoData.TargetAction = targetAction
			vmInfoData.TargetStatus = targetStatus

			vmInfoData.ConnectionName = k.ConnectionName
			vmInfoData.SpecId = k.SpecId
			vmInfoData.ImageId = k.ImageId
			vmInfoData.VNetId = k.VNetId
			vmInfoData.SubnetId = k.SubnetId
			vmInfoData.SecurityGroupIds = k.SecurityGroupIds
			vmInfoData.SshKeyId = k.SshKeyId
			vmInfoData.Description = k.Description
			vmInfoData.VmUserAccount = k.VmUserAccount
			vmInfoData.VmUserPassword = k.VmUserPassword

			// Avoid concurrent requests to CSP.
			time.Sleep(time.Duration(i) * time.Second)

			wg.Add(1)
			go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)
			//AddVmToMcis(nsId, req.Id, vmInfoData)

		}
	}
	wg.Wait()

	mcisTmp, err := GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	mcisStatusTmp, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	mcisTmp.Status = mcisStatusTmp.Status

	if mcisTmp.TargetStatus == mcisTmp.Status {
		mcisTmp.TargetStatus = StatusComplete
		mcisTmp.TargetAction = ActionComplete
	}
	UpdateMcisInfo(nsId, mcisTmp)

	fmt.Println("[MCIS has been created]" + mcisId)
	//common.PrintJsonPretty(mcisTmp)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisTmp.Id, req.InstallMonAgent)

	mcisTmp.InstallMonAgent = req.InstallMonAgent
	UpdateMcisInfo(nsId, mcisTmp)

	if req.InstallMonAgent != "no" {

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.UserName = "ubuntu" // this MCIS user name is temporal code. Need to improve.

			fmt.Printf("\n===========================\n")
			// Sleep for 60 seconds for a safe DF agent installation.
			fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n")
			time.Sleep(60 * time.Second)

			fmt.Printf("\n[InstallMonitorAgentToMcis]\n\n")
			content, err := InstallMonitorAgentToMcis(nsId, mcisId, reqToMon)
			if err != nil {
				common.CBLog.Error(err)
				//mcisTmp.InstallMonAgent = "no"
			}
			common.PrintJsonPretty(content)
			//mcisTmp.InstallMonAgent = "yes"
		}
	}

	mcisTmp, err = GetMcisObject(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	return &mcisTmp, nil
}

// AddVmToMcis is func to add VM to MCIS
func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo) error {
	fmt.Printf("\n[AddVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, _ := common.CBStore.Get(key)
	if keyValue == nil {
		return fmt.Errorf("AddVmToMcis: Cannot find mcisId. Key: %s", key)
	}

	configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	regionTmp, _ := common.GetRegion(configTmp.RegionName)

	nativeRegion := ""
	for _, v := range regionTmp.KeyValueInfoList {
		if strings.ToLower(v.Key) == "region" || strings.ToLower(v.Key) == "location" {
			nativeRegion = v.Value
			break
		}
	}

	vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(nativeRegion))

	//fmt.Printf("\n[configTmp]\n %+v regionTmp %+v \n", configTmp, regionTmp)
	//fmt.Printf("\n[vmInfoData.Location]\n %+v\n", vmInfoData.Location)

	//AddVmInfoToMcis(nsId, mcisId, *vmInfoData)
	// Make VM object
	key = common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	val, _ := json.Marshal(vmInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Printf("\n[AddVmToMcis Befor request vmInfoData]\n")
	//common.PrintJsonPretty(vmInfoData)

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err = CreateVm(nsId, mcisId, vmInfoData)

	fmt.Printf("\n[AddVmToMcis After request vmInfoData]\n")
	//common.PrintJsonPretty(vmInfoData)

	if err != nil {
		vmInfoData.Status = StatusFailed
		vmInfoData.SystemMessage = err.Error()
		UpdateVmInfo(nsId, mcisId, *vmInfoData)
		common.CBLog.Error(err)
		return err
	}

	// set initial TargetAction, TargetStatus
	vmInfoData.TargetAction = ActionComplete
	vmInfoData.TargetStatus = StatusComplete

	// get and set current vm status
	vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmInfoData.Id)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Printf("\n[AddVmToMcis vmStatusInfoTmp]\n")
	common.PrintJsonPretty(vmStatusInfoTmp)

	vmInfoData.Status = vmStatusInfoTmp.Status

	// Monitoring Agent Installation Status (init: notInstalled)
	vmInfoData.MonAgentStatus = "notInstalled"

	// set CreatedTime
	t := time.Now()
	vmInfoData.CreatedTime = t.Format("2006-01-02 15:04:05")
	fmt.Println(vmInfoData.CreatedTime)

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil

}

// CreateVm is func to create VM
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

	var tempSpiderVMInfo SpiderVMInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SpiderRestUrl + "/vm"

		method := "POST"

		fmt.Println("\n[Calling SPIDER]START")
		fmt.Println("url: " + url + " method: " + method)

		tempReq := SpiderVMReqInfoWrapper{}
		tempReq.ConnectionName = vmInfoData.ConnectionName

		//generate VM ID(Name) to request to CSP(Spider)
		//combination of nsId, mcidId, and vmName reqested from user
		cspVmIdToRequest := nsId + "-" + mcisId + "-" + vmInfoData.Name
		tempReq.ReqInfo.Name = cspVmIdToRequest

		err := fmt.Errorf("")

		tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, common.StrImage, vmInfoData.ImageId)
		if tempReq.ReqInfo.ImageName == "" || err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, common.StrSpec, vmInfoData.SpecId)
		if tempReq.ReqInfo.VMSpecName == "" || err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.VPCName, err = common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.VNetId)
		if tempReq.ReqInfo.VPCName == "" {
			common.CBLog.Error(err)
			return err
		}

		// TODO: needs to be enhnaced to use GetCspResourceId (GetCspResourceId needs to be updated as well)
		tempReq.ReqInfo.SubnetName = vmInfoData.SubnetId //common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.SubnetId)
		if tempReq.ReqInfo.SubnetName == "" {
			common.CBLog.Error(err)
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspSgId := v //common.GetCspResourceId(nsId, common.StrSecurityGroup, v)
			if CspSgId == "" {
				common.CBLog.Error(err)
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
		}
		tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		tempReq.ReqInfo.KeyPairName, err = common.GetCspResourceId(nsId, common.StrSSHKey, vmInfoData.SshKeyId)
		if tempReq.ReqInfo.KeyPairName == "" {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.VMUserId = vmInfoData.VmUserAccount
		tempReq.ReqInfo.VMUserPasswd = vmInfoData.VmUserPassword

		fmt.Printf("\n[Request body to CB-SPIDER for Creating VM]\n")
		common.PrintJsonPretty(tempReq)

		payload, _ := json.Marshal(tempReq)
		// fmt.Println("payload: " + string(payload))

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
			common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		fmt.Println("Called CB-Spider API.")
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		if err != nil {
			common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
		err = json.Unmarshal(body, &tempSpiderVMInfo)

		if err != nil {
			common.PrintJsonPretty(err)
			common.CBLog.Error(err)
			return err
		}

		fmt.Println("[Response from SPIDER]")
		common.PrintJsonPretty(tempSpiderVMInfo)
		fmt.Println("[Calling SPIDER]END")

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			fmt.Println("body: ", string(body))
			common.CBLog.Error(err)
			return err
		}

	} else {

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return err
		}
		defer ccm.Close()

		fmt.Println("\n[Calling SPIDER]START")

		tempReq := SpiderVMReqInfoWrapper{}
		tempReq.ConnectionName = vmInfoData.ConnectionName

		//generate VM ID(Name) to request to CSP(Spider)
		//combination of nsId, mcidId, and vmName reqested from user
		cspVmIdToRequest := nsId + "-" + mcisId + "-" + vmInfoData.Name
		tempReq.ReqInfo.Name = cspVmIdToRequest

		err = fmt.Errorf("")

		tempReq.ReqInfo.ImageName, err = common.GetCspResourceId(nsId, common.StrImage, vmInfoData.ImageId)
		if tempReq.ReqInfo.ImageName == "" || err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.VMSpecName, err = common.GetCspResourceId(nsId, common.StrSpec, vmInfoData.SpecId)
		if tempReq.ReqInfo.VMSpecName == "" || err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.VPCName = vmInfoData.VNetId //common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.VNetId)
		if tempReq.ReqInfo.VPCName == "" {
			common.CBLog.Error(err)
			return err
		}

		tempReq.ReqInfo.SubnetName = vmInfoData.SubnetId //common.GetCspResourceId(nsId, "subnet", vmInfoData.SubnetId)
		if tempReq.ReqInfo.SubnetName == "" {
			common.CBLog.Error(err)
			return err
		}

		var SecurityGroupIdsTmp []string
		for _, v := range vmInfoData.SecurityGroupIds {
			CspSgId := v //common.GetCspResourceId(nsId, common.StrSecurityGroup, v)
			if CspSgId == "" {
				common.CBLog.Error(err)
				return err
			}

			SecurityGroupIdsTmp = append(SecurityGroupIdsTmp, CspSgId)
		}
		tempReq.ReqInfo.SecurityGroupNames = SecurityGroupIdsTmp

		tempReq.ReqInfo.KeyPairName = vmInfoData.SshKeyId //common.GetCspResourceId(nsId, common.StrSSHKey, vmInfoData.SshKeyId)
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

		result, err := ccm.StartVM(string(payload))
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
		err2 := json.Unmarshal([]byte(result), &tempSpiderVMInfo)

		if err2 != nil {
			fmt.Println(err)
			common.CBLog.Error(err)
			return err
		}

	}

	vmInfoData.CspViewVmDetail = tempSpiderVMInfo

	vmInfoData.VmUserAccount = tempSpiderVMInfo.VMUserId
	vmInfoData.VmUserPassword = tempSpiderVMInfo.VMUserPasswd

	//vmInfoData.Location = vmInfoData.Location

	//vmInfoData.VcpuSize = vmInfoData.VcpuSize
	//vmInfoData.MemorySize = vmInfoData.MemorySize
	//vmInfoData.DiskSize = vmInfoData.DiskSize
	//vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.PlacementAlgo = vmInfoData.PlacementAlgo

	// 2. Provided by CB-Spider
	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = tempSpiderVMInfo.Region
	vmInfoData.PublicIP = tempSpiderVMInfo.PublicIP
	vmInfoData.SSHPort, _ = TrimIP(tempSpiderVMInfo.SSHAccessPoint)
	vmInfoData.PublicDNS = tempSpiderVMInfo.PublicDNS
	vmInfoData.PrivateIP = tempSpiderVMInfo.PrivateIP
	vmInfoData.PrivateDNS = tempSpiderVMInfo.PrivateDNS
	vmInfoData.VMBootDisk = tempSpiderVMInfo.VMBootDisk
	vmInfoData.VMBlockDisk = tempSpiderVMInfo.VMBlockDisk
	//vmInfoData.KeyValueList = temp.KeyValueList

	//configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	//vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(tempSpiderVMInfo.Region.Region))

	vmKey := common.GenMcisKey(nsId, mcisId, vmInfoData.Id)
	//mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrImage, vmInfoData.ImageId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSpec, vmInfoData.SpecId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrSSHKey, vmInfoData.SshKeyId, common.StrAdd, vmKey)
	mcir.UpdateAssociatedObjectList(nsId, common.StrVNet, vmInfoData.VNetId, common.StrAdd, vmKey)

	for _, v2 := range vmInfoData.SecurityGroupIds {
		mcir.UpdateAssociatedObjectList(nsId, common.StrSecurityGroup, v2, common.StrAdd, vmKey)
	}

	//content.Status = temp.
	//content.CloudId = temp.

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

	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	return nil
}

// ControlMcis is func to control MCIS
func ControlMcis(nsId string, mcisId string, action string) error {

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println("[ControlMcis] " + key + " to " + action)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)

	vmList, err := ListVmId(nsId, mcisId)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	if len(vmList) == 0 {
		return nil
	}
	fmt.Println("vmList ", vmList)

	for _, v := range vmList {
		ControlVm(nsId, mcisId, v, action)
	}
	return nil

	//need to change status

}

// CheckAllowedTransition is func to check status transition is acceptable
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
		fmt.Println("Unmarshal Error:", unmarshalErr)
	}

	mcisStatusTmp, _ := GetMcisStatus(nsId, mcisId)

	UpdateMcisInfo(nsId, mcisTmp)

	if strings.Contains(mcisStatusTmp.Status, StatusTerminating) || strings.Contains(mcisStatusTmp.Status, StatusResuming) || strings.Contains(mcisStatusTmp.Status, StatusSuspending) || strings.Contains(mcisStatusTmp.Status, StatusCreating) || strings.Contains(mcisStatusTmp.Status, StatusRebooting) {
		return errors.New(action + " is not allowed for MCIS under " + mcisStatusTmp.Status)
	}
	if !strings.Contains(mcisStatusTmp.Status, "Partial-") && strings.Contains(mcisStatusTmp.Status, StatusTerminated) {
		return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
	}
	if strings.Contains(mcisStatusTmp.Status, StatusSuspended) {
		if strings.EqualFold(action, ActionResume) || strings.EqualFold(action, ActionSuspend) {
			return nil
		} else {
			return errors.New(action + " is not allowed for " + mcisStatusTmp.Status + " MCIS")
		}
	}
	return nil
}

// ControlMcisAsync is func to control MCIS async
func ControlMcisAsync(nsId string, mcisId string, action string) error {

	checkError := CheckAllowedTransition(nsId, mcisId, action)
	if checkError != nil {
		return checkError
	}

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println("[ControlMcisAsync] " + key + " to " + action)
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
	fmt.Println("=============================================== ", vmList)
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

	for _, v := range vmList {
		wg.Add(1)

		// Avoid concurrent requests to CSP.
		time.Sleep(time.Duration(3) * time.Second)

		go ControlVmAsync(&wg, nsId, mcisId, v, action, &results)
	}
	wg.Wait() //goroutine sync wg

	checkErrFlag := ""
	for _, v := range results.ResultArray {
		if v.Error != nil {
			checkErrFlag += "["
			checkErrFlag += v.Error.Error()
			checkErrFlag += "]"
		}
	}
	if checkErrFlag != "" {
		return fmt.Errorf(checkErrFlag)
	}

	return nil

	//need to change status

}

// ControlVmResult is struct for result of VM control
type ControlVmResult struct {
	VmId   string `json:"vmId"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}

// ControlVmResultWrapper is struct for array of results of VM control
type ControlVmResultWrapper struct {
	ResultArray []ControlVmResult `json:"resultarray"`
}

// ControlVmAsync is func to control VM async
func ControlVmAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, action string, results *ControlVmResultWrapper) error {
	defer wg.Done() //goroutine sync done

	var errTmp error
	var err error
	var err2 error
	resultTmp := ControlVmResult{}
	resultTmp.VmId = vmId
	resultTmp.Status = ""
	temp := TbVmInfo{}

	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println("[ControlVmAsync] " + key)

	keyValue, err := common.CBStore.Get(key)

	if keyValue == nil || err != nil {

		resultTmp.Error = fmt.Errorf("CBStoreGetErr. keyValue == nil || err != nil. key[" + key + "]")
		results.ResultArray = append(results.ResultArray, resultTmp)
		common.PrintJsonPretty(resultTmp)
		return resultTmp.Error

	} else {
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
		if unmarshalErr != nil {
			fmt.Println("Unmarshal error:", unmarshalErr)
		}

		fmt.Println("\n[Calling SPIDER]START vmControl")

		cspVmId := temp.CspViewVmDetail.IId.NameId
		common.PrintJsonPretty(temp.CspViewVmDetail)

		// Prevent malformed cspVmId
		if cspVmId == "" || common.CheckString(cspVmId) != nil {
			resultTmp.Error = fmt.Errorf("Not valid requested CSPNativeVmId: [" + cspVmId + "]")
			temp.Status = StatusFailed
			temp.SystemMessage = resultTmp.Error.Error()
			UpdateVmInfo(nsId, mcisId, temp)
			//return err
		} else {
			if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

				url := ""
				method := ""
				switch action {
				case ActionTerminate:

					temp.TargetAction = ActionTerminate
					temp.TargetStatus = StatusTerminated
					temp.Status = StatusTerminating

					url = common.SpiderRestUrl + "/vm/" + cspVmId
					method = "DELETE"
				case ActionReboot:

					temp.TargetAction = ActionReboot
					temp.TargetStatus = StatusRunning
					temp.Status = StatusRebooting

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=reboot"
					method = "GET"
				case ActionSuspend:

					temp.TargetAction = ActionSuspend
					temp.TargetStatus = StatusSuspended
					temp.Status = StatusSuspending

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=suspend"
					method = "GET"
				case ActionResume:

					temp.TargetAction = ActionResume
					temp.TargetStatus = StatusRunning
					temp.Status = StatusResuming

					url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=resume"
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
					common.CBLog.Error(err)
					return err
				}
				req.Header.Add("Content-Type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					common.CBLog.Error(err)
					return err
				}
				defer res.Body.Close()
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					common.CBLog.Error(err)
					return err
				}

				fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
				switch {
				case res.StatusCode >= 400 || res.StatusCode < 200:
					err := fmt.Errorf(string(body))
					common.CBLog.Error(err)
					errTmp = err
				}

				err2 = json.Unmarshal(body, &resultTmp)

			} else {

				// Set CCM gRPC API
				ccm := api.NewCloudResourceHandler()
				err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
				if err != nil {
					common.CBLog.Error("ccm failed to set config : ", err)
					temp.Status = StatusFailed
					UpdateVmInfo(nsId, mcisId, temp)
					return err
				}
				err = ccm.Open()
				if err != nil {
					common.CBLog.Error("ccm api open failed : ", err)
					temp.Status = StatusFailed
					UpdateVmInfo(nsId, mcisId, temp)
					return err
				}
				defer ccm.Close()

				var result string

				switch action {
				case ActionTerminate:

					temp.TargetAction = ActionTerminate
					temp.TargetStatus = StatusTerminated
					temp.Status = StatusTerminating

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.TerminateVMByParam(temp.ConnectionName, cspVmId, "false")

				case ActionReboot:

					temp.TargetAction = ActionReboot
					temp.TargetStatus = StatusRunning
					temp.Status = StatusRebooting

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "reboot")

				case ActionSuspend:

					temp.TargetAction = ActionSuspend
					temp.TargetStatus = StatusSuspended
					temp.Status = StatusSuspending

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "suspend")

				case ActionResume:

					temp.TargetAction = ActionResume
					temp.TargetStatus = StatusRunning
					temp.Status = StatusResuming

					UpdateVmInfo(nsId, mcisId, temp)

					result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "resume")

				default:
					return errors.New(action + "is invalid actionType")
				}

				err2 = json.Unmarshal([]byte(result), &resultTmp)

			}

			if err2 != nil {
				fmt.Println(err2)
				common.CBLog.Error(err)
				errTmp = err
			}
			if errTmp != nil {
				resultTmp.Error = errTmp

				temp.Status = StatusFailed
				temp.SystemMessage = errTmp.Error()
				UpdateVmInfo(nsId, mcisId, temp)
			}
			results.ResultArray = append(results.ResultArray, resultTmp)

			common.PrintJsonPretty(resultTmp)

			fmt.Println("[Calling SPIDER]END vmControl")

			if action != ActionTerminate {
				//When VM is restared, temporal PublicIP will be chanaged. Need update.
				UpdateVmPublicIp(nsId, mcisId, temp)
			}
		}

	}

	return nil

}

// ControlVm is func to control VM
func ControlVm(nsId string, mcisId string, vmId string, action string) error {

	var content struct {
		CloudId string `json:"cloudId"`
		CspVmId string `json:"cspVmId"`
	}

	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println("[ControlVm] " + key)

	keyValue, _ := common.CBStore.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.CloudId)
	//fmt.Printf("%+v\n", content.CspVmId)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Println("\n[Calling SPIDER]START vmControl")

	//fmt.Println("temp.CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	/*
		cspType := getVMsCspType(nsId, mcisId, vmId)
		var cspVmId string
		if cspType == "AWS" {
			cspVmId = temp.CspViewVmDetail.Id
		} else {
	*/
	cspVmId := temp.CspViewVmDetail.IId.NameId
	common.PrintJsonPretty(temp.CspViewVmDetail)

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := ""
		method := ""
		switch action {
		case ActionTerminate:

			temp.TargetAction = ActionTerminate
			temp.TargetStatus = StatusTerminated
			temp.Status = StatusTerminating

			url = common.SpiderRestUrl + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			url = common.SpiderRestUrl + "/controlvm/" + cspVmId + "?action=resume"
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
			return err
		}
		req.Header.Add("Content-Type", "application/json")

		res, err := client.Do(req)
		//fmt.Println("Called mockAPI.")
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)

		fmt.Println(string(body))

		fmt.Println("[Calling SPIDER]END vmControl\n")
		/*
			if strings.Compare(content.CspVmId, "Not assigned yet") == 0 {
				return nil
			}
			if strings.Compare(content.CloudId, "aws") == 0 {
				controlVmAws(content.CspVmId)
			} else if strings.Compare(content.CloudId, "gcp") == 0 {
				controlVmGcp(content.CspVmId)
			} else if strings.Compare(content.CloudId, "azure") == 0 {
				controlVmAzure(content.CspVmId)
			} else {
				fmt.Println("==============ERROR=no matched providerId=================")
			}
		*/

		return nil

	} else {

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return err
		}
		defer ccm.Close()

		var result string

		switch action {
		case ActionTerminate:

			result, err = ccm.TerminateVMByParam(temp.ConnectionName, cspVmId, "false")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionReboot:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "reboot")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionSuspend:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "suspend")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case ActionResume:

			result, err = ccm.ControlVMByParam(temp.ConnectionName, cspVmId, "resume")
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		default:
			return errors.New(action + "is invalid actionType")
		}

		fmt.Println(result)
		fmt.Println("[Calling SPIDER]END vmControl\n")

		return nil
	}
}

// GetMcisObject is func to retrieve MCIS object from database (no current status update)
func GetMcisObject(nsId string, mcisId string) (TbMcisInfo, error) {
	fmt.Println("[GetMcisObject]" + mcisId)
	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return TbMcisInfo{}, err
	}
	mcisTmp := TbMcisInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisTmp)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return TbMcisInfo{}, err
	}

	for _, vmID := range vmList {
		vmtmp, err := GetVmObject(nsId, mcisId, vmID)
		if err != nil {
			common.CBLog.Error(err)
			return TbMcisInfo{}, err
		}
		mcisTmp.Vm = append(mcisTmp.Vm, vmtmp)
	}

	return mcisTmp, nil
}

// GetMcisStatus is func to Get Mcis Status
func GetMcisStatus(nsId string, mcisId string) (*McisStatusInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}

	fmt.Println("[GetMcisStatus]" + mcisId)

	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
	}
	if keyValue == nil {
		err := fmt.Errorf("Not found [" + key + "]")
		common.CBLog.Error(err)
		return &McisStatusInfo{}, err
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
		return &McisStatusInfo{}, err
	}
	if len(vmList) == 0 {
		return &McisStatusInfo{}, nil
	}

	//goroutin sync wg
	var wg sync.WaitGroup
	for _, v := range vmList {
		wg.Add(1)
		go GetVmStatusAsync(&wg, nsId, mcisId, v, &mcisStatus)
	}
	wg.Wait() //goroutine sync wg

	for _, v := range vmList {
		// set master IP of MCIS (Default rule: select 1st Running VM as master)
		vmtmp, _ := GetVmObject(nsId, mcisId, v)
		if vmtmp.Status == StatusRunning {
			mcisStatus.MasterVmId = vmtmp.Id
			mcisStatus.MasterIp = vmtmp.PublicIP
			mcisStatus.MasterSSHPort = vmtmp.SSHPort
			break
		}
	}

	statusFlag := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	statusFlagStr := []string{StatusFailed, StatusSuspended, StatusRunning, StatusTerminated, StatusCreating, StatusSuspending, StatusResuming, StatusRebooting, StatusTerminating, StatusUndefined}
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
	numUnNormalStatus := statusFlag[0] + statusFlag[9]
	numNormalStatus := numVm - numUnNormalStatus

	proportionStr := "-" + strconv.Itoa(tmpMax) + "(" + strconv.Itoa(numNormalStatus) + "/" + strconv.Itoa(numVm) + ")"
	if tmpMax == numVm {
		mcisStatus.Status = statusFlagStr[tmpMaxIndex] + proportionStr
	} else if tmpMax < numVm {
		mcisStatus.Status = "Partial-" + statusFlagStr[tmpMaxIndex] + proportionStr
	} else {
		mcisStatus.Status = statusFlagStr[9] + proportionStr
	}
	// for representing Failed status in front.

	proportionStr = "-" + strconv.Itoa(statusFlag[0]) + "(" + strconv.Itoa(numNormalStatus) + "/" + strconv.Itoa(numVm) + ")"
	if statusFlag[0] > 0 {
		mcisStatus.Status = "Partial-" + statusFlagStr[0] + proportionStr
		if statusFlag[0] == numVm {
			mcisStatus.Status = statusFlagStr[0] + proportionStr
		}
	}

	// proportionStr = "-(" + strconv.Itoa(statusFlag[9]) + "/" + strconv.Itoa(numVm) + ")"
	// if statusFlag[9] > 0 {
	// 	mcisStatus.Status = statusFlagStr[9] + proportionStr
	// }

	// Set mcisStatus.StatusCount
	mcisStatus.StatusCount.CountTotal = numVm
	mcisStatus.StatusCount.CountFailed = statusFlag[0]
	mcisStatus.StatusCount.CountSuspended = statusFlag[1]
	mcisStatus.StatusCount.CountRunning = statusFlag[2]
	mcisStatus.StatusCount.CountTerminated = statusFlag[3]
	mcisStatus.StatusCount.CountCreating = statusFlag[4]
	mcisStatus.StatusCount.CountSuspending = statusFlag[5]
	mcisStatus.StatusCount.CountResuming = statusFlag[6]
	mcisStatus.StatusCount.CountRebooting = statusFlag[7]
	mcisStatus.StatusCount.CountTerminating = statusFlag[8]
	mcisStatus.StatusCount.CountUndefined = statusFlag[9]

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
		mcisTmp.StatusCount = mcisStatus.StatusCount
		UpdateMcisInfo(nsId, mcisTmp)
	}

	return &mcisStatus, nil

	//need to change status

}

// GetMcisStatusAll is func to get MCIS status all
func GetMcisStatusAll(nsId string) ([]McisStatusInfo, error) {

	mcisStatuslist := []McisStatusInfo{}
	mcisList, err := ListMcisId(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return mcisStatuslist, err
	}

	for _, mcisId := range mcisList {
		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return mcisStatuslist, err
		}
		mcisStatuslist = append(mcisStatuslist, *mcisStatus)
	}
	return mcisStatuslist, nil

	//need to change status

}

// GetVmObject is func to get VM object
func GetVmObject(nsId string, mcisId string, vmId string) (TbVmInfo, error) {
	//fmt.Println("[GetVmObject] mcisId: " + mcisId + ", vmId: " + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	return vmTmp, nil
}

// GetVmStatusAsync is func to get VM status async
func GetVmStatusAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, results *McisStatusInfo) error {
	defer wg.Done() //goroutine sync done

	vmStatusTmp, err := GetVmStatus(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
		vmStatusTmp.Status = StatusFailed
		vmStatusTmp.SystemMessage = err.Error()
	}

	results.Vm = append(results.Vm, vmStatusTmp)
	return nil
}

// GetVmStatus is func to get VM status
func GetVmStatus(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	// defer func() {
	// 	if runtimeErr := recover(); runtimeErr != nil {
	// 		myErr := fmt.Errorf("in GetVmStatus; mcisId: " + mcisId + ", vmId: " + vmId)
	// 		common.CBLog.Error(myErr)
	// 		common.CBLog.Error(runtimeErr)
	// 	}
	// }()

	//fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)
	errorInfo := TbVmStatusInfo{}

	keyValue, err := common.CBStore.Get(key)
	if keyValue == nil || err != nil {
		fmt.Println("CBStoreGetErr. keyValue == nil || err != nil", err)
		fmt.Println(err)
		return errorInfo, err
	}

	// fmt.Println(keyValue.Value)
	// fmt.Println("<" + keyValue.Key + "> \n")
	// fmt.Println("===============================================")

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
		fmt.Println(err)
		return errorInfo, err
	}

	errorInfo.Id = temp.Id
	errorInfo.Name = temp.Name
	errorInfo.CspVmId = temp.CspViewVmDetail.IId.NameId
	errorInfo.PublicIp = temp.PublicIP
	errorInfo.SSHPort = temp.SSHPort
	errorInfo.PrivateIp = temp.PrivateIP
	errorInfo.NativeStatus = StatusUndefined
	errorInfo.TargetAction = temp.TargetAction
	errorInfo.TargetStatus = temp.TargetStatus
	errorInfo.Location = temp.Location
	errorInfo.MonAgentStatus = temp.MonAgentStatus
	errorInfo.CreatedTime = temp.CreatedTime
	errorInfo.SystemMessage = "Error in GetVmStatus"

	cspVmId := temp.CspViewVmDetail.IId.NameId

	type statusResponse struct {
		Status string
	}
	statusResponseTmp := statusResponse{}
	statusResponseTmp.Status = ""

	if cspVmId != "" && temp.Status != StatusTerminated {
		fmt.Print("[Calling SPIDER] vmstatus, ")
		fmt.Println("CspVmId: " + cspVmId)
		if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

			url := common.SpiderRestUrl + "/vmstatus/" + cspVmId
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

			// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
			retrycheck := 2
			for i := 0; i < retrycheck; i++ {

				req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
				errorInfo.Status = StatusFailed
				if err != nil {
					fmt.Println(err)
					return errorInfo, err
				}
				req.Header.Add("Content-Type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					fmt.Println(err)
					errorInfo.SystemMessage = err.Error()
					//return errorInfo, err
				} else {
					body, err := ioutil.ReadAll(res.Body)
					if err != nil {
						fmt.Println(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
					err = json.Unmarshal(body, &statusResponseTmp)
					if err != nil {
						fmt.Println(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
					defer res.Body.Close()
				}

				if statusResponseTmp.Status != "" {
					break
				}
				time.Sleep(1 * time.Second)
			}

		} else {

			// Set CCM gRPC API
			ccm := api.NewCloudResourceHandler()
			err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
			if err != nil {
				common.CBLog.Error("ccm failed to set config : ", err)
				return errorInfo, err
			}
			err = ccm.Open()
			if err != nil {
				common.CBLog.Error("ccm api open failed : ", err)
				return errorInfo, err
			}
			defer ccm.Close()

			// Retry to get right VM status from cb-spider. Sometimes cb-spider returns not approriate status.
			retrycheck := 2
			for i := 0; i < retrycheck; i++ {
				result, err := ccm.GetVMStatusByParam(temp.ConnectionName, cspVmId)
				if err != nil {
					common.CBLog.Error(err)
					errorInfo.SystemMessage = err.Error()
					//return errorInfo, err
				} else {
					err = json.Unmarshal([]byte(result), &statusResponseTmp)
					if err != nil {
						common.CBLog.Error(err)
						errorInfo.SystemMessage = err.Error()
						return errorInfo, err
					}
				}

				if statusResponseTmp.Status != "" {
					break
				}
				time.Sleep(1 * time.Second)
			}
		}

	} else {
		statusResponseTmp.Status = ""
	}

	nativeStatus := statusResponseTmp.Status
	// Temporal CODE. This should be changed after CB-Spider fixes status types and strings/
	if nativeStatus == "Creating" {
		statusResponseTmp.Status = StatusCreating
	} else if nativeStatus == "Running" {
		statusResponseTmp.Status = StatusRunning
	} else if nativeStatus == "Suspending" {
		statusResponseTmp.Status = StatusSuspending
	} else if nativeStatus == "Suspended" {
		statusResponseTmp.Status = StatusSuspended
	} else if nativeStatus == "Resuming" {
		statusResponseTmp.Status = StatusResuming
	} else if nativeStatus == "Rebooting" {
		statusResponseTmp.Status = StatusRebooting
	} else if nativeStatus == "Terminating" {
		statusResponseTmp.Status = StatusTerminating
	} else if nativeStatus == "Terminated" {
		statusResponseTmp.Status = StatusTerminated
	} else {
		statusResponseTmp.Status = StatusUndefined
	}
	// End of Temporal CODE.
	temp, err = GetVmObject(nsId, mcisId, vmId)
	if keyValue == nil || err != nil {
		fmt.Println("CBStoreGetErr. keyValue == nil || err != nil", err)
		fmt.Println(err)
		return errorInfo, err
	}
	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Id = temp.Id
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.CspVmId = temp.CspViewVmDetail.IId.NameId

	vmStatusTmp.PrivateIp = temp.PrivateIP
	vmStatusTmp.NativeStatus = nativeStatus
	vmStatusTmp.TargetAction = temp.TargetAction
	vmStatusTmp.TargetStatus = temp.TargetStatus
	vmStatusTmp.Location = temp.Location
	vmStatusTmp.MonAgentStatus = temp.MonAgentStatus
	vmStatusTmp.CreatedTime = temp.CreatedTime
	vmStatusTmp.SystemMessage = temp.SystemMessage

	// fmt.Println("[VM Native Status]" + temp.Id + ":" + nativeStatus)

	//Correct undefined status using TargetAction
	if vmStatusTmp.TargetAction == ActionCreate {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusCreating
		}
		if temp.Status == StatusFailed {
			statusResponseTmp.Status = StatusFailed
		}
	}
	if vmStatusTmp.TargetAction == ActionTerminate {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusTerminated
		}
		if statusResponseTmp.Status == StatusSuspending {
			statusResponseTmp.Status = StatusTerminated
		}
	}
	if vmStatusTmp.TargetAction == ActionResume {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusResuming
		}
		if statusResponseTmp.Status == StatusCreating {
			statusResponseTmp.Status = StatusResuming
		}
	}
	// for action reboot, some csp's native status are suspending, suspended, creating, resuming
	if vmStatusTmp.TargetAction == ActionReboot {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusRebooting
		}
		if statusResponseTmp.Status == StatusSuspending || statusResponseTmp.Status == StatusSuspended || statusResponseTmp.Status == StatusCreating || statusResponseTmp.Status == StatusResuming {
			statusResponseTmp.Status = StatusRebooting
		}
	}

	if vmStatusTmp.Status == StatusTerminated {
		statusResponseTmp.Status = StatusTerminated
	}

	vmStatusTmp.Status = statusResponseTmp.Status

	// TODO: Alibaba Undefined status error is not resolved yet.
	// (After Terminate action. "status": "Undefined", "targetStatus": "None", "targetAction": "None")

	//if TargetStatus == CurrentStatus, record to finialize the control operation
	if vmStatusTmp.TargetStatus == vmStatusTmp.Status {
		if vmStatusTmp.TargetStatus != StatusTerminated {
			vmStatusTmp.SystemMessage = vmStatusTmp.TargetStatus + "==" + vmStatusTmp.Status
			vmStatusTmp.TargetStatus = StatusComplete
			vmStatusTmp.TargetAction = ActionComplete

			//Get current public IP when status has been changed.
			//UpdateVmPublicIp(nsId, mcisId, temp)
			vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, temp.Id)
			if err != nil {
				common.CBLog.Error(err)
				errorInfo.SystemMessage = err.Error()
				return errorInfo, err
			}
			temp.PublicIP = vmInfoTmp.PublicIp
			temp.SSHPort = vmInfoTmp.SSHPort

		} else {
			// Don't init TargetStatus if the TargetStatus is StatusTerminated. It is to finalize VM lifecycle if StatusTerminated.
			vmStatusTmp.TargetStatus = StatusTerminated
			vmStatusTmp.TargetAction = ActionTerminate
			vmStatusTmp.Status = StatusTerminated
			vmStatusTmp.SystemMessage = "This VM has been terminated. No action is acceptable except deletion"
		}
	}

	vmStatusTmp.PublicIp = temp.PublicIP
	vmStatusTmp.SSHPort = temp.SSHPort

	// Apply current status to vmInfo
	temp.Status = vmStatusTmp.Status
	temp.SystemMessage = vmStatusTmp.SystemMessage
	temp.TargetAction = vmStatusTmp.TargetAction
	temp.TargetStatus = vmStatusTmp.TargetStatus

	if cspVmId != "" {
		// don't update VM info, if cspVmId is empty
		UpdateVmInfo(nsId, mcisId, temp)
	}

	return vmStatusTmp, nil
}

// UpdateVmPublicIp is func to update VM public IP
func UpdateVmPublicIp(nsId string, mcisId string, vmInfoData TbVmInfo) error {

	vmInfoTmp, err := GetVmCurrentPublicIp(nsId, mcisId, vmInfoData.Id)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	if vmInfoData.PublicIP != vmInfoTmp.PublicIp || vmInfoData.SSHPort != vmInfoTmp.SSHPort {
		vmInfoData.PublicIP = vmInfoTmp.PublicIp
		vmInfoData.SSHPort = vmInfoTmp.SSHPort
		UpdateVmInfo(nsId, mcisId, vmInfoData)
	}
	return nil
}

// GetVmCurrentPublicIp is func to get VM public IP
func GetVmCurrentPublicIp(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	fmt.Println("[GetVmStatus]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	errorInfo := TbVmStatusInfo{}
	//fmt.Println(key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil || keyValue == nil {
		fmt.Println(err)
		return errorInfo, err
	}

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("\n[Calling SPIDER]START")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)

	cspVmId := temp.CspViewVmDetail.IId.NameId

	type statusResponse struct {
		Status         string
		PublicIP       string
		SSHAccessPoint string
	}
	var statusResponseTmp statusResponse

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SpiderRestUrl + "/vm/" + cspVmId
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

		statusResponseTmp = statusResponse{}

		err2 := json.Unmarshal(body, &statusResponseTmp)
		if err2 != nil {
			fmt.Println(err2)
			return errorInfo, err2
		}

	} else {

		// Set CCM gRPC API
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return errorInfo, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return errorInfo, err
		}
		defer ccm.Close()

		result, err := ccm.GetVMByParam(temp.ConnectionName, cspVmId)
		if err != nil {
			common.CBLog.Error(err)
			return errorInfo, err
		}

		statusResponseTmp = statusResponse{}
		err2 := json.Unmarshal([]byte(result), &statusResponseTmp)
		if err2 != nil {
			common.CBLog.Error(err2)
			return errorInfo, err2
		}

	}

	//common.PrintJsonPretty(statusResponseTmp)
	fmt.Println(statusResponseTmp)
	//fmt.Println("[Calling SPIDER]END\n")

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.PublicIp = statusResponseTmp.PublicIP
	vmStatusTmp.SSHPort, _ = TrimIP(statusResponseTmp.SSHAccessPoint)

	return vmStatusTmp, nil

}

// GetVmSshKey is func to get VM SShKey
func GetVmSshKey(nsId string, mcisId string, vmId string) (string, string, string) {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}

	fmt.Println("[GetVmSshKey]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.SshKeyId)

	sshKey := common.GenResourceKey(nsId, common.StrSSHKey, content.SshKeyId)
	keyValue, _ = common.CBStore.Get(sshKey)
	var keyContent struct {
		Username         string `json:"username"`
		VerifiedUsername string `json:"verifiedUsername"`
		PrivateKey       string `json:"privateKey"`
	}
	json.Unmarshal([]byte(keyValue.Value), &keyContent)

	return keyContent.Username, keyContent.VerifiedUsername, keyContent.PrivateKey
}

// UpdateVmSshKey is func to update VM SShKey
func UpdateVmSshKey(nsId string, mcisId string, vmId string, verifiedUserName string) error {

	var content struct {
		SshKeyId string `json:"sshKeyId"`
	}
	fmt.Println("[GetVmSshKey]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, _ := common.CBStore.Get(key)
	json.Unmarshal([]byte(keyValue.Value), &content)

	sshKey := common.GenResourceKey(nsId, common.StrSSHKey, content.SshKeyId)
	keyValue, _ = common.CBStore.Get(sshKey)

	tmpSshKeyInfo := mcir.TbSshKeyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &tmpSshKeyInfo)

	tmpSshKeyInfo.VerifiedUsername = verifiedUserName

	val, _ := json.Marshal(tmpSshKeyInfo)
	err := common.CBStore.Put(string(keyValue.Key), string(val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	return nil
}

// GetVmIp is func to get VM IP
func GetVmIp(nsId string, mcisId string, vmId string) (string, string) {

	var content struct {
		PublicIP string `json:"publicIP"`
		SSHPort  string `json:"sshPort"`
	}

	fmt.Printf("[GetVmIp] " + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf(" %+v\n", content.PublicIP)

	return content.PublicIP, content.SSHPort
}

// GetVmSpecId is func to get VM SpecId
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

// GetVmListByLabel is func to list VM by label
func GetVmListByLabel(nsId string, mcisId string, label string) ([]string, error) {

	fmt.Println("[GetVmListByLabel]" + mcisId + " by " + label)

	var vmListByLabel []string

	vmList, err := ListVmId(nsId, mcisId)
	fmt.Println(vmList)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	if len(vmList) == 0 {
		return nil, nil
	}

	// delete vms info
	for _, v := range vmList {
		vmObj, vmErr := GetVmObject(nsId, mcisId, v)
		if vmErr != nil {
			common.CBLog.Error(err)
			return nil, vmErr
		}
		//fmt.Println("vmObj.Label: "+ vmObj.Label)
		if vmObj.Label == label {
			fmt.Println("Found VM with " + vmObj.Label + ", VM ID: " + vmObj.Id)
			vmListByLabel = append(vmListByLabel, vmObj.Id)
		}
	}
	return vmListByLabel, nil

}

// GetVmTemplate is func to get VM template
func GetVmTemplate(nsId string, mcisId string, algo string) (TbVmInfo, error) {

	fmt.Println("[GetVmTemplate]" + mcisId + " by algo: " + algo)

	vmList, err := ListVmId(nsId, mcisId)
	//fmt.Println(vmList)
	if err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	if len(vmList) == 0 {
		return TbVmInfo{}, nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(vmList))
	vmObj, vmErr := GetVmObject(nsId, mcisId, vmList[index])
	var vmTemplate TbVmInfo

	// only take template required to create VM
	vmTemplate.Name = vmObj.Name
	vmTemplate.ConnectionName = vmObj.ConnectionName
	vmTemplate.ImageId = vmObj.ImageId
	vmTemplate.SpecId = vmObj.SpecId
	vmTemplate.VNetId = vmObj.VNetId
	vmTemplate.SubnetId = vmObj.SubnetId
	vmTemplate.SecurityGroupIds = vmObj.SecurityGroupIds
	vmTemplate.SshKeyId = vmObj.SshKeyId
	vmTemplate.VmUserAccount = vmObj.VmUserAccount
	vmTemplate.VmUserPassword = vmObj.VmUserPassword

	if vmErr != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, vmErr
	}

	return vmTemplate, nil

}

// GetCloudLocation is to get location of clouds (need error handling)
func GetCloudLocation(cloudType string, nativeRegion string) GeoLocation {

	location := GeoLocation{}

	if cloudType == "" || nativeRegion == "" {

		// need error handling instead of assigning default value
		location.CloudType = "ufc"
		location.NativeRegion = "ufc"
		location.BriefAddr = "South Korea (Seoul)"
		location.Latitude = "37.4767"
		location.Longitude = "126.8841"

		return location
	}

	key := "/cloudtype/" + cloudType + "/region/" + nativeRegion

	fmt.Printf("[GetCloudLocation] KEY: %+v\n", key)

	keyValue, err := common.CBStore.Get(key)

	if err != nil {
		common.CBLog.Error(err)
		return location
	}

	if keyValue == nil {
		file, fileErr := os.Open("../assets/cloudlocation.csv")
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
		keyValue, err = common.CBStore.Get(key)
		if err != nil {
			common.CBLog.Error(err)
			return location
		}
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
