/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// ProvisionDataDisk is func to provision DataDisk to Node (create and attach to Node)
func ProvisionDataDisk(ctx context.Context, nsId string, infraId string, nodeId string, u *model.DataDiskNodeReq) (model.NodeInfo, error) {
	node, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	// A disk can only be attached to a Node in the same zone, so default to the
	// Node's zone instead of the connection's (they differ whenever the Node was
	// placed in a non-default zone)
	zone := u.Zone
	if zone == "" {
		zone = node.Region.Zone
	}

	createDiskReq := model.DataDiskReq{
		Name:           u.Name,
		ConnectionName: node.ConnectionName,
		Zone:           zone,
		DiskType:       u.DiskType,
		DiskSize:       u.DiskSize,
		Description:    u.Description,
	}

	newDataDisk, err := resource.CreateDataDisk(ctx, nsId, &createDiskReq, "")
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}
	retry := 3
	for range retry {
		nodeInfo, err := AttachDetachDataDisk(nsId, infraId, nodeId, model.AttachDataDisk, newDataDisk.Id, false)
		if err != nil {
			log.Error().Err(err).Msg("")
		} else {
			return nodeInfo, nil
		}
		time.Sleep(5 * time.Second)
	}
	return model.NodeInfo{}, err
}

// AttachDetachDataDisk is func to attach/detach DataDisk to/from Node
// fetchNodeDetailsWithRetry reads node details from Spider, retrying briefly to absorb
// CSP eventual-consistency lag after a change.
func fetchNodeDetailsWithRetry(node model.NodeInfo) ([]model.KeyValue, error) {
	const attempts = 4
	const interval = 3 * time.Second

	client := clientManager.NewHttpClient()
	url := fmt.Sprintf("%s/node/%s", model.SpiderRestUrl, node.CspResourceName)
	requestBodyConnection := model.SpiderConnectionName{ConnectionName: node.ConnectionName}

	var err error
	for i := range attempts {
		time.Sleep(interval)

		var callResultSpiderNodeInfo model.SpiderVMInfo
		_, err = clientManager.ExecuteHttpRequest(
			client,
			"GET",
			url,
			nil,
			clientManager.SetUseBody(requestBodyConnection),
			&requestBodyConnection,
			&callResultSpiderNodeInfo,
			clientManager.MediumDuration,
		)
		if err == nil {
			return callResultSpiderNodeInfo.KeyValueList, nil
		}
		log.Debug().Err(err).Msgf("Node details not available yet (attempt %d/%d)", i+1, attempts)
	}
	return nil, err
}

