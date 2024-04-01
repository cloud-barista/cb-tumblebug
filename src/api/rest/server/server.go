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

// Package server is to handle REST API
package server

import (
	"context"
	"encoding/json"

	// "log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"

	"github.com/rs/zerolog/log"

	rest_common "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/common"
	rest_mcir "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcir"
	rest_mcis "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcis"
	"github.com/cloud-barista/cb-tumblebug/src/api/rest/server/middlewares"
	rest_netutil "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/util"

	"crypto/subtle"
	"fmt"
	"os"

	"net/http"

	// REST API (echo)
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// echo-swagger middleware
	_ "github.com/cloud-barista/cb-tumblebug/src/api/rest/docs"
	echoSwagger "github.com/swaggo/echo-swagger"
)

//var masterConfigInfos confighandler.MASTERCONFIGTYPE

const (
	infoColor    = "\033[1;34m%s\033[0m"
	noticeColor  = "\033[1;36m%s\033[0m"
	warningColor = "\033[1;33m%s\033[0m"
	errorColor   = "\033[1;31m%s\033[0m"
	debugColor   = "\033[0;36m%s\033[0m"
)

const (
	website = " https://github.com/cloud-barista/cb-tumblebug"
	banner  = `

  ██████╗██████╗    ████████╗██████╗      
 ██╔════╝██╔══██╗   ╚══██╔══╝██╔══██╗     
 ██║     ██████╔╝█████╗██║   ██████╔╝     
 ██║     ██╔══██╗╚════╝██║   ██╔══██╗     
 ╚██████╗██████╔╝      ██║   ██████╔╝     
  ╚═════╝╚═════╝       ╚═╝   ╚═════╝      
                                         
 ██████╗ ███████╗ █████╗ ██████╗ ██╗   ██╗
 ██╔══██╗██╔════╝██╔══██╗██╔══██╗╚██╗ ██╔╝
 ██████╔╝█████╗  ███████║██║  ██║ ╚████╔╝ 
 ██╔══██╗██╔══╝  ██╔══██║██║  ██║  ╚██╔╝  
 ██║  ██║███████╗██║  ██║██████╔╝   ██║   
 ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═════╝    ╚═╝   

 Multi-cloud infrastructure management framework
 ________________________________________________`
)

