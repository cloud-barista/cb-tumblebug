package mcir

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	//"strings"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/go-resty/resty/v2"

	//"github.com/cloud-barista/cb-tumblebug/src/core/mcis"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xwb1989/sqlparser"
)

type SpiderSpecInfo struct { // Spider
	// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMSpecHandler.go

	Region string
	Name   string
	VCpu   SpiderVCpuInfo
	Mem    string
	Gpu    []SpiderGpuInfo

	KeyValueList []common.KeyValue
}

type SpiderVCpuInfo struct { // Spider
	Count string
	Clock string // GHz
}

type SpiderGpuInfo struct { // Spider
	Count string
	Mfr   string
	Model string
	Mem   string
}

type TbSpecReq struct { // Tumblebug
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
	Description    string `json:"description"`
}

type TbSpecInfo struct { // Tumblebug
	Id                    string  `json:"id"`
	Name                  string  `json:"name"`
	ConnectionName        string  `json:"connectionName"`
	CspSpecName           string  `json:"cspSpecName"`
	Os_type               string  `json:"os_type"`
	Num_vCPU              uint16  `json:"num_vCPU"`
	Num_core              uint16  `json:"num_core"`
	Mem_GiB               uint16  `json:"mem_GiB"`
	Storage_GiB           uint32  `json:"storage_GiB"`
	Description           string  `json:"description"`
	Cost_per_hour         float32 `json:"cost_per_hour"`
	Num_storage           uint8   `json:"num_storage"`
	Max_num_storage       uint8   `json:"max_num_storage"`
	Max_total_storage_TiB uint16  `json:"max_total_storage_TiB"`
	Net_bw_Gbps           uint16  `json:"net_bw_Gbps"`
	Ebs_bw_Mbps           uint32  `json:"ebs_bw_Mbps"`
	Gpu_model             string  `json:"gpu_model"`
	Num_gpu               uint8   `json:"num_gpu"`
	Gpumem_GiB            uint16  `json:"gpumem_GiB"`
	Gpu_p2p               string  `json:"gpu_p2p"`
	OrderInFilteredResult uint16  `json:"orderInFilteredResult"`
	EvaluationStatus      string  `json:"evaluationStatus"`
	EvaluationScore_01    float32 `json:"evaluationScore_01"`
	EvaluationScore_02    float32 `json:"evaluationScore_02"`
	EvaluationScore_03    float32 `json:"evaluationScore_03"`
	EvaluationScore_04    float32 `json:"evaluationScore_04"`
	EvaluationScore_05    float32 `json:"evaluationScore_05"`
	EvaluationScore_06    float32 `json:"evaluationScore_06"`
	EvaluationScore_07    float32 `json:"evaluationScore_07"`
	EvaluationScore_08    float32 `json:"evaluationScore_08"`
	EvaluationScore_09    float32 `json:"evaluationScore_09"`
	EvaluationScore_10    float32 `json:"evaluationScore_10"`
}

func ConvertSpiderSpecToTumblebugSpec(spiderSpec SpiderSpecInfo) (TbSpecInfo, error) {
	if spiderSpec.Name == "" {
		err := fmt.Errorf("ConvertSpiderSpecToTumblebugSpec failed; spiderSpec.Name == \"\" ")
		emptyTumblebugSpec := TbSpecInfo{}
		return emptyTumblebugSpec, err
	}

	tumblebugSpec := TbSpecInfo{}

	tumblebugSpec.Name = spiderSpec.Name
	tumblebugSpec.CspSpecName = spiderSpec.Name
	tempUint64, _ := strconv.ParseUint(spiderSpec.VCpu.Count, 10, 16)
	tumblebugSpec.Num_vCPU = uint16(tempUint64)
	tempFloat64, _ := strconv.ParseFloat(spiderSpec.Mem, 32)
	tumblebugSpec.Mem_GiB = uint16(tempFloat64 / 1024) //fmt.Sprintf("%.0f", tempFloat64/1024)

	return tumblebugSpec, nil
}

type SpiderSpecList struct {
	Vmspec []SpiderSpecInfo `json:"vmspec"`
}

