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

// Package resource is to manage multi-cloud infra resource
package resource

// import (
// 	"encoding/json"
// 	"fmt"
// 	"os"

// 	"github.com/cloud-barista/cb-tumblebug/src/core/common"
// 	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
// 	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
// 	"github.com/cloud-barista/cb-tumblebug/src/core/model"
// 	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
// 	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"
// 	"github.com/go-resty/resty/v2"
// 	"github.com/rs/zerolog/log"
// )

// // ObjectStorageStatus represents the status of a network resource.
// type ObjectStorageStatus string

// const (

// 	// CRUD operations
// 	ObjectStorageOnConfiguring ObjectStorageStatus = "Configuring" // Resources are being configured.
// 	// ObjectStorageOnReading     ObjectStorageStatus = "Reading"     // The network information is being read.
// 	// ObjectStorageOnUpdating    ObjectStorageStatus = "Updating"    // The network is being updated.
// 	ObjectStorageOnDeleting ObjectStorageStatus = "Deleting" // The network is being deleted.
// 	// // ObjectStorageOnRefinining  ObjectStorageStatus = "Refining"    // The network is being refined.

// 	// // Register/deregister operations
// 	// ObjectStorageOnRegistering   ObjectStorageStatus = "Registering"  // The network is being registered.
// 	// ObjectStorageOnDeregistering ObjectStorageStatus = "Dergistering" // The network is being registered.

// 	// NetworkAvailable status
// 	ObjectStorageAvailable ObjectStorageStatus = "Available" // The network is fully created and ready for use.

// 	// // In Use status
// 	// ObjectStorageInUse ObjectStorageStatus = "InUse" // The network is currently in use.

// 	// // Unknwon status
// 	// ObjectStorageUnknown ObjectStorageStatus = "Unknown" // The network status is unknown.

// 	// // ObjectStorageError Handling
// 	// ObjectStorageError              ObjectStorageStatus = "Error"              // An error occurred during a CRUD operation.
// 	// ObjectStorageErrorOnConfiguring ObjectStorageStatus = "ErrorOnConfiguring" // An error occurred during the configuring operation.
// 	// ObjectStorageErrorOnReading     ObjectStorageStatus = "ErrorOnReading"     // An error occurred during the reading operation.
// 	// ObjectStorageErrorOnUpdating    ObjectStorageStatus = "ErrorOnUpdating"    // An error occurred during the updating operation.
// 	// ObjectStorageErrorOnDeleting    ObjectStorageStatus = "ErrorOnDeleting"    // An error occurred during the deleting operation.
// 	// ObjectStorageErrorOnRegistering ObjectStorageStatus = "ErrorOnRegistering" // An error occurred during the registering operation.
// )

// type ObjectStorageAction string

// var validCspForObjectStorage = map[string]bool{
// 	"aws":   true,
// 	"azure": true,
// 	"gcp":   true,
// 	"ncp":   true,
// 	// "alibaba": true,
// 	// "nhn":     true,
// 	// "kt":      true,

// 	// Add more CSPs here
// }

// func IsValidCspForObjectStorage(csp string) (bool, error) {
// 	if !validCspForObjectStorage[csp] {
// 		return false, fmt.Errorf("currently not supported CSP, %s", csp)
// 	}
// 	return true, nil
// }

// // func whichCspForObjectStorage(csp1, csp2 string) string {
// // 	return csp1 + "," + csp2
// // }

// // CreateObjectStorage creates a SQL database via Terrarium
// func CreateObjectStorage(nsId string, objectStorageReq *model.RestPostObjectStorageRequest, retry string) (model.ObjectStorageInfo, error) {

// 	// Object Storage objects
// 	var emptyRet model.ObjectStorageInfo
// 	var objectStorageInfo model.ObjectStorageInfo
// 	var err error = nil
// 	var retried bool = (retry == "retry")

// 	/*
// 	 * Validate the input parameters
// 	 */

// 	err = common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = common.CheckString(objectStorageReq.Name)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	ok, err := IsValidCspForObjectStorage(objectStorageReq.CSP)
// 	if !ok {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Check the CSPs of the sites
// 	switch objectStorageReq.CSP {
// 	case "aws":
// 		// TODO: Check if the subnets are in the different AZs
// 		//

// 	case "azure":
// 		// Check the required CSP resources
// 		if objectStorageReq.RequiredCSPResource.Azure.ResourceGroup == "" {
// 			err = fmt.Errorf("required Azure resource group is empty")
// 			log.Error().Err(err).Msg("")
// 			return emptyRet, err
// 		}
// 	}