// RunServer func start Rest API server
func RunServer(port string) {

	log.Info().Msg("REST API Server is starting")

	e := echo.New()

	// Middleware
	// e.Use(middleware.Logger())
	e.Use(middlewares.Zerologger())
	e.Use(middleware.Recover())
	// limit the application to 20 requests/sec using the default in-memory store
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

	// Customized middleware for request logging
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Logger().Printf("Start - RequestLog()")
			// make X-Request-Id visible to all handlers
			c.Response().Header().Set("Access-Control-Expose-Headers", echo.HeaderXRequestID)

			// Get or generate Request ID
			reqID := c.Request().Header.Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = fmt.Sprintf("%d", time.Now().UnixNano())
			}

			// Set Request on the context
			c.Set("RequestID", reqID)

			c.Logger().Printf("Request ID: %s", reqID)
			if _, ok := common.RequestMap.Load(reqID); ok {
				return fmt.Errorf("the x-request-id is already in use")
			}

			details := common.RequestDetails{
				StartTime:   time.Now(),
				Status:      "Handling",
				RequestInfo: common.ExtractRequestInfo(c.Request()),
			}
			common.RequestMap.Store(reqID, details)

			c.Logger().Printf("End - RequestLog()")

			return next(c)
		}
	})

	e.Use(middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Skipper: func(c echo.Context) bool {
			if c.Path() == "/tumblebug/swagger" {
				return true
			}
			return false
		},
		Handler: func(c echo.Context, reqBody, resBody []byte) {
			c.Logger().Printf("Start - BodyDump()")

			reqID := c.Get("RequestID").(string)
			c.Logger().Printf("Request ID: %s", reqID)
			if v, ok := common.RequestMap.Load(reqID); ok {
				c.Logger().Printf("OK, common.RequestMap.Load(reqID)")
				details := v.(common.RequestDetails)
				details.EndTime = time.Now()

				c.Response().Header().Set("X-Request-ID", reqID)

				// 1XX: Information responses
				// 2XX: Successful responses (200 OK, 201 Created, 202 Accepted, 204 No Content)
				// 3XX: Redirection messages
				// 4XX: Client error responses (400 Bad Request, 401 Unauthorized, 404 Not Found, 408 Request Timeout)
				// 5XX: Server error responses (500 Internal Server Error, 501 Not Implemented, 503 Service Unavailable)
				if c.Response().Status >= 400 && c.Response().Status <= 599 {
					c.Logger().Printf("Error, c.Response().Status")
					var resMap map[string]interface{}
					err := json.Unmarshal(resBody, &resMap)
					if err != nil {
						// handle error
						c.Logger().Printf("Error while unmarshaling response body: %s", err)
					}

					details.Status = "Error"
					details.ErrorResponse = resMap["message"].(string)
				} else {
					c.Logger().Printf("Not error, c.Response().Status")
					details.Status = "Success"
					details.ResponseData = resBody
				}
				common.RequestMap.Store(reqID, details)
			}
			c.Logger().Printf("End - BodyDump()")
		},
	}))

	e.HideBanner = true
	//e.colorer.Printf(banner, e.colorer.Red("v"+Version), e.colorer.Blue(website))

	// Route for system management
	swaggerRedirect := func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/tumblebug/api/index.html")
	}
	e.GET("/tumblebug/api", swaggerRedirect)
	e.GET("/tumblebug/api/", swaggerRedirect)
	e.GET("/tumblebug/api/*", echoSwagger.WrapHandler)

	// e.GET("/tumblebug/swagger/*", echoSwagger.WrapHandler)
	// e.GET("/tumblebug/swaggerActive", rest_common.RestGetSwagger)
	e.GET("/tumblebug/health", rest_common.RestGetHealth)
	e.GET("/tumblebug/httpVersion", rest_common.RestCheckHTTPVersion)

	allowedOrigins := os.Getenv("ALLOW_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal().Msgf("ALLOW_ORIGINS env variable for CORS is " + allowedOrigins +
			". Please provide a proper value and source setup.env again. EXITING...")
		// allowedOrigins = "*"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{allowedOrigins},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Conditions to prevent abnormal operation due to typos (e.g., ture, falss, etc.)
	enableAuth := os.Getenv("ENABLE_AUTH") == "true"

	apiUser := os.Getenv("API_USERNAME")
	apiPass := os.Getenv("API_PASSWORD")

	if enableAuth {
		e.Use(middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
			Skipper: func(c echo.Context) bool {
				if c.Path() == "/tumblebug/health" ||
					c.Path() == "/tumblebug/httpVersion" {
					return true
				}
				return false
			},
			Validator: func(username, password string, c echo.Context) (bool, error) {
				// Be careful to use constant time comparison to prevent timing attacks
				if subtle.ConstantTimeCompare([]byte(username), []byte(apiUser)) == 1 &&
					subtle.ConstantTimeCompare([]byte(password), []byte(apiPass)) == 1 {
					return true, nil
				}
				return false, nil
			},
		}))
	}

	fmt.Println("\n \n ")
	fmt.Printf(banner)
	fmt.Println("\n ")
	fmt.Printf(infoColor, website)
	fmt.Println("\n \n ")

	// Route
	e.GET("/tumblebug/checkNs/:nsId", rest_common.RestCheckNs)

	e.GET("/tumblebug/connConfig", rest_common.RestGetConnConfigList)
	e.GET("/tumblebug/connConfig/:connConfigName", rest_common.RestGetConnConfig)
	e.GET("/tumblebug/region", rest_common.RestGetRegionList)
	e.GET("/tumblebug/region/:regionName", rest_common.RestGetRegion)

	e.POST("/tumblebug/lookupSpecs", rest_mcir.RestLookupSpecList)
	e.POST("/tumblebug/lookupSpec", rest_mcir.RestLookupSpec)

	e.POST("/tumblebug/lookupImages", rest_mcir.RestLookupImageList)
	e.POST("/tumblebug/lookupImage", rest_mcir.RestLookupImage)

	e.POST("/tumblebug/inspectResources", rest_common.RestInspectResources)
	e.GET("/tumblebug/inspectResourcesOverview", rest_common.RestInspectResourcesOverview)

	e.POST("/tumblebug/registerCspResources", rest_common.RestRegisterCspNativeResources)
	e.POST("/tumblebug/registerCspResourcesAll", rest_common.RestRegisterCspNativeResourcesAll)

	// @Tags [Admin] System environment
	e.POST("/tumblebug/config", rest_common.RestPostConfig)
	e.GET("/tumblebug/config/:configId", rest_common.RestGetConfig)
	e.GET("/tumblebug/config", rest_common.RestGetAllConfig)
	e.DELETE("/tumblebug/config/:configId", rest_common.RestInitConfig)
	e.DELETE("/tumblebug/config", rest_common.RestInitAllConfig)

	e.GET("/tumblebug/request/:reqId", rest_common.RestGetRequest)
	e.GET("/tumblebug/requests", rest_common.RestGetAllRequests)
	e.DELETE("/tumblebug/request/:reqId", rest_common.RestDeleteRequest)
	e.DELETE("/tumblebug/requests", rest_common.RestDeleteAllRequests)

	e.GET("/tumblebug/object", rest_common.RestGetObject)
	e.GET("/tumblebug/objects", rest_common.RestGetObjects)
	e.DELETE("/tumblebug/object", rest_common.RestDeleteObject)
	e.DELETE("/tumblebug/objects", rest_common.RestDeleteObjects)

	e.GET("/tumblebug/loadCommonResource", rest_mcir.RestLoadCommonResource)
	e.GET("/tumblebug/ns/:nsId/loadDefaultResource", rest_mcir.RestLoadDefaultResource)
	e.DELETE("/tumblebug/ns/:nsId/defaultResources", rest_mcir.RestDelAllDefaultResources)

	e.POST("/tumblebug/forward/*", rest_common.RestForwardAnyReqToAny)

	// Utility for network design
	e.POST("/tumblebug/util/net/design", rest_netutil.RestPostUtilToDesignNetwork)
	e.POST("/tumblebug/util/net/validate", rest_netutil.RestPostUtilToValidateNetwork)

	// Route for NameSpace subgroup
	g := e.Group("/tumblebug/ns", common.NsValidation())

	//Namespace Management
	g.POST("", rest_common.RestPostNs)
	g.GET("/:nsId", rest_common.RestGetNs)
	g.GET("", rest_common.RestGetAllNs)
	g.PUT("/:nsId", rest_common.RestPutNs)
	g.DELETE("/:nsId", rest_common.RestDelNs)
	g.DELETE("", rest_common.RestDelAllNs)

	//MCIS Management
	g.POST("/:nsId/mcis", rest_mcis.RestPostMcis)
	g.POST("/:nsId/registerCspVm", rest_mcis.RestPostRegisterCSPNativeVM)

	e.POST("/tumblebug/mcisRecommendVm", rest_mcis.RestRecommendVm)
	e.POST("/tumblebug/mcisDynamicCheckRequest", rest_mcis.RestPostMcisDynamicCheckRequest)
	e.POST("/tumblebug/systemMcis", rest_mcis.RestPostSystemMcis)

	g.POST("/:nsId/mcisDynamic", rest_mcis.RestPostMcisDynamic)
	g.POST("/:nsId/mcis/:mcisId/vmDynamic", rest_mcis.RestPostMcisVmDynamic)

	//g.GET("/:nsId/mcis/:mcisId", rest_mcis.RestGetMcis, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	//g.GET("/:nsId/mcis", rest_mcis.RestGetAllMcis, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	// path specific timeout and ratelimit
	// timeout middleware
	timeoutConfig := middleware.TimeoutConfig{
		Timeout:      60 * time.Second,
		Skipper:      middleware.DefaultSkipper,
		ErrorMessage: "Error: request time out (60s)",
	}

	g.GET("/:nsId/mcis/:mcisId", rest_mcis.RestGetMcis, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/mcis", rest_mcis.RestGetAllMcis, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))

	// g.PUT("/:nsId/mcis/:mcisId", rest_mcis.RestPutMcis)
	g.DELETE("/:nsId/mcis/:mcisId", rest_mcis.RestDelMcis)
	g.DELETE("/:nsId/mcis", rest_mcis.RestDelAllMcis)

	g.POST("/:nsId/mcis/:mcisId/vm", rest_mcis.RestPostMcisVm)
	g.GET("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestGetMcisVm)
	g.GET("/:nsId/mcis/:mcisId/subgroup", rest_mcis.RestGetMcisGroupIds)
	g.GET("/:nsId/mcis/:mcisId/subgroup/:subgroupId", rest_mcis.RestGetMcisGroupVms)
	g.POST("/:nsId/mcis/:mcisId/subgroup/:subgroupId", rest_mcis.RestPostMcisSubGroupScaleOut)

	//g.GET("/:nsId/mcis/:mcisId/vm", rest_mcis.RestGetAllMcisVm)
	// g.PUT("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestPutMcisVm)
	g.DELETE("/:nsId/mcis/:mcisId/vm/:vmId", rest_mcis.RestDelMcisVm)
	//g.DELETE("/:nsId/mcis/:mcisId/vm", rest_mcis.RestDelAllMcisVm)

	//g.POST("/:nsId/mcis/recommend", rest_mcis.RestPostMcisRecommend)

	g.GET("/:nsId/control/mcis/:mcisId", rest_mcis.RestGetControlMcis)
	g.GET("/:nsId/control/mcis/:mcisId/vm/:vmId", rest_mcis.RestGetControlMcisVm)

	g.POST("/:nsId/cmd/mcis/:mcisId", rest_mcis.RestPostCmdMcis)
	g.PUT("/:nsId/mcis/:mcisId/vm/:targetVmId/bastion/:bastionVmId", rest_mcis.RestSetBastionNodes)
	g.DELETE("/:nsId/mcis/:mcisId/bastion/:bastionVmId", rest_mcis.RestRemoveBastionNodes)
	g.GET("/:nsId/mcis/:mcisId/vm/:targetVmId/bastion", rest_mcis.RestGetBastionNodes)

	g.POST("/:nsId/installBenchmarkAgent/mcis/:mcisId", rest_mcis.RestPostInstallBenchmarkAgentToMcis)
	g.POST("/:nsId/benchmark/mcis/:mcisId", rest_mcis.RestGetBenchmark)
	g.POST("/:nsId/benchmarkAll/mcis/:mcisId", rest_mcis.RestGetAllBenchmark)
	g.GET("/:nsId/benchmarkLatency/mcis/:mcisId", rest_mcis.RestGetBenchmarkLatency)

	//MCIS AUTO Policy
	g.POST("/:nsId/policy/mcis/:mcisId", rest_mcis.RestPostMcisPolicy)
	g.GET("/:nsId/policy/mcis/:mcisId", rest_mcis.RestGetMcisPolicy)
	g.GET("/:nsId/policy/mcis", rest_mcis.RestGetAllMcisPolicy)
	g.PUT("/:nsId/policy/mcis/:mcisId", rest_mcis.RestPutMcisPolicy)
	g.DELETE("/:nsId/policy/mcis/:mcisId", rest_mcis.RestDelMcisPolicy)
	g.DELETE("/:nsId/policy/mcis", rest_mcis.RestDelAllMcisPolicy)

	g.POST("/:nsId/monitoring/install/mcis/:mcisId", rest_mcis.RestPostInstallMonitorAgentToMcis)
	g.GET("/:nsId/monitoring/mcis/:mcisId/metric/:metric", rest_mcis.RestGetMonitorData)
	g.PUT("/:nsId/monitoring/status/mcis/:mcisId/vm/:vmId", rest_mcis.RestPutMonitorAgentStatusInstalled)

	// MCIS Cloud Adaptive Network (for developer)
	g.POST("/:nsId/network/mcis/:mcisId", rest_mcis.RestPostConfigureCloudAdaptiveNetworkToMcis)
	g.PUT("/:nsId/network/mcis/:mcisId", rest_mcis.RestPutInjectCloudInformationForCloudAdaptiveNetwork)

	// Cluster
	g.POST("/:nsId/cluster", rest_mcis.RestPostCluster)
	g.POST("/:nsId/cluster/:clusterId/nodegroup", rest_mcis.RestPostNodeGroup)
	g.DELETE("/:nsId/cluster/:clusterId/nodegroup/:nodeGroupName", rest_mcis.RestDeleteNodeGroup)
	g.PUT("/:nsId/cluster/:clusterId/nodegroup/:nodeGroupName/onautoscaling", rest_mcis.RestPutSetAutoscaling)
	g.PUT("/:nsId/cluster/:clusterId/nodegroup/:nodeGroupName/autoscalesize", rest_mcis.RestPutChangeAutoscaleSize)
	g.GET("/:nsId/cluster/:clusterId", rest_mcis.RestGetCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/cluster", rest_mcis.RestGetAllCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.DELETE("/:nsId/cluster/:clusterId", rest_mcis.RestDeleteCluster)
	g.DELETE("/:nsId/cluster", rest_mcis.RestDeleteAllCluster)
	g.PUT("/:nsId/cluster/:clusterId/upgrade", rest_mcis.RestPutUpgradeCluster)

	// Network Load Balancer
	g.POST("/:nsId/mcis/:mcisId/mcSwNlb", rest_mcis.RestPostMcNLB)
	g.POST("/:nsId/mcis/:mcisId/nlb", rest_mcis.RestPostNLB)
	g.GET("/:nsId/mcis/:mcisId/nlb/:resourceId", rest_mcis.RestGetNLB)
	g.GET("/:nsId/mcis/:mcisId/nlb", rest_mcis.RestGetAllNLB)
	// g.PUT("/:nsId/mcis/:mcisId/nlb/:resourceId", rest_mcis.RestPutNLB)
	g.DELETE("/:nsId/mcis/:mcisId/nlb/:resourceId", rest_mcis.RestDelNLB)
	g.DELETE("/:nsId/mcis/:mcisId/nlb", rest_mcis.RestDelAllNLB)
	g.GET("/:nsId/mcis/:mcisId/nlb/:resourceId/healthz", rest_mcis.RestGetNLBHealth)

	// VM snapshot -> creates one customImage and 'n' dataDisks
	g.POST("/:nsId/mcis/:mcisId/vm/:vmId/snapshot", rest_mcis.RestPostMcisVmSnapshot)

	// These REST APIs are for dev/test only
	g.POST("/:nsId/mcis/:mcisId/nlb/:resourceId/vm", rest_mcis.RestAddNLBVMs)
	g.DELETE("/:nsId/mcis/:mcisId/nlb/:resourceId/vm", rest_mcis.RestRemoveNLBVMs)

	//MCIR Management
	g.POST("/:nsId/resources/dataDisk", rest_mcir.RestPostDataDisk)
	g.GET("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/dataDisk", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestPutDataDisk)
	g.DELETE("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/dataDisk", rest_mcir.RestDelAllResources)
	g.GET("/:nsId/mcis/:mcisId/vm/:vmId/dataDisk", rest_mcir.RestGetVmDataDisk)
	g.POST("/:nsId/mcis/:mcisId/vm/:vmId/dataDisk", rest_mcir.RestPostVmDataDisk)
	g.PUT("/:nsId/mcis/:mcisId/vm/:vmId/dataDisk", rest_mcir.RestPutVmDataDisk)

	g.POST("/:nsId/resources/image", rest_mcir.RestPostImage)
	g.GET("/:nsId/resources/image/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/image", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/image/:resourceId", rest_mcir.RestPutImage)
	g.DELETE("/:nsId/resources/image/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/image", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/customImage", rest_mcir.RestPostCustomImage)
	g.GET("/:nsId/resources/customImage/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/customImage", rest_mcir.RestGetAllResources)
	// g.PUT("/:nsId/resources/customImage/:resourceId", rest_mcir.RestPutCustomImage)
	g.DELETE("/:nsId/resources/customImage/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/customImage", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/sshKey", rest_mcir.RestPostSshKey)
	g.GET("/:nsId/resources/sshKey/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/sshKey", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/sshKey/:resourceId", rest_mcir.RestPutSshKey)
	g.DELETE("/:nsId/resources/sshKey/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/sshKey", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/spec", rest_mcir.RestPostSpec)
	g.GET("/:nsId/resources/spec/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/spec", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/spec/:resourceId", rest_mcir.RestPutSpec)
	g.DELETE("/:nsId/resources/spec/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/spec", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/fetchSpecs", rest_mcir.RestFetchSpecs)
	g.POST("/:nsId/resources/filterSpecs", rest_mcir.RestFilterSpecs)
	g.POST("/:nsId/resources/filterSpecsByRange", rest_mcir.RestFilterSpecsByRange)
	g.POST("/:nsId/resources/testSortSpecs", rest_mcir.RestTestSortSpecs)

	g.POST("/:nsId/resources/fetchImages", rest_mcir.RestFetchImages)
	g.POST("/:nsId/resources/searchImage", rest_mcir.RestSearchImage)

	g.POST("/:nsId/resources/securityGroup", rest_mcir.RestPostSecurityGroup)
	g.GET("/:nsId/resources/securityGroup/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/securityGroup", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/securityGroup/:resourceId", rest_mcir.RestPutSecurityGroup)
	g.DELETE("/:nsId/resources/securityGroup/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/securityGroup", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/securityGroup/:securityGroupId/rules", rest_mcir.RestPostFirewallRules)
	g.DELETE("/:nsId/resources/securityGroup/:securityGroupId/rules", rest_mcir.RestDelFirewallRules)

	g.POST("/:nsId/resources/vNet", rest_mcir.RestPostVNet)
	g.GET("/:nsId/resources/vNet/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/vNet", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/vNet/:resourceId", rest_mcir.RestPutVNet)
	g.DELETE("/:nsId/resources/vNet/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/vNet", rest_mcir.RestDelAllResources)

	g.POST("/:nsId/resources/vNet/:vNetId/subnet", rest_mcir.RestPostSubnet)
	// g.GET("/:nsId/resources/vNet/:vNetId/subnet/:subnetId", rest_mcir.RestGetSubnet)
	// g.GET("/:nsId/resources/vNet/:vNetId/subnet", rest_mcir.RestGetAllSubnet)
	// g.PUT("/:nsId/resources/vNet/:vNetId/subnet/:subnetId", rest_mcir.RestPutSubnet)
	g.DELETE("/:nsId/resources/vNet/:parentResourceId/subnet/:childResourceId", rest_mcir.RestDelChildResource)
	// g.DELETE("/:nsId/resources/vNet/:vNetId/subnet", rest_mcir.RestDelAllSubnet)

	/*
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

	// g.POST("/:nsId/registerExistingResources", rest_mcir.RestRegisterExistingResources)

	// Temporal test API for development of UpdateAssociatedObjectList
	g.PUT("/:nsId/testAddObjectAssociation/:resourceType/:resourceId", rest_mcir.RestTestAddObjectAssociation)
	g.PUT("/:nsId/testDeleteObjectAssociation/:resourceType/:resourceId", rest_mcir.RestTestDeleteObjectAssociation)
	g.GET("/:nsId/testGetAssociatedObjectCount/:resourceType/:resourceId", rest_mcir.RestTestGetAssociatedObjectCount)

	selfEndpoint := os.Getenv("SELF_ENDPOINT")
	apidashboard := " http://" + selfEndpoint + "/tumblebug/api"

	if enableAuth {
		fmt.Println(" Access to API dashboard" + " (username: " + apiUser + " / password: " + apiPass + ")")
	}
	fmt.Printf(noticeColor, apidashboard)
	fmt.Println("\n ")

	// A context for graceful shutdown (It is based on the signal package)selfEndpoint := os.Getenv("SELF_ENDPOINT")
	// NOTE -
	// Use os.Interrupt Ctrl+C or Ctrl+Break on Windows
	// Use syscall.KILL for Kill(can't be caught or ignored) (POSIX)
	// Use syscall.SIGTERM for Termination (ANSI)
	// Use syscall.SIGINT for Terminal interrupt (ANSI)
	// Use syscall.SIGQUIT for Terminal quit (POSIX)
	gracefulShutdownContext, stop := signal.NotifyContext(context.TODO(),
		os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	// Wait graceful shutdown (and then main thread will be finished)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()

		// Block until a signal is triggered
		<-gracefulShutdownContext.Done()

		fmt.Println("\n[Stop] CB-Tumblebug REST Server")
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error starting the server")
			e.Logger.Panic(err)
		}
	}(&wg)

	port = fmt.Sprintf(":%s", port)
	if err := e.Start(port); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Error starting the server")
		e.Logger.Panic("Shuttig down the server: ", err)
	}

	wg.Wait()
}
