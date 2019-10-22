package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/VNetworkHandler.go
/*
type VNetworkReqInfo struct {
     Name string
}

type VNetworkInfo struct {
     Id   string
     Name string
     AddressPrefix string
     Status string

     KeyValueList []KeyValue
}

type VNetworkHandler interface {
	CreateVNetwork(vNetworkReqInfo VNetworkReqInfo) (VNetworkInfo, error)
	ListVNetwork() ([]*VNetworkInfo, error)
	GetVNetwork(vNetworkID string) (VNetworkInfo, error)
	DeleteVNetwork(vNetworkID string) (bool, error)
}
*/

type networkReq struct {
	//Id                string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//CspNetworkId      string `json:"cspNetworkId"`
	CspNetworkName string `json:"cspNetworkName"`
	//CidrBlock         string `json:"cidrBlock"`
	//Region            string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description string `json:"description"`
}

type networkInfo struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspNetworkId   string `json:"cspNetworkId"`
	CspNetworkName string `json:"cspNetworkName"`
	CidrBlock      string `json:"cidrBlock"`
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string     `json:"description"`
	Status       string     `json:"string"`
	KeyValueList []KeyValue `json:"keyValueList"`
}

/* FYI
g.POST("/:nsId/resources/network", restPostNetwork)
g.GET("/:nsId/resources/network/:networkId", restGetNetwork)
g.GET("/:nsId/resources/network", restGetAllNetwork)
g.PUT("/:nsId/resources/network/:networkId", restPutNetwork)
g.DELETE("/:nsId/resources/network/:networkId", restDelNetwork)
g.DELETE("/:nsId/resources/network", restDelAllNetwork)
*/

// MCIS API Proxy: Network
func restPostNetwork(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &networkReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST Network requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating Network]")
		content, _ := createNetwork(nsId, u)
		return c.JSON(http.StatusCreated, content)
		/*
			} else if action == "register" {
				fmt.Println("[Registering Network]")
				content, _ := registerNetwork(nsId, u)
				return c.JSON(http.StatusCreated, content)
		*/
	} else {
		mapA := map[string]string{"message": "You must specify: action=create"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetNetwork(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("networkId")

	content := networkInfo{}

	fmt.Println("[Get network for id]" + id)
	key := genResourceKey(nsId, "network", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllNetwork(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Network []networkInfo `json:"network"`
	}

	networkList := getNetworkList(nsId)

	for _, v := range networkList {

		key := genResourceKey(nsId, "network", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		networkTmp := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &networkTmp)
		networkTmp.Id = v
		content.Network = append(content.Network, networkTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutNetwork(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelNetwork(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("networkId")

	err := delNetwork(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the network"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The network has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllNetwork(c echo.Context) error {

	nsId := c.Param("nsId")

	networkList := getNetworkList(nsId)

	for _, v := range networkList {
		err := delNetwork(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All networks"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All networks has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createNetwork(nsId string, u *networkReq) (networkInfo, error) {

	/* FYI
	type networkReq struct {
		//Id                string `json:"id"`
		ConnectionName    string `json:"connectionName"`
		//CspNetworkId      string `json:"cspNetworkId"`
		CspNetworkName    string `json:"cspNetworkName"`
		//CidrBlock         string `json:"cidrBlock"`
		//Region            string `json:"region"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description       string `json:"description"`
	}
	*/

	//ip := "http://localhost"
	//port := "1024"
	//url := ip + ":" + port + "/vnetwork?connection_name=" + u.ConnectionName

	// ip := "http://5ca45cf78bae720014a963d5.mockapi.io"
	// port := "80"
	// url := ip + ":" + port + "/vnetwork"

	url := SPIDER_URL + "/vnetwork?connection_name=" + u.ConnectionName

	method := "POST"

	payload := strings.NewReader("{ \"Name\": \"" + u.CspNetworkName + "\"}")

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
	type VNetworkInfo struct {
		Id            string
		Name          string
		AddressPrefix string
		Status        string

		KeyValueList []KeyValue
	}
	temp := VNetworkInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI
	type networkInfo struct {
		Id                string `json:"id"`
		ConnectionName    string `json:"connectionName"`
		CspNetworkId      string `json:"cspNetworkId"`
		CspNetworkName    string `json:"cspNetworkName"`
		CidrBlock         string `json:"cidrBlock"`
		//Region            string `json:"region"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description       string `json:"description"`
		Status            string `json:"string"`
		KeyValueList []KeyValue `json:"keyValueList"`
	}
	*/

	content := networkInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	content.CspNetworkId = temp.Id
	content.CspNetworkName = temp.Name // = u.CspNetworkName
	content.CidrBlock = temp.AddressPrefix
	content.Description = u.Description
	content.Status = temp.Status
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT createNetwork")
	Key := genResourceKey(nsId, "network", content.Id)
	/*
		mapA := map[string]string{
			"connectionName": content.ConnectionName,
			"cspNetworkId":   content.CspNetworkId,
			"cspNetworkName": content.CspNetworkName,
			"cidrBlock":      content.CidrBlock,
			//"region":            content.Region,
			//"resourceGroupName": content.ResourceGroupName,
			"description":  content.Description,
			"status":       content.Status,
			"keyValueList": content.KeyValueList}
		Val, _ := json.Marshal(mapA)
	*/
	Val, _ := json.Marshal(content)

	fmt.Println("Key: ", Key)
	fmt.Println("Val: ", Val)
	err3 := store.Put(string(Key), string(Val))
	if err3 != nil {
		cblog.Error(err3)
		return content, err3
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

func getNetworkList(nsId string) []string {

	fmt.Println("[Get networks")
	key := "/ns/" + nsId + "/resources/network"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var networkList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		networkList = append(networkList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/network/"))
		//}
	}
	for _, v := range networkList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return networkList

}

func delNetwork(nsId string, Id string) error {

	fmt.Println("[Delete network] " + Id)

	key := genResourceKey(nsId, "network", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := networkInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspNetworkId: " + temp.CspNetworkId)

	//url := "https://testapi.io/api/jihoon-seo/vnetwork/" + temp.CspNetworkId + "?connection_name=" + temp.ConnectionName // for CB-Spider
	url := SPIDER_URL + "/vnetwork?connection_name=" + temp.ConnectionName // for testapi.io
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

	// delete mcis info
	cbStoreDeleteErr := store.Delete(key)
	if cbStoreDeleteErr != nil {
		cblog.Error(cbStoreDeleteErr)
		return cbStoreDeleteErr
	}

	return nil
}
