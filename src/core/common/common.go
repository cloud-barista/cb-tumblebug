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

	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
	"xorm.io/xorm"
)

type KeyValue struct {
	Key   string
	Value string
}

type IdList struct {
	IdList []string `json:"output"`
}

// CB-Store
var CBLog *logrus.Logger
var CBStore icbs.Store

var SpiderRestUrl string
var DragonflyRestUrl string
var DBUrl string
var DBDatabase string
var DBUser string
var DBPassword string
var AutocontrolDurationMs string
var MyDB *sql.DB
var err error
var ORM *xorm.Engine

const (
	StrSpiderRestUrl              string = "SPIDER_REST_URL"
	StrDragonflyRestUrl           string = "DRAGONFLY_REST_URL"
	StrDBUrl                      string = "DB_URL"
	StrDBDatabase                 string = "DB_DATABASE"
	StrDBUser                     string = "DB_USER"
	StrDBPassword                 string = "DB_PASSWORD"
	StrAutocontrolDurationMs      string = "AUTOCONTROL_DURATION_MS"
	CbStoreKeyNotFoundErrorString string = "key not found"
	StrAdd                        string = "add"
	StrDelete                     string = "delete"
	StrSSHKey                     string = "sshKey"
	StrImage                      string = "image"
	StrCustomImage                string = "customImage"
	StrSecurityGroup              string = "securityGroup"
	StrSpec                       string = "spec"
	StrVNet                       string = "vNet"
	StrSubnet                     string = "subnet"
	StrDataDisk                   string = "dataDisk"
	StrNLB                        string = "nlb"
	StrVM                         string = "vm"
	StrMCIS                       string = "mcis"
	StrDefaultResourceName        string = "-systemdefault-"
	// StrFirewallRule               string = "firewallRule"

	// SystemCommonNs is const for SystemCommon NameSpace ID
	SystemCommonNs string = "system-purpose-common-ns"
)

var StartTime string

func init() {
	CBLog = config.Cblogger
	CBStore = cbstore.GetStore()

	StartTime = time.Now().Format("2006.01.02 15:04:05 Mon")
}

// Spider 2020-03-30 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/IId.go
type IID struct {
	NameId   string // NameID by user
	SystemId string // SystemID by CloudOS
}

type SpiderConnectionName struct {
	ConnectionName string `json:"ConnectionName"`
}

func OpenSQL(path string) error {
	/*
		common.MYDB, err = sql.Open("mysql", //"root:pwd@tcp(127.0.0.1:3306)/testdb")
			common.DB_USER+":"+
				common.DB_PASSWORD+"@tcp("+
				common.DB_URL+")/"+
				common.DB_DATABASE)
	*/

	fullPathString := "file:" + path
	MyDB, err = sql.Open("sqlite3", fullPathString)
	return err
}

func SelectDatabase(database string) error {
	query := "USE " + database + ";"
	_, err = MyDB.Exec(query)
	return err
}

/*
func CreateSpecTable() error {
	stmt, err := MYDB.Prepare("CREATE Table IF NOT EXISTS spec(" +
		"namespace varchar(50) NOT NULL," +
		"id varchar(50) NOT NULL," +
		"connectionName varchar(50) NOT NULL," +
		"cspSpecName varchar(50) NOT NULL," +
		"name varchar(50)," +
		"osType varchar(50)," +
		"numvCPU SMALLINT," + // SMALLINT: -32768 ~ 32767
		"numcore SMALLINT," + // SMALLINT: -32768 ~ 32767
		"memGiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"storageGiB MEDIUMINT," + // MEDIUMINT: -8388608 to 8388607
		"description varchar(50)," +
		"costPerHour FLOAT," +
		"numAtorage SMALLINT," + // SMALLINT: -32768 ~ 32767
		"maxNumStorage SMALLINT," + // SMALLINT: -32768 ~ 32767
		"maxTotalStorage_TiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"netBwGbps SMALLINT," + // SMALLINT: -32768 ~ 32767
		"ebsBwMbps MEDIUMINT," + // MEDIUMINT: -8388608 to 8388607
		"gpuModel varchar(50)," +
		"numGpu SMALLINT," + // SMALLINT: -32768 ~ 32767
		"gpumemGiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"gpuP2p varchar(50)," +
		"orderInFilteredResult SMALLINT," + // SMALLINT: -32768 ~ 32767
		"evaluationStatus varchar(50)," +
		"evaluationScore01 FLOAT," +
		"evaluationScore02 FLOAT," +
		"evaluationScore03 FLOAT," +
		"evaluationScore04 FLOAT," +
		"evaluationScore05 FLOAT," +
		"evaluationScore06 FLOAT," +
		"evaluationScore07 FLOAT," +
		"evaluationScore08 FLOAT," +
		"evaluationScore09 FLOAT," +
		"evaluationScore10 FLOAT," +
		"CONSTRAINT PK_Spec PRIMARY KEY (namespace, id));")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()

	return err
}

func CreateImageTable() error {
	stmt, err := MYDB.Prepare("CREATE Table IF NOT EXISTS image(" +
		"namespace varchar(50) NOT NULL," +
		"id varchar(50) NOT NULL," +
		"name varchar(50)," +
		"connectionName varchar(50) NOT NULL," +
		"cspImageId varchar(400) NOT NULL," +
		"cspImageName varchar(400) NOT NULL," +
		"creationDate varchar(50) NOT NULL," +
		"description varchar(400) NOT NULL," +
		"guestOS varchar(50) NOT NULL," +
		"status varchar(50) NOT NULL," +
		"CONSTRAINT PK_Image PRIMARY KEY (namespace, id));")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()

	return err
}
*/
