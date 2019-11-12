// Rest Runtime Server of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by powerkim@etri.re.kr, 2019.10.

package main

import (
	"fmt"

	"github.com/cloud-barista/cb-store/config"
	"github.com/sirupsen/logrus"

	// REST API (echo)
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var cblog *logrus.Logger

func init() {
	cblog = config.Cblogger
}

// REST API Return struct for boolena type
type BooleanInfo struct {
        Result string // true or false
}


type route struct {
	method, path string
	function     echo.HandlerFunc
}

func main() {

	//======================================= setup routes
	routes := []route{
		{"GET", "/test", callService},
	}
	//======================================= setup routes

	fmt.Println("\n[CB-Spider:Test Service]")
	fmt.Println("\n   Initiating REST API Server....__^..^__....\n\n")

	// Run API Server
	ApiServer(routes, ":119")
}

//================ REST API Server: setup & start
func ApiServer(routes []route, strPort string) {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	for _, route := range routes {
		switch route.method {
		case "POST":
			e.POST(route.path, route.function)
		case "GET":
			e.GET(route.path, route.function)
		case "PUT":
			e.PUT(route.path, route.function)
		case "DELETE":
			e.DELETE(route.path, route.function)

		}
	}

	e.HideBanner = true
	if strPort == "" {
		strPort = ":1323"
	}
	e.Logger.Fatal(e.Start(strPort))
}
