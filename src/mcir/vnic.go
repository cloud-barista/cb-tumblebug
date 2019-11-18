package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo"
	"github.com/cloud-barista/cb-tumblebug/src/common"
)

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/VNicHandler.go
/* FYI
type VNicReqInfo struct {
	Name             string
	VNetName         string
	SecurityGroupIds []string
	PublicIPid       string
}

type VNicInfo struct {
	Id               string
	Name             string
	PublicIP         string
	MacAddress        string
	OwnedVMID        string
	SecurityGroupIds []string
	Status           string

	KeyValueList []KeyValue
}
*/

type vNicReq struct {
	//Id                string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//CspVNicId     string `json:"cspVNicId"`
	CspVNicName string `json:"cspVNicName"`
	CspVNetName string `json:"cspVNetName"`
	PublicIpId  string `json:"publicIpId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description      string   `json:"description"`
	SecurityGroupIds []string `json:"securityGroupIds"`
}

type vNicInfo struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspVNicId      string `json:"cspVNicId"`
	CspVNicName    string `json:"cspVNicName"`
	CspVNetName    string `json:"cspVNetName"`
	PublicIpId     string `json:"publicIpId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description      string     `json:"description"`
	PublicIp         string     `json:"publicIp"`
	MacAddress       string     `json:"macAddress"`
	OwnedVmId        string     `json:"ownedVmId"`
	Status           string     `json:"status"`
	SecurityGroupIds []string   `json:"securityGroupIds"`
	KeyValueList     []common.KeyValue `json:"keyValueList"`
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
func RestPostVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &vNicReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
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
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST VNic")
	fmt.Println("[Creating VNic]")
	content, responseCode, body, err := createVNic(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a VNic"}
		*/
		return c.JSONBlob(responseCode, body)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("vNicId")

	content := vNicInfo{}

	fmt.Println("[Get vNic for id]" + id)
	key := common.GenResourceKey(nsId, "vNic", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func RestGetAllVNic(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		VNic []vNicInfo `json:"vNic"`
	}

	vNicList := getResourceList(nsId, "vNic")

	for _, v := range vNicList {

		key := common.GenResourceKey(nsId, "vNic", v)
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

func RestPutVNic(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelVNic(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("vNicId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delVNic(nsId, id, forceFlag)

	responseCode, body, err := delResource(nsId, "vNic", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the vNic"}
		return c.JSONBlob(responseCode, body)
	}
	

	mapA := map[string]string{"message": "The vNic has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllVNic(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	vNicList := getResourceList(nsId, "vNic")

	for _, v := range vNicList {
		//responseCode, body, err := delVNic(nsId, v, forceFlag)

		responseCode, body, err := delResource(nsId, "vNic", v, forceFlag)
		if err != nil {
			cblog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete the vNic"}
			return c.JSONBlob(responseCode, body)
		}
		
	}

	mapA := map[string]string{"message": "All vNics has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createVNic(nsId string, u *vNicReq) (vNicInfo, int, []byte, error) {

	/* FYI
	type vNicReq struct {
		//Id                string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//CspVNicId     string `json:"cspVNicId"`
		CspVNicName string `json:"cspVNicName"`
		CspVNetName string `json:"cspVNetName"`
		PublicIpId  string `json:"publicIpId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description string `json:"description"`
		SecurityGroupIds []string `json:"securityGroupIds"`
	}
	*/

	url := SPIDER_URL + "/vnic?connection_name=" + u.ConnectionName

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspSshKeyName + "\"}")

	/* Mark 1
	type VNicReqInfo struct {
		Name             string
		VNetName         string
		SecurityGroupIds []string
		PublicIPid       string
	}
	tempReq := VNicReqInfo{}
	tempReq.Name = u.CspVNicName
	tempReq.VNetName = u.CspVNetName
	//tempReq.SecurityGroupIds =
	tempReq.PublicIPid = u.PublicIpId
	*/

	/* Mark 2
	tempReq := map[string]string{
		"Name":     u.CspVNicName,
		"VNetName": u.CspVNetName,
		//"SecurityGroupIds":    content.Fingerprint,
		"PublicIPid": u.PublicIpId}
	*/

	// Mark 3
	type VNicReqInfo struct {
		Name             string
		VNetName         string
		SecurityGroupIds []string
		PublicIPid       string
	}
	tempReq := VNicReqInfo{}
	tempReq.Name = u.CspVNicName
	tempReq.VNetName = u.CspVNetName
	tempReq.SecurityGroupIds = u.SecurityGroupIds
	tempReq.PublicIPid = u.PublicIpId
	payload, _ := json.Marshal(tempReq)

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
		content := vNicInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := vNicInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		cblog.Error(err)
		content := vNicInfo{}
		return content, res.StatusCode, body, err
	}

	type VNicInfo struct {
		Id               string
		Name             string
		PublicIP         string
		MacAddress       string
		OwnedVMID        string
		SecurityGroupIds []string
		Status           string

		KeyValueList []common.KeyValue
	}
	temp := VNicInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI
	type vNicInfo struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`
		CspVNicId      string `json:"cspVNicId"`
		CspVNicName    string `json:"cspVNicName"`
		CspVNetName    string `json:"cspVNetName"`
		PublicIpId     string `json:"publicIpId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description string `json:"description"`
		PublicIp    string `json:"publicIp"`
		MacAddress  string `json:"macAddress"`
		OwnedVmId   string `json:"ownedVmId"`
		Status      string `json:"status"`
		SecurityGroupIds []string   `json:"securityGroupIds"`
		KeyValueList     []KeyValue `json:"keyValueList"`
	}
	*/

	content := vNicInfo{}
	content.Id = common.GenUuid()
	content.ConnectionName = u.ConnectionName
	content.CspVNicId = temp.Id
	content.CspVNicName = temp.Name // = u.CspVNicName
	content.CspVNetName = u.CspVNetName
	content.PublicIpId = u.PublicIpId
	content.Description = u.Description
	content.PublicIp = temp.PublicIP
	content.MacAddress = temp.MacAddress
	content.OwnedVmId = temp.OwnedVMID
	content.Status = temp.Status
	content.SecurityGroupIds = temp.SecurityGroupIds
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT createVNic")
	Key := common.GenResourceKey(nsId, "vNic", content.Id)
	/*
		mapA := map[string]string{
			"connectionName": content.ConnectionName,
			"cspVNicId":      content.CspVNicId,
			"cspVNicName":    content.CspVNicName,
			"cspVNetName":    content.CspVNetName,
			"publicIpId":     content.PublicIpId,
			//"resourceGroupName": content.ResourceGroupName,
			"description": content.Description,
			"publicIp":    content.PublicIp,
			"macAddress":  content.MacAddress,
			"ownedVmId":   content.OwnedVmId,
			"status":      content.Status}
		Val, _ := json.Marshal(mapA)
	*/
	Val, _ := json.Marshal(content)
	fmt.Println("Key: ", Key)
	fmt.Println("Val: ", Val)
	cbStorePutErr := store.Put(string(Key), string(Val))
	if cbStorePutErr != nil {
		cblog.Error(cbStorePutErr)
		return content, res.StatusCode, body, cbStorePutErr
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, res.StatusCode, body, nil
}

/*
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
*/

/*
func delVNic(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete vNic] " + Id)

	key := genResourceKey(nsId, "vNic", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := vNicInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspVNicId: " + temp.CspVNicId)

	//url := SPIDER_URL + "/vnic?connection_name=" + temp.ConnectionName // for testapi.io
	url := SPIDER_URL + "/vnic/" + temp.CspVNicId + "?connection_name=" + temp.ConnectionName // for CB-Spider
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
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
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