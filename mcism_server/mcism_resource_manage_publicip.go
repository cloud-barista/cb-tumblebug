package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type publicIpReq struct {
	//Id                string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//CspPublicIpId     string `json:"cspPublicIpId"`
	CspPublicIpName string `json:"cspPublicIpName"`
	//PublicIp          string `json:"publicIp"`
	//OwnedVmId         string `json:"ownedVmId"`
	ResourceGroupName string `json:"resourceGroupName"`
	//Description       string `json:"description"`
}

type publicIpInfo struct {
	Id                string `json:"id"`
	ConnectionName    string `json:"connectionName"`
	CspPublicIpId     string `json:"cspPublicIpId"`
	CspPublicIpName   string `json:"cspPublicIpName"`
	PublicIp          string `json:"publicIp"`
	OwnedVmId         string `json:"ownedVmId"`
	ResourceGroupName string `json:"resourceGroupName"`
	Description       string `json:"description"`
	Status            string `json:"string"`
}

/* FYI
g.POST("/:nsId/resources/publicIp", restPostPublicIp)
g.GET("/:nsId/resources/publicIp/:publicIpId", restGetPublicIp)
g.GET("/:nsId/resources/publicIp", restGetAllPublicIp)
g.PUT("/:nsId/resources/publicIp/:publicIpId", restPutPublicIp)
g.DELETE("/:nsId/resources/publicIp/:publicIpId", restDelPublicIp)
g.DELETE("/:nsId/resources/publicIp", restDelAllPublicIp)
*/

// MCIS API Proxy: PublicIp
func restPostPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &publicIpReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST PublicIp requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating PublicIp]")
		content, _ := createPublicIp(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else if action == "register" {
		fmt.Println("[Registering PublicIp]")
		content, _ := registerPublicIp(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("publicIpId")

	content := publicIpInfo{}
	/*
		var content struct {
			Id                string `json:"id"`
			ConnectionName    string `json:"connectionName"`
			CspPublicIpId      string `json:"cspPublicIpId"`
			CspPublicIpName    string `json:"cspPublicIpName"`
			CidrBlock         string `json:"cidrBlock"`
			Region            string `json:"region"`
			ResourceGroupName string `json:"resourceGroupName"`
			Description       string `json:"description"`
			Status            string `json:"string"`
		}
	*/

	fmt.Println("[Get publicIp for id]" + id)
	key := genResourceKey(nsId, "publicIp", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		PublicIp []publicIpInfo `json:"publicIp"`
	}

	publicIpList := getPublicIpList(nsId)

	for _, v := range publicIpList {

		key := genResourceKey(nsId, "publicIp", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		publicIpTmp := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &publicIpTmp)
		publicIpTmp.Id = v
		content.PublicIp = append(content.PublicIp, publicIpTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutPublicIp(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("publicIpId")

	err := delPublicIp(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the publicIp"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The publicIp has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	publicIpList := getPublicIpList(nsId)

	for _, v := range publicIpList {
		err := delPublicIp(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All publicIps"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All publicIps has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createPublicIp(nsId string, u *publicIpReq) (publicIpInfo, error) {

	/* FYI
	type publicIpReq struct {
		//Id                string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//CspPublicIpId     string `json:"cspPublicIpId"`
		CspPublicIpName string `json:"cspPublicIpName"`
		//PublicIp          string `json:"publicIp"`
		//OwnedVmId         string `json:"ownedVmId"`
		ResourceGroupName string `json:"resourceGroupName"`
		//Description       string `json:"description"`
	}
	*/

	content := publicIpInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	//content.CspPublicIpId = u.CspPublicIpId
	content.CspPublicIpName = u.CspPublicIpName
	//content.PublicIp = u.PublicIp
	//content.OwnedVmId = u.OwnedVmId
	content.ResourceGroupName = u.ResourceGroupName
	//content.Description = u.Description

	/* FYI
	type publicIpInfo struct {
		Id                string `json:"id"`
		ConnectionName    string `json:"connectionName"`
		CspPublicIpId     string `json:"cspPublicIpId"`
		CspPublicIpName   string `json:"cspPublicIpName"`
		PublicIp          string `json:"publicIp"`
		OwnedVmId         string `json:"ownedVmId"`
		ResourceGroupName string `json:"resourceGroupName"`
		Description       string `json:"description"`
		Status            string `json:"string"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createPublicIp")
	Key := genResourceKey(nsId, "publicIp", content.Id)
	mapA := map[string]string{
		"connectionName":    content.ConnectionName,
		"cspPublicIpId":     content.CspPublicIpId,
		"cspPublicIpName":   content.CspPublicIpName,
		"publicIp":          content.PublicIp,
		"ownedVmId":         content.OwnedVmId,
		"resourceGroupName": content.ResourceGroupName,
		"description":       content.Description,
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

func registerPublicIp(nsId string, u *publicIpReq) (publicIpInfo, error) {

	content := publicIpInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	//content.CspPublicIpId = u.CspPublicIpId
	content.CspPublicIpName = u.CspPublicIpName
	//content.PublicIp = u.PublicIp
	//content.OwnedVmId = u.OwnedVmId
	content.ResourceGroupName = u.ResourceGroupName
	//content.Description = u.Description

	// TODO here: implement the logic
	// - Fetch the publicIp info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerPublicIp")
	Key := genResourceKey(nsId, "publicIp", content.Id)
	mapA := map[string]string{
		"connectionName":    content.ConnectionName,
		"cspPublicIpId":     content.CspPublicIpId,
		"cspPublicIpName":   content.CspPublicIpName,
		"publicIp":          content.PublicIp,
		"ownedVmId":         content.OwnedVmId,
		"resourceGroupName": content.ResourceGroupName,
		"description":       content.Description,
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

func getPublicIpList(nsId string) []string {

	fmt.Println("[Get publicIps")
	key := "/ns/" + nsId + "/resources/publicIp"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var publicIpList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		publicIpList = append(publicIpList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/publicIp/"))
		//}
	}
	for _, v := range publicIpList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return publicIpList

}

func delPublicIp(nsId string, Id string) error {

	fmt.Println("[Delete publicIp] " + Id)

	key := genResourceKey(nsId, "publicIp", Id)
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
