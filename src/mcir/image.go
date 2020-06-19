package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo"
	"github.com/xwb1989/sqlparser"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

// 2020-04-03 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/ImageHandler.go

type SpiderImageReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderImageReqInfo
}

type SpiderImageReqInfo struct { // Spider
	//IId   IID 	// {NameId, SystemId}
	Name string
	// @todo
}

type SpiderImageInfo struct { // Spider
	//IId     IID    // {NameId, SystemId}
	Name    string
	GuestOS string // Windows7, Ubuntu etc.
	Status  string // available, unavailable

	KeyValueList []common.KeyValue
}

type TbImageReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	CspImageName   string `json:"cspImageName"`
	//CreationDate   string `json:"creationDate"`
	Description string `json:"description"`
}

type TbImageInfo struct {
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
		u := &TbImageInfo{}
		if err := c.Bind(u); err != nil {
			return err
		}
		content, err := RegisterImageWithInfo(nsId, u)
		if err != nil {
			cblog.Error(err)
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
		return c.JSON(http.StatusCreated, content)
	} else if action == "registerWithId" {
		fmt.Println("[Registering Image with ID]")
		u := &TbImageReq{}
		if err := c.Bind(u); err != nil {
			return err
		}
		//content, responseCode, body, err := RegisterImageWithId(nsId, u)
		content, err := RegisterImageWithId(nsId, u)
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

	resourceType := "image"

	id := c.Param("imageId")

	/*
		content := TbImageInfo{}

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
	*/

	res, err := GetResource(nsId, resourceType, id)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find " + resourceType + " " + id}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

func RestGetAllImage(c echo.Context) error {

	nsId := c.Param("nsId")

	resourceType := "image"

	var content struct {
		Image []TbImageInfo `json:"image"`
	}

	/*
		imageList := ListResourceId(nsId, "image")

		for _, v := range imageList {

			key := common.GenResourceKey(nsId, "image", v)
			fmt.Println(key)
			keyValue, _ := store.Get(key)
			fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			imageTmp := TbImageInfo{}
			json.Unmarshal([]byte(keyValue.Value), &imageTmp)
			imageTmp.Id = v
			content.Image = append(content.Image, imageTmp)

		}
		fmt.Printf("content %+v\n", content)

		return c.JSON(http.StatusOK, &content)
	*/

	resourceList, err := ListResource(nsId, resourceType)
	if err != nil {
		mapA := map[string]string{"message": "Failed to list " + resourceType + "s."}
		return c.JSON(http.StatusNotFound, &mapA)
	}

	if resourceList == nil {
		return c.JSON(http.StatusOK, &content)
	}

	// When err == nil && resourceList != nil
	content.Image = resourceList.([]TbImageInfo) // type assertion (interface{} -> array)
	return c.JSON(http.StatusOK, &content)
}

func RestPutImage(c echo.Context) error {
	//nsId := c.Param("nsId")

	return nil
}

func RestDelImage(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "image"
	id := c.Param("imageId")
	forceFlag := c.QueryParam("force")

	responseCode, _, err := delResource(nsId, resourceType, id, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(responseCode, &mapA)
	}

	mapA := map[string]string{"message": "The " + resourceType + " " + id + " has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllImage(c echo.Context) error {

	nsId := c.Param("nsId")
	resourceType := "image"
	forceFlag := c.QueryParam("force")

	/*
		imageList := ListResourceId(nsId, "image")

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
	*/

	err := delAllResources(nsId, resourceType, forceFlag)
	if err != nil {
		cblog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := map[string]string{"message": "All " + resourceType + "s has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

/*
func createImage(nsId string, u *TbImageReq) (TbImageInfo, error) {

}
*/

//func RegisterImageWithId(nsId string, u *TbImageReq) (TbImageInfo, int, []byte, error) {
func RegisterImageWithId(nsId string, u *TbImageReq) (TbImageInfo, error) {
	check, _ := checkResource(nsId, "image", u.Name)

	if check {
		temp := TbImageInfo{}
		err := fmt.Errorf("The image " + u.Name + " already exists.")
		return temp, err
	}

	/*
		// Step 1. Create a temp `SpiderImageReqInfo (from Spider)` object.
		type SpiderImageReqInfo struct {
			Name string
			Id   string
			// @todo
		}
		tempReq := SpiderImageReqInfo{}
		tempReq.Name = u.CspImageName
		tempReq.Id = u.CspImageId
	*/

	// Step 2. Send a req to Spider and save the response.
	url := common.SPIDER_URL + "/vmimage/" + u.CspImageId + "?connection_name=" + u.ConnectionName

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
		content := TbImageInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		content := TbImageInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		content := TbImageInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	temp := SpiderImageInfo{}
	err2 := json.Unmarshal(body, &temp)
	if err2 != nil {
		fmt.Println("whoops:", err2)
	}

	content := TbImageInfo{}
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

	sql := "INSERT INTO `image`(" +
		"`id`, " +
		"`name`, " +
		"`connectionName`, " +
		"`cspImageId`, " +
		"`cspImageName`, " +
		"`creationDate`, " +
		"`description`, " +
		"`guestOS`, " +
		"`status`) " +
		"VALUES ('" +
		content.Id + "', '" +
		content.Name + "', '" +
		content.ConnectionName + "', '" +
		content.CspImageId + "', '" +
		content.CspImageName + "', '" +
		content.CreationDate + "', '" +
		content.Description + "', '" +
		content.GuestOS + "', '" +
		content.Status + "');"

	fmt.Println("sql: " + sql)
	// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
	_, err = sqlparser.Parse(sql)
	if err != nil {
		return content, err
	}

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

	stmt, err := common.MYDB.Prepare(sql)
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Data inserted successfully..")
	}

	//return content, res.StatusCode, body, nil
	return content, nil
}

func RegisterImageWithInfo(nsId string, content *TbImageInfo) (TbImageInfo, error) {
	check, _ := checkResource(nsId, "image", content.Name)

	if check {
		temp := TbImageInfo{}
		err := fmt.Errorf("The image " + content.Name + " already exists.")
		return temp, err
	}

	//content.Id = common.GenUuid()
	content.Id = common.GenId(content.Name)

	sql := "INSERT INTO `image`(" +
		"`id`, " +
		"`name`, " +
		"`connectionName`, " +
		"`cspImageId`, " +
		"`cspImageName`, " +
		"`creationDate`, " +
		"`description`, " +
		"`guestOS`, " +
		"`status`) " +
		"VALUES ('" +
		content.Id + "', '" +
		content.Name + "', '" +
		content.ConnectionName + "', '" +
		content.CspImageId + "', '" +
		content.CspImageName + "', '" +
		content.CreationDate + "', '" +
		content.Description + "', '" +
		content.GuestOS + "', '" +
		content.Status + "');"

	fmt.Println("sql: " + sql)
	// https://stackoverflow.com/questions/42486032/golang-sql-query-syntax-validator
	_, err := sqlparser.Parse(sql)
	if err != nil {
		return *content, err
	}

	fmt.Println("=========================== PUT registerImage")
	Key := common.GenResourceKey(nsId, "image", content.Id)
	Val, _ := json.Marshal(content)
	err = store.Put(string(Key), string(Val))
	if err != nil {
		cblog.Error(err)
		return *content, err
	}
	keyValue, _ := store.Get(string(Key))
	fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
	fmt.Println("===========================")

	stmt, err := common.MYDB.Prepare(sql)
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Data inserted successfully..")
	}

	return *content, nil
}
