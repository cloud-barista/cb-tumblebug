package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	//uuid "github.com/google/uuid"
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/xwb1989/sqlparser"
	// CB-Store
)

// CB-Store
//var cblog *logrus.Logger
//var store icbs.Store

//var SPIDER_URL string

func init() {
	//cblog = config.Cblogger
	//store = cbstore.GetStore()
	//SPIDER_URL = os.Getenv("SPIDER_URL")
}

func DelAllResources(nsId string, resourceType string, forceFlag string) error {
	resourceIdList := ListResourceId(nsId, resourceType)

	if len(resourceIdList) == 0 {
		return nil
	}

	for _, v := range resourceIdList {
		err := DelResource(nsId, resourceType, v, forceFlag)
		if err != nil {
			return err
		}
	}
	return nil
}

//func DelResource(nsId string, resourceType string, resourceId string, forceFlag string) (int, []byte, error) {
func DelResource(nsId string, resourceType string, resourceId string, forceFlag string) error {

	//fmt.Println("[Delete " + resourceType + "] " + resourceId)
	fmt.Printf("DelResource() called; %s %s %s \n", nsId, resourceType, resourceId) // for debug

	check, _ := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		//return http.StatusNotFound, mapB, err
		return err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := common.CBStore.Get(key)
	/*
		if keyValue == nil {
			mapA := map[string]string{"message": "Failed to find the resource with given ID."}
			mapB, _ := json.Marshal(mapA)
			err := fmt.Errorf("Failed to find the resource with given ID.")
			return http.StatusNotFound, mapB, err
		}
	*/
	//fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	var url string

	// Create Req body
	type JsonTemplate struct {
		ConnectionName string
	}
	tempReq := JsonTemplate{}

	switch resourceType {
	case "image":
		// delete image info
		err := common.CBStore.Delete(key)
		if err != nil {
			common.CBLog.Error(err)
			//return http.StatusInternalServerError, nil, err
			return err
		}

		sql := "DELETE FROM `image` WHERE `id` = '" + resourceId + "';"
		fmt.Println("sql: " + sql)
		// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
		_, err = sqlparser.Parse(sql)
		if err != nil {
			//return
		}

		stmt, err := common.MYDB.Prepare(sql)
		if err != nil {
			fmt.Println(err.Error())
		}
		_, err = stmt.Exec()
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("Data deleted successfully..")
		}

		//return http.StatusOK, nil, nil
		return nil
	case "spec":
		// delete spec info

		//get related recommend spec
		//keyValue, err := common.CBStore.Get(key)
		content := TbSpecInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		/*
			if err != nil {
				common.CBLog.Error(err)
				return http.StatusInternalServerError, nil, err
			}
		*/
		//

		err := common.CBStore.Delete(key)
		if err != nil {
			common.CBLog.Error(err)
			//return http.StatusInternalServerError, nil, err
			return err
		}

		//delete related recommend spec
		err = DelRecommendSpec(nsId, resourceId, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB)
		if err != nil {
			common.CBLog.Error(err)
			//return http.StatusInternalServerError, nil, err
			return err
		}

		sql := "DELETE FROM `spec` WHERE `id` = '" + resourceId + "';"
		fmt.Println("sql: " + sql)
		// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
		_, err = sqlparser.Parse(sql)
		if err != nil {
			//return
		}

		stmt, err := common.MYDB.Prepare(sql)
		if err != nil {
			fmt.Println(err.Error())
		}
		_, err = stmt.Exec()
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Println("Data deleted successfully..")
		}

		//return http.StatusOK, nil, nil
		return nil
	case "sshKey":
		temp := TbSshKeyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = common.SPIDER_URL + "/keypair/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
	case "vNet":
		temp := TbVNetInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = common.SPIDER_URL + "/vpc/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
	case "securityGroup":
		temp := TbSecurityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = common.SPIDER_URL + "/securitygroup/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
		case "publicIp":
			temp := publicIpInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			tempReq.ConnectionName = temp.ConnectionName
			url = common.SPIDER_URL + "/publicip/" + temp.CspPublicIpName //+ "?connection_name=" + temp.ConnectionName
		case "vNic":
			temp := vNicInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			tempReq.ConnectionName = temp.ConnectionName
			url = common.SPIDER_URL + "/vnic/" + temp.CspVNicName //+ "?connection_name=" + temp.ConnectionName
	*/
	default:
		err := fmt.Errorf("invalid resourceType")
		//return http.StatusBadRequest, nil, err
		return err
	}

	fmt.Println("url: " + url)

	method := "DELETE"

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	//req, err := http.NewRequest(method, url, nil)
	payload, _ := json.MarshalIndent(tempReq, "", "  ")
	//fmt.Println("payload: " + string(payload))
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		common.CBLog.Error(err)
		//return res.StatusCode, nil, err
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		common.CBLog.Error(err)
		//return res.StatusCode, body, err
		return err
	}

	/*
		if res.StatusCode == 400 || res.StatusCode == 401 {
			fmt.Println("HTTP Status code 400 Bad Request or 401 Unauthorized.")
			err := fmt.Errorf("HTTP Status code 400 Bad Request or 401 Unauthorized")
			common.CBLog.Error(err)
			return res, err
		}

		// delete vNet info
		cbStoreDeleteErr := common.CBStore.Delete(key)
		if cbStoreDeleteErr != nil {
			common.CBLog.Error(cbStoreDeleteErr)
			return res, cbStoreDeleteErr
		}

		return res, nil
	*/

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case forceFlag == "true":
		cbStoreDeleteErr := common.CBStore.Delete(key)
		if cbStoreDeleteErr != nil {
			common.CBLog.Error(cbStoreDeleteErr)
			//return res.StatusCode, body, cbStoreDeleteErr
			return cbStoreDeleteErr
		}
		//return res.StatusCode, body, nil
		return nil
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		common.CBLog.Error(err)
		//return res.StatusCode, body, err
		return err
	default:
		cbStoreDeleteErr := common.CBStore.Delete(key)
		if cbStoreDeleteErr != nil {
			common.CBLog.Error(cbStoreDeleteErr)
			//return res.StatusCode, body, cbStoreDeleteErr
			return cbStoreDeleteErr
		}
		//return res.StatusCode, body, nil
		return nil
	}
}

