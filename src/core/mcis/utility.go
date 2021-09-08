package mcis

import (
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"fmt"
	"strconv"
	"strings"
	"sync"

	//"fmt"
	//"net/http"
	//"io/ioutil"
	//"strconv"

	// CB-Store

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	//"github.com/cloud-barista/cb-spider/cloud-control-manager/vm-ssh"
	//"github.com/cloud-barista/cb-tumblebug/src/core/mcism"
	//"github.com/cloud-barista/cb-tumblebug/src/core/common"

	"github.com/go-resty/resty/v2"

	"reflect"

	validator "github.com/go-playground/validator/v10"
)

// CB-Store
//var cblog *logrus.Logger
//var store icbs.Store

//var SPIDER_REST_URL string

// use a single instance of Validate, it caches struct info
var validate *validator.Validate

func init() {
	//cblog = config.Cblogger
	//store = cbstore.GetStore()
	//SPIDER_REST_URL = os.Getenv("SPIDER_REST_URL")

	validate = validator.New()

	// register function to get tag name from json tags.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// register validation for 'Tb*Req'
	// NOTE: only have to register a non-pointer type for 'Tb*Req', validator
	// internally dereferences during it's type checks.

	validate.RegisterStructValidation(TbMcisReqStructLevelValidation, TbMcisReq{})
	validate.RegisterStructValidation(TbVmReqStructLevelValidation, TbVmReq{})
	validate.RegisterStructValidation(TbMcisCmdReqStructLevelValidation, McisCmdReq{})
	// validate.RegisterStructValidation(TbMcisRecommendReqStructLevelValidation, McisRecommendReq{})
	// validate.RegisterStructValidation(TbVmRecommendReqStructLevelValidation, TbVmRecommendReq{})
	// validate.RegisterStructValidation(TbBenchmarkReqStructLevelValidation, BenchmarkReq{})
	// validate.RegisterStructValidation(TbMultihostBenchmarkReqStructLevelValidation, MultihostBenchmarkReq{})

	validate.RegisterStructValidation(DFMonAgentInstallReqStructLevelValidation, MonAgentInstallReq{})

}

/*
func genUuid() string {
	return uuid.New().String()
}
*/

/*
type mcirIds struct {
	CspImageId           string
	CspImageName         string
	CspSshKeyName        string
	Name                 string // Spec
	CspVNetId            string
	CspVNetName          string
	CspSecurityGroupId   string
	CspSecurityGroupName string
	CspPublicIpId        string
	CspPublicIpName      string
	CspVNicId            string
	CspVNicName          string

	ConnectionName string
}
*/

