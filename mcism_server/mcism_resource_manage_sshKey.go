package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type sshKeyReq struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Csp         string `json:"csp"`
	Fingerprint string `json:"fingerprint"`
	Username    string `json:"username"`
	PublicKey   string `json:"publicKey"`
	PrivateKey  string `json:"privateKey"`
	Description string `json:"description"`
}

type sshKeyInfo struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Csp         string `json:"csp"`
	Fingerprint string `json:"fingerprint"`
	Username    string `json:"username"`
	PublicKey   string `json:"publicKey"`
	PrivateKey  string `json:"privateKey"`
	Description string `json:"description"`
}

/* FYI
e.POST("/:nsId/resources/sshKey", restPostSshKey)
e.GET("/:nsId/resources/sshKey/:sshKeyId", restGetSshKey)
e.GET("/:nsId/resources/sshKey", restGetAllSshKey)
e.PUT("/:nsId/resources/sshKey/:sshKeyId", restPutSshKey)
e.DELETE("/:nsId/resources/sshKey/:sshKeyId", restDelSshKey)
e.DELETE("/:nsId/resources/sshKey", restDelAllSshKey)
*/

// MCIS API Proxy: SshKey
func restPostSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	u := &sshKeyReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST SshKey requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating SshKey]")
		createSshKey(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else if action == "register" {
		fmt.Println("[Registering SshKey]")
		registerSshKey(nsId, u)
		return c.JSON(http.StatusCreated, u)

	} else {
		mapA := map[string]string{"message": "You must specify: action=create or action=register"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("sshKeyId")

	content := sshKeyInfo{}
	/*
		var content struct {
			Id          string `json:"id"`
			Name        string `json:"name"`
			Csp         string `json:"csp"`
			Fingerprint string `json:"fingerprint"`
			Username    string `json:"username"`
			PublicKey   string `json:"publicKey"`
			PrivateKey  string `json:"privateKey"`
			Description string `json:"description"`
		}
	*/

	fmt.Println("[Get sshKey for id]" + id)
	key := "/ns/" + nsId + "/resources/sshKey/" + id
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		SshKey []sshKeyInfo `json:"sshKey"`
	}

	sshKeyList := getSshKeyList(nsId)

	for _, v := range sshKeyList {

		key := "/ns/" + nsId + "/resources/sshKey/" + v
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		sshKeyTmp := sshKeyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &sshKeyTmp)
		sshKeyTmp.Id = v
		content.SshKey = append(content.SshKey, sshKeyTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutSshKey(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelSshKey(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("sshKeyId")

	err := delSshKey(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the sshKey"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The sshKey has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllSshKey(c echo.Context) error {

	nsId := c.Param("nsId")

	sshKeyList := getSshKeyList(nsId)

	for _, v := range sshKeyList {
		err := delSshKey(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All sshKeys"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All sshKeys has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createSshKey(nsId string, u *sshKeyReq) {

	u.Id = genUuid()

	/* FYI
	type sshKeyReq struct {
		Id				string `json:"id"`
		Name			string `json:"name"`
		Csp				string `json:"csp"`
		Fingerprint		string `json:"fingerprint"`
		Username		string `json:"username"`
		PublicKey		string `json:"publicKey"`
		PrivateKey		string `json:"privateKey"`
		Description		string `json:"description"`
	}
	*/

	// cb-store
	fmt.Println("=========================== PUT createSshKey")
	Key := "/ns/" + nsId + "/resources/sshKey/" + u.Id
	mapA := map[string]string{"name": u.Name, "csp": u.Csp, "fingerprint": u.Fingerprint, "username": u.Username,
		"publicKey": u.PublicKey, "privateKey": u.PrivateKey, "description": u.Description}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func registerSshKey(nsId string, u *sshKeyReq) {

	u.Id = genUuid()

	// TODO here: implement the logic
	// - Fetch the sshKey info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerSshKey")
	Key := "/ns/" + nsId + "/resources/sshKey/" + u.Id
	mapA := map[string]string{"name": u.Name, "csp": u.Csp, "fingerprint": u.Fingerprint, "username": u.Username,
		"publicKey": u.PublicKey, "privateKey": u.PrivateKey, "description": u.Description}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func getSshKeyList(nsId string) []string {

	fmt.Println("[Get sshKeys")
	key := "/ns/" + nsId + "/resources/sshKey"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var sshKeyList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		sshKeyList = append(sshKeyList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/sshKey/"))
		//}
	}
	for _, v := range sshKeyList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return sshKeyList

}

func delSshKey(nsId string, Id string) error {

	fmt.Println("[Delete sshKey] " + Id)

	key := "/ns/" + nsId + "/resources/sshKey/" + Id
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
