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
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
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
// @ID GetReadyz
// RestGetReadyz godoc
// @Summary Check Tumblebug is ready
// @Description Check Tumblebug is ready
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Failure 503 {object} model.SimpleMsg
// @Router /readyz [get]
func RestGetReadyz(c echo.Context) error {
	message := model.SimpleMsg{}
	message.Message = "CB-Tumblebug is ready"
	if !model.SystemReady {
		message.Message = "CB-Tumblebug is NOT ready"
		return c.JSON(http.StatusServiceUnavailable, &message)
	}
	return c.JSON(http.StatusOK, &message)
}

// RestCheckHTTPVersion godoc
// @ID CheckHTTPVersion
// @Summary Check HTTP version of incoming request
// @Description Checks and logs the HTTP version of the incoming request to the server console.
// @Tags [Admin] API Request Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /httpVersion [get]
func RestCheckHTTPVersion(c echo.Context) error {
	// Access the *http.Request object from the echo.Context
	req := c.Request()

	// Determine the HTTP protocol version of the request
	okMessage := model.SimpleMsg{}
	okMessage.Message = req.Proto

	return c.JSON(http.StatusOK, &okMessage)
}

// RestGetPublicKeyForCredentialEncryption godoc
// @ID GetPublicKeyForCredentialEncryption
// @Summary Get RSA Public Key for Credential Encryption
// @Description Generates an RSA key pair using a 4096-bit key size with the RSA algorithm. The public key is generated using the RSA algorithm with OAEP padding and SHA-256 as the hash function. This key is used to encrypt an AES key that will be used for hybrid encryption of credentials.
// @Tags [Admin] Credential Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.PublicKeyResponse
// @Failure 500 {object} model.SimpleMsg
// @Router /credential/publicKey [get]
func RestGetPublicKeyForCredentialEncryption(c echo.Context) error {

	reqID, err := common.StartRequestWithLog(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
	}
	result, err := common.GetPublicKeyForCredentialEncryption()
	return common.EndRequestWithLog(c, reqID, err, result)

}

