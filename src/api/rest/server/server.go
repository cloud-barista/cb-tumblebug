package restapiserver

import (
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/webadmin"

	rest_common "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/common"
	rest_mcir "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcir"
	rest_mcis "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcis"

	"os"
	"fmt"
	"crypto/subtle"

	// REST API (echo)
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// CB-Store

	_ "github.com/cloud-barista/cb-tumblebug/src/docs"
	"github.com/swaggo/echo-swagger" // echo-swagger middleware
)

//var masterConfigInfos confighandler.MASTERCONFIGTYPE

const (
	InfoColor    = "\033[1;34m%s\033[0m"
	NoticeColor  = "\033[1;36m%s\033[0m"
	WarningColor = "\033[1;33m%s\033[0m"
	ErrorColor   = "\033[1;31m%s\033[0m"
	DebugColor   = "\033[0;36m%s\033[0m"
)

const (
	Version = " Version: Cappuccino"
	website = " Repository: https://github.com/cloud-barista/cb-tumblebug"
	banner  = `

  ██████╗██╗      ██████╗ ██╗   ██╗██████╗       ██████╗  █████╗ ██████╗ ██╗███████╗████████╗ █████╗
 ██╔════╝██║     ██╔═══██╗██║   ██║██╔══██╗      ██╔══██╗██╔══██╗██╔══██╗██║██╔════╝╚══██╔══╝██╔══██╗
 ██║     ██║     ██║   ██║██║   ██║██║  ██║█████╗██████╔╝███████║██████╔╝██║███████╗   ██║   ███████║
 ██║     ██║     ██║   ██║██║   ██║██║  ██║╚════╝██╔══██╗██╔══██║██╔══██╗██║╚════██║   ██║   ██╔══██║
 ╚██████╗███████╗╚██████╔╝╚██████╔╝██████╔╝      ██████╔╝██║  ██║██║  ██║██║███████║   ██║   ██║  ██║
  ╚═════╝╚══════╝ ╚═════╝  ╚═════╝ ╚═════╝       ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝

 ████████╗██╗   ██╗███╗   ███╗██████╗ ██╗     ███████╗██████╗ ██╗   ██╗ ██████╗
 ╚══██╔══╝██║   ██║████╗ ████║██╔══██╗██║     ██╔════╝██╔══██╗██║   ██║██╔════╝
    ██║   ██║   ██║██╔████╔██║██████╔╝██║     █████╗  ██████╔╝██║   ██║██║  ███╗
    ██║   ██║   ██║██║╚██╔╝██║██╔══██╗██║     ██╔══╝  ██╔══██╗██║   ██║██║   ██║
    ██║   ╚██████╔╝██║ ╚═╝ ██║██████╔╝███████╗███████╗██████╔╝╚██████╔╝╚██████╔╝
    ╚═╝    ╚═════╝ ╚═╝     ╚═╝╚═════╝ ╚══════╝╚══════╝╚═════╝  ╚═════╝  ╚═════╝              

 Multi-cloud infra service managemenet framework
 ________________________________________________`
)

