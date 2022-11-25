/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"bufio"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-spider/interface/api"
	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	uid "github.com/rs/xid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	"encoding/csv"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// MCIS utilities

// SimpleMsg is struct for JSON Simple message
type SimpleMsg struct {
	Message string `json:"message" example:"Any message"`
}

// GenUid is func to return a UUID string
func GenUid() string {
	return uid.New().String()
}

// GenRandomPassword is func to return a RandomPassword
func GenRandomPassword() string {
	rand.Seed(time.Now().Unix())

	charset := "ABCDEFG*!$"
	shuff := []rune(charset)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})

	pw := uid.New().String() + string(shuff)

	return pw
}

// RandomSleep is func to make a caller waits for during random time seconds (random value within x~y)
func RandomSleep(from int, to int) {
	if from > to {
		tmp := from
		from = to
		to = tmp
	}
	t := to - from
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(t * 1000)
	time.Sleep(time.Duration(n) * time.Millisecond)
}

// GetFuncName is func to get the name of the running function
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

// CheckString is func to check string by the given rule `[a-z]([-a-z0-9]*[a-z0-9])?`
func CheckString(name string) error {

	if name == "" {
		err := fmt.Errorf("The provided string is empty")
		return err
	}

	r, _ := regexp.Compile("[a-z]([-a-z0-9]*[a-z0-9])?")
	filtered := r.FindString(name)

	if filtered != name {
		err := fmt.Errorf(name + ": The first character of name must be a lowercase letter, and all following characters must be a dash, lowercase letter, or digit, except the last character, which cannot be a dash.")
		return err
	}

	return nil
}

// ToLower is func to change strings (_ to -, " " to -, to lower string ) (deprecated soon)
func ToLower(name string) string {
	out := strings.ReplaceAll(name, "_", "-")
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ToLower(out)
	return out
}

// ChangeIdString is func to change strings in id or name (special chars to -, to lower string )
func ChangeIdString(name string) string {
	// Regex for letters and numbers
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	changedString := strings.ToLower(reg.ReplaceAllString(name, "-"))
	if changedString[len(changedString)-1:] == "-" {
		changedString += "r"
	}
	return changedString
}

// GenMcisKey is func to generate a key used in keyValue store
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

// GenMcisSubGroupKey is func to generate a key from subGroupId used in keyValue store
func GenMcisSubGroupKey(nsId string, mcisId string, groupId string) string {

	return "/ns/" + nsId + "/mcis/" + mcisId + "/subgroup/" + groupId

}

// GenMcisPolicyKey is func to generate Mcis policy key
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

// LookupKeyValueList is func to lookup KeyValue list
func LookupKeyValueList(kvl []KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

// PrintJsonPretty is func to print JSON pretty with indent
func PrintJsonPretty(v interface{}) {
	prettyJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%+v\n", v)
	} else {
		fmt.Printf("%s\n", string(prettyJSON))
	}
}

// GenResourceKey is func to generate a key from resource type and id
func GenResourceKey(nsId string, resourceType string, resourceId string) string {

	if resourceType == StrImage ||
		resourceType == StrCustomImage ||
		resourceType == StrSSHKey ||
		resourceType == StrSpec ||
		resourceType == StrVNet ||
		resourceType == StrSecurityGroup ||
		resourceType == StrDataDisk {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalidKey"
	}
}

// GenChildResourceKey is func to generate a key from resource type and id
func GenChildResourceKey(nsId string, resourceType string, parentResourceId string, resourceId string) string {

	if resourceType == StrSubnet {
		parentResourceType := StrVNet
		// return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
		return fmt.Sprintf("/ns/%s/resources/%s/%s/%s/%s", nsId, parentResourceType, parentResourceId, resourceType, resourceId)
	} else {
		return "/invalidKey"
	}
}

// mcirIds is struct for containing id and name of each MCIR type
type mcirIds struct { // Tumblebug
	CspImageId           string
	CspImageName         string
	CspCustomImageId     string
	CspCustomImageName   string
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
	CspDataDiskId        string
	CspDataDiskName      string

	ConnectionName string
}

// GetCspResourceId is func to retrieve CSP native resource ID
func GetCspResourceId(nsId string, resourceType string, resourceId string) (string, error) {
	key := GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalidKey" {
		return "", fmt.Errorf("invalid nsId or resourceType or resourceId")
	}
	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		return "", err
	}
	if keyValue == nil {
		//CBLog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent function
		return "", fmt.Errorf("cannot find the key " + key)
	}

	switch resourceType {
	case StrImage:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspImageId, nil
	case StrCustomImage:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspCustomImageName, nil
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
	case StrDataDisk:
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspDataDiskName, nil
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
				// if there is no matched value for the key, return empty string. Error will be handled in a parent function
				return ""
			}
			return content.CspVNicName
	*/
	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}