// RestRegisterCredential is a REST API handler for registering credentials.
// @ID RegisterCredential
// @Summary Register Credential Information
// @Description This API registers credential information using hybrid encryption. The process involves compressing and encrypting sensitive data with AES-256, encrypting the AES key with a 4096-bit RSA public key (retrieved via `GET /credential/publicKey`), and using OAEP padding with SHA-256. All values, including the AES key, must be base64 encoded before sending, and the public key token ID must be included in the request.
// @Tags [Admin] Credential Management
// @Accept json
// @Produce json
// @Param CredentialReq body model.CredentialReq true "Credential request info"
// @Success 200 {object} model.CredentialInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /credential [post]
func RestRegisterCredential(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	u := &model.CredentialReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := common.RegisterCredential(*u)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestGetConnConfig func is a rest api wrapper for GetConnConfig.
// RestGetConnConfig godoc
// @ID GetConnConfig
// @Summary Get registered ConnConfig info
// @Description Get registered ConnConfig info
// @Tags [Admin] Credential Management
// @Accept  json
// @Produce  json
// @Param connConfigName path string true "Name of connection config (cloud config)"
// @Success 200 {object} model.ConnConfig
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetConnConfigList
// @Summary List all registered ConnConfig
// @Description List all registered ConnConfig
// @Tags [Admin] Credential Management
// @Accept  json
// @Produce  json
// @Param filterCredentialHolder query string false "filter objects by Credential Holder" default()
// @Param filterVerified query boolean false "filter verified connections only" Enums(true, false) default(true)
// @Param filterRegionRepresentative query boolean false "filter connections with the representative region only" Enums(true, false) default(false)
// @Success 200 {object} model.ConnConfigList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetProviderList
// @Summary List all registered Providers
// @Description List all registered Providers
// @Tags [Admin] Multi-Cloud Information
// @Accept  json
// @Produce  json
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetRegion
// @Summary Get registered region info
// @Description Get registered region info
// @Tags [Admin] Multi-Cloud Information
// @Accept  json
// @Produce  json
// @Param providerName path string true "Name of the CSP to retrieve"
// @Param regionName path string true "Name of region to retrieve"
// @Success 200 {object} model.RegionDetail
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetRegionList
// @Summary List all registered regions
// @Description List all registered regions
// @Tags [Admin] Multi-Cloud Information
// @Accept  json
// @Produce  json
// @Success 200 {object} model.RegionList
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetCloudInfo
// @Summary Get cloud information
// @Description Get cloud information
// @Tags [Admin] Multi-Cloud Information
// @Accept  json
// @Produce  json
// @Success 200 {object} model.CloudInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /cloudInfo [get]
func RestGetCloudInfo(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := common.GetCloudInfo()
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestGetK8sClusterInfo func is a rest api wrapper for K8sClsuterInfo.
// RestGetK8sClusterInfo godoc
// @ID GetK8sClusterInfo
// @Summary Get kubernetes cluster information
// @Description Get kubernetes cluster information
// @Tags [Kubernetes] Cluster Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.K8sClusterInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /k8sClusterInfo [get]
func RestGetK8sClusterInfo(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := common.GetK8sClusterInfo()
	return common.EndRequestWithLog(c, reqID, err, content)
}

// ObjectList struct consists of object IDs
type ObjectList struct {
	Object []string `json:"object"`
}

// func RestGetObjects is a rest api wrapper for GetObjectList.
// RestGetObjects godoc
// @ID GetObjects
// @Summary List all objects for a given key
// @Description List all objects for a given key
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param key query string true "retrieve objects by key"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID GetObject
// @Summary Get value of an object
// @Description Get value of an object
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param key query string true "get object value by key"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID DeleteObject
// @Summary Delete an object
// @Description Delete an object
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param key query string true "delete object value by key"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID DeleteObjects
// @Summary Delete child objects along with the given object
// @Description Delete child objects along with the given object
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param key query string true "Delete child objects based on the given key string"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
// @ID InspectResources
// @Summary Inspect Resources (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug, CB-Spider, CSP
// @Description Inspect Resources (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug, CB-Spider, CSP
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param connectionName body RestInspectResourcesRequest true "Specify connectionName and resource type"
// @Success 200 {object} model.InspectResource
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
	// if u.Type == model.StrVNet || u.Type == model.StrSecurityGroup || u.Type == model.StrSSHKey {
	// 	content, err = infra.InspectResources(u.ConnectionName, u.Type)
	// } else if u.Type == "vm" {
	// 	content, err = infra.InspectVMs(u.ConnectionName)
	// }
	content, err = infra.InspectResources(u.ConnectionName, u.ResourceType)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// RestInspectResourcesOverview godoc
// @ID InspectResourcesOverview
// @Summary Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections
// @Description Inspect Resources Overview (vNet, securityGroup, sshKey, vm) registered in CB-Tumblebug and CSP for all connections
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Success 200 {object} model.InspectResourceAllResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /inspectResourcesOverview [get]
func RestInspectResourcesOverview(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	content, err := infra.InspectResourcesOverview()
	return common.EndRequestWithLog(c, reqID, err, content)
}

// Request struct for RestRegisterCspNativeResources
type RestRegisterCspNativeResourcesRequest struct {
	ConnectionName string `json:"connectionName" example:"aws-ap-southeast-1"`
	NsId           string `json:"nsId" example:"default"`
	MciName        string `json:"mciName" example:"csp"`
}

// RestRegisterCspNativeResources godoc
// @ID RegisterCspNativeResources
// @Summary Register CSP Native Resources (vNet, securityGroup, sshKey, vm) to CB-Tumblebug
// @Description Register CSP Native Resources (vNet, securityGroup, sshKey, vm) to CB-Tumblebug
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param Request body RestRegisterCspNativeResourcesRequest true "Specify connectionName, NS Id, and MCI Name""
// @Param option query string false "Option to specify resourceType" Enums(onlyVm, exceptVm)
// @Param mciFlag query string false "Flag to show VMs in a collective MCI form (y,n)" Enums(y, n) default(y)
// @Success 200 {object} model.RegisterResourceResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
	mciFlag := c.QueryParam("mciFlag")

	content, err := infra.RegisterCspNativeResources(u.NsId, u.ConnectionName, u.MciName, option, mciFlag)
	return common.EndRequestWithLog(c, reqID, err, content)

}

// Request struct for RestRegisterCspNativeResources
type RestRegisterCspNativeResourcesRequestAll struct {
	NsId    string `json:"nsId" example:"default"`
	MciName string `json:"mciName" example:"csp"`
}

// RestRegisterCspNativeResourcesAll godoc
// @ID RegisterCspNativeResourcesAll
// @Summary Register CSP Native Resources (vNet, securityGroup, sshKey, vm) from all Clouds to CB-Tumblebug
// @Description Register CSP Native Resources (vNet, securityGroup, sshKey, vm) from all Clouds to CB-Tumblebug
// @Tags [Admin] System Management
// @Accept  json
// @Produce  json
// @Param Request body RestRegisterCspNativeResourcesRequestAll true "Specify NS Id and MCI Name"
// @Param option query string false "Option to specify resourceType" Enums(onlyVm, exceptVm)
// @Param mciFlag query string false "Flag to show VMs in a collective MCI form (y,n)" Enums(y, n) default(y)
// @Success 200 {object} model.RegisterResourceAllResult
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
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
	mciFlag := c.QueryParam("mciFlag")

	content, err := infra.RegisterCspNativeResourcesAll(u.NsId, u.MciName, option, mciFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestForwardAnyReqToAny godoc
// @ID ForwardAnyReqToAny
// @Summary Forward any (GET) request to CB-Spider
// @Description Forward any (GET) request to CB-Spider
// @Tags [Admin] API Request Management
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
