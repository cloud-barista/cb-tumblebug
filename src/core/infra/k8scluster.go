/*
Copyright 2023 The Cloud-Barista Authors.
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

// Package mci is to manage multi-cloud infra
package infra

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// TbK8sClusterReqStructLevelValidation is a function to validate 'model.TbK8sClusterReq' object.
func TbK8sClusterReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbK8sClusterReq)

	err := common.CheckString(u.Id)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Id, "id", "Id", err.Error(), "")
	}
}

// CreateK8sCluster create a k8s cluster
func CreateK8sCluster(nsId string, u *model.TbK8sClusterReq, option string) (model.TbK8sClusterInfo, error) {
	log.Info().Msg("CreateK8sCluster")

	emptyObj := model.TbK8sClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(u.Id)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("Failed to Create a K8sCluster")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, u.Id)
	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	if check {
		err := fmt.Errorf("The k8s cluster " + u.Id + " already exists.")
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Check for K8sCluster Enablement from K8sClusterSetting
	 */
	err = checkK8sClusterEnablement(u.ConnectionName)
	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for model.SpiderClusterReq{}
	 */

	// Validate
	err = validateAtCreateK8sCluster(u)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster: Requested K8sVersion(%s)", u.Version)
		return emptyObj, err
	}
	spVersion := u.Version

	spVPCName, err := resource.GetCspResourceId(nsId, model.StrVNet, u.VNetId)
	if spVPCName == "" {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	/*
		var spSubnetNames []string
		for _, v := range u.SubnetIds {
			spSnName, err := resource.GetCspResourceId(nsId, model.StrSubnet, v)
			if spSnName == "" {
				log.Error().Err(err).Msg("")
				return emptyObj, err
			}

			spSubnetNames = append(spSubnetNames, spSnName)
		}
	*/

	var spSnName string
	var spSubnetNames []string
	var found bool

	tmpInf, err := resource.GetResource(nsId, model.StrVNet, u.VNetId)
	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}
	tbVNetInfo := model.TbVNetInfo{}
	err = common.CopySrcToDest(&tmpInf, &tbVNetInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	for _, v := range u.SubnetIds {
		found = false
		for _, w := range tbVNetInfo.SubnetInfoList {
			if v == w.Name {
				spSnName = w.Name
				found = true
				break
			}
		}

		if found == true {
			spSubnetNames = append(spSubnetNames, spSnName)
		}
	}
	if len(spSubnetNames) == 0 {
		err := fmt.Errorf("No valid subnets in VNetId(%s)", u.VNetId)
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	var spSecurityGroupNames []string
	for _, v := range u.SecurityGroupIds {
		spSgName, err := resource.GetCspResourceId(nsId, model.StrSecurityGroup, v)
		if spSgName == "" {
			log.Err(err).Msg("Failed to Create a K8sCluster")
			return emptyObj, err
		}

		spSecurityGroupNames = append(spSecurityGroupNames, spSgName)
	}

	var spNodeGroupList []model.SpiderNodeGroupReqInfo
	for _, v := range u.K8sNodeGroupList {
		err := common.CheckString(v.Name)
		if err != nil {
			log.Err(err).Msg("Failed to Create a K8sCluster")
			return emptyObj, err
		}

		spImgName := "" // Some CSPs do not require ImageName for creating a k8s cluster
		if v.ImageId == "" || v.ImageId == "default" {
			spImgName = ""
		} else {
			spImgName, err = resource.GetCspResourceId(nsId, model.StrImage, v.ImageId)
			if spImgName == "" {
				log.Err(err).Msg("Failed to Create a K8sCluster")
				return emptyObj, err
			}
		}

		// specInfo, err := resource.GetSpec(model.SystemCommonNs, v.SpecId)
		// if err != nil {
		// 	log.Err(err).Msg("Failed to Create a K8sCluster")
		// 	return emptyObj, err
		// }
		// spSpecName := specInfo.CspSpecName
		spSpecName := v.SpecId

		spKpName, err := resource.GetCspResourceId(nsId, model.StrSSHKey, v.SshKeyId)
		if spKpName == "" {
			log.Err(err).Msg("Failed to Create a K8sCluster")
			return emptyObj, err
		}

		spNodeGroupList = append(spNodeGroupList, model.SpiderNodeGroupReqInfo{
			Name:            common.GenUid(),
			ImageName:       spImgName,
			VMSpecName:      spSpecName,
			RootDiskType:    v.RootDiskType,
			RootDiskSize:    v.RootDiskSize,
			KeyPairName:     spKpName,
			OnAutoScaling:   v.OnAutoScaling,
			DesiredNodeSize: v.DesiredNodeSize,
			MinNodeSize:     v.MinNodeSize,
			MaxNodeSize:     v.MaxNodeSize,
		})
	}

	uuid := common.GenUid()

	requestBody := model.SpiderClusterReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: u.ConnectionName,
		ReqInfo: model.SpiderClusterReqInfo{
			Name:               uuid,
			Version:            spVersion,
			VPCName:            spVPCName,
			SubnetNames:        spSubnetNames,
			SecurityGroupNames: spSecurityGroupNames,
			NodeGroupList:      spNodeGroupList,
		},
	}

	// Randomly sleep within 20 Secs to avoid rateLimit from CSP
	//common.RandomSleep(0, 20)
	client := resty.New()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := model.SpiderRestUrl

	if option == "register" {
		url = url + "/regcluster"
	} else { // option != "register"
		url = url + "/cluster"
	}

	var spClusterRes model.SpiderClusterRes

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build model.TbK8sClusterInfo object
	 */

	tbK8sCInfo := convertSpiderClusterInfoToTbK8sClusterInfo(&spClusterRes.ClusterInfo, u.Id, u.ConnectionName, u.Description)
	tbK8sCInfo.Uuid = uuid

	if option == "register" && u.CspK8sClusterId == "" {
		tbK8sCInfo.SystemLabel = "Registered from CB-Spider resource"
		// TODO: check to handle something to register
	} else if option == "register" && u.CspK8sClusterId != "" {
		tbK8sCInfo.SystemLabel = "Registered from CSP resource"
	}

	/*
	 * Put/Get model.TbK8sClusterInfo to/from kvstore
	 */
	k := GenK8sClusterKey(nsId, tbK8sCInfo.Id)
	Val, _ := json.Marshal(tbK8sCInfo)

	err = kvstore.Put(k, string(Val))
	if err != nil {
		log.Err(err).Msg("Failed to Create a K8sCluster")
		return tbK8sCInfo, err
	}

	kv, err := kvstore.GetKv(k)
	if err != nil {
		err = fmt.Errorf("In CreateK8sCluster(); kvstore.GetKv() returned an error: " + err.Error())
		log.Err(err).Msg("")
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("")
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		"provider":  "cb-tumblebug",
		"namespace": nsId,
	}
	err = label.CreateOrUpdateLabel(model.StrK8s, uuid, k, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return tbK8sCInfo, err
	}

	return storedTbK8sCInfo, nil
}