// 	// Set the resource type
// 	resourceType := model.StrObjectStorage

// 	// Set the Object Storage object in advance
// 	uid := common.GenUid()
// 	objectStorageInfo.ResourceType = resourceType
// 	objectStorageInfo.Name = objectStorageReq.Name
// 	objectStorageInfo.Id = objectStorageReq.Name
// 	objectStorageInfo.Uid = uid
// 	objectStorageInfo.Description = "Object Storage at " + objectStorageReq.Region + " in " + objectStorageReq.CSP
// 	objectStorageInfo.ConnectionName = objectStorageReq.ConnectionName
// 	objectStorageInfo.ConnectionConfig, err = common.GetConnConfig(objectStorageInfo.ConnectionName)
// 	if err != nil {
// 		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
// 		log.Error().Err(err).Msg("")
// 	}

// 	// Set a objectStorageKey for the Object Storage object
// 	objectStorageKey := common.GenResourceKey(nsId, resourceType, objectStorageInfo.Id)
// 	// Check if the Object Storage resource already exists or not
// 	exists, err := CheckResource(nsId, resourceType, objectStorageInfo.Id)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		err := fmt.Errorf("failed to check if the resource type, %s (%s) exists or not", resourceType, objectStorageInfo.Id)
// 		return emptyRet, err
// 	}
// 	// For retry, read the stored Object Storage info if exists
// 	if exists {
// 		if !retried {
// 			err := fmt.Errorf("already exists, Object Storage: %s", objectStorageInfo.Id)
// 			log.Error().Err(err).Msg("")
// 			return emptyRet, err
// 		}

// 		// Read the stored Object Storage info
// 		objectStorageKv, _, err := kvstore.GetKv(objectStorageKey)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return emptyRet, err
// 		}
// 		err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 		if err != nil {
// 			log.Error().Err(err).Msg("")
// 			return emptyRet, err
// 		}

// 		objectStorageInfo.Name = objectStorageReq.Name
// 		objectStorageInfo.Id = objectStorageReq.Name
// 		objectStorageInfo.Description = "Object Storage at " + objectStorageReq.Region + " in " + objectStorageReq.CSP
// 		objectStorageInfo.ConnectionName = objectStorageReq.ConnectionName
// 		objectStorageInfo.ConnectionConfig, err = common.GetConnConfig(objectStorageInfo.ConnectionName)
// 		if err != nil {
// 			err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
// 			log.Error().Err(err).Msg("")
// 		}
// 	}

// 	// [Set and store status]
// 	objectStorageInfo.Status = string(ObjectStorageOnConfiguring)
// 	val, err := json.Marshal(objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = kvstore.Put(objectStorageKey, string(val))
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("Object Storage Info(initial): %+v", objectStorageInfo)

// 	/*
// 	 * [Via Terrarium] Create a Object Storage
// 	 */

// 	// Initialize resty client with basic auth
// 	client := resty.New()
// 	apiUser := os.Getenv("TB_API_USERNAME")
// 	apiPass := os.Getenv("TB_API_PASSWORD")
// 	client.SetBasicAuth(apiUser, apiPass)

// 	// Set Terrarium endpoint
// 	epTerrarium := model.TerrariumRestUrl

// 	// Set a terrarium ID
// 	trId := objectStorageInfo.Uid

// 	if !retried {
// 		// Issue a terrarium
// 		method := "POST"
// 		url := fmt.Sprintf("%s/tr", epTerrarium)
// 		reqTr := new(terrariumModel.TerrariumInfo)
// 		reqTr.Id = trId
// 		reqTr.Description = "Object Storage at " + objectStorageReq.Region + " in " + objectStorageReq.CSP

// 		resTrInfo := new(terrariumModel.TerrariumInfo)

// 		err = clientManager.ExecuteHttpRequest(
// 			client,
// 			method,
// 			url,
// 			nil,
// 			clientManager.SetUseBody(*reqTr),
// 			reqTr,
// 			resTrInfo,
// 			clientManager.VeryShortDuration,
// 		)

// 		if err != nil {
// 			log.Err(err).Msg("")
// 			return emptyRet, err
// 		}

// 		log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
// 		log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