func AttachDetachDataDisk(nsId string, infraId string, nodeId string, command string, dataDiskId string, force bool) (model.NodeInfo, error) {
	nodeKey := common.GenInfraKey(nsId, infraId, nodeId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(nodeKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/node': %s/%s/%s \n", nsId, infraId, nodeId)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	node := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &node)

	isInList := common.CheckElement(dataDiskId, node.DataDiskIds)
	if strings.EqualFold(command, model.DetachDataDisk) && !isInList && !force {
		err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list %v", dataDiskId, node.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	} else if strings.EqualFold(command, model.AttachDataDisk) && isInList && !force {
		err := fmt.Errorf("The dataDisk %s is already in the attached dataDisk list %v", dataDiskId, node.DataDiskIds)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	dataDiskKey := common.GenResourceKey(nsId, model.StrDataDisk, dataDiskId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err = kvstore.GetKv(dataDiskKey)
	if !exists || err != nil {
		return model.NodeInfo{}, err
	}

	dataDisk := model.DataDiskInfo{}
	json.Unmarshal([]byte(keyValue.Value), &dataDisk)

	// A deletion tombstone is not usable; fail fast with the real reason
	if dataDisk.DeletionRequestedAt != "" && !force {
		err := fmt.Errorf("the dataDisk %s has a pending/unconfirmed deletion (status=%s); %s is not allowed — retry DELETE to complete the deletion first",
			dataDiskId, dataDisk.Status, command)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	// A disk cannot cross zones; fail early with a clear reason instead of a CSP error
	if strings.EqualFold(command, model.AttachDataDisk) && !force &&
		dataDisk.Zone != "" && node.Region.Zone != "" && dataDisk.Zone != node.Region.Zone {
		err := fmt.Errorf("the dataDisk %s is in zone %s but the node %s is in zone %s; a dataDisk can only be attached to a node in the same zone",
			dataDiskId, dataDisk.Zone, nodeId, node.Region.Zone)
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	client := clientManager.NewHttpClient()
	method := "PUT"
	var callResult any
	//var requestBody interface{}

	requestBody := model.SpiderDiskAttachDetachReqWrapper{
		ConnectionName: node.ConnectionName,
		ReqInfo: model.SpiderDiskAttachDetachReq{
			VMName: node.CspResourceName,
		},
	}

	var url string
	var cmdToUpdateAsso string

	switch command {
	case model.AttachDataDisk:
		//req = req.SetResult(&model.SpiderDiskInfo{})
		url = fmt.Sprintf("%s/disk/%s/attach", model.SpiderRestUrl, dataDisk.CspResourceName)

		cmdToUpdateAsso = model.StrAdd

	case model.DetachDataDisk:
		// req = req.SetResult(&bool)
		url = fmt.Sprintf("%s/disk/%s/detach", model.SpiderRestUrl, dataDisk.CspResourceName)

		cmdToUpdateAsso = model.StrDelete

	default:

	}

	_, err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&callResult,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.NodeInfo{}, err
	}

	switch command {
	case model.AttachDataDisk:
		node.DataDiskIds = append(node.DataDiskIds, dataDiskId)
		// resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, model.StrAdd, nodeKey)
	case model.DetachDataDisk:
		oldDataDiskIds := node.DataDiskIds
		newDataDiskIds := oldDataDiskIds

		flag := false

		for i, oldDataDisk := range oldDataDiskIds {
			if oldDataDisk == dataDiskId {
				flag = true
				newDataDiskIds = append(oldDataDiskIds[:i], oldDataDiskIds[i+1:]...)
				break
			}
		}

		// Actually, in here, 'flag' cannot be false,
		// since isDataDiskAttached is confirmed to be 'true' in the beginning of this function.
		// Below is just a code snippet of 'defensive programming'.
		if !flag && !force {
			err := fmt.Errorf("Failed to find the dataDisk %s in the attached dataDisk list.", dataDiskId)
			log.Error().Err(err).Msg("")
			return model.NodeInfo{}, err
		} else {
			node.DataDiskIds = newDataDiskIds
		}
	}

	// Persist first: the CSP change is already done and is not rolled back, so it must
	// not depend on the follow-up read succeeding (issue #2648)
	UpdateNodeInfo(nsId, infraId, node)

	// Status-only patch: a whole-object write of the in-memory copy (read before the
	// CSP call) could clobber concurrent updates, e.g. a deletion tombstone
	switch command {
	case model.AttachDataDisk:
		dataDisk.Status = model.DiskAttached
	case model.DetachDataDisk:
		dataDisk.Status = model.DiskAvailable
	}
	if err := resource.UpdateResourceStatus(nsId, model.StrDataDisk, dataDiskId, string(dataDisk.Status)); err != nil {
		log.Warn().Err(err).Msgf("Failed to persist status of dataDisk %s after %s", dataDiskId, command)
	}

	// Update TB DataDisk object's 'associatedObjects' field (re-reads the stored object)
	resource.UpdateAssociatedObjectList(nsId, model.StrDataDisk, dataDiskId, cmdToUpdateAsso, nodeKey)
	log.Debug().Msgf("Updated DataDisk %s status to %s after %s operation", dataDiskId, dataDisk.Status, command)

	// Best-effort refresh of auxiliary details; failure must not fail the operation
	if details, err := fetchNodeDetailsWithRetry(node); err != nil {
		log.Warn().Err(err).Msgf("Node details not refreshed after %s of dataDisk %s (operation itself succeeded)", command, dataDiskId)
	} else {
		node.AddtionalDetails = details
		UpdateNodeInfo(nsId, infraId, node)
	}
	/*
		url = fmt.Sprintf("%s/disk/%s", model.SpiderRestUrl, dataDisk.CspResourceName)

		req = client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(connectionName).
			SetResult(&resource.SpiderDiskInfo{}) // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).

		resp, err = req.Get(url)

		fmt.Printf("HTTP Status code: %d \n", resp.StatusCode())
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf("%s", string(resp.Body()))
			fmt.Println("body: ", string(resp.Body()))
			log.Error().Err(err).Msg("")
			return node, err
		}

		updatedSpiderDisk := resp.Result().(*resource.SpiderDiskInfo)
		dataDisk.Status = updatedSpiderDisk.Status
		fmt.Printf("dataDisk.Status: %s \n", dataDisk.Status) // for debug
		resource.UpdateResourceObject(nsId, model.StrDataDisk, dataDisk)
	*/

	return node, nil
}

func GetAvailableDataDisks(nsId string, infraId string, nodeId string, option string) (any, error) {
	nodeKey := common.GenInfraKey(nsId, infraId, nodeId)

	// Check existence of the key. If no key, no update.
	keyValue, exists, err := kvstore.GetKv(nodeKey)
	if !exists || err != nil {
		err := fmt.Errorf("Failed to find 'ns/infra/node': %s/%s/%s \n", nsId, infraId, nodeId)
		log.Error().Err(err).Msg("")
		return nil, err
	}

	node := model.NodeInfo{}
	json.Unmarshal([]byte(keyValue.Value), &node)

	tbDataDisksInterface, err := resource.ListResource(nsId, model.StrDataDisk, "", "")
	if err != nil {
		err := fmt.Errorf("Failed to get dataDisk List. \n")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	jsonString, err := json.Marshal(tbDataDisksInterface)
	if err != nil {
		err := fmt.Errorf("Failed to marshal dataDisk list into JSON string. \n")
		log.Error().Err(err).Msg("")
		return nil, err
	}

	tbDataDisks := []model.DataDiskInfo{}
	json.Unmarshal(jsonString, &tbDataDisks)

	if option != "id" {
		return tbDataDisks, nil
	} else { // option == "id"
		idList := []string{}

		for _, v := range tbDataDisks {
			// Update Tb dataDisk object's status; skip-and-continue so one broken
			// record cannot take down the whole availability listing
			newObj, err := resource.GetResource(nsId, model.StrDataDisk, v.Id)
			if err != nil {
				log.Warn().Err(err).Msgf("Skipping dataDisk %s in availability listing", v.Id)
				continue
			}
			tempObj := newObj.(model.DataDiskInfo)

			if v.ConnectionName == node.ConnectionName && tempObj.Status == "Available" {
				idList = append(idList, v.Id)
			}
		}

		return idList, nil
	}
}
