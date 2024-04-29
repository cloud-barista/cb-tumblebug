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
	"math/rand"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	uid "github.com/rs/xid"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v2"

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
func GenRandomPassword(length int) string {
	rand.Seed(time.Now().Unix())

	charset := "A1!$"
	shuff := []rune(charset)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})
	randomString := GenUid()
	if len(randomString) < length {
		randomString = randomString + GenUid()
	}
	reducedString := randomString[0 : length-len(charset)]
	reducedString = reducedString + string(shuff)

	shuff = []rune(reducedString)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})

	pw := string(shuff)

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

// GenConnectionKey is func to generate a key for connection info
func GenConnectionKey(connectionId string) string {
	return "/connection/" + connectionId
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
		log.Error().Err(err).Msg("")
		return "", err
	}
	if keyValue == nil {
		//log.Error().Err(err).Msg("")
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
				log.Error().Err(err).Msg("")
				// if there is no matched value for the key, return empty string. Error will be handled in a parent function
				return ""
			}
			return content.CspVNicName
	*/
	default:
		return "", fmt.Errorf("invalid resourceType")
	}
}

// ConnConfig is struct for containing modified CB-Spider struct for connection config
type ConnConfig struct {
	ConfigName           string       `json:"configName"`
	ProviderName         string       `json:"providerName"`
	DriverName           string       `json:"driverName"`
	CredentialName       string       `json:"credentialName"`
	CredentialHolder     string       `json:"credentialHolder"`
	RegionName           string       `json:"regionName"`
	RegionDetail         RegionDetail `json:"regionDetail"`
	RegionRepresentative bool         `json:"regionRepresentative"`
	Location             GeoLocation  `json:"location"`
	Enabled              bool         `json:"enabled"`
}

// CloudDriverInfo is struct for containing a CB-Spider struct for cloud driver info
type CloudDriverInfo struct {
	DriverName        string
	ProviderName      string
	DriverLibFileName string
}

// CredentialReq is struct for containing a struct for credential request
type CredentialReq struct {
	CredentialHolder string     `json:"credentialHolder"`
	ProviderName     string     `json:"providerName"`
	KeyValueInfoList []KeyValue `json:"keyValueInfoList"`
}

// CredentialInfo is struct for containing a struct for credential info
type CredentialInfo struct {
	CredentialName   string     `json:"credentialName"`
	CredentialHolder string     `json:"credentialHolder"`
	ProviderName     string     `json:"providerName"`
	KeyValueInfoList []KeyValue `json:"keyValueInfoList"`
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
func GetCloudLocation(cloudType string, nativeRegion string) (GeoLocation, error) {
	cloudType = strings.ToLower(cloudType)
	nativeRegion = strings.ToLower(nativeRegion)

	cspDetail, ok := RuntimeCloudInfo.CSPs[cloudType]
	if !ok {
		return GeoLocation{}, fmt.Errorf("cloudType '%s' not found", cloudType)
	}

	regionDetail, ok := cspDetail.Regions[nativeRegion]
	if !ok {
		return GeoLocation{}, fmt.Errorf("nativeRegion '%s' not found in cloudType '%s'", nativeRegion, cloudType)
	}

	return GeoLocation{
		Latitude:     fmt.Sprintf("%f", regionDetail.Location.Latitude),
		Longitude:    fmt.Sprintf("%f", regionDetail.Location.Longitude),
		BriefAddr:    regionDetail.Location.Display,
		CloudType:    cloudType,
		NativeRegion: nativeRegion,
	}, nil
}

// GetConnConfig is func to get connection config
func GetConnConfig(ConnConfigName string) (ConnConfig, error) {

	connConfig := ConnConfig{}

	key := GenConnectionKey(ConnConfigName)
	keyValue, err := CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}
	if keyValue == nil {
		return ConnConfig{}, fmt.Errorf("Cannot find the ConnConfig " + key)
	}
	err = json.Unmarshal([]byte(keyValue.Value), &connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}

	return connConfig, nil
}

// ConnConfigList is struct for containing a CB-Spider struct for connection config list
type ConnConfigList struct { // Spider
	Connectionconfig []ConnConfig `json:"connectionconfig"`
}

// CheckConnConfigAvailable is func to check if connection config is available by checking allkeypair list
func CheckConnConfigAvailable(connConfigName string) (bool, error) {

	var callResult interface{}
	client := resty.New()
	url := SpiderRestUrl + "/allkeypair"
	method := "GET"
	requestBody := SpiderConnectionName{}
	requestBody.ConnectionName = connConfigName

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		ShortDuration,
	)

	if err != nil {
		log.Info().Err(err).Msg("")
		return false, err
	}

	return true, nil
}

