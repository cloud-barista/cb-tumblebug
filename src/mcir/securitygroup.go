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

// https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/SecurityHandler.go
/* FYI
type SecurityReqInfo struct {
	Name          string
	SecurityRules *[]SecurityRuleInfo
}

type SecurityRuleInfo struct {
	FromPort   string
	ToPort     string
	IPProtocol string
	Direction  string
}

type SecurityInfo struct {
	Id            string
	Name          string
	SecurityRules *[]SecurityRuleInfo

	KeyValueList []KeyValue
}
*/

type firewallRuleInfo struct {
	FromPort   string `json:"fromPort"`
	ToPort     string `json:"toPort"`
	IPProtocol string `json:"ipProtocol"`
	Direction  string `json:"direction"`
}

type securityGroupReq struct {
	//Id                 string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//VirtualNetworkId     string `json:"virtualNetworkId"`
	//CspSecurityGroupId   string `json:"cspSecurityGroupId"`
	CspSecurityGroupName string `json:"cspSecurityGroupName"`
	//ResourceGroupName    string `json:"resourceGroupName"`
	Description   string              `json:"description"`
	FirewallRules *[]firewallRuleInfo `json:"firewallRules"`
}

type securityGroupInfo struct {
	Id             string `json:"id"`
	ConnectionName string `json:"connectionName"`
	//VirtualNetworkId   string `json:"virtualNetworkId"`
	CspSecurityGroupId   string `json:"cspSecurityGroupId"`
	CspSecurityGroupName string `json:"cspSecurityGroupName"`
	//ResourceGroupName  string `json:"resourceGroupName"`
	Description   string              `json:"description"`
	FirewallRules *[]firewallRuleInfo `json:"firewallRules"`
	KeyValueList  []common.KeyValue          `json:"keyValueList"`
}

/* FYI
g.POST("/resources/securityGroup", restPostSecurityGroup)
g.GET("/resources/securityGroup/:securityGroupId", restGetSecurityGroup)
g.GET("/resources/securityGroup", restGetAllSecurityGroup)
g.PUT("/resources/securityGroup/:securityGroupId", restPutSecurityGroup)
g.DELETE("/resources/securityGroup/:securityGroupId", restDelSecurityGroup)
g.DELETE("/resources/securityGroup", restDelAllSecurityGroup)
*/

// MCIS API Proxy: SecurityGroup
func RestPostSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &securityGroupReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	/*
		action := c.QueryParam("action")
		fmt.Println("[POST SecurityGroup requested action: " + action)
		if action == "create" {
			fmt.Println("[Creating SecurityGroup]")
			content, _ := createSecurityGroup(nsId, u)
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
	content, responseCode, body, err := createSecurityGroup(nsId, u)
	if err != nil {
		cblog.Error(err)
		/*
			mapA := map[string]string{
				"message": "Failed to create a SecurityGroup"}
		*/
		return c.JSONBlob(responseCode, body)
	}
	return c.JSON(http.StatusCreated, content)
}

func RestGetSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("securityGroupId")

	content := securityGroupInfo{}

	fmt.Println("[Get securityGroup for id]" + id)
	key := common.GenResourceKey(nsId, "securityGroup", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func RestGetAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		SecurityGroup []securityGroupInfo `json:"securityGroup"`
	}

	securityGroupList := getResourceList(nsId, "securityGroup")

	for _, v := range securityGroupList {

		key := common.GenResourceKey(nsId, "securityGroup", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		securityGroupTmp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &securityGroupTmp)
		securityGroupTmp.Id = v
		content.SecurityGroup = append(content.SecurityGroup, securityGroupTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutSecurityGroup(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("securityGroupId")
	forceFlag := c.QueryParam("force")

	//responseCode, body, err := delSecurityGroup(nsId, id, forceFlag)

	responseCode, body, err := delResource(nsId, "securityGroup", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to delete the securityGroup"}
		return c.JSONBlob(responseCode, body)
	}
	

	mapA := map[string]string{"message": "The securityGroup has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllSecurityGroup(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	securityGroupList := getResourceList(nsId, "securityGroup")

	for _, v := range securityGroupList {
		//responseCode, body, err := delSecurityGroup(nsId, v, forceFlag)

		responseCode, body, err := delResource(nsId, "securityGroup", v, forceFlag)
		if err != nil {
			cblog.Error(err)
			//mapA := map[string]string{"message": "Failed to delete the securityGroup"}
			return c.JSONBlob(responseCode, body)
		}
		
	}

	mapA := map[string]string{"message": "All securityGroups has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSecurityGroup(nsId string, u *securityGroupReq) (securityGroupInfo, int, []byte, error) {

	/* FYI
	type firewallRuleInfo struct {
		FromPort   string `json:"fromPort"`
		ToPort     string `json:"toPort"`
		IPProtocol string `json:"ipProtocol"`
		Direction  string `json:"direction"`
	}

	type securityGroupReq struct {
		//Id                 string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//VirtualNetworkId     string `json:"virtualNetworkId"`
		//CspSecurityGroupId   string `json:"cspSecurityGroupId"`
		CspSecurityGroupName string `json:"cspSecurityGroupName"`
		//ResourceGroupName    string `json:"resourceGroupName"`
		Description string `json:"description"`
		FirewallRules *[]firewallRuleInfo `json:"firewallRules"`

	}
	*/

	url := SPIDER_URL + "/securitygroup?connection_name=" + u.ConnectionName

	method := "POST"

	//payload := strings.NewReader("{ \"Name\": \"" + u.CspSecurityGroupName + "\"}")
	type SecurityReqInfo struct {
		Name          string
		SecurityRules *[]firewallRuleInfo
	}
	tempReq := SecurityReqInfo{}
	tempReq.Name = u.CspSecurityGroupName
	tempReq.SecurityRules = u.FirewallRules

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
		content := securityGroupInfo{}
		return content, res.StatusCode, nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := securityGroupInfo{}
		return content, res.StatusCode, body, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		cblog.Error(err)
		content := securityGroupInfo{}
		return content, res.StatusCode, body, err
	}

	/*
		type SecurityRuleInfo struct {
			FromPort   string
			ToPort     string
			IPProtocol string
			Direction  string
		}
	*/

	type SecurityInfo struct {
		Id            string
		Name          string
		SecurityRules *[]firewallRuleInfo //*[]SecurityRuleInfo

		KeyValueList []common.KeyValue
	}
	temp := SecurityInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	/* FYI
	type securityGroupInfo struct {
		Id             string `json:"id"`
		ConnectionName string `json:"connectionName"`
		//VirtualNetworkId   string `json:"virtualNetworkId"`
		CspSecurityGroupId   string `json:"cspSecurityGroupId"`
		CspSecurityGroupName string `json:"cspSecurityGroupName"`
		//ResourceGroupName  string `json:"resourceGroupName"`
		Description   string              `json:"description"`
		FirewallRules *[]firewallRuleInfo `json:"firewallRules"`
		KeyValueList  []common.KeyValue          `json:"keyValueList"`
	}
	*/

	content := securityGroupInfo{}
	content.Id = common.GenUuid()
	content.ConnectionName = u.ConnectionName
	content.CspSecurityGroupId = temp.Id
	content.CspSecurityGroupName = temp.Name // = u.CspSecurityGroupName
	content.Description = u.Description
	content.FirewallRules = temp.SecurityRules
	content.KeyValueList = temp.KeyValueList

	// cb-store
	fmt.Println("=========================== PUT createSecurityGroup")
	Key := common.GenResourceKey(nsId, "securityGroup", content.Id)
	/*
		mapA := map[string]string{
			"connectionName": content.ConnectionName,
			//"virtualNetworkId":   content.VirtualNetworkId,
			"cspSecurityGroupId":   content.CspSecurityGroupId,
			"cspSecurityGroupName": content.CspSecurityGroupName,
			//"resourceGroupName":  content.ResourceGroupName,
			"description": content.Description}
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
func getSecurityGroupList(nsId string) []string {

	fmt.Println("[Get securityGroups")
	key := "/ns/" + nsId + "/resources/securityGroup"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var securityGroupList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		securityGroupList = append(securityGroupList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/securityGroup/"))
		//}
	}
	for _, v := range securityGroupList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return securityGroupList

}
*/

/*
func delSecurityGroup(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete securityGroup] " + Id)

	key := genResourceKey(nsId, "securityGroup", Id)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)
	temp := securityGroupInfo{}
	unmarshalErr := json.Unmarshal([]byte(keyValue.Value), &temp)
	if unmarshalErr != nil {
		fmt.Println("unmarshalErr:", unmarshalErr)
	}
	fmt.Println("temp.CspSecurityGroupId: " + temp.CspSecurityGroupId)

	//url := SPIDER_URL + "/securitygroup?connection_name=" + temp.ConnectionName // for testapi.io
	url := SPIDER_URL + "/securitygroup/" + temp.CspSecurityGroupId + "?connection_name=" + temp.ConnectionName // for CB-Spider
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