func ListResourceId(nsId string, resourceType string) []string {

	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "vNet" ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == "securityGroup" {
		// continue
	} else {
		return []string{"invalid resource type"}
	}

	fmt.Println("[Get " + resourceType + " list")
	key := "/ns/" + nsId + "/resources/" + resourceType
	fmt.Println(key)

	keyValue, _ := common.CBStore.GetList(key, true)
	var resourceList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		resourceList = append(resourceList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/"+resourceType+"/"))
		//}
	}
	for _, v := range resourceList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return resourceList

}

func ListResource(nsId string, resourceType string) (interface{}, error) {
	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "vNet" ||
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" ||
		resourceType == "securityGroup" {
		// continue
	} else {
		errString := "Cannot list " + resourceType + "s."
		err := fmt.Errorf(errString)
		return nil, err
	}

	fmt.Println("[Get " + resourceType + " list")
	key := "/ns/" + nsId + "/resources/" + resourceType
	fmt.Println(key)

	keyValue, err := common.CBStore.GetList(key, true)

	if err != nil {
		common.CBLog.Error(err)
		/*
			fmt.Println("func ListResource; common.CBStore.GetList gave error")
			var resourceList []string
			for _, v := range keyValue {
				resourceList = append(resourceList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/"+resourceType+"/"))
			}
			for _, v := range resourceList {
				fmt.Println("<" + v + "> \n")
			}
			fmt.Println("===============================================")
		*/
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case "image":
			res := []TbImageInfo{}
			for _, v := range keyValue {
				tempObj := TbImageInfo{}
				json.Unmarshal([]byte(v.Value), &tempObj)
				res = append(res, tempObj)
			}
			return res, nil
		case "securityGroup":
			res := []TbSecurityGroupInfo{}
			for _, v := range keyValue {
				tempObj := TbSecurityGroupInfo{}
				json.Unmarshal([]byte(v.Value), &tempObj)
				res = append(res, tempObj)
			}
			return res, nil
		case "spec":
			res := []TbSpecInfo{}
			for _, v := range keyValue {
				tempObj := TbSpecInfo{}
				json.Unmarshal([]byte(v.Value), &tempObj)
				res = append(res, tempObj)
			}
			return res, nil
		case "sshKey":
			res := []TbSshKeyInfo{}
			for _, v := range keyValue {
				tempObj := TbSshKeyInfo{}
				json.Unmarshal([]byte(v.Value), &tempObj)
				res = append(res, tempObj)
			}
			return res, nil
		case "vNet":
			res := []TbVNetInfo{}
			for _, v := range keyValue {
				tempObj := TbVNetInfo{}
				json.Unmarshal([]byte(v.Value), &tempObj)
				res = append(res, tempObj)
			}
			return res, nil
		}

		//return true, nil
	}

	return nil, nil // When err == nil && keyValue == nil
}