// AddK8sNodeGroup adds a NodeGroup
func AddK8sNodeGroup(nsId string, k8sClusterId string, u *model.TbK8sNodeGroupReq) (model.TbK8sClusterInfo, error) {
	log.Info().Msg("AddK8sNodeGroup")

	emptyObj := model.TbK8sClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("Failed to Add K8sNodeGroup")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	/*
	 * Get model.TbK8sClusterInfo from kvstore
	 */
	oldTbK8sCInfo := model.TbK8sClusterInfo{}
	k := GenK8sClusterKey(nsId, k8sClusterId)
	kv, err := kvstore.GetKv(k)
	if err != nil {
		err = fmt.Errorf("In AddK8sNodeGroup(); kvstore.GetKv() returned an error: " + err.Error())
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	err = json.Unmarshal([]byte(kv.Value), &oldTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	/*
	 * Check for K8sCluster Enablement from ClusterSetting
	 */

	err = checkK8sClusterEnablement(oldTbK8sCInfo.ConnectionName)
	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for SpiderNodeGroupReq{}
	 */

	spName := u.Name
	err = common.CheckString(spName)
	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	spImgName := "" // Some CSPs do not require ImageName for creating a cluster
	if u.ImageId != "" {
		spImgName, err = resource.GetCspResourceId(nsId, model.StrImage, u.ImageId)
		if spImgName == "" {
			log.Err(err).Msg("Failed to Add K8sNodeGroup")
			return emptyObj, err
		}
	}

	// specInfo, err := resource.GetSpec(model.SystemCommonNs, u.SpecId)
	// if err != nil {
	// 	log.Err(err).Msg("Failed to Add K8sNodeGroup")
	// 	return emptyObj, err
	// }
	// spSpecName := specInfo.CspSpecName
	spSpecName := u.SpecId

	spKpName, err := resource.GetCspResourceId(nsId, model.StrSSHKey, u.SshKeyId)
	if spKpName == "" {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	requestBody := model.SpiderNodeGroupReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: oldTbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderNodeGroupReqInfo{
			Name:         spName,
			ImageName:    spImgName,
			VMSpecName:   spSpecName,
			RootDiskType: u.RootDiskType,
			RootDiskSize: u.RootDiskSize,
			KeyPairName:  spKpName,

			// autoscale config.
			OnAutoScaling:   u.OnAutoScaling,
			DesiredNodeSize: u.DesiredNodeSize,
			MinNodeSize:     u.MinNodeSize,
			MaxNodeSize:     u.MaxNodeSize,
		},
	}

	client := resty.New()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := model.SpiderRestUrl + "/cluster/" + oldTbK8sCInfo.CspK8sClusterName + "/nodegroup"

	var spClusterRes model.SpiderClusterRes

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build model.TbK8sClusterInfo object
	 */

	newTbK8sCInfo := convertSpiderClusterInfoToTbK8sClusterInfo(&spClusterRes.ClusterInfo, oldTbK8sCInfo.Id, oldTbK8sCInfo.ConnectionName, oldTbK8sCInfo.Description)

	/*
	 * Put/Get model.TbK8sClusterInfo to/from kvstore
	 */
	k = GenK8sClusterKey(nsId, newTbK8sCInfo.Id)
	Val, _ := json.Marshal(newTbK8sCInfo)

	err = kvstore.Put(k, string(Val))
	if err != nil {
		log.Err(err).Msg("Failed to Add K8sNodeGroup")
		return newTbK8sCInfo, err
	}

	kv, err = kvstore.GetKv(k)
	if err != nil {
		err = fmt.Errorf("In AddK8sNodeGroup(); kvstore.GetKv() returned an error: " + err.Error())
		log.Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("")
	}
	return storedTbK8sCInfo, nil
}

// RemoveK8sNodeGroup removes a specified NodeGroup
func RemoveK8sNodeGroup(nsId string, k8sClusterId string, k8sNodeGroupName string, forceFlag string) (bool, error) {
	log.Info().Msg("RemoveK8sNodeGroup")
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msg("Failed to Remove K8sNodeGroup")
		return false, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Remove K8sNodeGroup")
		return false, err
	}

	k := GenK8sClusterKey(nsId, k8sClusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := kvstore.GetKv(k)

	// Create Req body
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	tbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Remove K8sNodeGroup")
		return false, err
	}

	requestBody.NameSpace = "" // should be empty string from Tumblebug
	requestBody.ConnectionName = tbK8sCInfo.ConnectionName

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspK8sClusterName + "/nodegroup/" + k8sNodeGroupName
	if forceFlag == "true" {
		url += "?force=true"
	}
	method := "DELETE"

	var ifRes interface{}
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Remove K8sNodeGroup")
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				return true, nil
			}
		}
	}

	return false, nil
}

