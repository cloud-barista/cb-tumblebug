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

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
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
const StatusUndefined string = "Undefined"
const StatusComplete string = "None"

const milkywayPort string = ":1324/milkyway/"

const SshDefaultUserName01 string = "cb-user"
const SshDefaultUserName02 string = "ubuntu"
const SshDefaultUserName03 string = "root"
const SshDefaultUserName04 string = "ec2-user"

const LabelAutoGen string = "AutoGen"

// Structs for REST API

// 2020-04-13 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMHandler.go
type SpiderVMReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVMInfo
}

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

type RegionInfo struct { // Spider
	Region string
	Zone   string
}

type TbMcisReq struct {
	Name string `json:"name"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"yes" default:"yes" enums:"yes,no"` // yes or no

	Label string `json:"label"`

	PlacementAlgo string `json:"placementAlgo"`
	Description   string `json:"description"`

	Vm []TbVmReq `json:"vm"`
}

type TbMcisInfo struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"yes" default:"yes" enums:"yes,no"` // yes or no

	Label string `json:"label"`

	PlacementAlgo string     `json:"placementAlgo"`
	Description   string     `json:"description"`
	Vm            []TbVmInfo `json:"vm"`
}

// struct TbVmReq is to get requirements to create a new server instance.
type TbVmReq struct {
	// VM name or VM group name if is (not empty) && (> 0). If it is a group, actual VM name will be generated with -N postfix.
	Name string `json:"name"`

	// if vmGroupSize is (not empty) && (> 0), VM group will be gernetad. VMs will be created accordingly.
	VmGroupSize string `json:"vmGroupSize" example:"3" default:""`

	Label string `json:"label"`

	Description string `json:"description"`

	ConnectionName   string   `json:"connectionName"`
	SpecId           string   `json:"specId"`
	ImageId          string   `json:"imageId"`
	VNetId           string   `json:"vNetId"`
	SubnetId         string   `json:"subnetId"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	SshKeyId         string   `json:"sshKeyId"`
	VmUserAccount    string   `json:"vmUserAccount"`
	VmUserPassword   string   `json:"vmUserPassword"`
}

// struct TbVmGroupInfo is to define an object that includes homogeneous VMs.
type TbVmGroupInfo struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	VmId        []string `json:"vmId"`
	VmGroupSize string   `json:"vmGroupSize"`
}

// struct TbVmGroupInfo is to define a server instance object
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

type GeoLocation struct {
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
	BriefAddr    string `json:"briefAddr"`
	CloudType    string `json:"cloudType"`
	NativeRegion string `json:"nativeRegion"`
}

// struct McisStatusInfo is to define simple information of MCIS with updated status of all VMs
type McisStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	//Vm_num string         `json:"vm_num"`
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

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

// McisCmdReq is remote command struct
type McisCmdReq struct {
	McisId   string `json:"mcisId"`
	VmId     string `json:"vmId"`
	Ip       string `json:"ip"`
	UserName string `json:"userName"`
	SshKey   string `json:"sshKey"`
	Command  string `json:"command"`
}

type McisRecommendReq struct {
	VmReq          []TbVmRecommendReq `json:"vmReq"`
	PlacementAlgo  string             `json:"placementAlgo"`
	PlacementParam []common.KeyValue  `json:"placementParam"`
	MaxResultNum   string             `json:"maxResultNum"`
}

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

type TbVmPriority struct {
	Priority string          `json:"priority"`
	VmSpec   mcir.TbSpecInfo `json:"vmSpec"`
}

type TbVmRecommendInfo struct {
	VmReq          TbVmRecommendReq  `json:"vmReq"`
	VmPriority     []TbVmPriority    `json:"vmPriority"`
	PlacementAlgo  string            `json:"placementAlgo"`
	PlacementParam []common.KeyValue `json:"placementParam"`
}

