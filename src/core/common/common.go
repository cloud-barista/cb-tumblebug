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
	"sync"
	"time"

	cbstore "github.com/cloud-barista/cb-store"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"xorm.io/xorm"
)

// KeyValue is struct for key-value pair
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type IdList struct {
	IdList []string `json:"output"`
	mux    sync.Mutex
}

// AddItem adds a new item to the IdList
func (list *IdList) AddItem(id string) {
	list.mux.Lock()
	defer list.mux.Unlock()
	list.IdList = append(list.IdList, id)
}

// OptionalParameter is struct for optional parameter for function (ex. VmId)
type OptionalParameter struct {
	Value string
	Set   bool
}

// SystemReady is global variable for checking SystemReady status
var SystemReady bool

// CB-Store
var CBStore icbs.Store
var SpiderRestUrl string
var DragonflyRestUrl string
var TerrariumRestUrl string
var DBUrl string
var DBDatabase string
var DBUser string
var DBPassword string
var AutocontrolDurationMs string
var DefaultNamespace string
var DefaultCredentialHolder string
var MyDB *sql.DB
var err error
var ORM *xorm.Engine

const (
	StrSpiderRestUrl              string = "SPIDER_REST_URL"
	StrDragonflyRestUrl           string = "DRAGONFLY_REST_URL"
	StrTerrariumRestUrl           string = "TERRARIUM_REST_URL"
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
	StrK8s                        string = "k8s"
	StrKubernetes                 string = "kubernetes"
	StrContainer                  string = "container"
	StrDefaultResourceName        string = "-systemdefault-"
	// StrFirewallRule               string = "firewallRule"

	// SystemCommonNs is const for SystemCommon NameSpace ID
	SystemCommonNs string = "system-purpose-common-ns"
)

var StartTime string

func init() {
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
