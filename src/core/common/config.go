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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"

	"github.com/rs/zerolog/log"
)

// CloudInfo is structure for cloud information
type CloudInfo struct {
	CSPs map[string]CSPDetail `mapstructure:"cloud" json:"csps"`
}

// CSPDetail is structure for CSP information
type CSPDetail struct {
	Description string                  `mapstructure:"description" json:"description"`
	Driver      string                  `mapstructure:"driver" json:"driver"`
	Links       []string                `mapstructure:"link" json:"links"`
	Regions     map[string]RegionDetail `mapstructure:"region" json:"regions"`
}

// RegionDetail is structure for region information
type RegionDetail struct {
	Description string   `mapstructure:"description" json:"description"`
	Location    Location `mapstructure:"location" json:"location"`
	Zones       []string `mapstructure:"zone" json:"zones"`
}

// Location is structure for location information
type Location struct {
	Display   string  `mapstructure:"display" json:"display"`
	Latitude  float64 `mapstructure:"latitude" json:"latitude"`
	Longitude float64 `mapstructure:"longitude" json:"longitude"`
}

// RuntimeCloudInfo is global variable for CloudInfo
var RuntimeCloudInfo = CloudInfo{}

type Credential struct {
	Credentialholder map[string]map[string]map[string]string `yaml:"credentialholder"`
}

var RuntimeCredential = Credential{}

// AdjustKeysToLowercase adjusts the keys of nested maps to lowercase.
func AdjustKeysToLowercase(cloudInfo *CloudInfo) {
	newCSPs := make(map[string]CSPDetail)
	for cspKey, cspDetail := range cloudInfo.CSPs {
		lowerCSPKey := strings.ToLower(cspKey)
		newRegions := make(map[string]RegionDetail)
		for regionKey, regionDetail := range cspDetail.Regions {
			lowerRegionKey := strings.ToLower(regionKey)
			newRegions[lowerRegionKey] = regionDetail
		}
		cspDetail.Regions = newRegions
		newCSPs[lowerCSPKey] = cspDetail
	}
	cloudInfo.CSPs = newCSPs
}

// PrintCloudInfoTable prints CloudInfo in table format
func PrintCloudInfoTable(cloudInfo CloudInfo) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"CSP", "Region", "Location", "(Lati:Long)", "Zones"})

	for providerName, cspDetail := range cloudInfo.CSPs {
		for regionName, regionDetail := range cspDetail.Regions {
			latLong := formatLatLong(regionDetail.Location.Latitude, regionDetail.Location.Longitude)
			zones := formatZones(regionDetail.Zones)
			t.AppendRow(table.Row{providerName, regionName, regionDetail.Location.Display, latLong, zones})
		}
	}
	t.SortBy([]table.SortBy{
		{Name: "CSP", Mode: table.Asc},
		{Name: "Region", Mode: table.Asc},
	})
	t.Render()
}

// PrintCredentialInfo prints Credential information in table format
func PrintCredentialInfo(credential Credential) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Credentialholder", "Cloud Service Provider", "Credential Key", "Credential Value"})

	for credentialholder, providers := range credential.Credentialholder {
		for provider, credentials := range providers {
			for key, _ := range credentials {
				t.AppendRow(table.Row{credentialholder, provider, key, "********"})
			}
		}
	}
	t.SortBy([]table.SortBy{
		{Name: "Credentialholder", Mode: table.Asc},
		{Name: "Cloud Service Provider", Mode: table.Asc},
		{Name: "Credential Key", Mode: table.Asc},
	})
	t.Render()
}

func formatLatLong(latitude, longitude float64) string {
	if latitude == 0.0 && longitude == 0.0 {
		return ""
	}
	return "(" + fmt.Sprintf("%.2f", latitude) + ":" + fmt.Sprintf("%.2f", longitude) + ")"
}

func formatZones(zones []string) string {
	if len(zones) == 0 {
		return ""
	}
	var formattedZones string
	for i, zone := range zones {
		formattedZones += zone
		if i < len(zones)-1 {
			formattedZones += " "
		}
	}
	return formattedZones
}

// RuntimeLatancyMap is global variable for LatancyMap
var RuntimeLatancyMap = [][]string{}

// RuntimeLatancyMapIndex is global variable for LatancyMap (index)
var RuntimeLatancyMapIndex = make(map[string]int)

// RuntimeConf is global variable for cloud config
var RuntimeConf = RuntimeConfig{}

