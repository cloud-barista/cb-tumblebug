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
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

func RestCheckNs(c echo.Context) error {

	/*
		nsId := c.Param("nsId")

		exists, err := common.CheckNs(nsId)

		type JsonTemplate struct {
			Exists bool `json:"exists"`
		}
		content := JsonTemplate{}
		content.Exists = exists

		if err != nil {
			common.CBLog.Error(err)
			//mapA := common.SimpleMsg{err.Error()}
			//return c.JSON(http.StatusFailedDependency, &mapA)
			return c.JSON(http.StatusNotFound, &content)
		}

		return c.JSON(http.StatusOK, &content)
	*/

	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusNotFound, err.Error())
	}

	nsId := c.Param("nsId")
	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	exists, err := common.CheckNs(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusNotFound, err.Error())
	}

	return SendExistence(c, http.StatusOK, exists)
}

// RestDelAllNs godoc
// @Summary Delete all namespaces
// @Description Delete all namespaces
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns [delete]
func RestDelAllNs(c echo.Context) error {

	/*
		err := common.DelAllNs()
		if err != nil {
			common.CBLog.Error(err)
			mapA := common.SimpleMsg{err.Error()}
			return c.JSON(http.StatusConflict, &mapA)
		}

		mapA := common.SimpleMsg{"All namespaces has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	err := common.DelAllNs()
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	return SendMessage(c, http.StatusOK, "All namespaces has been deleted")
}

// RestDelNs godoc
// @Summary Delete namespace
// @Description Delete namespace
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId} [delete]
func RestDelNs(c echo.Context) error {

	/*
		id := c.Param("nsId")

		err := common.DelNs(id)
		if err != nil {
			common.CBLog.Error(err)
			mapA := common.SimpleMsg{err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := common.SimpleMsg{"The ns has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	err := common.DelNs(c.Param("nsId"))
	if err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	return SendMessage(c, http.StatusOK, "The ns "+c.Param("nsId")+" has been deleted")
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
	Ns []common.NsInfo `json:"ns"`
}

// RestGetAllNs godoc
// @Summary List all namespaces or namespaces' ID
// @Description List all namespaces or namespaces' ID
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Param option query string false "Option" Enums(id)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllNsResponse,[ID]=common.IdList} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [get]
func RestGetAllNs(c echo.Context) error {

	optionFlag := c.QueryParam("option")

	var content RestGetAllNsResponse
	if optionFlag == "id" {
		content := common.IdList{}

		var err error
		content.IdList, err = common.ListNsId()
		if err != nil {
			//mapA := common.SimpleMsg{"Failed to list namespaces."}
			//return c.JSON(http.StatusNotFound, &mapA)
			return SendMessage(c, http.StatusOK, "Failed to list namespaces' ID: "+err.Error())
		}

		return c.JSON(http.StatusOK, &content)
	} else {
		nsList, err := common.ListNs()
		if err != nil {
			//mapA := common.SimpleMsg{"Failed to list namespaces."}
			//return c.JSON(http.StatusNotFound, &mapA)
			return SendMessage(c, http.StatusOK, "Failed to list namespaces.")
		}

		if nsList == nil {
			//return c.JSON(http.StatusOK, &content)
			return Send(c, http.StatusOK, content)
		}

		// When err == nil && resourceList != nil
		content.Ns = nsList
		//return c.JSON(http.StatusOK, &content)
		return Send(c, http.StatusOK, content)
	}
}

// RestGetNs godoc
// @Summary Get namespace
// @Description Get namespace
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId} [get]
func RestGetNs(c echo.Context) error {
	//id := c.Param("nsId")
	if err := Validate(c, []string{"nsId"}); err != nil {
		common.CBLog.Error(err)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	res, err := common.GetNs(c.Param("nsId"))
	if err != nil {
		//mapA := common.SimpleMsg{"Failed to find the namespace " + id}
		//return c.JSON(http.StatusNotFound, &mapA)
		return SendMessage(c, http.StatusOK, "Failed to find the namespace "+c.Param("nsId"))
	} else {
		//return c.JSON(http.StatusOK, &res)
		return Send(c, http.StatusOK, res)
	}
}

// RestPostNs godoc
// @Summary Create namespace
// @Description Create namespace
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Param nsReq body common.NsReq true "Details for a new namespace"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns [post]
func RestPostNs(c echo.Context) error {

	u := &common.NsReq{}
	if err := c.Bind(u); err != nil {
		//return err
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}

	fmt.Println("[Creating Ns]")
	content, err := common.CreateNs(u)
	if err != nil {
		//common.CBLog.Error(err)
		////mapA := common.SimpleMsg{"Failed to create the ns " + u.Name}
		//mapA := common.SimpleMsg{err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return SendMessage(c, http.StatusBadRequest, err.Error())
	}
	//return c.JSON(http.StatusCreated, content)
	return Send(c, http.StatusOK, content)

}

/* function RestPutNs not yet implemented
// RestPutNs godoc
// @Summary Update namespace
// @Description Update namespace
// @Tags [Namespace] Namespace management
// @Accept  json
// @Produce  json
// @Param namespace body common.NsInfo true "Details to update existing namespace"
// @Success 200 {object} common.NsInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId} [put]
*/
func RestPutNs(c echo.Context) error {
	return nil
}
