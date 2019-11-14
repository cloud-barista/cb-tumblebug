package common

import (
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
)

type KeyValue struct {
	Key   string
	Value string
}

// CB-Store
var cblog *logrus.Logger
var store icbs.Store

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
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