func LookupSpecList(connConfig string) (SpiderSpecList, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		url := common.SPIDER_REST_URL + "/vmspec"

		// Create Req body
		type JsonTemplate struct {
			ConnectionName string `json:"ConnectionName"`
		}
		tempReq := JsonTemplate{}
		tempReq.ConnectionName = connConfig

		client := resty.New()
		client.SetAllowGetMethodPayload(true)

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderSpecList{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Get(url)

		if err != nil {
			common.CBLog.Error(err)
			content := SpiderSpecList{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body()))

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			content := SpiderSpecList{}
			return content, err
		}

		temp := resp.Result().(*SpiderSpecList)
		return *temp, nil

	} else {

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return SpiderSpecList{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return SpiderSpecList{}, err
		}
		defer ccm.Close()

		result, err := ccm.ListVMSpecByParam(connConfig)
		if err != nil {
			common.CBLog.Error(err)
			return SpiderSpecList{}, err
		}

		temp := SpiderSpecList{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}
		return temp, nil

	}
}

//func LookupSpec(u *TbSpecInfo) (SpiderSpecInfo, error) {
func LookupSpec(connConfig string, specName string) (SpiderSpecInfo, error) {

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		//url := common.SPIDER_REST_URL + "/vmspec/" + u.CspSpecName
		url := common.SPIDER_REST_URL + "/vmspec/" + specName

		// Create Req body
		type JsonTemplate struct {
			ConnectionName string `json:"ConnectionName"`
		}
		tempReq := JsonTemplate{}
		tempReq.ConnectionName = connConfig

		client := resty.New()
		client.SetAllowGetMethodPayload(true)

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Get(url)

		if err != nil {
			common.CBLog.Error(err)
			content := SpiderSpecInfo{}
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
			return content, err
		}

		fmt.Println(string(resp.Body()))

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			content := SpiderSpecInfo{}
			return content, err
		}

		temp := resp.Result().(*SpiderSpecInfo)
		return *temp, nil

	} else {

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return SpiderSpecInfo{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return SpiderSpecInfo{}, err
		}
		defer ccm.Close()

		result, err := ccm.GetVMSpecByParam(connConfig, specName)
		if err != nil {
			common.CBLog.Error(err)
			return SpiderSpecInfo{}, err
		}

		temp := SpiderSpecInfo{}
		err2 := json.Unmarshal([]byte(result), &temp)
		if err2 != nil {
			fmt.Errorf("an error occurred while unmarshaling: " + err2.Error())
		}
		return temp, nil

	}
}

func FetchSpecs(nsId string) (connConfigCount uint, specCount uint, err error) {

	nsId = common.GenId(nsId)

	connConfigs, err := common.GetConnConfigList()
	if err != nil {
		common.CBLog.Error(err)
		return 0, 0, err
	}

	for _, connConfig := range connConfigs.Connectionconfig {
		fmt.Println("connConfig " + connConfig.ConfigName)

		spiderSpecList, err := LookupSpecList(connConfig.ConfigName)
		if err != nil {
			common.CBLog.Error(err)
			return 0, 0, err
		}

		for _, spiderSpec := range spiderSpecList.Vmspec {
			tumblebugSpec, err := ConvertSpiderSpecToTumblebugSpec(spiderSpec)
			if err != nil {
				common.CBLog.Error(err)
				return 0, 0, err
			}

			tumblebugSpecId := connConfig.ConfigName + "-" + tumblebugSpec.Name
			//fmt.Println("tumblebugSpecId: " + tumblebugSpecId) // for debug

			check, _, err := LowerizeAndCheckResource(nsId, "spec", tumblebugSpecId)
			if check == true {
				common.CBLog.Infoln("The spec " + tumblebugSpecId + " already exists in TB; continue")
				continue
			} else if err != nil {
				common.CBLog.Infoln("Cannot check the existence of " + tumblebugSpecId + " in TB; continue")
				continue
			} else {
				tumblebugSpec.Id = tumblebugSpecId
				tumblebugSpec.Name = tumblebugSpecId
				tumblebugSpec.ConnectionName = connConfig.ConfigName

				_, err := RegisterSpecWithInfo(nsId, &tumblebugSpec)
				if err != nil {
					common.CBLog.Error(err)
					return 0, 0, err
				}
			}
			specCount++
		}
		connConfigCount++
	}
	return connConfigCount, specCount, nil
}

