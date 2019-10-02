package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type networkReq struct {
	Id                string `json:"id"`
	Csp               string `json:"csp"`
	CspNetworkId      string `json:"cspNetworkId"`
	CspNetworkName    string `json:"cspNetworkName"`
	CidrBlock         string `json:"cidrBlock"`
	Region            string `json:"region"`
	ResourceGroupName string `json:"resourceGroupName"`
	Description       string `json:"description"`
}

type networkInfo struct {
	Id                string `json:"id"`
	Csp               string `json:"csp"`
	CspNetworkId      string `json:"cspNetworkId"`
	CspNetworkName    string `json:"cspNetworkName"`
	CidrBlock         string `json:"cidrBlock"`
	Region            string `json:"region"`
	ResourceGroupName string `json:"resourceGroupName"`
	Description       string `json:"description"`
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
		createNetwork(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else if action == "register" {
		fmt.Println("[Registering Network]")
		registerNetwork(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetNetwork(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("networkId")

	content := networkInfo{}
	/*
		var content struct {
			Id                string `json:"id"`
			Csp               string `json:"csp"`
			CspNetworkId      string `json:"cspNetworkId"`
			CspNetworkName    string `json:"cspNetworkName"`
			CidrBlock         string `json:"cidrBlock"`
			Region            string `json:"region"`
			ResourceGroupName string `json:"resourceGroupName"`
			Description       string `json:"description"`
		}
	*/

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

func createNetwork(nsId string, u *networkReq) {

	u.Id = genUuid()

	/* FYI
	type networkReq struct {
		Id                string `json:"id"`
		Csp               string `json:"csp"`
		CspNetworkId      string `json:"cspNetworkId"`
		CspNetworkName    string `json:"cspNetworkName"`
		CidrBlock         string `json:"cidrBlock"`
		Region            string `json:"region"`
		ResourceGroupName string `json:"resourceGroupName"`
		Description       string `json:"description"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createNetwork")
	Key := genResourceKey(nsId, "network", u.Id)
	mapA := map[string]string{"csp": u.Csp, "cspNetworkId": u.CspNetworkId, "cspNetworkName": u.CspNetworkName,
		"cidrBlock": u.CidrBlock,
		"region":    u.Region, "resourceGroupName": u.ResourceGroupName, "description": u.Description}
	Val, _ := json.Marshal(mapA)
	fmt.Println("Key: ", Key)
	fmt.Println("Val: ", Val)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func registerNetwork(nsId string, u *networkReq) {

	u.Id = genUuid()

	// TODO here: implement the logic
	// - Fetch the network info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerNetwork")
	Key := genResourceKey(nsId, "network", u.Id)
	mapA := map[string]string{"csp": u.Csp, "cspNetworkId": u.CspNetworkId, "cspNetworkName": u.CspNetworkName,
		"cidrBlock": u.CidrBlock,
		"region":    u.Region, "resourceGroupName": u.ResourceGroupName, "description": u.Description}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

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
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
