package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	//"strings"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/labstack/echo"

	//"github.com/cloud-barista/cb-tumblebug/src/mcis"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xwb1989/sqlparser"
)

type TbSpecReq struct { // Tumblebug
	//Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
	Name           string `json:"name"`
	Os_type        string `json:"os_type"`
	Num_vCPU       string `json:"num_vCPU"`
	Num_core       string `json:"num_core"`
	Mem_GiB        string `json:"mem_GiB"`
	Mem_MiB        string `json:"mem_MiB"`
	Storage_GiB    string `json:"storage_GiB"`
	Description    string `json:"description"`
}

type TbSpecInfo struct { // Tumblebug
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
	Name           string `json:"name"`
	Os_type        string `json:"os_type"`
	Num_vCPU       string `json:"num_vCPU"`
	Num_core       string `json:"num_core"`
	Mem_GiB        string `json:"mem_GiB"`
	Mem_MiB        string `json:"mem_MiB"`
	Storage_GiB    string `json:"storage_GiB"`
	Description    string `json:"description"`

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

// MCIS API Proxy: Spec
func RestPostSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	fmt.Println("[POST Spec requested action: " + action)

	if action == "registerWithInfo" { // `RegisterSpecWithInfo` will be deprecated in Cappuccino.
		fmt.Println("[Registering Spec with info]")
		u := &TbSpecInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := RegisterSpecWithInfo(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} else { // if action == "registerWithCspSpecName" { // The default mode.
		fmt.Println("[Registering Spec with CspSpecName]")
		u := &TbSpecReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := RegisterSpecWithCspSpecName(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} /* else {
		mapA := map[string]string{"message": "LookupSpec(specRequest) failed."}
		return c.JSON(http.StatusFailedDependency, &mapA)
	} */

}

func RestLookupSpec(c echo.Context) error {
	u := &TbSpecReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	u.CspSpecName = c.Param("specName")
	fmt.Println("[Lookup spec]" + u.CspSpecName)
	content, err := LookupSpec(u)
	if err != nil {
		cblog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

func RestGetSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "spec"

	id := c.Param("specId")

	/*
		content := TbSpecInfo{}

		fmt.Println("[Get spec for id]" + id)
		key := common.GenResourceKey(nsId, "spec", id)
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Failed to find the spec with given ID."}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			fmt.Println("===============================================")

			json.Unmarshal([]byte(keyValue.Value), &content)
			content.Id = id // Optional. Can be omitted.

			return c.JSON(http.StatusOK, &content)
		}
	*/

	res, err := GetResource(nsId, resourceType, id)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find " + resourceType + " " + id}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

func RestGetAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "spec"

	var content struct {
		Spec []TbSpecInfo `json:"spec"`
	}

	/*
		specList := ListResourceId(nsId, "spec")

		for _, v := range specList {

			key := common.GenResourceKey(nsId, "spec", v)
			fmt.Println(key)
			keyValue, _ := store.Get(key)
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			specTmp := TbSpecInfo{}
			json.Unmarshal([]byte(keyValue.Value), &specTmp)
			specTmp.Id = v
			content.Spec = append(content.Spec, specTmp)

		}
		fmt.Printf("content %+v\n", content)

		return c.JSON(http.StatusOK, &content)
	*/

	resourceList, err := ListResource(nsId, resourceType)
	if err != nil {
		mapA := map[string]string{"message": "Failed to list " + resourceType + "s."}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	if resourceList == nil {
		return c.JSON(http.StatusOK, &content)
	}

	// When err == nil && resourceList != nil
	content.Spec = resourceList.([]TbSpecInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}

func RestPutSpec(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "spec"
	id := c.Param("specId")
	forceFlag := c.QueryParam("force")

	err := DelResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "spec"
	forceFlag := c.QueryParam("force")

	/*
		specList := ListResourceId(nsId, "spec")

		if len(specList) == 0 {
			mapA := map[string]string{"message": "There is no spec element in this namespace."}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			for _, v := range specList {
				responseCode, _, err := DelResource(nsId, "spec", v, forceFlag)
				if err != nil {
					cblog.Error(err)
					mapA := map[string]string{"message": err.Error()}
					return c.JSON(responseCode, &mapA)
				}

			}

			mapA := map[string]string{"message": "All specs has been deleted"}
			return c.JSON(http.StatusOK, &mapA)
		}
	*/

	err := DelAllResources(nsId, resourceType, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := map[string]string{"message": "All " + resourceType + "s has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestFetchSpecs(c echo.Context) error {

	nsId := c.Param("nsId")

	connConfigs, err := common.GetConnConfigList()
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{
			"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	var connConfigCount uint
	var specCount uint

	for _, connConfig := range connConfigs.Connectionconfig {
		fmt.Println("connConfig " + connConfig.ConfigName)

		spiderSpecList, err := LookupSpecList(connConfig.ConfigName)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		for _, spiderSpec := range spiderSpecList.Vmspec {
			tumblebugSpec, err := ConvertSpiderSpecToTumblebugSpec(spiderSpec)
			if err != nil {
				cblog.Error(err)
				mapA := map[string]string{
					"message": err.Error()}
				return c.JSON(http.StatusFailedDependency, &mapA)
			}

			tumblebugSpecId := connConfig.ConfigName + "-" + tumblebugSpec.Name
			//fmt.Println("tumblebugSpecId: " + tumblebugSpecId) // for debug

			check, _ := CheckResource(nsId, "spec", tumblebugSpecId)
			if check {
				cblog.Infoln("The spec " + tumblebugSpecId + " already exists in TB; continue")
				continue
			} else {
				tumblebugSpec.Id = tumblebugSpecId
				tumblebugSpec.Name = tumblebugSpecId
				tumblebugSpec.ConnectionName = connConfig.ConfigName

				_, err := RegisterSpecWithInfo(nsId, &tumblebugSpec)
				if err != nil {
					cblog.Error(err)
					mapA := map[string]string{
						"message": err.Error()}
					return c.JSON(http.StatusFailedDependency, &mapA)
				}
			}
			specCount++
		}
		connConfigCount++
	}
	mapA := map[string]string{
		"message": "Fetched " + fmt.Sprint(specCount) + " specs (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return c.JSON(http.StatusCreated, &mapA) //content)
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
	url := common.SPIDER_URL + "/vmspec"

	method := "GET"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}
	tempReq.ConnectionName = connConfig
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		cblog.Error(err)
		content := SpiderSpecList{}
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cblog.Error(err)
		content := SpiderSpecList{}
		return content, err
	}

	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		content := SpiderSpecList{}
		return content, err
	}

	temp := SpiderSpecList{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}
	return temp, nil
}

func RestLookupSpecList(c echo.Context) error {

	type JsonTemplate struct {
		ConnectionName string
	}

	u := &JsonTemplate{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Get Region List]")
	content, err := LookupSpecList(u.ConnectionName)
	if err != nil {
		cblog.Error(err)
		return c.JSONBlob(http.StatusFailedDependency, []byte(err.Error()))
	}

	return c.JSON(http.StatusOK, &content)

}

func LookupSpec(u *TbSpecReq) (SpiderSpecInfo, error) {
	url := common.SPIDER_URL + "/vmspec/" + u.CspSpecName

	method := "GET"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}
	tempReq.ConnectionName = u.ConnectionName
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		cblog.Error(err)
		content := SpiderSpecInfo{}
		//err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cblog.Error(err)
		content := SpiderSpecInfo{}
		//err := fmt.Errorf("an error occurred while reading CB-Spider's response")
		return content, err
	}

	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		content := SpiderSpecInfo{}
		return content, err
	}

	temp := SpiderSpecInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Errorf("an error occurred while unmarshaling:", err2)
	}
	return temp, nil
}

func RegisterSpecWithCspSpecName(nsId string, u *TbSpecReq) (TbSpecInfo, error) {
	check, _ := CheckResource(nsId, "spec", u.Name)

	if check {
		temp := TbSpecInfo{}
		err := fmt.Errorf("The spec " + u.Name + " already exists.")
		return temp, err
	}

	res, err := LookupSpec(u)
	if err != nil {
		cblog.Error(err)
		err := fmt.Errorf("an error occurred while lookup spec via CB-Spider")
		emptySpecInfoObj := TbSpecInfo{}
		return emptySpecInfoObj, err
	}

	content := TbSpecInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
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
	err = store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
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
	err = store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return *content, err
	}
	keyValue, _ := store.Get(string(Key))
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

	err := store.Put(string(key), string(Val))
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("===============================================")
	return nil

}

func DelRecommendSpec(nsId string, specId string, cpuSize string, memSize string, diskSize string) error {

	fmt.Println("DelRecommendSpec()")

	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize + "/specId/" + specId

	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil

}
