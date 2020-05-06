package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

// 2020-04-03 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/ImageHandler.go

type SpiderImageReqInfo struct { // Spider
	ConnectionName string
	ReqInfo        ImageReqInfo
}

type ImageReqInfo struct { // Spider
	//IId   IID 	// {NameId, SystemId}
	Name string
	// @todo
}

type ImageInfo struct { // Spider
	//IId     IID    // {NameId, SystemId}
	Name    string
	GuestOS string // Windows7, Ubuntu etc.
	Status  string // available, unavailable

	KeyValueList []common.KeyValue
}

type imageReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	CspImageName   string `json:"cspImageName"`
	//CreationDate   string `json:"creationDate"`
	Description string `json:"description"`
}

type imageInfo struct {
	Id             string            `json:"id"`
	Name           string            `json:"name"`
	ConnectionName string            `json:"connectionName"`
	CspImageId     string            `json:"cspImageId"`
	CspImageName   string            `json:"cspImageName"`
	CreationDate   string            `json:"creationDate"`
	Description    string            `json:"description"`
	GuestOS        string            `json:"guestOS"` // Windows7, Ubuntu etc.
	Status         string            `json:"status"`  // available, unavailable
	KeyValueList   []common.KeyValue `json:"keyValueList"`
}

// MCIS API Proxy: Image
func RestPostImage(c echo.Context) error {

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
		content, err := registerImageWithInfo(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)
	} else if action == "registerWithId" {
		fmt.Println("[Registering Image with ID]")
		u := &imageReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		//content, responseCode, body, err := registerImageWithId(nsId, u)
		content, err := registerImageWithId(nsId, u)
		if err != nil {
			cblog.Error(err)
			//fmt.Println("body: ", string(body))
			//return c.JSONBlob(responseCode, body)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)
	} else {
		mapA := map[string]string{"message": "You must specify: action=registerWithInfo or action=registerWithId"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

}

func RestGetImage(c echo.Context) error {

	nsId := c.Param("nsId")

	id := c.Param("imageId")

	content := imageInfo{}

	fmt.Println("[Get image for id]" + id)
	key := common.GenResourceKey(nsId, "image", id)
	fmt.Println(key)

	keyValue, _ := store.Get(key)
	if keyValue == nil {
		mapA := map[string]string{"message": "Failed to find the image with given ID."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)
		content.Id = id // Optional. Can be omitted.

		return c.JSON(http.StatusOK, &content)
	}
}

func RestGetAllImage(c echo.Context) error {

	nsId := c.Param("nsId")

	var content struct {
		//Name string     `json:"name"`
		Image []imageInfo `json:"image"`
	}

	imageList := getResourceList(nsId, "image")

	for _, v := range imageList {

		key := common.GenResourceKey(nsId, "image", v)
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

func RestPutImage(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelImage(c echo.Context) error {

	nsId := c.Param("nsId")
	id := c.Param("imageId")
	forceFlag := c.QueryParam("force")

	responseCode, _, err := delResource(nsId, "image", id, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}

	mapA := map[string]string{"message": "The image has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllImage(c echo.Context) error {

	nsId := c.Param("nsId")
	forceFlag := c.QueryParam("force")

	imageList := getResourceList(nsId, "image")

	if len(imageList) == 0 {
		mapA := map[string]string{"message": "There is no image element in this namespace."}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		for _, v := range imageList {
			//responseCode, _, err := delImage(nsId, v, forceFlag)

			responseCode, _, err := delResource(nsId, "image", v, forceFlag)
			if err != nil {
				cblog.Error(err)
				mapA := map[string]string{"message": "Failed to delete the image"}
				return c.JSON(responseCode, &mapA)
			}

		}

		mapA := map[string]string{"message": "All images has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	}
}

/*
func createImage(nsId string, u *imageReq) (imageInfo, error) {

}
*/

//func registerImageWithId(nsId string, u *imageReq) (imageInfo, int, []byte, error) {
func registerImageWithId(nsId string, u *imageReq) (imageInfo, error) {
	check, _ := checkResource(nsId, "image", u.Name)

	if check {
		temp := imageInfo{}
		err := fmt.Errorf("The image " + u.Name + " already exists.")
		return temp, err
	}

	/*
		// Step 1. Create a temp `ImageReqInfo (from Spider)` object.
		type ImageReqInfo struct {
			Name string
			Id   string
			// @todo
		}
		tempReq := ImageReqInfo{}
		tempReq.Name = u.CspImageName
		tempReq.Id = u.CspImageId
	*/

	// Step 2. Send a req to Spider and save the response.
	url := SPIDER_URL + "/vmimage/" + u.CspImageId + "?connection_name=" + u.ConnectionName

	method := "GET"

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
	if err != nil {
		cblog.Error(err)
		content := imageInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := imageInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		content := imageInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	temp := ImageInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	// Step 3. Create a temp `imageInfo (in this file)` object.
	/* FYI; as of 2020-04-17
	type imageInfo struct {
		Id             string            `json:"id"`
		Name           string            `json:"name"`
		ConnectionName string            `json:"connectionName"`
		CspImageId     string            `json:"cspImageId"`
		CspImageName   string            `json:"cspImageName"`
		CreationDate   string            `json:"creationDate"`
		Description    string            `json:"description"`
		GuestOS        string            `json:"guestOS"` // Windows7, Ubuntu etc.
		Status         string            `json:"status"`  // available, unavailable
		KeyValueList   []common.KeyValue `json:"keyValueList"`
	}
	*/
	content := imageInfo{}
	content.Id = common.GenId(u.Name)
	content.Name = u.Name
	content.ConnectionName = u.ConnectionName
	content.CspImageId = temp.Name   // = u.CspImageId
	content.CspImageName = temp.Name // = u.CspImageName
	content.CreationDate = common.LookupKeyValueList(temp.KeyValueList, "CreationDate")
	content.Description = common.LookupKeyValueList(temp.KeyValueList, "Description")
	content.GuestOS = temp.GuestOS
	content.Status = temp.Status
	content.KeyValueList = temp.KeyValueList

	// Step 4. Store the metadata to CB-Store.
	fmt.Println("=========================== PUT registerImage")
	Key := common.GenResourceKey(nsId, "image", content.Id)
	Val, _ := json.Marshal(content)
	cbStorePutErr := store.Put(string(Key), string(Val))
	if cbStorePutErr != nil {
		cblog.Error(cbStorePutErr)
		//return content, res.StatusCode, body, cbStorePutErr
		return content, cbStorePutErr
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")
	//return content, res.StatusCode, body, nil
	return content, nil
}

func registerImageWithInfo(nsId string, content *imageInfo) (imageInfo, error) {
	check, _ := checkResource(nsId, "image", content.Name)

	if check {
		temp := imageInfo{}
		err := fmt.Errorf("The image " + content.Name + " already exists.")
		return temp, err
	}

	//content.Id = common.GenUuid()
	content.Id = common.GenId(content.Name)

	fmt.Println("=========================== PUT registerImage")
	Key := common.GenResourceKey(nsId, "image", content.Id)
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

/*
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
*/

/*
func delImage(nsId string, Id string, forceFlag string) (int, []byte, error) {

	fmt.Println("[Delete image] " + Id)

	key := genResourceKey(nsId, "image", Id)
	fmt.Println(key)

	// delete image info
	err := store.Delete(key)
	if err != nil {
		cblog.Error(err)
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}
*/
