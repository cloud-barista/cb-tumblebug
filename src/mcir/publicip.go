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

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/PublicIPHandler.go
/*
type PublicIPReqInfo struct {
	Name         string
	KeyValueList []KeyValue
}

type PublicIPInfo struct {
	Name      string
	PublicIP  string
	OwnedVMID string
	Status    string

	KeyValueList []KeyValue
}
*/

type publicIpReq struct {
	//Id                string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//CspPublicIpId     string `json:"cspPublicIpId"`
	CspPublicIpName string `json:"cspPublicIpName"`
	//PublicIp          string `json:"publicIp"`
	//OwnedVmId         string `json:"ownedVmId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string     `json:"description"`
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type publicIpInfo struct {
	Id              string `json:"id"`
	ConnectionName  string `json:"connectionName"`
	CspPublicIpId   string `json:"cspPublicIpId"`
	CspPublicIpName string `json:"cspPublicIpName"`
	PublicIp        string `json:"publicIp"`
	OwnedVmId       string `json:"ownedVmId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	KeyValueList []common.KeyValue `json:"keyValueList"`
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
func RestPostPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &publicIpReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
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
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST PublicIp")
	fmt.Println("[Creating PublicIp]")
	content, responseCode, body, err := createPublicIp(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a PublicIp"}
		*/
		return c.JSONBlob(responseCode, body)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("publicIpId")

	content := publicIpInfo{}

	fmt.Println("[Get publicIp for id]" + id)
	key := common.GenResourceKey(nsId, "publicIp", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func RestGetAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		PublicIp []publicIpInfo `json:"publicIp"`
	}

	publicIpList := getResourceList(nsId, "publicIp")

	for _, v := range publicIpList {

		key := common.GenResourceKey(nsId, "publicIp", v)
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

func RestPutPublicIp(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("publicIpId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delPublicIp(nsId, id, forceFlag)

	responseCode, body, err := delResource(nsId, "publicIp", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the publicIp"}
		return c.JSONBlob(responseCode, body)
	}
	

	mapA := map[string]string{"message": "The publicIp has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	publicIpList := getResourceList(nsId, "publicIp")

	for _, v := range publicIpList {
		//responseCode, body, err := delPublicIp(nsId, v, forceFlag)

		responseCode, body, err := delResource(nsId, "publicIp", v, forceFlag)
		if err != nil {
			cblog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete the publicIp"}
			return c.JSONBlob(responseCode, body)
		}
		
	}

	mapA := map[string]string{"message": "All publicIps has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createPublicIp(nsId string, u *publicIpReq) (publicIpInfo, int, []byte, error) {

	/* FYI
	type publicIpReq struct {
		//Id                string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//CspPublicIpId     string `json:"cspPublicIpId"`
		CspPublicIpName string `json:"cspPublicIpName"`
		//PublicIp          string `json:"publicIp"`
		//OwnedVmId         string `json:"ownedVmId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description string `json:"description"`
		KeyValueList []KeyValue `json:"keyValueList"`
	}
	*/

	url := SPIDER_URL + "/publicip?connection_name=" + u.ConnectionName

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspPublicIpName + "\"}")
	type PublicIPReqInfo struct {
		Name         string
		KeyValueList []common.KeyValue
	}
	tempReq := PublicIPReqInfo{}
	tempReq.Name = u.CspPublicIpName
	tempReq.KeyValueList = u.KeyValueList
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
		content := publicIpInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		cblog.Error(err)
		content := publicIpInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		cblog.Error(err)
		content := publicIpInfo{}
		return content, res.StatusCode, body, err
	}

	type PublicIPInfo struct {
		Name      string
		PublicIP  string
		OwnedVMID string
		Status    string

		KeyValueList []common.KeyValue
	}
	temp := PublicIPInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	content := publicIpInfo{}
	content.Id = common.GenUuid()
	content.ConnectionName = u.ConnectionName
	content.CspPublicIpId = temp.Name
	content.CspPublicIpName = temp.Name //common.LookupKeyValueList(temp.KeyValueList, "Name")
	content.PublicIp = temp.PublicIP
	content.OwnedVmId = temp.OwnedVMID
	content.Description = u.Description
	content.Status = temp.Status
	content.KeyValueList = temp.KeyValueList

	/* FYI
	type publicIpInfo struct {
		Id              string `json:"id"`
		ConnectionName  string `json:"connectionName"`
		CspPublicIpId   string `json:"cspPublicIpId"`
		CspPublicIpName string `json:"cspPublicIpName"`
		PublicIp        string `json:"publicIp"`
		OwnedVmId       string `json:"ownedVmId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description string `json:"description"`
		Status      string `json:"string"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createPublicIp")
	Key := common.GenResourceKey(nsId, "publicIp", content.Id)
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
*/

/*
func delPublicIp(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete publicIp] " + Id)

	key := genResourceKey(nsId, "publicIp", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := publicIpInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspPublicIpName: " + temp.CspPublicIpName) // Identifier is subject to change.

	//url := SPIDER_URL + "/publicip?connection_name=" + temp.ConnectionName // for testapi.io
	url := SPIDER_URL + "/publicip/" + temp.CspPublicIpId + "?connection_name=" + temp.ConnectionName // for CB-Spider
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