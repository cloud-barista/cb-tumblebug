package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	//uuid "github.com/google/uuid"
	"github.com/cloud-barista/cb-tumblebug/src/common"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
)

// CB-Store
var cblog *logrus.Logger
var store icbs.Store
var SPIDER_URL string

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
	SPIDER_URL = os.Getenv("SPIDER_URL")
}

func delResource(nsId string, resourceType string, resourceId string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete " + resourceType + "] " + resourceId)

	check, _ := checkResource(nsId, resourceType, resourceId)

	if !check {
		errString := "The " + resourceType + " " + resourceId + " does not exist."
		mapA := map[string]string{"message": errString}
		mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return http.StatusNotFound, mapB, err
	}

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
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
		err := store.Delete(key)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}
		return http.StatusOK, nil, nil
	case "spec":
		// delete spec info

		//get related recommend spec
		//keyValue, err := store.Get(key)
		content := SpecInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		/*
			if err != nil {
				cblog.Error(err)
				return http.StatusInternalServerError, nil, err
			}
		*/
		//

		err := store.Delete(key)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}

		//delete related recommend spec
		err = delRecommendSpec(nsId, resourceId, content.Num_vCPU, content.Mem_GiB, content.Storage_GiB)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}

		return http.StatusOK, nil, nil
	case "sshKey":
		temp := sshKeyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = SPIDER_URL + "/keypair/" + temp.CspSshKeyName //+ "?connection_name=" + temp.ConnectionName
	case "network":
		temp := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = SPIDER_URL + "/vnetwork/" + temp.CspNetworkName //+ "?connection_name=" + temp.ConnectionName
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
	*/
	case "securityGroup":
		temp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = SPIDER_URL + "/securitygroup/" + temp.CspSecurityGroupName //+ "?connection_name=" + temp.ConnectionName
	case "publicIp":
		temp := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = SPIDER_URL + "/publicip/" + temp.CspPublicIpName //+ "?connection_name=" + temp.ConnectionName
	case "vNic":
		temp := vNicInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		tempReq.ConnectionName = temp.ConnectionName
		url = SPIDER_URL + "/vnic/" + temp.CspVNicName //+ "?connection_name=" + temp.ConnectionName
	default:
		err := fmt.Errorf("invalid resourceType")
		return http.StatusBadRequest, nil, err
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

	/*
		if res.StatusCode == 400 || res.StatusCode == 401 {
			fmt.Println("HTTP Status code 400 Bad Request or 401 Unauthorized.")
			err := fmt.Errorf("HTTP Status code 400 Bad Request or 401 Unauthorized")
			cblog.Error(err)
			return res, err
		}

		// delete network info
		cbStoreDeleteErr := store.Delete(key)
		if cbStoreDeleteErr != nil {
			cblog.Error(cbStoreDeleteErr)
			return res, cbStoreDeleteErr
		}

		return res, nil
	*/

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

func getResourceList(nsId string, resourceType string) []string {

	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "network" ||
		resourceType == "subnet" ||
		resourceType == "securityGroup" ||
		resourceType == "publicIp" ||
		resourceType == "vNic" {
		// continue
	} else {
		return []string{"invalid resource type"}
	}

	fmt.Println("[Get " + resourceType + " list")
	key := "/ns/" + nsId + "/resources/" + resourceType
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
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

func checkResource(nsId string, resourceType string, resourceId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("checkResource failed; nsId given is null.")
		return false, err
	} else if resourceType == "" {
		err := fmt.Errorf("checkResource failed; resourceType given is null.")
		return false, err
	} else if resourceId == "" {
		err := fmt.Errorf("checkResource failed; resourceId given is null.")
		return false, err
	}

	// Check resourceType's validity
	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "network" ||
		resourceType == "subnet" ||
		resourceType == "securityGroup" ||
		resourceType == "publicIp" ||
		resourceType == "vNic" {
		// continue
	} else {
		err := fmt.Errorf("invalid resource type")
		return false, err
	}

	fmt.Println("[Check resource] " + resourceType + ", " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	//fmt.Println(key)

	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

// https://stackoverflow.com/questions/45139954/dynamic-struct-as-parameter-golang

type ReturnValue struct {
	CustomStruct interface{}
}

type NameOnly struct {
	Name string
}

func getNameFromStruct(u interface{}) string {
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
