package mcis

import (
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"os"
	//"fmt"
	//"net/http"
	//"io/ioutil"
	//"strconv"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
	//"github.com/cloud-barista/cb-tumblebug/src/mcism"
	//"github.com/cloud-barista/cb-tumblebug/src/common"
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
func genUuid() string {
	return uuid.New().String()
}
*/

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
func getCspResourceId(nsId string, resourceType string, resourceId string) string {
	key := genResourceKey(nsId, resourceType, resourceId)
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

	cspType := getResourcesCspType(nsId, resourceType, resourceId)
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
			return content.CspVNicId
		default:
			return "invalid resourceType"
		}
	}	
}
*/

/*
func getVMsCspType(nsId string, mcisId string, vmId string) string {
	var content struct {
		Config_name        string   `json:"config_name"`
	}
	
	key := common.GenMcisKey(nsId, mcisId, vmId)
	keyValue, _ := store.Get(key)
	json.Unmarshal([]byte(keyValue.Value), &content)

	url := SPIDER_URL + "/connectionconfig/" + content.Config_name
	
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