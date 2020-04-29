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

type nsReq struct {
	//Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type nsInfo struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCIS API Proxy: Ns
func RestPostNs(c echo.Context) error {

	u := &nsReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Creating Ns]")
	content, err := createNs(u)
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
			check, _ := checkNs(nsId)

			if !check {
				return echo.NewHTTPError(http.StatusUnauthorized, "Not valid namespace")
			}
			return next(c)
		}
	}
}

func RestGetNs(c echo.Context) error {
	id := c.Param("nsId")

	content := nsInfo{}

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

}

func RestGetAllNs(c echo.Context) error {

	var content struct {
		//Name string     `json:"name"`
		Ns []nsInfo `json:"ns"`
	}

	nsList := getNsList()

	for _, v := range nsList {

		key := "/ns/" + v
		fmt.Println(key)
		keyValue, err := store.Get(key)
		if err != nil {
			cblog.Error(err)
			return err
		}

		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		nsTmp := nsInfo{}
		json.Unmarshal([]byte(keyValue.Value), &nsTmp)
		nsTmp.Id = v
		content.Ns = append(content.Ns, nsTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutNs(c echo.Context) error {
	return nil
}

func RestDelNs(c echo.Context) error {

	id := c.Param("nsId")

	err := delNs(id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The ns has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllNs(c echo.Context) error {

	nsList := getNsList()

	for _, v := range nsList {
		err := delNs(v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All nss has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createNs(u *nsReq) (nsInfo, error) {
	check, _ := checkNs(u.Name)

	if check {
		temp := nsInfo{}
		err := fmt.Errorf("The namespace " + u.Name + " already exists.")
		return temp, err
	}

	content := nsInfo{}
	//content.Id = GenUuid()
	content.Id = GenId(u.Name)
	content.Name = u.Name
	content.Description = u.Description

	// TODO here: implement the logic

	fmt.Println("=========================== PUT createNs")
	Key := "/ns/" + content.Id
	mapA := map[string]string{"name": content.Name, "description": content.Description}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

func getNsList() []string {

	fmt.Println("[List ns")
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

func delNs(Id string) error {

	fmt.Println("[Delete ns] " + Id)

	/*
		// Forbid deleting NS when there is at least one MCIS or one of resources.
		mcisList := mcis.getMcisList(Id)
		imageList := mcir.getResourceList(Id, "image")
		vNetList := mcir.getResourceList(Id, "vNet")
		publicIpList := mcir.getResourceList(Id, "publicIp")
		securityGroupList := mcir.getResourceList(Id, "securityGroup")
		specList := mcir.getResourceList(Id, "spec")
		sshKeyList := mcir.getResourceList(Id, "sshKey")
		subnetList := mcir.getResourceList(Id, "subnet")
		vNicList := mcir.getResourceList(Id, "vNic")

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
		        imports github.com/cloud-barista/cb-tumblebug/src/apiserver
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

func checkNs(Id string) (bool, error) {
	if Id == "" {
		err := fmt.Errorf("checkNs failed; nsId given is null.")
		return false, err
	}

	fmt.Println("[Check ns] " + Id)

	key := "/ns/" + Id
	//fmt.Println(key)

	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}

func RestCheckNs(c echo.Context) error {

	nsId := c.Param("nsId")

	exists, err := checkNs(nsId)

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
