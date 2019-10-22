package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/labstack/echo"
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
	KeyValueList   []KeyValue `json:"keyValueList"`
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
func restPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &sshKeyReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST SshKey requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating SshKey]")
		content, _ := createSshKey(nsId, u)
		return c.JSON(http.StatusCreated, content)
		/*
			} else if action == "register" {
				fmt.Println("[Registering SshKey]")
				content, _ := registerSshKey(nsId, u)
				return c.JSON(http.StatusCreated, content)
		*/
	} else {
		mapA := map[string]string{"message": "You must specify: action=create"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("sshKeyId")

	content := sshKeyInfo{}

	fmt.Println("[Get sshKey for id]" + id)
	key := genResourceKey(nsId, "sshKey", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		SshKey []sshKeyInfo `json:"sshKey"`
	}

	sshKeyList := getSshKeyList(nsId)

	for _, v := range sshKeyList {

		key := genResourceKey(nsId, "sshKey", v)
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

func restPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("sshKeyId")

	err := delSshKey(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the sshKey"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The sshKey has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	sshKeyList := getSshKeyList(nsId)

	for _, v := range sshKeyList {
		err := delSshKey(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All sshKeys"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All sshKeys has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSshKey(nsId string, u *sshKeyReq) (sshKeyInfo, error) {

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
	fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	// jhseo 191016
	//var s = new(imageInfo)
	//s := imageInfo{}
	type KeyPairInfo struct {
		Name        string
		Fingerprint string
		PublicKey   string
		PrivateKey  string
		VMUserID    string

		KeyValueList []KeyValue
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
	content.Id = genUuid()
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
	Key := genResourceKey(nsId, "sshKey", content.Id)
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
		return content, cbStorePutErr
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

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

func delSshKey(nsId string, Id string) error {

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

	//url := "https://testapi.io/api/jihoon-seo/keypair/" + temp.CspSshKeyName + "?connection_name=" + temp.ConnectionName // for CB-Spider
	url := SPIDER_URL + "/keypair?connection_name=" + temp.ConnectionName // for testapi.io
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

	// delete sshKey info
	cbStoreDeleteErr := store.Delete(key)
	if cbStoreDeleteErr != nil {
		cblog.Error(cbStoreDeleteErr)
		return cbStoreDeleteErr
	}

	return nil
}
