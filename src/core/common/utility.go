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

	r, _ := regexp.Compile("(?i)[a-z]([-a-z0-9+]*[a-z0-9])?")
	filtered := r.FindString(name)

	if filtered != name {
		err := fmt.Errorf(name + ": The name must follow these rules: " +
			"1. The first character must be a letter (case-insensitive). " +
			"2. All following characters can be a dash, letter (case-insensitive), digit, or +. " +
			"3. The last character cannot be a dash.")
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

// GenCredentialHolderKey is func to generate a key for credentialHolder info
func GenCredentialHolderKey(holderId string) string {
	return "/credentialHolder/" + holderId
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
	ConfigName           string         `json:"configName"`
	ProviderName         string         `json:"providerName"`
	DriverName           string         `json:"driverName"`
	CredentialName       string         `json:"credentialName"`
	CredentialHolder     string         `json:"credentialHolder"`
	RegionZoneInfoName   string         `json:"regionZoneInfoName"`
	RegionZoneInfo       RegionZoneInfo `json:"regionZoneInfo"`
	RegionDetail         RegionDetail   `json:"regionDetail"`
	RegionRepresentative bool           `json:"regionRepresentative"`
	Verified             bool           `json:"verified"`
}

// SpiderConnConfig is struct for containing a CB-Spider struct for connection config
type SpiderConnConfig struct {
	ConfigName     string
	ProviderName   string
	DriverName     string
	CredentialName string
	RegionName     string
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
		//log.Info().Err(err).Msg("")
		return false, err
	}

	return true, nil
}

