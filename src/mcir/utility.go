package mcir

import (
	"encoding/json"
	uuid "github.com/google/uuid"
	"os"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
	//"github.com/cloud-barista/cb-tumblebug/src/mcism"
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

func genUuid() string {
	return uuid.New().String()
}

func genResourceKey(nsId string, resourceType string, resourceId string) string {
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
	/*
		case "subnet":
			content := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
	*/
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

func lookupKeyValueList(kvl []KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}