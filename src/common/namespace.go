package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	//"github.com/cloud-barista/cb-tumblebug/src/mcir"
	//"github.com/cloud-barista/cb-tumblebug/src/mcis"
	"github.com/labstack/echo"
)

type NsReq struct {
	//Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NsInfo struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCIS API Proxy: Ns
func RestPostNs(c echo.Context) error {

	u := &NsReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Creating Ns]")
	content, err := CreateNs(u)
	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": "Failed to create the ns " + u.Name}
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)

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

func RestGetNs(c echo.Context) error {
	id := c.Param("nsId")

	/*
		content := NsInfo{}

		fmt.Println("[Get ns for id]" + id)
		key := "/ns/" + id
		fmt.Println(key)

		keyValue, err := store.Get(key)
		if err != nil {
			cblog.Error(err)
			return err
		}
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find the NS " + key}
			return c.JSON(http.StatusNotFound, &mapA)
		}

		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)
		content.Id = id // Optional. Can be omitted.

		return c.JSON(http.StatusOK, &content)
	*/

	res, err := GetNs(id)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find the namespace " + id}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

func RestGetAllNs(c echo.Context) error {

	var content struct {
		//Name string     `json:"name"`
		Ns []NsInfo `json:"ns"`
	}

	/*
		nsList := ListNsId()

		for _, v := range nsList {

			key := "/ns/" + v
			fmt.Println(key)
			keyValue, err := store.Get(key)
			if err != nil {
				cblog.Error(err)
				return err
			}

			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			nsTmp := NsInfo{}
			json.Unmarshal([]byte(keyValue.Value), &nsTmp)
			nsTmp.Id = v
			content.Ns = append(content.Ns, nsTmp)

		}
		fmt.Printf("content %+v\n", content)

		return c.JSON(http.StatusOK, &content)
	*/

	nsList, err := ListNs()
	if err != nil {
		mapA := map[string]string{"message": "Failed to list namespaces."}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	if nsList == nil {
		return c.JSON(http.StatusOK, &content)
	}

	// When err == nil && resourceList != nil
	content.Ns = nsList
	return c.JSON(http.StatusOK, &content)

}

func RestPutNs(c echo.Context) error {
	return nil
}

func RestDelNs(c echo.Context) error {

	id := c.Param("nsId")

	err := DelNs(id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The ns has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllNs(c echo.Context) error {
	/*
		nsList := ListNsId()

		for _, v := range nsList {
			err := DelNs(v)
			if err != nil {
				cblog.Error(err)
				mapA := map[string]string{"message": err.Error()}
				return c.JSON(http.StatusFailedDependency, &mapA)
			}
		}

		mapA := map[string]string{"message": "All nss has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	err := DelAllNs()
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := map[string]string{"message": "All namespaces has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
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
	content.Name = u.Name
	content.Description = u.Description

	// TODO here: implement the logic

	fmt.Println("CreateNs();")
	Key := "/ns/" + content.Id
	//mapA := map[string]string{"name": content.Name, "description": content.Description}
	Val, _ := json.Marshal(content)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
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

	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
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

	keyValue, err := store.GetList(key, true)

	if err != nil {
		cblog.Error(err)
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

	keyValue, _ := store.GetList(key, true)
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

	/*
		// Forbid deleting NS when there is at least one MCIS or one of resources.
		mcisList := mcis.ListMcisId(Id)
		imageList := mcir.ListResourceId(Id, "image")
		vNetList := mcir.ListResourceId(Id, "vNet")
		publicIpList := mcir.ListResourceId(Id, "publicIp")
		securityGroupList := mcir.ListResourceId(Id, "securityGroup")
		specList := mcir.ListResourceId(Id, "spec")
		sshKeyList := mcir.ListResourceId(Id, "sshKey")
		subnetList := mcir.ListResourceId(Id, "subnet")
		vNicList := mcir.ListResourceId(Id, "vNic")

		if len(mcisList)+len(imageList)+len(vNetList)+len(securityGroupList)+len(specList)+len(sshKeyList)+len(subnetList) > 0 {
			errString := "Cannot delete NS " + Id + ", which is not empty. There exists at least one MCIS or one of resources."
			errString += " \n len(mcisList): " + len(mcisList)
			errString += " \n len(imageList): " + len(imageList)
			errString += " \n len(vNetList): " + len(vNetList)
			errString += " \n len(publicIpList): " + len(publicIpList)
			errString += " \n len(securityGroupList): " + len(securityGroupList)
			errString += " \n len(specList): " + len(specList)
			errString += " \n len(sshKeyList): " + len(sshKeyList)
			errString += " \n len(subnetList): " + len(subnetList)
			errString += " \n len(vNicList): " + len(vNicList)

			err := fmt.Errorf(errString)
			cblog.Error(err)
			return err
		}
	*/

	/*
			import cycle not allowed
			package github.com/cloud-barista/cb-tumblebug/src
		        imports github.com/cloud-barista/cb-tumblebug/src/restapiserver
		        imports github.com/cloud-barista/cb-tumblebug/src/common
		        imports github.com/cloud-barista/cb-tumblebug/src/mcir
				imports github.com/cloud-barista/cb-tumblebug/src/common
	*/

	key := "/ns/" + Id
	fmt.Println(key)

	// delete ns info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
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

	keyValue, _ := store.Get(key)
	/*
		if err != nil {
			cblog.Error(err)
			return false, err
		}
	*/
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}

func RestCheckNs(c echo.Context) error {

	nsId := c.Param("nsId")

	exists, err := CheckNs(nsId)

	type JsonTemplate struct {
		Exists bool `json:exists`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}
