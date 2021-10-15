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

// RestPostMcis godoc
// @Summary Create MCIS
// @Description Create MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisReq body TbMcisReq true "Details for an MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [post]
func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CreateMcis(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	//fmt.Printf("%+v\n", *result)
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// JSONResult's data field will be overridden by the specific type
type JSONResult struct {
	//Code    int          `json:"code" `
	//Message string       `json:"message"`
	//Data    interface{}  `json:"data"`
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention Need to be revised.

// RestGetMcis godoc
// @Summary Get MCIS, Action to MCIS (status, suspend, resume, reboot, terminate), or Get VMs' ID
// @Description Get MCIS, Action to MCIS (status, suspend, resume, reboot, terminate), or Get VMs' ID
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param action query string false "Action to MCIS" Enums(status, suspend, resume, reboot, terminate)
// @Param option query string false "Option" Enums(id)
// @success 200 {object} JSONResult{[DEFAULT]=mcis.TbMcisInfo,[STATUS]=mcis.McisStatusInfo,[CONTROL]=common.SimpleMsg,[ID]=common.IdList} "Different return structures by the given action param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [get]
func RestGetMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")
	option := c.QueryParam("option")

	if option == "id" {
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListVmId(nsId, mcisId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		result, err := mcis.HandleMcisAction(nsId, mcisId, action)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		mapA := map[string]string{"message": result}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		result, err := mcis.GetMcisStatus(nsId, mcisId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		var content struct {
			Result *mcis.McisStatusInfo `json:"status"`
		}
		content.Result = result

		//fmt.Printf("%+v\n", content)
		common.PrintJsonPretty(content)

		return c.JSON(http.StatusOK, &content)

	} else {

		result, err := mcis.GetMcisInfo(nsId, mcisId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		//fmt.Printf("%+v\n", *result)
		common.PrintJsonPretty(*result)
		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, result)

	}
}

// Response structure for RestGetAllMcis
type RestGetAllMcisResponse struct {
	Mcis []mcis.TbMcisInfo `json:"mcis"`
}

// Response structure for RestGetAllMcisStatus
type RestGetAllMcisStatusResponse struct {
	Mcis []mcis.McisStatusInfo `json:"mcis"`
}

// RestGetAllMcis godoc
// @Summary List all MCISs or MCISs' ID
// @Description List all MCISs or MCISs' ID
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param option query string false "Option" Enums(id, simple, status)
// @Success 200 {object} JSONResult{[DEFAULT]=RestGetAllMcisResponse,[SIMPLE]=RestGetAllMcisResponse,[ID]=common.IdList,[STATUS]=RestGetAllMcisStatusResponse} "Different return structures by the given option param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [get]
func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")
	fmt.Println("[Get MCIS List requested with option: " + option)

	if option == "id" {
		// return MCIS IDs
		content := common.IdList{}
		var err error
		content.IdList, err = mcis.ListMcisId(nsId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		return c.JSON(http.StatusOK, &content)
	} else if option == "status" {
		// return MCIS Status objects (diffent with MCIS objects)
		result, err := mcis.GetMcisStatusAll(nsId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisStatusResponse{}
		content.Mcis = result
		common.PrintJsonPretty(content)
		return c.JSON(http.StatusOK, &content)
	} else if option == "simple" {
		// MCIS in simple (without VM information)
		result, err := mcis.CoreGetAllMcis(nsId, option)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisResponse{}
		content.Mcis = result
		common.PrintJsonPretty(content)
		return c.JSON(http.StatusOK, &content)
	} else {
		// MCIS in detail (with status information)
		result, err := mcis.CoreGetAllMcis(nsId, "status")
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}
		content := RestGetAllMcisResponse{}
		content.Mcis = result
		common.PrintJsonPretty(content)
		return c.JSON(http.StatusOK, &content)
	}

}

/* function RestPutMcis not yet implemented
// RestPutMcis godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param mcisInfo body TbMcisInfo true "Details for an MCIS object"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [put]
*/
func RestPutMcis(c echo.Context) error {
	return nil
}

// RestDelMcis godoc
// @Summary Delete MCIS
// @Description Delete MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param option query string false "Option for delete MCIS (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [delete]
func RestDelMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	option := c.QueryParam("option")

	err := mcis.DelMcis(nsId, mcisId, option)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the MCIS " + mcisId}
	return c.JSON(http.StatusOK, &mapA)
}

// RestDelAllMcis godoc
// @Summary Delete all MCISs
// @Description Delete all MCISs
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param option query string false "Option for delete MCIS (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [delete]
func RestDelAllMcis(c echo.Context) error {
	nsId := c.Param("nsId")
	option := c.QueryParam("option")

	result, err := mcis.CoreDelAllMcis(nsId, option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": result}
	return c.JSON(http.StatusOK, &mapA)
}

type RestPostMcisRecommendResponse struct {
	//VmReq          []TbVmRecommendReq    `json:"vmReq"`
	VmRecommend    []mcis.TbVmRecommendInfo `json:"vmRecommend"`
	PlacementAlgo  string                   `json:"placementAlgo"`
	PlacementParam []common.KeyValue        `json:"placementParam"`
}

// RestPostMcisRecommend godoc
// @Summary Get MCIS recommendation
// @Description Get MCIS recommendation
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisRecommendReq body mcis.McisRecommendReq true "Details for an MCIS object"
// @Success 200 {object} RestPostMcisRecommendResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/recommend [post]
// @Deprecated
func RestPostMcisRecommend(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.McisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CorePostMcisRecommend(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostMcisRecommendResponse{}
	content.VmRecommend = result
	content.PlacementAlgo = req.PlacementAlgo
	content.PlacementParam = req.PlacementParam

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

// RestGetControlMcis godoc
// @Summary Control the lifecycle of MCIS
// @Description Control the lifecycle of MCIS
// @Tags [MCIS] Control lifecycle
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param action query string false "Action to MCIS" Enums(status, suspend, resume, reboot, terminate, refine)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/control/mcis/{mcisId} [get]
func RestGetControlMcis(c echo.Context) error {
	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" || action == "refine" {

		result, err := mcis.HandleMcisAction(nsId, mcisId, action)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		mapA := map[string]string{"message": result}
		return c.JSON(http.StatusOK, &mapA)

	} else {
		mapA := map[string]string{"message": "'action' should be one of these: suspend, resume, reboot, terminate, refine"}
		return c.JSON(http.StatusBadRequest, &mapA)
	}
}

// RestGetControlMcisVm godoc
// @Summary Control the lifecycle of VM
// @Description Control the lifecycle of VM
// @Tags [MCIS] Control lifecycle
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Param action query string false "Action to MCIS" Enums(status, suspend, resume, reboot, terminate)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/control/mcis/{mcisId}/vm/{vmId} [get]
func RestGetControlMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		result, err := mcis.CoreGetMcisVmAction(nsId, mcisId, vmId, action)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		mapA := map[string]string{"message": result}
		return c.JSON(http.StatusOK, &mapA)

	} else {
		mapA := map[string]string{"message": "'action' should be one of these: suspend, resume, reboot, terminate"}
		return c.JSON(http.StatusBadRequest, &mapA)
	}
}

type RestPostCmdMcisVmResponse struct {
	Result string `json:"result"`
}

// RestPostCmdMcisVm godoc
// @Summary Send a command to specified VM
// @Description Send a command to specified VM
// @Tags [MCIS] Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Success 200 {object} RestPostCmdMcisVmResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cmd/mcis/{mcisId}/vm/{vmId} [post]
func RestPostCmdMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.RemoteCommandToMcisVm(nsId, mcisId, vmId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	response := RestPostCmdMcisVmResponse{Result: result}
	return c.JSON(http.StatusOK, response)
}

type RestPostCmdMcisResponse struct {
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
}

type RestPostCmdMcisResponseWrapper struct {
	ResultArray []RestPostCmdMcisResponse `json:"resultArray"`
}

// RestPostCmdMcis godoc
// @Summary Send a command to specified MCIS
// @Description Send a command to specified MCIS
// @Tags [MCIS] Remote command
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Success 200 {object} RestPostCmdMcisResponseWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/cmd/mcis/{mcisId} [post]
func RestPostCmdMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := mcis.RemoteCommandToMcis(nsId, mcisId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostCmdMcisResponseWrapper{}

	for _, v := range resultArray {

		resultTmp := RestPostCmdMcisResponse{}
		resultTmp.McisId = mcisId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Result
		content.ResultArray = append(content.ResultArray, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, content)

}

// RestPostInstallBenchmarkAgentToMcis godoc
// @Summary Install the benchmark agent to specified MCIS
// @Description Install the benchmark agent to specified MCIS
// @Tags [MCIS] Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Success 200 {object} mcis.RestPostCmdMcisResponseWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/installBenchmarkAgent/mcis/{mcisId} [post]
func RestPostInstallBenchmarkAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	resultArray, err := mcis.InstallBenchmarkAgentToMcis(nsId, mcisId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostCmdMcisResponseWrapper{}

	for _, v := range resultArray {

		resultTmp := RestPostCmdMcisResponse{}
		resultTmp.McisId = mcisId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Result
		content.ResultArray = append(content.ResultArray, resultTmp)

	}

	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, content)

}

// RestPostMcisVm godoc
// @Summary Create VM in specified MCIS
// @Description Create VM in specified MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmReq body mcis.TbVmReq true "Details for an VM object"
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm [post]
func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	vmInfoData := &mcis.TbVmInfo{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}
	common.PrintJsonPretty(*vmInfoData)

	result, err := mcis.CorePostMcisVm(nsId, mcisId, vmInfoData)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestPostMcisVmGroup godoc
// @Summary Create multiple VMs by VM group in specified MCIS
// @Description Create multiple VMs by VM group in specified MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmReq body mcis.TbVmReq true "Details for VM Group"
// @Success 200 {object} mcis.TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vmgroup [post]
func RestPostMcisVmGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	vmInfoData := &mcis.TbVmReq{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}
	common.PrintJsonPretty(*vmInfoData)

	result, err := mcis.CorePostMcisGroupVm(nsId, mcisId, vmInfoData)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// TODO: swag does not support multiple response types (success 200) in an API.
// Annotation for API documention Need to be revised.

// RestGetMcisVm godoc
// @Summary Get VM in specified MCIS
// @Description Get VM in specified MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Param action query string false "Action to MCIS" Enums(status, suspend, resume, reboot, terminate)
// @success 200 {object} JSONResult{[DEFAULT]=mcis.TbVmInfo,[STATUS]=mcis.TbVmStatusInfo,[CONTROL]=common.SimpleMsg} "Different return structures by the given action param"
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [get]
func RestGetMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		result, err := mcis.CoreGetMcisVmAction(nsId, mcisId, vmId, action)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		mapA := map[string]string{"message": result}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		result, err := mcis.CoreGetMcisVmStatus(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		//fmt.Printf("%+v\n", *result)
		common.PrintJsonPretty(*result)

		return c.JSON(http.StatusOK, result)

	} else {

		result, err := mcis.CoreGetMcisVmInfo(nsId, mcisId, vmId)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		//fmt.Printf("%+v\n", *result)
		common.PrintJsonPretty(*result)

		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, result)

	}
}

