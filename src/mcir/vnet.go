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

type SpiderVPCReqInfo struct { // Spider
	ConnectionName string
	ReqInfo        VPCReqInfo
}

type VPCReqInfo struct { // Spider
	IId            common.IID // {NameId, SystemId}
	IPv4_CIDR      string
	SubnetInfoList []SubnetInfo
}

type VPCInfo struct { // Spider
	IId            common.IID // {NameId, SystemId}
	IPv4_CIDR      string
	SubnetInfoList []SubnetInfo

	KeyValueList []common.KeyValue
}

type SubnetInfo struct { // Spider
	IId       common.IID // {NameId, SystemId}
	IPv4_CIDR string

	KeyValueList []common.KeyValue
}

type vNetReq struct { // Tumblebug
	//Id                string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	//CspVNetId      string `json:"cspVNetId"`
	CspVNetName    string       `json:"cspVNetName"`
	CidrBlock      string       `json:"cidrBlock"`
	SubnetInfoList []SubnetInfo `json:"subnetInfoList"`
	//Region            string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description string `json:"description"`
}

type vNetInfo struct { // Tumblebug
	Id             string       `json:"id"`
	Name           string       `json:"name"`
	ConnectionName string       `json:"connectionName"`
	CspVNetId      string       `json:"cspVNetId"`
	CspVNetName    string       `json:"cspVNetName"`
	CidrBlock      string       `json:"cidrBlock"`
	SubnetInfoList []SubnetInfo `json:"subnetInfoList"`
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// MCIS API Proxy: VNet
func RestPostVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &vNetReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST VNet requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating VNet]")
			content, _ := createVNet(nsId, u)
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
	//content, responseCode, body, err := createVNet(nsId, u)
	content, err := createVNet(nsId, u)
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

	id := c.Param("vNetId")

	content := vNetInfo{}

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
}

func RestGetAllVNet(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		VNet []vNetInfo `json:"vNet"`
	}

	vNetList := getResourceList(nsId, "vNet")

	for _, v := range vNetList {

		key := common.GenResourceKey(nsId, "vNet", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		vNetTmp := vNetInfo{}
		json.Unmarshal([]byte(keyValue.Value), &vNetTmp)
		vNetTmp.Id = v
		content.VNet = append(content.VNet, vNetTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutVNet(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelVNet(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("vNetId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delVNet(nsId, id, forceFlag)

	responseCode, body, err := delResource(nsId, "vNet", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the vNet"}
		return c.JSONBlob(responseCode, body)
	}

	mapA := map[string]string{"message": "The vNet has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllVNet(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	vNetList := getResourceList(nsId, "vNet")

	if len(vNetList) == 0 {
		mapA := map[string]string{"message": "There is no vNet element in this namespace."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		for _, v := range vNetList {
			//responseCode, body, err := delVNet(nsId, v, forceFlag)

			responseCode, body, err := delResource(nsId, "vNet", v, forceFlag)
			if err != nil {
				cblog.Error(err)
				//mapA := map[string]string{"message": "Failed to delete the vNet"}
				return c.JSONBlob(responseCode, body)
			}

		}

		mapA := map[string]string{"message": "All vNets has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	}
}

//func createVNet(nsId string, u *vNetReq) (vNetInfo, int, []byte, error) {
func createVNet(nsId string, u *vNetReq) (vNetInfo, error) {
	check, _ := checkResource(nsId, "vNet", u.Name)

	if check {
		temp := vNetInfo{}
		err := fmt.Errorf("The vNet " + u.Name + " already exists.")
		return temp, err
	}

	/* FYI; as of 2020-04-17
	type vNetReq struct {
		//Id                string `json:"id"`
		Name           string `json:"name"`
		ConnectionName string `json:"connectionName"`
		//CspVNetId      string `json:"cspVNetId"`
		CspVNetName string `json:"cspVNetName"`
		//CidrBlock         string `json:"cidrBlock"`
		//Region            string `json:"region"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description string `json:"description"`
	}
	*/

	//url := SPIDER_URL + "/vpc?connection_name=" + u.ConnectionName
	url := SPIDER_URL + "/vpc"

	method := "POST"

	tempReq := SpiderVPCReqInfo{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.IId.NameId = u.Name
	tempReq.ReqInfo.IId.SystemId = u.CspVNetName
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
		content := vNetInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	//fmt.Println("res.Body: " + string(res.Body)) // for debug

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := vNetInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		content := vNetInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	temp := VPCInfo{} // Spider
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI; as of 2020-04-17
	type vNetInfo struct {
		Id             string `json:"id"`
		Name           string `json:"name"`
		ConnectionName string `json:"connectionName"`
		CspVNetId   string `json:"cspVNetId"`
		CspVNetName string `json:"cspVNetName"`
		CidrBlock      string `json:"cidrBlock"`
		//Region         string `json:"region"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description  string            `json:"description"`
		Status       string            `json:"status"`
		KeyValueList []common.KeyValue `json:"keyValueList"`
	}
	*/

	content := vNetInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspVNetId = temp.IId.SystemId // CspSubnetId
	content.CspVNetName = temp.IId.NameId // = u.CspVNetName
	content.CidrBlock = temp.IPv4_CIDR
	content.SubnetInfoList = temp.SubnetInfoList
	content.Description = u.Description
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT createVNet")
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

/*
func getVNetList(nsId string) []string {

	fmt.Println("[Get vNets")
	key := "/ns/" + nsId + "/resources/vNet"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var vNetList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		vNetList = append(vNetList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/vNet/"))
		//}
	}
	for _, v := range vNetList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return vNetList

}
*/

/*
func delVNet(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete vNet] " + Id)

	key := genResourceKey(nsId, "vNet", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := vNetInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspVNetId: " + temp.CspVNetId)

	//url := SPIDER_URL + "/vpc?connection_name=" + temp.ConnectionName                           // for testapi.io
	url := SPIDER_URL + "/vpc/" + temp.CspVNetId + "?connection_name=" + temp.ConnectionName // for CB-Spider
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
	if err != nil {
		cblog.Error(err)
		return res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		return res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case forceFlag == "true":
		cbStoreDeleteErr := store.Delete(key)
		if cbStoreDeleteErr != nil {
			cblog.Error(cbStoreDeleteErr)
			return res.StatusCode, body, cbStoreDeleteErr
		}
		return res.StatusCode, body, nil
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		return res.StatusCode, body, err
	default:
		cbStoreDeleteErr := store.Delete(key)
		if cbStoreDeleteErr != nil {
			cblog.Error(cbStoreDeleteErr)
			return res.StatusCode, body, cbStoreDeleteErr
		}
		return res.StatusCode, body, nil
	}
}
*/
