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

	// "log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"

	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/api/rest/server/auth"
	rest_common "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/common"
	rest_mci "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mci"
	rest_mcir "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/api/rest/server/middlewares/authmw"
	middlewares "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/middlewares/custom-middleware"
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
	APILogSkipPatterns := [][]string{
		{"/tumblebug/api"},
		{"/mci", "option=status"},
	}
	e.Use(middlewares.Zerologger(APILogSkipPatterns))

	e.Use(middleware.Recover())
	// limit the application to 20 requests/sec using the default in-memory store
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

	// Custom middleware for RequestID and RequestDetails
	e.Use(middlewares.RequestIdAndDetailsIssuer)

	// Custom middleware for ResponseBodyDump
	e.Use(middlewares.ResponseBodyDump())

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
	e.GET("/tumblebug/readyz", rest_common.RestGetReadyz)
	e.GET("/tumblebug/httpVersion", rest_common.RestCheckHTTPVersion)

	allowedOrigins := os.Getenv("TB_ALLOW_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal().Msgf("TB_ALLOW_ORIGINS env variable for CORS is " + allowedOrigins +
			". Please provide a proper value and source setup.env again. EXITING...")
		// allowedOrigins = "*"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{allowedOrigins},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Conditions to prevent abnormal operation due to typos (e.g., ture, falss, etc.)
	authEnabled := os.Getenv("TB_AUTH_ENABLED") == "true"
	authMode := os.Getenv("TB_AUTH_MODE")

	apiUser := os.Getenv("TB_API_USERNAME")
	apiPass := os.Getenv("TB_API_PASSWORD")

	// Setup Middlewares for auth
	var basicAuthMw echo.MiddlewareFunc
	var jwtAuthMw echo.MiddlewareFunc

	if authEnabled {
		switch authMode {
		case "basic":
			// Setup Basic Auth Middleware
			basicAuthMw = middleware.BasicAuthWithConfig(middleware.BasicAuthConfig{
				Skipper: func(c echo.Context) bool {
					if c.Path() == "/tumblebug/readyz" ||
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
			})
			log.Info().Msg("Basic Auth Middleware is initialized successfully")
		case "jwt":
			// Setup JWT Auth Middleware
			err := authmw.InitJwtAuthMw(os.Getenv("TB_IAM_MANAGER_REST_URL"), "/api/auth/certs")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to initialize JWT Auth Middleware")
			} else {
				authSkipPatterns := [][]string{
					{"/tumblebug/readyz"},
					{"/tumblebug/httpVersion"},
				}
				jwtAuthMw = authmw.JwtAuthMw(authSkipPatterns)
				log.Info().Msg("JWT Auth Middleware is initialized successfully")
			}
		default:
			log.Fatal().Msg("TB_AUTH_MODE is not set properly. Please set it to 'basic' or 'jwt'. EXITING...")
		}
	}

	// Set basic auth middleware for root group
	if authEnabled && authMode == "basic" && basicAuthMw != nil {
		log.Debug().Msg("Setting up Basic Auth Middleware for root group")
		e.Use(basicAuthMw)
	}

	// [Temp - start] For JWT auth test, a route group and an API
	authGroup := e.Group("/tumblebug/auth")
	if authEnabled && authMode == "jwt" && jwtAuthMw != nil {
		log.Debug().Msg("Setting up JWT Auth Middleware for /tumblebug/auth group")
		authGroup.Use(jwtAuthMw)
	}
	authGroup.GET("/test", auth.TestJWTAuth)
	// [Temp - end] For JWT auth test, a route group and an API

	fmt.Print(banner)
	fmt.Println("\n ")
	fmt.Printf(infoColor, website)
	fmt.Println("\n \n ")

	// Route
	e.GET("/tumblebug/checkNs/:nsId", rest_common.RestCheckNs)

	e.GET("/tumblebug/cloudInfo", rest_common.RestGetCloudInfo)
	e.GET("/tumblebug/connConfig", rest_common.RestGetConnConfigList)
	e.GET("/tumblebug/connConfig/:connConfigName", rest_common.RestGetConnConfig)
	e.GET("/tumblebug/provider", rest_common.RestGetProviderList)
	e.GET("/tumblebug/region", rest_common.RestGetRegionList)
	e.GET("/tumblebug/provider/:providerName/region/:regionName", rest_common.RestGetRegion)
	e.GET("/tumblebug/k8sClusterInfo", rest_common.RestGetK8sClusterInfo)
	e.POST("/tumblebug/credential", rest_common.RestRegisterCredential)

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
	e.GET("/tumblebug/ns/:nsId/loadSharedResource", rest_mcir.RestLoadSharedResource)
	e.DELETE("/tumblebug/ns/:nsId/sharedResources", rest_mcir.RestDelAllSharedResources)

	e.POST("/tumblebug/forward/*", rest_common.RestForwardAnyReqToAny)

	// Utility for network design
	e.POST("/tumblebug/util/net/design", rest_netutil.RestPostUtilToDesignNetwork)
	e.POST("/tumblebug/util/net/validate", rest_netutil.RestPostUtilToValidateNetwork)

	// Route for NameSpace subgroup
	g := e.Group("/tumblebug/ns", common.NsValidation())

	// Route for stream response subgroup
	streamResponseGroup := e.Group("/tumblebug/stream-response/ns", common.NsValidation())

	//Namespace Management
	g.POST("", rest_common.RestPostNs)
	g.GET("/:nsId", rest_common.RestGetNs)
	g.GET("", rest_common.RestGetAllNs)
	g.PUT("/:nsId", rest_common.RestPutNs)
	g.DELETE("/:nsId", rest_common.RestDelNs)
	g.DELETE("", rest_common.RestDelAllNs)

	//MCI Management
	g.POST("/:nsId/mci", rest_mci.RestPostMci)
	g.POST("/:nsId/registerCspVm", rest_mci.RestPostRegisterCSPNativeVM)

	e.POST("/tumblebug/mciRecommendVm", rest_mci.RestRecommendVm)
	e.POST("/tumblebug/mciDynamicCheckRequest", rest_mci.RestPostMciDynamicCheckRequest)
	e.POST("/tumblebug/systemMci", rest_mci.RestPostSystemMci)

	g.POST("/:nsId/mciDynamic", rest_mci.RestPostMciDynamic)
	g.POST("/:nsId/mci/:mciId/vmDynamic", rest_mci.RestPostMciVmDynamic)

	//g.GET("/:nsId/mci/:mciId", rest_mci.RestGetMci, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	//g.GET("/:nsId/mci", rest_mci.RestGetAllMci, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	// path specific timeout and ratelimit
	// timeout middleware
	timeoutConfig := middleware.TimeoutConfig{
		Timeout:      60 * time.Second,
		Skipper:      middleware.DefaultSkipper,
		ErrorMessage: "Error: request time out (60s)",
	}

	g.GET("/:nsId/mci/:mciId", rest_mci.RestGetMci, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/mci", rest_mci.RestGetAllMci, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))

	// g.PUT("/:nsId/mci/:mciId", rest_mci.RestPutMci)
	g.DELETE("/:nsId/mci/:mciId", rest_mci.RestDelMci)
	g.DELETE("/:nsId/mci", rest_mci.RestDelAllMci)

	g.POST("/:nsId/mci/:mciId/vm", rest_mci.RestPostMciVm)
	g.GET("/:nsId/mci/:mciId/vm/:vmId", rest_mci.RestGetMciVm)
	g.GET("/:nsId/mci/:mciId/subgroup", rest_mci.RestGetMciGroupIds)
	g.GET("/:nsId/mci/:mciId/subgroup/:subgroupId", rest_mci.RestGetMciGroupVms)
	g.POST("/:nsId/mci/:mciId/subgroup/:subgroupId", rest_mci.RestPostMciSubGroupScaleOut)

	//g.GET("/:nsId/mci/:mciId/vm", rest_mci.RestGetAllMciVm)
	// g.PUT("/:nsId/mci/:mciId/vm/:vmId", rest_mci.RestPutMciVm)
	g.DELETE("/:nsId/mci/:mciId/vm/:vmId", rest_mci.RestDelMciVm)
	//g.DELETE("/:nsId/mci/:mciId/vm", rest_mci.RestDelAllMciVm)

	//g.POST("/:nsId/mci/recommend", rest_mci.RestPostMciRecommend)

	g.GET("/:nsId/control/mci/:mciId", rest_mci.RestGetControlMci)
	g.GET("/:nsId/control/mci/:mciId/vm/:vmId", rest_mci.RestGetControlMciVm)

	g.POST("/:nsId/cmd/mci/:mciId", rest_mci.RestPostCmdMci)
	g.PUT("/:nsId/mci/:mciId/vm/:targetVmId/bastion/:bastionVmId", rest_mci.RestSetBastionNodes)
	g.DELETE("/:nsId/mci/:mciId/bastion/:bastionVmId", rest_mci.RestRemoveBastionNodes)
	g.GET("/:nsId/mci/:mciId/vm/:targetVmId/bastion", rest_mci.RestGetBastionNodes)

	g.POST("/:nsId/installBenchmarkAgent/mci/:mciId", rest_mci.RestPostInstallBenchmarkAgentToMci)
	g.POST("/:nsId/benchmark/mci/:mciId", rest_mci.RestGetBenchmark)
	g.POST("/:nsId/benchmarkAll/mci/:mciId", rest_mci.RestGetAllBenchmark)
	g.GET("/:nsId/benchmarkLatency/mci/:mciId", rest_mci.RestGetBenchmarkLatency)

	// VPN Sites info
	g.GET("/:nsId/mci/:mciId/site", rest_mci.RestGetSitesInMci)

	// Site-to-stie VPN management
	streamResponseGroup.POST("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestPostSiteToSiteVpn)
	g.GET("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestGetSiteToSiteVpn)
	streamResponseGroup.PUT("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestPutSiteToSiteVpn)
	streamResponseGroup.DELETE("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestDeleteSiteToSiteVpn)
	g.GET("/:nsId/mci/:mciId/vpn/:vpnId/request/:requestId", rest_mci.RestGetRequestStatusOfSiteToSiteVpn)
	// TBD
	// g.POST("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestPostVpnGcpToAws)
	// g.PUT("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestPutVpnGcpToAws)
	// g.DELETE("/:nsId/mci/:mciId/vpn/:vpnId", rest_mci.RestDeleteVpnGcpToAws)

	//MCI AUTO Policy
	g.POST("/:nsId/policy/mci/:mciId", rest_mci.RestPostMciPolicy)
	g.GET("/:nsId/policy/mci/:mciId", rest_mci.RestGetMciPolicy)
	g.GET("/:nsId/policy/mci", rest_mci.RestGetAllMciPolicy)
	g.PUT("/:nsId/policy/mci/:mciId", rest_mci.RestPutMciPolicy)
	g.DELETE("/:nsId/policy/mci/:mciId", rest_mci.RestDelMciPolicy)
	g.DELETE("/:nsId/policy/mci", rest_mci.RestDelAllMciPolicy)

	g.POST("/:nsId/monitoring/install/mci/:mciId", rest_mci.RestPostInstallMonitorAgentToMci)
	g.GET("/:nsId/monitoring/mci/:mciId/metric/:metric", rest_mci.RestGetMonitorData)
	g.PUT("/:nsId/monitoring/status/mci/:mciId/vm/:vmId", rest_mci.RestPutMonitorAgentStatusInstalled)

	// K8sCluster
	e.GET("/tumblebug/availableK8sClusterVersion", rest_mci.RestGetAvailableK8sClusterVersion)
	e.GET("/tumblebug/availableK8sClusterNodeImage", rest_mci.RestGetAvailableK8sClusterNodeImage)
	e.GET("/tumblebug/checkNodeGroupsOnK8sCreation", rest_mci.RestCheckNodeGroupsOnK8sCreation)
	g.POST("/:nsId/k8scluster", rest_mci.RestPostK8sCluster)
	g.POST("/:nsId/k8scluster/:k8sClusterId/k8snodegroup", rest_mci.RestPostK8sNodeGroup)
	g.DELETE("/:nsId/k8scluster/:k8sClusterId/k8snodegroup/:k8sNodeGroupName", rest_mci.RestDeleteK8sNodeGroup)
	g.PUT("/:nsId/k8scluster/:k8sClusterId/k8snodegroup/:k8sNodeGroupName/onautoscaling", rest_mci.RestPutSetK8sNodeGroupAutoscaling)
	g.PUT("/:nsId/k8scluster/:k8sClusterId/k8snodegroup/:k8sNodeGroupName/autoscalesize", rest_mci.RestPutChangeK8sNodeGroupAutoscaleSize)
	g.GET("/:nsId/k8scluster/:k8sClusterId", rest_mci.RestGetK8sCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/k8scluster", rest_mci.RestGetAllK8sCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.DELETE("/:nsId/k8scluster/:k8sClusterId", rest_mci.RestDeleteK8sCluster)
	g.DELETE("/:nsId/k8scluster", rest_mci.RestDeleteAllK8sCluster)
	g.PUT("/:nsId/k8scluster/:k8sClusterId/upgrade", rest_mci.RestPutUpgradeK8sCluster)

	// Network Load Balancer
	g.POST("/:nsId/mci/:mciId/mcSwNlb", rest_mci.RestPostMcNLB)
	g.POST("/:nsId/mci/:mciId/nlb", rest_mci.RestPostNLB)
	g.GET("/:nsId/mci/:mciId/nlb/:resourceId", rest_mci.RestGetNLB)
	g.GET("/:nsId/mci/:mciId/nlb", rest_mci.RestGetAllNLB)
	// g.PUT("/:nsId/mci/:mciId/nlb/:resourceId", rest_mci.RestPutNLB)
	g.DELETE("/:nsId/mci/:mciId/nlb/:resourceId", rest_mci.RestDelNLB)
	g.DELETE("/:nsId/mci/:mciId/nlb", rest_mci.RestDelAllNLB)
	g.GET("/:nsId/mci/:mciId/nlb/:resourceId/healthz", rest_mci.RestGetNLBHealth)

	// VM snapshot -> creates one customImage and 'n' dataDisks
	g.POST("/:nsId/mci/:mciId/vm/:vmId/snapshot", rest_mci.RestPostMciVmSnapshot)

	// These REST APIs are for dev/test only
	g.POST("/:nsId/mci/:mciId/nlb/:resourceId/vm", rest_mci.RestAddNLBVMs)
	g.DELETE("/:nsId/mci/:mciId/nlb/:resourceId/vm", rest_mci.RestRemoveNLBVMs)

	//MCIR Management
	g.POST("/:nsId/resources/dataDisk", rest_mcir.RestPostDataDisk)
	g.GET("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestGetResource)
	g.GET("/:nsId/resources/dataDisk", rest_mcir.RestGetAllResources)
	g.PUT("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestPutDataDisk)
	g.DELETE("/:nsId/resources/dataDisk/:resourceId", rest_mcir.RestDelResource)
	g.DELETE("/:nsId/resources/dataDisk", rest_mcir.RestDelAllResources)
	g.GET("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_mcir.RestGetVmDataDisk)
	g.POST("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_mcir.RestPostVmDataDisk)
	g.PUT("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_mcir.RestPutVmDataDisk)

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
	g.GET("/:nsId/resources/spec/:resourceId", rest_mcir.RestGetSpec)
	g.PUT("/:nsId/resources/spec/:resourceId", rest_mcir.RestPutSpec)
	g.DELETE("/:nsId/resources/spec/:resourceId", rest_mcir.RestDelResource)

	g.POST("/:nsId/resources/fetchSpecs", rest_mcir.RestFetchSpecs)
	g.POST("/:nsId/resources/filterSpecsByRange", rest_mcir.RestFilterSpecsByRange)

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
	// g.DELETE("/:nsId/resources/vNet/:parentResourceId/subnet/:childResourceId", rest_mcir.RestDelChildResource)
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
	g.GET("/:nsId/checkMci/:mciId", rest_mci.RestCheckMci)
	g.GET("/:nsId/mci/:mciId/checkVm/:vmId", rest_mci.RestCheckVm)

	// g.POST("/:nsId/registerExistingResources", rest_mcir.RestRegisterExistingResources)

	// Temporal test API for development of UpdateAssociatedObjectList
	g.PUT("/:nsId/testAddObjectAssociation/:resourceType/:resourceId", rest_mcir.RestTestAddObjectAssociation)
	g.PUT("/:nsId/testDeleteObjectAssociation/:resourceType/:resourceId", rest_mcir.RestTestDeleteObjectAssociation)
	g.GET("/:nsId/testGetAssociatedObjectCount/:resourceType/:resourceId", rest_mcir.RestTestGetAssociatedObjectCount)

	selfEndpoint := os.Getenv("TB_SELF_ENDPOINT")
	apidashboard := " http://" + selfEndpoint + "/tumblebug/api"

	fmt.Println(" Default Namespace: " + common.DefaultNamespace)
	fmt.Println(" Default CredentialHolder: " + common.DefaultCredentialHolder + "\n")

	if authEnabled {
		fmt.Println(" Access to API dashboard" + " (username: " + apiUser + " / password: " + apiPass + ")")
	}

	fmt.Printf(noticeColor, apidashboard)
	fmt.Println("\n ")

	// A context for graceful shutdown (It is based on the signal package)selfEndpoint := os.Getenv("TB_SELF_ENDPOINT")
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

		log.Info().Msg("Stopping CB-Tumblebug API Server gracefully... (within 10s)")
		ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error in Gracefully Stopping CB-Tumblebug API Server")
			e.Logger.Panic(err)
		}
	}(&wg)

	port = fmt.Sprintf(":%s", port)
	common.SystemReady = true
	if err := e.Start(port); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Error in Starting CB-Tumblebug API Server")
		e.Logger.Panic("Shuttig down the server: ", err)
	}

	wg.Wait()
}
