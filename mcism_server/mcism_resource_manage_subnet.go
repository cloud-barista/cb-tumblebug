package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type subnetReq struct {
	Id                 string `json:"id"`
	Csp                string `json:"csp"`
	CspSubnetId        string `json:"cspSubnetId"`
	CspSubnetName      string `json:"cspSubnetName"`
	VirtualNetworkId   string `json:"virtualNetworkId"`
	VirtualNetworkName string `json:"virtualNetworkName"`
	CidrBlock          string `json:"cidrBlock"`
	Region             string `json:"region"`
	ResourceGroupName  string `json:"resourceGroupName"`
	Description        string `json:"description"`
}

type subnetInfo struct {
	Id                 string `json:"id"`
	Csp                string `json:"csp"`
	CspSubnetId        string `json:"cspSubnetId"`
	CspSubnetName      string `json:"cspSubnetName"`
	VirtualNetworkId   string `json:"virtualNetworkId"`
	VirtualNetworkName string `json:"virtualNetworkName"`
	CidrBlock          string `json:"cidrBlock"`
	Region             string `json:"region"`
	ResourceGroupName  string `json:"resourceGroupName"`
	Description        string `json:"description"`
}

/* FYI
g.POST("/:nsId/resources/subnet", restPostSubnet)
g.GET("/:nsId/resources/subnet/:subnetId", restGetSubnet)
g.GET("/:nsId/resources/subnet", restGetAllSubnet)
g.PUT("/:nsId/resources/subnet/:subnetId", restPutSubnet)
g.DELETE("/:nsId/resources/subnet/:subnetId", restDelSubnet)
g.DELETE("/:nsId/resources/subnet", restDelAllSubnet)
*/

// MCIS API Proxy: Subnet
func restPostSubnet(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &subnetReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST Subnet requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating Subnet]")
		createSubnet(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else if action == "register" {
		fmt.Println("[Registering Subnet]")
		registerSubnet(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetSubnet(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("subnetId")

	content := subnetInfo{}
	/*
		var content struct {
			Id                 string `json:"id"`
			Csp                string `json:"csp"`
			CspSubnetId        string `json:"cspSubnetId"`
			CspSubnetName      string `json:"cspSubnetName"`
			VirtualNetworkId   string `json:"virtualNetworkId"`
			VirtualNetworkName string `json:"virtualNetworkName"`
			CidrBlock          string `json:"cidrBlock"`
			Region             string `json:"region"`
			ResourceGroupName  string `json:"resourceGroupName"`
			Description        string `json:"description"`
		}
	*/

	fmt.Println("[Get subnet for id]" + id)
	key := genResourceKey(nsId, "subnet", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllSubnet(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Subnet []subnetInfo `json:"subnet"`
	}

	subnetList := getSubnetList(nsId)

	for _, v := range subnetList {

		key := genResourceKey(nsId, "subnet", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		subnetTmp := subnetInfo{}
		json.Unmarshal([]byte(keyValue.Value), &subnetTmp)
		subnetTmp.Id = v
		content.Subnet = append(content.Subnet, subnetTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutSubnet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelSubnet(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("subnetId")

	err := delSubnet(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the subnet"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The subnet has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllSubnet(c echo.Context) error {

	nsId := c.Param("nsId")

	subnetList := getSubnetList(nsId)

	for _, v := range subnetList {
		err := delSubnet(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All subnets"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All subnets has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSubnet(nsId string, u *subnetReq) {

	u.Id = genUuid()

	/* FYI
	type subnetReq struct {
		Id                 string `json:"id"`
		Csp                string `json:"csp"`
		CspSubnetId        string `json:"cspSubnetId"`
		CspSubnetName      string `json:"cspSubnetName"`
		VirtualNetworkId   string `json:"virtualNetworkId"`
		VirtualNetworkName string `json:"virtualNetworkName"`
		CidrBlock          string `json:"cidrBlock"`
		Region             string `json:"region"`
		ResourceGroupName  string `json:"resourceGroupName"`
		Description        string `json:"description"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createSubnet")
	Key := genResourceKey(nsId, "subnet", u.Id)
	mapA := map[string]string{"csp": u.Csp, "cspSubnetId": u.CspSubnetId, "cspSubnetName": u.CspSubnetName,
		"virtualNetworkId": u.VirtualNetworkId, "virtualNetworkName": u.VirtualNetworkName, "cidrBlock": u.CidrBlock,
		"region": u.Region, "resourceGroupName": u.ResourceGroupName, "description": u.Description}
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

func registerSubnet(nsId string, u *subnetReq) {

	u.Id = genUuid()

	// TODO here: implement the logic
	// - Fetch the subnet info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerSubnet")
	Key := genResourceKey(nsId, "subnet", u.Id)
	mapA := map[string]string{"csp": u.Csp, "cspSubnetId": u.CspSubnetId, "cspSubnetName": u.CspSubnetName,
		"virtualNetworkId": u.VirtualNetworkId, "virtualNetworkName": u.VirtualNetworkName, "cidrBlock": u.CidrBlock,
		"region": u.Region, "resourceGroupName": u.ResourceGroupName, "description": u.Description}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func getSubnetList(nsId string) []string {

	fmt.Println("[Get subnets")
	key := "/ns/" + nsId + "/resources/subnet"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var subnetList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		subnetList = append(subnetList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/subnet/"))
		//}
	}
	for _, v := range subnetList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return subnetList

}

func delSubnet(nsId string, Id string) error {

	fmt.Println("[Delete subnet] " + Id)

	key := genResourceKey(nsId, "subnet", Id)
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