// SetK8sNodeGroupAutoscaling set NodeGroup's Autoscaling On/Off
func SetK8sNodeGroupAutoscaling(nsId string, k8sClusterId string, k8sNodeGroupName string, u *model.TbSetK8sNodeGroupAutoscalingReq) (model.TbSetK8sNodeGroupAutoscalingRes, error) {
	log.Info().Msg("SetK8sNodeGroupAutoscaling")

	emptyObj := model.TbSetK8sNodeGroupAutoscalingRes{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msg("Failed to Set K8sNodeGroup Autoscaling")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Set K8sNodeGroup Autoscaling")
		return emptyObj, err
	}

	err = common.CheckString(k8sNodeGroupName)
	if err != nil {
		log.Err(err).Msg("Failed to Set K8sNodeGroup Autoscaling")
		return emptyObj, err
	}

	/*
	 * Get model.TbK8sClusterInfo object from kvstore
	 */

	k := GenK8sClusterKey(nsId, k8sClusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := kvstore.GetKv(k)

	tbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Set K8sNodeGroup Autoscaling")
		return emptyObj, err
	}

	requestBody := model.SpiderSetAutoscalingReq{
		ConnectionName: tbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderSetAutoscalingReqInfo{
			OnAutoScaling: u.OnAutoScaling,
		},
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspK8sClusterName + "/nodegroup/" + k8sNodeGroupName + "/onautoscaling"
	method := "PUT"

	var spSetAutoscalingRes model.SpiderSetAutoscalingRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spSetAutoscalingRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Set K8sNodeGroup Autoscaling")
		return emptyObj, err
	}

	var tbK8sSetAutoscalingRes model.TbSetK8sNodeGroupAutoscalingRes
	tbK8sSetAutoscalingRes.Result = spSetAutoscalingRes.Result

	return tbK8sSetAutoscalingRes, nil
}

