package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type specReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	Os_type        string `json:"os_type"`
	Num_vCPU       string `json:"num_vCPU"`
	Num_core       string `json:"num_core"`
	Mem_GiB        string `json:"mem_GiB"`
	Storage_GiB    string `json:"storage_GiB"`
	Description    string `json:"description"`
}

type specInfo struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
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

/* FYI
g.POST("/:nsId/resources/spec", restPostSpec)
g.GET("/:nsId/resources/spec/:specId", restGetSpec)
g.GET("/:nsId/resources/spec", restGetAllSpec)
g.PUT("/:nsId/resources/spec/:specId", restPutSpec)
g.DELETE("/:nsId/resources/spec/:specId", restDelSpec)
g.DELETE("/:nsId/resources/spec", restDelAllSpec)
*/

// MCIS API Proxy: Spec
func restPostSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST Spec requested action: " + action)

		if action == "registerWithInfo" {
			fmt.Println("[Registering Spec with info]")
			u := &specInfo{}
			if err := c.Bind(u); err != nil {
				return err
			}
			content, _ := registerSpecWithInfo(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "lookupSpec(specRequest) failed."}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST Spec")
	fmt.Println("[Registering Spec]")
	u := &specInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}
	content, err := registerSpecWithInfo(nsId, u)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{
			"message": "Failed to register a Spec"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

func restGetSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("specId")

	content := specInfo{}

	fmt.Println("[Get spec for id]" + id)
	key := genResourceKey(nsId, "spec", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Spec []specInfo `json:"spec"`
	}

	specList := getSpecList(nsId)

	for _, v := range specList {

		key := genResourceKey(nsId, "spec", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		specTmp := specInfo{}
		json.Unmarshal([]byte(keyValue.Value), &specTmp)
		specTmp.Id = v
		content.Spec = append(content.Spec, specTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutSpec(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelSpec(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("specId")

	err := delSpec(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the spec"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The spec has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllSpec(c echo.Context) error {

	nsId := c.Param("nsId")

	specList := getSpecList(nsId)

	for _, v := range specList {
		err := delSpec(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All specs"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All specs has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

/* Optional
func registerSpecWithCspFlavorName(nsId string, u *specReq) (specInfo, error) {

	// TODO: Implement error check logic
	// TODO: Implement spec retrieving logic

	content := specInfo{}

	// TODO: Implement the code below
	// content, err := lookupSpec(u)

	// if 1 { // if lookupSpec(u) succeeds
	// 	content.Id = genUuid()
	// 	...
	// } else { // if lookupSpec(u) fails

	// }
	//

	// Temporary code
	content.Id = genUuid()
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.Os_type = u.Os_type
	content.Num_vCPU = u.Num_vCPU
	content.Num_core = u.Num_core
	content.Mem_GiB = u.Mem_GiB
	content.Storage_GiB = u.Storage_GiB
	content.Description = u.Description

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := genResourceKey(nsId, "spec", content.Id)
	mapA := map[string]string{
		"name":           content.Name,
		"connectionName": content.ConnectionName,
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
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	// register information related with MCIS recommendation
	registerRecommendList(nsId, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	return content, nil
}
*/

func registerSpecWithInfo(nsId string, content *specInfo) (specInfo, error) {

	// TODO: Implement error check logic

	// Temporary code
	content.Id = genUuid()

	/* FYI
	type specInfo struct {
		Id          string `json:"id"`
		Name        string `json:"name"`
		ConnectionName         string `json:"connectionName"`
		Os_type     string `json:"os_type"`
		Num_vCPU    string `json:"num_vCPU"`
		Num_core    string `json:"num_core"`
		Mem_GiB     string `json:"mem_GiB"`
		Storage_GiB string `json:"storage_GiB"`
		Description string `json:"description"`

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
	*/

	// cb-store
	fmt.Println("=========================== PUT registerSpec")
	Key := genResourceKey(nsId, "spec", content.Id)
	mapA := map[string]string{
		"name":           content.Name,
		"connectionName": content.ConnectionName,
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
	registerRecommendList(nsId, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB, content.Id, content.Cost_per_hour)

	return *content, nil
}

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

func delSpec(nsId string, Id string) error {

	fmt.Println("[Delete spec] " + Id)

	key := genResourceKey(nsId, "spec", Id)
	fmt.Println(key)

	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
