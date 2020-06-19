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

// 2020-04-03 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/KeyPairHandler.go

type SpiderKeyPairReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderKeyPairReqInfo
}

type SpiderKeyPairReqInfo struct { // Spider
	Name string
}

type SpiderKeyPairInfo struct { // Spider
	IId         common.IID // {NameId, SystemId}
	Fingerprint string
	PublicKey   string
	PrivateKey  string
	VMUserID    string

	KeyValueList []common.KeyValue
}

type TbSshKeyReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	//CspSshKeyName  string `json:"cspSshKeyName"`
	//Fingerprint    string `json:"fingerprint"`
	//Username       string `json:"username"`
	//PublicKey      string `json:"publicKey"`
	//PrivateKey     string `json:"privateKey"`
	Description string `json:"description"`
}

type TbSshKeyInfo struct {
	Id             string            `json:"id"`
	Name           string            `json:"name"`
	ConnectionName string            `json:"connectionName"`
	CspSshKeyName  string            `json:"cspSshKeyName"`
	Fingerprint    string            `json:"fingerprint"`
	Username       string            `json:"username"`
	PublicKey      string            `json:"publicKey"`
	PrivateKey     string            `json:"privateKey"`
	Description    string            `json:"description"`
	KeyValueList   []common.KeyValue `json:"keyValueList"`
}

// MCIS API Proxy: SshKey
func RestPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &TbSshKeyReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SshKey requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SshKey]")
			content, _ := CreateSshKey(nsId, u)
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
	content, responseCode, _, err := CreateSshKey(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SshKey"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		*/
		//return c.JSON(res.StatusCode, res)
		//body, _ := ioutil.ReadAll(res.Body)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "sshKey"

	id := c.Param("sshKeyId")

	/*
		content := TbSshKeyInfo{}

		fmt.Println("[Get sshKey for id]" + id)
		key := common.GenResourceKey(nsId, "sshKey", id)
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Failed to find the sshKey with given ID."}
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

func RestGetAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "sshKey"

	var content struct {
		SshKey []TbSshKeyInfo `json:"sshKey"`
	}

	/*
		sshKeyList := ListResourceId(nsId, "sshKey")

		for _, v := range sshKeyList {

			key := common.GenResourceKey(nsId, "sshKey", v)
			fmt.Println(key)
			keyValue, _ := store.Get(key)
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			sshKeyTmp := TbSshKeyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &sshKeyTmp)
			sshKeyTmp.Id = v
			content.SshKey = append(content.SshKey, sshKeyTmp)

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
	content.SshKey = resourceList.([]TbSshKeyInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}

func RestPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "sshKey"
	id := c.Param("sshKeyId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delSshKey(nsId, id, forceFlag)

	responseCode, body, err := DelResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		cblog.Error(err)

		//mapA := map[string]string{"message": "Failed to delete the sshKey"}
		//return c.JSON(http.StatusFailedDependency, &mapA)

		return c.JSONBlob(responseCode, body)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
	//return c.JSON(http.StatusOK, body)
}

func RestDelAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "sshKey"
	forceFlag := c.QueryParam("force")

	/*
		sshKeyList := ListResourceId(nsId, "sshKey")

		if len(sshKeyList) == 0 {
			mapA := map[string]string{"message": "There is no sshKey element in this namespace."}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			for _, v := range sshKeyList {
				//responseCode, body, err := delSshKey(nsId, v, forceFlag)

				responseCode, body, err := DelResource(nsId, "sshKey", v, forceFlag)
				if err != nil {
					cblog.Error(err)

					//mapA := map[string]string{"message": "Failed to delete the sshKey"}
					//return c.JSON(http.StatusFailedDependency, &mapA)

					return c.JSONBlob(responseCode, body)
				}

			}

			mapA := map[string]string{"message": "All sshKeys has been deleted"}
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

func CreateSshKey(nsId string, u *TbSshKeyReq) (TbSshKeyInfo, int, []byte, error) {
	check, _ := CheckResource(nsId, "sshKey", u.Name)

	if check {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("The sshKey " + u.Name + " already exists.")
		return temp, http.StatusConflict, nil, err
	}

	//url := common.SPIDER_URL + "/keypair?connection_name=" + u.ConnectionName
	url := common.SPIDER_URL + "/keypair"

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspSshKeyName + "\"}")
	tempReq := SpiderKeyPairReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = u.Name
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
		cblog.Error(err)
		content := TbSshKeyInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := TbSshKeyInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		content := TbSshKeyInfo{}
		return content, res.StatusCode, body, err
	}

	temp := SpiderKeyPairInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	content := TbSshKeyInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspSshKeyName = temp.IId.NameId
	content.Fingerprint = temp.Fingerprint
	content.Username = temp.VMUserID
	content.PublicKey = temp.PublicKey
	content.PrivateKey = temp.PrivateKey
	content.Description = u.Description
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT CreateSshKey")
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