// CheckSpiderStatus is func to check if CB-Spider is ready
func CheckSpiderReady() error {

	var callResult interface{}
	client := resty.New()
	url := SpiderRestUrl + "/readyz"
	method := "GET"
	requestBody := NoBody

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		VeryShortDuration,
	)

	if err != nil {
		//log.Err(err).Msg("")
		return err
	}

	return nil
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

	// filter by credential holder
	if filterCredentialHolder != "" {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if strings.EqualFold(connConfig.CredentialHolder, filterCredentialHolder) {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = ConnConfigList{}
	}

	// filter only verified
	if filterVerified {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if connConfig.Verified {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = ConnConfigList{}
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
	}
	//log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
	return filteredConnections, nil
}

// SpiderRegionZoneInfo is struct for containing region struct of CB-Spider
type SpiderRegionZoneInfo struct {
	RegionName        string     // ex) "region01"
	ProviderName      string     // ex) "GCP"
	KeyValueInfoList  []KeyValue // ex) { {region, us-east1}, {zone, us-east1-c} }
	AvailableZoneList []string
}

// RegionZoneInfo is struct for containing region struct
type RegionZoneInfo struct {
	AssignedRegion string `json:"assignedRegion"`
	AssignedZone   string `json:"assignedZone"`
}

// RegisterAllCloudInfo is func to register all cloud info from asset to CB-Spider
func RegisterAllCloudInfo() error {
	for providerName := range RuntimeCloudInfo.CSPs {
		err := RegisterCloudInfo(providerName)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
	return nil
}

// GetProviderList is func to list all cloud providers
func GetProviderList() (*IdList, error) {
	providers := IdList{}
	for providerName := range RuntimeCloudInfo.CSPs {
		providers.IdList = append(providers.IdList, providerName)
	}
	return &providers, nil
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
	var callResult SpiderRegionZoneInfo
	requestBody := SpiderRegionZoneInfo{ProviderName: strings.ToUpper(providerName), RegionName: regionName}

	// register representative regionZone (region only)
	requestBody.RegionName = providerName + "-" + regionName
	keyValueInfoList := []KeyValue{}

	if len(RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones) > 0 {
		keyValueInfoList = []KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
			{Key: "Zone", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones[0]},
		}
	} else {
		keyValueInfoList = []KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
			{Key: "Zone", Value: "N/A"},
		}
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

	// register all regionZones
	for _, zoneName := range RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones {
		requestBody.RegionName = providerName + "-" + regionName + "-" + zoneName
		keyValueInfoList := []KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
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

	//PrintJsonPretty(requestBody)

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
	//PrintJsonPretty(callResult)

	callResult.CredentialHolder = req.CredentialHolder
	callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	for callResultKey := range callResult.KeyValueInfoList {
		callResult.KeyValueInfoList[callResultKey].Value = "************"
	}

	// TODO: add code to register CredentialHolder object

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
				ConfigName:         configName,
				ProviderName:       strings.ToUpper(callResult.ProviderName),
				DriverName:         cspDetail.Driver,
				CredentialName:     callResult.CredentialName,
				RegionZoneInfoName: region.RegionName,
				CredentialHolder:   req.CredentialHolder,
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
				verified, err := CheckConnConfigAvailable(connConfig.ConfigName)
				if err != nil {
					log.Error().Err(err).Msgf("Cannot check ConnConfig %s is available", connConfig.ConfigName)
				}
				connConfig.Verified = verified
				if verified {
					regionInfo, err := GetRegion(connConfig.ProviderName, connConfig.RegionDetail.RegionName)
					if err != nil {
						log.Error().Err(err).Msgf("Cannot get region for %s", connConfig.RegionDetail.RegionName)
						connConfig.Verified = false
					} else {
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
			if result.Verified {
				key := GenConnectionKey(result.ConfigName)
				val, err := json.Marshal(result)
				if err != nil {
					return CredentialInfo{}, err
				}
				err = CBStore.Put(string(key), string(val))
				if err != nil {
					return callResult, err
				}
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
			prefix := req.ProviderName + "-" + connConfig.RegionDetail.RegionName
			if strings.EqualFold(connConfig.RegionZoneInfoName, prefix) {
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

	verifyRegionRepresentativeAndUpdateZone := true
	if verifyRegionRepresentativeAndUpdateZone {
		verifiedConnections, err := GetConnConfigList(req.CredentialHolder, true, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}
		allRepresentativeRegionConnections, err := GetConnConfigList(req.CredentialHolder, false, true)
		for _, connConfig := range allRepresentativeRegionConnections.Connectionconfig {
			if strings.EqualFold(req.ProviderName, connConfig.ProviderName) {
				verified := false
				for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
					if strings.EqualFold(connConfig.ConfigName, verifiedConnConfig.ConfigName) {
						verified = true
					}
				}
				// update representative regionZone with the verified regionZone
				if !verified {
					for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
						if strings.HasPrefix(verifiedConnConfig.ConfigName, connConfig.ConfigName) {
							connConfig.RegionZoneInfoName = verifiedConnConfig.RegionZoneInfoName
							connConfig.RegionZoneInfo = verifiedConnConfig.RegionZoneInfo
							break
						}
					}
					// update DB
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
		}
	}

	return callResult, nil
}

// RegisterConnectionConfig is func to register connection config to CB-Spider
func RegisterConnectionConfig(connConfig ConnConfig) (ConnConfig, error) {
	client := resty.New()
	url := SpiderRestUrl + "/connectionconfig"
	method := "POST"
	var callResult SpiderConnConfig
	requestBody := SpiderConnConfig{}
	requestBody.ConfigName = connConfig.ConfigName
	requestBody.ProviderName = connConfig.ProviderName
	requestBody.DriverName = connConfig.DriverName
	requestBody.CredentialName = connConfig.CredentialName
	requestBody.RegionName = connConfig.RegionZoneInfoName

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
	// verified, err := CheckConnConfigAvailable(callResult.ConfigName)
	// if err != nil {
	// 	log.Error().Err(err).Msgf("Cannot check ConnConfig %s is available", connConfig.ConfigName)
	// }
	// callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	// if verified {
	// 	nativeRegion, _, err := GetRegion(callResult.RegionName)
	// 	if err != nil {
	// 		log.Error().Err(err).Msgf("Cannot get region for %s", callResult.RegionName)
	// 		callResult.Verified = false
	// 	} else {
	// 		location, err := GetCloudLocation(callResult.ProviderName, nativeRegion)
	// 		if err != nil {
	// 			log.Error().Err(err).Msgf("Cannot get location for %s/%s", callResult.ProviderName, nativeRegion)
	// 		}
	// 		callResult.Location = location
	// 	}
	// }

	connection := ConnConfig{}
	connection.ConfigName = callResult.ConfigName
	connection.ProviderName = strings.ToLower(callResult.ProviderName)
	connection.DriverName = callResult.DriverName
	connection.CredentialName = callResult.CredentialName
	connection.RegionZoneInfoName = callResult.RegionName
	connection.CredentialHolder = connConfig.CredentialHolder

	// load region info
	url = SpiderRestUrl + "/region/" + connection.RegionZoneInfoName
	method = "GET"
	var callResultRegion SpiderRegionZoneInfo
	requestNoBody := NoBody

	err = ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestNoBody),
		&requestNoBody,
		&callResultRegion,
		MediumDuration,
	)
	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}
	regionZoneInfo := RegionZoneInfo{}
	for _, keyVal := range callResultRegion.KeyValueInfoList {
		if keyVal.Key == "Region" {
			regionZoneInfo.AssignedRegion = keyVal.Value
		}
		if keyVal.Key == "Zone" {
			regionZoneInfo.AssignedZone = keyVal.Value
		}
	}
	connection.RegionZoneInfo = regionZoneInfo

	regionDetail, err := GetRegion(connection.ProviderName, connection.RegionZoneInfo.AssignedRegion)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot get region for %s", connection.RegionZoneInfo.AssignedRegion)
		return ConnConfig{}, err
	}
	connection.RegionDetail = regionDetail

	key := GenConnectionKey(connection.ConfigName)
	val, err := json.Marshal(connection)
	if err != nil {
		return ConnConfig{}, err
	}
	err = CBStore.Put(string(key), string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return ConnConfig{}, err
	}

	return connection, nil
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

	return RegionDetail{}, fmt.Errorf("nativeRegion '%s' not found in Provider '%s'", RegionName, ProviderName)
}

// RegionList is array struct for Region
type RegionList struct {
	Region []SpiderRegionZoneInfo `json:"region"`
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

// GetK8sClusterInfo is func to get all kubernetes cluster info from the asset
func GetK8sClusterInfo() (K8sClusterInfo, error) {
	return RuntimeK8sClusterInfo, nil
}

func getK8sClusterDetail(providerName string) *K8sClusterDetail {
	// Get K8sClusterDetail for providerName
	var k8sClusterDetail *K8sClusterDetail = nil
	for provider, detail := range RuntimeK8sClusterInfo.CSPs {
		provider = strings.ToLower(provider)
		if provider == providerName {
			k8sClusterDetail = &detail
			break
		}
	}

	return k8sClusterDetail
}

// GetAvailableK8sClusterVersion is func to get available kubernetes cluster versions for provider and region from K8sClusterInfo
func GetAvailableK8sClusterVersion(providerName string, regionName string) (*[]K8sClusterVersionDetailAvailable, error) {
	//
	// Check available K8sCluster version and node image in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Get Available Versions for regionName
	var availableVersion *[]K8sClusterVersionDetailAvailable = nil
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				availableVersion = &versionDetail.Available
				return availableVersion, nil
			}
		}
	}

	return nil, fmt.Errorf("no available kubernetes cluster version for region(%s) of provider(%s)", regionName, providerName)
}