func GetResource(nsId string, resourceType string, resourceId string) (interface{}, error) {

	check, _ := CheckResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return nil, err
	}
	fmt.Println("[Get resource] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	//fmt.Println(key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	if keyValue != nil {
		switch resourceType {
		case "image":
			res := TbImageInfo{}
			json.Unmarshal([]byte(keyValue.Value), &res)
			return res, nil
		case "securityGroup":
			res := TbSecurityGroupInfo{}
			json.Unmarshal([]byte(keyValue.Value), &res)
			return res, nil
		case "spec":
			res := TbSpecInfo{}
			json.Unmarshal([]byte(keyValue.Value), &res)
			return res, nil
		case "sshKey":
			res := TbSshKeyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &res)
			return res, nil
		case "vNet":
			res := TbVNetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &res)
			return res, nil
		}

		//return true, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return nil, err
}

func CheckResource(nsId string, resourceType string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, err
	}

	// Check resourceType's validity
	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "vNet" ||
		resourceType == "securityGroup" {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	fmt.Println("[Check resource] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	//fmt.Println(key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

/*
func convertSpiderResourceToTumblebugResource(resourceType string, i interface{}) (interface{}, error) {
	if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return nil, err
	}

	// Check resourceType's validity
	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "vNet" ||
		resourceType == "securityGroup" {
		//resourceType == "subnet" ||
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return nil, err
	}

}
*/

/*
func RestDelResource(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := c.Param("resourceType")
	resourceId := c.Param("resourceId")
	forceFlag := c.QueryParam("force")

	fmt.Printf("RestDelResource() called; %s %s %s \n", nsId, resourceType, resourceId) // for debug

	responseCode, _, err := DelResource(nsId, resourceType, resourceId, forceFlag)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + resourceId + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllResources(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := c.Param("resourceType")
	forceFlag := c.QueryParam("force")

	resourceList := ListResourceId(nsId, resourceType)

	if len(resourceList) == 0 {
		mapA := map[string]string{"message": "There is no " + resourceType + " element in this namespace."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		for _, v := range resourceList {
			responseCode, _, err := DelResource(nsId, resourceType, v, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				mapA := map[string]string{"message": err.Error()}
				return c.JSON(responseCode, &mapA)
			}

		}

		mapA := map[string]string{"message": "All " + resourceType + "s has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	}
}
*/

// https://stackoverflow.com/questions/45139954/dynamic-struct-as-parameter-golang

type ReturnValue struct {
	CustomStruct interface{}
}

type NameOnly struct {
	Name string
}

func GetNameFromStruct(u interface{}) string {
	var result = ReturnValue{CustomStruct: u}

	//fmt.Println(result)

	msg, ok := result.CustomStruct.(NameOnly)
	if ok {
		//fmt.Printf("Message1 is %s\n", msg.Name)
		return msg.Name
	} else {
		return ""
	}
}

//func createResource(nsId string, resourceType string, u interface{}) (interface{}, int, []byte, error) {
