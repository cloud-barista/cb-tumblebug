package mcir

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	//uuid "github.com/google/uuid"
	"github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/go-resty/resty/v2"
	"github.com/xwb1989/sqlparser"

	// CB-Store
	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CB-Store
//var cblog *logrus.Logger
//var store icbs.Store

//var SPIDER_REST_URL string

func init() {
	//cblog = config.Cblogger
	//store = cbstore.GetStore()
	//SPIDER_REST_URL = os.Getenv("SPIDER_REST_URL")
}

func DelAllResources(nsId string, resourceType string, forceFlag string) error {

	nsId = common.GenId(nsId)

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

	//check, lowerizedResourceId, err := LowerizeAndCheckResource(nsId, resourceType, resourceId)
	//resourceId = lowerizedResourceId
	nsId = common.ToLower(nsId)
	resourceId = common.ToLower(resourceId)
	check, err := CheckResource(nsId, resourceType, resourceId)

	if check == false {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		//return http.StatusNotFound, mapB, err
		return err
	}

	if err != nil {
		common.CBLog.Error(err)
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

	if os.Getenv("SPIDER_CALL_METHOD") == "REST" {

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
			url = common.SPIDER_REST_URL + "/keypair/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
		case "vNet":
			temp := TbVNetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			tempReq.ConnectionName = temp.ConnectionName
			url = common.SPIDER_REST_URL + "/vpc/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
		case "securityGroup":
			temp := TbSecurityGroupInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)
			tempReq.ConnectionName = temp.ConnectionName
			url = common.SPIDER_REST_URL + "/securitygroup/" + temp.Name //+ "?connection_name=" + temp.ConnectionName
		/*
			case "subnet":
				temp := subnetInfo{}
				json.Unmarshal([]byte(keyValue.Value), &content)
				return content.CspSubnetId
			case "publicIp":
				temp := publicIpInfo{}
				json.Unmarshal([]byte(keyValue.Value), &temp)
				tempReq.ConnectionName = temp.ConnectionName
				url = common.SPIDER_REST_URL + "/publicip/" + temp.CspPublicIpName //+ "?connection_name=" + temp.ConnectionName
			case "vNic":
				temp := vNicInfo{}
				json.Unmarshal([]byte(keyValue.Value), &temp)
				tempReq.ConnectionName = temp.ConnectionName
				url = common.SPIDER_REST_URL + "/vnic/" + temp.CspVNicName //+ "?connection_name=" + temp.ConnectionName
		*/
		default:
			err := fmt.Errorf("invalid resourceType")
			//return http.StatusBadRequest, nil, err
			return err
		}

		fmt.Println("url: " + url)

		client := resty.New()

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(tempReq).
			//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
			//SetError(&AuthError{}).       // or SetError(AuthError{}).
			Delete(url)

		if err != nil {
			common.CBLog.Error(err)
			err := fmt.Errorf("an error occurred while requesting to CB-Spider")
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
			err := common.CBStore.Delete(key)
			if err != nil {
				common.CBLog.Error(err)
				return res, err
			}

			return res, nil
		*/

		fmt.Println("HTTP Status code " + strconv.Itoa(resp.StatusCode()))
		switch {
		case forceFlag == "true":
			url += "?force=true"
			fmt.Println("forceFlag == true; url: " + url)

			_, err := client.R().
				SetHeader("Content-Type", "application/json").
				SetBody(tempReq).
				//SetResult(&SpiderSpecInfo{}). // or SetResult(AuthSuccess{}).
				//SetError(&AuthError{}).       // or SetError(AuthError{}).
				Delete(url)

			if err != nil {
				common.CBLog.Error(err)
				err := fmt.Errorf("an error occurred while requesting to CB-Spider")
				return err
			}

			err = common.CBStore.Delete(key)
			if err != nil {
				common.CBLog.Error(err)
				//return res.StatusCode, body, err
				return err
			}
			//return res.StatusCode, body, nil
			return nil
		case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
			err := fmt.Errorf(string(resp.Body()))
			common.CBLog.Error(err)
			//return res.StatusCode, body, err
			return err
		default:
			err := common.CBStore.Delete(key)
			if err != nil {
				common.CBLog.Error(err)
				//return res.StatusCode, body, err
				return err
			}
			//return res.StatusCode, body, nil
			return nil
		}

	} else {

		// CCM API 설정
		ccm := api.NewCloudResourceHandler()
		err := ccm.SetConfigPath(os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml")
		if err != nil {
			common.CBLog.Error("ccm failed to set config : ", err)
			return err
		}
		err = ccm.Open()
		if err != nil {
			common.CBLog.Error("ccm api open failed : ", err)
			return err
		}
		defer ccm.Close()

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
			content := TbSpecInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)

			err := common.CBStore.Delete(key)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

			//delete related recommend spec
			err = DelRecommendSpec(nsId, resourceId, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB)
			if err != nil {
				common.CBLog.Error(err)
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
			return nil

		case "sshKey":
			temp := TbSshKeyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)

			_, err := ccm.DeleteKeyByParam(temp.ConnectionName, temp.Name, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case "vNet":
			temp := TbVNetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)

			_, err := ccm.DeleteVPCByParam(temp.ConnectionName, temp.Name, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		case "securityGroup":
			temp := TbSecurityGroupInfo{}
			json.Unmarshal([]byte(keyValue.Value), &temp)

			_, err := ccm.DeleteSecurityByParam(temp.ConnectionName, temp.Name, forceFlag)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}

		default:
			err := fmt.Errorf("invalid resourceType")
			return err
		}

		err = common.CBStore.Delete(key)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}
		return nil

	}
}

