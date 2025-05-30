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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"database/sql"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

func init() {

	model.StartTime = time.Now().Format("2006.01.02 15:04:05 Mon")
}

func OpenSQL(path string) error {
	/*
		common.MYDB, err = sql.Open("mysql", //"root:pwd@tcp(127.0.0.1:3306)/testdb")
			common.TB_POSTGRES_USER+":"+
				common.TB_POSTGRES_PASSWORD+"@tcp("+
				common.TB_POSTGRES_ENDPOINT+")/"+
				common.TB_POSTGRES_DATABASE)
	*/
	err := error(nil)

	fullPathString := "file:" + path
	model.MyDB, err = sql.Open("sqlite3", fullPathString)
	return err
}

func SelectDatabase(database string) error {
	query := "USE " + database + ";"
	_, err := model.MyDB.Exec(query)
	return err
}
