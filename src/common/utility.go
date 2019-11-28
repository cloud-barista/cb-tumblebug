package common

import (
	"os"
	//"encoding/json"

	uuid "github.com/google/uuid"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
	//"github.com/cloud-barista/cb-tumblebug/src/mcir"

	"encoding/json"
	"fmt"
	//"net/http"
	//"io/ioutil"
	//"strconv"
)

type KeyValue struct {
	Key   string
	Value string
}

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

var SPIDER_URL string

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
	SPIDER_URL = os.Getenv("SPIDER_URL")
}

// MCIS utilities

func GenUuid() string {
	return uuid.New().String()
}

func GenMcisKey(nsId string, mcisId string, vmId string) string {

	if vmId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId + "/vm/" + vmId
	} else if mcisId != "" {
		return "/ns/" + nsId + "/mcis/" + mcisId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}

}

func LookupKeyValueList(kvl []KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

func PrintJsonPretty(v interface{}) {
	prettyJSON, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		fmt.Printf("%+v\n", v)
	} else {
		fmt.Printf("%s\n", string(prettyJSON))
	}
}

func GenResourceKey(nsId string, resourceType string, resourceId string) string {
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

type mcirIds struct {
	CspImageId string
	CspImageName string
	CspSshKeyName string
	Name string // Spec
	CspNetworkId string
	CspNetworkName string
	CspSecurityGroupId string
	CspSecurityGroupName string
	CspPublicIpId string
	CspPublicIpName string
	CspVNicId string
	CspVNicName string

	ConnectionName string
}

/*
func GetResourcesCspType(nsId string, resourceType string, resourceId string) string {
	key := GenResourceKey(nsId, resourceType, resourceId)
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
	
	content := mcirIds{}
	json.Unmarshal([]byte(keyValue.Value), &content)
	
	url := SPIDER_URL + "/connectionconfig/" + content.ConnectionName
	
	method := "GET"

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
		return "http request error"
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		return "ioutil.ReadAll error"
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		cblog.Error(err)
		return "Cannot get VM's CSP type"
	default:
		
	}

	type ConnConfigInfo struct {
		ProviderName            string
	}

	temp := ConnConfigInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	return temp.ProviderName
}
*/

func GetCspResourceId(nsId string, resourceType string, resourceId string) string {
	key := GenResourceKey(nsId, resourceType, resourceId)
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

	/*
	cspType := GetResourcesCspType(nsId, resourceType, resourceId)
	if cspType == "AWS" {
		switch resourceType {
		case "image":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspImageId
		case "sshKey":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSshKeyName
		case "spec":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.Name
		case "network":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspNetworkId // contains CspSubnetId
		// case "subnet":
		// 	content := subnetInfo{}
		// 	json.Unmarshal([]byte(keyValue.Value), &content)
		// 	return content.CspSubnetId
		case "securityGroup":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSecurityGroupId
		case "publicIp":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspPublicIpId
		case "vNic":
			content := mcirIds{}
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
	} else {
		*/
		switch resourceType {
		case "image":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspImageName
		case "sshKey":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSshKeyName
		case "spec":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.Name
		case "network":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspNetworkName // contains CspSubnetId
		// case "subnet":
		// 	content := subnetInfo{}
		// 	json.Unmarshal([]byte(keyValue.Value), &content)
		// 	return content.CspSubnetId
		case "securityGroup":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSecurityGroupName
		case "publicIp":
			content := mcirIds{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspPublicIpName
		case "vNic":
			content := mcirIds{}
			err = json.Unmarshal([]byte(keyValue.Value), &content)
			if err != nil {
				cblog.Error(err)
				// if there is no matched value for the key, return empty string. Error will be handled in a parent fucntion
				return ""
			}
			return content.CspVNicName
		default:
			return "invalid resourceType"
		}
	//}	
}