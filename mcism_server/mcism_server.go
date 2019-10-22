// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista

package main

import (
	"os"

	"github.com/cloud-barista/cb-tumblebug/mcism_server/confighandler"

	"fmt"

	uuid "github.com/google/uuid"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
)

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
}

const defaultMonitorPort = ":2019"

var masterConfigInfos confighandler.MASTERCONFIGTYPE

// Main Body

func apiServer() {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World! This is cloud-barista cb-tumblebug")
	})

	// Route
	g := e.Group("/ns", nsValidation())

	g.POST("", restPostNs)
	g.GET("/:nsId", restGetNs)
	g.GET("", restGetAllNs)
	g.PUT("/:nsId", restPutNs)
	g.DELETE("/:nsId", restDelNs)
	g.DELETE("", restDelAllNs)

	g.POST("/:nsId/mcis", restPostMcis)
	g.GET("/:nsId/mcis/:mcisId", restGetMcis)
	g.GET("/:nsId/mcis", restGetAllMcis)
	g.PUT("/:nsId/mcis/:mcisId", restPutMcis)
	g.DELETE("/:nsId/mcis/:mcisId", restDelMcis)
	g.DELETE("/:nsId/mcis", restDelAllMcis)

	g.POST("/:nsId/mcis/:mcisId/vm", restPostMcisVm)
	g.GET("/:nsId/mcis/:mcisId/vm/:vmId", restGetMcisVm)
	//g.GET("/:nsId/mcis", restGetAllMcis)
	//g.PUT("/:nsId/mcis/:mcisId", restPutMcis)
	g.DELETE("/:nsId/mcis/:mcisId/vm/:vmId", restDelMcisVm)
	//g.DELETE("/:nsId/mcis", restDelAllMcis)

	g.POST("/:nsId/mcis/recommend", restPostMcisRecommand)

	g.POST("/:nsId/resources/image", restPostImage)
	g.GET("/:nsId/resources/image/:imageId", restGetImage)
	g.GET("/:nsId/resources/image", restGetAllImage)
	g.PUT("/:nsId/resources/image/:imageId", restPutImage)
	g.DELETE("/:nsId/resources/image/:imageId", restDelImage)
	g.DELETE("/:nsId/resources/image", restDelAllImage)

	g.POST("/:nsId/resources/sshKey", restPostSshKey)
	g.GET("/:nsId/resources/sshKey/:sshKeyId", restGetSshKey)
	g.GET("/:nsId/resources/sshKey", restGetAllSshKey)
	g.PUT("/:nsId/resources/sshKey/:sshKeyId", restPutSshKey)
	g.DELETE("/:nsId/resources/sshKey/:sshKeyId", restDelSshKey)
	g.DELETE("/:nsId/resources/sshKey", restDelAllSshKey)

	g.POST("/:nsId/resources/spec", restPostSpec)
	g.GET("/:nsId/resources/spec/:specId", restGetSpec)
	g.GET("/:nsId/resources/spec", restGetAllSpec)
	g.PUT("/:nsId/resources/spec/:specId", restPutSpec)
	g.DELETE("/:nsId/resources/spec/:specId", restDelSpec)
	g.DELETE("/:nsId/resources/spec", restDelAllSpec)

	g.POST("/:nsId/resources/securityGroup", restPostSecurityGroup)
	g.GET("/:nsId/resources/securityGroup/:securityGroupId", restGetSecurityGroup)
	g.GET("/:nsId/resources/securityGroup", restGetAllSecurityGroup)
	g.PUT("/:nsId/resources/securityGroup/:securityGroupId", restPutSecurityGroup)
	g.DELETE("/:nsId/resources/securityGroup/:securityGroupId", restDelSecurityGroup)
	g.DELETE("/:nsId/resources/securityGroup", restDelAllSecurityGroup)

	g.POST("/:nsId/resources/subnet", restPostSubnet)
	g.GET("/:nsId/resources/subnet/:subnetId", restGetSubnet)
	g.GET("/:nsId/resources/subnet", restGetAllSubnet)
	g.PUT("/:nsId/resources/subnet/:subnetId", restPutSubnet)
	g.DELETE("/:nsId/resources/subnet/:subnetId", restDelSubnet)
	g.DELETE("/:nsId/resources/subnet", restDelAllSubnet)

	g.POST("/:nsId/resources/network", restPostNetwork)
	g.GET("/:nsId/resources/network/:networkId", restGetNetwork)
	g.GET("/:nsId/resources/network", restGetAllNetwork)
	g.PUT("/:nsId/resources/network/:networkId", restPutNetwork)
	g.DELETE("/:nsId/resources/network/:networkId", restDelNetwork)
	g.DELETE("/:nsId/resources/network", restDelAllNetwork)

	g.POST("/:nsId/resources/publicIp", restPostPublicIp)
	g.GET("/:nsId/resources/publicIp/:publicIpId", restGetPublicIp)
	g.GET("/:nsId/resources/publicIp", restGetAllPublicIp)
	g.PUT("/:nsId/resources/publicIp/:publicIpId", restPutPublicIp)
	g.DELETE("/:nsId/resources/publicIp/:publicIpId", restDelPublicIp)
	g.DELETE("/:nsId/resources/publicIp", restDelAllPublicIp)

	g.POST("/:nsId/resources/vNic", restPostVNic)
	g.GET("/:nsId/resources/vNic/:vNicId", restGetVNic)
	g.GET("/:nsId/resources/vNic", restGetAllVNic)
	g.PUT("/:nsId/resources/vNic/:vNicId", restPutVNic)
	g.DELETE("/:nsId/resources/vNic/:vNicId", restDelVNic)
	g.DELETE("/:nsId/resources/vNic", restDelAllVNic)

	e.Logger.Fatal(e.Start(":1323"))

}

