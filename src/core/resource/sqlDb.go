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

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// SqlDBStatus represents the status of a network resource.
type SqlDBStatus string

const (

	// CRUD operations
	SqlDBOnConfiguring SqlDBStatus = "Configuring" // Resources are being configured.
	// SqlDBOnReading     SqlDBStatus = "Reading"     // The network information is being read.
	// SqlDBOnUpdating    SqlDBStatus = "Updating"    // The network is being updated.
	SqlDBOnDeleting SqlDBStatus = "Deleting" // The network is being deleted.
	// // NetworkOnRefinining  SqlDBStatus = "Refining"    // The network is being refined.

	// // Register/deregister operations
	// SqlDBOnRegistering   SqlDBStatus = "Registering"  // The network is being registered.
	// SqlDBOnDeregistering SqlDBStatus = "Dergistering" // The network is being registered.

	// NetworkAvailable status
	SqlDBAvailable SqlDBStatus = "Available" // The network is fully created and ready for use.

	// // In Use status
	// SqlDBInUse SqlDBStatus = "InUse" // The network is currently in use.

	// // Unknwon status
	// SqlDBUnknown SqlDBStatus = "Unknown" // The network status is unknown.

	// // NetworkError Handling
	// SqlDBError              SqlDBStatus = "Error"              // An error occurred during a CRUD operation.
	// SqlDBErrorOnConfiguring SqlDBStatus = "ErrorOnConfiguring" // An error occurred during the configuring operation.
	// SqlDBErrorOnReading     SqlDBStatus = "ErrorOnReading"     // An error occurred during the reading operation.
	// SqlDBErrorOnUpdating    SqlDBStatus = "ErrorOnUpdating"    // An error occurred during the updating operation.
	// SqlDBErrorOnDeleting    SqlDBStatus = "ErrorOnDeleting"    // An error occurred during the deleting operation.
	// SqlDBErrorOnRegistering SqlDBStatus = "ErrorOnRegistering" // An error occurred during the registering operation.
)

type SqlDBAction string

var validCspForSqlDB = map[string]bool{
	"aws":   true,
	"azure": true,
	"gcp":   true,
	"ncp":   true,
	// "alibaba": true,
	// "nhn":     true,
	// "kt":      true,

	// Add more CSPs here
}

func IsValidCspForSqlDB(csp string) (bool, error) {
	if !validCspForSqlDB[csp] {
		return false, fmt.Errorf("currently not supported CSP, %s", csp)
	}
	return true, nil
}

// func whichCspForSqlDB(csp1, csp2 string) string {
// 	return csp1 + "," + csp2
// }

