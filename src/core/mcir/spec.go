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
	Id                    string `json:"id"`
	Name                  string `json:"name"`
	ConnectionName        string `json:"connectionName"`
	CspSpecName           string `json:"cspSpecName"`
	Os_type               string `json:"os_type"`
	Num_vCPU              string `json:"num_vCPU"`
	Num_core              string `json:"num_core"`
	Mem_GiB               string `json:"mem_GiB"`
	Mem_MiB               string `json:"mem_MiB"`
	Storage_GiB           string `json:"storage_GiB"`
	Description           string `json:"description"`
	Cost_per_hour         string `json:"cost_per_hour"`
	Num_storage           string `json:"num_storage"`
	Max_num_storage       string `json:"max_num_storage"`
	Max_total_storage_TiB string `json:"max_total_storage_TiB"`
	Net_bw_Gbps           string `json:"net_bw_Gbps"`
	Ebs_bw_Mbps           string `json:"ebs_bw_Mbps"`
	Gpu_model             string `json:"gpu_model"`
	Num_gpu               string `json:"num_gpu"`
	Gpumem_GiB            string `json:"gpumem_GiB"`
	Gpu_p2p               string `json:"gpu_p2p"`
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
	tumblebugSpec.Num_vCPU = spiderSpec.VCpu.Count
	tumblebugSpec.Mem_MiB = spiderSpec.Mem
	temp, _ := strconv.ParseFloat(tumblebugSpec.Mem_MiB, 32)
	tumblebugSpec.Mem_GiB = fmt.Sprintf("%.0f", temp/1024)

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

			check, _ := CheckResource(nsId, "spec", tumblebugSpecId)
			if check {
				common.CBLog.Infoln("The spec " + tumblebugSpecId + " already exists in TB; continue")
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
	check, _ := CheckResource(nsId, "spec", u.Name)

	if check {
		temp := TbSpecInfo{}
		err := fmt.Errorf("The spec " + u.Name + " already exists.")
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

	//content.Os_type = res.Os_type
	content.Num_vCPU = res.VCpu.Count
	//content.Num_core = res.Num_core
	content.Mem_MiB = res.Mem
	temp, _ := strconv.ParseFloat(content.Mem_MiB, 32)
	content.Mem_GiB = fmt.Sprintf("%.0f", temp/1024)
	//content.Storage_GiB = res.Storage_GiB
	//content.Description = res.Description

	sql := "INSERT INTO `spec`(" +
		"`id`, " +
		"`connectionName`, " +
		"`cspSpecName`, " +
		"`name`, " +
		"`os_type`, " +
		"`num_vCPU`, " +
		"`num_core`, " +
		"`mem_GiB`, " +
		"`mem_MiB`, " +
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
		"`gpu_p2p`) " +
		"VALUES ('" +
		content.Id + "', '" +
		content.ConnectionName + "', '" +
		content.CspSpecName + "', '" +
		content.Name + "', '" +
		content.Os_type + "', '" +
		content.Num_vCPU + "', '" +
		content.Num_core + "', '" +
		content.Mem_GiB + "', '" +
		content.Mem_MiB + "', '" +
		content.Storage_GiB + "', '" +
		content.Description + "', '" +
		content.Cost_per_hour + "', '" +
		content.Num_storage + "', '" +
		content.Max_num_storage + "', '" +
		content.Max_total_storage_TiB + "', '" +
		content.Net_bw_Gbps + "', '" +
		content.Ebs_bw_Mbps + "', '" +
		content.Gpu_model + "', '" +
		content.Num_gpu + "', '" +
		content.Gpumem_GiB + "', '" +
		content.Gpu_p2p + "');"

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
	check, _ := CheckResource(nsId, "spec", content.Name)

	if check {
		temp := TbSpecInfo{}
		err := fmt.Errorf("The spec " + content.Name + " already exists.")
		return temp, err
	}

	//content.Id = common.GenUuid()
	content.Id = common.GenId(content.Name)
	content.Name = common.GenId(content.Name)

	sql := "INSERT INTO `spec`(" +
		"`id`, " +
		"`connectionName`, " +
		"`cspSpecName`, " +
		"`name`, " +
		"`os_type`, " +
		"`num_vCPU`, " +
		"`num_core`, " +
		"`mem_GiB`, " +
		"`mem_MiB`, " +
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
		"`gpu_p2p`) " +
		"VALUES ('" +
		content.Id + "', '" +
		content.ConnectionName + "', '" +
		content.CspSpecName + "', '" +
		content.Name + "', '" +
		content.Os_type + "', '" +
		content.Num_vCPU + "', '" +
		content.Num_core + "', '" +
		content.Mem_GiB + "', '" +
		content.Mem_MiB + "', '" +
		content.Storage_GiB + "', '" +
		content.Description + "', '" +
		content.Cost_per_hour + "', '" +
		content.Num_storage + "', '" +
		content.Max_num_storage + "', '" +
		content.Max_total_storage_TiB + "', '" +
		content.Net_bw_Gbps + "', '" +
		content.Ebs_bw_Mbps + "', '" +
		content.Gpu_model + "', '" +
		content.Num_gpu + "', '" +
		content.Gpumem_GiB + "', '" +
		content.Gpu_p2p + "');"

	fmt.Println("sql: " + sql)
	// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
	_, err := sqlparser.Parse(sql)
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

func RegisterRecommendList(nsId string, connectionName string, cpuSize string, memSize string, diskSize string, specId string, price string) error {

	//fmt.Println("[Get MCISs")
	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize + "/specId/" + specId
	fmt.Println(key)

	mapA := map[string]string{"id": specId, "price": price, "connectionName": connectionName}
	Val, _ := json.Marshal(mapA)

	err := common.CBStore.Put(string(key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	fmt.Println("===============================================")
	return nil

}

func DelRecommendSpec(nsId string, specId string, cpuSize string, memSize string, diskSize string) error {

	fmt.Println("DelRecommendSpec()")

	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize + "/specId/" + specId

	err := common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil

}
