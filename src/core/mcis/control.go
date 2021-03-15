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
const StatusComplete string = "None"

const milkywayPort string = ":1324/milkyway/"

const SshDefaultUserName01 string = "cb-user"
const SshDefaultUserName02 string = "ubuntu"
const SshDefaultUserName03 string = "others"
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
	KeyValueList      []common.KeyValue
}

type RegionInfo struct { // Spider
	Region string
	Zone   string
}

type TbMcisReq struct {
	Name           string    `json:"name"`
	Vm             []TbVmReq `json:"vm"`
	Placement_algo string    `json:"placement_algo"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no

	Description string `json:"description"`
	Label       string `json:"label"`
}

type TbMcisInfo struct {
	Id             string     `json:"id"`
	Name           string     `json:"name"`
	Vm             []TbVmInfo `json:"vm"`
	Placement_algo string     `json:"placement_algo"`
	Description    string     `json:"description"`
	Label          string     `json:"label"`
	Status         string     `json:"status"`
	TargetStatus   string     `json:"targetStatus"`
	TargetAction   string     `json:"targetAction"`
	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no
	// Disabled for now
	//Vm             []vmOverview `json:"vm"`
}

// struct TbVmReq is to get requirements to create a new server instance.
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
	Label            string   `json:"label"`
	VmGroupSize		 string   `json:"vmGroupSize"`	// if vmGroupSize is (not empty) && (> 0), VM group will be gernetad.
}

// struct TbVmGroupInfo is to define an object that includes homogeneous VMs.
type TbVmGroupInfo struct {
	Id               string   `json:"id"`
	Name             string   `json:"name"`
	VmId             []string `json:"vmId"`
	VmGroupSize		 string   `json:"vmGroupSize"`
}

// struct TbVmGroupInfo is to define a server instance object
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
	Label            string   `json:"label"`
	VmGroupId        string   `json:"vmGroupId"`	// defined if the VM is in a group
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

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

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
	MasterVmId   string           `json:"masterVmId" example:"vm-asiaeast1-cb-01"`
	MasterIp     string           `json:"masterIp" example:"32.201.134.113"`
	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no
}

type TbVmStatusInfo struct {
	Id            string      `json:"id"`
	Csp_vm_id     string      `json:"csp_vm_id"`
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	TargetStatus  string      `json:"targetStatus"`
	TargetAction  string      `json:"targetAction"`
	Native_status string      `json:"native_status"`
	Public_ip     string      `json:"public_ip"`
	Location      GeoLocation `json:"location"`
	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed
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

	for i := 0; i < 100; i++ {
		for _, v := range userNames {
			fmt.Println("[SSH] " + "(" + vmIp + ")" + "with userName:" + v)
			fmt.Println("[CMD] " + cmd)
			if v != "" {
				result, err := RunSSH(vmIp, v, privateKey, cmd)
				if err != nil {
					fmt.Println("[ERR: result] " + "[ERR: err] " + err.Error())
				}
				if err == nil {
					theUserName = v
					fmt.Println("[RST] " + *result + "[Username] " + v)
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
		if theUserName != "" {
			break
		}
		fmt.Println("[Trying a SSH] trial:" + strconv.Itoa(i))
		time.Sleep(1 * time.Second)
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

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
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
		userNames := []string{userName, req.User_name, SshDefaultUserName01, SshDefaultUserName02, SshDefaultUserName03, SshDefaultUserName04}
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

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
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
		//mapError := map[string]string{"message": "Benchmark Error"}
		//return c.JSON(http.StatusFailedDependency, &mapError)
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

func CoreGetBenchmark(nsId string, mcisId string, action string, host string) (*BenchmarkInfoArray, error) {

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
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

	var err error
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

	nsId = common.ToLower(nsId)

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

	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)

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

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return err
	}

	fmt.Println("[Delete MCIS] " + mcisId)

	// ControlMcis first
	err := ControlMcis(nsId, mcisId, ActionTerminate)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished
	// Sleep for 5 seconds
	fmt.Printf("\n\n[Info] Sleep for 20 seconds for safe MCIS-VMs termination.\n\n")
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
		vmKey := common.GenMcisKey(nsId, mcisId, v)
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
	// delete mcis info
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil
}

func DelMcisVm(nsId string, mcisId string, vmId string) error {

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmId)
	//vmId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmId = common.ToLower(vmId)
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err
	}

	fmt.Println("[Delete VM] " + vmId)

	// ControlVm first
	err := ControlVm(nsId, mcisId, vmId, ActionTerminate)

	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	// for deletion, need to wait untill termination is finished
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
		vmPriorityTmp.Vm_spec = content2
		vmPriorityList = append(vmPriorityList, vmPriorityTmp)
	}

	fmt.Println("===============================================")
	return vmPriorityList, err

	//requires error handling

}

// MCIS Control

func CorePostMcis(nsId string, req *TbMcisReq) (*TbMcisInfo, error) {

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, req.Name)
	//req.Name = lowerizedName
	nsId = common.ToLower(nsId)
	req.Name = common.ToLower(req.Name)
	check, _ := CheckMcis(nsId, req.Name)

	if check {
		temp := &TbMcisInfo{}
		err := fmt.Errorf("The mcis " + req.Name + " already exists.")
		return temp, err
	}

	key := CreateMcis(nsId, req)
	mcisId := common.ToLower(req.Name)

	keyValue, _ := common.CBStore.Get(key)

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
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
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
		vmTmp.Id = v
		content.Vm = append(content.Vm, vmTmp)
	}

	//mcisStatus, err := GetMcisStatus(nsId, mcisId)
	//content.Status = mcisStatus.Status

	return &content, nil
}

func CoreGetMcisAction(nsId string, mcisId string, action string) (string, error) {

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
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

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
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

func CoreGetMcisInfo(nsId string, mcisId string) (*TbMcisInfo, error) {

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &TbMcisInfo{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	/*
		var content struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string          `json:"status"`
			TargetStatus   string          `json:"targetStatus"`
			TargetAction   string          `json:"targetAction"`
			Vm             []mcis.TbVmInfo `json:"vm"`
			Placement_algo string          `json:"placement_algo"`
			Description    string          `json:"description"`
		}
	*/
	content := TbMcisInfo{}

	fmt.Println("[Get MCIS for id]" + mcisId)
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	if keyValue == nil {
		//mapA := map[string]string{"message": "Cannot find " + key}
		//return c.JSON(http.StatusOK, &mapA)
		return nil, fmt.Errorf("Cannot find " + key)
	}
	//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	//fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)

	mcisStatus, err := GetMcisStatus(nsId, mcisId)
	content.Status = mcisStatus.Status

	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}

	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
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
		vmTmp.Id = v

		//get current vm status
		vmStatusInfoTmp, err := GetVmStatus(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
		}
		vmTmp.Status = vmStatusInfoTmp.Status
		vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
		vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

		content.Vm = append(content.Vm, vmTmp)
	}

	return &content, nil
}

func CoreGetAllMcis(nsId string, option string) ([]TbMcisInfo, error) {

	nsId = common.ToLower(nsId)

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
			return nil, fmt.Errorf("Cannot find " + key)
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
			vmKey := common.GenMcisKey(nsId, mcisId, v1)
			//fmt.Println(vmKey)
			vmKeyValue, _ := common.CBStore.Get(vmKey)
			if vmKeyValue == nil {
				//mapA := map[string]string{"message": "Cannot find " + key}
				//return c.JSON(http.StatusOK, &mapA)
				return nil, fmt.Errorf("Cannot find " + key)
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

	nsId = common.ToLower(nsId)

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

	nsId = common.ToLower(nsId)

	/*
		var content struct {
			//Vm_req          []TbVmRecommendReq    `json:"vm_req"`
			Vm_recommend    []mcis.TbVmRecommendInfo `json:"vm_recommend"`
			Placement_algo  string                   `json:"placement_algo"`
			Placement_param []common.KeyValue        `json:"placement_param"`
		}
	*/
	//content := RestPostMcisRecommandResponse{}
	//content.Vm_req = req.Vm_req
	//content.Placement_algo = req.Placement_algo
	//content.Placement_param = req.Placement_param

	Vm_recommend := []TbVmRecommendInfo{}

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

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmId)
	//vmId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmId = common.ToLower(vmId)
	check, _ := CheckVm(nsId, mcisId, vmId)

	if !check {
		err := fmt.Errorf("The vm " + vmId + " does not exist.")
		return err.Error(), err
	}

	vmIp := GetVmIp(nsId, mcisId, vmId)

	//fmt.Printf("[vmIp] " +vmIp)

	//sshKey := req.Ssh_key
	cmd := req.Command

	// find vaild username
	userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
	userNames := []string{
		userName,
		req.User_name,
		SshDefaultUserName01,
		SshDefaultUserName02,
		SshDefaultUserName03,
		SshDefaultUserName04,
	}
	userName = VerifySshUserName(vmIp, userNames, sshKey)
	if userName == "" {
		//return c.JSON(http.StatusInternalServerError, errors.New("No vaild username"))
		return "", fmt.Errorf("No vaild username")
	}

	//fmt.Printf("[userName] " +userName)

	fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
	fmt.Println("[CMD] " + cmd)

	if result, err := RunSSH(vmIp, userName, sshKey, cmd); err != nil {
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

	//check, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := []SshCmdResult{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	/*
		type contentSub struct {
			Mcis_id string `json:"mcis_id"`
			Vm_id   string `json:"vm_id"`
			Vm_ip   string `json:"vm_ip"`
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
		userNames := []string{
			userName,
			req.User_name,
			SshDefaultUserName01,
			SshDefaultUserName02,
			SshDefaultUserName03,
			SshDefaultUserName04,
		}
		userName = VerifySshUserName(vmIp, userNames, sshKey)

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		go RunSSHAsync(&wg, vmId, vmIp, userName, sshKey, cmd, &resultArray)

	}
	wg.Wait() //goroutine sync wg

	return resultArray, nil
}

func CorePostMcisVm(nsId string, mcisId string, vmInfoData *TbVmInfo) (*TbVmInfo, error) {

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmInfoData.Name)
	//vmInfoData.Name = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmInfoData.Name = common.ToLower(vmInfoData.Name)
	check, _ := CheckVm(nsId, mcisId, vmInfoData.Name)

	if check {
		temp := &TbVmInfo{}
		err := fmt.Errorf("The vm " + vmInfoData.Name + " already exists.")
		return temp, err
	}

	targetAction := ActionCreate
	targetStatus := StatusRunning

	vmInfoData.Id = common.ToLower(vmInfoData.Name)
	vmInfoData.PublicIP = "Not assigned yet"
	vmInfoData.PublicDNS = "Not assigned yet"
	vmInfoData.TargetAction = targetAction
	vmInfoData.TargetStatus = targetStatus
	vmInfoData.Status = StatusCreating

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	//CreateMcis(nsId, req)
	//err := AddVmToMcis(nsId, mcisId, vmInfoData)

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
			reqToMon.User_name = "ubuntu" // this MCIS user name is temporal code. Need to improve.

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

func CoreGetMcisVmAction(nsId string, mcisId string, vmId string, action string) (string, error) {

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmId)
	//vmId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmId = common.ToLower(vmId)
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

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmId)
	//vmId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmId = common.ToLower(vmId)
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

	//check, lowerizedName, _ := LowerizeAndCheckVm(nsId, mcisId, vmId)
	//vmId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)
	vmId = common.ToLower(vmId)
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
	//req.Id = common.ToLower(req.Name)
	mcisId := common.ToLower(req.Name)
	vmRequest := req.Vm

	fmt.Println("=========================== Create MCIS object")
	key := common.GenMcisKey(nsId, mcisId, "")
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
	//wg.Add(len(vmRequest))

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
				vmGroupInfoData.VmId = append(vmGroupInfoData.VmId, vmGroupInfoData.Id + "-" + strconv.Itoa(i) )
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

			if vmGroupSize == 0 { 			// for VM (not in a group)
				vmInfoData.Name = common.ToLower(k.Name) 
			} else { 						// for VM (in a group)
				if i == vmGroupSize {
					break	// if vmGroupSize != 0 && vmGroupSize == i, skip the final loop
				}
				vmInfoData.VmGroupId = common.ToLower(k.Name)
				vmInfoData.Name = common.ToLower(k.Name) + "-" + strconv.Itoa(i)
				fmt.Println("===========================")
				fmt.Println("vmInfoData.Name: " + vmInfoData.Name)
				fmt.Println("===========================")

			}
			vmInfoData.Id = vmInfoData.Name	

			vmInfoData.Description = k.Description
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
			//vmInfoData.Vnic_id = k.Vnic_id
			//vmInfoData.Public_ip_id = k.Public_ip_id
			vmInfoData.SecurityGroupIds = k.SecurityGroupIds
			vmInfoData.SshKeyId = k.SshKeyId
			vmInfoData.Description = k.Description

			vmInfoData.VmUserAccount = k.VmUserAccount
			vmInfoData.VmUserPassword = k.VmUserPassword

			wg.Add(1)
			go AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)
			//AddVmToMcis(nsId, req.Id, vmInfoData)

			if err != nil {
				errMsg := "Failed to add VM " + vmInfoData.Name + " to MCIS " + req.Name
				return errMsg
			}
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

	// Install CB-Dragonfly monitoring agent

	fmt.Printf("\n[Init monitoring agent] for %+v\n - req.InstallMonAgent: %+v\n\n", mcisTmp.Id, req.InstallMonAgent)

	mcisTmp.InstallMonAgent = req.InstallMonAgent
	UpdateMcisInfo(nsId, mcisTmp)

	if req.InstallMonAgent != "no" {

		// Sleep for 60 seconds for a safe DF agent installation.
		fmt.Printf("\n\n[Info] Sleep for 60 seconds for safe CB-Dragonfly Agent installation.\n\n")
		time.Sleep(60 * time.Second)

		check := CheckDragonflyEndpoint()
		if check != nil {
			fmt.Printf("\n\n[Warring] CB-Dragonfly is not available\n\n")
		} else {
			reqToMon := &McisCmdReq{}
			reqToMon.User_name = "ubuntu" // this MCIS user name is temporal code. Need to improve.

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
	// Monitoring Agent Installation Status (init: notInstalled)
	vmInfoData.MonAgentStatus = "notInstalled"

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

		fmt.Println("\n\n[Calling SPIDER]START")
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

		tempSpiderVMInfo = SpiderVMInfo{} // FYI; SpiderVMInfo: the struct in CB-Spider
		err2 := json.Unmarshal(body, &tempSpiderVMInfo)

		if err2 != nil {
			fmt.Println("whoops:", err2)
			fmt.Println(err)
			common.CBLog.Error(err)
			return err
		}

		fmt.Println("[Response from SPIDER]")
		common.PrintJsonPretty(tempSpiderVMInfo)
		fmt.Println("[Calling SPIDER]END\n\n")

		fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
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

		fmt.Println("\n\n[Calling SPIDER]START")

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

	//vmInfoData.Vcpu_size = vmInfoData.Vcpu_size
	//vmInfoData.Memory_size = vmInfoData.Memory_size
	//vmInfoData.Disk_size = vmInfoData.Disk_size
	//vmInfoData.Disk_type = vmInfoData.Disk_type

	//vmInfoData.Placement_algo = vmInfoData.Placement_algo

	// 2. Provided by CB-Spider
	//vmInfoData.CspVmId = temp.Id
	//vmInfoData.StartTime = temp.StartTime
	vmInfoData.Region = tempSpiderVMInfo.Region
	vmInfoData.PublicIP = tempSpiderVMInfo.PublicIP
	vmInfoData.PublicDNS = tempSpiderVMInfo.PublicDNS
	vmInfoData.PrivateIP = tempSpiderVMInfo.PrivateIP
	vmInfoData.PrivateDNS = tempSpiderVMInfo.PrivateDNS
	vmInfoData.VMBootDisk = tempSpiderVMInfo.VMBootDisk
	vmInfoData.VMBlockDisk = tempSpiderVMInfo.VMBlockDisk
	//vmInfoData.KeyValueList = temp.KeyValueList

	configTmp, _ := common.GetConnConfig(vmInfoData.ConnectionName)
	vmInfoData.Location = GetCloudLocation(strings.ToLower(configTmp.ProviderName), strings.ToLower(tempSpiderVMInfo.Region.Region))

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

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := ""
		method := ""
		switch action {
		case ActionTerminate:

			temp.TargetAction = ActionTerminate
			temp.TargetStatus = StatusTerminated
			temp.Status = StatusTerminating

			//url = common.SPIDER_REST_URL + "/vm/" + cspVmId + "?connection_name=" + temp.ConnectionName
			url = common.SPIDER_REST_URL + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=reboot"
			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=suspend"
			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
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

		fmt.Println("[Calling SPIDER]END vmControl\n\n")

		UpdateVmPublicIp(nsId, mcisId, temp)

		return nil

	}
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

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := ""
		method := ""
		switch action {
		case ActionTerminate:

			temp.TargetAction = ActionTerminate
			temp.TargetStatus = StatusTerminated
			temp.Status = StatusTerminating

			//url = common.SPIDER_REST_URL + "/vm/" + cspVmId + "?connection_name=" + temp.ConnectionName
			url = common.SPIDER_REST_URL + "/vm/" + cspVmId
			method = "DELETE"
		case ActionReboot:

			temp.TargetAction = ActionReboot
			temp.TargetStatus = StatusRunning
			temp.Status = StatusRebooting

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=reboot"
			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=reboot"
			method = "GET"
		case ActionSuspend:

			temp.TargetAction = ActionSuspend
			temp.TargetStatus = StatusSuspended
			temp.Status = StatusSuspending

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=suspend"
			url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?action=suspend"
			method = "GET"
		case ActionResume:

			temp.TargetAction = ActionResume
			temp.TargetStatus = StatusRunning
			temp.Status = StatusResuming

			//url = common.SPIDER_REST_URL + "/controlvm/" + cspVmId + "?connection_name=" + temp.ConnectionName + "&action=resume"
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
		fmt.Println("[Calling SPIDER]END vmControl\n\n")

		return nil
	}
}

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
	return mcisTmp, nil
}

func GetMcisStatus(nsId string, mcisId string) (McisStatusInfo, error) {

	//_, lowerizedName, _ := LowerizeAndCheckMcis(nsId, mcisId)
	//mcisId = lowerizedName
	nsId = common.ToLower(nsId)
	mcisId = common.ToLower(mcisId)

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

	for num, v := range vmList {
		vmStatusTmp, err := GetVmStatus(nsId, mcisId, v)
		if err != nil {
			common.CBLog.Error(err)
			vmStatusTmp.Status = StatusFailed
			return mcisStatus, err
		}

		mcisStatus.Vm = append(mcisStatus.Vm, vmStatusTmp)

		// set master IP of MCIS (Default rule: select 1st VM as master)
		if num == 0 {
			mcisStatus.MasterVmId = vmStatusTmp.Id
			mcisStatus.MasterIp = vmStatusTmp.Public_ip
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
	fmt.Println("[GetVmObject]" + mcisId + ", VM:" + vmId)
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return TbVmInfo{}, err
	}
	vmTmp := TbVmInfo{}
	json.Unmarshal([]byte(keyValue.Value), &vmTmp)
	return vmTmp, nil
}

func GetVmStatus(nsId string, mcisId string, vmId string) (TbVmStatusInfo, error) {

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

	type statusResponse struct {
		Status string
	}
	var statusResponseTmp statusResponse

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SPIDER_REST_URL + "/vmstatus/" + cspVmId // + "?connection_name=" + temp.ConnectionName
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

	vmStatusTmp.Location = temp.Location

	vmStatusTmp.MonAgentStatus = temp.MonAgentStatus

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

	type statusResponse struct {
		Status   string
		PublicIP string
	}
	var statusResponseTmp statusResponse

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SPIDER_REST_URL + "/vm/" + cspVmId // + "?connection_name=" + temp.ConnectionName
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

	common.PrintJsonPretty(statusResponseTmp)
	fmt.Println("[Calling SPIDER]END\n\n")

	vmStatusTmp := TbVmStatusInfo{}
	vmStatusTmp.Public_ip = statusResponseTmp.PublicIP

	return vmStatusTmp, nil

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

	sshKey := common.GenResourceKey(nsId, common.StrSSHKey, content.SshKeyId)
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