// ChangeK8sNodeGroupAutoscaleSize change NodeGroup's Autoscaling Size
func ChangeK8sNodeGroupAutoscaleSize(nsId string, k8sClusterId string, k8sNodeGroupName string, u *model.TbChangeK8sNodeGroupAutoscaleSizeReq) (model.TbChangeK8sNodeGroupAutoscaleSizeRes, error) {
	log.Info().Msg("ChangeK8sNodeGroupAutoscaleSize")

	emptyObj := model.TbChangeK8sNodeGroupAutoscaleSizeRes{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msg("Failed to Change K8sNodeGroup AutoscaleSize")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Change K8sNodeGroup AutoscaleSize")
		return emptyObj, err
	}

	err = common.CheckString(k8sNodeGroupName)
	if err != nil {
		log.Err(err).Msg("Failed to Change K8sNodeGroup AutoscaleSize")
		return emptyObj, err
	}

	/*
	 * Get model.TbK8sClusterInfo object from kvstore
	 */

	k := GenK8sClusterKey(nsId, k8sClusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := kvstore.GetKv(k)

	tbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Change K8sNodeGroup AutoscaleSize")
		return emptyObj, err
	}

	requestBody := model.SpiderChangeAutoscaleSizeReq{
		ConnectionName: tbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderChangeAutoscaleSizeReqInfo{
			DesiredNodeSize: u.DesiredNodeSize,
			MinNodeSize:     u.MinNodeSize,
			MaxNodeSize:     u.MaxNodeSize,
		},
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspK8sClusterName + "/nodegroup/" + k8sNodeGroupName + "/autoscalesize"
	method := "PUT"

	var spChangeAutoscaleSizeRes model.SpiderChangeAutoscaleSizeRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spChangeAutoscaleSizeRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Change K8sNodeGroup AutoscaleSize")
		return emptyObj, err
	}

	var tbK8sCAutoscaleSizeRes model.TbChangeK8sNodeGroupAutoscaleSizeRes
	tbK8sCAutoscaleSizeRes.TbK8sNodeGroupInfo = convertSpiderNodeGroupInfoToTbK8sNodeGroupInfo(&spChangeAutoscaleSizeRes.NodeGroupInfo)

	return tbK8sCAutoscaleSizeRes, nil
}

// GetK8sCluster retrives a k8s cluster information
func GetK8sCluster(nsId string, k8sClusterId string) (model.TbK8sClusterInfo, error) {

	emptyObj := model.TbK8sClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Get K8sCluster")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Get K8sCluster")
		return emptyObj, err
	}

	log.Debug().Msg("[Get K8sCluster] " + k8sClusterId)

	/*
	 * Get model.TbK8sClusterInfo object from kvstore
	 */
	k := GenK8sClusterKey(nsId, k8sClusterId)

	kv, err := kvstore.GetKv(k)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	storedTbK8sCInfo := model.TbK8sClusterInfo{}
	if kv == (kvstore.KeyValue{}) {
		err = fmt.Errorf("Cannot get the k8s cluster " + k8sClusterId + ".")
		log.Err(err).Msg("Failed to Get K8sCluster")
		return storedTbK8sCInfo, err
	}

	err = json.Unmarshal([]byte(kv.Value), &storedTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Get K8sCluster")
		return storedTbK8sCInfo, err
	}

	/*
	 * Get model.TbK8sClusterInfo object from CB-Spider
	 */

	client := resty.New()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/cluster/" + storedTbK8sCInfo.CspK8sClusterName
	method := "GET"

	// Create Request body for GetK8sCluster of CB-Spider
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: storedTbK8sCInfo.ConnectionName,
	}

	var spClusterRes model.SpiderClusterRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Get K8sCluster")
		return emptyObj, err
	}

	tbK8sCInfo := convertSpiderClusterInfoToTbK8sClusterInfo(&spClusterRes.ClusterInfo, k8sClusterId, storedTbK8sCInfo.ConnectionName, storedTbK8sCInfo.Description)

	/*
	 * FIXME: Do not compare, just store?
	 * Compare tbK8sCInfo with storedTbK8sCInfo
	 */
	if !isEqualTbK8sClusterInfoExceptStatus(storedTbK8sCInfo, tbK8sCInfo) {
		err := fmt.Errorf("The k8s cluster " + k8sClusterId + " has been changed something.")
		log.Err(err).Msg("Failed to Get K8sCluster")
		return emptyObj, err
	}

	return tbK8sCInfo, nil
}

func isEqualTbK8sClusterInfoExceptStatus(info1 model.TbK8sClusterInfo, info2 model.TbK8sClusterInfo) bool {

	// FIX: now compare some fields only

	if info1.Id != info2.Id ||
		info1.Name != info2.Name ||
		info1.ConnectionName != info2.ConnectionName ||
		info1.Description != info2.Description ||
		info1.CspK8sClusterId != info2.CspK8sClusterId ||
		info1.CspK8sClusterName != info2.CspK8sClusterName ||
		info1.CreatedTime != info2.CreatedTime {
		return false

	}

	return true
}

