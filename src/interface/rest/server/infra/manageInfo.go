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

// Package infra is to handle REST API for infra
package infra

import (
	"fmt"
	"net/http"
	"strings"

	resource "github.com/cloud-barista/cb-tumblebug/src/core/resource"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// JSONResult is a dummy struct for Swagger annotations.
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

func isInfraClusterNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "not found")
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention Need to be revised.

// RestGetInfra godoc
// @ID GetInfra
// @Summary Get Infra object (option: status, accessInfo, nodeId)
// @Description Get Infra object (option: status, accessInfo, nodeId)
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param option query string false "Option" Enums(default, id, status, accessinfo)
// @Param filterKey query string false "(For option=id) Field key for filtering (ex: connectionName)"
// @Param filterVal query string false "(For option=id) Field value for filtering (ex: aws-ap-northeast-2)"
// @Param accessInfoOption query string false "(For option=accessinfo) accessInfoOption (showSshKey)"
// @success 200 {object} JSONResult{[DEFAULT]=model.InfraInfo,[ID]=model.IdList,[STATUS]=model.InfraStatusInfo,[AccessInfo]=model.InfraAccessInfo} "Different return structures by the given action param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId} [get]
func RestGetInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	option := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")
	accessInfoOption := c.QueryParam("accessInfoOption")

	if option == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = infra.ListNodeByFilter(nsId, infraId, filterKey, filterVal)
		return clientManager.EndRequestWithLog(c, err, content)
	} else if option == "status" {

		result, err := infra.GetInfraStatus(nsId, infraId)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}

		var content struct {
			Result *model.InfraStatusInfo `json:"status"`
		}
		content.Result = result

		return clientManager.EndRequestWithLog(c, err, content)

	} else if option == "accessinfo" {

		result, err := infra.GetInfraAccessInfo(nsId, infraId, accessInfoOption)
		return clientManager.EndRequestWithLog(c, err, result)

	} else {

		result, err := infra.GetInfraInfo(nsId, infraId)
		return clientManager.EndRequestWithLog(c, err, result)

	}
}

// RestGetInfraReqFromInfra godoc
// @ID GetInfraReqFromInfra
// @Summary Extract Infra creation request configuration from an existing Infra
// @Description Reconstruct an Infra dynamic creation request body from an existing Infra's information.
// @Description Returns a dynamic request format where networking resources (vNet, subnet, SG, sshKey)
// @Description are auto-created, making it easy to clone or recreate a similar Infra configuration.
// @Description
// @Description **Template Option:**
// @Description When the `template` query parameter is provided, the extracted configuration is
// @Description saved as a reusable Infra Dynamic Template with the given name.
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param template query string false "If provided, save the extracted config as a template with this name"
// @Success 200 {object} JSONResult{[DEFAULT]=model.InfraDynamicReq,[TEMPLATE]=model.InfraDynamicTemplateInfo} "Without template param: extracted Infra config / With template param: created template info"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/configCopy [get]
func RestGetInfraReqFromInfra(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	templateName := c.QueryParam("template")

	if templateName != "" {
		// Extract and create template
		result, err := infra.ExtractAndCreateTemplate(nsId, infraId, templateName)
		return clientManager.EndRequestWithLog(c, err, result)
	}

	result, err := infra.ExtractInfraDynamicReqFromInfraInfo(nsId, infraId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetAllInfraResponse is a response structure for RestGetAllInfra
type RestGetAllInfraResponse struct {
	Infra []model.InfraInfo `json:"infra"`
}

// RestGetAllInfraStatusResponse is a response structure for RestGetAllInfraStatus
type RestGetAllInfraStatusResponse struct {
	Infra []model.InfraStatusInfo `json:"infra"`
}

// RestGetAllInfra godoc
// @ID GetAllInfra
// @Summary List all Infras or Infras' ID
// @Description List all Infras or Infras' ID
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id, simple, status)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllInfraResponse,[SIMPLE]=RestGetAllInfraResponse,[ID]=model.IdList,[STATUS]=RestGetAllInfraStatusResponse} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra [get]
func RestGetAllInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	if option == "id" {
		// return Infra IDs
		content := model.IdList{}
		var err error
		content.IdList, err = infra.ListInfraId(nsId)
		return clientManager.EndRequestWithLog(c, err, content)
	} else if option == "status" {
		// return Infra Status objects (diffent with Infra objects)
		result, err := infra.ListInfraStatus(nsId)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content := RestGetAllInfraStatusResponse{}
		content.Infra = result
		return clientManager.EndRequestWithLog(c, err, content)
	} else if option == "simple" {
		// Infra in simple (without Node information)
		result, err := infra.ListInfraInfo(nsId, option)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content := RestGetAllInfraResponse{}
		content.Infra = result
		return clientManager.EndRequestWithLog(c, err, content)
	} else {
		// Infra in detail (with status information)
		result, err := infra.ListInfraInfo(nsId, "status")
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, nil)
		}
		content := RestGetAllInfraResponse{}
		content.Infra = result
		return clientManager.EndRequestWithLog(c, err, content)
	}
}

