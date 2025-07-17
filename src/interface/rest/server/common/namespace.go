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
	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func RestCheckNs(c echo.Context) error {

	if err := Validate(c, []string{"nsId"}); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}
	content, err := common.CheckNs(nsId)
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelAllNs godoc
// @ID DelAllNs
// @Summary Delete all namespaces
// @Description Delete all namespaces
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns [delete]
func RestDelAllNs(c echo.Context) error {

	err := common.DelAllNs()
	content := map[string]string{"message": "All namespaces has been deleted"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestDelNs godoc
// @ID DelNs
// @Summary Delete namespace
// @Description Delete namespace
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Router /ns/{nsId} [delete]
func RestDelNs(c echo.Context) error {

	if err := Validate(c, []string{"nsId"}); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	err := common.DelNs(c.Param("nsId"))
	content := map[string]string{"message": "The ns " + c.Param("nsId") + " has been deleted"}
	return clientManager.EndRequestWithLog(c, err, content)
}

// JSONResult's data field will be overridden by the specific type
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

// Response structure for RestGetAllNs
type RestGetAllNsResponse struct {
	//Name string     `json:"name"`
	Ns []model.NsInfo `json:"ns"`
}

// RestGetAllNs godoc
// @ID GetAllNs
// @Summary List all namespaces or namespaces' ID
// @Description List all namespaces or namespaces' ID
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllNsResponse,[ID]=model.IdList} "Different return structures by the given option param"
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns [get]
func RestGetAllNs(c echo.Context) error {

	optionFlag := c.QueryParam("option")

	var content RestGetAllNsResponse
	if optionFlag == "id" {
		content := model.IdList{}
		var err error
		content.IdList, err = common.ListNsId()
		return clientManager.EndRequestWithLog(c, err, content)
	} else {
		nsList, err := common.ListNs()
		content.Ns = nsList
		return clientManager.EndRequestWithLog(c, err, content)
	}
}

// RestGetNs godoc
// @ID GetNs
// @Summary Get namespace
// @Description Get namespace
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Success 200 {object} model.NsInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId} [get]
func RestGetNs(c echo.Context) error {

	if err := Validate(c, []string{"nsId"}); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := common.GetNs(c.Param("nsId"))
	return clientManager.EndRequestWithLog(c, err, content)
}

// RestPostNs godoc
// @ID PostNs
// @Summary Create namespace
// @Description Create namespace
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param nsReq body model.NsReq true "Details for a new namespace"
// @Success 200 {object} model.NsInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns [post]
func RestPostNs(c echo.Context) error {

	u := &model.NsReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := common.CreateNs(u)
	return clientManager.EndRequestWithLog(c, err, content)

}

// RestPutNs godoc
// @ID PutNs
// @Summary Update namespace
// @Description Update namespace
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param namespace body model.NsReq true "Details to update existing namespace"
// @Success 200 {object} model.NsInfo
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /ns/{nsId} [put]
func RestPutNs(c echo.Context) error {

	u := &model.NsReq{}
	if err := c.Bind(u); err != nil {
		return clientManager.EndRequestWithLog(c, err, nil)
	}

	content, err := common.UpdateNs(c.Param("nsId"), u)
	return clientManager.EndRequestWithLog(c, err, content)
}
