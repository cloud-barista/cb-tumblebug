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

// Package mci is to manage multi-cloud infra
package infra

import (
	"encoding/json"
	"fmt"
	"io"

	"strings"

	"encoding/csv"
	"net/http"
	"os"

	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"

	"github.com/rs/zerolog/log"
)

// InstallBenchmarkAgentToMci is func to install milkyway agents in MCI
func InstallBenchmarkAgentToMci(nsId string, mciId string, req *model.MciCmdReq, option string) ([]model.SshCmdResult, error) {

	// SSH command to install benchmarking agent
	cmd := "wget https://github.com/cloud-barista/cb-milkyway/raw/master/src/milkyway -O ~/milkyway; chmod +x ~/milkyway; ~/milkyway > /dev/null 2>&1 & sudo netstat -tulpn | grep milkyway"

	if option == "update" {
		cmd = "killall milkyway; rm ~/milkyway; wget https://github.com/cloud-barista/cb-milkyway/raw/master/src/milkyway -O ~/milkyway; chmod +x ~/milkyway; ~/milkyway > /dev/null 2>&1 & sudo netstat -tulpn | grep milkyway"

	}

	// Replace given parameter with the installation cmd
	req.Command = append(req.Command, cmd)

	sshCmdResult, err := RemoteCommandToMci(nsId, mciId, "", "", "", req)

	if err != nil {
		temp := []model.SshCmdResult{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	return sshCmdResult, nil

}

// CallMilkyway is func to call milkyway agents
func CallMilkyway(wg *sync.WaitGroup, vmList []string, nsId string, mciId string, vmId string, vmIp string, action string, option string, results *model.BenchmarkInfoArray) {
	defer wg.Done() //goroutine sync done

	url := "http://" + vmIp + model.MilkywayPort + action
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
	requestBody := JsonTemplate{}
	requestBody.Host = option
	payload, _ := json.MarshalIndent(requestBody, "", "  ")

	if action == "mrtt" {
		reqTmp := model.MultihostBenchmarkReq{}
		for _, vm := range vmList {
			vmIdTmp := vm
			vmIpTmp, _, _, err := GetVmIp(nsId, mciId, vmIdTmp)
			if err != nil {
				log.Error().Err(err).Msg("")
			}
			log.Debug().Msg("[Test for vmList " + vmIdTmp + ", " + vmIpTmp + "]")

			hostTmp := model.BenchmarkReq{}
			hostTmp.Host = vmIpTmp
			hostTmp.Spec = GetVmSpecId(nsId, mciId, vmIdTmp)
			reqTmp.Multihost = append(reqTmp.Multihost, hostTmp)
		}
		common.PrintJsonPretty(reqTmp)
		payload, _ = json.MarshalIndent(reqTmp, "", "  ")
	}

	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		log.Err(err).Msg("")
	}
	errStr := ""
	resultTmp := model.BenchmarkInfo{}

	res, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("")
		errStr = err.Error()
	} else {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			log.Error().Err(err).Msg("")
			errStr = err.Error()
		}
		defer res.Body.Close()
		fmt.Println(string(body))

		// fmt.Println("HTTP Status code: " + strconv.Itoa(res.StatusCode))
		switch {
		case res.StatusCode >= 400 || res.StatusCode < 200:
			err := fmt.Errorf(string(body))
			log.Error().Err(err).Msg("")
			errStr = err.Error()
		}

		//benchInfoTmp := model.BenchmarkInfo{}

		err2 := json.Unmarshal(body, &resultTmp)
		if err2 != nil {
			log.Error().Err(err2).Msg("")
			errStr = err2.Error()
		}
	}
	if errStr != "" {
		resultTmp.Result = errStr
	}
	resultTmp.SpecId = GetVmSpecId(nsId, mciId, vmId)
	results.ResultArray = append(results.ResultArray, resultTmp)
}

