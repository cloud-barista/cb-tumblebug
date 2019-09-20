package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo"
)

type imageReq struct {
	CbImageId    string `json:"cbImageId"`
	Name         string `json:"name"`
	CreationDate string `json:"creationDate"`
	Csp          string `json:"csp"`
	CspImageId   string `json:"cspImageId"`
	Description  string `json:"description"`
}

type imageInfo struct {
	CbImageId    string `json:"cbImageId"`
	Name         string `json:"name"`
	CreationDate string `json:"creationDate"`
	Csp          string `json:"csp"`
	CspImageId   string `json:"cspImageId"`
	Description  string `json:"description"`
}

/* FYI
e.POST("/resources/image", restPostImage)
e.GET("/resources/image/:id", restGetImage)
e.GET("/resources/image", restGetAllImage)
e.PUT("/resources/image/:id", restPutImage)
e.DELETE("/resources/image/:id", restDelImage)
e.DELETE("/resources/image", restDelAllImage)
*/

// MCIS API Proxy
func restPostImage(c echo.Context) error {

	u := &imageReq{}
	if err := c.Bind(u); err != nil {
		return err
	}

	action := c.QueryParam("action")
	fmt.Println("[POST Image requested action: " + action)
	if action == "create" {
		fmt.Println("[Creating Image]")
		createImage(u)
		return c.JSON(http.StatusCreated, u)

	} else { //if action == "register" {
		fmt.Println("[Registering Image]")
		registerImage(u)
		return c.JSON(http.StatusCreated, u)

	}

}

func restGetImage(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	cbImageId := c.Param("cbImageId")

	content := imageInfo{}
	/*
		var content struct {
			CbImageId    string `json:"cbImageId"`
			Name         string `json:"name"`
			CreationDate string `json:"creationDate"`
			Csp          string `json:"csp"`
			CspImageId   string `json:"cspImageId"`
			Description  string `json:"description"`
		}
	*/

	fmt.Println("[Get image for id]" + cbImageId)
	key := "/resources/image/" + cbImageId
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===============================================")

	json.Unmarshal([]byte(keyValue.Value), &content)
	content.CbImageId = cbImageId // Optional. Can be omitted.

	return c.JSON(http.StatusOK, &content)

}

func restGetAllImage(c echo.Context) error {

	var content struct {
		//Name string     `json:"name"`
		Image []imageInfo `json:"image"`
	}

	imageList := getImageList()

	for _, v := range imageList {

		key := "/resources/image/" + v
		fmt.Println(key)
		keyValue, _ := store.Get(key)
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		imageTmp := imageInfo{}
		json.Unmarshal([]byte(keyValue.Value), &imageTmp)
		imageTmp.CbImageId = v
		content.Image = append(content.Image, imageTmp)

	}
	fmt.Printf("content %+v\n", content)

	return c.JSON(http.StatusOK, &content)

}

func restPutImage(c echo.Context) error {
	return nil
}

func restDelImage(c echo.Context) error {

	cbImageId := c.Param("cbImageId")

	err := delImage(cbImageId)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the image"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The image has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func restDelAllImage(c echo.Context) error {

	imageList := getImageList()

	for _, v := range imageList {
		err := delImage(v)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All images"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All images has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func createImage(u *imageReq) {

	u.CbImageId = genUuid()

	// TODO here: implement the logic
	// Option 1. Let the user upload an image file.
	// Option 2. Let the user specify the URL of an image file.
	// Option 3. Let the user snapshot specific VM for the new image file.

	// cb-store
	fmt.Println("=========================== PUT createImage")
	Key := "/resources/image/" + u.CbImageId
	mapA := map[string]string{"name": u.Name, "description": u.Description, "creationDate": u.CreationDate, "csp": u.Csp, "cspImageId": u.CspImageId}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func registerImage(u *imageReq) {

	u.CbImageId = genUuid()

	// TODO here: implement the logic
	// - Fetch the image info from CSP.

	// cb-store
	fmt.Println("=========================== PUT registerImage")
	Key := "/resources/image/" + u.CbImageId
	mapA := map[string]string{"name": u.Name, "description": u.Description, "creationDate": u.CreationDate, "csp": u.Csp, "cspImageId": u.CspImageId}
	Val, _ := json.Marshal(mapA)
	err := store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

}

func getImageList() []string {

	fmt.Println("[Get images")
	key := "/resources/image"
	fmt.Println(key)

	keyValue, _ := store.GetList(key, true)
	var imageList []string
	for _, v := range keyValue {
		//if !strings.Contains(v.Key, "vm") {
		imageList = append(imageList, strings.TrimPrefix(v.Key, "/resources/image/"))
		//}
	}
	for _, v := range imageList {
		fmt.Println("<" + v + "> \n")
	}
	fmt.Println("===============================================")
	return imageList

}

func delImage(CbImageId string) error {

	fmt.Println("[Delete image] " + CbImageId)

	key := "/resources/image/" + CbImageId
	fmt.Println(key)

	// delete mcis info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