func RegisterSpecWithCspSpecName(nsId string, u *TbSpecReq) (TbSpecInfo, error) {

	nsId = common.GenId(nsId)

	_, lowerizedNsId, _ := common.LowerizeAndCheckNs(nsId)
	nsId = lowerizedNsId

	check, lowerizedName, err := LowerizeAndCheckResource(nsId, "spec", u.Name)
	u.Name = lowerizedName

	if check == true {
		temp := TbSpecInfo{}
		err := fmt.Errorf("The spec " + u.Name + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := TbSpecInfo{}
		err := fmt.Errorf("Failed to check the existence of the spec " + lowerizedName + ".")
		return temp, err
	}

	res, err := LookupSpec(u.ConnectionName, u.CspSpecName)
	if err != nil {
		common.CBLog.Error(err)
		err := fmt.Errorf("an error occurred while lookup spec via CB-Spider")
		emptySpecInfoObj := TbSpecInfo{}
		return emptySpecInfoObj, err
	}

	content := TbSpecInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = common.GenId(u.Name)
	content.CspSpecName = res.Name
	content.ConnectionName = u.ConnectionName

	tempUint64, _ := strconv.ParseUint(res.VCpu.Count, 10, 16)
	content.Num_vCPU = uint16(tempUint64)

	//content.Num_core = res.Num_core

	tempFloat64, _ := strconv.ParseFloat(res.Mem, 32)
	content.Mem_GiB = uint16(tempFloat64 / 1024)

	//content.Storage_GiB = res.Storage_GiB
	//content.Description = res.Description

	sql := "INSERT INTO `spec`(" +
		"`namespace`, " +
		"`id`, " +
		"`connectionName`, " +
		"`cspSpecName`, " +
		"`name`, " +
		"`os_type`, " +
		"`num_vCPU`, " +
		"`num_core`, " +
		"`mem_GiB`, " +
		"`storage_GiB`, " +
		"`description`, " +
		"`cost_per_hour`, " +
		"`num_storage`, " +
		"`max_num_storage`, " +
		"`max_total_storage_TiB`, " +
		"`net_bw_Gbps`, " +
		"`ebs_bw_Mbps`, " +
		"`gpu_model`, " +
		"`num_gpu`, " +
		"`gpumem_GiB`, " +
		"`gpu_p2p`, " +
		"`orderInFilteredResult`, " +
		"`evaluationStatus`, " +
		"`evaluationScore_01`, " +
		"`evaluationScore_02`, " +
		"`evaluationScore_03`, " +
		"`evaluationScore_04`, " +
		"`evaluationScore_05`, " +
		"`evaluationScore_06`, " +
		"`evaluationScore_07`, " +
		"`evaluationScore_08`, " +
		"`evaluationScore_09`, " +
		"`evaluationScore_10`) " +
		"VALUES ('" +
		nsId + "', '" +
		content.Id + "', '" +
		content.ConnectionName + "', '" +
		content.CspSpecName + "', '" +
		content.Name + "', '" +
		content.Os_type + "', '" +
		strconv.Itoa(int(content.Num_vCPU)) + "', '" +
		strconv.Itoa(int(content.Num_core)) + "', '" +
		strconv.Itoa(int(content.Mem_GiB)) + "', '" +
		strconv.Itoa(int(content.Storage_GiB)) + "', '" +
		content.Description + "', '" +
		fmt.Sprintf("%.6f", content.Cost_per_hour) + "', '" +
		strconv.Itoa(int(content.Num_storage)) + "', '" +
		strconv.Itoa(int(content.Max_num_storage)) + "', '" +
		strconv.Itoa(int(content.Max_total_storage_TiB)) + "', '" +
		strconv.Itoa(int(content.Net_bw_Gbps)) + "', '" +
		strconv.Itoa(int(content.Ebs_bw_Mbps)) + "', '" +
		content.Gpu_model + "', '" +
		strconv.Itoa(int(content.Num_gpu)) + "', '" +
		strconv.Itoa(int(content.Gpumem_GiB)) + "', '" +
		content.Gpu_p2p + "', '" +
		strconv.Itoa(int(content.OrderInFilteredResult)) + "', '" +
		content.EvaluationStatus + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_01) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_02) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_03) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_04) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_05) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_06) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_07) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_08) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_09) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_10) + "');"

	fmt.Println("sql: " + sql)
	// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
	_, err = sqlparser.Parse(sql)
	if err != nil {
		return content, err
	}

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := common.GenResourceKey(nsId, "spec", content.Id)
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// register information related with MCIS recommendation
	RegisterRecommendList(nsId, content.ConnectionName, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	stmt, err := common.MYDB.Prepare(sql)
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Data inserted successfully..")
	}

	return content, nil
}

