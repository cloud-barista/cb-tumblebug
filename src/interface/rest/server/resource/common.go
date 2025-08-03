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
	"strings"

	"github.com/labstack/echo/v4"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
)

// JSONResult is a dummy struct for Swagger annotations.
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

// RestDelAllResources is a common function to handle 'DelAllResources' REST API requests.
// Dummy functions for Swagger exist in [resource/*.go]
func RestDelAllResources(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	forceFlag := c.QueryParam("force")
	subString := c.QueryParam("match")

	content, err := resource.DelAllResources(nsId, resourceType, subString, forceFlag)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelResource is a common function to handle 'DelResource' REST API requests.
// Dummy functions for Swagger exist in [resource/*.go]
func RestDelResource(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	resourceId := c.Param("resourceId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	forceFlag := c.QueryParam("force")

	err := resource.DelResource(nsId, resourceType, resourceId, forceFlag)
	content := map[string]string{"message": "The " + resourceType + " " + resourceId + " has been deleted"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// Todo: need to reimplment the following invalid function

// RestDelChildResource is a common function to handle 'DelChildResource' REST API requests.
// Dummy functions for Swagger exist in [resource/*.go]
// func RestDelChildResource(c echo.Context) error {
// 	reqID, idErr := common.StartRequestWithLog(c)
// 	if idErr != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
// 	}
// 	nsId := c.Param("nsId")

// 	childResourceType := strings.Split(c.Path(), "/")[7]
// 	// c.Path(): /tumblebug/ns/:nsId/resources/vNet/:vNetId/subnet/:subnetId

// 	parentResourceId := c.Param("parentResourceId")
// 	childResourceId := c.Param("childResourceId")
// 	parentResourceId = strings.ReplaceAll(parentResourceId, " ", "+")
// 	parentResourceId = strings.ReplaceAll(parentResourceId, "%2B", "+")
// 	childResourceId = strings.ReplaceAll(childResourceId, " ", "+")
// 	childResourceId = strings.ReplaceAll(childResourceId, "%2B", "+")

// 	forceFlag := c.QueryParam("force")

// 	err := model.DelChildResource(nsId, childResourceType, parentResourceId, childResourceId, forceFlag)
// 	content := map[string]string{"message": "The " + childResourceType + " " + childResourceId + " has been deleted"}
// 	return clientManager.EndRequestWithLog(c, err, content)
// }

// RestGetAllResources is a common function to handle 'GetAllResources' REST API requests.
// Dummy functions for Swagger exist in [resource/*.go]
func RestGetAllResources(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")
	filterKey := c.QueryParam("filterKey")
	filterVal := c.QueryParam("filterVal")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	if optionFlag == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = resource.ListResourceId(nsId, resourceType)
		return clientManager.EndRequestWithLog(c, err, content)
	} else {

		resourceList, err := resource.ListResource(nsId, resourceType, filterKey, filterVal)
		if err != nil {
			err := fmt.Errorf("Failed to list " + resourceType + "s; " + err.Error())
			return clientManager.EndRequestWithLog(c, err, nil)
		}

		switch resourceType {
		case model.StrImage:
			var content struct {
				Image []model.TbImageInfo `json:"image"`
			}

			content.Image = resourceList.([]model.TbImageInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrCustomImage:
			var content struct {
				Image []model.TbCustomImageInfo `json:"customImage"`
			}

			content.Image = resourceList.([]model.TbCustomImageInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrSecurityGroup:
			var content struct {
				SecurityGroup []model.TbSecurityGroupInfo `json:"securityGroup"`
			}

			content.SecurityGroup = resourceList.([]model.TbSecurityGroupInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrSpec:
			var content struct {
				Spec []model.TbSpecInfo `json:"spec"`
			}

			content.Spec = resourceList.([]model.TbSpecInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrSSHKey:
			var content struct {
				SshKey []model.TbSshKeyInfo `json:"sshKey"`
			}

			content.SshKey = resourceList.([]model.TbSshKeyInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrVNet:
			var content struct {
				VNet []model.TbVNetInfo `json:"vNet"`
			}

			content.VNet = resourceList.([]model.TbVNetInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		case model.StrDataDisk:
			var content struct {
				DataDisk []model.TbDataDiskInfo `json:"dataDisk"`
			}

			content.DataDisk = resourceList.([]model.TbDataDiskInfo) // type assertion (interface{} -> array)
			return clientManager.EndRequestWithLog(c, err, content)
		default:
			err := fmt.Errorf("Not accepatble resourceType: " + resourceType)
			return clientManager.EndRequestWithLog(c, err, nil)

		}
	}
}

// RestGetResource is a common function to handle 'GetResource' REST API requests.
// Dummy functions for Swagger exist in [resource/*.go]
func RestGetResource(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/resources/spec/:specId

	resourceId := c.Param("resourceId")
	// make " " and "+" to be "+" (web utilizes "+" for " " in URL)
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	result, err := resource.GetResource(nsId, resourceType, resourceId)
	if err != nil {
		errorMessage := fmt.Errorf("Failed to find " + resourceType + " " + resourceId)
		return clientManager.EndRequestWithLog(c, errorMessage, nil)
	}
	return clientManager.EndRequestWithLog(c, err, result)
}

// RestCheckResource godoc
// @ID CheckResource
// @Summary Check resources' existence
// @Description Check resources' existence
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param resourceType path string true "Resource Type"
// @Param resourceId path string true "Resource ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/checkResource/{resourceType}/{resourceId} [get]
func RestCheckResource(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	exists, err := resource.CheckResource(nsId, resourceType, resourceId)

	type JsonTemplate struct {
		Exists bool `json:"exists"`
	}
	content := JsonTemplate{}
	content.Exists = exists

	return clientManager.EndRequestWithLog(c, err, content)
}

// RestTestAddObjectAssociation is a REST API call handling function
// to test "model.UpdateAssociatedObjectList" function with "add" argument.
func RestTestAddObjectAssociation(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testAddObjectAssociation/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	content, err := resource.UpdateAssociatedObjectList(nsId, resourceType, resourceId, model.StrAdd, "/test/vm/key")

	return clientManager.EndRequestWithLog(c, err, content)
}

// RestTestDeleteObjectAssociation is a REST API call handling function
// to test "model.UpdateAssociatedObjectList" function with "delete" argument.
func RestTestDeleteObjectAssociation(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testDeleteObjectAssociation/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	content, err := resource.UpdateAssociatedObjectList(nsId, resourceType, resourceId, model.StrDelete, "/test/vm/key")
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestTestGetAssociatedObjectCount is a REST API call handling function
// to test "model.GetAssociatedObjectCount" function.
func RestTestGetAssociatedObjectCount(c echo.Context) error {

	nsId := c.Param("nsId")
	//resourceType := strings.Split(c.Path(), "/")[5]
	// c.Path(): /tumblebug/ns/:nsId/testGetAssociatedObjectCount/:resourceType/:resourceId
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")
	resourceId = strings.ReplaceAll(resourceId, " ", "+")
	resourceId = strings.ReplaceAll(resourceId, "%2B", "+")

	associatedObjectCount, err := resource.GetAssociatedObjectCount(nsId, resourceType, resourceId)
	content := map[string]int{"associatedObjectCount": associatedObjectCount}

	return clientManager.EndRequestWithLog(c, err, content)
}

// RestLoadAssets godoc
// @ID LoadAssets
// @Summary Load Common Resources from internal asset files
// @Description Load Common Resources from internal asset files (Spec, Image)
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /loadAssets [get]
func RestLoadAssets(c echo.Context) error {

	content, err := resource.LoadAssets()
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestCreateSharedResource godoc
// @ID CreateSharedResource
// @Summary Create shared resources for MC-Infra
// @Description Create shared resources for MC-Infra
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string true "Option" Enums(all,vnet,sg,sshkey)
// @Param connectionName query string false "connectionName of cloud for designated resource" default()
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/sharedResource [post]
func RestCreateSharedResource(c echo.Context) error {

	nsId := c.Param("nsId")
	resType := c.QueryParam("option")

	// default of connectionConfig is empty string. with empty string, register all resources.
	connectionName := c.QueryParam("connectionName")

	err := resource.CreateSharedResource(nsId, resType, connectionName)
	content := map[string]string{"message": "Done"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelAllSharedResources godoc
// @ID DelAllSharedResources
// @Summary Delete all Default Resource Objects in the given namespace
// @Description Delete all Default Resource Objects in the given namespace
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/sharedResources [delete]
func RestDelAllSharedResources(c echo.Context) error {

	nsId := c.Param("nsId")

	content, err := resource.DeleteSharedResources(nsId)
	return clientManager.EndRequestWithLog(c, err, content)
}

/*
// Request structure for RestRegisterExistingResources
type RestRegisterExistingResourcesRequest struct {
	ConnectionName string `json:"connectionName"`
}

// RestRegisterExistingResources godoc
// @ID RegisterExistingResources
// @Summary Register resources which are existing in CSP and/or CB-Spider
// @Description Register resources which are existing in CSP and/or CB-Spider
// @Tags
// @Accept  json
// @Produce  json
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/registerExistingResources [post]
func RestRegisterExistingResources(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &RestRegisterExistingResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	result, err := model.RegisterExistingResources(nsId, u.ConnectionName)
	if err != nil {
		log.Error().Err(err).Msg("")
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}
	return c.JSON(http.StatusOK, result)
}
*/
