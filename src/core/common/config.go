package common

import (
	"encoding/json"
	"fmt"
	"strings"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
)

// swagger:request ConfigReq
type ConfigReq struct {
	Name  string `json:"name" example:"SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}

// swagger:response ConfigInfo
type ConfigInfo struct {
	Id    string `json:"id" example:"SPIDER_REST_URL"`
	Name  string `json:"name" example:"SPIDER_REST_URL"`
	Value string `json:"value" example:"http://localhost:1024/spider"`
}

func UpdateConfig(u *ConfigReq) (ConfigInfo, error) {
	//_, lowerizedName, _ := LowerizeAndCheckConfig(u.Name)
	lowerizedName := ToLower(u.Name)

	content := ConfigInfo{}
	content.Id = lowerizedName
	content.Name = lowerizedName
	content.Value = u.Value

	key := "/config/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	val, _ := json.Marshal(content)
	err = CBStore.Put(string(key), string(val))
	if err != nil {
		CBLog.Error(err)
		return content, err
	}
	keyValue, _ := CBStore.Get(string(key))
	fmt.Println("UpdateConfig(); ===========================")
	fmt.Println("UpdateConfig(); Key: " + keyValue.Key + "\nValue: " + keyValue.Value)
	fmt.Println("UpdateConfig(); ===========================")

	UpdateEnv(content.Id)

	return content, nil
}

func UpdateEnv(id string) error {

	/*
		common.SPIDER_REST_URL = common.NVL(os.Getenv("SPIDER_REST_URL"), "http://localhost:1024/spider")
		common.DRAGONFLY_REST_URL = common.NVL(os.Getenv("DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
		common.DB_URL = common.NVL(os.Getenv("DB_URL"), "localhost:3306")
		common.DB_DATABASE = common.NVL(os.Getenv("DB_DATABASE"), "cb_tumblebug")
		common.DB_USER = common.NVL(os.Getenv("DB_USER"), "cb_tumblebug")
		common.DB_PASSWORD = common.NVL(os.Getenv("DB_PASSWORD"), "cb_tumblebug")
	*/

	lowStrSPIDER_REST_URL := ToLower(StrSPIDER_REST_URL)
	lowStrDRAGONFLY_REST_URL := ToLower(StrDRAGONFLY_REST_URL)
	lowStrDB_URL := ToLower(StrDB_URL)
	lowStrDB_DATABASE := ToLower(StrDB_DATABASE)
	lowStrDB_USER := ToLower(StrDB_USER)
	lowStrDB_PASSWORD := ToLower(StrDB_PASSWORD)
	lowStrAUTOCONTROL_DURATION_MS := ToLower(StrAUTOCONTROL_DURATION_MS)

	configInfo, err := GetConfig(id)
	if err != nil {
		//CBLog.Error(err)
		return err
	}

	switch id {
	case lowStrSPIDER_REST_URL:
		SPIDER_REST_URL = configInfo.Value
		fmt.Println("<SPIDER_REST_URL> " + SPIDER_REST_URL)
	case lowStrDRAGONFLY_REST_URL:
		DRAGONFLY_REST_URL = configInfo.Value
		fmt.Println("<DRAGONFLY_REST_URL> " + DRAGONFLY_REST_URL)
	case lowStrDB_URL:
		DB_URL = configInfo.Value
		fmt.Println("<DB_URL> " + DB_URL)
	case lowStrDB_DATABASE:
		DB_DATABASE = configInfo.Value
		fmt.Println("<DB_DATABASE> " + DB_DATABASE)
	case lowStrDB_USER:
		DB_USER = configInfo.Value
		fmt.Println("<DB_USER> " + DB_USER)
	case lowStrDB_PASSWORD:
		DB_PASSWORD = configInfo.Value
		fmt.Println("<DB_PASSWORD> " + DB_PASSWORD)
	case lowStrAUTOCONTROL_DURATION_MS:
		AUTOCONTROL_DURATION_MS = configInfo.Value
		fmt.Println("<AUTOCONTROL_DURATION_MS> " + AUTOCONTROL_DURATION_MS)
	default:

	}

	return nil
}

func GetConfig(id string) (ConfigInfo, error) {

	res := ConfigInfo{}

	//check, lowerizedId, err := LowerizeAndCheckConfig(id)
	lowerizedId := ToLower(id)
	check, err := CheckConfig(lowerizedId)

	if !check {
		errString := "The config " + lowerizedId + " does not exist."
		err := fmt.Errorf(errString)
		return res, err
	}

	if err != nil {
		temp := ConfigInfo{}
		CBLog.Error(err)
		return temp, err
	}

	fmt.Println("[Get config] " + lowerizedId)
	key := "/config/" + lowerizedId
	//fmt.Println(key)

	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		return res, err
	}

	fmt.Println("<" + keyValue.Key + "> " + keyValue.Value)
	//fmt.Println("===============================================")

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		CBLog.Error(err)
		return res, err
	}
	return res, nil
}

func ListConfig() ([]ConfigInfo, error) {
	fmt.Println("[List config]")
	key := "/config"
	fmt.Println(key)

	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		CBLog.Error(err)
		return nil, err
	}
	if keyValue != nil {
		res := []ConfigInfo{}
		for _, v := range keyValue {
			tempObj := ConfigInfo{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				CBLog.Error(err)
				return nil, err
			}
			res = append(res, tempObj)
		}
		return res, nil
		//return true, nil
	}
	return nil, nil // When err == nil && keyValue == nil
}

func ListConfigId() []string {

	fmt.Println("[List config]")
	key := "/config"
	fmt.Println(key)

	keyValue, _ := CBStore.GetList(key, true)

	var configList []string
	for _, v := range keyValue {
		configList = append(configList, strings.TrimPrefix(v.Key, "/config/"))
	}
	for _, v := range configList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return configList

}

func DelAllConfig() error {
	fmt.Printf("DelAllConfig() called;")

	key := "/config"
	fmt.Println(key)
	keyValue, _ := CBStore.GetList(key, true)

	if len(keyValue) == 0 {
		return nil
	}

	for _, v := range keyValue {
		err = CBStore.Delete(v.Key)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
func LowerizeAndCheckConfig(Id string) (bool, string, error) {

	if Id == "" {
		err := fmt.Errorf("CheckConfig failed; configId given is null.")
		return false, "", err
	}

	lowerizedId := ToLower(Id)

	fmt.Println("[Check config] " + lowerizedId)

	key := "/config/" + lowerizedId
	//fmt.Println(key)

	keyValue, _ := CBStore.Get(key)
	if keyValue != nil {
		return true, lowerizedId, nil
	}
	return false, lowerizedId, nil
}
*/

func CheckConfig(Id string) (bool, error) {

	if Id == "" {
		err := fmt.Errorf("CheckConfig failed; configId given is null.")
		return false, err
	}

	lowerizedId := ToLower(Id)

	//fmt.Println("[Check config] " + lowerizedId)

	key := "/config/" + lowerizedId
	//fmt.Println(key)

	keyValue, _ := CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}