func VerifySshUserName(nsId string, mcisId string, vmId string, vmIp string, sshPort string, givenUserName string) (string, string, error) {

	// verify if vm is running with a public ip.
	if vmIp == "" {
		return "", "", fmt.Errorf("Cannot do ssh, VM IP is null")
	}
	vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
		return "", "", err
	}
	if vmStatusInfoTmp.Status != StatusRunning || vmIp == "" {
		return "", "", fmt.Errorf("Cannot do ssh, VM IP is not Running")
	}

	// find vaild username
	userName, _, privateKey := GetVmSshKey(nsId, mcisId, vmId)
	userNames := []string{
		userName,
		givenUserName,
		SshDefaultUserName01,
		SshDefaultUserName02,
		SshDefaultUserName03,
		SshDefaultUserName04,
	}

	theUserName := ""
	cmd := "ls"

	_, verifiedUserName, _ := GetVmSshKey(nsId, mcisId, vmId)

	if verifiedUserName != "" {
		fmt.Println("[SSH] " + "(" + vmIp + ")" + "with userName:" + verifiedUserName)
		fmt.Println("[CMD] " + cmd)

		retrycheck := 10
		for i := 0; i < retrycheck; i++ {
			conerr := CheckConnectivity(vmIp, sshPort)
			if conerr == nil {
				fmt.Println("[ERR: CheckConnectivity] nil. break")
				break
			}
			if i == retrycheck-1 {
				return "", "", fmt.Errorf("Cannot do ssh, the port is not opened (10 trials)")
			}
			time.Sleep(2 * time.Second)
		}

		result, err := RunSSH(vmIp, sshPort, verifiedUserName, privateKey, cmd)
		if err != nil {
			fmt.Println("[ERR: result] " + "[ERR: err] " + err.Error())
			return "", "", fmt.Errorf("Cannot do ssh, with verifiedUserName")
		}
		if err == nil {
			theUserName = verifiedUserName
			fmt.Println("[RST] " + *result + "[Username] " + verifiedUserName)
			return theUserName, privateKey, nil
		}
	}

	retrycheck := 10
	for i := 0; i < retrycheck; i++ {
		conerr := CheckConnectivity(vmIp, sshPort)
		if conerr == nil {
			//fmt.Println("[ERR: conerr] nil. break")
			break
		}
		if i == retrycheck-1 {
			return "", "", fmt.Errorf("Cannot do ssh, the port is not opened (10 trials)")
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println("[Retrieve ssh username from the given list]")
	for _, v := range userNames {
		if v != "" {
			fmt.Println("[SSH] " + "(" + vmIp + ")" + "with userName:" + v)
			result, err := RunSSH(vmIp, sshPort, v, privateKey, cmd)
			if err != nil {
				fmt.Println("[ERR: result] " + "[ERR: err] " + err.Error())
			}
			if err == nil {
				theUserName = v
				fmt.Println("[RST] " + *result + "[Username] " + v)
				break
			}
			time.Sleep(2 * time.Second)
		}
	}
	if theUserName != "" {
		err := UpdateVmSshKey(nsId, mcisId, vmId, theUserName)
		if err != nil {
			fmt.Println("[ERR: result] " + "[ERR: err] " + err.Error())
			return "", "", err
		}
	} else {
		return "", "", fmt.Errorf("Could not find username")
	}

	return theUserName, privateKey, nil
}

type SshCmdResult struct { // Tumblebug
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
	Err    error  `json:"err"`
}

// AgentInstallContentWrapper ...
type AgentInstallContentWrapper struct {
	Result_array []AgentInstallContent `json:"result_array"`
}

// AgentInstallContent ...
type AgentInstallContent struct {
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
}

func InstallAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
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

		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
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

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

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
		fmt.Println("whoops:", err2)
	}
	//benchInfoTmp.ResultArray =  resultTmp.ResultArray
	if errStr != "" {
		resultTmp.Result = errStr
	}
	resultTmp.SpecId = GetVmSpecId(nsId, mcisId, vmId)
	results.ResultArray = append(results.ResultArray, resultTmp)
}

