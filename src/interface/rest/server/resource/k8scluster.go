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

// Package resource is to handle REST API for resource
package resource

import (
	"fmt"
	"io"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RestGetAvailableK8sVersion func is a rest api wrapper for GetAvailableK8sVersion.
// RestGetAvailableK8sVersion godoc
// @ID GetAvailableK8sVersion
// @Summary Get available kubernetes cluster version
// @Description Get available kubernetes cluster version
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param providerName query string true "Name of the CSP to retrieve"
// @Param regionName query string true "Name of region to retrieve"
// @Success 200 {object} model.K8sClusterVersionDetailAvailable
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /availableK8sVersion [get]
func RestGetAvailableK8sVersion(c echo.Context) error {

	providerName := c.QueryParam("providerName")
	regionName := c.QueryParam("regionName")

	content, err := common.GetAvailableK8sVersion(providerName, regionName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetAvailableK8sNodeImage func is a rest api wrapper for GetAvailableK8sNodeImage.
// RestGetAvailableK8sNodeImage godoc
// @ID GetAvailableK8sNodeImage
// @Summary (UNDER DEVELOPMENT!!!) Get available kubernetes cluster node image
// @Description (UNDER DEVELOPMENT!!!) Get available kubernetes cluster node image
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param providerName query string true "Name of the CSP to retrieve"
// @Param regionName query string true "Name of region to retrieve"
// @Success 200 {object} model.K8sClusterNodeImageDetailAvailable
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /availableK8sNodeImage [get]
func RestGetAvailableK8sNodeImage(c echo.Context) error {

	providerName := c.QueryParam("providerName")
	regionName := c.QueryParam("regionName")

	content, err := common.GetAvailableK8sNodeImage(providerName, regionName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestCheckK8sNodeGroupsOnK8sCreation func is a rest api wrapper for GetModelK8sNodeGroupsOnK8sCreation.
// RestCheckK8sNodeGroupsOnK8sCreation godoc
// @ID CheckK8sNodeGroupsOnK8sCreation
// @Summary Check whether nodegroups are required during the K8sCluster creation
// @Description Check whether nodegroups are required during the K8sCluster creation
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param providerName query string true "Name of the CSP to retrieve"
// @Success 200 {object} model.K8sClusterNodeGroupsOnCreation
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /checkK8sNodeGroupsOnK8sCreation [get]
func RestCheckK8sNodeGroupsOnK8sCreation(c echo.Context) error {

	providerName := c.QueryParam("providerName")

	content, err := common.GetModelK8sNodeGroupsOnK8sCreation(providerName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestCheckK8sNodeImageDesignation func is a rest api wrapper for GetK8sNodeImageDesignation.
// RestCheckK8sNodeImageDesignation godoc
// @ID CheckK8sNodeImageDesignation
// @Summary Check whether node image designation is possible to create a K8sCluster
// @Description Check whether node image designation is possible to create a K8sCluster
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param providerName query string true "Name of the CSP to retrieve"
// @Success 200 {object} model.K8sClusterNodeImageDesignation
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /checkK8sNodeImageDesignation [get]
func RestCheckK8sNodeImageDesignation(c echo.Context) error {

	providerName := c.QueryParam("providerName")

	content, err := common.GetModelK8sNodeImageDesignation(providerName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetRequiredK8sSubnetCount func is a rest api wrapper for GetModelK8sRequiredSubnetCount.
// RestGetRequiredK8sSubnetCount godoc
// @ID GetRequiredK8sSubnetCount
// @Summary Get the required subnet count to create a K8sCluster
// @Description Get the required subnet count to create a K8sCluster
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param providerName query string true "Name of the CSP to retrieve"
// @Success 200 {object} model.K8sClusterRequiredSubnetCount
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /requiredK8sSubnetCount [get]
func RestGetRequiredK8sSubnetCount(c echo.Context) error {

	providerName := c.QueryParam("providerName")

	content, err := common.GetModelK8sRequiredSubnetCount(providerName)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostK8sCluster func is a rest api wrapper for CreateK8sCluster.
// RestPostK8sCluster godoc
// @ID PostK8sCluster
// @Summary Create K8sCluster
// @Description Create K8sCluster<br>Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1614
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option: [required params for register] connectionName, name, cspResourceId" Enums(register)
// @Param k8sClusterReq body model.TbK8sClusterReq true "Details of the K8sCluster object"
// @Success 200 {object} model.TbK8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster [post]
func RestPostK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &model.TbK8sClusterReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msg("[POST K8sCluster]")

	content, err := resource.CreateK8sCluster(nsId, u, optionFlag)

	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusCreated, content)
}

/*
	function RestPutK8sCluster not yet implemented

// RestPutK8sCluster godoc
// @ID PutK8sCluster
// @Summary Update K8sCluster
// @Description Update K8sCluster
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sClusterInfo body model.TbK8sClusterInfo true "Details of the K8sCluster object"
// @Success 200 {object} model.TbK8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId} [put]
*/
func RestPutK8sCluster(c echo.Context) error {
	// nsId := c.Param("nsId")

	return nil
}

// RestPostK8sNodeGroup func is a rest api wrapper for AddK8sNodeGroup.
// RestPostK8sNodeGroup godoc
// @ID PostK8sNodeGroup
// @Summary Add a K8sNodeGroup
// @Description Add a K8sNodeGroup
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sNodeGroupReq body model.TbK8sNodeGroupReq true "Details of the K8sNodeGroup object"
// @Success 200 {object} model.TbK8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup [post]
func RestPostK8sNodeGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	u := &model.TbK8sNodeGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msg("[POST K8sNodeGroup]")

	content, err := resource.AddK8sNodeGroup(nsId, k8sClusterId, u)

	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusCreated, content)
}

// RestDeleteK8sNodeGroup func is a rest api wrapper for RemoveK8sNodeGroup.
// RestDeleteK8sNodeGroup godoc
// @ID DeleteK8sNodeGroup
// @Summary Remove a K8sNodeGroup
// @Description Remove a K8sNodeGroup
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sNodeGroupName path string true "K8sNodeGroup Name" default(k8sng01)
// @Param option query string false "Option for K8sNodeGroup deletion" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName} [delete]
func RestDeleteK8sNodeGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")
	k8sNodeGroupName := c.Param("k8sNodeGroupName")

	optionFlag := c.QueryParam("option")

	res, err := resource.RemoveK8sNodeGroup(nsId, k8sClusterId, k8sNodeGroupName, optionFlag)
	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	var mapA map[string]string
	if res == true {
		mapA = map[string]string{"message": "The K8sNodeGroup " + k8sNodeGroupName + " in K8sCluster " + k8sClusterId + " has been deleted"}
	} else { // res == false
		mapA = map[string]string{"message": "The K8sNodeGroup " + k8sNodeGroupName + " in K8sCluster " + k8sClusterId + " is not deleted"}
	}

	return c.JSON(http.StatusOK, &mapA)
}

// RestPutSetK8sNodeGroupAutoscaling func is a rest api wrapper for SetK8sNodeGroupAutoscaling.
// RestPutSetK8sNodeGroupAutoscaling godoc
// @ID PutSetK8sNodeGroupAutoscaling
// @Summary Set a K8sNodeGroup's Autoscaling On/Off
// @Description Set a K8sNodeGroup's Autoscaling On/Off
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sNodeGroupName path string true "K8sNodeGroup Name" default(k8sng01)
// @Param setK8sNodeGroupAutoscalingReq body model.TbSetK8sNodeGroupAutoscalingReq true "Details of the TbSetK8sNodeGroupAutoscalingReq object"
// @Success 200 {object} model.TbSetK8sNodeGroupAutoscalingRes
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/onAutoscaling [put]
func RestPutSetK8sNodeGroupAutoscaling(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")
	k8sNodeGroupName := c.Param("k8sNodeGroupName")

	u := &model.TbSetK8sNodeGroupAutoscalingReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msg("[PUT K8s Set AutoScaling]")

	content, err := resource.SetK8sNodeGroupAutoscaling(nsId, k8sClusterId, k8sNodeGroupName, u)

	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}

// RestPutChangeK8sNodeGroupAutoscaleSize func is a rest api wrapper for ChangeK8sNodeGroupAutoscaleSize.
// RestPutChangeK8sNodeGroupAutoscaleSize godoc
// @ID PutChangeK8sNodeGroupAutoscaleSize
// @Summary Change a K8sNodeGroup's Autoscale Size
// @Description Change a K8sNodeGroup's Autoscale Size
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sNodeGroupName path string true "K8sNodeGroup Name" default(k8sng01)
// @Param changeK8sNodeGroupAutoscaleSizeReq body model.TbChangeK8sNodeGroupAutoscaleSizeReq true "Details of the TbChangeK8sNodeGroupAutoscaleSizeReq object"
// @Success 200 {object} model.TbChangeK8sNodeGroupAutoscaleSizeRes
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroup/{k8sNodeGroupName}/autoscaleSize [put]
func RestPutChangeK8sNodeGroupAutoscaleSize(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")
	k8sNodeGroupName := c.Param("k8sNodeGroupName")

	u := &model.TbChangeK8sNodeGroupAutoscaleSizeReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msg("[PUT K8s Change AutoScale Size]")

	content, err := resource.ChangeK8sNodeGroupAutoscaleSize(nsId, k8sClusterId, k8sNodeGroupName, u)

	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}

// RestGetK8sCluster func is a rest api wrapper for GetK8sCluster.
// RestGetK8sCluster godoc
// @ID GetK8sCluster
// @Summary Get K8sCluster
// @Description Get K8sCluster
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Success 200 {object} model.TbK8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId} [get]
func RestGetK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	res, err := resource.GetK8sCluster(nsId, k8sClusterId)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find the K8sCluster " + k8sClusterId + ": " + err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

// Response structure for RestGetAllK8sCluster
type RestGetAllK8sClusterResponse struct {
	K8sCluster []model.TbK8sClusterInfo `json:"cluster"`
}

// RestGetAllK8sCluster godoc
// @ID GetAllK8sCluster
// @Summary List all K8sClusters or K8sClusters' ID
// @Description List all K8sClusters or K8sClusters' ID
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspResourceName)"
// @Param filterVal query string false "Field value for filtering (ex: default-alibaba-ap-northeast-2-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllK8sClusterResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster [get]
func RestGetAllK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")

	if optionFlag == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = resource.ListK8sClusterId(nsId)
		if err != nil {
			mapA := map[string]string{"message": "Failed to list K8sClusters' ID; " + err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else {

		resourceList, err := resource.ListK8sCluster(nsId, filterKey, filterVal)
		if err != nil {
			mapA := map[string]string{"message": "Failed to list K8sClusters; " + err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		var content struct {
			K8sCluster []model.TbK8sClusterInfo `json:"K8sClusterInfo"`
		}

		content.K8sCluster = resourceList.([]model.TbK8sClusterInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	}
}

// RestDeleteK8sCluster func is a rest api wrapper for DeleteK8sCluster.
// RestDeleteK8sCluster godoc
// @ID DeleteK8sCluster
// @Summary Delete K8sCluster
// @Description Delete K8sCluster
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param option query string false "Option for K8sCluster deletion" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId} [delete]
func RestDeleteK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	optionFlag := c.QueryParam("option")

	res, err := resource.DeleteK8sCluster(nsId, k8sClusterId, optionFlag)
	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	var mapA map[string]string
	if res == true {
		mapA = map[string]string{"message": "The K8sCluster " + k8sClusterId + " has been deleted"}
	} else { // res == false
		mapA = map[string]string{"message": "The K8sCluster " + k8sClusterId + " is not deleted"}
	}

	return c.JSON(http.StatusOK, &mapA)
}

// RestDeleteAllK8sCluster godoc
// @ID DeleteAllK8sCluster
// @Summary Delete all K8sClusters
// @Description Delete all K8sClusters
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Param option query string false "Option for K8sCluster deletion" Enums(force)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster [delete]
func RestDeleteAllK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")
	subString := c.QueryParam("match")

	output, err := resource.DeleteAllK8sCluster(nsId, subString, optionFlag)
	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	return c.JSON(http.StatusOK, output)
}

// RestPutUpgradeK8sCluster func is a rest api wrapper for UpgradeK8sCluster.
// RestPutUpgradeK8sCluster godoc
// @ID PutUpgradeK8sCluster
// @Summary Upgrade a K8sCluster's version
// @Description Upgrade a K8sCluster's version
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param upgradeK8sClusterReq body model.TbUpgradeK8sClusterReq true "Details of the TbUpgradeK8sClusterReq object"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/upgrade [put]
func RestPutUpgradeK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	u := &model.TbUpgradeK8sClusterReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msg("[PUT Upgrade K8sCluster]")

	content, err := resource.UpgradeK8sCluster(nsId, k8sClusterId, u)

	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}

// RestPostK8sClusterDynamicCheckRequest godoc
// @ID PostK8sClusterDynamicCheckRequest
// @Summary Check available ConnectionConfig list for creating K8sCluster Dynamically
// @Description Check available ConnectionConfig list before create K8sCluster Dynamically from common spec and image
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param k8sClusterConnectionConfigCandidatesReq body model.K8sClusterConnectionConfigCandidatesReq true "Details for K8sCluster dynamic request information"
// @Success 200 {object} model.CheckK8sClusterDynamicReqInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /k8sClusterDynamicCheckRequest [post]
func RestPostK8sClusterDynamicCheckRequest(c echo.Context) error {

	req := &model.K8sClusterConnectionConfigCandidatesReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CheckK8sClusterDynamicReq(req)
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestPostK8sClusterDynamic godoc
// @ID PostK8sClusterDynamic
// @Summary Create K8sCluster Dynamically
// @Description Create K8sCluster Dynamically from common spec and image
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterDyanmicReq body model.TbK8sClusterDynamicReq true "Request body to provision K8sCluster dynamically. <br> Must include commonSpec and commonImage info. <br> (ex: {name: k8scluster01, commonImage: azure+koreacentral+ubuntu22.04, commonSpec: azure+koreacentral+Standard_B2s}]}) <br> You can use /k8sClusterRecommendNode and /k8sClusterDynamicCheckRequest to get it. <br> Check the guide: https://github.com/cloud-barista/cb-tumblebug/discussions/1913"
// @Param option query string false "Option for K8sCluster creation" Enums(hold)
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbK8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sClusterDynamic [post]
func RestPostK8sClusterDynamic(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	optionFlag := c.QueryParam("option")

	req := &model.TbK8sClusterDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	result, err := infra.CreateK8sClusterDynamic(reqID, nsId, req, optionFlag)
	if err != nil {
		log.Error().Err(err).Msg("failed to create K8sCluster dynamically")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestPostK8sNodeGroupDynamic godoc
// @ID PostK8sNodeGroupDynamic
// @Summary Create K8sNodeGroup Dynamically
// @Description Create K8sNodeGroup Dynamically from common spec and image
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sNodeGroupDynamicReq body model.TbK8sNodeGroupDynamicReq true "Request body to provision K8sNodeGroup dynamically. <br> Must include commonSpec and commonImage info. <br> (ex: {name: k8sng01, commonImage: azure+koreacentral+ubuntu22.04, commonSpec: azure+koreacentral+Standard_B2s}]}) <br> You can use /k8sClusterRecommendNode and /k8sClusterDynamicCheckRequest to get it. <br> Check the guide: https://github.com/cloud-barista/cb-tumblebug/discussions/1913"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbK8sNodeGroupInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/k8sNodeGroupDynamic [post]
func RestPostK8sNodeGroupDynamic(c echo.Context) error {
	reqID := c.Request().Header.Get(echo.HeaderXRequestID)

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	req := &model.TbK8sNodeGroupDynamicReq{}
	if err := c.Bind(req); err != nil {
		log.Warn().Err(err).Msg("invalid request")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	log.Debug().Msgf("reqID: %s, nsId: %s, k8sClusterId: %s, req: %v\n", reqID, nsId, k8sClusterId, req)
	result, err := infra.CreateK8sNodeGroupDynamic(reqID, nsId, k8sClusterId, req)
	if err != nil {
		log.Error().Err(err).Msg("failed to create K8sNodeGroup dynamically")
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	return c.JSON(http.StatusOK, result)
}

// RestGetControlK8sCluster godoc
// @ID GetControlK8sCluster
// @Summary Control the creation of K8sCluster (continue, withdraw)
// @Description Control the creation of K8sCluster (continue, withdraw)
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param action query string true "Action to K8sCluster" Enums(continue, withdraw)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/control/k8sCluster/{k8sClusterId} [get]
func RestGetControlK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")

	action := c.QueryParam("action")
	returnObj := model.SimpleMsg{}

	if action == "continue" || action == "withdraw" {

		resultString, err := resource.HandleK8sClusterAction(nsId, k8sClusterId, action)
		if err != nil {
			return clientManager.EndRequestWithLog(c, err, returnObj)
		}
		returnObj.Message = resultString
		return clientManager.EndRequestWithLog(c, err, returnObj)

	} else {
		err := fmt.Errorf("'action' should be one of these: continue, withdraw")
		return clientManager.EndRequestWithLog(c, err, returnObj)
	}
}

// RestRecommendK8sNode godoc
// @ID RecommendK8sNode
// @Summary Recommend K8sCluster's Node plan (filter and priority)
// @Description Recommend K8sCluster's Node plan (filter and priority) Find details from https://github.com/cloud-barista/cb-tumblebug/discussions/1234
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Param recommendSpecReq body model.RecommendSpecReq false "Recommend K8sCluster's Node plan (filter and priority)"
// @Success 200 {object} []model.TbSpecInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /k8sClusterRecommendNode [post]
func RestRecommendK8sNode(c echo.Context) error {

	nsId := model.SystemCommonNs

	u := &model.RecommendSpecReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := infra.RecommendK8sNode(nsId, *u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostCmdK8sCluster godoc
// @ID PostCmdK8sCluster
// @Summary Send a command to specified Container in K8sCluster
// @Description Send a command to specified Container in K8sCluster
// @Description [note] This feature is not intended for general use
// @Description This API is provided as an exceptional and limited function for specific purposes such as migration.
// @Description Kubernetes resource information required as input for this API is not currently provided, and its availability in the future is uncertain.
// @Tags [Kubernetes] Cluster's Container Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sClusterNamespace query string true "Namespace in K8sCluster to apply the command" default(default)
// @Param k8sClusterPodName query string true "Pod Name in K8sCluster to apply the command" default(mypod)
// @Param k8sClusterContainerName query string false "Container Name in K8sCluster to apply the command"
// @Param k8sClusterContainerCmdReq body model.TbK8sClusterContainerCmdReq true "K8sCluster's Container Command Request"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbK8sClusterContainerCmdResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/cmd/k8sCluster/{k8sClusterId} [post]
func RestPostCmdK8sCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")
	k8sClusterNamespace := c.QueryParam("k8sClusterNamespace")
	k8sClusterPodName := c.QueryParam("k8sClusterPodName")
	k8sClusterContainerName := c.QueryParam("k8sClusterContainerName")

	req := &model.TbK8sClusterContainerCmdReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.RemoteCommandToK8sClusterContainer(nsId, k8sClusterId, k8sClusterNamespace, k8sClusterPodName, k8sClusterContainerName, req)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostFileToK8sCluster godoc
// @ID PostFileToK8sCluster
// @Summary Transfer a file to specified Container in K8sCluster
// @Description Transfer a file to specified Container in K8sCluster. The tar command is required in the container.
// @Description [note] This feature is not intended for general use
// @Description This API is provided as an exceptional and limited function for specific purposes such as migration.
// @Description Kubernetes resource information required as input for this API is not currently provided, and its availability in the future is uncertain.
// @Tags [Kubernetes] Cluster's Container Remote Command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param k8sClusterNamespace query string true "Namespace in K8sCluster to apply the command" default(default)
// @Param k8sClusterPodName query string true "Pod Name in K8sCluster to apply the command" default(mypod)
// @Param k8sClusterContainerName query string false "Container Name in K8sCluster to apply the command"
// @Param path formData string true "Target path where the file will be stored" default(/tmp)
// @Param file formData file true "The file to be uploaded (Max 10MB)"
// @Param x-request-id header string false "Custom request ID"
// @Success 200 {object} model.TbK8sClusterContainerCmdResults
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/transferFile/k8sCluster/{k8sClusterId} [post]
func RestPostFileToK8sCluster(c echo.Context) error {
	nsId := c.Param("nsId")
	k8sClusterId := c.Param("k8sClusterId")
	k8sClusterNamespace := c.QueryParam("k8sClusterNamespace")
	k8sClusterPodName := c.QueryParam("k8sClusterPodName")
	k8sClusterContainerName := c.QueryParam("k8sClusterContainerName")
	targetPath := c.FormValue("path")

	if targetPath == "" {
		err := fmt.Errorf("target path is required")
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Validate the file
	file, err := c.FormFile("file")
	if err != nil {
		err = fmt.Errorf("failed to read the file: %w", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// File size validation
	fileSizeLimit := int64(10 * 1024 * 1024) // (10MB limit)
	if file.Size > fileSizeLimit {
		err := fmt.Errorf("file too large, max size is %v", fileSizeLimit)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Open the file and read it into memory
	src, err := file.Open()
	if err != nil {
		err = fmt.Errorf("failed to open the file: %w", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	defer src.Close()

	// Read the file into memory
	bytesFile, err := io.ReadAll(src)
	if err != nil {
		err = fmt.Errorf("failed to read the file: %w", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.TransferFileToK8sClusterContainer(nsId, k8sClusterId, k8sClusterNamespace, k8sClusterPodName, k8sClusterContainerName, bytesFile, file.Filename, targetPath)
	if err != nil {
		err = fmt.Errorf("failed to transfer the file to k8sCluster: %v", err)
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	// Return the result
	return clientManager.EndRequestWithLog(c, err, content)
}