// ConnConfig is struct for containing a CB-Spider struct for connection config
type ConnConfig struct { // Spider
	ConfigName     string
	ProviderName   string
	DriverName     string
	CredentialName string
	RegionName     string
	Location       GeoLocation
}

// GeoLocation is struct for geographical location
type GeoLocation struct {
	Latitude     string `json:"latitude"`
	Longitude    string `json:"longitude"`
	BriefAddr    string `json:"briefAddr"`
	CloudType    string `json:"cloudType"`
	NativeRegion string `json:"nativeRegion"`
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

	keyValue, err := CBStore.Get(key)

	if err != nil {
		CBLog.Error(err)
		return location
	}

	if keyValue == nil {
		file, fileErr := os.Open("../assets/cloudlocation.csv")
		defer file.Close()
		if fileErr != nil {
			CBLog.Error(fileErr)
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
			dbErr := CBStore.Put(keyLoc, string(valLoc))
			if dbErr != nil {
				CBLog.Error(dbErr)
				return location
			}
			for j := range row {
				fmt.Printf("%s ", rows[i][j])
			}
			fmt.Println()
		}
		keyValue, err = CBStore.Get(key)
		if err != nil {
			CBLog.Error(err)
			return location
		}
	}

	if keyValue != nil {
		fmt.Printf("[GetCloudLocation] %+v %+v\n", keyValue.Key, keyValue.Value)
		err = json.Unmarshal([]byte(keyValue.Value), &location)
		if err != nil {
			CBLog.Error(err)
			return location
		}
	}

	return location
}

// GetConnConfig is func to get connection config from CB-Spider
func GetConnConfig(ConnConfigName string) (ConnConfig, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SpiderRestUrl + "/connectionconfig/" + ConnConfigName

		client := resty.New().SetCloseConnection(true)

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

		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := ConnConfig{}
			return content, err
		}

		temp, _ := resp.Result().(*ConnConfig)

		// Get geolocation
		nativeRegion, err := GetNativeRegion(temp.ConfigName)
		if err != nil {
			CBLog.Error(err)
			content := ConnConfig{}
			return content, err
		}

		location := GetCloudLocation(strings.ToLower(temp.ProviderName), strings.ToLower(nativeRegion))
		temp.Location = location

		return *temp, nil

	} else {

		// CIM API init
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
		err = json.Unmarshal([]byte(result), &temp)
		if err != nil {
			CBLog.Error("cim api request failed : ", err)
			return ConnConfig{}, err
		}
		return temp, nil
	}
}

// ConnConfigList is struct for containing a CB-Spider struct for connection config list
type ConnConfigList struct { // Spider
	Connectionconfig []ConnConfig `json:"connectionconfig"`
}

// GetConnConfigList is func to list connection configs from CB-Spider
func GetConnConfigList() (ConnConfigList, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SpiderRestUrl + "/connectionconfig"

		client := resty.New().SetCloseConnection(true)

		resp, err := client.R().
			SetResult(&ConnConfigList{}).
			//SetError(&SimpleMsg{}).
			Get(url)

		if err != nil {
			CBLog.Error(err)
			content := ConnConfigList{}
			err := fmt.Errorf("Error from CB-Spider: " + err.Error())
			return content, err
		}

		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := ConnConfigList{}
			return content, err
		}

		temp, _ := resp.Result().(*ConnConfigList)

		// Get geolocations
		for i, connConfig := range temp.Connectionconfig {
			nativeRegion, err := GetNativeRegion(connConfig.ConfigName)
			if err != nil {
				CBLog.Error(err)
				content := ConnConfigList{}
				return content, err
			}

			location := GetCloudLocation(strings.ToLower(connConfig.ProviderName), strings.ToLower(nativeRegion))
			temp.Connectionconfig[i].Location = location
		}

		return *temp, nil

	} else {

		// CIM API init
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
		err = json.Unmarshal([]byte(result), &temp)
		if err != nil {
			CBLog.Error("cim api Unmarshal failed : ", err)
			return ConnConfigList{}, err
		}
		return temp, nil

	}
}

// Region is struct for containing region struct of CB-Spider
type Region struct {
	RegionName       string     // ex) "region01"
	ProviderName     string     // ex) "GCP"
	KeyValueInfoList []KeyValue // ex) { {region, us-east1}, {zone, us-east1-c} }
}

// GetRegion is func to get region from CB-Spider
func GetRegion(RegionName string) (Region, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SpiderRestUrl + "/region/" + RegionName

		client := resty.New().SetCloseConnection(true)

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

		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := Region{}
			return content, err
		}

		temp, _ := resp.Result().(*Region)
		return *temp, nil

	} else {

		// CIM API init
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
		err = json.Unmarshal([]byte(result), &temp)
		if err != nil {
			CBLog.Error("cim api Unmarshal failed : ", err)
			return Region{}, err
		}
		return temp, nil

	}
}

