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

// Package mcis is to handle REST API for mcis
package mcis

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"
)

// RestPostCluster godoc
// @Summary Create Cluster
// @Description Create Cluster
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option: [required params for register] connectionName, name, cspClusterId" Enums(register)
// @Param clusterReq body mcis.TbClusterReq true "Details of the Cluster object"
// @Success 200 {object} mcis.TbClusterInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster [post]
func RestPostCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &mcis.TbClusterReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[POST Cluster]")

	content, err := mcis.CreateCluster(nsId, u, optionFlag)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusCreated, content)
}

/*
	function RestPutCluster not yet implemented

// RestPutCluster godoc
// @Summary Update Cluster
// @Description Update Cluster
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID" default(c1)
// @Param nlbInfo body mcis.TbClusterInfo true "Details of the Cluster object"
// @Success 200 {object} mcis.TbClusterInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId} [put]
*/
func RestPutCluster(c echo.Context) error {
	// nsId := c.Param("nsId")

	return nil
}

// RestPostNodeGroup godoc
// @Summary Add a NodeGroup
// @Description Add a NodeGroup
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Param nodeGroupReq body mcis.TbNodeGroupReq true "Details of the NodeGroup object"
// @Success 200 {object} mcis.TbClusterInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId}/nodegroup [post]
func RestPostNodeGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")

	u := &mcis.TbNodeGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[POST NodeGroup]")

	content, err := mcis.AddNodeGroup(nsId, clusterId, u)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusCreated, content)
}

// RestDeleteNodeGroup godoc
// @Summary Remove a NodeGroup
// @Description Remove a NodeGroup
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Param nodeGroupName path string true "NodeGroup Name"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId}/nodegroup/{nodeGroupName} [delete]
func RestDeleteNodeGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")
	nodeGroupName := c.Param("nodeGroupName")

	forceFlag := c.QueryParam("force")

	res, err := mcis.RemoveNodeGroup(nsId, clusterId, nodeGroupName, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	var mapA map[string]string
	if res == true {
		mapA = map[string]string{"message": "The NodeGroup " + nodeGroupName + " in Cluster " + clusterId + " has been deleted"}
	} else { // res == false
		mapA = map[string]string{"message": "The NodeGroup " + nodeGroupName + " in Cluster " + clusterId + " is not deleted"}
	}

	return c.JSON(http.StatusOK, &mapA)
}

// RestPutSetAutoscaling godoc
// @Summary Set a NodeGroup's Autoscaling On/Off
// @Description Set a NodeGroup's Autoscaling On/Off
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Param nodeGroupName path string true "NodeGroup Name"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId}/nodegroup/{nodeGroupName}/onautoscaling [put]
func RestPutSetAutoscaling(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")
	nodeGroupName := c.Param("nodeGroupName")

	u := &mcis.TbSetAutoscalingReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[PUT Set AutoScaling]")

	content, err := mcis.SetAutoscaling(nsId, clusterId, nodeGroupName, u)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}

// RestPutChangeAutoscaleSize godoc
// @Summary Change a NodeGroup's Autoscale Size
// @Description Change a NodeGroup's Autoscale Size
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Param nodeGroupName path string true "NodeGroup Name"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId}/nodegroup/{nodeGroupName}/autoscalesize [put]
func RestPutChangeAutoscaleSize(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")
	nodeGroupName := c.Param("nodeGroupName")

	u := &mcis.TbChangeAutoscaleSizeReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[PUT Change AutoScale Size]")

	content, err := mcis.ChangeAutoscaleSize(nsId, clusterId, nodeGroupName, u)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}

// RestGetCluster godoc
// @Summary Get Cluster
// @Description Get Cluster
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID" default(c1)
// @Success 200 {object} mcis.TbClusterInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId} [get]
func RestGetCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")

	res, err := mcis.GetCluster(nsId, clusterId)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find the Cluster " + clusterId + ": " + err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

// Response structure for RestGetAllCluster
type RestGetAllClusterResponse struct {
	Cluster []mcis.TbClusterInfo `json:"cluster"`
}

// RestGetAllCluster godoc
// @Summary List all Clusters or Clusters' ID
// @Description List all Clusters or Clusters' ID
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: cspClusterName)"
// @Param filterVal query string false "Field value for filtering (ex: ns01-alibaba-ap-northeast-1-vpc)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllClusterResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster [get]
func RestGetAllCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")

	if optionFlag == "id" {
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListClusterId(nsId)
		if err != nil {
			mapA := map[string]string{"message": "Failed to list Clusters' ID; " + err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else {

		resourceList, err := mcis.ListCluster(nsId, filterKey, filterVal)
		if err != nil {
			mapA := map[string]string{"message": "Failed to list Clusters; " + err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		var content struct {
			Cluster []mcis.TbClusterInfo `json:"ClusterInfo"`
		}

		content.Cluster = resourceList.([]mcis.TbClusterInfo) // type assertion (interface{} -> array)
		return c.JSON(http.StatusOK, &content)
	}
}

// RestDeleteCluster godoc
// @Summary Delete Cluster
// @Description Delete Cluster
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId} [delete]
func RestDeleteCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")

	forceFlag := c.QueryParam("force")

	res, err := mcis.DeleteCluster(nsId, clusterId, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	var mapA map[string]string
	if res == true {
		mapA = map[string]string{"message": "The Cluster " + clusterId + " has been deleted"}
	} else { // res == false
		mapA = map[string]string{"message": "The Cluster " + clusterId + " is not deleted"}
	}

	return c.JSON(http.StatusOK, &mapA)
}

// RestDeleteAllCluster godoc
// @Summary Delete all Clusters
// @Description Delete all Clusters
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster [delete]
func RestDeleteAllCluster(c echo.Context) error {

	nsId := c.Param("nsId")

	forceFlag := c.QueryParam("force")
	subString := c.QueryParam("match")

	output, err := mcis.DeleteAllCluster(nsId, subString, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	return c.JSON(http.StatusOK, output)
}

// RestPutClusterUpgrade godoc
// @Summary Upgrade a Cluster's version
// @Description Upgrade a Cluster's version
// @Tags [Infra resource] Cluster management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(ns01)
// @Param clusterId path string true "Cluster ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cluster/{clusterId}/upgrade [put]
func RestPutUpgradeCluster(c echo.Context) error {

	nsId := c.Param("nsId")
	clusterId := c.Param("clusterId")

	u := &mcis.TbUpgradeClusterReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[PUT Upgrade Cluster]")

	content, err := mcis.UpgradeCluster(nsId, clusterId, u)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, content)
}
