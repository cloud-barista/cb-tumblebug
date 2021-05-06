package mcir

import (
	"fmt"
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/labstack/echo/v4"
)

// RestPostSshKey godoc
// @Summary Create SSH Key
// @Description Create SSH Key
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param sshKeyInfo body mcir.TbSshKeyReq true "Details for an SSH Key object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [post]
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &mcir.TbSshKeyReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[POST SshKey")
	//fmt.Println("[Creating SshKey]")
	//content, responseCode, _, err := CreateSshKey(nsId, u)
	content, err := mcir.CreateSshKey(nsId, u)
	if err != nil {
		common.CBLog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SshKey"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		*/
		//return c.JSON(res.StatusCode, res)
		//body, _ := ioutil.ReadAll(res.Body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusInternalServerError, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

/* function RestPutSshKey not yet implemented
// RestPutSshKey godoc
// @Summary Update SSH Key
// @Description Update SSH Key
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param sshKeyInfo body mcir.TbSshKeyInfo true "Details for an SSH Key object"
// @Success 200 {object} mcir.TbSshKeyInfo
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [put]
*/
func RestPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

// RestGetSshKey godoc
// @Summary Get SSH Key
// @Description Get SSH Key
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
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
// @Summary List all SSH Keys
// @Description List all SSH Keys
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} RestGetAllSshKeyResponse
// @Failure 404 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [get]
func RestGetAllSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelSshKey godoc
// @Summary Delete SSH Key
// @Description Delete SSH Key
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Param sshKeyId path string true "SSH Key ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey/{sshKeyId} [delete]
func RestDelSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}

// RestDelAllSshKey godoc
// @Summary Delete all SSH Keys
// @Description Delete all SSH Keys
// @Tags [MCIR] Access key management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID"
// @Success 200 {object} common.SimpleMsg
// @Failure 404 {object} common.SimpleMsg
// @Router /ns/{nsId}/resources/sshKey [delete]
func RestDelAllSshKey(c echo.Context) error {
	// This is a dummy function for Swagger.
	return nil
}
