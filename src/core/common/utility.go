package common

import (
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	//"encoding/json"

	"github.com/cloud-barista/cb-spider/interface/api"
	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	uuid "github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	// CB-Store
	//"github.com/cloud-barista/cb-grpc-project/pkg/logging"

	//"github.com/cloud-barista/cb-tumblebug/src/core/mcir"

	"encoding/json"
	"fmt"

	//"net/http"
	//"io/ioutil"
	//"strconv"
	"github.com/go-resty/resty/v2"
)

// MCIS utilities

// JSON Simple message struct
type SimpleMsg struct {
	Message string `json:"message" example:"Any message"`
}

func GenUuid() string {
	return uuid.New().String()
}

func ToLower(name string) string {
	r, _ := regexp.Compile("_")
	out := r.ReplaceAllString(name, "-")
	out = strings.ToLower(out)
	return out
}

func GenMcisKey(nsId string, mcisId string, vmId string) string {

	if vmId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId + "/vm/" + vmId
	} else if mcisId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}

}

// Generate Mcis policy key
func GenMcisPolicyKey(nsId string, mcisId string, vmId string) string {
	if vmId != "" {
		return "/ns/" + nsId + "/policy/mcis/" + mcisId + "/vm/" + vmId
	} else if mcisId != "" {
		return "/ns/" + nsId + "/policy/mcis/" + mcisId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}
}

func LookupKeyValueList(kvl []KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

func PrintJsonPretty(v interface{}) {
	prettyJSON, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		fmt.Printf("%+v\n", v)
	} else {
		fmt.Printf("%s\n", string(prettyJSON))
	}
}

func GenResourceKey(nsId string, resourceType string, resourceId string) string {
	//resourceType = strings.ToLower(resourceType)

	if resourceType == StrImage ||
		resourceType == StrSSHKey ||
		resourceType == StrSpec ||
		resourceType == StrVNet ||
		resourceType == StrSecurityGroup {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalid_key"
	}
}

type mcirIds struct { // Tumblebug
	CspImageId           string
	CspImageName         string
	CspSshKeyName        string
	CspSpecName          string
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

/*
func GetResourcesCspType(nsId string, resourceType string, resourceId string) string {
	key := GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalid_key" {
		return "invalid nsId or resourceType or resourceId"
	}
	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return ""
	}
	if keyValue == nil {
		//CBLog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return ""
	}

	content := mcirIds{}
	json.Unmarshal([]byte(keyValue.Value), &content)

	url := SPIDER_REST_URL + "/connectionconfig/" + content.ConnectionName

	method := "GET"

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
	if err != nil {
		CBLog.Error(err)
		return "http request error"
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		CBLog.Error(err)
		return "ioutil.ReadAll error"
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		CBLog.Error(err)
		return "Cannot get VM's CSP type"
	default:

	}

	type ConnConfigInfo struct {
		ProviderName            string
	}

	temp := ConnConfigInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	return temp.ProviderName
}
*/

func GetCspResourceId(nsId string, resourceType string, resourceId string) (string, error) {
	key := GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalid_key" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return "", err
	}
	if keyValue == nil {
		//CBLog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return "", err
	}

	switch resourceType {
	case StrImage:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspImageId, nil
	case StrSSHKey:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspSshKeyName, nil
	case StrSpec:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspSpecName, nil
	case StrVNet:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspVNetName, nil // contains CspSubnetId
	// case "subnet":
	// 	content := subnetInfo{}
	// 	json.Unmarshal([]byte(keyValue.Value), &content)
	// 	return content.CspSubnetId
	case StrSecurityGroup:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspSecurityGroupName, nil
	/*
		case "publicIp":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspPublicIpName
		case "vNic":
			content := mcirIds{}
			err = json.Unmarshal([]byte(keyValue.Value), &content)
			if err != nil {
				CBLog.Error(err)
				// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
				return ""
			}
			return content.CspVNicName
	*/
	default:
		return "", fmt.Errorf("invalid resourceType")
	}
	//}
}

