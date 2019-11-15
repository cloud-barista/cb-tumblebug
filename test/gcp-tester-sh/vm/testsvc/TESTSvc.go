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
	"fmt"
	"runtime"
	"strconv"
	"github.com/shirou/gopsutil/cpu"
	"github.com/labstack/echo"
	"net/http"
)

func dealwithErr(err error) {
	if err != nil {
			fmt.Println(err)
			//os.Exit(-1)
	}
}

// the number of total cores(logical CPUs)
func GetLogicalCPUNumber() int {
return runtime.NumCPU()
}

func GetCPUModelName() string {
cpuStat, err := cpu.Info()
dealwithErr(err)

return cpuStat[0].ModelName;
}

// percentages of each logical CPUs
func GetAllUtilPercentages() []string {
percentage, err := cpu.Percent(0, true)
dealwithErr(err)

strPercentageArr := make([]string,0)
cpuNum := runtime.NumCPU()
for idx := 0; idx < cpuNum; idx++ {
   //strPercentageArr[idx] = strconv.FormatFloat(cpupercent, 'f', 2, 64)
   strPercentageArr = append(strPercentageArr, strconv.FormatFloat(percentage[idx], 'f', 2, 64))
}

return strPercentageArr
}

var cnt uint64 = 0

type TESTSvcReqInfo struct {
        Date        	string  // ex) "Fri Nov  1 20:15:54 KST 2019"
		HostName        string  // ex) "localhost"
		IP				string
		Country			string
}

//================ Call Service for test
func callService(c echo.Context) error {

	req := &TESTSvcReqInfo{}
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	cnt++
	date := strings.ReplaceAll(req.Date, "%20", " ")

	strCPUUtilizationArr := GetAllUtilPercentages()
	for i, cpupercent := range strCPUUtilizationArr {
		println("CPU" + strconv.Itoa(i) +":   " + cpupercent + " %")
	}

	//cblog.Infof("[%#v][Request From] DATE: %#v, HOSTNAME: %#v", cnt, date, req.HostName)
	fmt.Printf("[%#v][Request From] DATE: %#v, HOSTNAME: %#v \n", cnt, date, req.HostName)
	fmt.Printf("[%#v] IP: %#v, Location: %#v \n", cnt, req.IP, req.Country)
	fmt.Printf("Processing ............ \n")
	for i := 1; i <= 65535; i++ {
			for j := 1; j <= 65535; j++ {
					_ = 159 * 753 * 654
			}
	}
	fmt.Printf("Finished the Processing for HOST: %#v \n", req.HostName)

	resultInfo := BooleanInfo{
			Result: "OK",
	}

	return c.JSON(http.StatusOK, &resultInfo)
}