func RegisterSpecWithInfo(nsId string, content *TbSpecInfo) (TbSpecInfo, error) {

	nsId = common.GenId(nsId)

	//_, lowerizedNsId, _ := common.LowerizeAndCheckNs(nsId)
	//nsId = lowerizedNsId
	nsId = common.GenId(nsId)

	check, lowerizedName, err := LowerizeAndCheckResource(nsId, "spec", content.Name)
	content.Name = lowerizedName

	if check == true {
		temp := TbSpecInfo{}
		err := fmt.Errorf("The spec " + content.Name + " already exists.")
		return temp, err
	}

	content.Id = content.Name
	//content.Name = content.Name

	sql := "INSERT INTO `spec`(" +
		"`namespace`, " +
		"`id`, " +
		"`connectionName`, " +
		"`cspSpecName`, " +
		"`name`, " +
		"`os_type`, " +
		"`num_vCPU`, " +
		"`num_core`, " +
		"`mem_GiB`, " +
		"`storage_GiB`, " +
		"`description`, " +
		"`cost_per_hour`, " +
		"`num_storage`, " +
		"`max_num_storage`, " +
		"`max_total_storage_TiB`, " +
		"`net_bw_Gbps`, " +
		"`ebs_bw_Mbps`, " +
		"`gpu_model`, " +
		"`num_gpu`, " +
		"`gpumem_GiB`, " +
		"`gpu_p2p`, " +
		"`orderInFilteredResult`, " +
		"`evaluationStatus`, " +
		"`evaluationScore_01`, " +
		"`evaluationScore_02`, " +
		"`evaluationScore_03`, " +
		"`evaluationScore_04`, " +
		"`evaluationScore_05`, " +
		"`evaluationScore_06`, " +
		"`evaluationScore_07`, " +
		"`evaluationScore_08`, " +
		"`evaluationScore_09`, " +
		"`evaluationScore_10`) " +
		"VALUES ('" +
		nsId + "', '" +
		content.Id + "', '" +
		content.ConnectionName + "', '" +
		content.CspSpecName + "', '" +
		content.Name + "', '" +
		content.Os_type + "', '" +
		strconv.Itoa(int(content.Num_vCPU)) + "', '" +
		strconv.Itoa(int(content.Num_core)) + "', '" +
		strconv.Itoa(int(content.Mem_GiB)) + "', '" +
		strconv.Itoa(int(content.Storage_GiB)) + "', '" +
		content.Description + "', '" +
		fmt.Sprintf("%.6f", content.Cost_per_hour) + "', '" +
		strconv.Itoa(int(content.Num_storage)) + "', '" +
		strconv.Itoa(int(content.Max_num_storage)) + "', '" +
		strconv.Itoa(int(content.Max_total_storage_TiB)) + "', '" +
		strconv.Itoa(int(content.Net_bw_Gbps)) + "', '" +
		strconv.Itoa(int(content.Ebs_bw_Mbps)) + "', '" +
		content.Gpu_model + "', '" +
		strconv.Itoa(int(content.Num_gpu)) + "', '" +
		strconv.Itoa(int(content.Gpumem_GiB)) + "', '" +
		content.Gpu_p2p + "', '" +
		strconv.Itoa(int(content.OrderInFilteredResult)) + "', '" +
		content.EvaluationStatus + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_01) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_02) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_03) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_04) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_05) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_06) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_07) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_08) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_09) + "', '" +
		fmt.Sprintf("%.6f", content.EvaluationScore_10) + "');"

	fmt.Println("sql: " + sql)
	// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
	_, err = sqlparser.Parse(sql)
	if err != nil {
		return *content, err
	}

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := common.GenResourceKey(nsId, "spec", content.Id)
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return *content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// register information related with MCIS recommendation
	RegisterRecommendList(nsId, content.ConnectionName, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	stmt, err := common.MYDB.Prepare(sql)
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Data inserted successfully..")
	}

	return *content, nil
}

