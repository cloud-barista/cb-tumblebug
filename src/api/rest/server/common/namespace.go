package common

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"

	"github.com/cloud-barista/cb-tumblebug/src/common"
)

func RestCheckNs(c echo.Context) error {

	nsId := c.Param("nsId")

	exists, err := common.CheckNs(nsId)

	type JsonTemplate struct {
		Exists bool `json:exists`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		common.CBLog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}

func RestDelAllNs(c echo.Context) error {
	/*
		nsList := ListNsId()

		for _, v := range nsList {
			err := DelNs(v)
			if err != nil {
				CBLog.Error(err)
				mapA := map[string]string{"message": err.Error()}
				return c.JSON(http.StatusFailedDependency, &mapA)
			}
		}

		mapA := map[string]string{"message": "All nss has been deleted"}
		return c.JSON(http.StatusOK, &mapA)
	*/

	err := common.DelAllNs()
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusConflict, &mapA)
	}

	mapA := map[string]string{"message": "All namespaces has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelNs(c echo.Context) error {

	id := c.Param("nsId")

	err := common.DelNs(id)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "The ns has been deleted"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestGetAllNs(c echo.Context) error {

	var content struct {
		//Name string     `json:"name"`
		Ns []common.NsInfo `json:"ns"`
	}

	/*
		nsList := ListNsId()

		for _, v := range nsList {

			key := "/ns/" + v
			fmt.Println(key)
			keyValue, err := CBStore.Get(key)
			if err != nil {
				CBLog.Error(err)
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

	nsList, err := common.ListNs()
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

func RestGetNs(c echo.Context) error {
	id := c.Param("nsId")

	/*
		content := NsInfo{}

		fmt.Println("[Get ns for id]" + id)
		key := "/ns/" + id
		fmt.Println(key)

		keyValue, err := CBStore.Get(key)
		if err != nil {
			CBLog.Error(err)
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

	res, err := common.GetNs(id)
	if err != nil {
		mapA := map[string]string{"message": "Failed to find the namespace " + id}
		return c.JSON(http.StatusNotFound, &mapA)
	} else {
		return c.JSON(http.StatusOK, &res)
	}
}

func RestPostNs(c echo.Context) error {

	u := &common.NsInfo{}
	if err := c.Bind(u); err != nil {
		return err
	}

	fmt.Println("[Creating Ns]")
	content, err := common.CreateNs(u)
	if err != nil {
		common.CBLog.Error(err)
		//mapA := map[string]string{"message": "Failed to create the ns " + u.Name}
		mapA := map[string]string{"message": err.Error()}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}
	return c.JSON(http.StatusCreated, content)

}

func RestPutNs(c echo.Context) error {
	return nil
}
