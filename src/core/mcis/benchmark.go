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

// Package mcis is to manage multi-cloud infra service
package mcis

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	//"log"
	"strconv"
	"strings"

	//csv file handling

	"encoding/csv"
	"os"

	// REST API (echo)
	"net/http"

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// SpecBenchmarkInfo is struct for SpecBenchmarkInfo
type SpecBenchmarkInfo struct {
	SpecId     string `json:"specid"`
	Cpus       string `json:"cpus"`
	Cpum       string `json:"cpum"`
	MemR       string `json:"memR"`
	MemW       string `json:"memW"`
	FioR       string `json:"fioR"`
	FioW       string `json:"fioW"`
	DbR        string `json:"dbR"`
	DbW        string `json:"dbW"`
	Rtt        string `json:"rtt"`
	EvaledTime string `json:"evaledTime"`
}

// BenchmarkInfo is struct for BenchmarkInfo
type BenchmarkInfo struct {
	Result      string          `json:"result"`
	Unit        string          `json:"unit"`
	Desc        string          `json:"desc"`
	Elapsed     string          `json:"elapsed"`
	SpecId      string          `json:"specid"`
	ResultArray []BenchmarkInfo `json:"resultarray"` // struct-element cycle ?
}

// BenchmarkInfoArray is struct for BenchmarkInfoArray
type BenchmarkInfoArray struct {
	ResultArray []BenchmarkInfo `json:"resultarray"`
}

// BenchmarkReq is struct for BenchmarkReq
type BenchmarkReq struct {
	Host string `json:"host"`
	Spec string `json:"spec"`
}

// MultihostBenchmarkReq is struct for MultihostBenchmarkReq
type MultihostBenchmarkReq struct {
	Multihost []BenchmarkReq `json:"multihost"`
}

const milkywayPort string = ":1324/milkyway/"

// AgentInstallContentWrapper ...
type AgentInstallContentWrapper struct {
	ResultArray []AgentInstallContent `json:"resultArray"`
}