var SPIDER_URL string

func main() {

	fmt.Println("\n[cb-tumblebug (Multi-Cloud Infra Service Management Framework)]")
	fmt.Println("\nInitiating REST API Server ...")
	fmt.Println("\n[REST API call examples]")

	SPIDER_URL = os.Getenv("SPIDER_URL")

	/*
		fmt.Println("[List MCISs]:\t\t curl <ServerIP>:1323/mcis")
		fmt.Println("[Create MCIS]:\t\t curl -X POST <ServerIP>:1323/mcis  -H 'Content-Type: application/json' -d '{<MCIS_REQ_JSON>}'")
		fmt.Println("[Get MCIS Info]:\t curl <ServerIP>:1323/mcis/<McisID>")
		fmt.Println("[Get MCIS status]:\t curl <ServerIP>:1323/mcis/<McisID>?action=monitor")
		fmt.Println("[Terminate MCIS]:\t curl <ServerIP>:1323/mcis/<McisID>?action=terminate")
		fmt.Println("[Del MCIS Info]:\t curl -X DELETE <ServerIP>:1323/mcis/<McisID>")
		fmt.Println("[Del MCISs Info]:\t curl -X DELETE <ServerIP>:1323/mcis")

		fmt.Println("\n")
		fmt.Println("[List Images]:\t\t curl <ServerIP>:1323/image")
		fmt.Println("[Create Image]:\t\t curl -X POST <ServerIP>:1323/image?action=create -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
		fmt.Println("[Register Image]:\t\t curl -X POST <ServerIP>:1323/image?action=register -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
		fmt.Println("[Get Image Info]:\t curl <ServerIP>:1323/image/<imageID>")
		fmt.Println("[Del Image Info]:\t curl -X DELETE <ServerIP>:1323/image/<imageID>")
		fmt.Println("[Del Images Info]:\t curl -X DELETE <ServerIP>:1323/image")
	*/

	// load config
	masterConfigInfos = confighandler.GetMasterConfigInfos()

	// Run API Server
	apiServer()

}

// MCIS utilities

func genUuid() string {
	return uuid.New().String()
}

func genMcisKey(nsId string, mcisId string, vmId string) string {

	if vmId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId + "/vm/" + vmId
	} else if mcisId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}

}

func genResourceKey(nsId string, resourceType string, resourceId string) string {
	//resourceType = strings.ToLower(resourceType)

	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "network" ||
		resourceType == "subnet" ||
		resourceType == "securityGroup" ||
		resourceType == "publicIp" ||
		resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalid_key"
	}

}
