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
)

type specReq struct {
	//Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
	Name           string `json:"name"`
	Os_type        string `json:"os_type"`
	Num_vCPU       string `json:"num_vCPU"`
	Num_core       string `json:"num_core"`
	Mem_GiB        string `json:"mem_GiB"`
	Storage_GiB    string `json:"storage_GiB"`
	Description    string `json:"description"`
}

type SpecInfo struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspSpecName    string `json:"cspSpecName"`
	Name           string `json:"name"`
	Os_type        string `json:"os_type"`
	Num_vCPU       string `json:"num_vCPU"`
	Num_core       string `json:"num_core"`
	Mem_GiB        string `json:"mem_GiB"`
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

type SpiderSpecInfo struct {
	// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMSpecHandler.go

	Region string
	Name   string
	VCpu   VCpuInfo
	Mem    string
	Gpu    []GpuInfo

	KeyValueList []common.KeyValue
}

type VCpuInfo struct {
	Count string
	Clock string // GHz
}

type GpuInfo struct {
	Count string
	Mfr   string
	Model string
	Mem   string
}

/* FYI
g.POST("/:nsId/resources/spec", restPostSpec)
g.GET("/:nsId/resources/spec/:specId", restGetSpec)
g.GET("/:nsId/resources/spec", restGetAllSpec)
g.PUT("/:nsId/resources/spec/:specId", restPutSpec)
g.DELETE("/:nsId/resources/spec/:specId", restDelSpec)
g.DELETE("/:nsId/resources/spec", restDelAllSpec)
*/