// GetRegion is func to get NativRegion from file
func GetNativeRegion(connectionName string) (string, error) {
	// Read default resources from file and create objects
	// HEADER: ProviderName, CONN_CONFIG, RegionName, NativeRegionName, RegionLocation, DriverLibFileName, DriverName
	file, fileErr := os.Open("../assets/cloudconnection.csv")
	defer file.Close()
	if fileErr != nil {
		CBLog.Error(fileErr)
		return "", fileErr
	}

	rdr := csv.NewReader(bufio.NewReader(file))
	rows, err := rdr.ReadAll()
	if err != nil {
		CBLog.Error(err)
		return "", err
	}

	nativeRegionName := ""

	for _, row := range rows[1:] {
		if connectionName != "" {
			// find only given connectionName (if not skip)
			if connectionName != row[1] {
				continue
			}
			fmt.Println("Found a line for the connectionName from file: " + row[1])
		}

		if connectionName != "" {
			// After finish handling line for the connectionName, break
			if connectionName == row[1] {
				nativeRegionName = row[3]
				fmt.Println("Handled for the connectionName from file: " + row[1])
				break
			}
		}

	}
	return nativeRegionName, nil

}

// RegionList is array struct for Region
type RegionList struct {
	Region []Region `json:"region"`
}

// GetRegionList is func to retrieve region list
func GetRegionList() (RegionList, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := SpiderRestUrl + "/region"

		client := resty.New().SetCloseConnection(true)

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

		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
			err := fmt.Errorf(string(resp.Body()))
			CBLog.Error(err)
			content := RegionList{}
			return content, err
		}

		temp, _ := resp.Result().(*RegionList)
		return *temp, nil

	} else {

		// CIM API init
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
		err = json.Unmarshal([]byte(result), &temp)
		if err != nil {
			CBLog.Error("cim api Unmarshal failed : ", err)
			return RegionList{}, err
		}
		return temp, nil

	}
}

// ConvertToMessage is func to change input data to gRPC message
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

// ConvertToOutput is func to convert gRPC message to print format
func ConvertToOutput(outType string, obj interface{}) (string, error) {
	//logger := logging.NewLogger()

	if outType == "yaml" {
		// marshal using JSON to remove fields with XXX prefix
		j, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}

		// use MapSlice to avoid sorting fields
		jsonObj := yaml.MapSlice{}
		err2 := yaml.Unmarshal(j, &jsonObj)
		if err2 != nil {
			return "", err2
		}

		// yaml marshal
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

// CopySrcToDest is func to copy data from source to target
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

// ConvGrpcStatusErr is func to convert error code into GRPC status code
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

// NewGrpcStatusErr is func to generate GRPC status error code
func NewGrpcStatusErr(msg string, tag string, method string) error {
	//logger := logging.NewLogger()

	//_, fn, line, _ := runtime.Caller(1)
	runtime.Caller(1)
	//logger.Error(tag, " error while calling ", method, " method: [", fn, ":", line, "] ", msg)
	return status.Errorf(codes.Internal, "%s error while calling %s method: %s ", tag, method, msg)
}

// NVL is func for null value logic
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

// GetChildIdList is func to get child id list from given key
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

// GetObjectList is func to return IDs of each child objects that has the same key
func GetObjectList(key string) []string {

	keyValue, _ := CBStore.GetList(key, true)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, v.Key)
	}

	fmt.Println("===============================================")
	return childIdList

}

// GetObjectValue is func to return the object value
func GetObjectValue(key string) (string, error) {

	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		return "", err
	}
	if keyValue == nil {
		return "", nil
	}
	return keyValue.Value, nil
}

// DeleteObject is func to delete the object
func DeleteObject(key string) error {

	err := CBStore.Delete(key)
	if err != nil {
		CBLog.Error(err)
		return err
	}
	return nil
}

// DeleteObjects is func to delete objects
func DeleteObjects(key string) error {
	keyValue, _ := CBStore.GetList(key, true)
	for _, v := range keyValue {
		err := CBStore.Delete(v.Key)
		if err != nil {
			CBLog.Error(err)
			return err
		}
	}
	return nil
}

func CheckElement(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

const (
	// Random string generation
	letterBytes   = "abcdefghijklmnopqrstuvwxyz1234567890"
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

/* generate a random string (from CB-MCKS source code) */
func GenerateNewRandomString(n int) string {
	randSrc := rand.NewSource(time.Now().UnixNano()) //Random source by nano time
	b := make([]byte, n)
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}