// CreateSqlDb creates a SQL database via Terrarium
func CreateSqlDb(nsId string, sqlDbReq *model.RestPostSqlDbRequest, retry string) (model.SqlDBInfo, error) {

	// SQL DB objects
	var emptyRet model.SqlDBInfo
	var sqlDBInfo model.SqlDBInfo
	var err error = nil
	var retried bool = (retry == "retry")

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(sqlDbReq.Name)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	ok, err := IsValidCspForSqlDB(sqlDbReq.CSP)
	if !ok {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check the CSPs of the sites
	switch sqlDbReq.CSP {
	case "aws":
		// Check the required CSP resources
		if sqlDbReq.RequiredCSPResource.AWS.VNetID == "" {
			err = fmt.Errorf("required AWS VNetID is empty")
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		if sqlDbReq.RequiredCSPResource.AWS.Subnet1ID == "" {
			err = fmt.Errorf("required AWS subnet1ID is empty")
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		if sqlDbReq.RequiredCSPResource.AWS.Subnet2ID == "" {
			err = fmt.Errorf("required AWS subnet2ID is empty")
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		// TODO: Check if the subnets are in the different AZs
		//

	case "ncp":
		// Check the required CSP resources
		if sqlDbReq.RequiredCSPResource.NCP.SubnetID == "" {
			err = fmt.Errorf("required NCP subnetID is empty")
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Set the resource type
	resourceType := model.StrSqlDB

	// Set the SQL DB object in advance
	uid := common.GenUid()
	sqlDBInfo.ResourceType = resourceType
	sqlDBInfo.Name = sqlDbReq.Name
	sqlDBInfo.Id = sqlDbReq.Name
	sqlDBInfo.Uid = uid
	sqlDBInfo.Description = "SQL DB at " + sqlDbReq.Region + " in " + sqlDbReq.CSP
	sqlDBInfo.ConnectionName = sqlDbReq.ConnectionName
	sqlDBInfo.ConnectionConfig, err = common.GetConnConfig(sqlDBInfo.ConnectionName)
	if err != nil {
		err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
		log.Error().Err(err).Msg("")
	}

	// Set a sqlDBKey for the SQL DB object
	sqlDBKey := common.GenResourceKey(nsId, resourceType, sqlDBInfo.Id)
	// Check if the SQL DB resource already exists or not
	exists, err := CheckResource(nsId, resourceType, sqlDBInfo.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the resource type, %s (%s) exists or not", resourceType, sqlDBInfo.Id)
		return emptyRet, err
	}
	// For retry, read the stored SQL DB info if exists
	if exists {
		if !retried {
			err := fmt.Errorf("already exists, SQL DB: %s", sqlDBInfo.Id)
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		// Read the stored SQL DB info
		sqlDBKv, err := kvstore.GetKv(sqlDBKey)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		sqlDBInfo.Name = sqlDbReq.Name
		sqlDBInfo.Id = sqlDbReq.Name
		sqlDBInfo.Description = "SQL DB at " + sqlDbReq.Region + " in " + sqlDbReq.CSP
		sqlDBInfo.ConnectionName = sqlDbReq.ConnectionName
		sqlDBInfo.ConnectionConfig, err = common.GetConnConfig(sqlDBInfo.ConnectionName)
		if err != nil {
			err = fmt.Errorf("Cannot retrieve ConnectionConfig" + err.Error())
			log.Error().Err(err).Msg("")
		}
	}

	// [Set and store status]
	sqlDBInfo.Status = string(SqlDBOnConfiguring)
	val, err := json.Marshal(sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(sqlDBKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("SQL DB Info(initial): %+v", sqlDBInfo)

	/*
	 * [Via Terrarium] Create a SQL DB
	 */

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	// Set Terrarium endpoint
	epTerrarium := model.TerrariumRestUrl

	// Set a terrarium ID
	trId := sqlDBInfo.Uid

	if !retried {
		// Issue a terrarium
		method := "POST"
		url := fmt.Sprintf("%s/tr", epTerrarium)
		reqTr := new(terrariumModel.TerrariumInfo)
		reqTr.Id = trId
		reqTr.Description = "SQL DB at " + sqlDbReq.Region + " in " + sqlDbReq.CSP

		resTrInfo := new(terrariumModel.TerrariumInfo)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqTr),
			reqTr,
			resTrInfo,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
		log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

		// init env
		method = "POST"
		url = fmt.Sprintf("%s/tr/%s/sql-db/env", epTerrarium, trId)
		queryParams := "provider=" + sqlDbReq.CSP
		url += "?" + queryParams

		requestBody := common.NoBody
		resTerrariumEnv := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(requestBody),
			&requestBody,
			resTerrariumEnv,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}

		log.Debug().Msgf("resInit: %+v", resTerrariumEnv.Message)
		log.Trace().Msgf("resInit: %+v", resTerrariumEnv.Detail)
	}

	/*
	 * [Via Terrarium] Generate the infracode for the SQL DB of each CSP
	 */
	switch sqlDbReq.CSP {
	case "aws":
		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/sql-db/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfSqlDbRequest)
		reqInfracode.TfVars.TerrariumID = trId
		reqInfracode.TfVars.CSPRegion = sqlDbReq.Region
		// reqInfracode.TfVars.CSPResourceGroup
		reqInfracode.TfVars.DBInstanceSpec = sqlDbReq.DBInstanceSpec
		reqInfracode.TfVars.DBEngineVersion = sqlDbReq.DBEngineVersion
		reqInfracode.TfVars.DBAdminPassword = sqlDbReq.DBAdminPassword
		reqInfracode.TfVars.DBAdminUsername = sqlDbReq.DBAdminUsername
		reqInfracode.TfVars.CSPVNetID = sqlDbReq.RequiredCSPResource.AWS.VNetID
		reqInfracode.TfVars.CSPSubnet1ID = sqlDbReq.RequiredCSPResource.AWS.Subnet1ID
		reqInfracode.TfVars.CSPSubnet2ID = sqlDbReq.RequiredCSPResource.AWS.Subnet2ID
		reqInfracode.TfVars.DBEnginePort = sqlDbReq.DBEnginePort
		reqInfracode.TfVars.EgressCIDRBlock = "0.0.0.0/0"
		reqInfracode.TfVars.IngressCIDRBlock = "0.0.0.0/0"

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

	case "azure":
		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/sql-db/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfSqlDbRequest)
		reqInfracode.TfVars.TerrariumID = trId
		reqInfracode.TfVars.CSPRegion = sqlDbReq.Region
		reqInfracode.TfVars.DBInstanceSpec = sqlDbReq.DBInstanceSpec
		reqInfracode.TfVars.DBEngineVersion = sqlDbReq.DBEngineVersion
		reqInfracode.TfVars.DBAdminPassword = sqlDbReq.DBAdminPassword
		reqInfracode.TfVars.DBAdminUsername = sqlDbReq.DBAdminUsername
		reqInfracode.TfVars.CSPResourceGroup = sqlDbReq.RequiredCSPResource.Azure.ResourceGroup

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

	case "gcp":
		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/sql-db/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfSqlDbRequest)
		reqInfracode.TfVars.TerrariumID = trId
		reqInfracode.TfVars.CSPRegion = sqlDbReq.Region
		reqInfracode.TfVars.DBInstanceSpec = sqlDbReq.DBInstanceSpec
		reqInfracode.TfVars.DBEngineVersion = sqlDbReq.DBEngineVersion
		reqInfracode.TfVars.DBAdminUsername = sqlDbReq.DBAdminUsername
		reqInfracode.TfVars.DBAdminPassword = sqlDbReq.DBAdminPassword

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

	case "ncp":
		// generate infracode
		method := "POST"
		url := fmt.Sprintf("%s/tr/%s/sql-db/infracode", epTerrarium, trId)
		reqInfracode := new(terrariumModel.CreateInfracodeOfSqlDbRequest)
		reqInfracode.TfVars.TerrariumID = trId
		reqInfracode.TfVars.CSPRegion = sqlDbReq.Region
		reqInfracode.TfVars.DBAdminUsername = sqlDbReq.DBAdminUsername
		reqInfracode.TfVars.DBAdminPassword = sqlDbReq.DBAdminPassword
		reqInfracode.TfVars.CSPSubnet1ID = sqlDbReq.RequiredCSPResource.NCP.SubnetID

		resInfracode := new(model.Response)

		err = common.ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			common.SetUseBody(*reqInfracode),
			reqInfracode,
			resInfracode,
			common.VeryShortDuration,
		)

		if err != nil {
			log.Err(err).Msg("")
			return emptyRet, err
		}
		log.Debug().Msgf("resInfracode: %+v", resInfracode.Message)
		log.Trace().Msgf("resInfracode: %+v", resInfracode.Detail)

	default:
		log.Warn().Msgf("not valid CSP: %s", sqlDbReq.CSP)
	}

	/*
	 * [Via Terrarium] Check the infracode
	 */

	// check the infracode (by `tofu plan`)
	method := "POST"
	url := fmt.Sprintf("%s/tr/%s/sql-db/plan", epTerrarium, trId)
	requestBody := common.NoBody
	resPlan := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resPlan,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("resPlan: %+v", resPlan.Message)
	log.Trace().Msgf("resPlan: %+v", resPlan.Detail)

	// apply
	// wait until the task is completed
	// or response immediately with requestId as it is a time-consuming task
	// and provide seperate api to check the status
	method = "POST"
	url = fmt.Sprintf("%s/tr/%s/sql-db", epTerrarium, trId)
	requestBody = common.NoBody
	resApply := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resApply,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("resApply: %+v", resApply.Message)
	log.Trace().Msgf("resApply: %+v", resApply.Detail)

	// Set the SQL DB info
	var trSqlDBInfo terrariumModel.OutputSQLDBInfo
	jsonData, err := json.Marshal(resApply.Object)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	err = json.Unmarshal(jsonData, &trSqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	sqlDBInfo.CspResourceId = trSqlDBInfo.SQLDBDetail.InstanceResourceID
	sqlDBInfo.CspResourceName = trSqlDBInfo.SQLDBDetail.InstanceName
	sqlDBInfo.Details = trSqlDBInfo.SQLDBDetail

	/*
	 * Set opeartion status and store sqlDBInfo
	 */

	sqlDBInfo.Status = string(SqlDBAvailable)

	log.Debug().Msgf("SQL DB Info(final): %+v", sqlDBInfo)

	value, err := json.Marshal(sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(sqlDBKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the SQL DB info is stored
	sqlDBKv, err := kvstore.GetKv(sqlDBKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if sqlDBKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, SQL DB: %s", sqlDBInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Store label info using CreateOrUpdateLabel
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrSqlDB,
		model.LabelId:              sqlDBInfo.Id,
		model.LabelName:            sqlDBInfo.Name,
		model.LabelUid:             sqlDBInfo.Uid,
		model.LabelCspResourceId:   sqlDBInfo.CspResourceId,
		model.LabelCspResourceName: sqlDBInfo.CspResourceName,
		model.LabelStatus:          sqlDBInfo.Status,
		model.LabelDescription:     sqlDBInfo.Description,
	}
	err = label.CreateOrUpdateLabel(model.StrSqlDB, sqlDBInfo.Uid, sqlDBKey, labels)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return sqlDBInfo, nil
}

// GetSqlDb returns a SQL DB via Terrarium
func GetSqlDb(nsId string, sqlDbId string, detail string) (model.SqlDBInfo, error) {

	var emptyRet model.SqlDBInfo
	var sqlDBInfo model.SqlDBInfo
	var err error = nil
	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if detail != "refined" && detail != "raw" && detail != "" {
		err = fmt.Errorf("not valid detail: %s", detail)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if detail == "" {
		log.Warn().Msg("detail is empty, set to refined")
		detail = "refined"
	}

	// Set the resource type
	resourceType := model.StrSqlDB

	// Set a sqlDBKey for the SQL DB object
	sqlDBKey := common.GenResourceKey(nsId, resourceType, sqlDbId)
	// Check if the SQL DB resource already exists or not
	exists, err := CheckResource(nsId, resourceType, sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the SQL DB(%s) exists or not", sqlDbId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, SQL DB: %s", sqlDbId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored SQL DB info
	sqlDBKv, err := kvstore.GetKv(sqlDBKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := sqlDBInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)

	// e.g. "sql-db"
	enrichments := resTrInfo.Enrichments

	// Get resource info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s/%s?detail=%s", epTerrarium, trId, enrichments, detail)
	requestBody = common.NoBody
	resResourceInfo := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resResourceInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	// switch enrichments {
	// case "vpn/gcp-aws":
	var trVpnInfo terrariumModel.OutputSQLDBInfo
	jsonData, err := json.Marshal(resResourceInfo.Object)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	err = json.Unmarshal(jsonData, &trVpnInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	sqlDBInfo.CspResourceId = trVpnInfo.SQLDBDetail.InstanceResourceID
	sqlDBInfo.CspResourceName = trVpnInfo.SQLDBDetail.InstanceName
	sqlDBInfo.Details = trVpnInfo.SQLDBDetail

	// case "vpn/gcp-azure":
	// 	var trVpnInfo terrariumModel.OutputGcpAzureVpnInfo
	// 	jsonData, err := json.Marshal(resResourceInfo.Object)
	// 	if err != nil {
	// 		log.Error().Err(err).Msg("")
	// 	}
	// 	err = json.Unmarshal(jsonData, &trVpnInfo)
	// 	if err != nil {
	// 		log.Error().Err(err).Msg("")
	// 	}

	// 	sqlDBInfo.VPNGatewayInfo[0].CspResourceId = trVpnInfo.Azure.VirtualNetworkGateway.ID
	// 	sqlDBInfo.VPNGatewayInfo[0].CspResourceName = trVpnInfo.Azure.VirtualNetworkGateway.Name
	// 	sqlDBInfo.VPNGatewayInfo[0].Details = trVpnInfo.Azure
	// 	sqlDBInfo.VPNGatewayInfo[1].CspResourceId = trVpnInfo.GCP.HaVpnGateway.ID
	// 	sqlDBInfo.VPNGatewayInfo[1].CspResourceName = trVpnInfo.GCP.HaVpnGateway.Name
	// 	sqlDBInfo.VPNGatewayInfo[1].Details = trVpnInfo.GCP
	// default:
	// 	log.Warn().Msgf("not valid enrichments: %s", enrichments)
	// 	return emptyRet, fmt.Errorf("not valid enrichments: %s", enrichments)
	// }

	log.Debug().Msgf("SQL DB Info(final): %+v", sqlDBInfo)

	value, err := json.Marshal(sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(sqlDBKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the SQL DB info is stored
	sqlDBKv, err = kvstore.GetKv(sqlDBKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if sqlDBKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, SQL DB: %s", sqlDBInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return sqlDBInfo, nil
}

// DeleteSqlDb deletes a SQL database via Terrarium
func DeleteSqlDb(nsId string, sqlDbId string) (model.SimpleMsg, error) {

	// VPN objects
	var emptyRet model.SimpleMsg
	var sqlDBInfo model.SqlDBInfo
	var err error = nil

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrSqlDB

	// Set a sqlDbKey for the SQL DB object
	sqlDbKey := common.GenResourceKey(nsId, resourceType, sqlDbId)
	// Check if the SQL DB resource already exists or not
	exists, err := CheckResource(nsId, resourceType, sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the SQL DB (%s) exists or not", sqlDbId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, SQL DB: %s", sqlDbId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored SQL DB info
	sqlDBKv, err := kvstore.GetKv(sqlDbKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Set and store status]
	sqlDBInfo.Status = string(SqlDBOnDeleting)
	val, err := json.Marshal(sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(sqlDbKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := sqlDBInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// delete enrichments
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s/%s", epTerrarium, trId, enrichments)
	requestBody = common.NoBody
	resDeleteEnrichments := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteEnrichments,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Message)
	log.Trace().Msgf("resDeleteEnrichments: %+v", resDeleteEnrichments.Detail)

	// delete env
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s/%s/env", epTerrarium, trId, enrichments)
	requestBody = common.NoBody
	resDeleteEnv := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteEnv,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteEnv: %+v", resDeleteEnv.Message)
	log.Trace().Msgf("resDeleteEnv: %+v", resDeleteEnv.Detail)

	// delete terrarium
	method = "DELETE"
	url = fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody = common.NoBody
	resDeleteTr := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resDeleteTr,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resDeleteTr: %+v", resDeleteTr.Message)
	log.Trace().Msgf("resDeleteTr: %+v", resDeleteTr.Detail)

	// [Set and store status]
	err = kvstore.Delete(sqlDbKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Remove label info using DeleteLabelObject
	err = label.DeleteLabelObject(model.StrSqlDB, sqlDBInfo.Uid)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	res := model.SimpleMsg{
		Message: resDeleteTr.Message,
	}

	return res, nil
}

// GetRequestStatusOfSqlDb checks the status of a specific request
func GetRequestStatusOfSqlDb(nsId string, sqlDbId string, reqId string) (model.Response, error) {

	var emptyRet model.Response
	var sqlDBInfo model.SqlDBInfo
	var err error = nil

	/*
	 * Validate the input parameters
	 */

	err = common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the resource type
	resourceType := model.StrSqlDB

	// Set a sqlDBKey for the SQL DB object
	sqlDBKey := common.GenResourceKey(nsId, resourceType, sqlDbId)
	// Check if the SQL DB resource already exists or not
	exists, err := CheckResource(nsId, resourceType, sqlDbId)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the SQL DB(%s) exists or not", sqlDbId)
		return emptyRet, err
	}
	if !exists {
		err := fmt.Errorf("does not exist, SQL DB: %s", sqlDbId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored SQL DB info
	sqlDBKv, err := kvstore.GetKv(sqlDBKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = json.Unmarshal([]byte(sqlDBKv.Value), &sqlDBInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Initialize resty client with basic auth
	client := resty.New()
	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")
	client.SetBasicAuth(apiUser, apiPass)

	trId := sqlDBInfo.Uid

	// set endpoint
	epTerrarium := model.TerrariumRestUrl

	// Get the terrarium info
	method := "GET"
	url := fmt.Sprintf("%s/tr/%s", epTerrarium, trId)
	requestBody := common.NoBody
	resTrInfo := new(terrariumModel.TerrariumInfo)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		resTrInfo,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}

	log.Debug().Msgf("resTrInfo.Id: %s", resTrInfo.Id)
	log.Trace().Msgf("resTrInfo: %+v", resTrInfo)
	enrichments := resTrInfo.Enrichments

	// Get resource info
	method = "GET"
	url = fmt.Sprintf("%s/tr/%s/%s/request/%s", epTerrarium, trId, enrichments, reqId)
	reqReqStatus := common.NoBody
	resReqStatus := new(model.Response)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(reqReqStatus),
		&reqReqStatus,
		resReqStatus,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("resReqStatus: %+v", resReqStatus.Detail)

	return *resReqStatus, nil
}
