package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/labstack/echo"
)

// 2020-04-09 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

type SpiderVPCReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVPCInfo
}

/*
type SpiderVPCReqInfo struct { // Spider
	Name      string
	IPv4_CIDR string
	//SubnetInfoList []SpiderSubnetReqInfo
	SubnetInfoList []SpiderSubnetInfo
}
*/

/*
type SpiderSubnetReqInfo struct { // Spider
	Name      string
	IPv4_CIDR string

	KeyValueList []common.KeyValue
}
*/

type SpiderVPCInfo struct { // Spider
	// Fields for request
	Name string

	// Fields for both request and response
	IPv4_CIDR      string
	SubnetInfoList []SpiderSubnetInfo

	// Fields for response
	IId          common.IID // {NameId, SystemId}
	KeyValueList []common.KeyValue
}

type SpiderSubnetInfo struct { // Spider
	// Fields for request
	Name string

	// Fields for both request and response
	IPv4_CIDR    string
	KeyValueList []common.KeyValue

	// Fields for response
	IId common.IID // {NameId, SystemId}
}

/*
type TbVNetReq struct { // Tumblebug
	Name              string                `json:"name"`
	ConnectionName    string                `json:"connectionName"`
	CidrBlock         string                `json:"cidrBlock"`
	SubnetReqInfoList []SpiderSubnetReqInfo `json:"subnetReqInfoList"`
	//Region            string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description string `json:"description"`
}
*/

type TbVNetInfo struct { // Tumblebug
	// Fields for both request and response
	Name           string             `json:"name"`
	ConnectionName string             `json:"connectionName"`
	CidrBlock      string             `json:"cidrBlock"`
	SubnetInfoList []SpiderSubnetInfo `json:"subnetInfoList"`
	Description    string             `json:"description"`

	// Additional fields for response
	Id           string            `json:"id"`
	CspVNetId    string            `json:"cspVNetId"`
	CspVNetName  string            `json:"cspVNetName"`
	Status       string            `json:"status"`
	KeyValueList []common.KeyValue `json:"keyValueList"`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

// MCIS API Proxy: VNet
func RestPostVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &TbVNetInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST VNet requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating VNet]")
			content, _ := CreateVNet(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else if action == "register" {
			fmt.Println("[Registering VNet]")
			content, _ := registerVNet(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST VNet")
	fmt.Println("[Creating VNet]")
	//content, responseCode, body, err := CreateVNet(nsId, u)
	content, err := CreateVNet(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a vNet"}
		*/
		//return c.JSONBlob(responseCode, body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "vNet"

	id := c.Param("vNetId")

	/*
		content := TbVNetInfo{}

		fmt.Println("[Get vNet for id]" + id)
		key := common.GenResourceKey(nsId, "vNet", id)
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Failed to find the vNet with given ID."}
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

func RestGetAllVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "vNet"

	var content struct {
		VNet []TbVNetInfo `json:"vNet"`
	}

	/*
		vNetList := ListResourceId(nsId, "vNet")

		for _, v := range vNetList {

			key := common.GenResourceKey(nsId, "vNet", v)
			fmt.Println(key)
			keyValue, _ := store.Get(key)
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			vNetTmp := TbVNetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &vNetTmp)
			vNetTmp.Id = v
			content.VNet = append(content.VNet, vNetTmp)

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
	content.VNet = resourceList.([]TbVNetInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}

func RestPutVNet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelVNet(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "vNet"
	id := c.Param("vNetId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delVNet(nsId, id, forceFlag)

	err := DelResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the vNet"}
		//return c.JSONBlob(responseCode, body)
		return c.JSON(http.StatusFailedDependency, err)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllVNet(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "vNet"
	forceFlag := c.QueryParam("force")

	/*
		vNetList := ListResourceId(nsId, "vNet")

		if len(vNetList) == 0 {
			mapA := map[string]string{"message": "There is no vNet element in this namespace."}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			for _, v := range vNetList {
				//responseCode, body, err := delVNet(nsId, v, forceFlag)

				responseCode, body, err := DelResource(nsId, "vNet", v, forceFlag)
				if err != nil {
					cblog.Error(err)
					//mapA := map[string]string{"message": "Failed to delete the vNet"}
					return c.JSONBlob(responseCode, body)
				}

			}

			mapA := map[string]string{"message": "All vNets has been deleted"}
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

//func CreateVNet(nsId string, u *TbVNetReq) (TbVNetInfo, int, []byte, error) {
func CreateVNet(nsId string, u *TbVNetInfo) (TbVNetInfo, error) {
	check, _ := CheckResource(nsId, "vNet", u.Name)

	if check {
		temp := TbVNetInfo{}
		err := fmt.Errorf("The vNet " + u.Name + " already exists.")
		return temp, err
	}

	//url := common.SPIDER_URL + "/vpc?connection_name=" + u.ConnectionName
	url := common.SPIDER_URL + "/vpc"

	method := "POST"

	tempReq := SpiderVPCReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = u.Name
	tempReq.ReqInfo.IPv4_CIDR = u.CidrBlock
	tempReq.ReqInfo.SubnetInfoList = u.SubnetInfoList
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	fmt.Println("payload: " + string(payload)) // for debug

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		cblog.Error(err)
		content := TbVNetInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	//fmt.Println("res.Body: " + string(res.Body)) // for debug

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := TbVNetInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		content := TbVNetInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	temp := SpiderVPCInfo{} // Spider
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	content := TbVNetInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspVNetId = temp.IId.SystemId
	content.CspVNetName = temp.IId.NameId
	content.CidrBlock = temp.IPv4_CIDR
	content.SubnetInfoList = temp.SubnetInfoList
	content.Description = u.Description
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT CreateVNet")
	Key := common.GenResourceKey(nsId, "vNet", content.Id)
	/*
		mapA := map[string]string{
			"connectionName": content.ConnectionName,
			"cspVNetId":   content.CspVNetId,
			"cspVNetName": content.CspVNetName,
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
		//return content, res.StatusCode, body, err3
		return content, err3
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	//return content, res.StatusCode, body, nil
	return content, nil
}
