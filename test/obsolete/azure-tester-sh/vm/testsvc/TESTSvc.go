// Rest Runtime Server for VM's SSH and SCP of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by powerkim@etri.re.kr, 2019.10.

package main

import (
	"strings"

	"github.com/labstack/echo"
	"net/http"
)


type TESTSvcReqInfo struct {
        Date        	string  // ex) "Fri Nov  1 20:15:54 KST 2019"
        HostName        string  // ex) "localhost"
}

//================ Call Service for test
func callService(c echo.Context) error {

	req := &TESTSvcReqInfo{}
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	date := strings.ReplaceAll(req.Date, "%20", " ")	
	cblog.Infof("DATE: %#v, HOSTNAME: %#v", date, req.HostName)

        resultInfo := BooleanInfo{
                Result: "OK",
        }

	return c.JSON(http.StatusOK, &resultInfo)
}

