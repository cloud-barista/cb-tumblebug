package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type securityGroupReq struct {
	//Id                 string `json:"id"`
	Name               string `json:"name"`
	ConnectionName     string `json:"connectionName"`
	VirtualNetworkId   string `json:"virtualNetworkId"`
	CspSecurityGroupId string `json:"cspSecurityGroupId"`
	ResourceGroupName  string `json:"resourceGroupName"`
	Description        string `json:"description"`
}

type securityGroupInfo struct {
	Id                 string `json:"id"`
	Name               string `json:"name"`
	ConnectionName     string `json:"connectionName"`
	VirtualNetworkId   string `json:"virtualNetworkId"`
	CspSecurityGroupId string `json:"cspSecurityGroupId"`
	ResourceGroupName  string `json:"resourceGroupName"`
	Description        string `json:"description"`
}

/* FYI
g.POST("/resources/securityGroup", restPostSecurityGroup)
g.GET("/resources/securityGroup/:securityGroupId", restGetSecurityGroup)
g.GET("/resources/securityGroup", restGetAllSecurityGroup)
g.PUT("/resources/securityGroup/:securityGroupId", restPutSecurityGroup)
g.DELETE("/resources/securityGroup/:securityGroupId", restDelSecurityGroup)
g.DELETE("/resources/securityGroup", restDelAllSecurityGroup)
*/

// MCIS API Proxy: SecurityGroup
func restPostSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &securityGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST SecurityGroup requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating SecurityGroup]")
		content, _ := createSecurityGroup(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else if action == "register" {
		fmt.Println("[Registering SecurityGroup]")
		content, _ := registerSecurityGroup(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("securityGroupId")

	content := securityGroupInfo{}
	/*
		var content struct {
			Id                 string `json:"id"`
			Name               string `json:"name"`
			ConnectionName                string `json:"connectionName"`
			VirtualNetworkId   string `json:"virtualNetworkId"`
			CspSecurityGroupId string `json:"cspSecurityGroupId"`
			ResourceGroupName  string `json:"resourceGroupName"`
			Description        string `json:"description"`
		}
	*/

	fmt.Println("[Get securityGroup for id]" + id)
	key := genResourceKey(nsId, "securityGroup", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		SecurityGroup []securityGroupInfo `json:"securityGroup"`
	}

	securityGroupList := getSecurityGroupList(nsId)

	for _, v := range securityGroupList {

		key := genResourceKey(nsId, "securityGroup", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		securityGroupTmp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &securityGroupTmp)
		securityGroupTmp.Id = v
		content.SecurityGroup = append(content.SecurityGroup, securityGroupTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("securityGroupId")

	err := delSecurityGroup(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the securityGroup"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The securityGroup has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	securityGroupList := getSecurityGroupList(nsId)

	for _, v := range securityGroupList {
		err := delSecurityGroup(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All securityGroups"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All securityGroups has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSecurityGroup(nsId string, u *securityGroupReq) (securityGroupInfo, error) {

	/* FYI
	type securityGroupInfo struct {
		Id                 string `json:"id"`
		Name               string `json:"name"`
		ConnectionName                string `json:"connectionName"`
		VirtualNetworkId   string `json:"virtualNetworkId"`
		CspSecurityGroupId string `json:"cspSecurityGroupId"`
		ResourceGroupName  string `json:"resourceGroupName"`
		Description        string `json:"description"`
	}
	*/

	content := securityGroupInfo{}
	content.Id = genUuid()
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.VirtualNetworkId = u.VirtualNetworkId
	content.CspSecurityGroupId = u.CspSecurityGroupId
	content.ResourceGroupName = u.ResourceGroupName
	content.Description = u.Description

	// cb-store
	fmt.Println("=========================== PUT createSecurityGroup")
	Key := genResourceKey(nsId, "securityGroup", content.Id)
	mapA := map[string]string{
		"name":               content.Name,
		"connectionName":     content.ConnectionName,
		"virtualNetworkId":   content.VirtualNetworkId,
		"cspSecurityGroupId": content.CspSecurityGroupId,
		"resourceGroupName":  content.ResourceGroupName,
		"description":        content.Description}
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

func registerSecurityGroup(nsId string, u *securityGroupReq) (securityGroupInfo, error) {

	content := securityGroupInfo{}
	content.Id = genUuid()
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.VirtualNetworkId = u.VirtualNetworkId
	content.CspSecurityGroupId = u.CspSecurityGroupId
	content.ResourceGroupName = u.ResourceGroupName
	content.Description = u.Description

	// TODO here: implement the logic
	// - Fetch the securityGroup info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerSecurityGroup")
	Key := genResourceKey(nsId, "securityGroup", content.Id)
	mapA := map[string]string{
		"name":               content.Name,
		"connectionName":     content.ConnectionName,
		"virtualNetworkId":   content.VirtualNetworkId,
		"cspSecurityGroupId": content.CspSecurityGroupId,
		"resourceGroupName":  content.ResourceGroupName,
		"description":        content.Description}
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

func getSecurityGroupList(nsId string) []string {

	fmt.Println("[Get securityGroups")
	key := "/ns/" + nsId + "/resources/securityGroup"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var securityGroupList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		securityGroupList = append(securityGroupList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/securityGroup/"))
		//}
	}
	for _, v := range securityGroupList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return securityGroupList

}

func delSecurityGroup(nsId string, Id string) error {

	fmt.Println("[Delete securityGroup] " + Id)

	key := genResourceKey(nsId, "securityGroup", Id)
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