// CheckK8sCluster returns the existence of the TB K8sCluster object in bool form.
func CheckK8sCluster(nsId string, k8sClusterId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckK8sCluster failed; nsId given is empty.")
		return false, err
	} else if k8sClusterId == "" {
		err := fmt.Errorf("CheckK8sCluster failed; k8sClusterId given is empty.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sCluster")
		return false, err
	}

	err = common.CheckString(k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sCluster")
		return false, err
	}

	log.Debug().Msg("[Check K8sCluster] " + k8sClusterId)

	key := GenK8sClusterKey(nsId, k8sClusterId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sCluster")
		return false, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		return true, nil
	}
	return false, nil
}

// GenK8sClusterKey is func to generate a key from K8sCluster ID
func GenK8sClusterKey(nsId string, k8sClusterId string) string {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	err = common.CheckString(k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Generate K8sCluster Key")
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/k8scluster/%s", nsId, k8sClusterId)
}

// ListK8sClusterId returns the list of TB K8sCluster object IDs of given nsId
func ListK8sClusterId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[ListK8sClusterId] ns: " + nsId)
	// key := "/ns/" + nsId + "/"
	k := fmt.Sprintf("/ns/%s/", nsId)
	log.Debug().Msg(k)

	kv, err := kvstore.GetKvList(k)

	if err != nil {
		log.Error().Err(err).Msg("Failed to Get K8sClusterId List")
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		log.Error().Err(err).Msg("")
		return nil, err
	}
	*/

	var k8sClusterIds []string
	for _, v := range kv {
		trimmed := strings.TrimPrefix(v.Key, (k + "k8scluster/"))
		// prevent malformed key (if key for K8sCluster ID includes '/', the key does not represent K8sCluster ID)
		if !strings.Contains(trimmed, "/") {
			k8sClusterIds = append(k8sClusterIds, trimmed)
		}
	}

	return k8sClusterIds, nil
}

// ListK8sCluster returns the list of TB K8sCluster objects of given nsId
func ListK8sCluster(nsId string, filterKey string, filterVal string) (interface{}, error) {
	log.Info().Msg("ListK8sCluster")

	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("Failed to List K8sCluster")
		return nil, err
	}

	log.Debug().Msg("[Get] K8sCluster list")
	k := fmt.Sprintf("/ns/%s/k8scluster", nsId)
	log.Debug().Msg(k)

	/*
	 * Get model.TbK8sClusterInfo objects from kvstore
	 */

	kv, err := kvstore.GetKvList(k)
	kv = kvutil.FilterKvListBy(kv, k, 1)

	if err != nil {
		log.Err(err).Msg("Failed to List K8sCluster")
		return nil, err
	}

	tbK8sCInfoList := []model.TbK8sClusterInfo{}

	if kv != nil {
		for _, v := range kv {
			tbK8sCInfo := model.TbK8sClusterInfo{}
			err = json.Unmarshal([]byte(v.Value), &tbK8sCInfo)
			if err != nil {
				log.Err(err).Msg("Failed to List K8sCluster")
				return nil, err
			}
			// Check the JSON body includes both filterKey and filterVal strings. (assume key and value)
			if filterKey != "" {
				// If not includes both, do not append current item to the list result.
				itemValueForCompare := strings.ToLower(v.Value)
				if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) &&
					strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
					continue
				}
			}
			tbK8sCInfoList = append(tbK8sCInfoList, tbK8sCInfo)
		}
	}

	return tbK8sCInfoList, nil
}

// DeleteK8sCluster deletes a k8s cluster
func DeleteK8sCluster(nsId string, k8sClusterId string, forceFlag string) (bool, error) {
	log.Info().Msg("DeleteK8sCluster")
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(k8sClusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msg("Failed to Delete K8sCluster")
		return false, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Delete K8sCluster")
		return false, err
	}

	/*
	 * Get model.TbK8sClusterInfo object from kvstore
	 */

	k := GenK8sClusterKey(nsId, k8sClusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := kvstore.GetKv(k)

	// Create Req body
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	tbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Delete K8sCluster")
		return false, err
	}

	requestBody.NameSpace = "" // should be empty string from Tumblebug
	requestBody.ConnectionName = tbK8sCInfo.ConnectionName

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspK8sClusterName
	if forceFlag == "true" {
		url += "?force=true"
	}
	method := "DELETE"

	var ifRes interface{}
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		common.VeryShortDuration,
	)

	if forceFlag == "true" {
		err = kvstore.Delete(k)
		if err != nil {
			log.Err(err).Msg("Failed to Delete K8sCluster")
			return false, err
		}
	}

	if err != nil {
		log.Err(err).Msg("Failed to Delete K8sCluster")
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				if forceFlag != "true" {
					err = kvstore.Delete(k)
					if err != nil {
						log.Err(err).Msg("Failed to Delete K8sCluster")
						return false, err
					}
				}

				return true, nil
			}
		}
	}

	return false, nil
}

