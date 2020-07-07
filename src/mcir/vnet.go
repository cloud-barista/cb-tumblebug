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

// 2020-04-09 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

type SpiderVPCReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderVPCReqInfo
}

type SpiderVPCReqInfo struct { // Spider
	Name           string
	IPv4_CIDR      string
	SubnetInfoList []SpiderSubnetReqInfo
	//SubnetInfoList []SpiderSubnetInfo
}

type SpiderSubnetReqInfo struct { // Spider
	Name         string
	IPv4_CIDR    string
	KeyValueList []common.KeyValue
}

type SpiderVPCInfo struct { // Spider
	IId            common.IID // {NameId, SystemId}
	IPv4_CIDR      string
	SubnetInfoList []SpiderSubnetInfo
	KeyValueList   []common.KeyValue
}

type SpiderSubnetInfo struct { // Spider
	IId          common.IID // {NameId, SystemId}
	IPv4_CIDR    string
	KeyValueList []common.KeyValue
}

type TbVNetReq struct { // Tumblebug
	Name           string                `json:"name"`
	ConnectionName string                `json:"connectionName"`
	CidrBlock      string                `json:"cidrBlock"`
	SubnetInfoList []SpiderSubnetReqInfo `json:"subnetInfoList"`
	Description    string                `json:"description"`
}

type TbVNetInfo struct { // Tumblebug
	Id             string             `json:"id"`
	Name           string             `json:"name"`
	ConnectionName string             `json:"connectionName"`
	CidrBlock      string             `json:"cidrBlock"`
	SubnetInfoList []SpiderSubnetInfo `json:"subnetInfoList"`
	Description    string             `json:"description"`
	CspVNetId      string             `json:"cspVNetId"`
	CspVNetName    string             `json:"cspVNetName"`
	Status         string             `json:"status"`
	KeyValueList   []common.KeyValue  `json:"keyValueList"`

	// Disabled for now
	//Region         string `json:"region"`
	//ResourceGroupName string `json:"resourceGroupName"`
}

func CreateVNet(nsId string, u *TbVNetReq) (TbVNetInfo, error) {
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
		common.CBLog.Error(err)
		content := TbVNetInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	//fmt.Println("res.Body: " + string(res.Body)) // for debug

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		common.CBLog.Error(err)
		content := TbVNetInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
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
	err3 := common.CBStore.Put(string(Key), string(Val))
	if err3 != nil {
		common.CBLog.Error(err3)
		//return content, res.StatusCode, body, err3
		return content, err3
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	//return content, res.StatusCode, body, nil
	return content, nil
}
