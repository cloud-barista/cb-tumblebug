// 2020-04-08 jhseo; The PublicIP mgmt feature will be deprecated in TB & Spider.

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
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	//CspPublicIpId     string `json:"cspPublicIpId"`
	CspPublicIpName string `json:"cspPublicIpName"`
	//PublicIp          string `json:"publicIp"`
	//OwnedVmId         string `json:"ownedVmId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string            `json:"description"`
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

type publicIpInfo struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	ConnectionName  string `json:"connectionName"`
	CspPublicIpId   string `json:"cspPublicIpId"`
	CspPublicIpName string `json:"cspPublicIpName"`
	PublicIp        string `json:"publicIp"`
	OwnedVmId       string `json:"ownedVmId"`
	//ResourceGroupName string `json:"resourceGroupName"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

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
		common.CBLog.Error(err)
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

	keyValue, _ := common.CBStore.Get(key)
	if keyValue == nil {
		mapA := map[string]string{"message": "Failed to find the publicIp with given ID."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)
		content.Id = id // Optional. Can be omitted.

		return c.JSON(http.StatusOK, &content)
	}
}

func RestGetAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		PublicIp []publicIpInfo `json:"publicIp"`
	}

	publicIpList := ListResourceId(nsId, "publicIp")

	for _, v := range publicIpList {

		key := common.GenResourceKey(nsId, "publicIp", v)
		fmt.Println(key)
		keyValue, _ := common.CBStore.Get(key)
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

	responseCode, body, err := DelResource(nsId, "publicIp", id, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the publicIp"}
		return c.JSONBlob(responseCode, body)
	}

	mapA := map[string]string{"message": "The publicIp has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllPublicIp(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	publicIpList := ListResourceId(nsId, "publicIp")

	if len(publicIpList) == 0 {
		mapA := map[string]string{"message": "There is no publicIp element in this namespace."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		for _, v := range publicIpList {
			//responseCode, body, err := delPublicIp(nsId, v, forceFlag)

			responseCode, body, err := DelResource(nsId, "publicIp", v, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				//mapA := map[string]string{"message": "Failed to delete the publicIp"}
				return c.JSONBlob(responseCode, body)
			}

		}

		mapA := map[string]string{"message": "All publicIps has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	}
}

func createPublicIp(nsId string, u *publicIpReq) (publicIpInfo, int, []byte, error) {
	check, _ := CheckResource(nsId, "publicIp", u.Name)

	if check {
		temp := publicIpInfo{}
		err := fmt.Errorf("The publicIp " + u.Name + " already exists.")
		return temp, http.StatusConflict, nil, err
	}

	/* FYI; as of 2020-04-17
	type publicIpReq struct {
		//Id                string `json:"id"`
		Name           string `json:"name"`
		ConnectionName string `json:"connectionName"`
		//CspPublicIpId     string `json:"cspPublicIpId"`
		CspPublicIpName string `json:"cspPublicIpName"`
		//PublicIp          string `json:"publicIp"`
		//OwnedVmId         string `json:"ownedVmId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description  string            `json:"description"`
		KeyValueList []common.KeyValue `json:"keyValueList"`
	}
	*/

	//url := common.SPIDER_URL + "/publicip?connection_name=" + u.ConnectionName
	url := common.SPIDER_URL + "/publicip"

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspPublicIpName + "\"}")
	type PublicIPReqInfo struct {
		ConnectionName string
		ReqInfo        struct {
			Name         string
			KeyValueList []common.KeyValue
		}
	}
	tempReq := PublicIPReqInfo{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = u.CspPublicIpName
	tempReq.ReqInfo.KeyValueList = u.KeyValueList
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload)) // for debug

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
		common.CBLog.Error(err)
		content := publicIpInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		common.CBLog.Error(err)
		content := publicIpInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println(string(body))

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
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
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspPublicIpId = temp.Name
	content.CspPublicIpName = temp.Name //common.LookupKeyValueList(temp.KeyValueList, "Name")
	content.PublicIp = temp.PublicIP
	content.OwnedVmId = temp.OwnedVMID
	content.Description = u.Description
	content.Status = temp.Status
	content.KeyValueList = temp.KeyValueList

	/* FYI; as of 2020-04-17
	type publicIpInfo struct {
		Id              string `json:"id"`
		Name            string `json:"name"`
		ConnectionName  string `json:"connectionName"`
		CspPublicIpId   string `json:"cspPublicIpId"`
		CspPublicIpName string `json:"cspPublicIpName"`
		PublicIp        string `json:"publicIp"`
		OwnedVmId       string `json:"ownedVmId"`
		//ResourceGroupName string `json:"resourceGroupName"`
		Description  string            `json:"description"`
		Status       string            `json:"status"`
		KeyValueList []common.KeyValue `json:"keyValueList"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createPublicIp")
	Key := common.GenResourceKey(nsId, "publicIp", content.Id)
	Val, _ := json.Marshal(content)
	fmt.Println("Key: ", Key)
	fmt.Println("Val: ", Val)
	err := common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, res.StatusCode, body, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, res.StatusCode, body, nil
}

/*
func getPublicIpList(nsId string) []string {

	fmt.Println("[Get publicIps")
	key := "/ns/" + nsId + "/resources/publicIp"
	fmt.Println(key)

	keyValue, _ := common.CBStore.GetList(key, true)
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

	keyValue, _ := common.CBStore.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := publicIpInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspPublicIpName: " + temp.CspPublicIpName) // Identifier is subject to change.

	//url := common.SPIDER_URL + "/publicip?connection_name=" + temp.ConnectionName // for testapi.io
	url := common.SPIDER_URL + "/publicip/" + temp.CspPublicIpId + "?connection_name=" + temp.ConnectionName // for CB-Spider
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
		common.CBLog.Error(err)
		return res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		common.CBLog.Error(err)
		return res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case forceFlag == "true":
		cbStoreDeleteErr := common.CBStore.Delete(key)
		if cbStoreDeleteErr != nil {
			common.CBLog.Error(cbStoreDeleteErr)
			return res.StatusCode, body, cbStoreDeleteErr
		}
		return res.StatusCode, body, nil
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		return res.StatusCode, body, err
	default:
		cbStoreDeleteErr := common.CBStore.Delete(key)
		if cbStoreDeleteErr != nil {
			common.CBLog.Error(cbStoreDeleteErr)
			return res.StatusCode, body, cbStoreDeleteErr
		}
		return res.StatusCode, body, nil
	}
}
*/