// 		// init env
// 		method = "POST"
// 		url = fmt.Sprintf("%s/tr/%s/object-storage/env", epTerrarium, trId)
// 		queryParams := "provider=" + objectStorageReq.CSP
// 		url += "?" + queryParams

// 		requestBody := clientManager.NoBody
// 		resTerrariumEnv := new(model.Response)

// 		err = clientManager.ExecuteHttpRequest(
// 			client,
// 			method,
// 			url,
// 			nil,
// 			clientManager.SetUseBody(requestBody),
// 			&requestBody,
// 			resTerrariumEnv,
// 			clientManager.VeryShortDuration,
// 		)

// 		if err != nil {
// 			log.Err(err).Msg("")
// 			return emptyRet, err
// 		}

// 		log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
// 		log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)
// 	}

// 	/*
// 	 * [Via Terrarium] Generate the infracode for the Object Storage of each CSP
// 	 */
// 	switch objectStorageReq.CSP {
// 	case "aws":
// 		// generate infracode
// 		method := "POST"
// 		url := fmt.Sprintf("%s/tr/%s/object-storage/infracode", epTerrarium, trId)
// 		reqInfracode := new(terrariumModel.CreateInfracodeOfObjectStorageRequest)
// 		reqInfracode.TfVars.TerrariumID = trId
// 		reqInfracode.TfVars.CSPRegion = objectStorageReq.Region
// 		// reqInfracode.TfVars.CSPResourceGroup

// 		resInfracode := new(model.Response)

// 		err = clientManager.ExecuteHttpRequest(
// 			client,
// 			method,
// 			url,
// 			nil,
// 			clientManager.SetUseBody(*reqInfracode),
// 			reqInfracode,
// 			resInfracode,
// 			clientManager.VeryShortDuration,
// 		)

// 		if err != nil {
// 			log.Err(err).Msg("")
// 			return emptyRet, err
// 		}
// 		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
// 		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

// 	case "azure":
// 		// generate infracode
// 		method := "POST"
// 		url := fmt.Sprintf("%s/tr/%s/object-storage/infracode", epTerrarium, trId)
// 		reqInfracode := new(terrariumModel.CreateInfracodeOfObjectStorageRequest)
// 		reqInfracode.TfVars.TerrariumID = trId
// 		reqInfracode.TfVars.CSPRegion = objectStorageReq.Region
// 		reqInfracode.TfVars.CSPResourceGroup = objectStorageReq.RequiredCSPResource.Azure.ResourceGroup

// 		resInfracode := new(model.Response)

// 		err = clientManager.ExecuteHttpRequest(
// 			client,
// 			method,
// 			url,
// 			nil,
// 			clientManager.SetUseBody(*reqInfracode),
// 			reqInfracode,
// 			resInfracode,
// 			clientManager.VeryShortDuration,
// 		)

// 		if err != nil {
// 			log.Err(err).Msg("")
// 			return emptyRet, err
// 		}
// 		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
// 		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

// 	// case "gcp":
// 	// 	// generate infracode
// 	// 	method := "POST"
// 	// 	url := fmt.Sprintf("%s/tr/%s/object-storage/infracode", epTerrarium, trId)
// 	// 	reqInfracode := new(terrariumModel.CreateInfracodeOfObjectStorageRequest)
// 	// 	reqInfracode.TfVars.TerrariumID = trId
// 	// 	reqInfracode.TfVars.CSPRegion = objectStorageReq.Region

// 	// 	resInfracode := new(model.Response)

// 	// 	err = clientManager.ExecuteHttpRequest(
// 	// 		client,
// 	// 		method,
// 	// 		url,
// 	// 		nil,
// 	// 		clientManager.SetUseBody(*reqInfracode),
// 	// 		reqInfracode,
// 	// 		resInfracode,
// 	// 		clientManager.VeryShortDuration,
// 	// 	)

// 	// 	if err != nil {
// 	// 		log.Err(err).Msg("")
// 	// 		return emptyRet, err
// 	// 	}
// 	// 	log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
// 	// 	log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

// 	// case "ncp":
// 	// 	// generate infracode
// 	// 	method := "POST"
// 	// 	url := fmt.Sprintf("%s/tr/%s/object-storage/infracode", epTerrarium, trId)
// 	// 	reqInfracode := new(terrariumModel.CreateInfracodeOfObjectStorageRequest)
// 	// 	reqInfracode.TfVars.TerrariumID = trId
// 	// 	reqInfracode.TfVars.CSPRegion = objectStorageReq.Region

