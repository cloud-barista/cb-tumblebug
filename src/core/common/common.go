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
var EtcdEndpoints string
var MyDB *sql.DB
var err error
var ORM *xorm.Engine

const (
	StrSpiderRestUrl         string = "TB_SPIDER_REST_URL"
	StrDragonflyRestUrl      string = "TB_DRAGONFLY_REST_URL"
	StrTerrariumRestUrl      string = "TB_TERRARIUM_REST_URL"
	StrDBUrl                 string = "TB_SQLITE_URL"
	StrDBDatabase            string = "TB_SQLITE_DATABASE"
	StrDBUser                string = "TB_SQLITE_USER"
	StrDBPassword            string = "TB_SQLITE_PASSWORD"
	StrAutocontrolDurationMs string = "TB_AUTOCONTROL_DURATION_MS"
	StrEtcdEndpoints         string = "TB_ETCD_ENDPOINTS"
	ErrStrKeyNotFound        string = "key not found"
	StrAdd                   string = "add"
	StrDelete                string = "delete"
	StrSSHKey                string = "sshKey"
	StrImage                 string = "image"
	StrCustomImage           string = "customImage"
	StrSecurityGroup         string = "securityGroup"
	StrSpec                  string = "spec"
	StrVNet                  string = "vNet"
	StrSubnet                string = "subnet"
	StrDataDisk              string = "dataDisk"
	StrNLB                   string = "nlb"
	StrVM                    string = "vm"
	StrMCI                   string = "mci"
	StrK8s                   string = "k8s"
	StrKubernetes            string = "kubernetes"
	StrContainer             string = "container"
	StrCommon                string = "common"
	StrEmpty                 string = "empty"
	StrDefaultResourceName   string = "-systemdefault-"
	// StrFirewallRule               string = "firewallRule"

	// SystemCommonNs is const for SystemCommon NameSpace ID
	SystemCommonNs string = "system"
)

var StartTime string

func init() {

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
			common.TB_SQLITE_USER+":"+
				common.TB_SQLITE_PASSWORD+"@tcp("+
				common.TB_SQLITE_URL+")/"+
				common.TB_SQLITE_DATABASE)
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