func CheckMcis(nsId string, mcisId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMcis failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckMcis failed; mcisId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check mcis] " + mcisId)

	//key := "/ns/" + nsId + "/mcis/" + mcisId
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func CheckVm(nsId string, mcisId string, vmId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckVm failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckVm failed; mcisId given is null.")
		return false, err
	} else if vmId == "" {
		err := fmt.Errorf("CheckVm failed; vmId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	err = common.CheckString(vmId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check vm] " + mcisId + ", " + vmId)

	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func CheckMcisPolicy(nsId string, mcisId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMcis failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckMcis failed; mcisId given is null.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	fmt.Println("[Check McisPolicy] " + mcisId)

	//key := "/ns/" + nsId + "/mcis/" + mcisId
	key := common.GenMcisPolicyKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)

	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func RunSSH(vmIP string, sshPort string, userName string, privateKey string, cmd string) (*string, error) {

	// VM SSH 접속정보 설정 (외부 연결 정보, 사용자 아이디, Private Key)
	serverEndpoint := fmt.Sprintf("%s:%s", vmIP, sshPort)
	sshInfo := SSHInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// VM SSH 명령어 실행
	if result, err := SSHRun(sshInfo, cmd); err != nil {
		return nil, err
	} else {
		return &result, nil
	}
}

func RunSSHAsync(wg *sync.WaitGroup, vmID string, vmIP string, sshPort string, userName string, privateKey string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	// VM SSH 접속정보 설정 (외부 연결 정보, 사용자 아이디, Private Key)
	serverEndpoint := fmt.Sprintf("%s:%s", vmIP, sshPort)
	sshInfo := SSHInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// VM SSH 명령어 실행
	result, err := SSHRun(sshInfo, cmd)

	//wg.Done() //goroutin sync done

	sshResultTmp := SshCmdResult{}
	sshResultTmp.McisId = ""
	sshResultTmp.VmId = vmID
	sshResultTmp.VmIp = vmIP

	if err != nil {
		sshResultTmp.Result = err.Error()
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		fmt.Println("cmd result " + result)
		sshResultTmp.Result = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
	}

}

func TrimIP(sshAccessPoint string) (string, error) {
	splitted := strings.Split(sshAccessPoint, ":")
	if len(splitted) != 2 {
		err := fmt.Errorf("In TrimIP(), sshAccessPoint does not seem 8.8.8.8:22 form.")
		return strconv.Itoa(0), err
	}
	portString := splitted[1]
	port, err := strconv.Atoi(portString)
	if err != nil {
		err := fmt.Errorf("In TrimIP(), strconv.Atoi returned an error.")
		return strconv.Itoa(0), err
	}
	if port >= 1 && port <= 65535 { // valid port number
		return portString, nil
	} else {
		err := fmt.Errorf("In TrimIP(), detected port number seems wrong: " + portString)
		return strconv.Itoa(0), err
	}
}

type SpiderNameIdSystemId struct {
	NameId   string
	SystemId string
}

type SpiderAllListWrapper struct {
	AllList SpiderAllList
}

type SpiderAllList struct {
	MappedList     []SpiderNameIdSystemId
	OnlySpiderList []SpiderNameIdSystemId
	OnlyCSPList    []SpiderNameIdSystemId
}

// Response struct for InspectResources
type TbInspectResourcesResponse struct {
	// ResourcesOnCsp       interface{} `json:"resourcesOnCsp"`
	// ResourcesOnSpider    interface{} `json:"resourcesOnSpider"`
	// ResourcesOnTumblebug interface{} `json:"resourcesOnTumblebug"`
	ResourcesOnCsp       []resourceOnCspOrSpider `json:"resourcesOnCsp"`
	ResourcesOnSpider    []resourceOnCspOrSpider `json:"resourcesOnSpider"`
	ResourcesOnTumblebug []resourceOnTumblebug   `json:"resourcesOnTumblebug"`
}

type resourceOnCspOrSpider struct {
	Id          string `json:"id"`
	CspNativeId string `json:"cspNativeId"`
}

type resourceOnTumblebug struct {
	Id          string `json:"id"`
	CspNativeId string `json:"cspNativeId"`
	NsId        string `json:"nsId"`
	McisId      string `json:"mcisId"`
	Type        string `json:"type"`
	ObjectKey   string `json:"objectKey"`
}

// InspectVMs returns the state list of TB VM objects of given connConfig
func InspectVMs(connConfig string) (interface{}, error) {

	nsList, err := common.ListNsId()
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return nil, err
	}
	// var TbResourceList []string
	var TbResourceList []resourceOnTumblebug
	for _, ns := range nsList {

		mcisListinNs, _ := ListMcisId(ns)
		if mcisListinNs == nil {
			continue
		}

		for _, mcis := range mcisListinNs {
			vmListInMcis, err := ListVmId(ns, mcis)
			if err != nil {
				common.CBLog.Error(err)
				err := fmt.Errorf("an error occurred while getting resource list")
				return nil, err
			}
			if vmListInMcis == nil {
				continue
			}

			for _, vmId := range vmListInMcis {
				vm, err := GetVmObject(ns, mcis, vmId)
				if err != nil {
					common.CBLog.Error(err)
					err := fmt.Errorf("an error occurred while getting resource list")
					return nil, err
				}

				if vm.ConnectionName == connConfig { // filtering
					temp := resourceOnTumblebug{}
					temp.Id = vm.Id
					temp.CspNativeId = vm.CspViewVmDetail.IId.SystemId
					temp.NsId = ns
					temp.McisId = mcis
					temp.Type = "vm"
					temp.ObjectKey = common.GenMcisKey(ns, mcis, vm.Id)

					TbResourceList = append(TbResourceList, temp)
				}
			}
		}
	}

	client := resty.New().SetCloseConnection(true)
	client.SetAllowGetMethodPayload(true)

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}
	tempReq.ConnectionName = connConfig

	spiderRequestURL := common.SpiderRestUrl + "/allvm"

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(tempReq).
		SetResult(&SpiderAllListWrapper{}). // or SetResult(AuthSuccess{}).
		//SetError(&AuthError{}).       // or SetError(AuthError{}).
		Get(spiderRequestURL)

	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return nil, err
	}

	fmt.Println("HTTP Status code: " + strconv.Itoa(resp.StatusCode()))
	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		err := fmt.Errorf(string(resp.Body()))
		common.CBLog.Error(err)
		return nil, err
	default:
	}

	temp, _ := resp.Result().(*SpiderAllListWrapper) // type assertion

	result := TbInspectResourcesResponse{}

	/*
		// Implementation style 1
		if len(TbResourceList) > 0 {
			result.ResourcesOnTumblebug = TbResourceList
		} else {
			result.ResourcesOnTumblebug = []resourceOnTumblebug{}
		}
	*/
	// Implementation style 2
	result.ResourcesOnTumblebug = []resourceOnTumblebug{}
	result.ResourcesOnTumblebug = append(result.ResourcesOnTumblebug, TbResourceList...)

	// result.ResourcesOnCsp = append((*temp).AllList.MappedList, (*temp).AllList.OnlyCSPList...)
	// result.ResourcesOnSpider = append((*temp).AllList.MappedList, (*temp).AllList.OnlySpiderList...)
	result.ResourcesOnCsp = []resourceOnCspOrSpider{}
	result.ResourcesOnSpider = []resourceOnCspOrSpider{}

	for _, v := range (*temp).AllList.MappedList {
		tmpObj := resourceOnCspOrSpider{}
		tmpObj.Id = v.NameId
		tmpObj.CspNativeId = v.SystemId

		result.ResourcesOnCsp = append(result.ResourcesOnCsp, tmpObj)
		result.ResourcesOnSpider = append(result.ResourcesOnSpider, tmpObj)
	}

	for _, v := range (*temp).AllList.OnlySpiderList {
		tmpObj := resourceOnCspOrSpider{}
		tmpObj.Id = v.NameId
		tmpObj.CspNativeId = v.SystemId

		result.ResourcesOnSpider = append(result.ResourcesOnSpider, tmpObj)
	}

	for _, v := range (*temp).AllList.OnlyCSPList {
		tmpObj := resourceOnCspOrSpider{}
		tmpObj.Id = v.NameId
		tmpObj.CspNativeId = v.SystemId

		result.ResourcesOnCsp = append(result.ResourcesOnCsp, tmpObj)
	}

	return result, nil
}
