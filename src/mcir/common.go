package mcir

import (
	"strings"
	"strconv"
	"io/ioutil"
	"net/http"
	"fmt"
	"encoding/json"
	"os"

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

/*
func genResourceKey(nsId string, resourceType string, resourceId string) string { // can be moved to common/utility.go
	//resourceType = strings.ToLower(resourceType)

	if resourceType == "image" ||
		resourceType == "sshKey" ||
		resourceType == "spec" ||
		resourceType == "network" ||
		resourceType == "subnet" ||
		resourceType == "securityGroup" ||
		resourceType == "publicIp" ||
		resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalid_key"
	}

}
*/

/*
func getCspResourceId(nsId string, resourceType string, resourceId string) string {
	key := common.GenResourceKey(nsId, resourceType, resourceId)
	if key == "/invalid_key" {
		return "invalid nsId or resourceType or resourceId"
	}
	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return ""
	}
	if keyValue == nil {
		//cblog.Error(err)
		// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
		return ""
	}

	switch resourceType {
	case "image":
		content := imageInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspImageId
	case "sshKey":
		content := sshKeyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspSshKeyName
	case "spec":
		content := specInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.Name
	case "network":
		content := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspNetworkId // contains CspSubnetId
	// case "subnet":
	// 	content := subnetInfo{}
	// 	json.Unmarshal([]byte(keyValue.Value), &content)
	// 	return content.CspSubnetId
	case "securityGroup":
		content := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspSecurityGroupId
	case "publicIp":
		content := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		return content.CspPublicIpId
	case "vNic":
		content := vNicInfo{}
		err = json.Unmarshal([]byte(keyValue.Value), &content)
		if err != nil {
			cblog.Error(err)
			// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
			return ""
		}
		return content.CspVNicId
	default:
		return "invalid resourceType"
	}
}
*/

func delResource(nsId string, resourceType string, resourceId string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete " + resourceType + "] " + resourceId)

	key := common.GenResourceKey(nsId, resourceType, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)

	//cspType := common.GetResourcesCspType(nsId, resourceType, resourceId)

	var url string

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
		keyValue, err := store.Get(key)
		content := specInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}
		//

		err = store.Delete(key)
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
		url = SPIDER_URL + "/keypair/" + temp.CspSshKeyName + "?connection_name=" + temp.ConnectionName
	case "network":
		temp := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnetwork/" + temp.CspNetworkName + "?connection_name=" + temp.ConnectionName
	/*
		case "subnet":
			temp := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
	*/
	case "securityGroup":
		temp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/securitygroup/" + temp.CspSecurityGroupName + "?connection_name=" + temp.ConnectionName
	case "publicIp":
		temp := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/publicip/" + temp.CspPublicIpName + "?connection_name=" + temp.ConnectionName
	case "vNic":
		temp := vNicInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnic/" + temp.CspVNicName + "?connection_name=" + temp.ConnectionName
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

/*
func delResourceById(nsId string, resourceType string, resourceId string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete " + resourceType + "] " + resourceId)

	key := genResourceKey(nsId, resourceType, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)

	var url string

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
		keyValue, err := store.Get(key)
		content := specInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}
		//

		err = store.Delete(key)
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
		url = SPIDER_URL + "/keypair/" + temp.CspSshKeyName + "?connection_name=" + temp.ConnectionName
	case "network":
		temp := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnetwork/" + temp.CspNetworkId + "?connection_name=" + temp.ConnectionName
	case "securityGroup":
		temp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/securitygroup/" + temp.CspSecurityGroupId + "?connection_name=" + temp.ConnectionName
	case "publicIp":
		temp := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/publicip/" + temp.CspPublicIpId + "?connection_name=" + temp.ConnectionName
	case "vNic":
		temp := vNicInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnic/" + temp.CspVNicId + "?connection_name=" + temp.ConnectionName
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

func delResourceByName(nsId string, resourceType string, resourceId string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete " + resourceType + "] " + resourceId)

	key := genResourceKey(nsId, resourceType, resourceId)
	fmt.Println("key: " + key)

	keyValue, _ := store.Get(key)
	fmt.Println("keyValue: " + keyValue.Key + " / " + keyValue.Value)

	var url string

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
		keyValue, err := store.Get(key)
		content := specInfo{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		if err != nil {
			cblog.Error(err)
			return http.StatusInternalServerError, nil, err
		}
		//

		err = store.Delete(key)
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
		url = SPIDER_URL + "/keypair/" + temp.CspSshKeyName + "?connection_name=" + temp.ConnectionName
	case "network":
		temp := networkInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnetwork/" + temp.CspNetworkName + "?connection_name=" + temp.ConnectionName
	case "securityGroup":
		temp := securityGroupInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/securitygroup/" + temp.CspSecurityGroupName + "?connection_name=" + temp.ConnectionName
	case "publicIp":
		temp := publicIpInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/publicip/" + temp.CspPublicIpName + "?connection_name=" + temp.ConnectionName
	case "vNic":
		temp := vNicInfo{}
		json.Unmarshal([]byte(keyValue.Value), &temp)
		url = SPIDER_URL + "/vnic/" + temp.CspVNicName + "?connection_name=" + temp.ConnectionName
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
			resourceList = append(resourceList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/" + resourceType + "/"))
		//}
	}
	for _, v := range resourceList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return resourceList

}
