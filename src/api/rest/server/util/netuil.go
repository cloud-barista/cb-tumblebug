package netutil

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/labstack/echo/v4"
)

type RestPostUtilToDesignNetworkRequest struct {
	netutil.SubnettingRequest
}

type RestPostUtilToDesignNetworkReponse struct {
	netutil.Network
}

// RestPostUtilToDesignNetwork godoc
// @Summary Design a multi-cloud network configuration
// @Description Design a hierarchical network configuration of a VPC network or multi-cloud network consisting of multiple VPC networks
// @Tags [Utility] Multi-cloud network design
// @Accept  json
// @Produce  json
// @Param subnettingReq body RestPostUtilToDesignNetworkRequest true "A root/main network CIDR block and subnetting rules"
// @Success 201 {object} RestPostUtilToDesignNetworkReponse
// @Failure 400 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /util/net/design [post]
func RestPostUtilToDesignNetwork(c echo.Context) error {

	// ID for API request tracing
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	// Bind the request body to SubnettingRequest struct
	subnettingReq := new(netutil.SubnettingRequest)
	if err := c.Bind(subnettingReq); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	// Subnetting as many as requested rules
	networkConfig, err := netutil.SubnettingBy(*subnettingReq)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	return common.EndRequestWithLog(c, reqID, err, networkConfig)
}

type RestPostUtilToValidateNetworkRequest struct {
	netutil.NetworkConfig
}

// RestPostUtilToValidateNetwork godoc
// @Summary Validate a multi-cloud network configuration
// @Description Validate a hierarchical configuration of a VPC network or multi-cloud network consisting of multiple VPC networks
// @Tags [Utility] Multi-cloud network design
// @Accept  json
// @Produce  json
// @Param subnettingReq body RestPostUtilToValidateNetworkRequest true "A hierarchical network configuration"
// @Success 200 {object} common.SimpleMsg
// @Failure 400 {object} common.SimpleMsg
// @Failure 500 {object} common.SimpleMsg
// @Router /util/net/validate [post]
func RestPostUtilToValidateNetwork(c echo.Context) error {

	// ID for API request tracing
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	// Bind the request body to SubnettingRequest struct
	req := new(netutil.NetworkConfig)
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	// Validate the network configuration
	netConf := req.NetworkConfiguration
	err := netutil.ValidateNetwork(netConf)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	okMessage := common.SimpleMsg{}
	okMessage.Message = "Network configuration is valid."

	return common.EndRequestWithLog(c, reqID, err, okMessage)
}
