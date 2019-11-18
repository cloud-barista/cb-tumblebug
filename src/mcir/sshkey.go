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

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/KeyPairHandler.go
/* FYI
type KeyPairReqInfo struct {
        Name     string
}

type KeyPairInfo struct {
     Name        string
     Fingerprint string
     PublicKey   string
     PrivateKey  string
     VMUserID      string

     KeyValueList []KeyValue
}
*/

type sshKeyReq struct {
	//Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	CspSshKeyName  string `json:"cspSshKeyName"`
	//Fingerprint    string `json:"fingerprint"`
	//Username       string `json:"username"`
	//PublicKey      string `json:"publicKey"`
	//PrivateKey     string `json:"privateKey"`
	Description string `json:"description"`
}

type sshKeyInfo struct {
	Id             string     `json:"id"`
	ConnectionName string     `json:"connectionName"`
	CspSshKeyName  string     `json:"cspSshKeyName"`
	Fingerprint    string     `json:"fingerprint"`
	Username       string     `json:"username"`
	PublicKey      string     `json:"publicKey"`
	PrivateKey     string     `json:"privateKey"`
	Description    string     `json:"description"`
	KeyValueList   []common.KeyValue `json:"keyValueList"`
}

/* FYI
g.POST("/:nsId/resources/sshKey", restPostSshKey)
g.GET("/:nsId/resources/sshKey/:sshKeyId", restGetSshKey)
g.GET("/:nsId/resources/sshKey", restGetAllSshKey)
g.PUT("/:nsId/resources/sshKey/:sshKeyId", restPutSshKey)
g.DELETE("/:nsId/resources/sshKey/:sshKeyId", restDelSshKey)
g.DELETE("/:nsId/resources/sshKey", restDelAllSshKey)
*/

// MCIS API Proxy: SshKey
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &sshKeyReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SshKey requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SshKey]")
			content, _ := createSshKey(nsId, u)
			return c.JSON(http.StatusCreated, content)

				} else if action == "register" {
					fmt.Println("[Registering SshKey]")
					content, _ := registerSshKey(nsId, u)
					return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST SshKey")
	fmt.Println("[Creating SshKey]")
	content, responseCode, body, err := createSshKey(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SshKey"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		*/
		//return c.JSON(res.StatusCode, res)
		//body, _ := ioutil.ReadAll(res.Body)
		fmt.Println("body: ", string(body))
		return c.JSONBlob(responseCode, body)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("sshKeyId")

	content := sshKeyInfo{}

	fmt.Println("[Get sshKey for id]" + id)
	key := common.GenResourceKey(nsId, "sshKey", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func RestGetAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		SshKey []sshKeyInfo `json:"sshKey"`
	}

	sshKeyList := getResourceList(nsId, "sshKey")

	for _, v := range sshKeyList {

		key := common.GenResourceKey(nsId, "sshKey", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		sshKeyTmp := sshKeyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &sshKeyTmp)
		sshKeyTmp.Id = v
		content.SshKey = append(content.SshKey, sshKeyTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("sshKeyId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delSshKey(nsId, id, forceFlag)

	responseCode, body, err := delResource(nsId, "sshKey", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{"message": "Failed to delete the sshKey"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		*/
		return c.JSONBlob(responseCode, body)
	}
	

	mapA := map[string]string{"message": "The sshKey has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
	//return c.JSON(http.StatusOK, body)
}

func RestDelAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	sshKeyList := getResourceList(nsId, "sshKey")

	for _, v := range sshKeyList {
		//responseCode, body, err := delSshKey(nsId, v, forceFlag)

		responseCode, body, err := delResource(nsId, "sshKey", v, forceFlag)
		if err != nil {
			cblog.Error(err)
			/*
				mapA := map[string]string{"message": "Failed to delete the sshKey"}
				return c.JSON(http.StatusFailedDependency, &mapA)
			*/
			return c.JSONBlob(responseCode, body)
		}
		
	}

	mapA := map[string]string{"message": "All sshKeys has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSshKey(nsId string, u *sshKeyReq) (sshKeyInfo, int, []byte, error) {

	/* FYI
	type sshKeyReq struct {
		//Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`
		CspSshKeyName  string `json:"cspSshKeyName"`
		//Fingerprint    string `json:"fingerprint"`
		//Username       string `json:"username"`
		//PublicKey      string `json:"publicKey"`
		//PrivateKey     string `json:"privateKey"`
		Description string `json:"description"`
	}
	*/

	url := SPIDER_URL + "/keypair?connection_name=" + u.ConnectionName

	method := "POST"

	payload := strings.NewReader("{ \"Name\": \"" + u.CspSshKeyName + "\"}")

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
	if err != nil {
		cblog.Error(err)
		content := sshKeyInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := sshKeyInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		content := sshKeyInfo{}
		return content, res.StatusCode, body, err
	}

	type KeyPairInfo struct {
		Name        string
		Fingerprint string
		PublicKey   string
		PrivateKey  string
		VMUserID    string

		KeyValueList []common.KeyValue
	}
	temp := KeyPairInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI
	type sshKeyInfo struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`
		CspSshKeyName  string `json:"cspSshKeyName"`
		Fingerprint    string `json:"fingerprint"`
		Username       string `json:"username"`
		PublicKey      string `json:"publicKey"`
		PrivateKey     string `json:"privateKey"`
		Description    string `json:"description"`
		KeyValueList []KeyValue `json:"keyValueList"`
	}
	*/

	content := sshKeyInfo{}
	content.Id = common.GenUuid()
	content.ConnectionName = u.ConnectionName
	content.CspSshKeyName = temp.Name // = u.CspSshKeyName
	content.Fingerprint = temp.Fingerprint
	content.Username = temp.VMUserID
	content.PublicKey = temp.PublicKey
	content.PrivateKey = temp.PrivateKey
	content.Description = u.Description
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT createSshKey")
	Key := common.GenResourceKey(nsId, "sshKey", content.Id)
	/*
		mapA := map[string]string{
			"connectionName": content.ConnectionName,
			"cspSshKeyName":  content.CspSshKeyName,
			"fingerprint":    content.Fingerprint,
			"username":       content.Username,
			"publicKey":      content.PublicKey,
			"privateKey":     content.PrivateKey,
			"description":    content.Description}
		Val, _ := json.Marshal(mapA)
	*/
	Val, _ := json.Marshal(content)
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
func getSshKeyList(nsId string) []string {

	fmt.Println("[Get sshKeys")
	key := "/ns/" + nsId + "/resources/sshKey"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var sshKeyList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		sshKeyList = append(sshKeyList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/sshKey/"))
		//}
	}
	for _, v := range sshKeyList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return sshKeyList

}
*/

/*
func delSshKey(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete sshKey] " + Id)

	key := genResourceKey(nsId, "sshKey", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := sshKeyInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspSshKeyName: " + temp.CspSshKeyName)

	//url := SPIDER_URL + "/keypair?connection_name=" + temp.ConnectionName // for testapi.io
	url := SPIDER_URL + "/keypair/" + temp.CspSshKeyName + "?connection_name=" + temp.ConnectionName // for CB-Spider
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