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

// Package common is to handle REST API for common funcitonalities
package common

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

type TbConnectionName struct {
	ConnectionName string `json:"connectionName"`
}

type Existence struct {
	Exists bool `json:"exists"`
}

func SendExistence(c echo.Context, httpCode int, existence bool) error {
	return c.JSON(httpCode, Existence{Exists: existence})
}

type Status struct {
	Message string `json:"message"`
}

func SendMessage(c echo.Context, httpCode int, msg string) error {
	return c.JSON(httpCode, Status{Message: msg})
}

func Send(c echo.Context, httpCode int, json interface{}) error {
	return c.JSON(httpCode, json)
}

func Validate(c echo.Context, params []string) error {
	var err error
	for _, name := range params {
		err = validate.Var(c.Param(name), "required")
		if err != nil {
			return err
		}
	}
	return nil
}

// RestGetHealth func is for checking Tumblebug server health.
// RestGetHealth godoc
// @Summary Check Tumblebug is alive
// @Description Check Tumblebug is alive
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /health [get]
func RestGetHealth(c echo.Context) error {
	okMessage := common.SimpleMsg{}
	okMessage.Message = "API server of CB-Tumblebug is alive"

	return c.JSON(http.StatusOK, &okMessage)
}

/*
// RestGetSwagger func is to get API document web.
// RestGetSwagger godoc
// @Summary Get API document web
// @Description Get API document web
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /swaggerActive [get]
func RestGetSwagger(c echo.Context) error {
	docFile := os.Getenv("API_DOC_PATH")

	f, err := os.Open(docFile)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	data := make(map[string]interface{}, 0)
	if err := dec.Decode(&data); err != nil {
		return err
	}
	data["host"] = os.Getenv("SELF_ENDPOINT")
	return c.JSON(http.StatusOK, data)
}
*/

// RestGetConnConfig func is a rest api wrapper for GetConnConfig.
// RestGetConnConfig godoc
// @Summary Get registered ConnConfig info
// @Description Get registered ConnConfig info
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Param connConfigName path string true "Name of connection config (cloud config)"
// @Success 200 {object} common.ConnConfig
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /connConfig/{connConfigName} [get]
func RestGetConnConfig(c echo.Context) error {

	connConfigName := c.Param("connConfigName")

	fmt.Println("[Get ConnConfig for name]" + connConfigName)
	content, err := common.GetConnConfig(connConfigName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}
	return c.JSON(http.StatusOK, &content)

}

// RestGetConnConfigList func is a rest api wrapper for GetConnConfigList.
// RestGetConnConfigList godoc
// @Summary List all registered ConnConfig
// @Description List all registered ConnConfig
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} common.ConnConfigList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /connConfig [get]
func RestGetConnConfigList(c echo.Context) error {

	fmt.Println("[Get ConnConfig List]")
	content, err := common.GetConnConfigList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestGetRegion func is a rest api wrapper for GetRegion.
// RestGetRegion godoc
// @Summary Get registered region info
// @Description Get registered region info
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Param regionName path string true "Name of region to retrieve"
// @Success 200 {object} common.Region
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /region/{regionName} [get]
func RestGetRegion(c echo.Context) error {

	regionName := c.Param("regionName")

	fmt.Println("[Get Region for name]" + regionName)
	content, err := common.GetRegion(regionName)
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// RestGetRegionList func is a rest api wrapper for GetRegionList.
// RestGetRegionList godoc
// @Summary List all registered regions
// @Description List all registered regions
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} common.RegionList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /region [get]
func RestGetRegionList(c echo.Context) error {

	fmt.Println("[Get Region List]")
	content, err := common.GetRegionList()
	if err != nil {
		common.CBLog.Error(err)
		return c.JSONBlob(http.StatusNotFound, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

// ObjectList struct consists of object IDs
type ObjectList struct {
	Object []string `json:"object"`
}

// func RestGetObjects is a rest api wrapper for GetObjectList.
// RestGetObjects godoc
// @Summary List all objects for a given key
// @Description List all objects for a given key
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param key query string true "retrieve objects by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /objects [get]
func RestGetObjects(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Get Tumblebug Object List] with Key: %s \n", parentKey)

	content := common.GetObjectList(parentKey)

	objectList := ObjectList{}
	for i, v := range content {
		fmt.Printf("[Obj: %d] %s \n", i, v)
		objectList.Object = append(objectList.Object, v)
	}
	return c.JSON(http.StatusOK, &objectList)
}

// func RestGetObject is a rest api wrapper for GetObject.
// RestGetObject godoc
// @Summary Get value of an object
// @Description Get value of an object
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param key query string true "get object value by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /object [get]
func RestGetObject(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Get Tumblebug Object Value] with Key: %s \n", parentKey)

	content, err := common.GetObjectValue(parentKey)
	if err != nil || content == "" {
		return SendMessage(c, http.StatusOK, "Cannot find ["+parentKey+"] object")
	}

	var contentJSON map[string]interface{}
	json.Unmarshal([]byte(content), &contentJSON)

	return c.JSON(http.StatusOK, &contentJSON)
}

// func RestDeleteObject is a rest api wrapper for DeleteObject.
// RestDeleteObject godoc
// @Summary Delete an object
// @Description Delete an object
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param key query string true "delete object value by key"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /object [delete]
func RestDeleteObject(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Delete Tumblebug Object] with Key: %s \n", parentKey)

	content, err := common.GetObjectValue(parentKey)
	if err != nil || content == "" {
		return SendMessage(c, http.StatusOK, "Cannot find ["+parentKey+"] object")
	}

	err = common.DeleteObject(parentKey)
	if err != nil {
		return SendMessage(c, http.StatusOK, "Cannot delete ["+parentKey+"] object")
	}

	return SendMessage(c, http.StatusOK, "The object has been deleted")
}

// func RestDeleteObjects is a rest api wrapper for DeleteObjects.
// RestDeleteObjects godoc
// @Summary Delete child objects along with the given object
// @Description Delete child objects along with the given object
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param key query string true "Delete child objects based on the given key string"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /objects [delete]
func RestDeleteObjects(c echo.Context) error {
	parentKey := c.QueryParam("key")
	fmt.Printf("[Delete Tumblebug child Objects] with Key: %s \n", parentKey)

	err := common.DeleteObjects(parentKey)
	if err != nil {
		return SendMessage(c, http.StatusOK, "Cannot delete  objects")
	}

	return SendMessage(c, http.StatusOK, "Objects have been deleted")
}

// Request struct for RestInspectResources
type RestInspectResourcesRequest struct {
	ConnectionName string `json:"connectionName" example:"aws-ap-southeast-1"`
	ResourceType   string `json:"resourceType" example:"vNet" enums:"vNet,securityGroup,sshKey,vm"`
}

// RestInspectResources godoc
// @Summary Inspect Resources (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug, CB-Spider, CSP
// @Description Inspect Resources (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug, CB-Spider, CSP
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param connectionName body RestInspectResourcesRequest true "Specify connectionName and resource type"
// @Success 200 {object} mcis.InspectResource
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /inspectResources [post]
func RestInspectResources(c echo.Context) error {

	u := &RestInspectResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Printf("[List Resource Status: %s] \n", u.ResourceType)
	var content interface{}
	var err error
	// if u.Type == common.StrVNet || u.Type == common.StrSecurityGroup || u.Type == common.StrSSHKey {
	// 	content, err = mcis.InspectResources(u.ConnectionName, u.Type)
	// } else if u.Type == "vm" {
	// 	content, err = mcis.InspectVMs(u.ConnectionName)
	// }
	content, err = mcis.InspectResources(u.ConnectionName, u.ResourceType)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, &content)

}