// AgentInstallContent ...
type AgentInstallContent struct {
	McisId string `json:"mcisId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
}

// InstallAgentToMcis is func to install milkyway agents in MCIS
func InstallAgentToMcis(nsId string, mcisId string, req *McisCmdReq) (AgentInstallContentWrapper, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := AgentInstallContentWrapper{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := AgentInstallContentWrapper{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	content := AgentInstallContentWrapper{}

	//install script
	cmd := "wget https://github.com/cloud-barista/cb-milkyway/raw/master/src/milkyway -O ~/milkyway; chmod +x ~/milkyway; ~/milkyway > /dev/null 2>&1 & netstat -tulpn | grep milkyway"

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	var resultArray []SshCmdResult

	for _, v := range vmList {

		vmId := v
		vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

		//cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.UserName
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }

		// find vaild username
		userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, req.UserName)

		fmt.Println("")
		fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName:" + userName)
		fmt.Println("[CMD] " + cmd)
		fmt.Println("")

		// Avoid RunRemoteCommand to not ready VM
		if err != nil {
			wg.Add(1)
			go RunRemoteCommandAsync(&wg, vmId, vmIp, sshPort, userName, sshKey, cmd, &resultArray)
		} else {
			common.CBLog.Error(err)
			sshResultTmp := SshCmdResult{}
			sshResultTmp.McisId = mcisId
			sshResultTmp.VmId = vmId
			sshResultTmp.VmIp = vmIp
			sshResultTmp.Result = err.Error()
			sshResultTmp.Err = err
		}

	}
	wg.Wait() //goroutin sync wg

	for _, v := range resultArray {

		resultTmp := AgentInstallContent{}
		resultTmp.McisId = mcisId
		resultTmp.VmId = v.VmId
		resultTmp.VmIp = v.VmIp
		resultTmp.Result = v.Result
		content.ResultArray = append(content.ResultArray, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return content, nil

}

// CallMilkyway is func to call milkyway agents
func CallMilkyway(wg *sync.WaitGroup, vmList []string, nsId string, mcisId string, vmId string, vmIp string, action string, option string, results *BenchmarkInfoArray) {
	defer wg.Done() //goroutine sync done

	url := "http://" + vmIp + milkywayPort + action
	method := "GET"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Create Req body
	type JsonTemplate struct {
		Host string `json:"host"`
	}
	tempReq := JsonTemplate{}
	tempReq.Host = option
	payload, _ := json.MarshalIndent(tempReq, "", "  ")

	if action == "mrtt" {
		reqTmp := MultihostBenchmarkReq{}
		for _, vm := range vmList {
			vmIdTmp := vm
			vmIpTmp, _ := GetVmIp(nsId, mcisId, vmIdTmp)
			fmt.Println("[Test for vmList " + vmIdTmp + ", " + vmIpTmp + "]")

			hostTmp := BenchmarkReq{}
			hostTmp.Host = vmIpTmp
			hostTmp.Spec = GetVmSpecId(nsId, mcisId, vmIdTmp)
			reqTmp.Multihost = append(reqTmp.Multihost, hostTmp)
		}
		common.PrintJsonPretty(reqTmp)
		payload, _ = json.MarshalIndent(reqTmp, "", "  ")
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		fmt.Println(err)
	}
	errStr := ""
	res, err := client.Do(req)
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.CBLog.Error(err)
		errStr = err.Error()
	}
	fmt.Println(string(body))

	fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		errStr = err.Error()
	}

	//benchInfoTmp := BenchmarkInfo{}
	resultTmp := BenchmarkInfo{}
	err2 := json.Unmarshal(body, &resultTmp)
	if err2 != nil {
		common.CBLog.Error(err2)
		errStr = err2.Error()
	}
	//benchInfoTmp.ResultArray =  resultTmp.ResultArray
	if errStr != "" {
		resultTmp.Result = errStr
	}
	resultTmp.SpecId = GetVmSpecId(nsId, mcisId, vmId)
	results.ResultArray = append(results.ResultArray, resultTmp)
}

// CoreGetAllBenchmark is func to get alls Benchmarks
func CoreGetAllBenchmark(nsId string, mcisId string, host string) (*BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &BenchmarkInfoArray{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	target := host

	action := "all"
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	content := BenchmarkInfoArray{}

	allBenchCmd := []string{"cpus", "cpum", "memR", "memW", "fioR", "fioW", "dbR", "dbW", "rtt"}

	resultMap := make(map[string]SpecBenchmarkInfo)

	for i, v := range allBenchCmd {
		fmt.Println("[Benchmark] " + v)
		content, err = BenchmarkAction(nsId, mcisId, v, option)
		for _, k := range content.ResultArray {
			SpecId := k.SpecId
			Result := k.Result
			specBenchInfoTmp := SpecBenchmarkInfo{}

			val, exist := resultMap[SpecId]
			if exist {
				specBenchInfoTmp = val
			} else {
				specBenchInfoTmp.SpecId = SpecId
			}

			switch i {
			case 0:
				specBenchInfoTmp.Cpus = Result
			case 1:
				specBenchInfoTmp.Cpum = Result
			case 2:
				specBenchInfoTmp.MemR = Result
			case 3:
				specBenchInfoTmp.MemW = Result
			case 4:
				specBenchInfoTmp.FioR = Result
			case 5:
				specBenchInfoTmp.FioW = Result
			case 6:
				specBenchInfoTmp.DbR = Result
			case 7:
				specBenchInfoTmp.DbW = Result
			case 8:
				specBenchInfoTmp.Rtt = Result
			}

			resultMap[SpecId] = specBenchInfoTmp

		}
	}

	file, err := os.OpenFile("benchmarking.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	strsTmp := []string{}
	for key, val := range resultMap {
		strsTmp = nil
		fmt.Println(key, val)
		strsTmp = append(strsTmp, val.SpecId)
		strsTmp = append(strsTmp, val.Cpus)
		strsTmp = append(strsTmp, val.Cpum)
		strsTmp = append(strsTmp, val.MemR)
		strsTmp = append(strsTmp, val.MemW)
		strsTmp = append(strsTmp, val.FioR)
		strsTmp = append(strsTmp, val.FioW)
		strsTmp = append(strsTmp, val.DbR)
		strsTmp = append(strsTmp, val.DbW)
		strsTmp = append(strsTmp, val.Rtt)
		csvWriter.Write(strsTmp)
		csvWriter.Flush()
	}

	file2, err := os.OpenFile("rttmap.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file2.Close()
	csvWriter2 := csv.NewWriter(file2)

	const mrttArrayXMax = 50
	const mrttArrayYMax = 50
	mrttArray := make([][]string, mrttArrayXMax)
	for i := 0; i < mrttArrayXMax; i++ {
		mrttArray[i] = make([]string, mrttArrayYMax)
		for j := 0; j < mrttArrayYMax; j++ {
			mrttArray[i][j] = "0"
		}
	}

	rttIndexMapX := make(map[string]int)
	cntTargetX := 1
	rttIndexMapY := make(map[string]int)
	cntTargetY := 1

	action = "mrtt"
	fmt.Println("[Benchmark] " + action)
	content, err = BenchmarkAction(nsId, mcisId, action, option)
	for _, k := range content.ResultArray {
		SpecId := k.SpecId
		iX, exist := rttIndexMapX[SpecId]
		if !exist {
			rttIndexMapX[SpecId] = cntTargetX
			iX = cntTargetX
			mrttArray[iX][0] = SpecId
			cntTargetX++
		}
		for _, m := range k.ResultArray {
			tagetSpecId := m.SpecId
			tagetRtt := m.Result
			iY, exist2 := rttIndexMapY[tagetSpecId]
			if !exist2 {
				rttIndexMapY[tagetSpecId] = cntTargetY
				iY = cntTargetY
				mrttArray[0][iY] = tagetSpecId
				cntTargetY++
			}
			mrttArray[iX][iY] = tagetRtt
		}
	}

	csvWriter2.WriteAll(mrttArray)
	csvWriter2.Flush()

	if err != nil {
		//mapError := map[string]string{"message": "Benchmark Error"}
		//return c.JSON(http.StatusFailedDependency, &mapError)
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// CoreGetBenchmark is func to get Benchmark
func CoreGetBenchmark(nsId string, mcisId string, action string, host string) (*BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := BenchmarkInfoArray{}
		common.CBLog.Error(err)
		return &temp, err
	}
	check, _ := CheckMcis(nsId, mcisId)

	if !check {
		temp := &BenchmarkInfoArray{}
		err := fmt.Errorf("The mcis " + mcisId + " does not exist.")
		return temp, err
	}

	target := host

	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	content := BenchmarkInfoArray{}

	vaildActions := "install init cpus cpum memR memW fioR fioW dbR dbW rtt mrtt clean"

	fmt.Println("[Benchmark] " + action)
	if strings.Contains(vaildActions, action) {
		content, err = BenchmarkAction(nsId, mcisId, action, option)
	} else {
		//mapA := map[string]string{"message": "Not available action"}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return nil, fmt.Errorf("Not available action")
	}

	if err != nil {
		//mapError := map[string]string{"message": "Benchmark Error"}
		//return c.JSON(http.StatusFailedDependency, &mapError)
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// BenchmarkAction is func to action Benchmark
func BenchmarkAction(nsId string, mcisId string, action string, option string) (BenchmarkInfoArray, error) {

	var results BenchmarkInfoArray

	vmList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return BenchmarkInfoArray{}, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp, _ := GetVmIp(nsId, mcisId, vmId)

		go CallMilkyway(&wg, vmList, nsId, mcisId, vmId, vmIp, action, option, &results)
	}
	wg.Wait() //goroutine sync wg

	return results, nil

}