type ConnConfig struct { // Spider
	ConfigName     string
	ProviderName   string
	DriverName     string
	CredentialName string
	RegionName     string
}

func GetConnConfig(ConnConfigName string) (ConnConfig, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SPIDER_REST_URL + "/connectionconfig/" + ConnConfigName

		client := resty.New()

		resp, err := client.R().
			SetResult(&ConnConfig{}).
			//SetError(&SimpleMsg{}).
			Get(url)

		if err != nil {
			CBLog.Error(err)
			content := ConnConfig{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body())) // for debug

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := ConnConfig{}
			return content, err
		}

		temp, _ := resp.Result().(*ConnConfig)
		return *temp, nil

	} else {

		// CIM API 설정
		cim := api.NewCloudInfoManager()
		err := cim.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			CBLog.Error("cim failed to set config : ", err)
			return ConnConfig{}, err
		}
		err = cim.Open()
		if err != nil {
			CBLog.Error("cim api open failed : ", err)
			return ConnConfig{}, err
		}
		defer cim.Close()

		result, err := cim.GetConnectionConfigByParam(ConnConfigName)
		if err != nil {
			CBLog.Error("cim api request failed : ", err)
			return ConnConfig{}, err
		}

		temp := ConnConfig{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		return temp, nil
	}
}

type ConnConfigList struct { // Spider
	Connectionconfig []ConnConfig `json:"connectionconfig"`
}

func GetConnConfigList() (ConnConfigList, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SPIDER_REST_URL + "/connectionconfig"

		client := resty.New()

		resp, err := client.R().
			SetResult(&ConnConfigList{}).
			//SetError(&SimpleMsg{}).
			Get(url)

		if err != nil {
			CBLog.Error(err)
			content := ConnConfigList{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body())) // for debug

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := ConnConfigList{}
			return content, err
		}

		temp, _ := resp.Result().(*ConnConfigList)
		return *temp, nil

	} else {

		// CIM API 설정
		cim := api.NewCloudInfoManager()
		err := cim.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			CBLog.Error("cim failed to set config : ", err)
			return ConnConfigList{}, err
		}
		err = cim.Open()
		if err != nil {
			CBLog.Error("cim api open failed : ", err)
			return ConnConfigList{}, err
		}
		defer cim.Close()

		result, err := cim.ListConnectionConfig()
		if err != nil {
			CBLog.Error("cim api request failed : ", err)
			return ConnConfigList{}, err
		}

		temp := ConnConfigList{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		return temp, nil

	}
}

type Region struct { // Spider
	RegionName       string
	ProviderName     string
	KeyValueInfoList []KeyValue
}

func GetRegion(RegionName string) (Region, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SPIDER_REST_URL + "/region/" + RegionName

		client := resty.New()

		resp, err := client.R().
			SetResult(&Region{}).
			//SetError(&SimpleMsg{}).
			Get(url)

		if err != nil {
			CBLog.Error(err)
			content := Region{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body())) // for debug

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := Region{}
			return content, err
		}

		temp, _ := resp.Result().(*Region)
		return *temp, nil

	} else {

		// CIM API 설정
		cim := api.NewCloudInfoManager()
		err := cim.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			CBLog.Error("cim failed to set config : ", err)
			return Region{}, err
		}
		err = cim.Open()
		if err != nil {
			CBLog.Error("cim api open failed : ", err)
			return Region{}, err
		}
		defer cim.Close()

		result, err := cim.GetRegionByParam(RegionName)
		if err != nil {
			CBLog.Error("cim api request failed : ", err)
			return Region{}, err
		}

		temp := Region{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		return temp, nil

	}
}

type RegionList struct { // Spider
	Region []Region `json:"region"`
}

