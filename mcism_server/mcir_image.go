package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	CspImageName   string `json:"cspImageName"`
	//CreationDate   string `json:"creationDate"`
	Description string `json:"description"`
}

type imageInfo struct {
	Id             string     `json:"id"`
	ConnectionName string     `json:"connectionName"`
	CspImageId     string     `json:"cspImageId"`
	CspImageName   string     `json:"cspImageName"`
	CreationDate   string     `json:"creationDate"`
	Description    string     `json:"description"`
	GuestOS        string     `json:"guestOS"` // Windows7, Ubuntu etc.
	Status         string     `json:"status"`  // available, unavailable
	KeyValueList   []KeyValue `json:"keyValueList"`
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

		} else */
	if action == "registerWithInfo" {
		fmt.Println("[Registering Image with info]")
		u := &imageInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, _ := registerImageWithInfo(nsId, u)
		return c.JSON(http.StatusCreated, content)
	} else if action == "registerWithName" {
		fmt.Println("[Registering Image with name]")
		u := &imageReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, _ := registerImageWithName(nsId, u)
		return c.JSON(http.StatusCreated, content)
	} else {
		mapA := map[string]string{"message": "You must specify: action=registerWithInfo or action=registerWithName"}
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

}
*/

func registerImageWithName(nsId string, u *imageReq) (imageInfo, error) {

	// Step 1. Create a temp `ImageReqInfo (from Spider)` object.
	type ImageReqInfo struct {
		Name string
		Id   string
		// @todo
	}
	tempReq := ImageReqInfo{}
	tempReq.Name = u.CspImageName
	tempReq.Id = u.CspImageId

	// Step 2. Send a req to Spider and save the response.
	url := SPIDER_URL + "/vmimage?connection_name=" + u.ConnectionName

	method := "POST"

	payload := strings.NewReader("{ \"Name\": \"" + u.CspImageName + "\"}")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	fmt.Println("Called mockAPI.")
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	fmt.Println(string(body))

	// jhseo 191016
	//var s = new(imageInfo)
	//s := imageInfo{}
	type ImageInfo struct {
		Id      string
		Name    string
		GuestOS string // Windows7, Ubuntu etc.
		Status  string // available, unavailable

		KeyValueList []KeyValue
	}
	temp := ImageInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	// Step 3. Create a temp `imageInfo (in this file)` object.
	/* FYI
	type imageInfo struct {
		Id             string     `json:"id"`
		ConnectionName string     `json:"connectionName"`
		CspImageId     string     `json:"cspImageId"`
		CspImageName   string     `json:"cspImageName"`
		CreationDate   string     `json:"creationDate"`
		Description    string     `json:"description"`
		GuestOS        string     `json:"guestOS"` // Windows7, Ubuntu etc.
		Status         string     `json:"status"`  // available, unavailable
		KeyValueList   []KeyValue `json:"keyValueList"`
	}
	*/
	content := imageInfo{}
	content.Id = genUuid()
	content.ConnectionName = u.ConnectionName
	content.CspImageId = temp.Id     // = u.CspImageId
	content.CspImageName = temp.Name // = u.CspImageName
	//content.CreationDate =
	content.Description = u.Description
	content.GuestOS = temp.GuestOS
	content.Status = temp.Status
	content.KeyValueList = temp.KeyValueList

	// Step 4. Store the metadata to CB-Store.
	fmt.Println("=========================== PUT registerImage")
	Key := genResourceKey(nsId, "image", content.Id)
	Val, _ := json.Marshal(content)
	cbStorePutErr := store.Put(string(Key), string(Val))
	if cbStorePutErr != nil {
		cblog.Error(cbStorePutErr)
		return content, cbStorePutErr
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	return content, nil
}

func registerImageWithInfo(nsId string, content *imageInfo) (imageInfo, error) {

	content.Id = genUuid()

	fmt.Println("=========================== PUT registerImage")
	Key := genResourceKey(nsId, "image", content.Id)
	Val, _ := json.Marshal(content)
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

	// delete image info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return err
	}

	return nil
}