/*
	function RestPutInfra not yet implemented

// RestPutInfra godoc
// @ID PutInfra
// @Summary Update Infra
// @Description Update Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param infraInfo body InfraInfo true "Details for an Infra object"
// @Success 200 {object} InfraInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId} [put]
func RestPutInfra(c echo.Context) error {
	return nil
}
*/

// RestDelInfra godoc
// @ID DelInfra
// @Summary Delete Infra
// @Description Delete Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param option query string false "Option for delete Infra (support force delete)" Enums(terminate,force)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId} [delete]
func RestDelInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	option := c.QueryParam("option")

	content, err := infra.DelInfra(nsId, infraId, option)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelAllInfra godoc
// @ID DelAllInfra
// @Summary Delete all Infras
// @Description Delete all Infras
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option for delete all Infras (support force object delete, terminate before delete)" Enums(force, terminate)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra [delete]
func RestDelAllInfra(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	message, err := infra.DelAllInfra(nsId, option)
	result := model.SimpleMsg{Message: message}
	return clientManager.EndRequestWithLog(c, err, result)
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention needs to be revised.

// RestGetInfraNode godoc
// @ID GetInfraNode
// @Summary Get node in specified Infra
// @Description Get node in specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Param option query string false "Option for Infra" Enums(default, status, idsInDetail, accessinfo)
// @Param accessInfoOption query string false "(For option=accessinfo) accessInfoOption (showSshKey)"
// @success 200 {object} JSONResult{[DEFAULT]=model.NodeInfo,[STATUS]=model.NodeStatusInfo,[IDNAME]=model.IdNameInDetailInfo} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/node/{nodeId} [get]
func RestGetInfraNode(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	option := c.QueryParam("option")
	accessInfoOption := c.QueryParam("accessInfoOption")

	switch option {
	case "status":
		result, err := infra.GetInfraNodeStatus(nsId, infraId, nodeId, false)
		return clientManager.EndRequestWithLog(c, err, result)

	case "idsInDetail":
		result, err := infra.GetNodeIdNameInDetail(nsId, infraId, nodeId)
		return clientManager.EndRequestWithLog(c, err, result)

	case "accessinfo":
		result, err := infra.GetInfraNodeAccessInfo(nsId, infraId, nodeId, accessInfoOption)
		return clientManager.EndRequestWithLog(c, err, result)

	default:
		result, err := infra.GetNodeObject(nsId, infraId, nodeId)
		return clientManager.EndRequestWithLog(c, err, result)
	}
}

/* RestPutInfraNode function not yet implemented
// RestPutSshKey godoc
// @ID PutSshKey
// @Summary Update Infra
// @Description Update Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Param nodeInfo body model.NodeInfo true "Details for a node object"
// @Success 200 {object} model.NodeInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/node/{nodeId} [put]
func RestPutInfraNode(c echo.Context) error {
	return nil
}
*/

// RestDelInfraNode godoc
// @ID DelInfraNode
// @Summary Delete node in specified Infra
// @Description Delete node in specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Param option query string false "Option for delete node (support force delete)" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/node/{nodeId} [delete]
func RestDelInfraNode(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")
	option := c.QueryParam("option")

	err := infra.DelInfraNode(nsId, infraId, nodeId, option)
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("Failed to delete the Node info")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := map[string]string{"message": "Deleted the Node info"}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestDeregisterInfraNode godoc
// @ID DeregisterInfraNode
// @Summary Deregister node in specified Infra
// @Description Deregister node from Spider and TB without deleting the actual CSP resource
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodeId path string true "Node ID" default(g1-1)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/deregisterResource/infra/{infraId}/node/{nodeId} [delete]
func RestDeregisterInfraNode(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodeId := c.Param("nodeId")

	err := infra.DeregisterInfraNode(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result := map[string]string{"message": "Deregistered the Node info (CSP resource remains intact)"}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetInfraGroupNodes godoc
// @ID GetInfraGroupNodes
// @Summary List nodes with a NodeGroup label in a specified Infra
// @Description List nodes with a NodeGroup label in a specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param nodegroupId path string true "nodeGroup ID" default(g1)
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nodegroup/{nodegroupId} [get]
func RestGetInfraGroupNodes(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	nodegroupId := c.Param("nodegroupId")
	//option := c.QueryParam("option")

	content := model.IdList{}
	var err error
	content.IdList, err = infra.ListNodeByNodeGroup(nsId, infraId, nodegroupId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetInfraGroupIds godoc
// @ID GetInfraGroupIds
// @Summary List NodeGroup IDs in a specified Infra
// @Description List NodeGroup IDs in a specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/nodegroup [get]
func RestGetInfraGroupIds(c echo.Context) error {

	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	//option := c.QueryParam("option")

	content := model.IdList{}
	var err error
	content.IdList, err = infra.ListNodeGroupId(nsId, infraId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetInfraClusters godoc
// @ID GetInfraClusters
// @Summary List implicit clusters in a specified Infra
// @Description List implicit clusters synthesized at query-time from NodeGroups/Nodes in a specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.InfraClusterList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/cluster [get]
func RestGetInfraClusters(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	result, err := infra.ListInfraClusterInfo(nsId, infraId)
	if err != nil {
		if isInfraClusterNotFoundError(err) {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: err.Error()})
		}
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content := model.InfraClusterList{Cluster: result}
	return clientManager.EndRequestWithLog(c, nil, content)
}

// RestGetInfraCluster godoc
// @ID GetInfraCluster
// @Summary Get implicit cluster in a specified Infra
// @Description Get a single implicit cluster synthesized at query-time from NodeGroups/Nodes in a specified Infra
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param clusterId path string true "Cluster ID" default(vnet01)
// @Success 200 {object} model.InfraClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/cluster/{clusterId} [get]
func RestGetInfraCluster(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")
	clusterId := c.Param("clusterId")

	result, err := infra.GetInfraClusterInfo(nsId, infraId, clusterId)
	if err != nil {
		if isInfraClusterNotFoundError(err) {
			return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: err.Error()})
		}
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	return clientManager.EndRequestWithLog(c, err, result)
}

// RestGetInfraAssociatedResources godoc
// @ID GetInfraAssociatedResources
// @Summary Get associated resource ID list for a given Infra
// @Description Get associated resource ID list for a given Infra (VNet, Subnet, SecurityGroup, SSHKey, etc.)
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Success 200 {object} model.InfraAssociatedResourceList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/associatedResources [get]
func RestGetInfraAssociatedResources(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	result, err := infra.GetInfraAssociatedResources(nsId, infraId)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPutInfraAssociatedSecurityGroups godoc
// @ID PutInfraAssociatedSecurityGroups
// @Summary Update all Security Groups associated with a given Infra
// @Description Update all Security Groups associated with a given Infra. The firewall rules of all Security Groups will be synchronized to match the requested set.
// @Tags [MC-Infra] Infra Provisioning and Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param infraId path string true "Infra ID" default(infra01)
// @Param securityGroupInfo body model.SecurityGroupUpdateReq true "Details for SecurityGroup update (only firewallRules field is used for update)"
// @Success 200 {array} model.SecurityGroupInfo "Updated Security Group info list with synchronized firewall rules"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Param x-credential-holder header string false "Credential holder ID for selecting which credentials to use (default: system default holder)"
// @Router /ns/{nsId}/infra/{infraId}/associatedSecurityGroups [put]
// @Summary Update all Security Groups associated with a given Infra (Synchronize Firewall Rules)
// @Description Update all Security Groups associated with a given Infra. The firewall rules of all associated Security Groups will be synchronized to match the requested set.
// @Description
// @Description This API will add missing rules and delete extra rules so that each Security Group's rules become identical to the requested set.
// @Description Only firewall rules are updated; other metadata (name, description, etc.) is not changed.
// @Description
// @Description Usage:
// @Description Use this API to update (synchronize) the firewall rules of all Security Groups associated with the specified Infra. The rules in the request body will become the only rules in each Security Group after the operation.
// @Description - All existing rules not present in the request will be deleted.
// @Description - All rules in the request that do not exist will be added.
// @Description - If a rule exists but differs in CIDR or port range, it will be replaced.
// @Description - Special protocols (ICMP, etc.) are handled in the same way.
// @Description
// @Description Notes:
// @Description - "Ports" field supports single port ("22"), port range ("80-100"), and multiple ports/ranges ("22,80-100,443").
// @Description - The valid port number range is 0 to 65535 (inclusive).
// @Description - "Protocol" can be TCP, UDP, ICMP, etc. (as supported by the cloud provider).
// @Description - "Direction" must be either "inbound" or "outbound".
// @Description - "CIDR" is the allowed IP range.
// @Description - All existing rules not in the request (including default ICMP, etc.) will be deleted.
// @Description - Metadata (name, description, etc.) is not changed.
// @Success 200 {object} model.RestWrapperSecurityGroupUpdateResponse "Updated Security Group info list with synchronized firewall rules"
func RestPutInfraAssociatedSecurityGroups(c echo.Context) error {
	nsId := c.Param("nsId")
	infraId := c.Param("infraId")

	req := &model.SecurityGroupUpdateReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	assocList := model.InfraAssociatedResourceList{}
	assocList, err := infra.GetInfraAssociatedResources(nsId, infraId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Use the new parallel processing function
	response := resource.UpdateMultipleFirewallRules(nsId, assocList.SecurityGroupIds, req.FirewallRules)

	return clientManager.EndRequestWithLog(c, nil, response)
}
