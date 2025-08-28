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

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"

	"github.com/rs/zerolog/log"
)

// RuntimeCloudInfo is global variable for model.CloudInfo
var RuntimeCloudInfo = model.CloudInfo{}
var RuntimeCredential = model.Credential{}

// RuntimeNetworkInfo is global variable for model.NetworkInfo
var RuntimeCloudNetworkInfo = model.CloudNetworkInfo{}

// RuntimeK8sClusterInfo is global variable for model.K8sClusterAssetInfo
var RuntimeK8sClusterInfo = model.K8sClusterAssetInfo{}

// RuntimeExtractPatternsInfo is global variable for model.ExtractPatternsInfo
var RuntimeExtractPatternsInfo = model.ExtractPatternsInfo{}

// RuntimeLatancyMap is global variable for LatancyMap
var RuntimeLatancyMap = [][]string{}

// RuntimeLatancyMapIndex is global variable for LatancyMap (index)
var RuntimeLatancyMapIndex = make(map[string]int)

// RuntimeConf is global variable for cloud config
var RuntimeConf = model.RuntimeConfig{}

// AdjustKeysToLowercase adjusts the keys of nested maps to lowercase.
func AdjustKeysToLowercase(cloudInfo *model.CloudInfo) {
	newCSPs := make(map[string]model.CSPDetail)
	for cspKey, cspDetail := range cloudInfo.CSPs {
		lowerCSPKey := strings.ToLower(cspKey)
		newRegions := make(map[string]model.RegionDetail)
		for regionKey, regionDetail := range cspDetail.Regions {
			lowerRegionKey := strings.ToLower(regionKey)
			regionDetail.RegionName = lowerRegionKey
			// keep the original regionId if it is not empty (some CSP uses regionName case-sensitive)
			if regionDetail.RegionId == "" {
				regionDetail.RegionId = regionKey
			}
			newRegions[lowerRegionKey] = regionDetail
		}
		cspDetail.Regions = newRegions
		newCSPs[lowerCSPKey] = cspDetail
	}
	cloudInfo.CSPs = newCSPs
}

// PrintCloudInfoTable prints model.CloudInfo in table format
func PrintCloudInfoTable(cloudInfo model.CloudInfo) {
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

// PrintCredentialInfo prints model.Credential information in table format
func PrintCredentialInfo(credential model.Credential) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Credentialholder", "Cloud Service Provider", "model.Credential Key", "model.Credential Value"})

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
		{Name: "model.Credential Key", Mode: table.Asc},
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

func UpdateConfig(u *model.ConfigReq) (model.ConfigInfo, error) {

	if u.Name == "" {
		return model.ConfigInfo{}, fmt.Errorf("The provided name is empty.")
	}

	content := model.ConfigInfo{}
	content.Id = u.Name
	content.Name = u.Name
	content.Value = u.Value

	key := "/config/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	val, _ := json.Marshal(content)
	err := kvstore.Put(key, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return content, err
	}

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
	case model.StrSpiderRestUrl:
		model.SpiderRestUrl = configInfo.Value
		log.Debug().Msg("<TB_SPIDER_REST_URL> " + model.SpiderRestUrl)
	case model.StrDragonflyRestUrl:
		model.DragonflyRestUrl = configInfo.Value
		log.Debug().Msg("<TB_DRAGONFLY_REST_URL> " + model.DragonflyRestUrl)
	case model.StrTerrariumRestUrl:
		model.TerrariumRestUrl = configInfo.Value
		log.Debug().Msg("<TB_TERRARIUM_REST_URL> " + model.TerrariumRestUrl)
	case model.StrDBUrl:
		model.DBUrl = configInfo.Value
		log.Debug().Msg("<TB_POSTGRES_ENDPOINT> " + model.DBUrl)
	case model.StrDBDatabase:
		model.DBDatabase = configInfo.Value
		log.Debug().Msg("<TB_POSTGRES_DATABASE> " + model.DBDatabase)
	case model.StrDBUser:
		model.DBUser = configInfo.Value
		log.Debug().Msg("<TB_POSTGRES_USER> " + model.DBUser)
	case model.StrDBPassword:
		model.DBPassword = configInfo.Value
		log.Debug().Msg("<TB_POSTGRES_PASSWORD> " + model.DBPassword)
	case model.StrAutocontrolDurationMs:
		model.AutocontrolDurationMs = configInfo.Value
		log.Debug().Msg("<TB_AUTOCONTROL_DURATION_MS> " + model.AutocontrolDurationMs)
	case model.StrEtcdEndpoints:
		model.EtcdEndpoints = configInfo.Value
		log.Debug().Msg("<TB_ETCD_ENDPOINTS> " + model.EtcdEndpoints)
	default:

	}

	return nil
}

