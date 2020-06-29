package mcir

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/xwb1989/sqlparser"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

// 2020-04-03 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/ImageHandler.go

type SpiderImageReqInfoWrapper struct { // Spider
	ConnectionName string
	ReqInfo        SpiderImageInfo
}

/*
type SpiderImageReqInfo struct { // Spider
	//IId   IID 	// {NameId, SystemId}
	Name string
	// @todo
}
*/

// TODO: Need to update (after CB-Spider's implementing lookupImage feature)
type SpiderImageInfo struct { // Spider
	// Fields for request
	Name string

	// Fields for response
	IId          common.IID // {NameId, SystemId}
	GuestOS      string     // Windows7, Ubuntu etc.
	Status       string     // available, unavailable
	KeyValueList []common.KeyValue
}

/*
type TbImageReq struct {
	//Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	CspImageName   string `json:"cspImageName"`
	//CreationDate   string `json:"creationDate"`
	Description string `json:"description"`
}
*/

type TbImageInfo struct {
	// Fields for both request and response
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`
	CspImageId     string `json:"cspImageId"`
	CspImageName   string `json:"cspImageName"`
	Description    string `json:"description"`

	// Additional fields for response
	Id           string            `json:"id"`
	CreationDate string            `json:"creationDate"`
	GuestOS      string            `json:"guestOS"` // Windows7, Ubuntu etc.
	Status       string            `json:"status"`  // available, unavailable
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

/*
func createImage(nsId string, u *TbImageReq) (TbImageInfo, error) {

}
*/

// TODO: Need to update (after CB-Spider's implementing lookupImage feature)
func RegisterImageWithId(nsId string, u *TbImageInfo) (TbImageInfo, error) {
	check, _ := CheckResource(nsId, "image", u.Name)

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
		common.CBLog.Error(err)
		content := TbImageInfo{}
		//return content, res.StatusCode, nil, err
		return content, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		common.CBLog.Error(err)
		content := TbImageInfo{}
		//return content, res.StatusCode, body, err
		return content, err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf(string(body))
		fmt.Println("body: ", string(body))
		common.CBLog.Error(err)
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
	err = common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		//return content, res.StatusCode, body, err
		return content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
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
	check, _ := CheckResource(nsId, "image", content.Name)

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
	err = common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return *content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
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
