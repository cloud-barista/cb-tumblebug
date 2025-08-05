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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"golang.org/x/crypto/bcrypt"

	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/interface/rest/docs"
	"github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/auth"

	rest_common "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/common"
	rest_label "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/common/label"
	rest_infra "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/infra"
	"github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/middlewares"
	"github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/middlewares/authmw"
	rest_resource "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/resource"
	rest_netutil "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server/util"

	"crypto/subtle"
	"fmt"
	"os"

	"net/http"

	// REST API (echo)
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// echo-swagger middleware
	_ "github.com/cloud-barista/cb-tumblebug/src/interface/rest/docs"
	echoSwagger "github.com/swaggo/echo-swagger"
)

//var masterConfigInfos confighandler.MASTERCONFIGTYPE

const (
	infoColor    = "\033[1;34m%s\033[0m"
	titleColor   = "\033[1;34m"
	noticeColor  = "\033[1;36m%s\033[0m"
	warningColor = "\033[1;33m"
	errorColor   = "\033[1;31m%s\033[0m"
	debugColor   = "\033[0;36m%s\033[0m"
	resetColor   = "\033[0m" // Reset color
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

 Multi-cloud infrastructure management
 _____________________________________`
)

// Request log skip patterns - used across multiple middlewares
var (
	RequestLogSkipPatterns = [][]string{
		{"/tumblebug/api"},
		{"/tumblebug/readyz"},
		{"/tumblebug/httpVersion"},
		{"/tumblebug/testStreamResponse"},
		{"/tumblebug/request"},
		{"/tumblebug/requests"},
	}

	APILogSkipPatterns = [][]string{
		{"/tumblebug/api"},
		{"/tumblebug/readyz"},
		{"/tumblebug/httpVersion"},
		{"/mci", "option=status"},
		{"/k8sCluster"},
		{"/resources/vNet"},
		{"/resources/securityGroup"},
		{"/resources/vpn"},
		{"/resources/sshKey"},
	}
)

// Helper function to check if request should skip logging/tracking
func shouldSkipRequestLog(c echo.Context) bool {
	path := c.Request().URL.Path
	method := c.Request().Method
	queryParams := c.QueryParams()

	// Skip OPTIONS method requests
	if method == "OPTIONS" {
		return true
	}

	for _, pattern := range RequestLogSkipPatterns {
		if len(pattern) == 1 {
			// Path-only pattern
			if strings.Contains(path, pattern[0]) {
				return true
			}
		} else if len(pattern) == 2 {
			// Path + query parameter pattern
			if strings.Contains(path, pattern[0]) {
				for key, values := range queryParams {
					for _, value := range values {
						if key+"="+value == pattern[1] {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// RunServer func start Rest API server
func RunServer() {

	log.Info().Msg("REST API Server is starting")

	e := echo.New()

	// Middleware

	e.Use(middlewares.Zerologger(APILogSkipPatterns))

	e.Use(middleware.Recover())
	// limit the application to 20 requests/sec using the default in-memory store
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

	// Custom middleware for RequestID and RequestDetails
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if shouldSkipRequestLog(c) {
				return next(c)
			}
			return middlewares.RequestIdAndDetailsIssuer(next)(c)
		}
	})

	// Custom middleware for tracing
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if shouldSkipRequestLog(c) {
				return next(c)
			}
			return middlewares.TracingMiddleware(next)(c)
		}
	})

	// Custom middleware for ResponseBodyDump
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if shouldSkipRequestLog(c) {
				return next(c)
			}
			return middlewares.ResponseBodyDump()(next)(c)
		}
	})

	e.HideBanner = true
	//e.colorer.Printf(banner, e.colorer.Red("v"+Version), e.colorer.Blue(website))

	// Route for system management
	docs.SwaggerInfo.Host = model.SelfEndpoint
	swaggerRedirect := func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/tumblebug/api/index.html")
	}
	e.GET("/tumblebug/api", swaggerRedirect)
	e.GET("/tumblebug/api/", swaggerRedirect)
	e.GET("/tumblebug/api/*", echoSwagger.WrapHandler)

	e.GET("/tumblebug/readyz", rest_common.RestGetReadyz)
	e.GET("/tumblebug/httpVersion", rest_common.RestCheckHTTPVersion)
	e.POST("/tumblebug/testStreamResponse", rest_common.RestTestStreamResponse)

	allowedOrigins := os.Getenv("TB_ALLOW_ORIGINS")
	if allowedOrigins == "" {
		log.Fatal().Msg("TB_ALLOW_ORIGINS env variable for CORS is " + allowedOrigins +
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

	trApiUser := os.Getenv("TB_TERRARIUM_API_USERNAME")
	trApiPass := os.Getenv("TB_TERRARIUM_API_PASSWORD")

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
					if subtle.ConstantTimeCompare([]byte(username), []byte(apiUser)) == 1 {
						// bcrypt verification
						// log.Debug().Msgf("bcrypt.CompareHashAndPassword(%s, %s)", apiPass, password)
						err := bcrypt.CompareHashAndPassword([]byte(apiPass), []byte(password))
						if err == nil {
							return true, nil
						}
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
	e.GET("/tumblebug/provider/:providerName/region", rest_common.RestGetRegions)
	e.GET("/tumblebug/provider/:providerName/region/:regionName", rest_common.RestGetRegion)
	e.GET("/tumblebug/regionFromCsp", rest_common.RestGetRegionListFromCsp)
	e.GET("/tumblebug/k8sClusterInfo", rest_common.RestGetK8sClusterInfo)

	e.GET("/tumblebug/credential/publicKey", rest_common.RestGetPublicKeyForCredentialEncryption)
	e.POST("/tumblebug/credential", rest_common.RestRegisterCredential)

	e.POST("/tumblebug/lookupSpecs", rest_resource.RestLookupSpecList)
	e.POST("/tumblebug/lookupSpec", rest_resource.RestLookupSpec)

	e.POST("/tumblebug/lookupImages", rest_resource.RestLookupImageList)
	e.POST("/tumblebug/lookupImage", rest_resource.RestLookupImage)

	e.POST("/tumblebug/inspectResources", rest_common.RestInspectResources)
	e.GET("/tumblebug/inspectResourcesOverview", rest_common.RestInspectResourcesOverview)

	e.POST("/tumblebug/registerCspResources", rest_common.RestRegisterCspNativeResources)
	e.POST("/tumblebug/registerCspResourcesAll", rest_common.RestRegisterCspNativeResourcesAll)

	// @Tags [Admin] System Configuration
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

	e.GET("/tumblebug/loadAssets", rest_resource.RestLoadAssets)
	e.POST("/tumblebug/ns/:nsId/sharedResource", rest_resource.RestCreateSharedResource)
	e.DELETE("/tumblebug/ns/:nsId/sharedResources", rest_resource.RestDelAllSharedResources)

	e.POST("/tumblebug/forward/*", rest_common.RestForwardAnyReqToAny)

	// Utility for network design
	e.POST("/tumblebug/util/net/design", rest_netutil.RestPostUtilToDesignNetwork)
	e.POST("/tumblebug/util/net/validate", rest_netutil.RestPostUtilToValidateNetwork)
	e.POST("/tumblebug/util/vNet/design", rest_netutil.RestPostUtilToDesignVNet)

	// Route for NameSpace subgroup
	g := e.Group("/tumblebug/ns", common.NsValidation())

	//Namespace Management
	g.POST("", rest_common.RestPostNs)
	g.GET("/:nsId", rest_common.RestGetNs)
	g.GET("", rest_common.RestGetAllNs)
	g.PUT("/:nsId", rest_common.RestPutNs)
	g.DELETE("/:nsId", rest_common.RestDelNs)
	g.DELETE("", rest_common.RestDelAllNs)

	// Resource Label
	e.PUT("/tumblebug/label/:labelType/:uid", rest_label.RestCreateOrUpdateLabel)
	e.PUT("/tumblebug/mergeCSPLabel/:labelType/:uid", rest_label.RestMergeCSPResourceLabel)
	e.DELETE("/tumblebug/label/:labelType/:uid/:key", rest_label.RestRemoveLabel)
	e.GET("/tumblebug/label/:labelType/:uid", rest_label.RestGetLabels)
	e.GET("/tumblebug/resources/:labelType", rest_label.RestGetResourcesByLabelSelector)
	e.GET("/tumblebug/labelInfo", rest_label.RestGetSystemLabelInfo)

	//MCI Management
	g.POST("/:nsId/mci", rest_infra.RestPostMci)
	g.POST("/:nsId/registerCspVm", rest_infra.RestPostRegisterCSPNativeVM)

	e.POST("/tumblebug/mciRecommendVm", rest_infra.RestRecommendVm)
	e.POST("/tumblebug/mciDynamicCheckRequest", rest_infra.RestPostMciDynamicCheckRequest)
	e.POST("/tumblebug/systemMci", rest_infra.RestPostSystemMci)

	g.POST("/:nsId/mciDynamic", rest_infra.RestPostMciDynamic)
	g.POST("/:nsId/mciDynamicReview", rest_infra.RestPostMciDynamicReview)
	g.POST("/:nsId/mci/:mciId/vmDynamic", rest_infra.RestPostMciVmDynamic)

	g.GET("/:nsId/mci/:mciId/associatedResources", rest_infra.RestGetMciAssociatedResources)
	g.PUT("/:nsId/mci/:mciId/associatedSecurityGroups", rest_infra.RestPutMciAssociatedSecurityGroups)

	//g.GET("/:nsId/mci/:mciId", rest_infra.RestGetMci, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	//g.GET("/:nsId/mci", rest_infra.RestGetAllMci, middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 20 * time.Second}), middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(1)))
	// path specific timeout and ratelimit
	// timeout middleware
	timeoutConfig := middleware.TimeoutConfig{
		Timeout:      60 * time.Second,
		Skipper:      middleware.DefaultSkipper,
		ErrorMessage: "Error: request time out (60s)",
	}

	g.GET("/:nsId/mci/:mciId", rest_infra.RestGetMci, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/mci", rest_infra.RestGetAllMci, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))

	// g.PUT("/:nsId/mci/:mciId", rest_infra.RestPutMci)
	g.DELETE("/:nsId/mci/:mciId", rest_infra.RestDelMci)
	g.DELETE("/:nsId/mci", rest_infra.RestDelAllMci)

	g.POST("/:nsId/mci/:mciId/vm", rest_infra.RestPostMciVm)
	g.GET("/:nsId/mci/:mciId/vm/:vmId", rest_infra.RestGetMciVm)
	g.GET("/:nsId/mci/:mciId/subgroup", rest_infra.RestGetMciGroupIds)
	g.GET("/:nsId/mci/:mciId/subgroup/:subgroupId", rest_infra.RestGetMciGroupVms)
	g.POST("/:nsId/mci/:mciId/subgroup/:subgroupId", rest_infra.RestPostMciSubGroupScaleOut)

	//g.GET("/:nsId/mci/:mciId/vm", rest_infra.RestGetAllMciVm)
	// g.PUT("/:nsId/mci/:mciId/vm/:vmId", rest_infra.RestPutMciVm)
	g.DELETE("/:nsId/mci/:mciId/vm/:vmId", rest_infra.RestDelMciVm)
	//g.DELETE("/:nsId/mci/:mciId/vm", rest_infra.RestDelAllMciVm)

	//g.POST("/:nsId/mci/recommend", rest_infra.RestPostMciRecommend)

	g.GET("/:nsId/control/mci/:mciId", rest_infra.RestGetControlMci)
	g.GET("/:nsId/control/mci/:mciId/vm/:vmId", rest_infra.RestGetControlMciVm)

	g.POST("/:nsId/cmd/mci/:mciId", rest_infra.RestPostCmdMci)
	g.POST("/:nsId/transferFile/mci/:mciId", rest_infra.RestPostFileToMci)
	g.PUT("/:nsId/mci/:mciId/vm/:targetVmId/bastion/:bastionVmId", rest_infra.RestSetBastionNodes)
	g.DELETE("/:nsId/mci/:mciId/bastion/:bastionVmId", rest_infra.RestRemoveBastionNodes)
	g.GET("/:nsId/mci/:mciId/vm/:targetVmId/bastion", rest_infra.RestGetBastionNodes)

	g.POST("/:nsId/installBenchmarkAgent/mci/:mciId", rest_infra.RestPostInstallBenchmarkAgentToMci)
	g.POST("/:nsId/benchmark/mci/:mciId", rest_infra.RestGetBenchmark)
	g.POST("/:nsId/benchmarkAll/mci/:mciId", rest_infra.RestGetAllBenchmark)
	g.GET("/:nsId/benchmarkLatency/mci/:mciId", rest_infra.RestGetBenchmarkLatency)

	// VPN Sites info
	g.GET("/:nsId/mci/:mciId/site", rest_resource.RestGetSitesInMci)

	// Site-to-stie VPN management
	// Route for stream response subgroup
	// streamResponseGroup := e.Group("/tumblebug/stream-response/ns", common.NsValidation())
	g.GET("/:nsId/mci/:mciId/vpn", rest_resource.RestGetAllSiteToSiteVpn)
	vpnGroup := g.Group("/:nsId/mci/:mciId/vpn")
	terrariumURL := model.TerrariumRestUrl + "/readyz"
	vpnGroup.Use(middlewares.CheckReadiness(terrariumURL, trApiUser, trApiPass))
	vpnGroup.POST("", rest_resource.RestPostSiteToSiteVpn)
	vpnGroup.GET("/:vpnId", rest_resource.RestGetSiteToSiteVpn)
	// g.PUT("/:vpnId", rest_resource.RestPutSiteToSiteVpn)
	vpnGroup.DELETE("/:vpnId", rest_resource.RestDeleteSiteToSiteVpn)
	vpnGroup.GET("/:vpnId/request/:requestId", rest_resource.RestGetRequestStatusOfSiteToSiteVpn)
	// TBD
	// g.POST("/:nsId/mci/:mciId/vpn/:vpnId", rest_infra.RestPostVpnGcpToAws)
	// g.PUT("/:nsId/mci/:mciId/vpn/:vpnId", rest_infra.RestPutVpnGcpToAws)
	// g.DELETE("/:nsId/mci/:mciId/vpn/:vpnId", rest_infra.RestDeleteVpnGcpToAws)

	//MCI AUTO Policy
	g.POST("/:nsId/policy/mci/:mciId", rest_infra.RestPostMciPolicy)
	g.GET("/:nsId/policy/mci/:mciId", rest_infra.RestGetMciPolicy)
	g.GET("/:nsId/policy/mci", rest_infra.RestGetAllMciPolicy)
	g.PUT("/:nsId/policy/mci/:mciId", rest_infra.RestPutMciPolicy)
	g.DELETE("/:nsId/policy/mci/:mciId", rest_infra.RestDelMciPolicy)
	g.DELETE("/:nsId/policy/mci", rest_infra.RestDelAllMciPolicy)

	g.POST("/:nsId/monitoring/install/mci/:mciId", rest_infra.RestPostInstallMonitorAgentToMci)
	g.GET("/:nsId/monitoring/mci/:mciId/metric/:metric", rest_infra.RestGetMonitorData)
	g.PUT("/:nsId/monitoring/status/mci/:mciId/vm/:vmId", rest_infra.RestPutMonitorAgentStatusInstalled)

	// K8sCluster
	e.GET("/tumblebug/availableK8sVersion", rest_resource.RestGetAvailableK8sVersion)
	e.GET("/tumblebug/availableK8sNodeImage", rest_resource.RestGetAvailableK8sNodeImage)
	e.GET("/tumblebug/checkK8sNodeGroupsOnK8sCreation", rest_resource.RestCheckK8sNodeGroupsOnK8sCreation)
	e.GET("/tumblebug/checkK8sNodeImageDesignation", rest_resource.RestCheckK8sNodeImageDesignation)
	e.GET("/tumblebug/requiredK8sSubnetCount", rest_resource.RestGetRequiredK8sSubnetCount)
	g.POST("/:nsId/k8sCluster", rest_resource.RestPostK8sCluster)
	g.POST("/:nsId/k8sCluster/:k8sClusterId/k8sNodeGroup", rest_resource.RestPostK8sNodeGroup)
	g.DELETE("/:nsId/k8sCluster/:k8sClusterId/k8sNodeGroup/:k8sNodeGroupName", rest_resource.RestDeleteK8sNodeGroup)
	g.PUT("/:nsId/k8sCluster/:k8sClusterId/k8sNodeGroup/:k8sNodeGroupName/onAutoscaling", rest_resource.RestPutSetK8sNodeGroupAutoscaling)
	g.PUT("/:nsId/k8sCluster/:k8sClusterId/k8sNodeGroup/:k8sNodeGroupName/autoscaleSize", rest_resource.RestPutChangeK8sNodeGroupAutoscaleSize)
	g.GET("/:nsId/k8sCluster/:k8sClusterId", rest_resource.RestGetK8sCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.GET("/:nsId/k8sCluster", rest_resource.RestGetAllK8sCluster, middleware.TimeoutWithConfig(timeoutConfig),
		middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(2)))
	g.DELETE("/:nsId/k8sCluster/:k8sClusterId", rest_resource.RestDeleteK8sCluster)
	g.DELETE("/:nsId/k8sCluster", rest_resource.RestDeleteAllK8sCluster)
	g.PUT("/:nsId/k8sCluster/:k8sClusterId/upgrade", rest_resource.RestPutUpgradeK8sCluster)

	e.POST("/tumblebug/k8sClusterRecommendNode", rest_resource.RestRecommendK8sNode)
	e.POST("/tumblebug/k8sClusterDynamicCheckRequest", rest_resource.RestPostK8sClusterDynamicCheckRequest)
	g.POST("/:nsId/k8sClusterDynamic", rest_resource.RestPostK8sClusterDynamic)
	g.POST("/:nsId/k8sCluster/:k8sClusterId/k8sNodeGroupDynamic", rest_resource.RestPostK8sNodeGroupDynamic)
	g.GET("/:nsId/control/k8sCluster/:k8sClusterId", rest_resource.RestGetControlK8sCluster)

	g.POST("/:nsId/cmd/k8sCluster/:k8sClusterId", rest_resource.RestPostCmdK8sCluster)
	g.POST("/:nsId/transferFile/k8sCluster/:k8sClusterId", rest_resource.RestPostFileToK8sCluster)

	// Network Load Balancer
	g.POST("/:nsId/mci/:mciId/mcSwNlb", rest_infra.RestPostMcNLB)
	g.POST("/:nsId/mci/:mciId/nlb", rest_infra.RestPostNLB)
	g.GET("/:nsId/mci/:mciId/nlb/:resourceId", rest_infra.RestGetNLB)
	g.GET("/:nsId/mci/:mciId/nlb", rest_infra.RestGetAllNLB)
	// g.PUT("/:nsId/mci/:mciId/nlb/:resourceId", rest_infra.RestPutNLB)
	g.DELETE("/:nsId/mci/:mciId/nlb/:resourceId", rest_infra.RestDelNLB)
	g.DELETE("/:nsId/mci/:mciId/nlb", rest_infra.RestDelAllNLB)
	g.GET("/:nsId/mci/:mciId/nlb/:resourceId/healthz", rest_infra.RestGetNLBHealth)

	// VM snapshot -> creates one customImage and 'n' dataDisks
	g.POST("/:nsId/mci/:mciId/vm/:vmId/snapshot", rest_infra.RestPostMciVmSnapshot)

	// These REST APIs are for dev/test only
	g.POST("/:nsId/mci/:mciId/nlb/:resourceId/vm", rest_infra.RestAddNLBVMs)
	g.DELETE("/:nsId/mci/:mciId/nlb/:resourceId/vm", rest_infra.RestRemoveNLBVMs)

	// Resource Management
	g.POST("/:nsId/resources/dataDisk", rest_resource.RestPostDataDisk)
	g.GET("/:nsId/resources/dataDisk/:resourceId", rest_resource.RestGetResource)
	g.GET("/:nsId/resources/dataDisk", rest_resource.RestGetAllResources)
	g.PUT("/:nsId/resources/dataDisk/:resourceId", rest_resource.RestPutDataDisk)
	g.DELETE("/:nsId/resources/dataDisk/:resourceId", rest_resource.RestDelResource)
	g.DELETE("/:nsId/resources/dataDisk", rest_resource.RestDelAllResources)
	g.GET("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_resource.RestGetVmDataDisk)
	g.POST("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_resource.RestPostVmDataDisk)
	g.PUT("/:nsId/mci/:mciId/vm/:vmId/dataDisk", rest_resource.RestPutVmDataDisk)

	g.POST("/:nsId/resources/image", rest_resource.RestPostImage)
	g.GET("/:nsId/resources/image/:imageId", rest_resource.RestGetImage)
	g.GET("/:nsId/resources/image", rest_resource.RestGetAllResources)
	g.PUT("/:nsId/resources/image/:imageId", rest_resource.RestPutImage)
	g.DELETE("/:nsId/resources/image/:resourceId", rest_resource.RestDelResource)
	g.DELETE("/:nsId/resources/image", rest_resource.RestDelAllResources)

	g.POST("/:nsId/resources/customImage", rest_resource.RestPostCustomImage)
	g.GET("/:nsId/resources/customImage/:resourceId", rest_resource.RestGetResource)
	g.GET("/:nsId/resources/customImage", rest_resource.RestGetAllResources)
	// g.PUT("/:nsId/resources/customImage/:resourceId", rest_resource.RestPutCustomImage)
	g.DELETE("/:nsId/resources/customImage/:resourceId", rest_resource.RestDelResource)
	g.DELETE("/:nsId/resources/customImage", rest_resource.RestDelAllResources)

	g.POST("/:nsId/resources/sshKey", rest_resource.RestPostSshKey)
	g.GET("/:nsId/resources/sshKey/:resourceId", rest_resource.RestGetResource)
	g.GET("/:nsId/resources/sshKey", rest_resource.RestGetAllResources)
	g.PUT("/:nsId/resources/sshKey/:resourceId", rest_resource.RestPutSshKey)
	g.DELETE("/:nsId/resources/sshKey/:resourceId", rest_resource.RestDelResource)
	g.DELETE("/:nsId/resources/sshKey", rest_resource.RestDelAllResources)

	g.POST("/:nsId/resources/spec", rest_resource.RestPostSpec)
	g.GET("/:nsId/resources/spec/:resourceId", rest_resource.RestGetSpec)
	g.PUT("/:nsId/resources/spec/:resourceId", rest_resource.RestPutSpec)
	g.DELETE("/:nsId/resources/spec/:resourceId", rest_resource.RestDelResource)

	e.POST("/tumblebug/fetchSpecs", rest_resource.RestFetchSpecs)
	e.POST("/tumblebug/fetchPrice", rest_resource.RestFetchPrice)
	g.POST("/:nsId/resources/filterSpecsByRange", rest_resource.RestFilterSpecsByRange)

	e.POST("/tumblebug/fetchImages", rest_resource.RestFetchImages)
	e.POST("/tumblebug/fetchImagesAsync", rest_resource.RestFetchImagesAsync)
	e.POST("/tumblebug/fetchImagesResult", rest_resource.RestGetFetchImagesAsyncResult)
	e.POST("/tumblebug/updateImagesFromAsset", rest_resource.RestUpdateImagesFromAsset)
	g.POST("/:nsId/resources/searchImage", rest_resource.RestSearchImage)
	g.GET("/:nsId/resources/searchImageOptions", rest_resource.RestSearchImageOptions)

	g.POST("/:nsId/resources/securityGroup", rest_resource.RestPostSecurityGroup)
	g.GET("/:nsId/resources/securityGroup/:resourceId", rest_resource.RestGetResource)
	g.GET("/:nsId/resources/securityGroup", rest_resource.RestGetAllResources)
	g.PUT("/:nsId/resources/securityGroup/:resourceId", rest_resource.RestPutSecurityGroup)
	g.DELETE("/:nsId/resources/securityGroup/:resourceId", rest_resource.RestDelResource)
	g.DELETE("/:nsId/resources/securityGroup", rest_resource.RestDelAllResources)

	g.POST("/:nsId/resources/securityGroup/:securityGroupId/rules", rest_resource.RestPostFirewallRules)
	g.DELETE("/:nsId/resources/securityGroup/:securityGroupId/rules", rest_resource.RestDelFirewallRules)

	// Network management: vNet
	g.POST("/:nsId/resources/vNet", rest_resource.RestPostVNet)
	g.GET("/:nsId/resources/vNet/:vNetId", rest_resource.RestGetVNet)
	g.GET("/:nsId/resources/vNet", rest_resource.RestGetAllResources)
	// g.PUT("/:nsId/resources/vNet/:resourceId", rest_resource.RestPutVNet)
	g.DELETE("/:nsId/resources/vNet/:vNetId", rest_resource.RestDelVNet)
	g.DELETE("/:nsId/resources/vNet", rest_resource.RestDelAllResources)

	// Network management: subnet associated with vNet
	g.POST("/:nsId/resources/vNet/:vNetId/subnet", rest_resource.RestPostSubnet)
	g.GET("/:nsId/resources/vNet/:vNetId/subnet/:subnetId", rest_resource.RestGetSubnet)
	g.GET("/:nsId/resources/vNet/:vNetId/subnet", rest_resource.RestGetListSubnet)
	// g.PUT("/:nsId/resources/vNet/:vNetId/subnet/:subnetId", rest_resource.RestPutSubnet)
	g.DELETE("/:nsId/resources/vNet/:vNetId/subnet/:subnetId", rest_resource.RestDelSubnet)
	// g.DELETE("/:nsId/resources/vNet/:vNetId/subnet", rest_resource.RestDelAllSubnet)

	// Network management: register vNet and/or subnets, which was created in CSP
	g.POST("/:nsId/registerCspResource/vNet", rest_resource.RestPostRegisterVNet)
	g.DELETE("/:nsId/deregisterCspResource/vNet/:vNetId", rest_resource.RestDeleteDeregisterVNet)
	g.POST("/:nsId/registerCspResource/vNet/:vNetId/subnet", rest_resource.RestPostRegisterSubnet)
	g.DELETE("/:nsId/deregisterCspResource/vNet/:vNetId/subnet/:subnetId", rest_resource.RestDeleteDeregisterSubnet)

	// SQL database management
	// g.GET("/:nsId/resources/sqlDb", rest_resource.)
	sqlDbGroup := g.Group("/:nsId/resources/sqlDb")
	terrariumURL = model.TerrariumRestUrl + "/readyz"
	sqlDbGroup.Use(middlewares.CheckReadiness(terrariumURL, trApiUser, trApiPass))
	sqlDbGroup.POST("", rest_resource.RestPostSqlDB)
	sqlDbGroup.GET("/:sqlDbId", rest_resource.RestGetSqlDB)
	sqlDbGroup.DELETE("/:sqlDbId", rest_resource.RestDeleteSqlDB)
	// sqlDbGroup.GET("/:sqlDbId/request/:requestId", rest_resource.RestGetRequestStatusOfSqlDB)
	// sqlDbGroup.PUT("//:sqlDbId", rest_resource.RestPutSqlDB)

	// Object Storage management
	// g.GET("/:nsId/resources/objectStorage", rest_resource.)
	objectStorageGroup := g.Group("/:nsId/resources/objectStorage")
	terrariumURL = model.TerrariumRestUrl + "/readyz"
	objectStorageGroup.Use(middlewares.CheckReadiness(terrariumURL, trApiUser, trApiPass))
	objectStorageGroup.POST("", rest_resource.RestPostObjectStorage)
	objectStorageGroup.GET("/:objectStorageId", rest_resource.RestGetObjectStorage)
	objectStorageGroup.DELETE("/:objectStorageId", rest_resource.RestDeleteObjectStorage)
	// objectStorageGroup.GET("/:objectStorageId/request/:requestId", rest_resource.RestGetRequestStatusOfObjectStorage)
	// objectStorageGroup.PUT("//:objectStorageId", rest_resource.RestPutObjectStorage)

	/*
		g.POST("/:nsId/resources/publicIp", resource.RestPostPublicIp)
		g.GET("/:nsId/resources/publicIp/:publicIpId", resource.RestGetPublicIp)
		g.GET("/:nsId/resources/publicIp", resource.RestGetAllPublicIp)
		g.PUT("/:nsId/resources/publicIp/:publicIpId", resource.RestPutPublicIp)
		g.DELETE("/:nsId/resources/publicIp/:publicIpId", resource.RestDelPublicIp)
		g.DELETE("/:nsId/resources/publicIp", resource.RestDelAllPublicIp)

		g.POST("/:nsId/resources/vNic", resource.RestPostVNic)
		g.GET("/:nsId/resources/vNic/:vNicId", resource.RestGetVNic)
		g.GET("/:nsId/resources/vNic", resource.RestGetAllVNic)
		g.PUT("/:nsId/resources/vNic/:vNicId", resource.RestPutVNic)
		g.DELETE("/:nsId/resources/vNic/:vNicId", resource.RestDelVNic)
		g.DELETE("/:nsId/resources/vNic", resource.RestDelAllVNic)
	*/

	// We cannot use these wildcard method below.
	// https://github.com/labstack/echo/issues/382
	//g.DELETE("/:nsId/resources/:resourceType/:resourceId", resource.RestDelResource)
	//g.DELETE("/:nsId/resources/:resourceType", resource.RestDelAllResources)

	g.GET("/:nsId/checkResource/:resourceType/:resourceId", rest_resource.RestCheckResource)
	g.GET("/:nsId/checkMci/:mciId", rest_infra.RestCheckMci)
	g.GET("/:nsId/mci/:mciId/checkVm/:vmId", rest_infra.RestCheckVm)

	// g.POST("/:nsId/registerExistingResources", rest_resource.RestRegisterExistingResources)

	// Temporal test API for development of UpdateAssociatedObjectList
	g.PUT("/:nsId/testAddObjectAssociation/:resourceType/:resourceId", rest_resource.RestTestAddObjectAssociation)
	g.PUT("/:nsId/testDeleteObjectAssociation/:resourceType/:resourceId", rest_resource.RestTestDeleteObjectAssociation)
	g.GET("/:nsId/testGetAssociatedObjectCount/:resourceType/:resourceId", rest_resource.RestTestGetAssociatedObjectCount)

	selfEndpoint := strings.Split(model.SelfEndpoint, ":")
	selfIp := selfEndpoint[0]
	selfPort := selfEndpoint[1]

	apiServer := fmt.Sprintf("http://%s:%s/tumblebug/readyz", selfIp, selfPort)
	//apiDashboard := fmt.Sprintf("http://%s:%s", selfIp, "1325")
	apiDashboard := fmt.Sprintf("http://%s:%s/tumblebug/api", selfIp, selfPort)
	mapUI := fmt.Sprintf("http://%s:%s", selfIp, "1324")

	fmt.Print(resetColor)
	fmt.Printf(" Default Namespace: %s%s%s\n", warningColor, model.DefaultNamespace, resetColor)
	fmt.Printf(" Default CredentialHolder: %s%s%s\n\n", warningColor, model.DefaultCredentialHolder, resetColor)
	fmt.Printf(" API Endpoint: %s%s%s\n", warningColor, apiServer, resetColor)
	fmt.Println(" ")
	fmt.Printf(" MapUI (Simple GUI): %s%s%s\n", warningColor, mapUI, resetColor)
	fmt.Printf(" SwaggerUI (API Dashboard): %s%s%s\n", warningColor, apiDashboard, resetColor)
	// Check if auth is enabled and print message accordingly
	if authEnabled {
		fmt.Printf(" %s(Check ENV to get the API username and password)%s\n", titleColor, resetColor)
	}

	fmt.Println("\n ")

	// A context for graceful shutdown (It is based on the signal package)
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

	model.SystemReady = true
	if err := e.Start(":" + selfPort); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Error in Starting CB-Tumblebug API Server")
		e.Logger.Panic("Shuttig down the server: ", err)
	}

	wg.Wait()
}
