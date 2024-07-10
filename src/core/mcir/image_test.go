/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcir is to manage multi-cloud infra resource
package mcir

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestImage(t *testing.T) {
	/*
		expected := 1
		actual := 0
		assert.Equal(t, expected, actual, "expected value and actual value are different")
	*/

	/*
		nsName := "tb-unit-test"

		nsReq := common.NsReq{}
		nsReq.Name = nsName

		_, err := common.CreateNs(&nsReq)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Namespace created successfully")
		}

		err = common.OpenSQL("../../../meta_db/dat/tb-unit-test.s3db")
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Database access info set successfully")
		}

		err = common.SelectDatabase("tb-unit-test")
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("DB selected successfully..")
		}

		// err = common.CreateImageTable()
		// if err != nil {
		// 	fmt.Println(err.Error())
		// } else {
		// 	log.Debug().Msg("Table image created successfully..")
		// }

		imageName := "tb-unit-test"

		imageReq := TbImageInfo{}
		imageReq.Name = imageName

		result, _ := RegisterImageWithInfo(nsName, &imageReq, false)
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		log.Debug().Msg("result: " + string(resultJSON))
		assert.Equal(t, imageName, result.Name, "CreateImage: expected value and actual value are different.")

		resultInterface, _ := GetResource(nsName, common.StrImage, imageName)
		result = resultInterface.(TbImageInfo) // type assertion
		assert.Equal(t, imageName, result.Name, "GetImage: expected value and actual value are different.")

		//result, _ := ListImage()

		//result, _ := ListImageId()

		resultErr := DelResource(nsName, common.StrImage, imageName, "false")
		assert.Nil(t, resultErr)

		err = common.DelNs(nsName)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			log.Debug().Msg("Namespace deleted successfully")
		}
	*/
}