// GetAvailableK8sClusterNodeImage is func to get available kubernetes cluster node images for provider and region from K8sClusterInfo
func GetAvailableK8sClusterNodeImage(providerName string, regionName string) (*[]K8sClusterNodeImageDetailAvailable, error) {
	//
	// Check available K8sCluster node image in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Get Available Node Image for regionName
	var availableNodeImage *[]K8sClusterNodeImageDetailAvailable = nil
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				availableNodeImage = &nodeImageDetail.Available
				return availableNodeImage, nil
			}
		}
	}

	return nil, fmt.Errorf("no available kubernetes cluster node image for region(%s) of provider(%s)", regionName, providerName)
}

/*
func isValidSpecForK8sCluster(spec *mcir.TbSpecInfo) bool {
	//
	// Check for Provider
	//

	providerName := strings.ToLower(spec.ProviderName)

	var k8sClusterDetail *common.K8sClusterDetail = nil
	for provider, detail := range common.RuntimeK8sClusterInfo.CSPs {
		provider = strings.ToLower(provider)
		if provider == providerName {
			k8sClusterDetail = &detail
			break
		}
	}
	if k8sClusterDetail == nil {
		return false
	}

	//
	// Check for Region
	//

	regionName := strings.ToLower(spec.RegionName)

	// Check for Version
	isExist := false
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(versionDetail.Available) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	// Check for NodeImage
	isExist = false
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(nodeImageDetail.Available) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	// Check for RootDisk
	isExist = false
	for _, rootDiskDetail := range k8sClusterDetail.RootDisk {
		for _, region := range rootDiskDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(rootDiskDetail.Type) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	return true
}
*/
