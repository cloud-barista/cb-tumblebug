package mcis

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
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		//return content.CspImageId // AWS
		return content.CspImageName // Azure
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
		//return content.CspNetworkId // contains CspSubnetId // AWS
		return content.CspNetworkName // contains CspSubnetId // Azure
	/*
		case "subnet":
			content := subnetInfo{}
			json.Unmarshal([]byte(keyValue.Value), &content)
			return content.CspSubnetId
	*/
	case "securityGroup":
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		//return content.CspSecurityGroupId // AWS
		return content.CspSecurityGroupName // Azure
	case "publicIp":
		content := mcirIds{}
		json.Unmarshal([]byte(keyValue.Value), &content)
		//return content.CspPublicIpId // AWS
		return content.CspPublicIpName // Azure
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