func GetRegionList() (RegionList, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SPIDER_REST_URL + "/region"

		client := resty.New()

		resp, err := client.R().
			SetResult(&RegionList{}).
			//SetError(&SimpleMsg{}).
			Get(url)

		if err != nil {
			CBLog.Error(err)
			content := RegionList{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body())) // for debug

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := RegionList{}
			return content, err
		}

		temp, _ := resp.Result().(*RegionList)
		return *temp, nil

	} else {

		// CIM API 설정
		cim := api.NewCloudInfoManager()
		err := cim.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			CBLog.Error("cim failed to set config : ", err)
			return RegionList{}, err
		}
		err = cim.Open()
		if err != nil {
			CBLog.Error("cim api open failed : ", err)
			return RegionList{}, err
		}
		defer cim.Close()

		result, err := cim.ListRegion()
		if err != nil {
			CBLog.Error("cim api request failed : ", err)
			return RegionList{}, err
		}

		temp := RegionList{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		return temp, nil

	}
}

// ConvertToMessage - 입력 데이터를 grpc 메시지로 변환
func ConvertToMessage(inType string, inData string, obj interface{}) error {
	//logger := logging.NewLogger()

	if inType == "yaml" {
		err := yaml.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("yaml Unmarshal: \n", obj)
	}

	if inType == "json" {
		err := json.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("json Unmarshal: \n", obj)
	}

	return nil
}

// ConvertToOutput - grpc 메시지를 출력포맷으로 변환
func ConvertToOutput(outType string, obj interface{}) (string, error) {
	//logger := logging.NewLogger()

	if outType == "yaml" {
		// 메시지 포맷에서 불필요한 필드(XXX_로 시작하는 필드)를 제거하기 위해 json 태그를 이용하여 마샬링
		j, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}

		// 필드를 소팅하지 않고 지정된 순서대로 출력하기 위해 MapSlice 이용
		jsonObj := yaml.MapSlice{}
		err2 := yaml.Unmarshal(j, &jsonObj)
		if err2 != nil {
			return "", err2
		}

		// yaml 마샬링
		y, err3 := yaml.Marshal(jsonObj)
		if err3 != nil {
			return "", err3
		}
		//logger.Debug("yaml Marshal: \n", string(y))

		return string(y), nil
	}

	if outType == "json" {
		j, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		//logger.Debug("json Marshal: \n", string(j))

		return string(j), nil
	}

	return "", nil
}

// CopySrcToDest - 소스에서 타켓으로 데이터 복사
func CopySrcToDest(src interface{}, dest interface{}) error {
	//logger := logging.NewLogger()

	j, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("source value : \n", string(j))

	err = json.Unmarshal(j, dest)
	if err != nil {
		return err
	}

	j, err = json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("target value : \n", string(j))

	return nil
}

// ConvGrpcStatusErr - GRPC 상태 코드 에러로 변환
func ConvGrpcStatusErr(err error, tag string, method string) error {
	//logger := logging.NewLogger()

	//_, fn, line, _ := runtime.Caller(1)
	runtime.Caller(1)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok {
			//logger.Error(tag, " error while calling ", method, " method: [", fn, ":", line, "] ", errStatus.Message())
			return status.Errorf(errStatus.Code(), "%s error while calling %s method: %v ", tag, method, errStatus.Message())
		}
		//logger.Error(tag, " error while calling ", method, " method: [", fn, ":", line, "] ", err)
		return status.Errorf(codes.Internal, "%s error while calling %s method: %v ", tag, method, err)
	}

	return nil
}

// NewGrpcStatusErr - GRPC 상태 코드 에러 생성
func NewGrpcStatusErr(msg string, tag string, method string) error {
	//logger := logging.NewLogger()

	//_, fn, line, _ := runtime.Caller(1)
	runtime.Caller(1)
	//logger.Error(tag, " error while calling ", method, " method: [", fn, ":", line, "] ", msg)
	return status.Errorf(codes.Internal, "%s error while calling %s method: %s ", tag, method, msg)
}

// NVL is null value logic
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

func GetChildIdList(key string) []string {

	keyValue, _ := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, strings.TrimPrefix(v.Key, key+"/"))

	}
	for _, v := range childIdList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return childIdList

}