func ListResourceId(nsId string, resourceType string) []string {

	nsId = common.GenId(nsId)

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

	nsId = common.GenId(nsId)

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
	keyValue = cbstore_utils.GetChildList(keyValue, key)

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

func GetInUseCount(nsId string, resourceType string, resourceId string) (int8, error) {

	//check, lowerizedResourceId, err := LowerizeAndCheckResource(nsId, resourceType, resourceId)
	//resourceId = lowerizedResourceId
	nsId = common.ToLower(nsId)
	resourceId = common.ToLower(resourceId)
	check, err := CheckResource(nsId, resourceType, resourceId)

	if check == false {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return -1, err
	}

	if err != nil {
		common.CBLog.Error(err)
		return -1, err
	}
	fmt.Println("[Get count] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	//fmt.Println(key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return -1, err
	}
	if keyValue != nil {
		inUseCount := int8(gjson.Get(keyValue.Value, "inUseCount").Uint())
		return inUseCount, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return -1, err
}

func SetInUseCount(nsId string, resourceType string, resourceId string, cmd string) (int8, error) {

	var to_be int8
	as_is, err := GetInUseCount(nsId, resourceType, resourceId)
	if err != nil {
		common.CBLog.Error(err)
		return -1, err
	}

	switch cmd {
	case "-1":
		switch {
		case as_is <= 0:
			errString := "inUseCount was " + strconv.Itoa(int(as_is)) + ". Cannot decrease."
			err = fmt.Errorf(errString)
			return -1, err
		default:
			to_be = as_is - 1
		}
	case "+1":
		switch {
		case as_is <= -1:
			errString := "inUseCount was " + strconv.Itoa(int(as_is)) + ". Cannot increase."
			err = fmt.Errorf(errString)
			return -1, err
		default:
			to_be = as_is + 1
		}
	default:
		errString := "cmd should be either -1 or +1."
		to_be = -1
		err = fmt.Errorf(errString)
		return to_be, err
	}

	nsId = common.ToLower(nsId)
	resourceId = common.ToLower(resourceId)
	/*
		check, err := CheckResource(nsId, resourceType, resourceId)

		if check == false {
			errString := "The " + resourceType + " " + resourceId + " does not exist."
			//mapA := map[string]string{"message": errString}
			//mapB, _ := json.Marshal(mapA)
			err := fmt.Errorf(errString)
			return -1, err
		}

		if err != nil {
			common.CBLog.Error(err)
			return -1, err
		}
	*/
	fmt.Println("[Set count] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	//fmt.Println(key)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return -1, err
	}
	if keyValue != nil {
		keyValue.Value, err = sjson.Set(keyValue.Value, "inUseCount", to_be)
		if err != nil {
			common.CBLog.Error(err)
			//return content, res.StatusCode, body, err
			return -1, err
		}
		err = common.CBStore.Put(key, keyValue.Value)
		if err != nil {
			common.CBLog.Error(err)
			//return content, res.StatusCode, body, err
			return -1, err
		}
		keyValue, _ := common.CBStore.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===========================")
		to_be = int8(gjson.Get(keyValue.Value, "inUseCount").Uint())
		return to_be, nil
	}
	errString := "Cannot get " + resourceType + " " + resourceId + "."
	err = fmt.Errorf(errString)
	return -1, err
}

func GetResource(nsId string, resourceType string, resourceId string) (interface{}, error) {

	//check, lowerizedResourceId, err := LowerizeAndCheckResource(nsId, resourceType, resourceId)
	//resourceId = lowerizedResourceId
	nsId = common.ToLower(nsId)
	resourceId = common.ToLower(resourceId)
	check, err := CheckResource(nsId, resourceType, resourceId)

	if check == false {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return nil, err
	}

	if err != nil {
		common.CBLog.Error(err)
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

/*
func LowerizeAndCheckResource(nsId string, resourceType string, resourceId string) (bool, string, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckResource failed; nsId given is null.")
		return false, "", err
	} else if resourceType == "" {
		err := fmt.Errorf("CheckResource failed; resourceType given is null.")
		return false, "", err
	} else if resourceId == "" {
		err := fmt.Errorf("CheckResource failed; resourceId given is null.")
		return false, "", err
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
		return false, "", err
	}

	lowerizedNsId := common.GenId(nsId)
	lowerizedResourceId := common.GenId(resourceId)

	fmt.Println("[Check resource] " + resourceType + ", " + lowerizedResourceId)

	key := common.GenResourceKey(lowerizedNsId, resourceType, lowerizedResourceId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	if keyValue != nil {
		return true, lowerizedResourceId, nil
	}
	return false, lowerizedResourceId, nil

}
*/

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

	lowerizedNsId := common.ToLower(nsId)
	lowerizedResourceId := common.ToLower(resourceId)

	fmt.Println("[Check resource] " + resourceType + ", " + lowerizedResourceId)

	key := common.GenResourceKey(lowerizedNsId, resourceType, lowerizedResourceId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
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
