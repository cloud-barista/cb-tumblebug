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

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

func TestSpec(t *testing.T) {
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

		// err = common.CreateSpecTable()
		// if err != nil {
		// 	fmt.Println(err.Error())
		// } else {
		// 	log.Debug().Msg("Table spec created successfully..")
		// }

		specName := "tb-unit-test"

		specReq := TbSpecInfo{}
		specReq.Name = specName

		result, _ := RegisterSpecWithInfo(nsName, &specReq, false)
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("result: " + string(resultJSON))
		assert.Equal(t, specName, result.Name, "CreateSpec: expected value and actual value are different.")

		resultInterface, _ := GetResource(nsName, common.StrSpec, specName)
		result = resultInterface.(TbSpecInfo) // type assertion
		assert.Equal(t, specName, result.Name, "GetSpec: expected value and actual value are different.")

		//result, _ := ListSpec()

		//result, _ := ListSpecId()

		resultErr := DelResource(nsName, common.StrSpec, specName, "false")
		assert.Nil(t, resultErr)

		err = common.DelNs(nsName)

		if err != nil {
			fmt.Println(err.Error())
		} else {

			log.Debug().Msg("Namespace deleted successfully")
		}
	*/
}

func TestFilterSpecsByRange(t *testing.T) {
	/*
		expected := 1
		actual := 0
		assert.Equal(t, expected, actual, "expected value and actual value are different")
	*/

	var err error
	common.ORM, err = xorm.NewEngine("sqlite3", "../../../meta_db/dat/cbtumblebug.s3db")
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Database access info set successfully")
	}
	common.ORM.SetTableMapper(names.SameMapper{})
	common.ORM.SetColumnMapper(names.SameMapper{})

	err = common.ORM.Sync2(new(TbSpecInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table spec set successfully..")
	}

	err = common.ORM.Sync2(new(TbImageInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table image set successfully..")
	}

	err = common.ORM.Sync2(new(TbCustomImageInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table customImage set successfully..")
	}

	filter := FilterSpecsByRangeRequest{
		InfraType: common.StrK8s,
	}

	nsId := common.SystemCommonNs
	filteredSpecs, err := FilterSpecsByRange(nsId, filter)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	for _, spec := range filteredSpecs {
		assert.Contains(t, spec.InfraType, common.StrK8s)
		log.Info().Msgf("Id: %s, Name: %s, InfraType: %s", spec.Id, spec.Name, spec.InfraType)
	}
}
