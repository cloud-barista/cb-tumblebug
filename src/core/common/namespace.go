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
			check, _ := CheckNs(nsId)

			if !check {
				return echo.NewHTTPError(http.StatusUnauthorized, "Not valid namespace")
			}
			return next(c)
		}
	}
}

func CreateNs(u *NsReq) (NsInfo, error) {
	check, _ := CheckNs(u.Name)

	if check {
		temp := NsInfo{}
		err := fmt.Errorf("CreateNs(); The namespace " + u.Name + " already exists.")
		return temp, err
	}

	content := NsInfo{}
	//content.Id = GenUuid()
	content.Id = GenId(u.Name)
	content.Name = GenId(u.Name)
	content.Description = u.Description

	// TODO here: implement the logic

	fmt.Println("CreateNs();")
	Key := "/ns/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	Val, _ := json.Marshal(content)
	err := CBStore.Put(string(Key), string(Val))
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
	fmt.Println("[Get namespace] " + id)

	res := NsInfo{}

	check, _ := CheckNs(id)
	if !check {
		errString := "The namespace " + id + " does not exist."
		//mapA := map[string]string{"message": errString}
		//mapB, _ := json.Marshal(mapA)
		err := fmt.Errorf(errString)
		return res, err
	}

	key := "/ns/" + id
	fmt.Println(key)

	keyValue, err := CBStore.Get(key)
	if err != nil {
		CBLog.Error(err)
		return res, err
	}

	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &res)
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
			json.Unmarshal([]byte(v.Value), &tempObj)
			res = append(res, tempObj)
		}
		return res, nil
		//return true, nil
	}
	return nil, nil // When err == nil && keyValue == nil
}

func ListNsId() []string {

	fmt.Println("[List ns]")
	key := "/ns"
	fmt.Println(key)

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
	for _, v := range nsList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return nsList

}

func DelNs(Id string) error {

	fmt.Println("[Delete ns] " + Id)

	check, _ := CheckNs(Id)
	if !check {
		errString := "The namespace " + Id + " does not exist."
		err := fmt.Errorf(errString)
		return err
	}

	key := "/ns/" + Id
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
		errString := "Cannot delete NS " + Id + ", which is not empty. There exists at least one MCIS or one of resources."
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
	err := CBStore.Delete(key)
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

	fmt.Println("[Check ns] " + Id)

	key := "/ns/" + Id
	//fmt.Println(key)

	keyValue, _ := CBStore.Get(key)
	/*
		if err != nil {
			CBLog.Error(err)
			return false, err
		}
	*/
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}
