package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/SecurityHandler.go
/* FYI
type SecurityReqInfo struct {
	Name          string
	SecurityRules *[]SecurityRuleInfo
}

type SecurityRuleInfo struct {
	FromPort   string
	ToPort     string
	IPProtocol string
	Direction  string
}

type SecurityInfo struct {
	Id            string
	Name          string
	SecurityRules *[]SecurityRuleInfo

	KeyValueList []KeyValue
}
*/

type securityGroupReq struct {
	//Id                 string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//VirtualNetworkId     string `json:"virtualNetworkId"`
	//CspSecurityGroupId   string `json:"cspSecurityGroupId"`
	CspSecurityGroupName string `json:"cspSecurityGroupName"`
	//ResourceGroupName    string `json:"resourceGroupName"`
	Description string `json:"description"`
}

type securityGroupInfo struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//VirtualNetworkId   string `json:"virtualNetworkId"`
	CspSecurityGroupId   string `json:"cspSecurityGroupId"`
	CspSecurityGroupName string `json:"cspSecurityGroupName"`
	//ResourceGroupName  string `json:"resourceGroupName"`
	Description string `json:"description"`
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
		/*
			} else if action == "register" {
				fmt.Println("[Registering SecurityGroup]")
				content, _ := registerSecurityGroup(nsId, u)
				return c.JSON(http.StatusCreated, content)
		*/
	} else {
		mapA := map[string]string{"message": "You must specify: action=create"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("securityGroupId")

	content := securityGroupInfo{}

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
		//Id                 string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//VirtualNetworkId     string `json:"virtualNetworkId"`
		//CspSecurityGroupId   string `json:"cspSecurityGroupId"`
		CspSecurityGroupName string `json:"cspSecurityGroupName"`
		//ResourceGroupName    string `json:"resourceGroupName"`
		Description string `json:"description"`
	}
	*/

	url := "https://testapi.io/api/jihoon-seo/securitygroup?connection_name=" + u.ConnectionName

	method := "POST"

	payload := strings.NewReader("{ \"Name\": \"" + u.CspSecurityGroupName + "\"}")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	// jhseo 191016
	//var s = new(imageInfo)
	//s := imageInfo{}
	type SecurityRuleInfo struct {
		FromPort   string
		ToPort     string
		IPProtocol string
		Direction  string
	}

	type SecurityInfo struct {
		Id            string
		Name          string
		SecurityRules *[]SecurityRuleInfo

		KeyValueList []KeyValue
	}
	temp := SecurityInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI
	type securityGroupInfo struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//VirtualNetworkId   string `json:"virtualNetworkId"`
		CspSecurityGroupId   string `json:"cspSecurityGroupId"`
		CspSecurityGroupName string `json:"cspSecurityGroupName"`
		//ResourceGroupName  string `json:"resourceGroupName"`
		Description string `json:"description"`
	}
	*/

	content := securityGroupInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	content.CspSecurityGroupId = temp.Id
	content.CspSecurityGroupName = temp.Name // = u.CspSecurityGroupName
	content.Description = u.Description

	// cb-store
	fmt.Println("=========================== PUT createSecurityGroup")
	Key := genResourceKey(nsId, "securityGroup", content.Id)
	mapA := map[string]string{
		"connectionName": content.ConnectionName,
		//"virtualNetworkId":   content.VirtualNetworkId,
		"cspSecurityGroupId":   content.CspSecurityGroupId,
		"cspSecurityGroupName": content.CspSecurityGroupName,
		//"resourceGroupName":  content.ResourceGroupName,
		"description": content.Description}
	Val, _ := json.Marshal(mapA)
	cbStorePutErr := store.Put(string(Key), string(Val))
	if cbStorePutErr != nil {
		cblog.Error(cbStorePutErr)
		return content, cbStorePutErr
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
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := securityGroupInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspSecurityGroupId: " + temp.CspSecurityGroupId)

	//url := "https://testapi.io/api/jihoon-seo/securitygroup/" + temp.CspSecurityGroupId + "?connection_name=" + temp.ConnectionName // for CB-Spider
	url := "https://testapi.io/api/jihoon-seo/securitygroup?connection_name=" + temp.ConnectionName // for testapi.io
	fmt.Println("url: " + url)

	method := "DELETE"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
	}

	res, err := client.Do(req)
	fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	// delete securityGroup info
	cbStoreDeleteErr := store.Delete(key)
	if cbStoreDeleteErr != nil {
		cblog.Error(cbStoreDeleteErr)
		return cbStoreDeleteErr
	}

	return nil
}
