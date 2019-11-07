package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"

	//"github.com/cloud-barista/cb-tumblebug/src/mcis"
)

type nsReq struct {
	Id          string `json:"id"`
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
	content, _ := createNs(u)
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
		mapA := map[string]string{"message": "Cannot find " + key}
		return c.JSON(http.StatusOK, &mapA)
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
		mapA := map[string]string{"message": "Failed to delete the ns"}
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
			mapA := map[string]string{"message": "Failed to delete All nss"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All nss has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createNs(u *nsReq) (nsInfo, error) {

	//u.Id = genUuid()
	content := nsInfo{}
	content.Id = GenUuid()
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

	fmt.Println("[Get nss")
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
	imageList := getImageList(Id)
	networkList := getNetworkList(Id)
	securityGroupList := getSecurityGroupList(Id)
	specList := getSpecList(Id)
	sshKeyList := getSshKeyList(Id)
	subnetList := getSubnetList(Id)

	if len(mcisList)+len(imageList)+len(networkList)+len(securityGroupList)+len(specList)+len(sshKeyList)+len(subnetList) > 0 {
		fmt.Println("Cannot delete NS " + Id + ", which is not empty. There exists at least one MCIS or one of resources.")
		fmt.Printf(" len(mcisList): %d \n len(imageList): %d \n len(networkList): %d \n len(securityGroupList): %d \n len(specList): %d \n len(sshKeyList): %d \n len(subnetList): %d \n",
			len(mcisList), len(imageList), len(networkList), len(securityGroupList), len(specList), len(sshKeyList), len(subnetList))

		err := store.Delete("") // TODO: We don't need to call store.Delete("") to make an error object.
		cblog.Error(err)
		return err // TODO: Pass len(****List) to restDelNs() / restDelAllNs()
	}
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

	fmt.Println("[Delete ns] " + Id)

	key := "/ns/" + Id
	fmt.Println(key)

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