func RegisterRecommendList(nsId string, connectionName string, cpuSize uint16, memSize uint16, diskSize uint32, specId string, price float32) error {

	nsId = common.GenId(nsId)

	//fmt.Println("[Get MCISs")
	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + strconv.Itoa(int(cpuSize)) + "/memSize/" + strconv.Itoa(int(memSize)) + "/diskSize/" + strconv.Itoa(int(diskSize)) + "/specId/" + specId
	fmt.Println(key)

	mapA := map[string]string{"id": specId, "price": fmt.Sprintf("%.6f", price), "connectionName": connectionName}
	Val, _ := json.Marshal(mapA)

	err := common.CBStore.Put(string(key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("===============================================")
	return nil

}

func DelRecommendSpec(nsId string, specId string, cpuSize uint16, memSize uint16, diskSize uint32) error {

	nsId = common.GenId(nsId)

	fmt.Println("DelRecommendSpec()")

	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + strconv.Itoa(int(cpuSize)) + "/memSize/" + strconv.Itoa(int(memSize)) + "/diskSize/" + strconv.Itoa(int(diskSize)) + "/specId/" + specId

	err := common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil

}

func FilterSpecs(nsId string, filter TbSpecInfo) ([]TbSpecInfo, error) {

	nsId = common.GenId(nsId)

	tempList := []TbSpecInfo{}

	sqlQuery := "SELECT * FROM `spec` WHERE `namespace`='" + nsId + "'"

	if filter.Num_vCPU > 0 {
		sqlQuery += " AND `num_vCPU`=" + strconv.Itoa(int(filter.Num_vCPU))
	}
	if filter.Mem_GiB > 0 {
		sqlQuery += " AND `mem_GiB`=" + strconv.Itoa(int(filter.Mem_GiB))
	}
	if filter.Storage_GiB > 0 {
		sqlQuery += " AND `storage_GiB`=" + strconv.Itoa(int(filter.Storage_GiB))
	}
	sqlQuery += ";"
	_, err := sqlparser.Parse(sqlQuery)
	if err != nil {
		return tempList, err
	}

	/*
		stmt, err := common.MYDB.Prepare(sqlQuery)
		if err != nil {
			fmt.Println(err.Error())
		}
		result, err := stmt.Exec()
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("SELECTed successfully..")
		}

		result.RowsAffected

		temp := []TbSpecInfo{}
		return temp, nil
	*/

	rows, err := common.MYDB.Query(sqlQuery)
	if err != nil {
		common.CBLog.Error(err)
		return tempList, err
	}

	for rows.Next() {
		temp := TbSpecInfo{}
		var tempString string
		err := rows.Scan(
			&tempString,
			&temp.Id,
			&temp.Name,
			&temp.ConnectionName,
			&temp.CspSpecName,
			&temp.Os_type,
			&temp.Num_vCPU,
			&temp.Num_core,
			&temp.Mem_GiB,
			&temp.Storage_GiB,
			&temp.Description,
			&temp.Cost_per_hour,
			&temp.Num_storage,
			&temp.Max_num_storage,
			&temp.Max_total_storage_TiB,
			&temp.Net_bw_Gbps,
			&temp.Ebs_bw_Mbps,
			&temp.Gpu_model,
			&temp.Num_gpu,
			&temp.Gpumem_GiB,
			&temp.Gpu_p2p,
			&temp.OrderInFilteredResult,
			&temp.EvaluationStatus,
			&temp.EvaluationScore_01,
			&temp.EvaluationScore_02,
			&temp.EvaluationScore_03,
			&temp.EvaluationScore_04,
			&temp.EvaluationScore_05,
			&temp.EvaluationScore_06,
			&temp.EvaluationScore_07,
			&temp.EvaluationScore_08,
			&temp.EvaluationScore_09,
			&temp.EvaluationScore_10,
		)
		if err != nil {
			common.CBLog.Error(err)
			return tempList, err
		}
		tempList = append(tempList, temp)
	}
	return tempList, nil
}
