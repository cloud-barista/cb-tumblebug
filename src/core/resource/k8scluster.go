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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// K8sClusterReqStructLevelValidation is a function to validate 'model.K8sClusterReq' object.
func K8sClusterReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.K8sClusterReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

var holdingK8sClusterMap sync.Map

// HandleK8sClusterAction is func to handle actions to K8sCluster
func HandleK8sClusterAction(nsId string, k8sClusterId string, action string) (string, error) {
	action = common.ToLower(action)

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	err = common.CheckString(k8sClusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	check, _ := CheckK8sCluster(nsId, k8sClusterId)

	if !check {
		err := fmt.Errorf("The K8sCluster " + k8sClusterId + " does not exist.")
		return err.Error(), err
	}

	log.Debug().Msgf("[Get K8sCluster requested action] %s", action)
	if action == "continue" {
		log.Debug().Msg("[continue K8sCluster provisioning]")
		key := common.GenK8sClusterKey(nsId, k8sClusterId)
		holdingK8sClusterMap.Store(key, action)

		return "Continue the holding K8sCluster", nil
	} else if action == "withdraw" {
		log.Debug().Msg("[withdraw K8sCluster provisioning]")
		key := common.GenK8sClusterKey(nsId, k8sClusterId)
		holdingK8sClusterMap.Store(key, action)

		return "Withdraw the holding K8sCluster", nil
	} else {
		return "", fmt.Errorf(action + " not supported")
	}
}

func createK8sClusterInfo(nsId string, tbK8sCInfo model.K8sClusterInfo) error {
	log.Debug().Msg("[Create K8sClusterInfo] " + tbK8sCInfo.Id)

	k8sClusterId := tbK8sCInfo.Id
	k := common.GenK8sClusterKey(nsId, k8sClusterId)
	_, exists, err := kvstore.GetKv(k)
	if err != nil {
		err := fmt.Errorf("failed to create K8sClusterInfo(%s): %v", k8sClusterId, err)
		return err
	}

	if exists {
		err := fmt.Errorf("failed to create K8sClusterInfo(%s): already exists", k8sClusterId)
		return err
	}

	val, err := json.Marshal(&tbK8sCInfo)
	if err != nil {
		err := fmt.Errorf("failed to create K8sClusterInfo(%s): %v", k8sClusterId, err)
		return err
	}

	err = kvstore.Put(k, string(val))
	if err != nil {
		err := fmt.Errorf("failed to create K8sClusterInfo(%s): %v", k8sClusterId, err)
		return err
	}

	return nil
}

func getK8sClusterInfo(nsId, k8sClusterId string) (*model.K8sClusterInfo, error) {
	//log.Debug().Msg("[Get K8sClusterInfo] " + k8sClusterId)

	emptyObj := &model.K8sClusterInfo{}

	k := common.GenK8sClusterKey(nsId, k8sClusterId)
	kv, exists, err := kvstore.GetKv(k)
	if err != nil {
		err := fmt.Errorf("failed to get K8sClusterInfo(%s): %v", k8sClusterId, err)
		return emptyObj, err
	}

	tbK8sCInfo := &model.K8sClusterInfo{}
	if !exists {
		err := fmt.Errorf("failed to get K8sClusterInfo(%s): empty keyvalue", k8sClusterId)
		return emptyObj, err
	}

	err = json.Unmarshal([]byte(kv.Value), tbK8sCInfo)
	if err != nil {
		err := fmt.Errorf("failed to get K8sClusterInfo(%s): %v", k8sClusterId, err)
		return emptyObj, err
	}

	return tbK8sCInfo, nil
}

// storeK8sClusterInfo is func to update K8sClusterInfo
func storeK8sClusterInfo(nsId string, newTbK8sCInfo *model.K8sClusterInfo) {
	k8sClusterId := newTbK8sCInfo.Id
	log.Debug().Msg("[Update K8sClusterInfo] " + k8sClusterId)

	k := common.GenK8sClusterKey(nsId, k8sClusterId)

	// Check existence of the key. If no key, no update.
	kv, exists, err := kvstore.GetKv(k)
	if !exists || err != nil {
		return
	}

	oldTbK8sCInfo := &model.K8sClusterInfo{}
	json.Unmarshal([]byte(kv.Value), oldTbK8sCInfo)

	if !reflect.DeepEqual(oldTbK8sCInfo, newTbK8sCInfo) {
		val, _ := json.Marshal(newTbK8sCInfo)
		err = kvstore.Put(k, string(val))
		if err != nil {
			err := fmt.Errorf("failed to update K8sClusterInfo(%s): %v", k8sClusterId, err)
			log.Err(err).Msgf("nsId=%s", nsId)
		}
	}
}

// deleteK8sClusterInfo is func to delete K8sClusterInfo
func deleteK8sClusterInfo(nsId, k8sClusterId string) error {
	log.Debug().Msg("[Delete K8sClusterInfo] " + k8sClusterId)

	k := common.GenK8sClusterKey(nsId, k8sClusterId)
	err := kvstore.Delete(k)
	if err != nil {
		err := fmt.Errorf("failed to delete K8sClusterInfo(%s): %v", k8sClusterId, err)
		return err
	}

	return nil
}

// CreateK8sCluster create a k8s cluster
func CreateK8sCluster(nsId string, req *model.K8sClusterReq, option string) (*model.K8sClusterInfo, error) {
	log.Info().Msg("CreateK8sCluster")

	emptyObj := &model.K8sClusterInfo{}

	k8sClusterId := req.Name

	err := validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	if check {
		err := fmt.Errorf("already exists", k8sClusterId)
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	/*
	 * Check for K8sCluster Enablement from K8sClusterSetting
	 */
	err = checkK8sClusterEnablement(req.ConnectionName)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	uid := common.GenUid()
	connConfig, err := common.GetConnConfig(req.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	tbK8sCInfo := &model.K8sClusterInfo{
		ResourceType:     model.StrK8s,
		Id:               k8sClusterId,
		Uid:              uid,
		Name:             k8sClusterId,
		ConnectionName:   req.ConnectionName,
		ConnectionConfig: connConfig,
		Description:      req.Description,
		Network: model.K8sClusterNetworkInfo{
			VNetId:           req.VNetId,
			SubnetIds:        req.SubnetIds,
			SecurityGroupIds: req.SecurityGroupIds,
		},
	}
	fillK8sNodeGroupInfoListFromK8sNodeGroupReqList(&tbK8sCInfo.K8sNodeGroupList, &req.K8sNodeGroupList)

	err = createK8sClusterInfo(nsId, *tbK8sCInfo)
	if err != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	var createErr error
	defer func() {
		if createErr != nil {
			log.Err(createErr).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)

			if tbK8sCInfo != nil {
				err := deleteK8sClusterInfo(nsId, k8sClusterId)
				if err != nil {
					log.Err(err).Msgf("")
				}
			}
		}
	}()

	// hold option will hold the K8sCluster creation process until the user releases it.
	if option == "hold" {
		key := common.GenK8sClusterKey(nsId, k8sClusterId)
		holdingK8sClusterMap.Store(key, "holding")
		for {
			value, ok := holdingK8sClusterMap.Load(key)
			if !ok {
				break
			}
			if value == "continue" {
				holdingK8sClusterMap.Delete(key)
				break
			} else if value == "withdraw" {
				holdingK8sClusterMap.Delete(key)
				DeleteK8sCluster(nsId, k8sClusterId, "force")
				createErr = fmt.Errorf("Withdrawed K8sCluster creation")
				log.Error().Err(err).Msg("")
				return nil, createErr
			}

			log.Info().Msgf("K8sCluster: %s (holding)", key)
			time.Sleep(5 * time.Second)
		}
		option = "create"
	}

	// Validate
	createErr = validateAtCreateK8sCluster(req)
	if createErr != nil {
		log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
		return emptyObj, createErr
	}

	// Build RequestBody for model.SpiderClusterReq{}
	spVersion := req.Version

	var spVPCName string
	spVPCName, createErr = GetCspResourceName(nsId, model.StrVNet, req.VNetId)
	if spVPCName == "" || createErr != nil {
		return emptyObj, createErr
	}

	var tmpInf interface{}
	tmpInf, createErr = GetResource(nsId, model.StrVNet, req.VNetId)
	if createErr != nil {
		return emptyObj, createErr
	}
	tbVNetInfo := model.VNetInfo{}
	createErr = common.CopySrcToDest(&tmpInf, &tbVNetInfo)
	if createErr != nil {
		return emptyObj, createErr
	}

	var spSnName string
	var spSubnetNames []string

	for _, v := range req.SubnetIds {
		found := false
		for _, w := range tbVNetInfo.SubnetInfoList {
			if v == w.Name {
				spSnName = w.CspResourceName
				found = true
				break
			}
		}

		if found == true {
			spSubnetNames = append(spSubnetNames, spSnName)

			var k8sRequiredSubnetCount int
			k8sRequiredSubnetCount, createErr = common.GetK8sRequiredSubnetCount(connConfig.ProviderName)
			if createErr != nil {
				return emptyObj, createErr
			}

			if k8sRequiredSubnetCount <= len(spSubnetNames) {
				break
			}
		}
	}
	if len(spSubnetNames) == 0 {
		createErr = fmt.Errorf("no valid subnets in vnet(%s)", req.VNetId)
		return emptyObj, createErr
	}

	var spSecurityGroupNames []string
	for _, v := range req.SecurityGroupIds {
		var spSgName string
		spSgName, createErr = GetCspResourceName(nsId, model.StrSecurityGroup, v)
		if spSgName == "" || createErr != nil {
			return emptyObj, createErr
		}

		spSecurityGroupNames = append(spSecurityGroupNames, spSgName)
	}

	var spNodeGroupList []model.SpiderNodeGroupReqInfo
	for _, v := range req.K8sNodeGroupList {
		spName := v.Name
		createErr = common.CheckString(spName)
		if createErr != nil {
			return emptyObj, createErr
		}

		spImgName := "" // Some CSPs do not require ImageName for creating a k8s cluster
		if v.ImageId == "" || v.ImageId == "default" {
			spImgName = ""
		} else {
			spImgName, err = GetCspResourceName(nsId, model.StrImage, v.ImageId)
			if spImgName == "" || createErr != nil {
				log.Warn().Msgf("Not found the Image %s in ns %s, find it from SystemCommonNs", v.ImageId, nsId)
				errAgg := err.Error()
				// If cannot find the resource, use common resource
				spImgName, err = GetCspResourceName(model.SystemCommonNs, model.StrImage, v.ImageId)
				if spImgName == "" || err != nil {
					errAgg += err.Error()
					createErr = fmt.Errorf(errAgg)
					log.Err(createErr).Msgf("Not found the Image %s both from ns %s and SystemCommonNs", v.ImageId, nsId)
					return emptyObj, createErr
				} else {
					log.Info().Msgf("Use the ImageId %s in SystemCommonNs", spImgName)
				}
			} else {
				log.Info().Msgf("Use the Image %s in ns %s", spImgName, nsId)
			}
		}

		spSpecName := ""
		spSpecName, err = GetCspResourceName(nsId, model.StrSpec, v.SpecId)
		if spSpecName == "" || err != nil {
			log.Warn().Msgf("Not found the Spec %s in ns %s, find it from SystemCommonNs", v.SpecId, nsId)
			errAgg := err.Error()
			// If cannot find resource, use common resource
			spSpecName, err = GetCspResourceName(model.SystemCommonNs, model.StrSpec, v.SpecId)
			if spSpecName == "" || err != nil {
				errAgg += err.Error()
				createErr = fmt.Errorf(errAgg)
				log.Err(createErr).Msgf("Not found the Spec %s both from ns %s and SystemCommonNs", v.SpecId, nsId)
				return emptyObj, createErr
			} else {
				log.Info().Msgf("Use the SpecId %s in SystemCommonNs", spSpecName)
			}
		} else {
			log.Info().Msgf("Use the Spec %s in ns %s", spSpecName, nsId)
		}

		var spKpName string
		spKpName, createErr = GetCspResourceName(nsId, model.StrSSHKey, v.SshKeyId)
		if spKpName == "" || createErr != nil {
			return emptyObj, createErr
		}

		spNodeGroupList = append(spNodeGroupList, model.SpiderNodeGroupReqInfo{
			Name:            spName,
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

	requestBody := model.SpiderClusterReq{
		ConnectionName: req.ConnectionName,
		ReqInfo: model.SpiderClusterReqInfo{
			Name:               uid,
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

	createErr = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		clientManager.MediumDuration,
	)

	if createErr != nil {
		return emptyObj, createErr
	}

	// Update model.K8sClusterInfo object
	updateK8sClusterInfoFromSpiderClusterInfo(tbK8sCInfo, &spClusterRes.SpiderClusterInfo)
	tbK8sCInfo.SpiderViewK8sClusterDetail = spClusterRes.SpiderClusterInfo

	if option == "register" && req.CspResourceId == "" {
		tbK8sCInfo.SystemLabel = "Registered from CB-Spider resource"
		// TODO: check to handle something to register
	} else if option == "register" && req.CspResourceId != "" {
		tbK8sCInfo.SystemLabel = "Registered from CSP resource"
	}

	storeK8sClusterInfo(nsId, tbK8sCInfo)

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrK8s,
		model.LabelId:              tbK8sCInfo.Id,
		model.LabelName:            tbK8sCInfo.Name,
		model.LabelUid:             tbK8sCInfo.Uid,
		model.LabelVersion:         tbK8sCInfo.Version,
		model.LabelCspResourceId:   tbK8sCInfo.CspResourceId,
		model.LabelCspResourceName: tbK8sCInfo.CspResourceName,
		model.LabelDescription:     tbK8sCInfo.Description,
		model.LabelCreatedTime:     tbK8sCInfo.CreatedTime.String(),
		model.LabelConnectionName:  tbK8sCInfo.ConnectionName,
	}
	k8sClusterKey := common.GenK8sClusterKey(nsId, k8sClusterId)
	createErr = label.CreateOrUpdateLabel(model.StrK8s, uid, k8sClusterKey, labels)
	if createErr != nil {
		return emptyObj, createErr
	}

	return tbK8sCInfo, nil
}

/*
// CheckK8sNodeGroup returns the existence of the K8sNodeGroup in K8sCluster object in bool form.
func CheckK8sNodeGroup(nsId string, k8sClusterId string, k8sNodeGroupName string) (bool, error) {

	check, err := resource.CheckK8sCluster(nsId, k8sClusterId)
  if err != nil {
		log.Err(err).Msg("Failed to Check K8sNodeGroup")
		return false, err
  }

	err = common.CheckString(k8sNodeGroupName)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sNodeGroup")
		return false, err
	}

  log.Debug().Msgf("[Check K8sNodeGroup] %s:%s", k8sClusterId, k8sNodeGroupName)

	key := common.GenK8sClusterKey(nsId, k8sClusterId)

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sNodeGroup")
		return false, err
	}
	if keyValue != (kvstore.KeyValue{}) {
		return true, nil
	}
	return false, nil
}
*/

// AddK8sNodeGroup adds a K8sNodeGroup
func AddK8sNodeGroup(nsId string, k8sClusterId string, u *model.K8sNodeGroupReq) (*model.K8sClusterInfo, error) {
	log.Info().Msg("AddK8sNodeGroup")

	emptyObj := &model.K8sClusterInfo{}

	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Check for K8sCluster Enablement from ClusterSetting
	err = checkK8sClusterEnablement(tbK8sCInfo.ConnectionName)
	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Build RequestBody for SpiderNodeGroupReq{}
	spName := u.Name
	err = common.CheckString(spName)
	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	spImgName := "" // Some CSPs do not require ImageName for creating a k8s cluster
	if u.ImageId == "" || u.ImageId == "default" {
		spImgName = ""
	} else {
		spImgName, err = GetCspResourceName(nsId, model.StrImage, u.ImageId)
		if spImgName == "" || err != nil {
			log.Warn().Msgf("Not found the Image %s in ns %s, find it from SystemCommonNs", u.ImageId, nsId)
			errAgg := err.Error()
			// If cannot find the resource, use common resource
			spImgName, err = GetCspResourceName(model.SystemCommonNs, model.StrImage, u.ImageId)
			if spImgName == "" || err != nil {
				errAgg += err.Error()
				err = fmt.Errorf(errAgg)
				log.Err(err).Msgf("Not found the Image %s both from ns %s and SystemCommonNs", u.ImageId, nsId)
				log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
				return emptyObj, err
			} else {
				log.Info().Msgf("Use the ImageId %s in SystemCommonNs", spImgName)
			}
		} else {
			log.Info().Msgf("Use the Image %s in ns %s", spImgName, nsId)
		}
	}

	spSpecName := ""
	spSpecName, err = GetCspResourceName(nsId, model.StrSpec, u.SpecId)
	if spSpecName == "" || err != nil {
		log.Warn().Msgf("Not found the Spec %s in ns %s, find it from SystemCommonNs", u.SpecId, nsId)
		errAgg := err.Error()
		// If cannot find resource, use common resource
		spSpecName, err = GetCspResourceName(model.SystemCommonNs, model.StrSpec, u.SpecId)
		if spSpecName == "" || err != nil {
			errAgg += err.Error()
			err = fmt.Errorf(errAgg)
			log.Err(err).Msgf("Not found the Spec %s both from ns %s and SystemCommonNs", u.SpecId, nsId)
			log.Err(err).Msgf("Failed to Create a K8sCluster(%s)", k8sClusterId)
			return emptyObj, err
		} else {
			log.Info().Msgf("Use the SpecId %s in SystemCommonNs", spSpecName)
		}
	} else {
		log.Info().Msgf("Use the Spec %s in ns %s", spSpecName, nsId)
	}

	spKpName, err := GetCspResourceName(nsId, model.StrSSHKey, u.SshKeyId)
	if spKpName == "" {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update K8sClusterInfo.K8NodeGroupList - Add new nodegroup to existing list
	var tbK8sNGReqList []model.K8sNodeGroupReq
	tbK8sNGReqList = append(tbK8sNGReqList, *u)

	// Create new nodegroup info and append to existing list
	var newK8sNodeGroupInfoList []model.K8sNodeGroupInfo
	newK8sNodeGroupInfoList = append(newK8sNodeGroupInfoList, tbK8sCInfo.K8sNodeGroupList...)

	// Add the new nodegroup
	tbK8sNGInfo := model.K8sNodeGroupInfo{}
	fillK8sNodeGroupInfoFromK8sNodeGroupReq(&tbK8sNGInfo, u)
	newK8sNodeGroupInfoList = append(newK8sNodeGroupInfoList, tbK8sNGInfo)

	// Update the cluster's nodegroup list
	tbK8sCInfo.K8sNodeGroupList = newK8sNodeGroupInfoList

	requestBody := model.SpiderNodeGroupReq{
		ConnectionName: tbK8sCInfo.ConnectionName,
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

	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName + "/nodegroup"

	var spClusterRes model.SpiderClusterRes

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update/Get model.K8sClusterInfo object to/from kvstore
	updateK8sClusterInfoFromSpiderClusterInfo(tbK8sCInfo, &spClusterRes.SpiderClusterInfo)
	tbK8sCInfo.SpiderViewK8sClusterDetail = spClusterRes.SpiderClusterInfo
	storeK8sClusterInfo(nsId, tbK8sCInfo)

	storedTbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Add K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	return storedTbK8sCInfo, nil
}

// RemoveK8sNodeGroup removes a specified NodeGroup
func RemoveK8sNodeGroup(nsId, k8sClusterId, k8sNodeGroupName, option string) (bool, error) {
	log.Info().Msg("RemoveK8sNodeGroup")

	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msgf("Failed to Remove K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return false, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Remove K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return false, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Remove K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return false, err
	}

	// Create Request body for RemoveK8sNodeGroup of CB-Spider
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{
		ConnectionName: tbK8sCInfo.ConnectionName,
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName + "/nodegroup/" + k8sNodeGroupName
	if option == "force" {
		url += "?force=true"
	}
	method := "DELETE"
	client.SetTimeout(10 * time.Minute)

	var ifRes interface{}
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Remove K8sNodeGroup(k8scluster=%s)", k8sClusterId)
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				// Successfully removed from CSP, now update local cluster info
				err = removeNodeGroupFromLocalClusterInfo(nsId, k8sClusterId, k8sNodeGroupName)
				if err != nil {
					log.Warn().Err(err).Msgf("NodeGroup removed from CSP but failed to update local cluster info")
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// SetK8sNodeGroupAutoscaling set NodeGroup's Autoscaling On/Off
func SetK8sNodeGroupAutoscaling(nsId string, k8sClusterId string, k8sNodeGroupName string, u *model.SetK8sNodeGroupAutoscalingReq) (*model.SetK8sNodeGroupAutoscalingRes, error) {
	log.Info().Msg("SetK8sNodeGroupAutoscaling")

	emptyObj := &model.SetK8sNodeGroupAutoscalingRes{}

	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	err = common.CheckString(k8sNodeGroupName)
	if err != nil {
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		log.Err(err).Msg("")
		return emptyObj, err
	}

	// Find the specific nodegroup
	var tbK8sNGInfo *model.K8sNodeGroupInfo = nil
	for i := range tbK8sCInfo.K8sNodeGroupList {
		if tbK8sCInfo.K8sNodeGroupList[i].Name == k8sNodeGroupName {
			tbK8sNGInfo = &tbK8sCInfo.K8sNodeGroupList[i]
			break
		}
	}
	if tbK8sNGInfo == nil {
		err = fmt.Errorf("failed to find the K8sNodeGroup(%s)", k8sNodeGroupName)
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Create Request body for SetAutoScaling of CB-Spider
	requestBody := model.SpiderSetAutoscalingReq{
		ConnectionName: tbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderSetAutoscalingReqInfo{
			OnAutoScaling: u.OnAutoScaling,
		},
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName + "/nodegroup/" + k8sNodeGroupName + "/onautoscaling"
	method := "PUT"

	var spSetAutoscalingRes model.SpiderSetAutoscalingRes
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spSetAutoscalingRes,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Set K8sNodeGroup Autoscaling(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	tbK8sSetAutoscalingRes := &model.SetK8sNodeGroupAutoscalingRes{
		Result: spSetAutoscalingRes.Result,
	}

	// if request is applied, update tbK8sNGInfo.OnAutoScaling
	bResult, _ := strconv.ParseBool(spSetAutoscalingRes.Result)
	if bResult == true {
		tbK8sNGInfo.OnAutoScaling, _ = strconv.ParseBool(u.OnAutoScaling)
		storeK8sClusterInfo(nsId, tbK8sCInfo)
	}

	return tbK8sSetAutoscalingRes, nil
}

// ChangeK8sNodeGroupAutoscaleSize change NodeGroup's Autoscaling Size
func ChangeK8sNodeGroupAutoscaleSize(nsId string, k8sClusterId string, k8sNodeGroupName string, u *model.ChangeK8sNodeGroupAutoscaleSizeReq) (*model.ChangeK8sNodeGroupAutoscaleSizeRes, error) {
	log.Info().Msg("ChangeK8sNodeGroupAutoscaleSize")

	emptyObj := &model.ChangeK8sNodeGroupAutoscaleSizeRes{}

	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	err = common.CheckString(k8sNodeGroupName)
	if err != nil {
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	// Find the specific nodegroup
	var tbK8sNGInfo *model.K8sNodeGroupInfo = nil
	for i := range tbK8sCInfo.K8sNodeGroupList {
		if tbK8sCInfo.K8sNodeGroupList[i].Name == k8sNodeGroupName {
			tbK8sNGInfo = &tbK8sCInfo.K8sNodeGroupList[i]
			break
		}
	}
	if tbK8sNGInfo == nil {
		err = fmt.Errorf("failed to find the K8sNodeGroup(%s)", k8sNodeGroupName)
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
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
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName + "/nodegroup/" + k8sNodeGroupName + "/autoscalesize"
	method := "PUT"

	var spChangeAutoscaleSizeRes model.SpiderChangeAutoscaleSizeRes
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spChangeAutoscaleSizeRes,
		clientManager.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Change K8sNodeGroup AutoscaleSize(k8scluster=%s)", k8sClusterId)
		return emptyObj, err
	}

	updateK8sNodeGroupInfoFromSpiderNodeGroupInfo(tbK8sNGInfo, &spChangeAutoscaleSizeRes.SpiderNodeGroupInfo)
	storeK8sClusterInfo(nsId, tbK8sCInfo)

	tbK8sCAutoscaleSizeRes := &model.ChangeK8sNodeGroupAutoscaleSizeRes{
		K8sNodeGroupInfo: *tbK8sNGInfo,
	}

	return tbK8sCAutoscaleSizeRes, nil
}

// GetK8sCluster retrives a k8s cluster information
func GetK8sCluster(nsId string, k8sClusterId string) (*model.K8sClusterInfo, error) {
	//log.Debug().Msg("[Get K8sCluster] " + k8sClusterId)

	emptyObj := &model.K8sClusterInfo{}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update model.K8sClusterInfo from CB-Spider
	client := resty.New()
	client.SetTimeout(10 * time.Minute)
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName
	method := "GET"

	// Create Request body for GetK8sCluster of CB-Spider
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{
		ConnectionName: tbK8sCInfo.ConnectionName,
	}

	var spClusterRes model.SpiderClusterRes
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update/Get model.K8sClusterInfo object to/from kvstore
	updateK8sClusterInfoFromSpiderClusterInfo(tbK8sCInfo, &spClusterRes.SpiderClusterInfo)
	tbK8sCInfo.SpiderViewK8sClusterDetail = spClusterRes.SpiderClusterInfo
	storeK8sClusterInfo(nsId, tbK8sCInfo)

	storedTbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// add label info
	labelInfo, err := label.GetLabels(model.StrK8s, storedTbK8sCInfo.Uid)
	if err != nil {
		log.Err(err).Msgf("Failed to Get K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}
	storedTbK8sCInfo.Label = labelInfo.Labels

	return storedTbK8sCInfo, nil
}

// CheckK8sCluster returns the existence of the TB K8sCluster object in bool form.
func CheckK8sCluster(nsId string, k8sClusterId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("nsId given is empty.")
		return false, err
	} else if k8sClusterId == "" {
		err := fmt.Errorf("k8sClusterId given is empty.")
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

	//log.Debug().Msg("[Check K8sCluster] " + k8sClusterId)

	key := common.GenK8sClusterKey(nsId, k8sClusterId)

	_, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Err(err).Msg("Failed to Check K8sCluster")
		return false, err
	}
	if exists {
		return true, nil
	}
	return false, nil
}

// ListK8sClusterId returns the list of TB K8sCluster object IDs of given nsId
func ListK8sClusterId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	k := fmt.Sprintf("/ns/%s/", nsId)

	kv, err := kvstore.GetKvList(k)

	if err != nil {
		log.Error().Err(err).Msg("Failed to Get K8sClusterId List")
		return nil, err
	}

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
	// log.Info().Msg("ListK8sCluster")

	k8sIdList, err := ListK8sClusterId(nsId)
	if err != nil {
		log.Err(err).Msg("Failed to List K8sCluster")
		return nil, err
	}

	tbK8sCInfoList := []model.K8sClusterInfo{}

	for _, id := range k8sIdList {
		k := common.GenK8sClusterKey(nsId, id)
		kv, exists, err := kvstore.GetKv(k)
		if err != nil {
			log.Error().Err(err).Msg("")
		}

		if !exists {
			err = fmt.Errorf("%s cannot be found", k)
			log.Err(err).Msg("Failed to List K8sCluster")
			return nil, err
		}

		storedTbK8sCInfo := model.K8sClusterInfo{}
		err = json.Unmarshal([]byte(kv.Value), &storedTbK8sCInfo)
		if err != nil {
			log.Err(err).Msg("Failed to List K8sCluster")
			return nil, err
		}
		// Check the JSON body includes both filterKey and filterVal strings. (assume key and value)
		if filterKey != "" {
			// If not includes both, do not append current item to the list result.
			itemValueForCompare := strings.ToLower(kv.Value)
			if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) &&
				strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
				continue
			}
		}

		tbK8sCInfo, err := GetK8sCluster(nsId, storedTbK8sCInfo.Id)
		if err != nil {
			// If circuit breaker is active or too many requests error, use stored info with warning
			if strings.Contains(err.Error(), "API call temporarily blocked") || strings.Contains(err.Error(), "circuit breaker") || strings.Contains(err.Error(), "too many same requests") {
				log.Warn().Err(err).Msgf("Using stored cluster info due to API limitation for K8sCluster(%s)", storedTbK8sCInfo.Id)
				// Add label info to stored cluster info
				if labelInfo, labelErr := label.GetLabels(model.StrK8s, storedTbK8sCInfo.Uid); labelErr == nil {
					storedTbK8sCInfo.Label = labelInfo.Labels
				}
				tbK8sCInfoList = append(tbK8sCInfoList, storedTbK8sCInfo)
			} else {
				log.Err(err).Msgf("Failed to get K8sCluster(%s) in list", storedTbK8sCInfo.Id)
			}
			continue
		}

		tbK8sCInfoList = append(tbK8sCInfoList, *tbK8sCInfo)
	}

	return tbK8sCInfoList, nil
}

// DeleteK8sCluster deletes a k8s cluster
func DeleteK8sCluster(nsId, k8sClusterId, option string) (bool, error) {
	log.Info().Msg("DeleteK8sCluster")

	check, err := CheckK8sCluster(nsId, k8sClusterId)

	if err != nil {
		log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
		return false, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
		return false, err
	}

	// Get model.K8sClusterInfo object from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
		return false, err
	}

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	requestBody := JsonTemplate{
		ConnectionName: tbK8sCInfo.ConnectionName,
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName
	if option == "force" {
		url += "?force=true"
	}
	method := "DELETE"
	client.SetTimeout(20 * time.Minute)

	var ifRes interface{}
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		clientManager.VeryShortDuration,
	)

	log.Debug().Msgf("option=%s", option)
	if option == "force" {
		err = deleteK8sClusterInfo(nsId, k8sClusterId)
		if err != nil {
			log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
			return false, err
		}
	}

	if err != nil {
		log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				if option != "force" {
					err = deleteK8sClusterInfo(nsId, k8sClusterId)
					if err != nil {
						log.Err(err).Msgf("Failed to Delete K8sCluster(%s)", k8sClusterId)
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
func DeleteAllK8sCluster(nsId, subString, option string) (*model.IdList, error) {
	log.Info().Msg("DeleteAllK8sCluster")

	deletedK8sClusters := &model.IdList{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Err(err).Msgf("Failed to Delete All K8sCluster")
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

			res, err := DeleteK8sCluster(nsId, v, option)

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
func UpgradeK8sCluster(nsId string, k8sClusterId string, u *model.UpgradeK8sClusterReq) (*model.K8sClusterInfo, error) {
	log.Info().Msg("UpgradeK8sCluster")

	emptyObj := &model.K8sClusterInfo{}

	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("not exist")
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Get model.K8sClusterInfo from kvstore
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Check for K8sCluster Enablement from K8sClusterSetting
	err = checkK8sClusterEnablement(tbK8sCInfo.ConnectionName)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Validate
	err = validateAtUpgradeK8sCluster(tbK8sCInfo.ConnectionName, u)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Build RequestBody for model.SpiderUpgradeClusterReq{}
	spVersion := u.Version
	requestBody := model.SpiderUpgradeClusterReq{
		ConnectionName: tbK8sCInfo.ConnectionName,
		ReqInfo: model.SpiderUpgradeClusterReqInfo{
			Version: spVersion,
		},
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/cluster/" + tbK8sCInfo.CspResourceName + "/upgrade"
	method := "PUT"
	client.SetTimeout(10 * time.Minute)

	var spClusterRes model.SpiderClusterRes
	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update/Get model.K8sClusterInfo object to/from kvstore
	updateK8sClusterInfoFromSpiderClusterInfo(tbK8sCInfo, &spClusterRes.SpiderClusterInfo)
	tbK8sCInfo.SpiderViewK8sClusterDetail = spClusterRes.SpiderClusterInfo
	storeK8sClusterInfo(nsId, tbK8sCInfo)

	storedTbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	// Update label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelVersion:     tbK8sCInfo.Version,
		model.LabelCreatedTime: tbK8sCInfo.CreatedTime.String(),
	}
	k8sClusterKey := common.GenK8sClusterKey(nsId, k8sClusterId)
	err = label.CreateOrUpdateLabel(model.StrK8s, storedTbK8sCInfo.Uid, k8sClusterKey, labels)
	if err != nil {
		log.Err(err).Msgf("Failed to Upgrade a K8sCluster(%s)", k8sClusterId)
		return emptyObj, err
	}

	return storedTbK8sCInfo, nil
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

func validateAtCreateK8sCluster(tbK8sClusterReq *model.K8sClusterReq) error {
	connConfig, err := common.GetConnConfig(tbK8sClusterReq.ConnectionName)

	// Validate K8sCluster Version
	err = validateK8sVersion(connConfig.ProviderName, connConfig.RegionDetail.RegionName, tbK8sClusterReq.Version)
	if err != nil {
		log.Err(err).Msgf("Requested K8sVersion(%s)", tbK8sClusterReq.Version)
		return err
	}

	// Validate K8sNodeGroups On K8s Creation
	k8sNgOnCreation, err := common.GetK8sNodeGroupsOnK8sCreation(connConfig.ProviderName)
	if err != nil {
		log.Err(err).Msgf("Failed to Get Nodegroups on K8sCluster Creation")
		return err
	}

	if k8sNgOnCreation {
		if len(tbK8sClusterReq.K8sNodeGroupList) <= 0 {
			err := fmt.Errorf("Need to Set One more K8sNodeGroupList")
			log.Err(err).Msgf("Provider(%s)", connConfig.ProviderName)
			return err
		}
	} else {
		if len(tbK8sClusterReq.K8sNodeGroupList) > 0 {
			err := fmt.Errorf("Need to Set Empty K8sNodeGroupList")
			log.Err(err).Msgf("Provider(%s)", connConfig.ProviderName)
			return err
		}
	}

	// Validate K8sNodeGroup's Naming Rule
	k8sNgNamingRule, err := common.GetK8sNodeGroupNamingRule(connConfig.ProviderName)
	if err != nil {
		log.Err(err).Msgf("Failed to Get Nodegroup's Naming Rule")
		return err
	}

	if len(tbK8sClusterReq.K8sNodeGroupList) > 0 {
		re := regexp.MustCompile(k8sNgNamingRule)
		for _, ng := range tbK8sClusterReq.K8sNodeGroupList {
			if re.MatchString(ng.Name) == false {
				err := fmt.Errorf("K8sNodeGroup's Name(%s) should be match regular expression(%s)", ng.Name, k8sNgNamingRule)
				log.Err(err).Msgf("Provider(%s)", connConfig.ProviderName)
				return err
			}
		}
	}

	return nil
}

func validateAtUpgradeK8sCluster(connectionName string, tbUpgradeK8sClusterReq *model.UpgradeK8sClusterReq) error {
	connConfig, err := common.GetConnConfig(connectionName)

	// Validate K8sCluster Version
	err = validateK8sVersion(connConfig.ProviderName, connConfig.RegionDetail.RegionName, tbUpgradeK8sClusterReq.Version)
	if err != nil {
		log.Err(err).Msgf("Requested K8sVersion(%s)", tbUpgradeK8sClusterReq.Version)
		return err
	}

	return nil
}

func validateK8sVersion(providerName, regionName, version string) error {
	availableVersion, err := common.GetAvailableK8sVersion(providerName, regionName)
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

	if valid == false {
		return fmt.Errorf("Available K8sCluster Version(k8sclusterinfo.yaml) for Provider/Region(%s/%s): %s",
			providerName, regionName, strings.Join(versionIdList, ", "))
	}

	return nil
}

// RemoteCommandToK8sClusterContainer is func to command to specified Container in K8sCluster by Kubernetes API
func RemoteCommandToK8sClusterContainer(nsId string, k8sClusterId string, k8sClusterNamespace string, k8sClusterPodName string, k8sClusterContainerName string, req *model.K8sClusterContainerCmdReq) (*model.K8sClusterContainerCmdResults, error) {
	log.Info().Msg("RemoteCommandToK8sClusterContainer")

	emptyObj := &model.K8sClusterContainerCmdResults{}

	err := validate.Struct(req)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msgf("Failed to Run Remote Command to K8sCluster(%s)'s Pod(%s)", k8sClusterId, k8sClusterPodName)
			return emptyObj, err
		}

		return emptyObj, err
	}

	if len(req.Command) <= 0 {
		err := fmt.Errorf("empty commands")
		log.Err(err).Msgf("Failed to Run Remote Command to K8sCluster(%s)'s Pod(%s)", k8sClusterId, k8sClusterPodName)
		return emptyObj, err
	}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Run Remote Command to K8sCluster(%s)'s Pod(%s)", k8sClusterId, k8sClusterPodName)
		return emptyObj, err
	}

	if !check {
		err = fmt.Errorf("The K8sCluster(%s) does not exist", k8sClusterId)
		return emptyObj, err
	}

	// Execute commands
	results, err := runRemoteCommandToK8sClusterContainer(nsId, k8sClusterId, k8sClusterPodName, k8sClusterNamespace, k8sClusterContainerName, req.Command)
	return results, nil
}

// TransferFileToK8sClusterContainer is func to transfer a file to specified Container in K8sCluster by Kubernetes API
func TransferFileToK8sClusterContainer(nsId string, k8sClusterId string, k8sClusterNamespace string, k8sClusterPodName string, k8sClusterContainerName string, fileData []byte, fileName, targetPath string) (*model.K8sClusterContainerCmdResult, error) {
	log.Info().Msg("TransferFileToK8sClusterContainer")

	emptyObj := &model.K8sClusterContainerCmdResult{}

	check, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("Failed to Run Remote Command to K8sCluster(%s)'s Pod(%s)", k8sClusterId, k8sClusterPodName)
		return emptyObj, err
	}

	if !check {
		err = fmt.Errorf("The K8sCluster(%s) does not exist", k8sClusterId)
		return emptyObj, err
	}

	// Execute commands
	return transferFileToK8sClusterContainer(nsId, k8sClusterId, k8sClusterPodName, k8sClusterNamespace, k8sClusterContainerName, fileData, fileName, targetPath)
}

func getKubeconfigFromK8sClusterInfo(nsId, k8sClusterId string) (string, error) {
	log.Debug().Msg("[Get Kubeconfig from K8sClusterInfo] " + k8sClusterId)

	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		err = fmt.Errorf("failed to get kubeconfig from K8sClusterInfo(%s): %v", k8sClusterId, err)
		return "", err
	}

	if tbK8sCInfo.Status != model.K8sClusterActive {
		// Check K8sCluster's Status again
		newTbK8sCInfo, err := GetK8sCluster(nsId, k8sClusterId)
		if err != nil {
			err = fmt.Errorf("failed to get kubeconfig from K8sClusterInfo(%s): %v", k8sClusterId, err)
			return "", err
		}

		if newTbK8sCInfo.Status != model.K8sClusterActive {
			err = fmt.Errorf("failed to get kubeconfig from K8sClusterInfo(%s): K8sCluster is not active", k8sClusterId)
			return "", err
		}
	}

	return tbK8sCInfo.AccessInfo.Kubeconfig, nil
}

func runRemoteCommandToK8sClusterContainer(nsId, k8sClusterId, k8sClusterPodName, k8sClusterNamespace, k8sClusterContainerName string, commands []string) (*model.K8sClusterContainerCmdResults, error) {
	log.Debug().Msgf("[Run Remote Command To K8sCluster's Conatiner] %s, %s, %s, %s", k8sClusterId, k8sClusterNamespace, k8sClusterPodName, k8sClusterContainerName)

	results := &model.K8sClusterContainerCmdResults{}

	// Check whether K8sCluster is active
	kubeconfig, err := getKubeconfigFromK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return results, err
	}

	// Access K8sCluster via kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return results, err
	}

	cset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return results, err
	}

	for _, cmd := range commands {
		// Split the command string into individual arguments
		cmdArgs := strings.Fields(cmd)

		podExecOptions := &corev1.PodExecOptions{
			Container: k8sClusterContainerName,
			Command:   cmdArgs,
			Stdout:    true,
			Stderr:    true,
		}

		req := cset.CoreV1().RESTClient().
			Post().
			Namespace(k8sClusterNamespace).
			Resource("pods").
			Name(k8sClusterPodName).
			SubResource("exec").
			VersionedParams(podExecOptions, scheme.ParameterCodec)

		var stdout, stderr bytes.Buffer

		executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
		if err != nil {
			log.Err(err).Msgf("failed to run some remote command(%s) to K8sCluster(%s)'s Container(%s)", cmd, k8sClusterId, k8sClusterContainerName)
		} else {
			err = executor.Stream(remotecommand.StreamOptions{
				Stdout: &stdout,
				Stderr: &stderr,
				Tty:    false,
			})
			if err != nil {
				log.Err(err).Msgf("failed to run some remote command(%s) to K8sCluster(%s)'s Container(%s)", cmd, k8sClusterId, k8sClusterContainerName)
				return results, err
			}
		}

		results.Results = append(results.Results, &model.K8sClusterContainerCmdResult{
			Command: cmd,
			Stdout:  stdout.String(),
			Stderr:  stderr.String(),
			Err:     nil,
		})
	}

	return results, nil
}

func transferFileToK8sClusterContainer(nsId, k8sClusterId, k8sClusterPodName, k8sClusterNamespace, k8sClusterContainerName string, fileData []byte, fileName, targetPath string) (*model.K8sClusterContainerCmdResult, error) {
	log.Debug().Msgf("[Transfer a File To K8sCluster's Conatiner] %s, %s, %s, %s, %s, %s", k8sClusterId, k8sClusterNamespace, k8sClusterPodName, k8sClusterContainerName, fileName, targetPath)

	result := &model.K8sClusterContainerCmdResult{}

	// Check whether K8sCluster is active
	kubeconfig, err := getKubeconfigFromK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return result, err
	}

	// Access K8sCluster via kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return result, err
	}

	cset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Err(err).Msgf("failed to run remote commands To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return result, err
	}

	// Create in-memory tar stream from []byte
	var bufFile bytes.Buffer
	tw := tar.NewWriter(&bufFile)
	hdr := &tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: int64(len(fileData)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		log.Err(err).Msgf("failed to transfer a file To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return result, err
	}
	if _, err := tw.Write(fileData); err != nil {
		log.Err(err).Msgf("failed to transfer a file To K8sCluster(%s)'s container(%s)", k8sClusterId, k8sClusterContainerName)
		return result, err
	}
	tw.Close()

	// Extract tar from stdin
	cmd := []string{"tar", "xf", "-", "-C", path.Clean(targetPath)}

	podExecOptions := &corev1.PodExecOptions{
		Container: k8sClusterContainerName,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
	}

	req := cset.CoreV1().RESTClient().
		Post().
		Namespace(k8sClusterNamespace).
		Resource("pods").
		Name(k8sClusterPodName).
		SubResource("exec").
		VersionedParams(podExecOptions, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		log.Err(err).Msgf("failed to run some remote command(%s) to K8sCluster(%s)'s Container(%s)", cmd, k8sClusterId, k8sClusterContainerName)
	} else {
		err = executor.Stream(remotecommand.StreamOptions{
			Stdin:  &bufFile,
			Stdout: &stdout,
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			log.Err(err).Msgf("failed to run some remote command(%s) to K8sCluster(%s)'s Container(%s)", cmd, k8sClusterId, k8sClusterContainerName)
			return result, err
		}
	}

	result.Command = strings.Join(cmd, " ")
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.Err = nil

	return result, nil
}

func updateK8sClusterNetworkInfoFromSpiderNetworkInfo(tbK8sCNInfo *model.K8sClusterNetworkInfo, spNetworkInfo *model.SpiderNetworkInfo) {
	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spNetworkInfo.KeyValueList)

	tbK8sCNInfo.KeyValueList = tbKeyValueList
}

func updateK8sNodeGroupInfoFromSpiderNodeGroupInfo(tbK8sNGInfo *model.K8sNodeGroupInfo, spNGInfo *model.SpiderNodeGroupInfo) {
	// Update basic identification info from Spider response
	tbK8sNGInfo.Id = spNGInfo.IId.NameId
	tbK8sNGInfo.Name = spNGInfo.IId.NameId

	// Keep existing ImageId and SpecId if they are set, otherwise try to get from Spider
	if tbK8sNGInfo.ImageId == "" {
		tbK8sNGInfo.ImageId = spNGInfo.ImageIID.SystemId
	}
	if tbK8sNGInfo.SpecId == "" {
		tbK8sNGInfo.SpecId = spNGInfo.VMSpecName
	}
	if tbK8sNGInfo.SshKeyId == "" {
		tbK8sNGInfo.SshKeyId = spNGInfo.KeyPairIID.NameId
	}

	tbK8sNGInfo.RootDiskType = spNGInfo.RootDiskType
	tbK8sNGInfo.RootDiskSize = spNGInfo.RootDiskSize
	tbK8sNGInfo.OnAutoScaling = spNGInfo.OnAutoScaling
	tbK8sNGInfo.DesiredNodeSize = spNGInfo.DesiredNodeSize
	tbK8sNGInfo.MinNodeSize = spNGInfo.MinNodeSize
	tbK8sNGInfo.MaxNodeSize = spNGInfo.MaxNodeSize
	tbK8sNGInfo.Status = convertSpiderNodeGroupStatusToK8sNodeGroupStatus(spNGInfo.Status)

	tbK8sNGInfo.K8sNodes = []model.K8sNodeInfo{}
	for _, v := range spNGInfo.Nodes {
		tbK8sNGInfo.K8sNodes = append(tbK8sNGInfo.K8sNodes, model.K8sNodeInfo{
			CspResourceName: v.NameId,
			CspResourceId:   v.SystemId,
		})
	}

	tbK8sNGInfo.KeyValueList = convertSpiderKeyValueListToTbKeyValueList(spNGInfo.KeyValueList)

	tbK8sNGInfo.CspResourceName = spNGInfo.IId.NameId
	tbK8sNGInfo.CspResourceId = spNGInfo.IId.SystemId

	tbK8sNGInfo.SpiderViewK8sNodeGroupDetail = *spNGInfo
}

func updateK8sAccessInfoFromSpiderAccessInfo(tbK8sAccInfo *model.K8sAccessInfo, spAccInfo *model.SpiderAccessInfo) {
	tbK8sAccInfo.Endpoint = spAccInfo.Endpoint
	tbK8sAccInfo.Kubeconfig = spAccInfo.Kubeconfig
}

func updateK8sAddonsInfoFromSpiderAddonsInfo(tbK8sAddInfo *model.K8sAddonsInfo, spAddInfo *model.SpiderAddonsInfo) {
	tbK8sAddInfo.KeyValueList = convertSpiderKeyValueListToTbKeyValueList(spAddInfo.KeyValueList)
}

// removeNodeGroupFromLocalClusterInfo removes a node group from local cluster info after successful CSP deletion
func removeNodeGroupFromLocalClusterInfo(nsId, k8sClusterId, k8sNodeGroupName string) error {
	// Get current cluster info
	tbK8sCInfo, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		return fmt.Errorf("failed to get cluster info: %w", err)
	}

	// Find and remove the node group
	var updatedNodeGroups []model.K8sNodeGroupInfo
	found := false
	for _, ng := range tbK8sCInfo.K8sNodeGroupList {
		if ng.Name != k8sNodeGroupName {
			updatedNodeGroups = append(updatedNodeGroups, ng)
		} else {
			found = true
		}
	}

	if !found {
		log.Debug().Msgf("NodeGroup %s not found in local cluster info, might be already removed", k8sNodeGroupName)
		return nil
	}

	// Update the cluster info with the new node group list
	tbK8sCInfo.K8sNodeGroupList = updatedNodeGroups

	// Store the updated cluster info
	storeK8sClusterInfo(nsId, tbK8sCInfo)

	log.Info().Msgf("Successfully removed NodeGroup %s from local cluster info", k8sNodeGroupName)
	return nil
}

func convertSpiderKeyValueListToTbKeyValueList(spKeyValueList []model.KeyValue) []model.KeyValue {
	var tbKeyValueList []model.KeyValue
	for _, v := range spKeyValueList {
		tbKeyValueList = append(tbKeyValueList, v)
	}
	return tbKeyValueList
}

func updateK8sClusterInfoFromSpiderClusterInfo(tbK8sCInfo *model.K8sClusterInfo, spCInfo *model.SpiderClusterInfo) {
	tbK8sCInfo.Version = spCInfo.Version
	updateK8sClusterNetworkInfoFromSpiderNetworkInfo(&tbK8sCInfo.Network, &spCInfo.Network)
	updateK8sNodeGroupInfoListFromSpiderNodeGroupInfoList(&tbK8sCInfo.K8sNodeGroupList, &spCInfo.NodeGroupList)
	updateK8sAccessInfoFromSpiderAccessInfo(&tbK8sCInfo.AccessInfo, &spCInfo.AccessInfo)
	updateK8sAddonsInfoFromSpiderAddonsInfo(&tbK8sCInfo.Addons, &spCInfo.Addons)
	tbK8sCInfo.Status = convertSpiderClusterStatusToK8sClusterStatus(spCInfo.Status)
	tbK8sCInfo.CreatedTime = spCInfo.CreatedTime
	tbK8sCInfo.KeyValueList = convertSpiderKeyValueListToTbKeyValueList(spCInfo.KeyValueList)

	tbK8sCInfo.CspResourceName = spCInfo.IId.NameId
	tbK8sCInfo.CspResourceId = spCInfo.IId.SystemId
}

func fillK8sNodeGroupInfoFromK8sNodeGroupReq(tbK8sNGInfo *model.K8sNodeGroupInfo, tbK8sNGReq *model.K8sNodeGroupReq) {
	tbK8sNGInfo.Id = tbK8sNGReq.Name
	tbK8sNGInfo.Name = tbK8sNGReq.Name
	tbK8sNGInfo.ImageId = tbK8sNGReq.ImageId
	tbK8sNGInfo.SpecId = tbK8sNGReq.SpecId
	tbK8sNGInfo.RootDiskType = tbK8sNGReq.RootDiskType
	tbK8sNGInfo.RootDiskSize = tbK8sNGReq.RootDiskSize
	tbK8sNGInfo.SshKeyId = tbK8sNGReq.SshKeyId

	// Convert string to appropriate types with better error handling
	if on, err := strconv.ParseBool(tbK8sNGReq.OnAutoScaling); err == nil {
		tbK8sNGInfo.OnAutoScaling = on
	} else {
		log.Warn().Msgf("Failed to parse OnAutoScaling '%s', defaulting to true", tbK8sNGReq.OnAutoScaling)
		tbK8sNGInfo.OnAutoScaling = true
	}

	if size, err := strconv.Atoi(tbK8sNGReq.DesiredNodeSize); err == nil {
		tbK8sNGInfo.DesiredNodeSize = size
	} else {
		log.Warn().Msgf("Failed to parse DesiredNodeSize '%s', defaulting to 1", tbK8sNGReq.DesiredNodeSize)
		tbK8sNGInfo.DesiredNodeSize = 1
	}

	if size, err := strconv.Atoi(tbK8sNGReq.MinNodeSize); err == nil {
		tbK8sNGInfo.MinNodeSize = size
	} else {
		log.Warn().Msgf("Failed to parse MinNodeSize '%s', defaulting to 1", tbK8sNGReq.MinNodeSize)
		tbK8sNGInfo.MinNodeSize = 1
	}

	if size, err := strconv.Atoi(tbK8sNGReq.MaxNodeSize); err == nil {
		tbK8sNGInfo.MaxNodeSize = size
	} else {
		log.Warn().Msgf("Failed to parse MaxNodeSize '%s', defaulting to 2", tbK8sNGReq.MaxNodeSize)
		tbK8sNGInfo.MaxNodeSize = 2
	}
}

func fillK8sNodeGroupInfoListFromK8sNodeGroupReqList(tbK8sNGInfoList *[]model.K8sNodeGroupInfo, tbK8sNGReqList *[]model.K8sNodeGroupReq) {
	var err error
	if tbK8sNGInfoList == nil {
		err = fmt.Errorf("invalid K8sNodeGroupInfoList")
		log.Err(err).Msgf("")
		return
	}
	if tbK8sNGReqList == nil {
		err = fmt.Errorf("invalid K8sNodeGroupReqList")
		log.Err(err).Msgf("")
		return
	}

	for _, tbK8sNGReq := range *tbK8sNGReqList {
		tbK8sNGInfo := model.K8sNodeGroupInfo{}
		fillK8sNodeGroupInfoFromK8sNodeGroupReq(&tbK8sNGInfo, &tbK8sNGReq)
		*tbK8sNGInfoList = append(*tbK8sNGInfoList, tbK8sNGInfo)
	}
}

func updateK8sNodeGroupInfoListFromSpiderNodeGroupInfoList(tbK8sNGInfoList *[]model.K8sNodeGroupInfo, spNGInfoList *[]model.SpiderNodeGroupInfo) {
	var err error
	if tbK8sNGInfoList == nil {
		err = fmt.Errorf("invalid K8sNodeGroupInfoList")
		log.Err(err).Msgf("")
		return
	}
	if spNGInfoList == nil {
		err = fmt.Errorf("invalid SpiderNodeGroupInfoList")
		log.Err(err).Msgf("")
		return
	}

	// Make tbK8sNGInfoListNew without deleted NodeGroupInfo from tbK8sNGInfoList
	var newList []model.K8sNodeGroupInfo
	for _, tbK8sNGInfo := range *tbK8sNGInfoList {
		found := false
		for _, spNGInfo := range *spNGInfoList {
			if tbK8sNGInfo.Name == spNGInfo.IId.NameId {
				found = true
				break
			}
		}
		if found == true {
			newList = append(newList, tbK8sNGInfo)
		}
	}

	// Update recent SpiderNodeGroupInfo to K8sNodeGroupInfo
	var absentList []model.K8sNodeGroupInfo
	for _, spNGInfo := range *spNGInfoList {
		found := false
		for i := range newList {
			if newList[i].Name == spNGInfo.IId.NameId {
				updateK8sNodeGroupInfoFromSpiderNodeGroupInfo(&newList[i], &spNGInfo)
				found = true
				break
			}
		}
		if found == false {
			// In case of removing the nodegroup
			absentInfo := model.K8sNodeGroupInfo{}
			updateK8sNodeGroupInfoFromSpiderNodeGroupInfo(&absentInfo, &spNGInfo)
			absentList = append(absentList, absentInfo)
		}
	}

	if len(absentList) > 0 {
		newList = append(newList, absentList...)
	}

	// Replace *tbK8sNGInfoList to newList
	*tbK8sNGInfoList = newList
}

func convertSpiderClusterStatusToK8sClusterStatus(spClusterStatus model.SpiderClusterStatus) model.K8sClusterStatus {
	if spClusterStatus == model.SpiderClusterCreating {
		return model.K8sClusterCreating
	} else if spClusterStatus == model.SpiderClusterActive {
		return model.K8sClusterActive
	} else if spClusterStatus == model.SpiderClusterInactive {
		return model.K8sClusterInactive
	} else if spClusterStatus == model.SpiderClusterUpdating {
		return model.K8sClusterUpdating
	} else if spClusterStatus == model.SpiderClusterDeleting {
		return model.K8sClusterDeleting
	}

	return model.K8sClusterInactive
}

func convertSpiderNodeGroupStatusToK8sNodeGroupStatus(spNodeGroupStatus model.SpiderNodeGroupStatus) model.K8sNodeGroupStatus {
	if spNodeGroupStatus == model.SpiderNodeGroupCreating {
		return model.K8sNodeGroupCreating
	} else if spNodeGroupStatus == model.SpiderNodeGroupActive {
		return model.K8sNodeGroupActive
	} else if spNodeGroupStatus == model.SpiderNodeGroupInactive {
		return model.K8sNodeGroupInactive
	} else if spNodeGroupStatus == model.SpiderNodeGroupUpdating {
		return model.K8sNodeGroupUpdating
	} else if spNodeGroupStatus == model.SpiderNodeGroupDeleting {
		return model.K8sNodeGroupDeleting
	}

	return model.K8sNodeGroupInactive
}
