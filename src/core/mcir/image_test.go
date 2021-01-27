package mcir

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestImage(t *testing.T) {
	/*
		    expected := 1
		    actual := 0
			assert.Equal(t, expected, actual, "기대값과 결과값이 다릅니다.")
	*/

	nsName := "tb-unit-test"

	nsReq := common.NsReq{}
	nsReq.Name = nsName

	_, err := common.CreateNs(&nsReq)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Namespace created successfully")
	}

	err = common.OpenSQL("../../../meta_db/dat/tb-unit-test.s3db")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Database access info set successfully")
	}

	err = common.SelectDatabase("tb-unit-test")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("DB selected successfully..")
	}

	err = common.CreateImageTable()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table image created successfully..")
	}

	imageName := "tb-unit-test"

	imageReq := TbImageInfo{}
	imageReq.Name = imageName

	result, _ := RegisterImageWithInfo(nsName, &imageReq)
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println("result: " + string(resultJSON))
	assert.Equal(t, imageName, result.Name, "CreateImage 기대값과 결과값이 다릅니다.")

	resultInterface, _ := GetResource(nsName, common.StrImage, imageName)
	result = resultInterface.(TbImageInfo) // type assertion
	assert.Equal(t, imageName, result.Name, "GetImage 기대값과 결과값이 다릅니다.")

	//result, _ := ListImage()

	//result, _ := ListImageId()

	resultErr := DelResource(nsName, common.StrImage, imageName, "false")
	assert.Nil(t, resultErr)

	err = common.DelNs(nsName)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Namespace deleted successfully")
	}

}
