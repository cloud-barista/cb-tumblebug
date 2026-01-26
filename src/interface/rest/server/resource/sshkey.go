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
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
)

// RestPostSshKey godoc
// @ID PostSshKey
// @Summary Create SSH Key
// @Description Create SSH Key
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option: [required params for register] connectionName, name, cspResourceId, fingerprint, username, publicKey, privateKey" Enums(register)
// @Param sshKeyInfo body model.SshKeyReq true "Details for an SSH Key object"
// @Success 200 {object} model.SshKeyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [post]
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &model.SshKeyReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.CreateSshKey(nsId, u, optionFlag)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPutSshKey godoc
// @ID PutSshKey
// @Summary Update SSH Key
// @Description Update SSH Key
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param sshKeyInfo body model.SshKeyInfo true "Details for an SSH Key object"
// @Param nsId path string true "Namespace ID" default(default)
// @Param sshKeyId path string true "SshKey ID"
// @Success 200 {object} model.SshKeyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [put]
func RestPutSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	sshKeyId := c.Param("resourceId")

	u := &model.SshKeyInfo{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := resource.UpdateSshKey(nsId, sshKeyId, *u)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestGetSshKey godoc
// @ID GetSshKey
// @Summary Get SSH Key
// @Description Get SSH Key
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} model.SshKeyInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [get]
func RestGetSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response struct for RestGetAllSshKey
type RestGetAllSshKeyResponse struct {
	SshKey []model.SshKeyInfo `json:"sshKey"`
}

// RestGetAllSshKey godoc
// @ID GetAllSshKey
// @Summary List all SSH Keys or SSH Keys' ID
// @Description List all SSH Keys or SSH Keys' ID
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param option query string false "Option" Enums(id)
// @Param filterKey query string false "Field key for filtering (ex: systemLabel)"
// @Param filterVal query string false "Field value for filtering (ex: Registered from CSP resource)"
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSshKeyResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [get]
func RestGetAllSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelSshKey godoc
// @ID DelSshKey
// @Summary Delete SSH Key
// @Description Delete SSH Key
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [delete]
func RestDelSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDeregisterSshKey godoc
// @ID DeregisterSshKey
// @Summary Deregister SSH Key
// @Description Deregister SSH Key from Spider and TB without deleting the actual CSP resource
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/deregisterCspResource/sshKey/{sshKeyId} [delete]
func RestDeregisterSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllSshKey godoc
// @ID DelAllSshKey
// @Summary Delete all SSH Keys
// @Description Delete all SSH Keys
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param match query string false "Delete resources containing matched ID-substring only" default()
// @Success 200 {object} model.IdList
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [delete]
func RestDelAllSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