// DeleteAllK8sCluster deletes all clusters
func DeleteAllK8sCluster(nsId string, subString string, forceFlag string) (model.IdList, error) {
	log.Info().Msg("DeleteAllK8sCluster")

	deletedK8sClusters := model.IdList{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msg("Failed to Delete All K8sCluster")
		return deletedK8sClusters, err
	}

	k8sClusterIdList, err := ListK8sClusterId(nsId)
	if err != nil {
		return deletedK8sClusters, err
	}

	for _, v := range k8sClusterIdList {
		// if subString is provided, check the k8sClusterId contains the subString.
		if subString == "" || strings.Contains(v, subString) {
			deleteStatus := ""

			res, err := DeleteK8sCluster(nsId, v, forceFlag)

			if err != nil {
				deleteStatus = err.Error()
			} else {
				deleteStatus = " [" + fmt.Sprintf("%t", res) + "]"
			}

			deletedK8sClusters.IdList = append(deletedK8sClusters.IdList, "Cluster: "+v+deleteStatus)
		}
	}
	return deletedK8sClusters, nil
}

// UpgradeK8sCluster upgrades an existing k8s cluster to the specified version
func UpgradeK8sCluster(nsId string, k8sClusterId string, u *model.TbUpgradeK8sClusterReq) (model.TbK8sClusterInfo, error) {
	log.Info().Msg("UpgradeK8sCluster")

	emptyObj := model.TbK8sClusterInfo{}

	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("Failed to Upgrade a K8sCluster")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Get model.TbK8sClusterInfo from kvstore
	 */
	oldTbK8sCInfo := model.TbK8sClusterInfo{}
	k := GenK8sClusterKey(nsId, k8sClusterId)
	kv, err := kvstore.GetKv(k)
	if err != nil {
		err = fmt.Errorf("In UpgradeK8sCluster(); kvstore.GetKv() returned an error: " + err.Error())
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	err = json.Unmarshal([]byte(kv.Value), &oldTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Check for K8sCluster Enablement from K8sClusterSetting
	 */

	err = checkK8sClusterEnablement(oldTbK8sCInfo.ConnectionName)
	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for model.SpiderUpgradeClusterReq{}
	 */

	// Validate
	err = validateAtUpgradeK8sCluster(oldTbK8sCInfo.ConnectionName, u)
	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}
	spVersion := u.Version

	requestBody := model.SpiderUpgradeClusterReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: oldTbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderUpgradeClusterReqInfo{
			Version: spVersion,
		},
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + oldTbK8sCInfo.CspK8sClusterName + "/upgrade"
	method := "PUT"

	var spClusterRes model.SpiderClusterRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build model.TbK8sClusterInfo object
	 */

	newTbK8sCInfo := convertSpiderClusterInfoToTbK8sClusterInfo(&spClusterRes.ClusterInfo, oldTbK8sCInfo.Id, oldTbK8sCInfo.ConnectionName, oldTbK8sCInfo.Description)

	/*
	 * Put/Get model.TbK8sClusterInfo to/from kvstore
	 */
	k = GenK8sClusterKey(nsId, newTbK8sCInfo.Id)
	Val, _ := json.Marshal(newTbK8sCInfo)

	err = kvstore.Put(k, string(Val))
	if err != nil {
		log.Err(err).Msg("Failed to Upgrade a K8sCluster")
		return emptyObj, err
	}

	kv, err = kvstore.GetKv(k)
	if err != nil {
		err = fmt.Errorf("In UpgradeK8sCluster(); kvstore.GetKv() returned an error: " + err.Error())
		log.Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbK8sCInfo := model.TbK8sClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbK8sCInfo)
	if err != nil {
		log.Err(err).Msg("")
	}

	return storedTbK8sCInfo, nil
}

func convertSpiderNetworkInfoToTbK8sClusterNetworkInfo(spNetworkInfo model.SpiderNetworkInfo) model.TbK8sClusterNetworkInfo {
	tbVNetId := spNetworkInfo.VpcIID.SystemId

	var tbSubnetIds []string
	for _, v := range spNetworkInfo.SubnetIIDs {
		tbSubnetIds = append(tbSubnetIds, v.SystemId)
	}

	var tbSecurityGroupIds []string
	for _, v := range spNetworkInfo.SecurityGroupIIDs {
		tbSecurityGroupIds = append(tbSecurityGroupIds, v.SystemId)
	}

	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spNetworkInfo.KeyValueList)

	tbK8sClusterNetworkInfo := model.TbK8sClusterNetworkInfo{
		VNetId:           tbVNetId,
		SubnetIds:        tbSubnetIds,
		SecurityGroupIds: tbSecurityGroupIds,
		KeyValueList:     tbKeyValueList,
	}

	return tbK8sClusterNetworkInfo
}