/* RestPutMcisVm function not yet implemented
// RestPutSshKey godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Param vmInfo body mcis.TbVmInfo true "Details for an VM object"
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [put]
*/
func RestPutMcisVm(c echo.Context) error {
	return nil
}

// RestDelMcisVm godoc
// @Summary Delete VM in specified MCIS
// @Description Delete VM in specified MCIS
// @Tags [MCIS] Provisioning management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Param option query string false "Option for delete VM (support force delete)" Enums(force)
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [delete]
func RestDelMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")
	option := c.QueryParam("option")

	err := mcis.DelMcisVm(nsId, mcisId, vmId, option)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the VM info"}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the VM info"}
	return c.JSON(http.StatusOK, &mapA)
}

// Request struct for RestGetAllBenchmark
type RestGetAllBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetAllBenchmark godoc
// @Summary Run MCIS benchmark for all performance metrics and return results
// @Description Run MCIS benchmark for all performance metrics and return results
// @Tags [MCIS] Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param hostIP body RestGetAllBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkAll/mcis/{mcisId} [post]
func RestGetAllBenchmark(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &RestGetAllBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CoreGetAllBenchmark(nsId, mcisId, req.Host)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)
	return c.JSON(http.StatusOK, result)
}

type RestGetBenchmarkRequest struct {
	Host string `json:"host"`
}

// RestGetBenchmark godoc
// @Summary Run MCIS benchmark for a single performance metric and return results
// @Description Run MCIS benchmark for a single performance metric and return results
// @Tags [MCIS] Performance benchmarking (WIP)
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param hostIP body RestGetBenchmarkRequest true "Host IP address to benchmark"
// @Param action query string true "Benchmark Action to MCIS" Enums(install, init, cpus, cpum, memR, memW, fioR, fioW, dbR, dbW, rtt, mrtt, clean)
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmark/mcis/{mcisId} [post]
func RestGetBenchmark(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")

	req := &RestGetBenchmarkRequest{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CoreGetBenchmark(nsId, mcisId, action, req.Host)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	common.PrintJsonPretty(*result)
	return c.JSON(http.StatusOK, result)
}
