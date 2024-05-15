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
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/rs/zerolog/log"
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

// RestGetReadyz func is for checking CB-Tumblebug server is ready.
// RestGetReadyz godoc
// @Summary Check Tumblebug is ready
// @Description Check Tumblebug is ready
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 503 {object} common.SimpleMsg
// @Router /readyz [get]
func RestGetReadyz(c echo.Context) error {
	message := common.SimpleMsg{}
	message.Message = "CB-Tumblebug is ready"
	if !common.SystemReady {
		message.Message = "CB-Tumblebug is NOT ready"
		return c.JSON(http.StatusServiceUnavailable, &message)
	}
	return c.JSON(http.StatusOK, &message)
}

// RestCheckHTTPVersion godoc
// @Summary Check HTTP version of incoming request
// @Description Checks and logs the HTTP version of the incoming request to the server console.
// @Tags [Admin] System management
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /httpVersion [get]
func RestCheckHTTPVersion(c echo.Context) error {
	// Access the *http.Request object from the echo.Context
	req := c.Request()

	// Determine the HTTP protocol version of the request
	okMessage := common.SimpleMsg{}
	okMessage.Message = req.Proto

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

// RestRegisterCredential func is a rest api wrapper for RegisterCredential.
// RestRegisterCredential godoc
// @Summary Post register Credential info
// @Description Post register Credential info
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Param CredentialReq body common.CredentialReq true "Credential request info"
// @Success 200 {object} common.CredentialInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /credential [post]
func RestRegisterCredential(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &common.CredentialReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := common.RegisterCredential(*u)
	return common.EndRequestWithLog(c, reqID, err, content)

}

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
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	connConfigName := c.Param("connConfigName")

	content, err := common.GetConnConfig(connConfigName)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestGetConnConfigList func is a rest api wrapper for GetConnConfigList.
// RestGetConnConfigList godoc
// @Summary List all registered ConnConfig
// @Description List all registered ConnConfig
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Param filterCredentialHolder query string false "filter objects by Credential Holder" default()
// @Param filterVerified query boolean false "filter verified connections only" Enums(true, false) default(true)
// @Param filterRegionRepresentative query boolean false "filter connections with the representative region only" Enums(true, false) default(false)
// @Success 200 {object} common.ConnConfigList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /connConfig [get]
func RestGetConnConfigList(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	filterCredentialHolder := c.QueryParam("filterCredentialHolder")
	filterVerified := c.QueryParam("filterVerified")
	filterRegionRepresentative := c.QueryParam("filterRegionRepresentative")

	filterVerifiedBool, err := strconv.ParseBool(filterVerified)
	if err != nil {
		filterVerifiedBool = true
	}
	filterRegionRepresentativeBool, err := strconv.ParseBool(filterRegionRepresentative)
	if err != nil {
		filterRegionRepresentativeBool = false
	}

	content, err := common.GetConnConfigList(filterCredentialHolder, filterVerifiedBool, filterRegionRepresentativeBool)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetProviderList func is a rest api wrapper for GetProviderList.
// RestGetProviderList godoc
// @Summary List all registered Providers
// @Description List all registered Providers
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /provider [get]
func RestGetProviderList(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := common.GetProviderList()
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestGetRegion func is a rest api wrapper for GetRegion.
// RestGetRegion godoc
// @Summary Get registered region info
// @Description Get registered region info
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Param providerName path string true "Name of the CSP to retrieve"
// @Param regionName path string true "Name of region to retrieve"
// @Success 200 {object} common.RegionDetail
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /provider/{providerName}/region/{regionName} [get]
func RestGetRegion(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	providerName := c.Param("providerName")
	regionName := c.Param("regionName")

	content, err := common.GetRegion(providerName, regionName)
	return common.EndRequestWithLog(c, reqID, err, content)

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
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := common.GetRegionList()
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestGetCloudInfo func is a rest api wrapper for CloudInfo.
// RestGetCloudInfo godoc
// @Summary Get cloud information
// @Description Get cloud information
// @Tags [Admin] Multi-Cloud environment configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} common.CloudInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /cloudInfo [get]
func RestGetCloudInfo(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := common.GetCloudInfo()
	return common.EndRequestWithLog(c, reqID, err, content)
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
	content := common.GetObjectList(parentKey)

	objectList := ObjectList{}
	for _, v := range content {
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
	log.Debug().Msgf("[Get Tumblebug Object Value] with Key: %s ", parentKey)

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
	log.Debug().Msgf("[Delete Tumblebug Object Value] with Key: %s", parentKey)

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
	log.Debug().Msgf("[Delete Tumblebug child Object Value] with Key: %s", parentKey)

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
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestInspectResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return err
	}

	log.Debug().Msgf("[List Resource Status: %s]", u.ResourceType)

	var content interface{}
	var err error
	// if u.Type == common.StrVNet || u.Type == common.StrSecurityGroup || u.Type == common.StrSSHKey {
	// 	content, err = mcis.InspectResources(u.ConnectionName, u.Type)
	// } else if u.Type == "vm" {
	// 	content, err = mcis.InspectVMs(u.ConnectionName)
	// }
	content, err = mcis.InspectResources(u.ConnectionName, u.ResourceType)
	return common.EndRequestWithLog(c, reqID, err, content)

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
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := mcis.InspectResourcesOverview()
	return common.EndRequestWithLog(c, reqID, err, content)
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
// @Param mcisFlag query string false "Flag to show VMs in a collective MCIS form (y,n)" Enums(y, n) default(y)
// @Success 200 {object} mcis.RegisterResourceResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /registerCspResources [post]
func RestRegisterCspNativeResources(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestRegisterCspNativeResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}
	option := c.QueryParam("option")
	mcisFlag := c.QueryParam("mcisFlag")

	content, err := mcis.RegisterCspNativeResources(u.NsId, u.ConnectionName, u.McisName, option, mcisFlag)
	return common.EndRequestWithLog(c, reqID, err, content)

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
// @Param mcisFlag query string false "Flag to show VMs in a collective MCIS form (y,n)" Enums(y, n) default(y)
// @Success 200 {object} mcis.RegisterResourceAllResult
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /registerCspResourcesAll [post]
func RestRegisterCspNativeResourcesAll(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &RestRegisterCspNativeResourcesRequest{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}
	option := c.QueryParam("option")
	mcisFlag := c.QueryParam("mcisFlag")

	content, err := mcis.RegisterCspNativeResourcesAll(u.NsId, u.McisName, option, mcisFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestForwardAnyReqToAny godoc
// @Summary Forward any (GET) request to CB-Spider
// @Description Forward any (GET) request to CB-Spider
// @Tags [Admin] System utility
// @Accept  json
// @Produce  json
// @Param path path string true "Internal call path to CB-Spider (path without /spider/ prefix) - see [https://documenter.getpostman.com/view/24786935/2s9Ykq8Lpf#231eec23-b0ab-4966-83ce-a0ef92ead7bc] for more details"" default(vmspec)
// @Param Request body interface{} false "Request body (various formats) - see [https://documenter.getpostman.com/view/24786935/2s9Ykq8Lpf#231eec23-b0ab-4966-83ce-a0ef92ead7bc] for more details"
// @Success 200 {object} map[string]interface{}
// @Router /forward/{path} [post]
func RestForwardAnyReqToAny(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	reqPath := c.Param("*")
	reqPath, err := url.PathUnescape(reqPath)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	log.Info().Msgf("reqPath: %s", reqPath)

	method := "GET"
	var requestBody interface{}
	if c.Request().Body != nil {
		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return common.EndRequestWithLog(c, reqID, fmt.Errorf("Failed to read request body: %v", err), nil)
		}
		requestBody = bodyBytes
	} else {
		requestBody = common.NoBody
	}

	content, err := common.ForwardRequestToAny(reqPath, method, requestBody)
	return common.EndRequestWithLog(c, reqID, err, content)
}
