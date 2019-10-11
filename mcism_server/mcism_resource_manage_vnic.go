package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type vNicReq struct {
	//Id                string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//CspVNicId     string `json:"cspVNicId"`
	CspVNicName       string `json:"cspVNicName"`
	CspVNetName       string `json:"cspVNetName"`
	PublicIpId        string `json:"publicIpId"`
	ResourceGroupName string `json:"resourceGroupName"`
	//Description       string `json:"description"`
}

type vNicInfo struct {
	Id                string `json:"id"`
	ConnectionName    string `json:"connectionName"`
	CspVNicId         string `json:"cspVNicId"`
	CspVNicName       string `json:"cspVNicName"`
	CspVNetName       string `json:"cspVNetName"`
	PublicIpId        string `json:"publicIpId"`
	ResourceGroupName string `json:"resourceGroupName"`
	Description       string `json:"description"`
	PublicIp          string `json:"publicIp"`
	MacAddress        string `json:"macAddress"`
	OwnedVmId         string `json:"ownedVmId"`
	Status            string `json:"string"`
}

/* FYI
g.POST("/:nsId/resources/vNic", restPostVNic)
g.GET("/:nsId/resources/vNic/:vNicId", restGetVNic)
g.GET("/:nsId/resources/vNic", restGetAllVNic)
g.PUT("/:nsId/resources/vNic/:vNicId", restPutVNic)
g.DELETE("/:nsId/resources/vNic/:vNicId", restDelVNic)
g.DELETE("/:nsId/resources/vNic", restDelAllVNic)
*/

// MCIS API Proxy: VNic
func restPostVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &vNicReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST VNic requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating VNic]")
		content, _ := createVNic(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else if action == "register" {
		fmt.Println("[Registering VNic]")
		content, _ := registerVNic(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("vNicId")

	content := vNicInfo{}
	/*
		var content struct {
			Id                string `json:"id"`
			ConnectionName    string `json:"connectionName"`
			CspVNicId         string `json:"cspVNicId"`
			CspVNicName       string `json:"cspVNicName"`
			CspVNetName       string `json:"cspVNetName"`
			PublicIpId        string `json:"publicIpId"`
			ResourceGroupName string `json:"resourceGroupName"`
			Description       string `json:"description"`
			PublicIp          string `json:"publicIp"`
			MacAddress        string `json:"macAddress"`
			OwnedVmId         string `json:"ownedVmId"`
			Status            string `json:"string"`
		}
	*/

	fmt.Println("[Get vNic for id]" + id)
	key := genResourceKey(nsId, "vNic", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		VNic []vNicInfo `json:"vNic"`
	}

	vNicList := getVNicList(nsId)

	for _, v := range vNicList {

		key := genResourceKey(nsId, "vNic", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		vNicTmp := vNicInfo{}
		json.Unmarshal([]byte(keyValue.Value), &vNicTmp)
		vNicTmp.Id = v
		content.VNic = append(content.VNic, vNicTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutVNic(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelVNic(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("vNicId")

	err := delVNic(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the vNic"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The vNic has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	vNicList := getVNicList(nsId)

	for _, v := range vNicList {
		err := delVNic(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All vNics"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All vNics has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createVNic(nsId string, u *vNicReq) (vNicInfo, error) {

	/* FYI
	type vNicReq struct {
		//Id                string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//CspVNicId     string `json:"cspVNicId"`
		CspVNicName       string `json:"cspVNicName"`
		CspVNetName       string `json:"cspVNetName"`
		PublicIpId        string `json:"publicIpId"`
		ResourceGroupName string `json:"resourceGroupName"`
		//Description       string `json:"description"`
	}
	*/

	content := vNicInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	//content.CspVNicId = u.CspVNicId
	content.CspVNicName = u.CspVNicName
	content.CspVNetName = u.CspVNetName
	content.PublicIpId = u.PublicIpId
	content.ResourceGroupName = u.ResourceGroupName
	//content.Description = u.Description

	/* FYI
	type vNicInfo struct {
		Id                string `json:"id"`
		ConnectionName    string `json:"connectionName"`
		CspVNicId         string `json:"cspVNicId"`
		CspVNicName       string `json:"cspVNicName"`
		CspVNetName       string `json:"cspVNetName"`
		PublicIpId        string `json:"publicIpId"`
		ResourceGroupName string `json:"resourceGroupName"`
		Description       string `json:"description"`
		PublicIp          string `json:"publicIp"`
		MacAddress        string `json:"macAddress"`
		OwnedVmId         string `json:"ownedVmId"`
		Status            string `json:"string"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createVNic")
	Key := genResourceKey(nsId, "vNic", content.Id)
	mapA := map[string]string{
		"connectionName":    content.ConnectionName,
		"cspVNicId":         content.CspVNicId,
		"cspVNicName":       content.CspVNicName,
		"cspVNetName":       content.CspVNetName,
		"publicIpId":        content.PublicIpId,
		"resourceGroupName": content.ResourceGroupName,
		"description":       content.Description,
		"publicIp":          content.PublicIp,
		"macAddress":        content.MacAddress,
		"ownedVmId":         content.OwnedVmId,
		"status":            content.Status}
	Val, _ := json.Marshal(mapA)
	fmt.Println("Key: ", Key)
	fmt.Println("Val: ", Val)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

func registerVNic(nsId string, u *vNicReq) (vNicInfo, error) {

	content := vNicInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	//content.CspVNicId = u.CspVNicId
	content.CspVNicName = u.CspVNicName
	content.CspVNetName = u.CspVNetName
	content.PublicIpId = u.PublicIpId
	content.ResourceGroupName = u.ResourceGroupName
	//content.Description = u.Description

	// TODO here: implement the logic
	// - Fetch the vNic info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerVNic")
	Key := genResourceKey(nsId, "vNic", content.Id)
	mapA := map[string]string{
		"connectionName":    content.ConnectionName,
		"cspVNicId":         content.CspVNicId,
		"cspVNicName":       content.CspVNicName,
		"cspVNetName":       content.CspVNetName,
		"publicIpId":        content.PublicIpId,
		"resourceGroupName": content.ResourceGroupName,
		"description":       content.Description,
		"publicIp":          content.PublicIp,
		"macAddress":        content.MacAddress,
		"ownedVmId":         content.OwnedVmId,
		"status":            content.Status}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

func getVNicList(nsId string) []string {

	fmt.Println("[Get vNics")
	key := "/ns/" + nsId + "/resources/vNic"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var vNicList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		vNicList = append(vNicList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/vNic/"))
		//}
	}
	for _, v := range vNicList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return vNicList

}

func delVNic(nsId string, Id string) error {

	fmt.Println("[Delete vNic] " + Id)

	key := genResourceKey(nsId, "vNic", Id)
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