// RuntimeConfig is structure for global variable for cloud config
type RuntimeConfig struct {
	Cloud Cloud `yaml:"cloud"`
	Nlbsw Nlbsw `yaml:"nlbsw"`
}

// Cloud is structure for cloud settings per CSP
type Cloud struct {
	Common    CloudSetting `yaml:"common"`
	Aws       CloudSetting `yaml:"aws"`
	Azure     CloudSetting `yaml:"azure"`
	Gcp       CloudSetting `yaml:"gcp"`
	Alibaba   CloudSetting `yaml:"alibaba"`
	Tencent   CloudSetting `yaml:"tencent"`
	Ibm       CloudSetting `yaml:"ibm"`
	Openstack CloudSetting `yaml:"openstack"`
	Cloudit   CloudSetting `yaml:"cloudit"`
}

// CloudSetting is structure for cloud settings per CSP in details
type CloudSetting struct {
	Enable  string         `yaml:"enable"`
	Nlb     NlbSetting     `yaml:"nlb"`
	Cluster ClusterSetting `yaml:"cluster"`
}

// NlbSetting is structure for NLB setting
type NlbSetting struct {
	Enable    string `yaml:"enable"`
	Interval  string `yaml:"interval"`
	Timeout   string `yaml:"timeout"`
	Threshold string `yaml:"threshold"`
}

// Nlbsw is structure for NLB setting
type Nlbsw struct {
	Sw                      string `yaml:"sw"`
	Version                 string `yaml:"version"`
	CommandNlbPrepare       string `yaml:"commandNlbPrepare"`
	CommandNlbDeploy        string `yaml:"commandNlbDeploy"`
	CommandNlbAddTargetNode string `yaml:"commandNlbAddTargetNode"`
	CommandNlbApplyConfig   string `yaml:"commandNlbApplyConfig"`
	NlbMcisCommonSpec       string `yaml:"nlbMcisCommonSpec"`
	NlbMcisCommonImage      string `yaml:"nlbMcisCommonImage"`
	NlbMcisSubGroupSize     string `yaml:"nlbMcisSubGroupSize"`
}

// ClusterSetting is structure for Cluster setting
type ClusterSetting struct {
	Enable string `yaml:"enable"`
}

// type DataDiskCmd string
const (
	AttachDataDisk    string = "attach"
	DetachDataDisk    string = "detach"
	AvailableDataDisk string = "available"
)

// swagger:request ConfigReq
type ConfigReq struct {
	Name  string `json:"name" example:"SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}

// swagger:response ConfigInfo
type ConfigInfo struct {
	Id    string `json:"id" example:"SPIDER_REST_URL"`
	Name  string `json:"name" example:"SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}

func UpdateConfig(u *ConfigReq) (ConfigInfo, error) {

	if u.Name == "" {
		return ConfigInfo{}, fmt.Errorf("The provided name is empty.")
	}

	content := ConfigInfo{}
	content.Id = u.Name
	content.Name = u.Name
	content.Value = u.Value

	key := "/config/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	val, _ := json.Marshal(content)
	err := CBStore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}
	keyValue, _ := CBStore.Get(key)
	fmt.Println("UpdateConfig(); ===========================")
	fmt.Println("UpdateConfig(); Key: " + keyValue.Key + "\nValue: " + keyValue.Value)
	fmt.Println("UpdateConfig(); ===========================")

	UpdateGlobalVariable(content.Id)

	return content, nil
}

func UpdateGlobalVariable(id string) error {

	configInfo, err := GetConfig(id)
	if err != nil {
		log.Info().Msg(err.Error())
		return err
	}

	switch id {
	case StrSpiderRestUrl:
		SpiderRestUrl = configInfo.Value
		log.Debug().Msg("<SPIDER_REST_URL> " + SpiderRestUrl)
	case StrDragonflyRestUrl:
		DragonflyRestUrl = configInfo.Value
		log.Debug().Msg("<DRAGONFLY_REST_URL> " + DragonflyRestUrl)
	case StrDBUrl:
		DBUrl = configInfo.Value
		log.Debug().Msg("<DB_URL> " + DBUrl)
	case StrDBDatabase:
		DBDatabase = configInfo.Value
		log.Debug().Msg("<DB_DATABASE> " + DBDatabase)
	case StrDBUser:
		DBUser = configInfo.Value
		log.Debug().Msg("<DB_USER> " + DBUser)
	case StrDBPassword:
		DBPassword = configInfo.Value
		log.Debug().Msg("<DB_PASSWORD> " + DBPassword)
	case StrAutocontrolDurationMs:
		AutocontrolDurationMs = configInfo.Value
		log.Debug().Msg("<AUTOCONTROL_DURATION_MS> " + AutocontrolDurationMs)
	default:

	}

	return nil
}