func ApiServer() {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World! This is cloud-barista cb-tumblebug")
	})
	e.HideBanner = true
	//e.colorer.Printf(banner, e.colorer.Red("v"+Version), e.colorer.Blue(website))

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	API_USERNAME := os.Getenv("API_USERNAME")
	API_PASSWORD := os.Getenv("API_PASSWORD")
	fmt.Println(API_USERNAME, API_PASSWORD)
	e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		// Be careful to use constant time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(username), []byte(API_USERNAME)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(API_PASSWORD)) == 1 {
			return true, nil
		}
		return false, nil
	}))

	fmt.Println("")
	fmt.Println("")
	fmt.Println("")
	fmt.Println("")
	fmt.Printf(banner)
	fmt.Println("")
	fmt.Printf(ErrorColor, Version)
	fmt.Println("")
	fmt.Printf(InfoColor, website)
	fmt.Println("")
	fmt.Println("")

	// Route
	g := e.Group("/tumblebug/ns", common.NsValidation())

	g.POST("", rest_common.RestPostNs)
	g.GET("/:nsId", rest_common.RestGetNs)
	g.GET("", rest_common.RestGetAllNs)
	g.PUT("/:nsId", rest_common.RestPutNs)
	g.DELETE("/:nsId", rest_common.RestDelNs)
	g.DELETE("", rest_common.RestDelAllNs)

	g.POST("/:nsId/mcis", rest_mcis.RestPostMcis)
	g.GET("/:nsId/mcis/:mcisId", rest_mcis.RestGetMcis)
	g.GET("/:nsId/mcis", rest_mcis.RestGetAllMcis)
	g.PUT("/:nsId/mcis/:mcisId", rest_mcis.RestPutMcis)
	g.DELETE("/:nsId/mcis/:mcisId", rest_mcis.RestDelMcis)
	g.DELETE("/:nsId/mcis", rest_mcis.RestDelAllMcis)

	g.POST("/:nsId/mcis/:mcisId/vm", rest_mcis.RestPostMcisVm)
	g.GET("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestGetMcisVm)
	//g.GET("/:nsId/mcis/:mcisId/vm", rest_mcis.RestGetAllMcisVm)
	//g.PUT("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestPutMcisVm)
	g.DELETE("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestDelMcisVm)
	//g.DELETE("/:nsId/mcis/:mcisId/vm", rest_mcis.RestDelAllMcisVm)

	g.POST("/:nsId/mcis/recommend", rest_mcis.RestPostMcisRecommand)
	g.POST("/:nsId/cmd/mcis/:mcisId", rest_mcis.RestPostCmdMcis)
	g.POST("/:nsId/cmd/mcis/:mcisId/vm/:vmId", rest_mcis.RestPostCmdMcisVm)
	g.POST("/:nsId/install/mcis/:mcisId", rest_mcis.RestPostInstallAgentToMcis)
	g.GET("/:nsId/benchmark/mcis/:mcisId", rest_mcis.RestGetBenchmark)
	g.GET("/:nsId/benchmarkall/mcis/:mcisId", rest_mcis.RestGetAllBenchmark)

	g.POST("/:nsId/monitoring/install/mcis/:mcisId", rest_mcis.RestPostInstallMonitorAgentToMcis)
	g.GET("/:nsId/monitoring/mcis/:mcisId/metric/:metric", rest_mcis.RestGetMonitorData)

	g.POST("/:nsId/resources/image", rest_mcir.RestPostImage)
	g.GET("/:nsId/resources/image/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/image", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/image/:imageId", rest_mcir.RestPutImage)
	g.DELETE("/:nsId/resources/image/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/image", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/sshKey", rest_mcir.RestPostSshKey)
	g.GET("/:nsId/resources/sshKey/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/sshKey", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/sshKey/:sshKeyId", rest_mcir.RestPutSshKey)
	g.DELETE("/:nsId/resources/sshKey/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/sshKey", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/spec", rest_mcir.RestPostSpec)
	g.GET("/:nsId/resources/spec/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/spec", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/spec/:specId", rest_mcir.RestPutSpec)
	g.DELETE("/:nsId/resources/spec/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/spec", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/fetchSpecs", rest_mcir.RestFetchSpecs)

	g.POST("/:nsId/resources/securityGroup", rest_mcir.RestPostSecurityGroup)
	g.GET("/:nsId/resources/securityGroup/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/securityGroup", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/securityGroup/:securityGroupId", rest_mcir.RestPutSecurityGroup)
	g.DELETE("/:nsId/resources/securityGroup/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/securityGroup", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/vNet", rest_mcir.RestPostVNet)
	g.GET("/:nsId/resources/vNet/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/vNet", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/vNet/:vNetId", rest_mcir.RestPutVNet)
	g.DELETE("/:nsId/resources/vNet/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/vNet", rest_mcir.RestDelAllResources)

	/*
		g.POST("/:nsId/resources/subnet", mcir.RestPostSubnet)
		g.GET("/:nsId/resources/subnet/:subnetId", mcir.RestGetSubnet)
		g.GET("/:nsId/resources/subnet", mcir.RestGetAllSubnet)
		g.PUT("/:nsId/resources/subnet/:subnetId", mcir.RestPutSubnet)
		g.DELETE("/:nsId/resources/subnet/:subnetId", mcir.RestDelSubnet)
		g.DELETE("/:nsId/resources/subnet", mcir.RestDelAllSubnet)

		g.POST("/:nsId/resources/publicIp", mcir.RestPostPublicIp)
		g.GET("/:nsId/resources/publicIp/:publicIpId", mcir.RestGetPublicIp)
		g.GET("/:nsId/resources/publicIp", mcir.RestGetAllPublicIp)
		g.PUT("/:nsId/resources/publicIp/:publicIpId", mcir.RestPutPublicIp)
		g.DELETE("/:nsId/resources/publicIp/:publicIpId", mcir.RestDelPublicIp)
		g.DELETE("/:nsId/resources/publicIp", mcir.RestDelAllPublicIp)

		g.POST("/:nsId/resources/vNic", mcir.RestPostVNic)
		g.GET("/:nsId/resources/vNic/:vNicId", mcir.RestGetVNic)
		g.GET("/:nsId/resources/vNic", mcir.RestGetAllVNic)
		g.PUT("/:nsId/resources/vNic/:vNicId", mcir.RestPutVNic)
		g.DELETE("/:nsId/resources/vNic/:vNicId", mcir.RestDelVNic)
		g.DELETE("/:nsId/resources/vNic", mcir.RestDelAllVNic)
	*/

	// We cannot use these wildcard method below.
	// https://github.com/labstack/echo/issues/382
	//g.DELETE("/:nsId/resources/:resourceType/:resourceId", mcir.RestDelResource)
	//g.DELETE("/:nsId/resources/:resourceType", mcir.RestDelAllResources)

	g.GET("/:nsId/checkResource/:resourceType/:resourceId", rest_mcir.RestCheckResource)
	g.GET("/:nsId/checkMcis/:mcisId", rest_mcis.RestCheckMcis)
	g.GET("/:nsId/mcis/:mcisId/checkVm/:vmId", rest_mcis.RestCheckVm)

	e.GET("/tumblebug/checkNs/:nsId", rest_common.RestCheckNs)

	e.GET("/tumblebug/connConfig", rest_common.RestGetConnConfigList)
	e.GET("/tumblebug/connConfig/:connConfigName", rest_common.RestGetConnConfig)
	e.GET("/tumblebug/region", rest_common.RestGetRegionList)
	e.GET("/tumblebug/region/:regionName", rest_common.RestGetRegion)

	e.GET("/tumblebug/lookupSpec", rest_mcir.RestLookupSpecList)
	e.GET("/tumblebug/lookupSpec/:specName", rest_mcir.RestLookupSpec)

	e.GET("/tumblebug/webadmin", webadmin.Mainpage)
	e.GET("/tumblebug/webadmin/menu", webadmin.Menu)
	e.GET("/tumblebug/webadmin/ns", webadmin.Ns)
	e.GET("/tumblebug/webadmin/spec", webadmin.Spec)

	e.Logger.Fatal(e.Start(":1323"))

}
