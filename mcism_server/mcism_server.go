// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista

package main

import (
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

	g.POST("/:nsId/mcis", restPostMcis)
	g.GET("/:nsId/mcis/:mcisId", restGetMcis)
	g.GET("/:nsId/mcis", restGetAllMcis)
	g.PUT("/:nsId/mcis/:mcisId", restPutMcis)
	g.DELETE("/:nsId/mcis/:mcisId", restDelMcis)
	g.DELETE("/:nsId/mcis", restDelAllMcis)

	g.POST("", restPostNs)
	g.GET("/:nsId", restGetNs)
	g.GET("", restGetAllNs)
	g.PUT("/:nsId", restPutNs)
	g.DELETE("/:nsId", restDelNs)
	g.DELETE("", restDelAllNs)

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

	/*
		g.POST("/resources/spec", restPostSpec)
		g.GET("/resources/spec/:id", restGetSpec)
		g.GET("/resources/spec", restGetAllSpec)
		g.PUT("/resources/spec/:id", restPutSpec)
		g.DELETE("/resources/spec/:id", restDelSpec)
		g.DELETE("/resources/spec", restDelAllSpec)

		g.POST("/resources/network", restPostNetwork)
		g.GET("/resources/network/:id", restGetNetwork)
		g.GET("/resources/network", restGetAllNetwork)
		g.PUT("/resources/network/:id", restPutNetwork)
		g.DELETE("/resources/network/:id", restDelNetwork)
		g.DELETE("/resources/network", restDelAllNetwork)

		g.POST("/resources/subnet", restPostSubnet)
		g.GET("/resources/subnet/:id", restGetSubnet)
		g.GET("/resources/subnet", restGetAllSubnet)
		g.PUT("/resources/subnet/:id", restPutSubnet)
		g.DELETE("/resources/subnet/:id", restDelSubnet)
		g.DELETE("/resources/subnet", restDelAllSubnet)

		g.POST("/resources/securityGroup", restPostSecurityGroup)
		g.GET("/resources/securityGroup/:id", restGetSecurityGroup)
		g.GET("/resources/securityGroup", restGetAllSecurityGroup)
		g.PUT("/resources/securityGroup/:id", restPutSecurityGroup)
		g.DELETE("/resources/securityGroup/:id", restDelSecurityGroup)
		g.DELETE("/resources/securityGroup", restDelAllSecurityGroup)


	*/
	e.Logger.Fatal(e.Start(":1323"))

}

func main() {

	fmt.Println("\n[cb-tumblebug (Multi-Cloud Infra Service Management Framework)]")
	fmt.Println("\nInitiating REST API Server ...")
	fmt.Println("\n[REST API call examples]")

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
