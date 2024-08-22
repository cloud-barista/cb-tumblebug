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

// Package mcir is to handle REST API for mcir
package mcir

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
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
// @Param option query string false "Option: [required params for register] connectionName, name, cspSshKeyId, fingerprint, username, publicKey, privateKey" Enums(register)
// @Param sshKeyInfo body mcir.TbSshKeyReq true "Details for an SSH Key object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [post]
func RestPostSshKey(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	nsId := c.Param("nsId")

	optionFlag := c.QueryParam("option")

	u := &mcir.TbSshKeyReq{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.CreateSshKey(nsId, u, optionFlag)
	return common.EndRequestWithLog(c, reqID, err, content)
}

// RestPutSshKey godoc
// @ID PutSshKey
// @Summary Update SSH Key
// @Description Update SSH Key
// @Tags [Infra Resource] Access Key Management
// @Accept  json
// @Produce  json
// @Param sshKeyInfo body mcir.TbSshKeyInfo true "Details for an SSH Key object"
// @Param nsId path string true "Namespace ID" default(default)
// @Param sshKeyId path string true "SshKey ID"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [put]
func RestPutSshKey(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}
	nsId := c.Param("nsId")
	sshKeyId := c.Param("resourceId")

	u := &mcir.TbSshKeyInfo{}
	if err := c.Bind(u); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	content, err := mcir.UpdateSshKey(nsId, sshKeyId, *u)
	return common.EndRequestWithLog(c, reqID, err, content)
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
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [get]
func RestGetSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// Response struct for RestGetAllSshKey
type RestGetAllSshKeyResponse struct {
	SshKey []mcir.TbSshKeyInfo `json:"sshKey"`
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
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllSshKeyResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
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
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [delete]
func RestDelSshKey(c echo.Context) error {
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
// @Success 200 {object} common.IdList
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [delete]
func RestDelAllSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