// 	// 	resInfracode := new(model.Response)

// 	// 	err = clientManager.ExecuteHttpRequest(
// 	// 		client,
// 	// 		method,
// 	// 		url,
// 	// 		nil,
// 	// 		clientManager.SetUseBody(*reqInfracode),
// 	// 		reqInfracode,
// 	// 		resInfracode,
// 	// 		clientManager.VeryShortDuration,
// 	// 	)

// 	// 	if err != nil {
// 	// 		log.Err(err).Msg("")
// 	// 		return emptyRet, err
// 	// 	}
// 	// 	log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
// 	// 	log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

// 	default:
// 		log.Warn().Msgf("not valid CSP: %s", objectStorageReq.CSP)
// 	}

// 	/*
// 	 * [Via Terrarium] Check the infracode
// 	 */

// 	// check the infracode (by `tofu plan`)
// 	method := "POST"
// 	url := fmt.Sprintf("%s/tr/%s/object-storage/plan", epTerrarium, trId)
// 	requestBody := clientManager.NoBody
// 	resPlan := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resPlan,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	log.Debug().Msgf("resPlan: %+v", resPlan.Message)
// 	log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

// 	// apply
// 	// wait until the task is completed
// 	// or response immediately with requestId as it is a time-consuming task
// 	// and provide seperate api to check the status
// 	method = "POST"
// 	url = fmt.Sprintf("%s/tr/%s/object-storage", epTerrarium, trId)
// 	requestBody = clientManager.NoBody
// 	resApply := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resApply,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	log.Debug().Msgf("resApply: %+v", resApply.Message)
// 	log.Trace().Msgf("resApply: %+v", resApply.Detail)

// 	// Set the Object Storage info
// 	var trObjectStorageInfo terrariumModel.OutputObjectStorageInfo
// 	jsonData, err := json.Marshal(resApply.Object)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 	}
// 	err = json.Unmarshal(jsonData, &trObjectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 	}

// 	objectStorageInfo.CspResourceId = ""
// 	objectStorageInfo.CspResourceName = trObjectStorageInfo.ObjectStorageDetail.StorageName
// 	objectStorageInfo.Details = trObjectStorageInfo.ObjectStorageDetail

// 	/*
// 	 * Set opeartion status and store objectStorageInfo
// 	 */

// 	objectStorageInfo.Status = string(ObjectStorageAvailable)

// 	log.Debug().Msgf("Object Storage Info(final): %+v", objectStorageInfo)

// 	value, err := json.Marshal(objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = kvstore.Put(objectStorageKey, string(value))
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Check if the Object Storage info is stored
// 	objectStorageKv, exists, err := kvstore.GetKv(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	if !exists {
// 		err := fmt.Errorf("does not exist, Object Storage: %s", objectStorageInfo.Id)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Store label info using CreateOrUpdateLabel
// 	labels := map[string]string{
// 		model.LabelManager:         model.StrManager,
// 		model.LabelNamespace:       nsId,
// 		model.LabelLabelType:       model.StrObjectStorage,
// 		model.LabelId:              objectStorageInfo.Id,
// 		model.LabelName:            objectStorageInfo.Name,
// 		model.LabelUid:             objectStorageInfo.Uid,
// 		model.LabelCspResourceId:   objectStorageInfo.CspResourceId,
// 		model.LabelCspResourceName: objectStorageInfo.CspResourceName,
// 		model.LabelStatus:          objectStorageInfo.Status,
// 		model.LabelDescription:     objectStorageInfo.Description,
// 	}
// 	err = label.CreateOrUpdateLabel(model.StrObjectStorage, objectStorageInfo.Uid, objectStorageKey, labels)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	return objectStorageInfo, nil
// }

// // GetObjectStorage returns a Object Storage via Terrarium
// func GetObjectStorage(nsId string, objectStorageId string, detail string) (model.ObjectStorageInfo, error) {

// 	var emptyRet model.ObjectStorageInfo
// 	var objectStorageInfo model.ObjectStorageInfo
// 	var err error = nil
// 	/*
// 	 * Validate the input parameters
// 	 */

// 	err = common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = common.CheckString(objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	if detail != "refined" && detail != "raw" && detail != "" {
// 		err = fmt.Errorf("not valid detail: %s", detail)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	if detail == "" {
// 		log.Warn().Msg("detail is empty, set to refined")
// 		detail = "refined"
// 	}

// 	// Set the resource type
// 	resourceType := model.StrObjectStorage

