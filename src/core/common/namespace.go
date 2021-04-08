package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	//"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	//"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/labstack/echo/v4"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
)

type NsReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// swagger:response NsInfo
type NsInfo struct {
	Id          string `json:"id" example:"namespaceid01"`
	Name        string `json:"name" example:"namespacename01"`
	Description string `json:"description" example:"Description for this namespace"`
}

func NsValidation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			fmt.Printf("%v\n", "[API request!]")
			nsId := c.Param("nsId")
			if nsId == "" {
				return next(c)
			}
			nsId = ToLower(nsId)
			check, err := CheckNs(nsId)

			if !check || err != nil {
				return echo.NewHTTPError(http.StatusNotFound, "Not valid namespace")
			}
			return next(c)
		}
	}
}

func CreateNs(u *NsReq) (NsInfo, error) {
	lowerizedName := ToLower(u.Name)
	check, err := CheckNs(lowerizedName)

	if check {
		temp := NsInfo{}
		err := fmt.Errorf("CreateNs(); The namespace " + lowerizedName + " already exists.")
		return temp, err
	}

	if err != nil {
		temp := NsInfo{}
		CBLog.Error(err)
		return temp, err
	}

	content := NsInfo{}
	//content.Id = GenUuid()
	content.Id = lowerizedName
	content.Name = lowerizedName
	content.Description = u.Description

	// TODO here: implement the logic

	fmt.Println("CreateNs();")
	Key := "/ns/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	Val, _ := json.Marshal(content)
	err = CBStore.Put(string(Key), string(Val))
	if err != nil {
		CBLog.Error(err)
		return content, err
	}
	keyValue, _ := CBStore.Get(string(Key))
	fmt.Println("CreateNs(); ===========================")
	fmt.Println("CreateNs(); Key: " + keyValue.Key + "\nValue: " + keyValue.Value)
	fmt.Println("CreateNs(); ===========================")
	return content, nil
}

func GetNs(id string) (NsInfo, error) {

	res := NsInfo{}

	lowerizedId := ToLower(id)
	check, err := CheckNs(lowerizedId)

	if !check {
		errString := "The namespace " + lowerizedId + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return res, err
	}

	if err != nil {
		temp := NsInfo{}
		CBLog.Error(err)
		return temp, err
	}

	fmt.Println("[Get namespace] " + lowerizedId)
	key := "/ns/" + lowerizedId
	fmt.Println(key)

	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		return res, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	err = json.Unmarshal([]byte(keyValue.Value), &res)
	if err != nil {
		CBLog.Error(err)
		return res, err
	}
	return res, nil
}

func ListNs() ([]NsInfo, error) {
	fmt.Println("[List namespace]")
	key := "/ns"
	fmt.Println(key)

	keyValue, err := CBStore.GetList(key, true)
	keyValue = cbstore_utils.GetChildList(keyValue, key)

	if err != nil {
		CBLog.Error(err)
		return nil, err
	}
	if keyValue != nil {
		res := []NsInfo{}
		for _, v := range keyValue {
			tempObj := NsInfo{}
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

func ListNsId() []string {

	//fmt.Println("[List ns]")
	key := "/ns"
	//fmt.Println(key)

	keyValue, _ := CBStore.GetList(key, true)

	var nsList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		//nsList = append(nsList, strings.TrimPrefix(v.Key, "/ns/"))
		//}
		if !strings.Contains(v.Key, "mcis") && !strings.Contains(v.Key, "cpu") && !strings.Contains(v.Key, "resources") {
			nsList = append(nsList, strings.TrimPrefix(v.Key, "/ns/"))
		}

	}
	//for _, v := range nsList {
	//	fmt.Println("<" + v + "> \n")
	//}
	//fmt.Println("===============================================")
	return nsList

}

func DelNs(Id string) error {

	lowerizedId := ToLower(Id)
	check, err := CheckNs(lowerizedId)

	if !check {
		errString := "The namespace " + lowerizedId + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	if err != nil {
		CBLog.Error(err)
		return err
	}

	fmt.Println("[Delete ns] " + lowerizedId)
	key := "/ns/" + lowerizedId
	fmt.Println(key)

	mcisList := GetChildIdList(key + "/mcis")
	imageList := GetChildIdList(key + "/resources/image")
	vNetList := GetChildIdList(key + "/resources/vNet")
	//subnetList := GetChildIdList(key + "/resources/subnet")
	//publicIpList := GetChildIdList(key + "/resources/publicIp")
	securityGroupList := GetChildIdList(key + "/resources/securityGroup")
	specList := GetChildIdList(key + "/resources/spec")
	sshKeyList := GetChildIdList(key + "/resources/sshKey")
	//vNicList := GetChildIdList(key + "/resources/vNic")

	if len(mcisList)+
		len(imageList)+
		len(vNetList)+
		//len(subnetList)
		len(securityGroupList)+
		len(specList)+
		len(sshKeyList) > 0 {
		errString := "Cannot delete NS " + lowerizedId + ", which is not empty. There exists at least one MCIS or one of resources."
		errString += " \n len(mcisList): " + strconv.Itoa(len(mcisList))
		errString += " \n len(imageList): " + strconv.Itoa(len(imageList))
		errString += " \n len(vNetList): " + strconv.Itoa(len(vNetList))
		//errString += " \n len(publicIpList): " + strconv.Itoa(len(publicIpList))
		errString += " \n len(securityGroupList): " + strconv.Itoa(len(securityGroupList))
		errString += " \n len(specList): " + strconv.Itoa(len(specList))
		errString += " \n len(sshKeyList): " + strconv.Itoa(len(sshKeyList))
		//errString += " \n len(subnetList): " + strconv.Itoa(len(subnetList))
		//errString += " \n len(vNicList): " + strconv.Itoa(len(vNicList))

		err := fmt.Errorf(errString)
		CBLog.Error(err)
		return err
	}

	// delete ns info
	err = CBStore.Delete(key)
	if err != nil {
		CBLog.Error(err)
		return err
	}

	return nil
}

func DelAllNs() error {
	fmt.Printf("DelAllNs() called;")

	nsIdList := ListNsId()

	if len(nsIdList) == 0 {
		return nil
	}

	for _, v := range nsIdList {
		err := DelNs(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckNs(Id string) (bool, error) {

	if Id == "" {
		err := fmt.Errorf("CheckNs failed; nsId given is null.")
		return false, err
	}

	lowerizedId := ToLower(Id)

	//fmt.Println("[Check ns] " + lowerizedId)

	key := "/ns/" + lowerizedId
	//fmt.Println(key)

	keyValue, _ := CBStore.Get(key)
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}