func InitConfig(id string) error {

	switch id {
	case model.StrSpiderRestUrl:
		model.SpiderRestUrl = NVL(os.Getenv("TB_SPIDER_REST_URL"), "http://localhost:1024/spider")
		log.Debug().Msg("<TB_SPIDER_REST_URL> " + model.SpiderRestUrl)
	case model.StrDragonflyRestUrl:
		model.DragonflyRestUrl = NVL(os.Getenv("TB_DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
		log.Debug().Msg("<TB_DRAGONFLY_REST_URL> " + model.DragonflyRestUrl)
	case model.StrTerrariumRestUrl:
		model.TerrariumRestUrl = NVL(os.Getenv("TB_TERRARIUM_REST_URL"), "http://localhost:8055/terrarium")
		log.Debug().Msg("<TB_TERRARIUM_REST_URL> " + model.TerrariumRestUrl)
	case model.StrDBUrl:
		model.DBUrl = NVL(os.Getenv("TB_POSTGRES_ENDPOINT"), "localhost:3306")
		log.Debug().Msg("<TB_POSTGRES_ENDPOINT> " + model.DBUrl)
	case model.StrDBDatabase:
		model.DBDatabase = NVL(os.Getenv("TB_POSTGRES_DATABASE"), "cb_tumblebug")
		log.Debug().Msg("<TB_POSTGRES_DATABASE> " + model.DBDatabase)
	case model.StrDBUser:
		model.DBUser = NVL(os.Getenv("TB_POSTGRES_USER"), "cb_tumblebug")
		log.Debug().Msg("<TB_POSTGRES_USER> " + model.DBUser)
	case model.StrDBPassword:
		model.DBPassword = NVL(os.Getenv("TB_POSTGRES_PASSWORD"), "cb_tumblebug")
		log.Debug().Msg("<TB_POSTGRES_PASSWORD> " + model.DBPassword)
	case model.StrAutocontrolDurationMs:
		model.AutocontrolDurationMs = NVL(os.Getenv("TB_AUTOCONTROL_DURATION_MS"), "10000")
		log.Debug().Msg("<TB_AUTOCONTROL_DURATION_MS> " + model.AutocontrolDurationMs)
	default:

	}

	check, err := CheckConfig(id)

	if check && err == nil {
		log.Debug().Msg("[Init config] " + id)
		key := "/config/" + id

		kvstore.Delete(key)
		// if err != nil {
		// 	log.Error().Err(err).Msg("")
		// 	return err
		// }
	}

	return nil
}

func GetConfig(id string) (model.ConfigInfo, error) {

	res := model.ConfigInfo{}

	check, err := CheckConfig(id)
	if !check {
		err = fmt.Errorf("config '%s' not found in key-value store: no configuration data exists, will fallback to environment variables", id)
		return res, err
	}

	if err != nil {
		err := fmt.Errorf("failed to check config '%s': configuration validation error - %v", id, err)
		return res, err
	}

	key := "/config/" + id

	keyValue, _, err := kvstore.GetKv(key)
	if err != nil {
		err := fmt.Errorf("failed to retrieve config '%s' from key-value store: %v (path: %s)", id, err, key)
		return res, err
	}

	log.Debug().Msg("<" + keyValue.Key + "> " + keyValue.Value)

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		err := fmt.Errorf("failed to parse config '%s': invalid JSON format in key-value store - %v (raw value: %s)", id, err, keyValue.Value)
		return res, err
	}
	return res, nil
}

func ListConfig() ([]model.ConfigInfo, error) {
	log.Debug().Msg("[List config]")
	key := "/config"
	log.Debug().Msg(key)

	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	if keyValue != nil {
		res := []model.ConfigInfo{}
		for _, v := range keyValue {
			tempObj := model.ConfigInfo{}
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

	keyValue, _ := kvstore.GetKvList(key)

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
	keyValue, _ := kvstore.GetKvList(key)

	if len(keyValue) == 0 {
		return nil
	}

	for _, v := range keyValue {
		err = kvstore.Delete(v.Key)
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

	_, exists, _ := kvstore.GetKv(key)
	if exists {
		return true, nil
	}
	return false, nil
}