// 	// Set a objectStorageKey for the Object Storage object
// 	objectStorageKey := common.GenResourceKey(nsId, resourceType, objectStorageId)
// 	// Check if the Object Storage resource already exists or not
// 	exists, err := CheckResource(nsId, resourceType, objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		err := fmt.Errorf("failed to check if the Object Storage(%s) exists or not", objectStorageId)
// 		return emptyRet, err
// 	}
// 	if !exists {
// 		err := fmt.Errorf("does not exist, Object Storage: %s", objectStorageId)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Read the stored Object Storage info
// 	objectStorageKv, _, err := kvstore.GetKv(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Initialize resty client with basic auth
// 	client := resty.New()
// 	apiUser := os.Getenv("TB_API_USERNAME")
// 	apiPass := os.Getenv("TB_API_PASSWORD")
// 	client.SetBasicAuth(apiUser, apiPass)

// 	trId := objectStorageInfo.Uid

// 	// set endpoint
// 	epTerrarium := model.TerrariumRestUrl

// 	// Get the terrarium info
// 	method := "GET"
// 	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
// 	requestBody := clientManager.NoBody
// 	resTrInfo := new(terrariumModel.TerrariumInfo)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resTrInfo,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
// 	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

// 	// e.g. "object-storage"
// 	enrichments := resTrInfo.Enrichments

// 	// Get resource info
// 	method = "GET"
// 	url = fmt.Sprintf("%s/tr/%s/%s?detail=%s", epTerrarium, trId, enrichments, detail)
// 	requestBody = clientManager.NoBody
// 	resResourceInfo := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resResourceInfo,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	var trObjectStorageInfo terrariumModel.OutputObjectStorageInfo
// 	jsonData, err := json.Marshal(resResourceInfo.Object)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 	}
// 	err = json.Unmarshal(jsonData, &trObjectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 	}

// 	objectStorageInfo.CspResourceId = ""
// 	objectStorageInfo.CspResourceName = trObjectStorageInfo.ObjectStorageDetail.StorageName
// 	objectStorageInfo.Details = trObjectStorageInfo.ObjectStorageDetail

// 	log.Debug().Msgf("Object Storage Info(final): %+v", objectStorageInfo)

// 	value, err := json.Marshal(objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = kvstore.Put(objectStorageKey, string(value))
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Check if the Object Storage info is stored
// 	objectStorageKv, exists, err = kvstore.GetKv(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	if !exists {
// 		err := fmt.Errorf("does not exist, Object Storage: %s", objectStorageInfo.Id)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	return objectStorageInfo, nil
// }

// // DeleteObjectStorage deletes a SQL database via Terrarium
// func DeleteObjectStorage(nsId string, objectStorageId string) (model.SimpleMsg, error) {

// 	// VPN objects
// 	var emptyRet model.SimpleMsg
// 	var objectStorageInfo model.ObjectStorageInfo
// 	var err error = nil

// 	/*
// 	 * Validate the input parameters
// 	 */

// 	err = common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = common.CheckString(objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Set the resource type
// 	resourceType := model.StrObjectStorage

// 	// Set a objectStorageKey for the Object Storage object
// 	objectStorageKey := common.GenResourceKey(nsId, resourceType, objectStorageId)
// 	// Check if the Object Storage resource already exists or not
// 	exists, err := CheckResource(nsId, resourceType, objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		err := fmt.Errorf("failed to check if the Object Storage (%s) exists or not", objectStorageId)
// 		return emptyRet, err
// 	}
// 	if !exists {
// 		err := fmt.Errorf("does not exist, Object Storage: %s", objectStorageId)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Read the stored Object Storage info
// 	objectStorageKv, _, err := kvstore.GetKv(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// [Set and store status]
// 	objectStorageInfo.Status = string(ObjectStorageOnDeleting)
// 	val, err := json.Marshal(objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = kvstore.Put(objectStorageKey, string(val))
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Initialize resty client with basic auth
// 	client := resty.New()
// 	apiUser := os.Getenv("TB_API_USERNAME")
// 	apiPass := os.Getenv("TB_API_PASSWORD")
// 	client.SetBasicAuth(apiUser, apiPass)

// 	trId := objectStorageInfo.Uid

// 	// set endpoint
// 	epTerrarium := model.TerrariumRestUrl