// RestInspectResourcesOverview godoc
// @Summary Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections
// @Description Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} mcis.InspectResourceAllResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /inspectResourcesOverview [get]
func RestInspectResourcesOverview(c echo.Context) error {
	content, err := mcis.InspectResourcesOverview()
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusOK, &content)
}

// Request struct for RestRegisterCspNativeResources
type RestRegisterCspNativeResourcesRequest struct {
	ConnectionName string `json:"connectionName" example:"aws-ap-southeast-1"`
	NsId           string `json:"nsId" example:"ns01"`
	McisName       string `json:"mcisName" example:"csp"`
}

// RestRegisterCspNativeResources godoc
// @Summary Register CSP Native Resources (vNet, securityGroup, sshKey, vm) to CB-Tumblebug
// @Description Register CSP Native Resources (vNet, securityGroup, sshKey, vm) to CB-Tumblebug
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param Request body RestRegisterCspNativeResourcesRequest true "Specify connectionName, NS Id, and MCIS Name""
// @Param option query string false "Option to specify resourceType" Enums(onlyVm, exceptVm)
// @Success 200 {object} mcis.RegisterResourceResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /registerCspResources [post]
func RestRegisterCspNativeResources(c echo.Context) error {

	u := &RestRegisterCspNativeResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}
	option := c.QueryParam("option")

	content, err := mcis.RegisterCspNativeResources(u.NsId, u.ConnectionName, u.McisName, option)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, &content)

}

// Request struct for RestRegisterCspNativeResources
type RestRegisterCspNativeResourcesRequestAll struct {
	NsId     string `json:"nsId" example:"ns01"`
	McisName string `json:"mcisName" example:"csp"`
}

// RestRegisterCspNativeResourcesAll godoc
// @Summary Register CSP Native Resources (vNet, securityGroup, sshKey, vm) from all Clouds to CB-Tumblebug
// @Description Register CSP Native Resources (vNet, securityGroup, sshKey, vm) from all Clouds to CB-Tumblebug
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Param Request body RestRegisterCspNativeResourcesRequestAll true "Specify NS Id and MCIS Name"
// @Param option query string false "Option to specify resourceType" Enums(onlyVm, exceptVm)
// @Success 200 {object} mcis.RegisterResourceAllResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /registerCspResourcesAll [post]
func RestRegisterCspNativeResourcesAll(c echo.Context) error {

	u := &RestRegisterCspNativeResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}
	option := c.QueryParam("option")

	content, err := mcis.RegisterCspNativeResourcesAll(u.NsId, u.McisName, option)

	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	return c.JSON(http.StatusOK, &content)

}
