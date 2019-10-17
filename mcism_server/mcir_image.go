package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

// Ref: https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/new-resources/ImageHandler.go
/* 2019-10-16
type ImageReqInfo struct {
	Name string
	Id   string
	// @todo
}

type ImageInfo struct {
     Id   string
     Name string
     GuestOS string // Windows7, Ubuntu etc.
     Status string  // available, unavailable

     KeyValueList []KeyValue
}

type ImageHandler interface {
	CreateImage(imageReqInfo ImageReqInfo) (ImageInfo, error)
	ListImage() ([]*ImageInfo, error)
	GetImage(imageID string) (ImageInfo, error)
	DeleteImage(imageID string) (bool, error)
}
*/

type imageReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	Description    string `json:"description"`
}

type imageInfo struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	Description    string `json:"description"`

	GuestOS string `json:"guestOS"` // Windows7, Ubuntu etc.
	Status  string `json:"status"`  // available, unavailable

}

/* FYI
g.POST("/:nsId/resources/image", restPostImage)
g.GET("/:nsId/resources/image/:imageId", restGetImage)
g.GET("/:nsId/resources/image", restGetAllImage)
g.PUT("/:nsId/resources/image/:imageId", restPutImage)
g.DELETE("/:nsId/resources/image/:imageId", restDelImage)
g.DELETE("/:nsId/resources/image", restDelAllImage)
*/

// MCIS API Proxy: Image
func restPostImage(c echo.Context) error {

	nsId := c.Param("nsId")

	action := c.QueryParam("action")
	fmt.Println("[POST Image requested action: " + action)
	/*
		if action == "create" {
			fmt.Println("[Creating Image]")
			content, _ := createImage(nsId, u)
			return c.JSON(http.StatusCreated, content)

		} else */if action == "registerWithInfo" {
		fmt.Println("[Registering Image]")
		u := &imageInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, _ := registerImageWithInfo(nsId, u)
		return c.JSON(http.StatusCreated, content)

	} else {
		mapA := map[string]string{"message": "You must specify: action=registerWithInfo"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func restGetImage(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("imageId")

	content := imageInfo{}

	fmt.Println("[Get image for id]" + id)
	key := genResourceKey(nsId, "image", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.Id = id // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllImage(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Image []imageInfo `json:"image"`
	}

	imageList := getImageList(nsId)

	for _, v := range imageList {

		key := genResourceKey(nsId, "image", v)
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		imageTmp := imageInfo{}
		json.Unmarshal([]byte(keyValue.Value), &imageTmp)
		imageTmp.Id = v
		content.Image = append(content.Image, imageTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutImage(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func restDelImage(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("imageId")

	err := delImage(nsId, id)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the image"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The image has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllImage(c echo.Context) error {

	nsId := c.Param("nsId")

	imageList := getImageList(nsId)

	for _, v := range imageList {
		err := delImage(nsId, v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All images"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All images has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

/*
func createImage(nsId string, u *imageReq) (imageInfo, error) {

	content := imageInfo{}
	content.Id = genUuid()
	content.Name = u.Name
	content.CreationDate = u.CreationDate
	content.ConnectionName = u.ConnectionName
	content.CspImageId = u.CspImageId
	content.Description = u.Description

	// TODO here: implement the logic
	// Option 1. Let the user upload an image file.
	// Option 2. Let the user specify the URL of an image file.
	// Option 3. Let the user snapshot specific VM for the new image file.

	// cb-store
	fmt.Println("=========================== PUT createImage")
	Key := genResourceKey(nsId, "image", content.Id)
	mapA := map[string]string{
		"name":           content.Name,
		"creationDate":   content.CreationDate,
		"connectionName": content.ConnectionName,
		"cspImageId":     content.CspImageId,
		"description":    content.Description,
		"guestOS":        content.GuestOS,
		"status":         content.Status}
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
*/

/* Optional
func registerImageWithId(nsId string, u *imageReq) (imageInfo, error) {

	content := imageInfo{}
	content.Id = genUuid()
	content.Name = u.Name
	content.CreationDate = u.CreationDate
	content.ConnectionName = u.ConnectionName
	content.CspImageId = u.CspImageId
	content.Description = u.Description

	// TODO here: implement the logic
	// - Fetch the image info from CSP via CB-Spider.

	// cb-store
	fmt.Println("=========================== PUT registerImage")
	Key := genResourceKey(nsId, "image", content.Id)
	mapA := map[string]string{
		"name":           content.Name,
		"creationDate":   content.CreationDate,
		"connectionName": content.ConnectionName,
		"cspImageId":     content.CspImageId,
		"description":    content.Description,
		"guestOS":        content.GuestOS,
		"status":         content.Status}
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
*/

func registerImageWithInfo(nsId string, content *imageInfo) (imageInfo, error) {

	/* FYI
	type imageInfo struct {
		Id             string `json:"id"`
		Name           string `json:"name"`
		CreationDate   string `json:"creationDate"`
		ConnectionName string `json:"connectionName"`
		CspImageId     string `json:"cspImageId"`
		Description    string `json:"description"`

		GuestOS string `json:"guestOS"` // Windows7, Ubuntu etc.
		Status  string `json:"status"`  // available, unavailable

	}
	*/

	content.Id = genUuid()

	// cb-store
	fmt.Println("=========================== PUT registerImage")
	Key := genResourceKey(nsId, "image", content.Id)
	mapA := map[string]string{
		"name":           content.Name,
		"creationDate":   content.CreationDate,
		"connectionName": content.ConnectionName,
		"cspImageId":     content.CspImageId,
		"description":    content.Description,
		"guestOS":        content.GuestOS,
		"status":         content.Status}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return *content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return *content, nil
}

func getImageList(nsId string) []string {

	fmt.Println("[Get images")
	key := "/ns/" + nsId + "/resources/image"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var imageList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		imageList = append(imageList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/resources/image/"))
		//}
	}
	for _, v := range imageList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return imageList

}

func delImage(nsId string, Id string) error {

	fmt.Println("[Delete image] " + Id)

	key := genResourceKey(nsId, "image", Id)
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