func convertSpiderNodeGroupInfoToTbK8sNodeGroupInfo(spNodeGroupInfo *model.SpiderNodeGroupInfo) model.TbK8sNodeGroupInfo {
	tbNodeId := spNodeGroupInfo.IId.SystemId
	tbImageId := spNodeGroupInfo.ImageIID.SystemId
	tbSpecId := spNodeGroupInfo.VMSpecName
	tbRootDiskType := spNodeGroupInfo.RootDiskType
	tbRootDiskSize := spNodeGroupInfo.RootDiskSize
	tbSshKeyId := spNodeGroupInfo.KeyPairIID.SystemId
	tbOnAutoScaling := spNodeGroupInfo.OnAutoScaling
	tbDesiredNodeSize := spNodeGroupInfo.DesiredNodeSize
	tbMinNodeSize := spNodeGroupInfo.MinNodeSize
	tbMaxNodeSize := spNodeGroupInfo.MaxNodeSize
	tbStatus := convertSpiderNodeGroupStatusToTbK8sNodeGroupStatus(spNodeGroupInfo.Status)

	var tbK8sNodes []string
	for _, v := range spNodeGroupInfo.Nodes {
		tbK8sNodes = append(tbK8sNodes, v.SystemId)
	}

	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spNodeGroupInfo.KeyValueList)
	tbK8sNodeGroupInfo := model.TbK8sNodeGroupInfo{
		Id:              tbNodeId,
		ImageId:         tbImageId,
		SpecId:          tbSpecId,
		RootDiskType:    tbRootDiskType,
		RootDiskSize:    tbRootDiskSize,
		SshKeyId:        tbSshKeyId,
		OnAutoScaling:   tbOnAutoScaling,
		DesiredNodeSize: tbDesiredNodeSize,
		MinNodeSize:     tbMinNodeSize,
		MaxNodeSize:     tbMaxNodeSize,
		Status:          tbStatus,
		K8sNodes:        tbK8sNodes,
		KeyValueList:    tbKeyValueList,
	}

	return tbK8sNodeGroupInfo
}

func convertSpiderNodeGroupListToTbK8sNodeGroupList(spNodeGroupList []model.SpiderNodeGroupInfo) []model.TbK8sNodeGroupInfo {
	var tbK8sNodeGroupList []model.TbK8sNodeGroupInfo
	for _, v := range spNodeGroupList {
		tbK8sNodeGroupInfo := convertSpiderNodeGroupInfoToTbK8sNodeGroupInfo(&v)
		tbK8sNodeGroupList = append(tbK8sNodeGroupList, tbK8sNodeGroupInfo)
	}

	return tbK8sNodeGroupList
}

func convertSpiderClusterAccessInfoToTbK8sAccessInfo(spAccessInfo model.SpiderAccessInfo) model.TbK8sAccessInfo {
	return model.TbK8sAccessInfo{spAccessInfo.Endpoint, spAccessInfo.Kubeconfig}
}

func convertSpiderClusterAddonsInfoToTbK8sAddonsInfo(spAddonsInfo model.SpiderAddonsInfo) model.TbK8sAddonsInfo {
	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spAddonsInfo.KeyValueList)
	return model.TbK8sAddonsInfo{tbKeyValueList}
}

func convertSpiderKeyValueListToTbKeyValueList(spKeyValueList []model.KeyValue) []model.KeyValue {
	var tbKeyValueList []model.KeyValue
	for _, v := range spKeyValueList {
		tbKeyValueList = append(tbKeyValueList, v)
	}
	return tbKeyValueList
}

func convertSpiderClusterInfoToTbK8sClusterInfo(spClusterInfo *model.SpiderClusterInfo, id string, connectionName string, description string) model.TbK8sClusterInfo {
	tbK8sCNInfo := convertSpiderNetworkInfoToTbK8sClusterNetworkInfo(spClusterInfo.Network)
	tbK8sNGList := convertSpiderNodeGroupListToTbK8sNodeGroupList(spClusterInfo.NodeGroupList)
	tbK8sCAccInfo := convertSpiderClusterAccessInfoToTbK8sAccessInfo(spClusterInfo.AccessInfo)
	tbK8sCAddInfo := convertSpiderClusterAddonsInfoToTbK8sAddonsInfo(spClusterInfo.Addons)
	tbK8sCStatus := convertSpiderClusterStatusToTbK8sClusterStatus(spClusterInfo.Status)
	tbKVList := convertSpiderKeyValueListToTbKeyValueList(spClusterInfo.KeyValueList)
	tbK8sCInfo := model.TbK8sClusterInfo{
		Id:                id,
		Name:              id,
		ConnectionName:    connectionName,
		Version:           spClusterInfo.Version,
		Network:           tbK8sCNInfo,
		K8sNodeGroupList:  tbK8sNGList,
		AccessInfo:        tbK8sCAccInfo,
		Addons:            tbK8sCAddInfo,
		Status:            tbK8sCStatus,
		CreatedTime:       spClusterInfo.CreatedTime,
		KeyValueList:      tbKVList,
		Description:       description,
		CspK8sClusterId:   spClusterInfo.IId.SystemId,
		CspK8sClusterName: spClusterInfo.IId.NameId,
	}

	return tbK8sCInfo
}

