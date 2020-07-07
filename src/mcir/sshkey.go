package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

// 2020-04-03 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/KeyPairHandler.go

type SpiderKeyPairReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderKeyPairInfo
}

/*
type SpiderKeyPairReqInfo struct { // Spider
	Name string
}
*/

type SpiderKeyPairInfo struct { // Spider
	// Fields for request
	Name string

	// Fields for response
	IId          common.IID // {NameId, SystemId}
	Fingerprint  string
	PublicKey    string
	PrivateKey   string
	VMUserID     string
	KeyValueList []common.KeyValue
}

type TbSshKeyReq struct {
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	Description    string `json:"description"`
}

type TbSshKeyInfo struct {
	Id             string            `json:"id"`
	Name           string            `json:"name"`
	ConnectionName string            `json:"connectionName"`
	Description    string            `json:"description"`
	CspSshKeyName  string            `json:"cspSshKeyName"`
	Fingerprint    string            `json:"fingerprint"`
	Username       string            `json:"username"`
	PublicKey      string            `json:"publicKey"`
	PrivateKey     string            `json:"privateKey"`
	KeyValueList   []common.KeyValue `json:"keyValueList"`
}

func CreateSshKey(nsId string, u *TbSshKeyReq) (TbSshKeyInfo, error) {
	check, _ := CheckResource(nsId, "sshKey", u.Name)

	if check {
		temp := TbSshKeyInfo{}
		err := fmt.Errorf("The sshKey " + u.Name + " already exists.")
		//return temp, http.StatusConflict, nil, err
		return temp, err
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
		common.CBLog.Error(err)
		content := TbSshKeyInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		common.CBLog.Error(err)
		content := TbSshKeyInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		common.CBLog.Error(err)
		content := TbSshKeyInfo{}
		//return content, res.StatusCode, body, err
		return content, err
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
	Val, _ := json.Marshal(content)
	err = common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		//return content, res.StatusCode, body, err
		return content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	//return content, res.StatusCode, body, nil
	return content, nil
}