// 	// Get the terrarium info
// 	method := "GET"
// 	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
// 	requestBody := clientManager.NoBody
// 	resTrInfo := new(terrariumModel.TerrariumInfo)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resTrInfo,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
// 	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
// 	enrichments := resTrInfo.Enrichments

// 	// delete enrichments
// 	method = "DELETE"
// 	url = fmt.Sprintf("%s/tr/%s/%s", epTerrarium, trId, enrichments)
// 	requestBody = clientManager.NoBody
// 	resDeleteEnrichments := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resDeleteEnrichments,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Message)
// 	log.Trace().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Detail)

// 	// delete env
// 	method = "DELETE"
// 	url = fmt.Sprintf("%s/tr/%s/%s/env", epTerrarium, trId, enrichments)
// 	requestBody = clientManager.NoBody
// 	resDeleteEnv := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resDeleteEnv,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resDeleteEnv: %+v", resDeleteEnv.Message)
// 	log.Trace().Msgf("resDeleteEnv: %+v", resDeleteEnv.Detail)

// 	// delete terrarium
// 	method = "DELETE"
// 	url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
// 	requestBody = clientManager.NoBody
// 	resDeleteTr := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resDeleteTr,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resDeleteTr: %+v", resDeleteTr.Message)
// 	log.Trace().Msgf("resDeleteTr: %+v", resDeleteTr.Detail)

// 	// [Set and store status]
// 	err = kvstore.Delete(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Remove label info using DeleteLabelObject
// 	err = label.DeleteLabelObject(model.StrObjectStorage, objectStorageInfo.Uid)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	res := model.SimpleMsg{
// 		Message: resDeleteTr.Message,
// 	}

// 	return res, nil
// }

// // GetRequestStatusOfObjectStorage checks the status of a specific request
// func GetRequestStatusOfObjectStorage(nsId string, objectStorageId string, reqId string) (model.Response, error) {

// 	var emptyRet model.Response
// 	var objectStorageInfo model.ObjectStorageInfo
// 	var err error = nil

// 	/*
// 	 * Validate the input parameters
// 	 */

// 	err = common.CheckString(nsId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = common.CheckString(objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Set the resource type
// 	resourceType := model.StrObjectStorage

// 	// Set a objectStorageKey for the Object Storage object
// 	objectStorageKey := common.GenResourceKey(nsId, resourceType, objectStorageId)
// 	// Check if the Object Storage resource already exists or not
// 	exists, err := CheckResource(nsId, resourceType, objectStorageId)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		err := fmt.Errorf("failed to check if the Object Storage(%s) exists or not", objectStorageId)
// 		return emptyRet, err
// 	}
// 	if !exists {
// 		err := fmt.Errorf("does not exist, Object Storage: %s", objectStorageId)
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Read the stored Object Storage info
// 	objectStorageKv, _, err := kvstore.GetKv(objectStorageKey)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	err = json.Unmarshal([]byte(objectStorageKv.Value), &objectStorageInfo)
// 	if err != nil {
// 		log.Error().Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	// Initialize resty client with basic auth
// 	client := resty.New()
// 	apiUser := os.Getenv("TB_API_USERNAME")
// 	apiPass := os.Getenv("TB_API_PASSWORD")
// 	client.SetBasicAuth(apiUser, apiPass)

// 	trId := objectStorageInfo.Uid

// 	// set endpoint
// 	epTerrarium := model.TerrariumRestUrl

// 	// Get the terrarium info
// 	method := "GET"
// 	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
// 	requestBody := clientManager.NoBody
// 	resTrInfo := new(terrariumModel.TerrariumInfo)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(requestBody),
// 		&requestBody,
// 		resTrInfo,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}

// 	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
// 	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
// 	enrichments := resTrInfo.Enrichments

// 	// Get resource info
// 	method = "GET"
// 	url = fmt.Sprintf("%s/tr/%s/%s/request/%s", epTerrarium, trId, enrichments, reqId)
// 	reqReqStatus := clientManager.NoBody
// 	resReqStatus := new(model.Response)

// 	err = clientManager.ExecuteHttpRequest(
// 		client,
// 		method,
// 		url,
// 		nil,
// 		clientManager.SetUseBody(reqReqStatus),
// 		&reqReqStatus,
// 		resReqStatus,
// 		clientManager.VeryShortDuration,
// 	)

// 	if err != nil {
// 		log.Err(err).Msg("")
// 		return emptyRet, err
// 	}
// 	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

// 	return *resReqStatus, nil
// }