func convertSpiderClusterStatusToTbK8sClusterStatus(spClusterStatus model.SpiderClusterStatus) model.TbK8sClusterStatus {
	if spClusterStatus == model.SpiderClusterCreating {
		return model.TbK8sClusterCreating
	} else if spClusterStatus == model.SpiderClusterActive {
		return model.TbK8sClusterActive
	} else if spClusterStatus == model.SpiderClusterInactive {
		return model.TbK8sClusterInactive
	} else if spClusterStatus == model.SpiderClusterUpdating {
		return model.TbK8sClusterUpdating
	} else if spClusterStatus == model.SpiderClusterDeleting {
		return model.TbK8sClusterDeleting
	}

	return model.TbK8sClusterInactive
}

func convertSpiderNodeGroupStatusToTbK8sNodeGroupStatus(spNodeGroupStatus model.SpiderNodeGroupStatus) model.TbK8sNodeGroupStatus {
	if spNodeGroupStatus == model.SpiderNodeGroupCreating {
		return model.TbK8sNodeGroupCreating
	} else if spNodeGroupStatus == model.SpiderNodeGroupActive {
		return model.TbK8sNodeGroupActive
	} else if spNodeGroupStatus == model.SpiderNodeGroupInactive {
		return model.TbK8sNodeGroupInactive
	} else if spNodeGroupStatus == model.SpiderNodeGroupUpdating {
		return model.TbK8sNodeGroupUpdating
	} else if spNodeGroupStatus == model.SpiderNodeGroupDeleting {
		return model.TbK8sNodeGroupDeleting
	}

	return model.TbK8sNodeGroupInactive
}

// checkK8sClusterEnablement returns the enablement status(nil or error) for K8sCluster related to Connection.
func checkK8sClusterEnablement(connectionName string) error {
	connConfig, err := common.GetConnConfig(connectionName)
	if err != nil {
		err := fmt.Errorf("failed to get the connConfig " + connectionName + ": " + err.Error())
		return err
	}

	cloudType := connConfig.ProviderName

	// Convert cloud type to field name (e.g., AWS to Aws, OPENSTACK to Openstack)
	lowercase := strings.ToLower(cloudType)
	fnCloudType := strings.ToUpper(string(lowercase[0])) + lowercase[1:]

	// Get cloud setting with field name
	cloudSetting := model.CloudSetting{}

	getCloudSetting := func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("%v", err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(model.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fnCloudType).Interface().(model.CloudSetting)
	}

	getCloudSetting()

	if cloudSetting.K8sCluster.Enable != "y" {
		err := fmt.Errorf("k8scluster management function is not enabled for cloud(" + fnCloudType + ")")
		return err
	}

	return nil
}

func validateAtCreateK8sCluster(tbK8sClusterReq *model.TbK8sClusterReq) error {
	connConfig, err := common.GetConnConfig(tbK8sClusterReq.ConnectionName)

	// Validate K8sCluster Version
	err = validateK8sClusterVersion(connConfig.ProviderName, connConfig.RegionDetail.RegionName, tbK8sClusterReq.Version)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster: Requested K8sVersion(%s)", tbK8sClusterReq.Version)
		return err
	}

	return nil
}

func validateAtUpgradeK8sCluster(connectionName string, tbUpgradeK8sClusterReq *model.TbUpgradeK8sClusterReq) error {
	connConfig, err := common.GetConnConfig(connectionName)

	// Validate K8sCluster Version
	err = validateK8sClusterVersion(connConfig.ProviderName, connConfig.RegionDetail.RegionName, tbUpgradeK8sClusterReq.Version)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster: Requested K8sVersion(%s)", tbUpgradeK8sClusterReq.Version)
		return err
	}

	return nil
}

func validateK8sClusterVersion(providerName, regionName, version string) error {
	availableVersion, err := common.GetAvailableK8sClusterVersion(providerName, regionName)
	if err != nil {
		return err
	}

	valid := false
	versionIdList := []string{}
	for _, verDetail := range *availableVersion {
		if strings.EqualFold(verDetail.Id, version) {
			valid = true
			break
		} else {
			versionIdList = append(versionIdList, verDetail.Id)
		}
	}

	if valid {
		return nil
	} else {
		return fmt.Errorf("Available K8sCluster Version(k8sclusterinfo.yaml) for Provider/Region(%s/%s): %s",
			providerName, regionName, strings.Join(versionIdList, ", "))
	}
}