// RunAllBenchmarks is func to get all Benchmarks
func RunAllBenchmarks(nsId string, mciId string, host string) (*model.BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := &model.BenchmarkInfoArray{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	target := host

	action := "all"
	fmt.Println("[Get MCI benchmark action: " + action + target)

	option := "localhost"
	option = target

	content := model.BenchmarkInfoArray{}

	allBenchCmd := []string{"cpus", "cpum", "memR", "memW", "fioR", "fioW", "dbR", "dbW"}

	resultMap := make(map[string]model.SpecBenchmarkInfo)

	for i, v := range allBenchCmd {
		log.Debug().Msg("[Benchmark] " + v)
		content, err = BenchmarkAction(nsId, mciId, v, option)
		for _, k := range content.ResultArray {
			SpecId := k.SpecId
			Result := k.Result
			specBenchInfoTmp := model.SpecBenchmarkInfo{}

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

	const empty = ""

	const mrttArrayXMax = 300
	const mrttArrayYMax = 300
	mrttArray := make([][]string, mrttArrayXMax)
	for i := 0; i < mrttArrayXMax; i++ {
		mrttArray[i] = make([]string, mrttArrayYMax)
		for j := 0; j < mrttArrayYMax; j++ {
			mrttArray[i][j] = empty
		}
	}

	rttIndexMapX := make(map[string]int)
	cntTargetX := 1
	rttIndexMapY := make(map[string]int)
	cntTargetY := 1

	action = "mrtt"
	log.Debug().Msg("[Benchmark] " + action)
	content, err = BenchmarkAction(nsId, mciId, action, option)
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
	// ordering

	// fmt.Printf("mrttArray[0]: %v", mrttArray[0])
	// fmt.Printf("rttIndexMapX: %v", rttIndexMapX)
	// fmt.Printf("rttIndexMapY: %v", rttIndexMapY)

	for refIndex, refVal := range mrttArray[0] {
		if refIndex == 0 {
			continue
		}
		if refVal == empty {
			break
		}
		orgIndex := rttIndexMapX[refVal]

		// fmt.Printf("[Replace] refIndex:%v (refVal:%v), mrttArray[refIndex]:%v \n", refIndex, refVal, mrttArray[refIndex])
		// fmt.Printf("[Replace] orgIndex:%v, mrttArray[orgIndex]:%v \n", orgIndex, mrttArray[orgIndex])

		tmp := mrttArray[refIndex]
		mrttArray[refIndex] = mrttArray[orgIndex]
		mrttArray[orgIndex] = tmp

		rttIndexMapX[refVal] = refIndex
		rttIndexMapX[mrttArray[orgIndex][0]] = orgIndex

	}
	// change index name from specId to regionName
	for i := 1; i < len(mrttArray[0]); i++ {
		targetSpecId := mrttArray[0][i]
		if targetSpecId == empty {
			break
		}
		specInfo := model.TbSpecInfo{}
		specInfo, err = resource.GetSpec(model.SystemCommonNs, targetSpecId)
		if err == nil {

			mrttArray[0][i] = specInfo.RegionName
			mrttArray[i][0] = specInfo.RegionName
		}
	}

	// Fill empty with transpose matix
	for i := 1; i < len(mrttArray[0]); i++ {
		firstValue := mrttArray[i][1]
		if firstValue == empty {
			for j := 1; j < len(mrttArray[0]); j++ {
				mrttArray[i][j] = mrttArray[j][i]
			}
		}
	}

	file2, err := os.OpenFile("cloudlatencymap.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file2.Close()
	csvWriter2 := csv.NewWriter(file2)
	csvWriter2.WriteAll(mrttArray)
	csvWriter2.Flush()

	if err != nil {
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// RunLatencyBenchmark is func to get MCI benchmark for network latency
func RunLatencyBenchmark(nsId string, mciId string, host string) (*model.BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := &model.BenchmarkInfoArray{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	target := host
	option := target

	content := model.BenchmarkInfoArray{}

	const empty = ""

	const mrttArrayXMax = 300
	const mrttArrayYMax = 300
	mrttArray := make([][]string, mrttArrayXMax)
	for i := 0; i < mrttArrayXMax; i++ {
		mrttArray[i] = make([]string, mrttArrayYMax)
		for j := 0; j < mrttArrayYMax; j++ {
			mrttArray[i][j] = empty
		}
	}

	rttIndexMapX := make(map[string]int)
	cntTargetX := 1
	rttIndexMapY := make(map[string]int)
	cntTargetY := 1

	action := "mrtt"
	log.Debug().Msg("[Benchmark] " + action)
	content, err = BenchmarkAction(nsId, mciId, action, option)
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
	// ordering

	// fmt.Printf("mrttArray[0]: %v", mrttArray[0])
	// fmt.Printf("rttIndexMapX: %v", rttIndexMapX)
	// fmt.Printf("rttIndexMapY: %v", rttIndexMapY)

	for refIndex, refVal := range mrttArray[0] {
		if refIndex == 0 {
			continue
		}
		if refVal == empty {
			break
		}
		orgIndex := rttIndexMapX[refVal]

		// fmt.Printf("[Replace] refIndex:%v (refVal:%v), mrttArray[refIndex]:%v \n", refIndex, refVal, mrttArray[refIndex])
		// fmt.Printf("[Replace] orgIndex:%v, mrttArray[orgIndex]:%v \n", orgIndex, mrttArray[orgIndex])

		tmp := mrttArray[refIndex]
		mrttArray[refIndex] = mrttArray[orgIndex]
		mrttArray[orgIndex] = tmp

		rttIndexMapX[refVal] = refIndex
		rttIndexMapX[mrttArray[orgIndex][0]] = orgIndex

	}
	// change index name from specId to regionName
	for i := 1; i < len(mrttArray[0]); i++ {
		targetSpecId := mrttArray[0][i]
		if targetSpecId == empty {
			break
		}
		specInfo := model.TbSpecInfo{}
		specInfo, err = resource.GetSpec(model.SystemCommonNs, targetSpecId)
		if err == nil {
			mrttArray[0][i] = specInfo.RegionName
			mrttArray[i][0] = specInfo.RegionName
		}
	}

	// Fill empty with transpose matix
	for i := 1; i < len(mrttArray[0]); i++ {
		if mrttArray[i][0] == empty {
			break
		}
		firstValue := mrttArray[i][1]
		if firstValue == empty {
			for j := 1; j < len(mrttArray[0]); j++ {
				mrttArray[i][j] = mrttArray[j][i]
			}
		}
	}

	file2, err := os.OpenFile("cloudlatencymap.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file2.Close()
	csvWriter2 := csv.NewWriter(file2)
	csvWriter2.WriteAll(mrttArray)
	csvWriter2.Flush()

	if err != nil {
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// CoreGetBenchmark is func to get Benchmark
func CoreGetBenchmark(nsId string, mciId string, action string, host string) (*model.BenchmarkInfoArray, error) {

	var err error

	err = common.CheckString(nsId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.BenchmarkInfoArray{}
		log.Error().Err(err).Msg("")
		return &temp, err
	}
	check, _ := CheckMci(nsId, mciId)

	if !check {
		temp := &model.BenchmarkInfoArray{}
		err := fmt.Errorf("The mci " + mciId + " does not exist.")
		return temp, err
	}

	target := host

	log.Debug().Msg("[Get MCI benchmark action: " + action + target)

	option := "localhost"
	option = target

	content := model.BenchmarkInfoArray{}

	vaildActions := "install init cpus cpum memR memW fioR fioW dbR dbW rtt mrtt clean"

	log.Debug().Msg("[Benchmark] " + action)
	if strings.Contains(vaildActions, action) {
		content, err = BenchmarkAction(nsId, mciId, action, option)
	} else {
		return nil, fmt.Errorf("Not available action")
	}

	if err != nil {
		return nil, fmt.Errorf("Benchmark Error")
	}

	return &content, nil
}

// BenchmarkAction is func to action Benchmark
func BenchmarkAction(nsId string, mciId string, action string, option string) (model.BenchmarkInfoArray, error) {

	var results model.BenchmarkInfoArray

	vmList, err := ListVmId(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.BenchmarkInfoArray{}, err
	}

	//goroutin sync wg
	var wg sync.WaitGroup

	for _, vmId := range vmList {
		wg.Add(1)

		vmIp, _, _, err := GetVmIp(nsId, mciId, vmId)
		if err != nil {
			log.Error().Err(err).Msg("")
			wg.Done()
			// continue to next vm even if error occurs
		} else {
			go CallMilkyway(&wg, vmList, nsId, mciId, vmId, vmIp, action, option, &results)
		}
	}
	wg.Wait() //goroutine sync wg

	return results, nil

}