// GetConnConfigList is func to list filtered connection configs
func GetConnConfigList(filterCredentialHolder string, filterVerified bool, filterRegionRepresentative bool) (ConnConfigList, error) {
	var filteredConnections ConnConfigList
	var tmpConnections ConnConfigList

	key := "/connection"
	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfigList{}, err
	}
	if keyValue != nil {
		for _, v := range keyValue {
			tempObj := ConnConfig{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				log.Error().Err(err).Msg("")
				return filteredConnections, err
			}
			filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, tempObj)
		}
	} else {
		return ConnConfigList{}, nil
	}

	log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))

	// filter by credential holder
	if filterCredentialHolder != "" {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if strings.EqualFold(connConfig.CredentialHolder, filterCredentialHolder) {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = ConnConfigList{}
		log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
	}

	// filter only verified
	if filterVerified {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if connConfig.Enabled {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = ConnConfigList{}
		log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
	}

	// filter only region representative
	if filterRegionRepresentative {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if connConfig.RegionRepresentative {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = ConnConfigList{}
		log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
	}

	return filteredConnections, nil
}

// Region is struct for containing region struct of CB-Spider
type Region struct {
	RegionName        string     // ex) "region01"
	ProviderName      string     // ex) "GCP"
	KeyValueInfoList  []KeyValue // ex) { {region, us-east1}, {zone, us-east1-c} }
	AvailableZoneList []string
}

// RegisterAllCloudInfo is func to register all cloud info from asset to CB-Spider
func RegisterAllCloudInfo() error {
	for providerName, _ := range RuntimeCloudInfo.CSPs {
		err := RegisterCloudInfo(providerName)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
	return nil
}

// RegisterCloudInfo is func to register cloud info from asset to CB-Spider
func RegisterCloudInfo(providerName string) error {

	driverName := RuntimeCloudInfo.CSPs[providerName].Driver

	client := resty.New()
	url := SpiderRestUrl + "/driver"
	method := "POST"
	var callResult CloudDriverInfo
	requestBody := CloudDriverInfo{ProviderName: strings.ToUpper(providerName), DriverName: driverName, DriverLibFileName: driverName}

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	for regionName, _ := range RuntimeCloudInfo.CSPs[providerName].Regions {
		err := RegisterRegionZone(providerName, regionName)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	return nil
}

// RegisterRegionZone is func to register all regions to CB-Spider
func RegisterRegionZone(providerName string, regionName string) error {
	client := resty.New()
	url := SpiderRestUrl + "/region"
	method := "POST"
	var callResult Region
	requestBody := Region{ProviderName: strings.ToUpper(providerName), RegionName: regionName}

	if RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones == nil {
		requestBody.RegionName = providerName + "-" + regionName
		keyValueInfoList := []KeyValue{
			{Key: "Region", Value: regionName},
			{Key: "Zone", Value: "N/A"},
		}
		requestBody.KeyValueInfoList = keyValueInfoList

		err := ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			SetUseBody(requestBody),
			&requestBody,
			&callResult,
			MediumDuration,
		)

		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	for _, zoneName := range RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones {
		requestBody.RegionName = providerName + "-" + regionName + "-" + zoneName
		keyValueInfoList := []KeyValue{
			{Key: "Region", Value: regionName},
			{Key: "Zone", Value: zoneName},
		}
		requestBody.AvailableZoneList = RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones
		requestBody.KeyValueInfoList = keyValueInfoList

		err := ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			SetUseBody(requestBody),
			&requestBody,
			&callResult,
			MediumDuration,
		)

		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

	}

	return nil
}

// RegisterCredential is func to register credential and all releated connection configs
func RegisterCredential(req CredentialReq) (CredentialInfo, error) {

	req.CredentialHolder = strings.ToLower(req.CredentialHolder)
	req.ProviderName = strings.ToLower(req.ProviderName)
	genneratedCredentialName := req.CredentialHolder + "-" + req.ProviderName
	if req.CredentialHolder == DefaultCredentialHolder {
		// credential with default credental holder (e.g., admin) has no prefix
		genneratedCredentialName = req.ProviderName
	}

	// replace `\\n` with `\n` in the value to restore the original PEM value
	for i, keyValue := range req.KeyValueInfoList {
		req.KeyValueInfoList[i].Value = strings.ReplaceAll(keyValue.Value, "\\n", "\n")
	}

	reqToSpider := CredentialInfo{
		CredentialName:   genneratedCredentialName,
		ProviderName:     strings.ToUpper(req.ProviderName),
		KeyValueInfoList: req.KeyValueInfoList,
	}

	client := resty.New()
	url := SpiderRestUrl + "/credential"
	method := "POST"
	var callResult CredentialInfo
	requestBody := reqToSpider

	PrintJsonPretty(requestBody)

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return CredentialInfo{}, err
	}
	PrintJsonPretty(callResult)

	callResult.CredentialHolder = req.CredentialHolder
	callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	for callResultKey, _ := range callResult.KeyValueInfoList {
		callResult.KeyValueInfoList[callResultKey].Value = "************"
	}

	cloudInfo, err := GetCloudInfo()
	if err != nil {
		return callResult, err
	}
	cspDetail, ok := cloudInfo.CSPs[callResult.ProviderName]
	if !ok {
		return callResult, fmt.Errorf("cloudType '%s' not found", callResult.ProviderName)
	}

	// register connection config for all regions with the credential
	allRegisteredRegions, err := GetRegionList()
	if err != nil {
		return callResult, err
	}
	for _, region := range allRegisteredRegions.Region {
		if strings.ToLower(region.ProviderName) == callResult.ProviderName {
			configName := callResult.CredentialHolder + "-" + region.RegionName
			if callResult.CredentialHolder == DefaultCredentialHolder {
				configName = region.RegionName
			}
			connConfig := ConnConfig{
				ConfigName:       configName,
				ProviderName:     strings.ToUpper(callResult.ProviderName),
				DriverName:       cspDetail.Driver,
				CredentialName:   callResult.CredentialName,
				RegionName:       region.RegionName,
				CredentialHolder: req.CredentialHolder,
			}
			_, err := RegisterConnectionConfig(connConfig)
			if err != nil {
				log.Error().Err(err).Msg("")
				return callResult, err
			}
		}
	}

	validate := true
	// filter only verified
	if validate {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		filteredConnections := ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if strings.EqualFold(callResult.ProviderName, connConfig.ProviderName) {
				connConfig.ProviderName = strings.ToLower(connConfig.ProviderName)
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}

		var wg sync.WaitGroup
		results := make(chan ConnConfig, len(filteredConnections.Connectionconfig))

		for _, connConfig := range filteredConnections.Connectionconfig {
			wg.Add(1)
			go func(connConfig ConnConfig) {
				defer wg.Done()
				RandomSleep(0, 30)
				enabled, err := CheckConnConfigAvailable(connConfig.ConfigName)
				if err != nil {
					log.Error().Err(err).Msgf("Cannot check ConnConfig %s is available", connConfig.ConfigName)
				}
				connConfig.Enabled = enabled
				if enabled {
					regionInfo, err := GetRegion(connConfig.ProviderName, connConfig.Location.NativeRegion)
					if err != nil {
						log.Error().Err(err).Msgf("Cannot get region for %s", connConfig.RegionName)
						connConfig.Enabled = false
					} else {
						location, err := GetCloudLocation(connConfig.ProviderName, connConfig.Location.NativeRegion)
						if err != nil {
							log.Error().Err(err).Msgf("Cannot get location for %s/%s", connConfig.ProviderName, connConfig.Location.NativeRegion)
						}
						connConfig.Location = location
						connConfig.RegionDetail = regionInfo
					}
				}
				results <- connConfig
			}(connConfig)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for result := range results {
			if result.Enabled {
				key := GenConnectionKey(result.ConfigName)
				val, err := json.Marshal(result)
				if err != nil {
					return CredentialInfo{}, err
				}
				err = CBStore.Put(string(key), string(val))
			}
		}
	}

	setRegionRepresentative := true
	if setRegionRepresentative {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		filteredConnections := ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if strings.EqualFold(req.ProviderName, connConfig.ProviderName) {
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}
		log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
		regionRepresentative := make(map[string]ConnConfig)
		for _, connConfig := range allConnections.Connectionconfig {
			prefix := req.ProviderName + "-" + connConfig.Location.NativeRegion
			prefix = strings.ToLower(prefix)
			if strings.HasPrefix(connConfig.RegionName, prefix) {
				if _, exists := regionRepresentative[prefix]; !exists {
					regionRepresentative[prefix] = connConfig
				}
			}
		}
		for _, connConfig := range regionRepresentative {
			connConfig.RegionRepresentative = true
			key := GenConnectionKey(connConfig.ConfigName)
			val, err := json.Marshal(connConfig)
			if err != nil {
				return callResult, err
			}
			err = CBStore.Put(string(key), string(val))
			if err != nil {
				return callResult, err
			}
		}
	}

	return callResult, nil
}

// RegisterConnectionConfig is func to register connection config to CB-Spider
func RegisterConnectionConfig(connConfig ConnConfig) (ConnConfig, error) {
	client := resty.New()
	url := SpiderRestUrl + "/connectionconfig"
	method := "POST"
	var callResult ConnConfig
	requestBody := connConfig

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}

	// Register connection to cb-tumblebug with availability check
	// enabled, err := CheckConnConfigAvailable(callResult.ConfigName)
	// if err != nil {
	// 	log.Error().Err(err).Msgf("Cannot check ConnConfig %s is available", connConfig.ConfigName)
	// }
	// callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	// if enabled {
	// 	nativeRegion, _, err := GetRegion(callResult.RegionName)
	// 	if err != nil {
	// 		log.Error().Err(err).Msgf("Cannot get region for %s", callResult.RegionName)
	// 		callResult.Enabled = false
	// 	} else {
	// 		location, err := GetCloudLocation(callResult.ProviderName, nativeRegion)
	// 		if err != nil {
	// 			log.Error().Err(err).Msgf("Cannot get location for %s/%s", callResult.ProviderName, nativeRegion)
	// 		}
	// 		callResult.Location = location
	// 	}
	// }

	callResult.CredentialHolder = connConfig.CredentialHolder
	callResult.ProviderName = strings.ToLower(callResult.ProviderName)

	key := GenConnectionKey(callResult.ConfigName)
	val, err := json.Marshal(callResult)
	if err != nil {
		return ConnConfig{}, err
	}
	err = CBStore.Put(string(key), string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}

	return callResult, nil
}

// GetRegion is func to get regionInfo with the native region name
func GetRegion(ProviderName, RegionName string) (RegionDetail, error) {

	ProviderName = strings.ToLower(ProviderName)
	RegionName = strings.ToLower(RegionName)

	cloudInfo, err := GetCloudInfo()
	if err != nil {
		return RegionDetail{}, err
	}

	cspDetail, ok := cloudInfo.CSPs[ProviderName]
	if !ok {
		return RegionDetail{}, fmt.Errorf("cloudType '%s' not found", ProviderName)
	}

	// using map directly is not working because of the prefix
	// need to be used after we deprecate zone description in test scripts
	// regionDetail, ok := cspDetail.Regions[nativeRegion]
	// if !ok {
	// 	return nativeRegion, RegionDetail{}, fmt.Errorf("nativeRegion '%s' not found in cloudType '%s'", nativeRegion, cloudType)
	// }

	// return nativeRegion, regionDetail, nil

	for key, regionDetail := range cspDetail.Regions {
		if strings.HasPrefix(RegionName, key) {
			return regionDetail, nil
		}
	}

	return RegionDetail{}, fmt.Errorf("nativeRegion '%s' not found in cloudType '%s'", RegionName, ProviderName)
}

// RegionList is array struct for Region
type RegionList struct {
	Region []Region `json:"region"`
}

// GetRegionList is func to retrieve region list
func GetRegionList() (RegionList, error) {

	url := SpiderRestUrl + "/region"

	client := resty.New().SetCloseConnection(true)

	resp, err := client.R().
		SetResult(&RegionList{}).
		//SetError(&SimpleMsg{}).
		Get(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := RegionList{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := RegionList{}
		return content, err
	}

	temp, _ := resp.Result().(*RegionList)
	return *temp, nil

}

// GetCloudInfo is func to get all cloud info from the asset
func GetCloudInfo() (CloudInfo, error) {
	return RuntimeCloudInfo, nil
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

	return childIdList

}

// GetObjectList is func to return IDs of each child objects that has the same key
func GetObjectList(key string) []string {

	keyValue, _ := CBStore.GetList(key, true)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, v.Key)
	}

	return childIdList

}

// GetObjectValue is func to return the object value
func GetObjectValue(key string) (string, error) {

	keyValue, err := CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
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
		log.Error().Err(err).Msg("")
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
			log.Error().Err(err).Msg("")
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