func CoreGetAllBenchmark(nsId string, mcisId string, host string) (*BenchmarkInfoArray, error) {

	var err error

	nsId = strings.ToLower(nsId)
	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	mcisId = strings.ToLower(mcisId)
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

func CoreGetBenchmark(nsId string, mcisId string, action string, host string) (*BenchmarkInfoArray, error) {

	var err error

	nsId = strings.ToLower(nsId)
	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	mcisId = strings.ToLower(mcisId)
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

func BenchmarkAction(nsId string, mcisId string, action string, option string) (BenchmarkInfoArray, error) {

	var results BenchmarkInfoArray

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return BenchmarkInfoArray{}, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	for i, v := range vmList {
		wg.Add(1)

		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
			vmList[i] = vmId
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
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

// UpdateMcisInfo func Update MCIS Info (without VM info in MCIS)
func UpdateMcisInfo(nsId string, mcisInfoData TbMcisInfo) {

	mcisInfoData.Vm = nil

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

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil
	}

	fmt.Println("[Get MCIS ID list]")
	key := "/ns/" + nsId + "/mcis"
	//fmt.Println(key)

	keyValue, _ := common.CBStore.GetList(key, true)

	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			//fmt.Println(v.Key)
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

	// common.CBLog.Info("ListVmId called. nsId: " + nsId + ", mcisId: " + mcisId) // for debug
	fmt.Println("ListVmId called. nsId: " + nsId + ", mcisId: " + mcisId) // for debug
	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

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
			common.CBLog.Info("in ListVmId; v.Key: " + v.Key + ", key: " + key + ", after TrimPrefix: " + strings.TrimPrefix(v.Key, (key+"/vm/"))) // for debug
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

// func ListVmGroupId returns list of VmGroups in a given MCIS.
func ListVmGroupId(nsId string, mcisId string) ([]string, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	fmt.Println("[ListVmGroupId]")
	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	var vmGroupList []string
	for _, v := range keyValue {
		if strings.Contains(v.Key, "/vmgroup/") {
			vmGroupList = append(vmGroupList, strings.TrimPrefix(v.Key, (key+"/vmgroup/")))
		}
	}
	return vmGroupList, nil
}

func DelMcis(nsId string, mcisId string) error {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	mcisId = strings.ToLower(mcisId)
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

	// ControlMcis first
	err = ControlMcisAsync(nsId, mcisId, ActionTerminate)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	// for deletion, need to wait until termination is finished
	// Sleep for 5 seconds
	fmt.Printf("\n\n[Info] Sleep for 5 seconds for safe MCIS-VMs termination.\n\n")
	time.Sleep(5 * time.Second)

	key := common.GenMcisKey(nsId, mcisId, "")
	fmt.Println(key)

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	// delete vms info
	for _, v := range vmList {
		var vmKey string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmKey = v
		} else {
			// The case that v is a string in form of "vm-01".
			vmKey = common.GenMcisKey(nsId, mcisId, v)
		}

		fmt.Println(vmKey)

		// get vm info
		vmInfo, _ := GetVmObject(nsId, mcisId, v)

		err := common.CBStore.Delete(vmKey)
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

func DelMcisVm(nsId string, mcisId string, vmId string) error {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	vmId = strings.ToLower(vmId)
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
		return err
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

//// Info manage for MCIS recommendation
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

func CoreGetMcisAction(nsId string, mcisId string, action string) (string, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	mcisId = strings.ToLower(mcisId)
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
			//mapA := map[string]string{"message": err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", err
		}

		//mapA := map[string]string{"message": "Terminating the MCIS"}
		//return c.JSON(http.StatusOK, &mapA)
		return "Terminating the MCIS", nil
	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

func CoreGetMcisStatus(nsId string, mcisId string) (*McisStatusInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &McisStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := &McisStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &McisStatusInfo{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	fmt.Println("[status MCIS]")

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	if len(vmList) == 0 {
		//mapA := map[string]string{"message": "No VM to check in the MCIS"}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, nil
	}
	mcisStatusResponse, err := GetMcisStatus(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	return &mcisStatusResponse, nil
}

// GetMcisInfo func returns MCIS information with the current status update
func GetMcisInfo(nsId string, mcisId string) (*TbMcisInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
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

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	for num := range vmList {
		fmt.Println("[GetMcisInfo compare two VMs]")
		common.PrintJsonPretty(mcisObj.Vm[num])
		common.PrintJsonPretty(mcisStatus.Vm[num])

		mcisObj.Vm[num].Status = mcisStatus.Vm[num].Status
		mcisObj.Vm[num].TargetStatus = mcisStatus.Vm[num].TargetStatus
		mcisObj.Vm[num].TargetAction = mcisStatus.Vm[num].TargetAction
	}

	return &mcisObj, nil
}

func CoreGetAllMcis(nsId string, option string) ([]TbMcisInfo, error) {

	nsId = strings.ToLower(nsId)
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

	mcisList := ListMcisId(nsId)

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

		if option == "status" {
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

		vmList, err := ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return nil, err
		}

		for _, v1 := range vmList {
			var vmKey string
			if strings.Contains(v1, "/ns/") && strings.Contains(v1, "/mcis/") && strings.Contains(v1, "/vm/") {
				// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
				vmKey = v1
			} else {
				// The case that v is a string in form of "vm-01".
				vmKey = common.GenMcisKey(nsId, mcisId, v1)
			}

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

		Mcis = append(Mcis, mcisTmp)

	}

	return Mcis, nil
}

func CoreDelAllMcis(nsId string) (string, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}

	mcisList := ListMcisId(nsId)

	if len(mcisList) == 0 {
		//mapA := map[string]string{"message": "No MCIS to delete"}
		//return c.JSON(http.StatusOK, &mapA)
		return "No MCIS to delete", nil
	}

	for _, v := range mcisList {
		err := DelMcis(nsId, v)
		if err != nil {
			common.CBLog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete All MCISs"}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return "", fmt.Errorf("Failed to delete All MCISs")
		}
	}

	return "All MCISs has been deleted", nil
}

func CorePostMcisRecommand(nsId string, req *McisRecommendReq) ([]TbVmRecommendInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	/*
		var content struct {
			//VmReq          []TbVmRecommendReq    `json:"vmReq"`
			Vm_recommend    []mcis.TbVmRecommendInfo `json:"vm_recommend"`
			PlacementAlgo  string                   `json:"placementAlgo"`
			PlacementParam []common.KeyValue        `json:"placementParam"`
		}
	*/
	//content := RestPostMcisRecommandResponse{}
	//content.VmReq = req.VmReq
	//content.PlacementAlgo = req.PlacementAlgo
	//content.PlacementParam = req.PlacementParam

	Vm_recommend := []TbVmRecommendInfo{}

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

		Vm_recommend = append(Vm_recommend, vmTmp)
	}

	return Vm_recommend, nil
}

func CorePostCmdMcisVm(nsId string, mcisId string, vmId string, req *McisCmdReq) (string, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	vmId = strings.ToLower(vmId)
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

	vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

	//fmt.Printf("[vmIp] " +vmIp)

	//sshKey := req.SshKey
	cmd := req.Command

	// find vaild username
	userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)

	if userName == "" {
		//return c.JSON(http.StatusInternalServerError, errors.New("No vaild username"))
		return "", fmt.Errorf("No vaild username")
	}
	if err != nil {
		//return c.JSON(http.StatusInternalServerError, errors.New("No vaild username"))
		return "", err
	}

	fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
	fmt.Println("[CMD] " + cmd)

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

func CorePostCmdMcis(nsId string, mcisId string, req *McisCmdReq) ([]SshCmdResult, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
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
			Result_array []contentSub `json:"result_array"`
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

		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
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

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		// Avoid RunSSH to not ready VM
		if err == nil {
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
	wg.Wait() //goroutine sync wg

	return resultArray, nil
}

func CorePostMcisVm(nsId string, mcisId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	vmInfoData.Name = strings.ToLower(vmInfoData.Name)
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
		return nil, fmt.Errorf("Cannot find " + common.GenMcisKey(nsId, mcisId, ""))
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

// CorePostMcisGroupVm function is a wrapper for CreateMcisGroupVm
func CorePostMcisGroupVm(nsId string, mcisId string, vmReq *TbVmReq) (*TbMcisInfo, error) {

	content, err := CreateMcisGroupVm(nsId, mcisId, vmReq)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	return content, nil
}

func CreateMcisGroupVm(nsId string, mcisId string, vmRequest *TbVmReq) (*TbMcisInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	vmRequest.Name = strings.ToLower(vmRequest.Name)
	err = common.CheckString(vmRequest.Name)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
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

func CoreGetMcisVmAction(nsId string, mcisId string, vmId string, action string) (string, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	vmId = strings.ToLower(vmId)
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

func CoreGetMcisVmStatus(nsId string, mcisId string, vmId string) (*TbVmStatusInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	vmId = strings.ToLower(vmId)
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

func CoreGetMcisVmInfo(nsId string, mcisId string, vmId string) (*TbVmInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := &TbVmInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	vmId = strings.ToLower(vmId)
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

// CreateMcis function create MCIS obeject and deploy requested VMs.
func CreateMcis(nsId string, req *TbMcisReq) (*TbMcisInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	req.Name = strings.ToLower(req.Name)
	err = common.CheckString(req.Name)
	if err != nil {
		temp := &TbMcisInfo{}
		common.CBLog.Error(err)
		return temp, err
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
		"InstallMonAgent": req.InstallMonAgent,
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

	fmt.Println("\n[MCIS has been created]")
	common.PrintJsonPretty(mcisTmp)

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisTmp.Id, req.InstallMonAgent)

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

func AddVmToMcis(wg *sync.WaitGroup, nsId string, mcisId string, vmInfoData *TbVmInfo) error {
	fmt.Printf("\n[AddVmToMcis]\n")
	//goroutin
	defer wg.Done()

	key := common.GenMcisKey(nsId, mcisId, "")
	keyValue, _ := common.CBStore.Get(key)
	if keyValue == nil {
		return fmt.Errorf("Cannot find %s", key)
	}

	configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	regionTmp, _ := common.GetRegionInfo(configTmp.RegionName)

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
	UpdateVmInfo(nsId, mcisId, *vmInfoData)

	fmt.Printf("\n[AddVmToMcis Befor request vmInfoData]\n")
	common.PrintJsonPretty(vmInfoData)

	//instanceIds, publicIPs := CreateVm(&vmInfoData)
	err := CreateVm(nsId, mcisId, vmInfoData)

	fmt.Printf("\n[AddVmToMcis After request vmInfoData]\n")
	common.PrintJsonPretty(vmInfoData)

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

	var tempSpiderVMInfo SpiderVMInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SPIDER_REST_URL + "/vm"

		method := "POST"

		fmt.Println("\n[Calling SPIDER]START")
		fmt.Println("url: " + url + " method: " + method)

		tempReq := SpiderVMReqInfoWrapper{}
		tempReq.ConnectionName = vmInfoData.ConnectionName

		tempReq.ReqInfo.Name = vmInfoData.Name

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

		tempReq.ReqInfo.VPCName = vmInfoData.VNetId //common.GetCspResourceId(nsId, common.StrVNet, vmInfoData.VNetId)
		if tempReq.ReqInfo.VPCName == "" {
			common.CBLog.Error(err)
			return err
		}

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
		fmt.Println("[Calling SPIDER]END\n")

		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			fmt.Println("body: ", string(body))
			common.CBLog.Error(err)
			return err
		}

	} else {

		// CCM API 설정
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

		tempReq.ReqInfo.Name = vmInfoData.Name

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
			fmt.Println("whoops:", err2)
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
		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
		ControlVm(nsId, mcisId, vmId, action)
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

		// Avoid concurrent requests to CSP.
		time.Sleep(time.Duration(2) * time.Second)

		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}

		go ControlVmAsync(&wg, nsId, mcisId, vmId, action, &results)
	}
	wg.Wait() //goroutine sync wg

	return nil

	//need to change status

}

type ControlVmResult struct {
	VmId   string `json:"vmId"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}
type ControlVmResultWrapper struct {
	ResultArray []ControlVmResult `json:"resultarray"`
}

func ControlVmAsync(wg *sync.WaitGroup, nsId string, mcisId string, vmId string, action string, results *ControlVmResultWrapper) error {
	defer wg.Done() //goroutine sync done

	var content struct {
		CloudId string `json:"cloudId"`
		CspVmId string `json:"cspVmId"`
	}

	fmt.Println("[ControlVm]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

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

	//fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)

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

			url = common.SPIDER_REST_URL + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=resume"
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
		fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
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

		return nil

	} else {

		// CCM API 설정
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

		var errTmp error

		resultTmp := ControlVmResult{}
		err2 := json.Unmarshal([]byte(result), &resultTmp)
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

		fmt.Println("[Calling SPIDER]END vmControl\n")

		UpdateVmPublicIp(nsId, mcisId, temp)

		return nil

	}
}

func ControlVm(nsId string, mcisId string, vmId string, action string) error {

	var content struct {
		CloudId string `json:"cloudId"`
		CspVmId string `json:"cspVmId"`
	}

	fmt.Println("[ControlVm]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	fmt.Println(key)

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

			url = common.SPIDER_REST_URL + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=resume"
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

		// CCM API 설정
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

// GetMcisObject func retrieve MCIS object from database (no current status update)
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

	for _, v := range vmList {
		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
		vmtmp, err := GetVmObject(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			return TbMcisInfo{}, err
		}
		mcisTmp.Vm = append(mcisTmp.Vm, vmtmp)
	}

	return mcisTmp, nil
}

func GetMcisStatus(nsId string, mcisId string) (McisStatusInfo, error) {

	nsId = strings.ToLower(nsId)
	err := common.CheckString(nsId)
	if err != nil {
		temp := McisStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	mcisId = strings.ToLower(mcisId)
	err = common.CheckString(mcisId)
	if err != nil {
		temp := McisStatusInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

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

	// for num, v := range vmList {
	// 	vmStatusTmp, err := GetVmStatus(nsId, mcisId, v)
	// 	if err != nil {
	// 		common.CBLog.Error(err)
	// 		vmStatusTmp.Status = StatusFailed
	// 		return mcisStatus, err
	// 	}

	// 	mcisStatus.Vm = append(mcisStatus.Vm, vmStatusTmp)

	// 	// set master IP of MCIS (Default rule: select 1st VM as master)
	// 	if num == 0 {
	// 		mcisStatus.MasterVmId = vmStatusTmp.Id
	// 		mcisStatus.MasterIp = vmStatusTmp.PublicIp
	// 		mcisStatus.MasterSSHPort = vmStatusTmp.SSHPort
	// 	}
	// }

	//goroutin sync wg
	var wg sync.WaitGroup
	for _, v := range vmList {
		wg.Add(1)
		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
		go GetVmStatusAsync(&wg, nsId, mcisId, vmId, &mcisStatus)
	}
	wg.Wait() //goroutine sync wg

	for _, v := range vmList {
		// set master IP of MCIS (Default rule: select 1st Running VM as master)
		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
		vmtmp, _ := GetVmObject(nsId, mcisId, vmId)
		if vmtmp.Status == StatusRunning {
			mcisStatus.MasterVmId = vmtmp.Id
			mcisStatus.MasterIp = vmtmp.PublicIP
			mcisStatus.MasterSSHPort = vmtmp.SSHPort
			break
		}
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

func GetMcisStatusAll(nsId string) ([]McisStatusInfo, error) {

	mcisList := ListMcisId(nsId)
	mcisStatuslist := []McisStatusInfo{}
	for _, mcisId := range mcisList {
		mcisStatus, err := GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return mcisStatuslist, err
		}
		mcisStatuslist = append(mcisStatuslist, mcisStatus)
	}
	return mcisStatuslist, nil

	//need to change status

}

func GetVmObject(nsId string, mcisId string, vmId string) (TbVmInfo, error) {
	fmt.Println("[GetVmObject] mcisId: " + mcisId + ", vmId: " + vmId)
	var key string
	if strings.Contains(vmId, "/ns/") && strings.Contains(vmId, "/mcis/") && strings.Contains(vmId, "/vm/") {
		key = vmId
	} else {
		key = common.GenMcisKey(nsId, mcisId, vmId)
	}
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	return vmTmp, nil
}

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

func GetVmStatus(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

	defer func() {
		if runtimeErr := recover(); runtimeErr != nil {
			myErr := fmt.Errorf("in GetVmStatus; mcisId: " + mcisId + ", vmId: " + vmId)
			common.CBLog.Error(myErr)
			common.CBLog.Error(runtimeErr)
		}
	}()

	fmt.Println("[GetVmStatus]" + vmId)
	var key string
	if strings.Contains(vmId, "/ns/") && strings.Contains(vmId, "/mcis/") && strings.Contains(vmId, "/vm/") {
		key = vmId
	} else {
		key = common.GenMcisKey(nsId, mcisId, vmId)
	}
	//fmt.Println(key)
	errorInfo := TbVmStatusInfo{}

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		fmt.Println(err)
		return errorInfo, err
	}

	fmt.Println(keyValue.Value)

	fmt.Println("<" + keyValue.Key + "> \n")

	fmt.Println("===============================================")

	//json.Unmarshal([]byte(keyValue.Value), &content)

	//fmt.Printf("%+v\n", content.CloudId)
	//fmt.Printf("%+v\n", content.CspVmId)

	temp := TbVmInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
		fmt.Println(err)
		return errorInfo, err
	}

	//UpdateVmPublicIp. update temp TbVmInfo{} with changed IP
	UpdateVmPublicIp(nsId, mcisId, temp)
	keyValue, _ = common.CBStore.Get(key)
	unmarshalErr = json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}

	fmt.Print("\n[Calling SPIDER] ")
	fmt.Println("CspVmId: " + temp.CspViewVmDetail.IId.NameId)
	/*
		var cspVmId string
		cspType := getVMsCspType(nsId, mcisId, vmId)
		if cspType == "AWS" {
			cspVmId = temp.CspViewVmDetail.Id
		} else {
	*/
	cspVmId := temp.CspViewVmDetail.IId.NameId

	type statusResponse struct {
		Status string
	}
	var statusResponseTmp statusResponse

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SPIDER_REST_URL + "/vmstatus/" + cspVmId
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

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return TbVmStatusInfo{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return TbVmStatusInfo{}, err
		}
		defer ccm.Close()

		result, err := ccm.GetVMStatusByParam(temp.ConnectionName, cspVmId)
		if err != nil {
			common.CBLog.Error(err)
			return TbVmStatusInfo{}, err
		}

		statusResponseTmp = statusResponse{}
		err2 := json.Unmarshal([]byte(result), &statusResponseTmp)
		if err2 != nil {
			common.CBLog.Error(err2)
			return TbVmStatusInfo{}, err2
		}
	}

	//common.PrintJsonPretty(statusResponseTmp)
	fmt.Println(statusResponseTmp)
	//fmt.Println("[Calling SPIDER]END\n")

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Id = vmId
	vmStatusTmp.Name = temp.Name
	vmStatusTmp.CspVmId = temp.CspViewVmDetail.IId.NameId
	vmStatusTmp.PublicIp = temp.PublicIP
	vmStatusTmp.SSHPort = temp.SSHPort
	vmStatusTmp.PrivateIp = temp.PrivateIP
	vmStatusTmp.NativeStatus = statusResponseTmp.Status

	vmStatusTmp.TargetAction = temp.TargetAction
	vmStatusTmp.TargetStatus = temp.TargetStatus

	vmStatusTmp.Location = temp.Location

	vmStatusTmp.MonAgentStatus = temp.MonAgentStatus

	vmStatusTmp.CreatedTime = temp.CreatedTime
	vmStatusTmp.SystemMessage = temp.SystemMessage

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
		statusResponseTmp.Status = StatusUndefined
	}

	//Correct undefined status using TargetAction
	if vmStatusTmp.TargetAction == ActionCreate {
		if statusResponseTmp.Status == StatusUndefined {
			statusResponseTmp.Status = StatusCreating
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

	// End of Temporal CODE.
	if temp.Status == StatusFailed {
		statusResponseTmp.Status = StatusFailed
	}

	vmStatusTmp.Status = statusResponseTmp.Status
	/*
		if err != nil {
			common.CBLog.Error(err)
			vmStatusTmp.Status = StatusFailed
		}
	*/
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

	vmInfoData.PublicIP = vmInfoTmp.PublicIp
	vmInfoData.SSHPort = vmInfoTmp.SSHPort

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

		url := common.SPIDER_REST_URL + "/vm/" + cspVmId
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

		statusResponseTmp = statusResponse{}

		err2 := json.Unmarshal(body, &statusResponseTmp)
		if err2 != nil {
			fmt.Println(err2)
			return errorInfo, err2
		}

	} else {

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return TbVmStatusInfo{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return TbVmStatusInfo{}, err
		}
		defer ccm.Close()

		result, err := ccm.GetVMByParam(temp.ConnectionName, cspVmId)
		if err != nil {
			common.CBLog.Error(err)
			return TbVmStatusInfo{}, err
		}

		statusResponseTmp = statusResponse{}
		err2 := json.Unmarshal([]byte(result), &statusResponseTmp)
		if err2 != nil {
			common.CBLog.Error(err2)
			return TbVmStatusInfo{}, err2
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

// func UpdateVmInfo(nsId string, mcisId string, vmInfoData TbVmInfo)
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

func GetVmIp(nsId string, mcisId string, vmId string) (string, string) {

	var content struct {
		PublicIP string `json:"publicIP"`
		SSHPort  string `json:"sshPort"`
	}

	fmt.Println("[GetVmIp]" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	fmt.Printf("%+v\n", content.PublicIP)

	return content.PublicIP, content.SSHPort
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
		var vmId string
		if strings.Contains(v, "/ns/") && strings.Contains(v, "/mcis/") && strings.Contains(v, "/vm/") {
			// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
			vmId = strings.Split(v, "/")[6]
		} else {
			// The case that v is a string in form of "vm-01".
			vmId = v
		}
		vmObj, vmErr := GetVmObject(nsId, mcisId, vmId)
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
	var vmId string
	if strings.Contains(vmList[index], "/ns/") && strings.Contains(vmList[index], "/mcis/") && strings.Contains(vmList[index], "/vm/") {
		// The case that v is a string in form of "/ns/ns-01/mcis/mcis-01/vm/vm-01".
		vmId = strings.Split(vmList[index], "/")[6]
	} else {
		// The case that v is a string in form of "vm-01".
		vmId = vmList[index]
	}
	vmObj, vmErr := GetVmObject(nsId, mcisId, vmId)
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

// GetCloudLocation. (need error handling)
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