// MCIS API Proxy: Spec
func RestPostSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	fmt.Println("[POST Spec requested action: " + action)

	if action == "registerWithInfo" { // `registerSpecWithInfo` will be deprecated in Cappuccino.
		fmt.Println("[Registering Spec with info]")
		u := &SpecInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := registerSpecWithInfo(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} else { // if action == "registerWithCspSpecName" { // The default mode.
		fmt.Println("[Registering Spec with CspSpecName]")
		u := &specReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := registerSpecWithCspSpecName(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{
				"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)

	} /* else {
		mapA := map[string]string{"message": "lookupSpec(specRequest) failed."}
		return c.JSON(http.StatusFailedDependency, &mapA)
	} */

}

func RestLookupSpec(c echo.Context) error {
	u := &specReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	u.CspSpecName = c.Param("specName")
	fmt.Println("[Lookup spec]" + u.CspSpecName)
	content, _ := lookupSpec(u)

	return c.JSON(http.StatusOK, &content)

}

func RestGetSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("specId")

	content := SpecInfo{}

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
}

func RestGetAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Spec []SpecInfo `json:"spec"`
	}

	specList := getResourceList(nsId, "spec")

	for _, v := range specList {

		key := common.GenResourceKey(nsId, "spec", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		specTmp := SpecInfo{}
		json.Unmarshal([]byte(keyValue.Value), &specTmp)
		specTmp.Id = v
		content.Spec = append(content.Spec, specTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutSpec(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("specId")
	forceFlag := c.QueryParam("force")

	responseCode, _, err := delResource(nsId, "spec", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}

	mapA := map[string]string{"message": "The spec has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	specList := getResourceList(nsId, "spec")

	if len(specList) == 0 {
		mapA := map[string]string{"message": "There is no spec element in this namespace."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		for _, v := range specList {
			responseCode, _, err := delResource(nsId, "spec", v, forceFlag)
			if err != nil {
				cblog.Error(err)
				mapA := map[string]string{"message": err.Error()}
				return c.JSON(responseCode, &mapA)
			}

		}

		mapA := map[string]string{"message": "All specs has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	}
}

func lookupSpec(u *specReq) (SpiderSpecInfo, error) {
	url := SPIDER_URL + "/vmspec/" + u.CspSpecName

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
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cblog.Error(err)
		content := SpiderSpecInfo{}
		err := fmt.Errorf("an error occurred while reading CB-Spider's response")
		return content, err
	}

	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
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

func registerSpecWithCspSpecName(nsId string, u *specReq) (SpecInfo, error) {
	check, _ := checkResource(nsId, "spec", u.Name)

	if check {
		temp := SpecInfo{}
		err := fmt.Errorf("The spec " + u.Name + " already exists.")
		return temp, err
	}

	res, err := lookupSpec(u)
	if err != nil {
		cblog.Error(err)
		err := fmt.Errorf("an error occurred while lookup spec via CB-Spider")
		emptySpecInfoObj := SpecInfo{}
		return emptySpecInfoObj, err
	}

	/* FYI
	type SpiderSpecInfo struct {
		// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VMSpecHandler.go

		Region string
		Name   string
		VCpu   VCpuInfo
		Mem    string
		Gpu    []GpuInfo

		KeyValueList []common.KeyValue
	}
	*/

	content := SpecInfo{}
	//content.Id = common.GenUuid()
	content.Id = u.Name
	content.Name = u.Name
	content.CspSpecName = res.Name
	content.ConnectionName = u.ConnectionName

	//content.Os_type = res.Os_type
	content.Num_vCPU = res.VCpu.Count
	//content.Num_core = res.Num_core
	content.Mem_GiB = res.Mem
	//content.Storage_GiB = res.Storage_GiB
	//content.Description = res.Description

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := common.GenResourceKey(nsId, "spec", content.Id)
	mapA := map[string]string{
		"connectionName": content.ConnectionName,
		"cspSpecName":    content.CspSpecName,
		"name":           content.Name,
		"os_type":        content.Os_type,
		"Num_vCPU":       content.Num_vCPU,
		"Num_core":       content.Num_core,
		"mem_GiB":        content.Mem_GiB,
		"storage_GiB":    content.Storage_GiB,
		"description":    content.Description,

		"cost_per_hour":         content.Cost_per_hour,
		"num_storage":           content.Num_storage,
		"max_num_storage":       content.Max_num_storage,
		"max_total_storage_TiB": content.Max_total_storage_TiB,
		"net_bw_Gbps":           content.Net_bw_Gbps,
		"ebs_bw_Mbps":           content.Ebs_bw_Mbps,
		"gpu_model":             content.Gpu_model,
		"num_gpu":               content.Num_gpu,
		"gpumem_GiB":            content.Gpumem_GiB,
		"gpu_p2p":               content.Gpu_p2p,
	}
	Val, _ := json.Marshal(mapA)
	err = store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// register information related with MCIS recommendation
	registerRecommendList(nsId, content.ConnectionName, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	return content, nil
}

func registerSpecWithInfo(nsId string, content *SpecInfo) (SpecInfo, error) {
	check, _ := checkResource(nsId, "spec", content.Name)

	if check {
		temp := SpecInfo{}
		err := fmt.Errorf("The spec " + content.Name + " already exists.")
		return temp, err
	}

	//content.Id = common.GenUuid()
	content.Id = content.Name

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := common.GenResourceKey(nsId, "spec", content.Id)
	mapA := map[string]string{
		"connectionName": content.ConnectionName,
		"cspSpecName":    content.CspSpecName,
		"name":           content.Name,
		"os_type":        content.Os_type,
		"Num_vCPU":       content.Num_vCPU,
		"Num_core":       content.Num_core,
		"mem_GiB":        content.Mem_GiB,
		"storage_GiB":    content.Storage_GiB,
		"description":    content.Description,

		"cost_per_hour":         content.Cost_per_hour,
		"num_storage":           content.Num_storage,
		"max_num_storage":       content.Max_num_storage,
		"max_total_storage_TiB": content.Max_total_storage_TiB,
		"net_bw_Gbps":           content.Net_bw_Gbps,
		"ebs_bw_Mbps":           content.Ebs_bw_Mbps,
		"gpu_model":             content.Gpu_model,
		"num_gpu":               content.Num_gpu,
		"gpumem_GiB":            content.Gpumem_GiB,
		"gpu_p2p":               content.Gpu_p2p,
	}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return *content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// register information related with MCIS recommendation
	registerRecommendList(nsId, content.ConnectionName, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	return *content, nil
}

func registerRecommendList(nsId string, connectionName string, cpuSize string, memSize string, diskSize string, specId string, price string) error {

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

/*
func getSpecList(nsId string) []string {

	fmt.Println("[Get specs")
	key := "/ns/" + nsId + "/resources/spec"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var specList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		specList = append(specList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/spec/"))
		//}
	}
	for _, v := range specList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return specList

}
*/

/*
func delSpec(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete spec] " + Id)

	key := genResourceKey(nsId, "spec", Id)
	fmt.Println(key)

	//get related recommend spec
	keyValue, err := store.Get(key)
	content := specInfo{}
	json.Unmarshal([]byte(keyValue.Value), &content)
	if err != nil {
		cblog.Error(err)
		return err
	}
	//

	err = store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	//delete related recommend spec
	err = delRecommendSpec(nsId, Id, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
*/

func delRecommendSpec(nsId string, specId string, cpuSize string, memSize string, diskSize string) error {

	fmt.Println("delRecommendSpec()")

	key := common.GenMcisKey(nsId, "", "") + "/cpuSize/" + cpuSize + "/memSize/" + memSize + "/diskSize/" + diskSize + "/specId/" + specId

	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil

}
