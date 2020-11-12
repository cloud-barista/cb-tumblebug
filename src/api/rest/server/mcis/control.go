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
// @Tags MCIS
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

	result, err := mcis.CorePostMcis(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	//fmt.Printf("%+v\n", *result)
	common.PrintJsonPretty(*result)

	return c.JSON(http.StatusCreated, result)
}

// RestGetMcis godoc
// @Summary Get MCIS
// @Description Get MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Success 200 {object} TbMcisInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [get]
func RestGetMcis(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")

	if action == "suspend" || action == "resume" || action == "reboot" || action == "terminate" {

		result, err := mcis.CoreGetMcisAction(nsId, mcisId, action)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusInternalServerError, &mapA)
		}

		mapA := map[string]string{"message": result}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		result, err := mcis.CoreGetMcisStatus(nsId, mcisId)
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

		result, err := mcis.CoreGetMcisInfo(nsId, mcisId)
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

// RestGetAllMcis godoc
// @Summary List all MCISs
// @Description List all MCISs
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllMcisResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [get]
func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")
	fmt.Println("[Get MCIS List requested with option: " + option)

	result, err := mcis.CoreGetAllMcis(nsId, option)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	content := RestGetAllMcisResponse{}
	content.Mcis = result

	//fmt.Printf("content %+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, &content)

}

/* function RestPutMcis not yet implemented
// RestPutMcis godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags MCIS
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
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId} [delete]
func RestDelMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	err := mcis.DelMcis(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the MCIS"}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the MCIS info"}
	return c.JSON(http.StatusOK, &mapA)
}

// RestDelAllMcis godoc
// @Summary Delete all MCISs
// @Description Delete all MCISs
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis [delete]
func RestDelAllMcis(c echo.Context) error {
	nsId := c.Param("nsId")

	result, err := mcis.CoreDelAllMcis(nsId)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	mapA := map[string]string{"message": result}
	return c.JSON(http.StatusOK, &mapA)
}

type RestPostMcisRecommandResponse struct {
	//Vm_req          []TbVmRecommendReq    `json:"vm_req"`
	Vm_recommend    []mcis.TbVmRecommendInfo `json:"vm_recommend"`
	Placement_algo  string                   `json:"placement_algo"`
	Placement_param []common.KeyValue        `json:"placement_param"`
}

// RestPostMcisRecommand godoc
// @Summary Get MCIS recommendation
// @Description Get MCIS recommendation
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisRecommendReq body mcis.McisRecommendReq true "Details for an MCIS object"
// @Success 200 {object} RestPostMcisRecommandResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/recommend [post]
func RestPostMcisRecommand(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.McisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	result, err := mcis.CorePostMcisRecommand(nsId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostMcisRecommandResponse{}
	content.Vm_recommend = result
	content.Placement_algo = req.Placement_algo
	content.Placement_param = req.Placement_param

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

type RestPostCmdMcisVmResponse struct {
	Result string `json:"result"`
}

// RestPostCmdMcisVm godoc
// @Summary Send a command to specified VM
// @Description Send a command to specified VM
// @Tags MCIS
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

	result, err := mcis.CorePostCmdMcisVm(nsId, mcisId, vmId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	response := RestPostCmdMcisVmResponse{Result: result}
	return c.JSON(http.StatusOK, response)
}

type RestPostCmdMcisResponse struct {
	Mcis_id string `json:"mcis_id"`
	Vm_id   string `json:"vm_id"`
	Vm_ip   string `json:"vm_ip"`
	Result  string `json:"result"`
}

type RestPostCmdMcisResponseWrapper struct {
	Result_array []RestPostCmdMcisResponse `json:"result_array"`
}

// RestPostCmdMcis godoc
// @Summary Send a command to specified MCIS
// @Description Send a command to specified MCIS
// @Tags MCIS
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

	resultArray, err := mcis.CorePostCmdMcis(nsId, mcisId, req)
	if err != nil {
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}

	content := RestPostCmdMcisResponseWrapper{}

	for _, v := range resultArray {

		resultTmp := RestPostCmdMcisResponse{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, content)

}

// RestPostInstallAgentToMcis godoc
// @Summary Install the benchmark agent to specified MCIS
// @Description Install the benchmark agent to specified MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param mcisCmdReq body mcis.McisCmdReq true "MCIS Command Request"
// @Success 200 {object} mcis.AgentInstallContentWrapper
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/install/mcis/{mcisId} [post]
func RestPostInstallAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.InstallAgentToMcis(nsId, mcisId, req)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}

// RestPostMcisVm godoc
// @Summary Create VM in specified MCIS
// @Description Create VM in specified MCIS
// @Tags MCIS
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

// RestGetMcisVm godoc
// @Summary Get MCIS
// @Description Get MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Success 200 {object} mcis.TbVmInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [get]
func RestGetMcisVm(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

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

/* function RestPutMcisVm not yet implemented
// RestPutSshKey godoc
// @Summary Update MCIS
// @Description Update MCIS
// @Tags MCIS
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
// @Summary Delete MCIS
// @Description Delete MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param vmId path string true "VM ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/mcis/{mcisId}/vm/{vmId} [delete]
func RestDelMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	err := mcis.DelMcisVm(nsId, mcisId, vmId)
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
// @Summary List all MCISs
// @Description List all MCISs
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param hostIP body RestGetAllBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmarkall/mcis/{mcisId} [get]
func RestGetAllBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	/*
		type bmReq struct {
			Host string `json:"host"`
		}
		req := &bmReq{}
	*/
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
// @Summary Get MCIS
// @Description Get MCIS
// @Tags MCIS
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param mcisId path string true "MCIS ID"
// @Param hostIP body RestGetBenchmarkRequest true "Host IP address to benchmark"
// @Success 200 {object} mcis.BenchmarkInfoArray
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/benchmark/mcis/{mcisId} [get]
func RestGetBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")

	/*
		type bmReq struct {
			Host string `json:"host"`
		}
		req := &bmReq{}
	*/
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