func InitConfig(id string) error {

	switch id {
	case StrSpiderRestUrl:
		SpiderRestUrl = NVL(os.Getenv("SPIDER_REST_URL"), "http://localhost:1024/spider")
		log.Debug().Msg("<SPIDER_REST_URL> " + SpiderRestUrl)
	case StrDragonflyRestUrl:
		DragonflyRestUrl = NVL(os.Getenv("DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
		log.Debug().Msg("<DRAGONFLY_REST_URL> " + DragonflyRestUrl)
	case StrDBUrl:
		DBUrl = NVL(os.Getenv("DB_URL"), "localhost:3306")
		log.Debug().Msg("<DB_URL> " + DBUrl)
	case StrDBDatabase:
		DBDatabase = NVL(os.Getenv("DB_DATABASE"), "cb_tumblebug")
		log.Debug().Msg("<DB_DATABASE> " + DBDatabase)
	case StrDBUser:
		DBUser = NVL(os.Getenv("DB_USER"), "cb_tumblebug")
		log.Debug().Msg("<DB_USER> " + DBUser)
	case StrDBPassword:
		DBPassword = NVL(os.Getenv("DB_PASSWORD"), "cb_tumblebug")
		log.Debug().Msg("<DB_PASSWORD> " + DBPassword)
	case StrAutocontrolDurationMs:
		AutocontrolDurationMs = NVL(os.Getenv("AUTOCONTROL_DURATION_MS"), "10000")
		log.Debug().Msg("<AUTOCONTROL_DURATION_MS> " + AutocontrolDurationMs)
	default:

	}

	check, err := CheckConfig(id)

	if check && err == nil {
		log.Debug().Msg("[Init config] " + id)
		key := "/config/" + id

		CBStore.Delete(key)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }
	}

	return nil
}

func GetConfig(id string) (ConfigInfo, error) {

	res := ConfigInfo{}

	check, err := CheckConfig(id)
	errString := id + " config is not found from Key-value store. Envirionment variable will be used."

	if !check {
		err := fmt.Errorf(errString)
		return res, err
	}

	if err != nil {
		err := fmt.Errorf(errString)
		return res, err
	}

	key := "/config/" + id

	keyValue, err := CBStore.Get(key)
	if err != nil {
		err := fmt.Errorf(errString)
		return res, err
	}

	log.Debug().Msg("<" + keyValue.Key + "> " + keyValue.Value)

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		err := fmt.Errorf(errString)
		return res, err
	}
	return res, nil
}

func ListConfig() ([]ConfigInfo, error) {
	log.Debug().Msg("[List config]")
	key := "/config"
	log.Debug().Msg(key)

	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		res := []ConfigInfo{}
		for _, v := range keyValue {
			tempObj := ConfigInfo{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			res = append(res, tempObj)
		}
		return res, nil
		//return true, nil
	}
	return nil, nil // When err == nil && keyValue == nil
}

func ListConfigId() []string {

	log.Debug().Msg("[List config]")
	key := "/config"
	log.Debug().Msg(key)

	keyValue, _ := CBStore.GetList(key, true)

	var configList []string
	for _, v := range keyValue {
		configList = append(configList, strings.TrimPrefix(v.Key, "/config/"))
	}
	for _, v := range configList {
		fmt.Println("<" + v + "> \n")
	}

	return configList

}

/*
func DelAllConfig() error {
	fmt.Printf("DelAllConfig() called;")

	key := "/config"
	log.Debug().Msg(key)
	keyValue, _ := CBStore.GetList(key, true)

	if len(keyValue) == 0 {
		return nil
	}

	for _, v := range keyValue {
		err = CBStore.Delete(v.Key)
		if err != nil {
			return err
		}
	}
	return nil
}
*/

func InitAllConfig() error {
	configIdList := ListConfigId()
	for _, v := range configIdList {
		InitConfig(v)
	}
	return nil
}

func CheckConfig(id string) (bool, error) {

	if id == "" {
		err := fmt.Errorf("CheckConfig failed; configId given is null.")
		return false, err
	}

	key := "/config/" + id

	keyValue, _ := CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}
