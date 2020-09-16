package mcir

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/go-resty/resty/v2"
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

	tempReq := SpiderVPCReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = u.Name
	tempReq.ReqInfo.IPv4_CIDR = u.CidrBlock
	tempReq.ReqInfo.SubnetInfoList = u.SubnetInfoList

	var tempSpiderVPCInfo *SpiderVPCInfo

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

		//url := common.SPIDER_REST_URL + "/vpc?connection_name=" + u.ConnectionName
		url := common.SPIDER_REST_URL + "/vpc"

		/*
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

			tempSpiderVPCInfo = SpiderVPCInfo{} // Spider
			err2 := json.Unmarshal(body, &tempSpiderVPCInfo)
			if err2 != nil {
				fmt.Println("whoops:", err2)
			}
		*/

		client := resty.New()

		resp, _ := client.R().
			SetBody(tempReq).
			SetResult(&SpiderVPCInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Post(url)

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			content := TbVNetInfo{}
			//return content, res.StatusCode, body, err
			return content, err
		}

		tempSpiderVPCInfo = resp.Result().(*SpiderVPCInfo)

	} else {

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return TbVNetInfo{}, err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return TbVNetInfo{}, err
		}
		defer ccm.Close()

		payload, _ := json.MarshalIndent(tempReq, "", "  ")
		fmt.Println("payload: " + string(payload)) // for debug

		result, err := ccm.CreateVPC(string(payload))
		if err != nil {
			common.CBLog.Error(err)
			return TbVNetInfo{}, err
		}

		tempSpiderVPCInfo = &SpiderVPCInfo{} // Spider
		err2 := json.Unmarshal([]byte(result), &tempSpiderVPCInfo)
		if err2 != nil {
			fmt.Println("whoops:", err2)
		}

	}

	content := TbVNetInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspVNetId = tempSpiderVPCInfo.IId.SystemId
	content.CspVNetName = tempSpiderVPCInfo.IId.NameId
	content.CidrBlock = tempSpiderVPCInfo.IPv4_CIDR
	content.SubnetInfoList = tempSpiderVPCInfo.SubnetInfoList
	content.Description = u.Description
	content.KeyValueList = tempSpiderVPCInfo.KeyValueList

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
