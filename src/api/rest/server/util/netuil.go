package netutil

import (
	"net/http"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type RestPostUtilToDesignVNetRequest struct {
	model.VNetDesignRequest
}

type RestPostUtilToDesignVNetReponse struct {
	model.VNetDesignResponse
}

// RestPostUtilToDesignVNet godoc
// @ID PostUtilToDesignVNet
// @Summary Design VNet and subnets based on user-friendly properties
// @Description Design VNet and subnets based on user-friendly properties
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param vNetDesignReq body RestPostUtilToDesignVNetRequest true "User-friendly properties to design VNet and subnets"
// @Success 201 {object} RestPostUtilToDesignVNetReponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /util/vNet/design [post]
func RestPostUtilToDesignVNet(c echo.Context) error {

	// Bind the request body to SubnettingRequest struct
	reqt := new(RestPostUtilToDesignVNetRequest)
	if err := c.Bind(reqt); err != nil {
		log.Warn().Msgf("invalid request: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
	}

	resp, err := resource.DesignVNets(&reqt.VNetDesignRequest)
	if err != nil {
		log.Error().Msgf("failed to design VNets: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
	}

	return c.JSON(http.StatusCreated, resp)
}

type RestPostUtilToDesignNetworkRequest struct {
	netutil.SubnettingRequest
}

type RestPostUtilToDesignNetworkReponse struct {
	netutil.Network
}

// RestPostUtilToDesignNetwork godoc
// @ID PostUtilToDesignNetwork
// @Summary Design a multi-cloud network configuration
// @Description Design a hierarchical network configuration of a VPC network or multi-cloud network consisting of multiple VPC networks
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param subnettingReq body RestPostUtilToDesignNetworkRequest true "A root/main network CIDR block and subnetting rules"
// @Success 201 {object} RestPostUtilToDesignNetworkReponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /util/net/design [post]
func RestPostUtilToDesignNetwork(c echo.Context) error {

	// ID for API request tracing

	// Bind the request body to SubnettingRequest struct
	subnettingReq := new(netutil.SubnettingRequest)
	if err := c.Bind(subnettingReq); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	// Subnetting as many as requested rules
	networkConfig, err := netutil.SubnettingBy(*subnettingReq)
	if err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	return common.EndRequestWithLog(c, err, networkConfig)
}

type RestPostUtilToValidateNetworkRequest struct {
	netutil.NetworkConfig
}

// RestPostUtilToValidateNetwork godoc
// @ID PostUtilToValidateNetwork
// @Summary Validate a multi-cloud network configuration
// @Description Validate a hierarchical configuration of a VPC network or multi-cloud network consisting of multiple VPC networks
// @Tags [Infra Resource] Network Management
// @Accept  json
// @Produce  json
// @Param subnettingReq body RestPostUtilToValidateNetworkRequest true "A hierarchical network configuration"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /util/net/validate [post]
func RestPostUtilToValidateNetwork(c echo.Context) error {

	// Bind the request body to SubnettingRequest struct
	req := new(netutil.NetworkConfig)
	if err := c.Bind(req); err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	// Validate the network configuration
	netConf := req.NetworkConfiguration
	err := netutil.ValidateNetwork(netConf)
	if err != nil {
		return common.EndRequestWithLog(c, err, nil)
	}

	okMessage := model.SimpleMsg{}
	okMessage.Message = "Network configuration is valid."

	return common.EndRequestWithLog(c, err, okMessage)
}
