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

// 2020-04-13 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/SecurityHandler.go

type SpiderSecurityReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderSecurityReqInfo
}

type SpiderSecurityReqInfo struct { // Spider
	Name          string
	VPCName       string
	SecurityRules *[]SpiderSecurityRuleInfo
	//Direction     string // @todo used??
}

type SpiderSecurityRuleInfo struct { // Spider
	FromPort   string `json:"fromPort"`
	ToPort     string `json:"toPort"`
	IPProtocol string `json:"ipProtocol"`
	Direction  string `json:"direction"`
}

type SpiderSecurityInfo struct { // Spider
	IId           common.IID // {NameId, SystemId}
	VpcIID        common.IID // {NameId, SystemId}
	Direction     string     // @todo userd??
	SecurityRules *[]SpiderSecurityRuleInfo

	KeyValueList []common.KeyValue
}

type TbSecurityGroupReq struct { // Tumblebug
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	VNetId         string `json:"vNetId"`
	//ResourceGroupName    string `json:"resourceGroupName"`
	Description   string                    `json:"description"`
	FirewallRules *[]SpiderSecurityRuleInfo `json:"firewallRules"`
}

type TbSecurityGroupInfo struct { // Tumblebug
	Id                   string `json:"id"`
	Name                 string `json:"name"`
	ConnectionName       string `json:"connectionName"`
	VNetId               string `json:"vNetId"`
	CspSecurityGroupId   string `json:"cspSecurityGroupId"`
	CspSecurityGroupName string `json:"cspSecurityGroupName"`
	//ResourceGroupName  string `json:"resourceGroupName"`
	Description   string                    `json:"description"`
	FirewallRules *[]SpiderSecurityRuleInfo `json:"firewallRules"`
	KeyValueList  []common.KeyValue         `json:"keyValueList"`
}

// MCIS API Proxy: SecurityGroup
func RestPostSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &TbSecurityGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SecurityGroup requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SecurityGroup]")
			content, _ := CreateSecurityGroup(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else if action == "register" {
			fmt.Println("[Registering SecurityGroup]")
			content, _ := registerSecurityGroup(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else {
			mapA := map[string]string{"message": "You must specify: action=create"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	*/

	fmt.Println("[POST SecurityGroup")
	fmt.Println("[Creating SecurityGroup]")
	content, responseCode, _, err := CreateSecurityGroup(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SecurityGroup"}
		*/
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "securityGroup"

	id := c.Param("securityGroupId")

	/*
		content := TbSecurityGroupInfo{}

		fmt.Println("[Get securityGroup for id]" + id)
		key := common.GenResourceKey(nsId, "securityGroup", id)
		fmt.Println(key)

		keyValue, _ := store.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Failed to find the securityGroup with given ID."}
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

func RestGetAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "securityGroup"

	var content struct {
		SecurityGroup []TbSecurityGroupInfo `json:"securityGroup"`
	}

	/*
		securityGroupList := ListResourceId(nsId, "securityGroup")

		for _, v := range securityGroupList {

			key := common.GenResourceKey(nsId, "securityGroup", v)
			fmt.Println(key)
			keyValue, _ := store.Get(key)
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			securityGroupTmp := TbSecurityGroupInfo{}
			json.Unmarshal([]byte(keyValue.Value), &securityGroupTmp)
			securityGroupTmp.Id = v
			content.SecurityGroup = append(content.SecurityGroup, securityGroupTmp)

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
	content.SecurityGroup = resourceList.([]TbSecurityGroupInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}

func RestPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "securityGroup"
	id := c.Param("securityGroupId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delSecurityGroup(nsId, id, forceFlag)

	err := DelResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the securityGroup"}
		//return c.JSONBlob(responseCode, body)
		return c.JSON(http.StatusFailedDependency, err)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "securityGroup"
	forceFlag := c.QueryParam("force")

	/*
		securityGroupList := ListResourceId(nsId, "securityGroup")

		if len(securityGroupList) == 0 {
			mapA := map[string]string{"message": "There is no securityGroup element in this namespace."}
			return c.JSON(http.StatusNotFound, &mapA)
		} else {
			for _, v := range securityGroupList {
				//responseCode, body, err := delSecurityGroup(nsId, v, forceFlag)

				responseCode, body, err := DelResource(nsId, "securityGroup", v, forceFlag)
				if err != nil {
					cblog.Error(err)
					//mapA := map[string]string{"message": "Failed to delete the securityGroup"}
					return c.JSONBlob(responseCode, body)
				}

			}

			mapA := map[string]string{"message": "All securityGroups has been deleted"}
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

func CreateSecurityGroup(nsId string, u *TbSecurityGroupReq) (TbSecurityGroupInfo, int, []byte, error) {
	check, _ := CheckResource(nsId, "securityGroup", u.Name)

	if check {
		temp := TbSecurityGroupInfo{}
		err := fmt.Errorf("The securityGroup " + u.Name + " already exists.")
		return temp, http.StatusConflict, nil, err
	}

	//url := common.SPIDER_URL + "/securitygroup?connection_name=" + u.ConnectionName
	url := common.SPIDER_URL + "/securitygroup"

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspSecurityGroupName + "\"}")
	tempReq := SpiderSecurityReqInfoWrapper{}
	tempReq.ConnectionName = u.ConnectionName
	tempReq.ReqInfo.Name = u.Name
	tempReq.ReqInfo.VPCName = u.VNetId
	tempReq.ReqInfo.SecurityRules = u.FirewallRules

	payload, _ := json.Marshal(tempReq)
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
		content := TbSecurityGroupInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := TbSecurityGroupInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		cblog.Error(err)
		content := TbSecurityGroupInfo{}
		return content, res.StatusCode, body, err
	}

	temp := SpiderSecurityInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	content := TbSecurityGroupInfo{}
	//content.Id = common.GenUuid()
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.VNetId = temp.VpcIID.NameId
	content.CspSecurityGroupId = temp.IId.SystemId
	content.CspSecurityGroupName = temp.IId.NameId
	content.Description = u.Description
	content.FirewallRules = temp.SecurityRules
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT CreateSecurityGroup")
	Key := common.GenResourceKey(nsId, "securityGroup", content.Id)
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
