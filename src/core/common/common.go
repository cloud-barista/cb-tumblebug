package common

import (
	"database/sql"
	"fmt"
	"time"

	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/sirupsen/logrus"
)

type KeyValue struct {
	Key   string
	Value string
}

// CB-Store
var CBLog *logrus.Logger
var CBStore icbs.Store

var SPIDER_REST_URL string
var DRAGONFLY_REST_URL string
var DB_URL string
var DB_DATABASE string
var DB_USER string
var DB_PASSWORD string
var AUTOCONTROL_DURATION_MS string
var MYDB *sql.DB
var err error

const (
	StrSPIDER_REST_URL            string = "SPIDER_REST_URL"
	StrDRAGONFLY_REST_URL         string = "DRAGONFLY_REST_URL"
	StrDB_URL                     string = "DB_URL"
	StrDB_DATABASE                string = "DB_DATABASE"
	StrDB_USER                    string = "DB_USER"
	StrDB_PASSWORD                string = "DB_PASSWORD"
	StrAUTOCONTROL_DURATION_MS    string = "AUTOCONTROL_DURATION_MS"
	CbStoreKeyNotFoundErrorString string = "key not found"
	StrAdd                        string = "add"
	StrDelete                     string = "delete"
	StrSSHKey                     string = "sshKey"
	StrImage                      string = "image"
	StrSecurityGroup              string = "securityGroup"
	StrSpec                       string = "spec"
	StrVNet                       string = "vNet"
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
	MYDB, err = sql.Open("sqlite3", fullPathString)
	return err
}

func SelectDatabase(database string) error {
	query := "USE " + database
	_, err = MYDB.Exec(query)
	return err
}

func CreateSpecTable() error {
	stmt, err := MYDB.Prepare("CREATE Table IF NOT EXISTS spec(" +
		"namespace varchar(50) NOT NULL," +
		"id varchar(50) NOT NULL," +
		"connectionName varchar(50) NOT NULL," +
		"cspSpecName varchar(50) NOT NULL," +
		"name varchar(50)," +
		"os_type varchar(50)," +
		"num_vCPU SMALLINT," + // SMALLINT: -32768 ~ 32767
		"num_core SMALLINT," + // SMALLINT: -32768 ~ 32767
		"mem_GiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"storage_GiB MEDIUMINT," + // MEDIUMINT: -8388608 to 8388607
		"description varchar(50)," +
		"cost_per_hour FLOAT," +
		"num_storage SMALLINT," + // SMALLINT: -32768 ~ 32767
		"max_num_storage SMALLINT," + // SMALLINT: -32768 ~ 32767
		"max_total_storage_TiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"net_bw_Gbps SMALLINT," + // SMALLINT: -32768 ~ 32767
		"ebs_bw_Mbps MEDIUMINT," + // MEDIUMINT: -8388608 to 8388607
		"gpu_model varchar(50)," +
		"num_gpu SMALLINT," + // SMALLINT: -32768 ~ 32767
		"gpumem_GiB SMALLINT," + // SMALLINT: -32768 ~ 32767
		"gpu_p2p varchar(50)," +
		"orderInFilteredResult SMALLINT," + // SMALLINT: -32768 ~ 32767
		"evaluationStatus varchar(50)," +
		"evaluationScore_01 FLOAT," +
		"evaluationScore_02 FLOAT," +
		"evaluationScore_03 FLOAT," +
		"evaluationScore_04 FLOAT," +
		"evaluationScore_05 FLOAT," +
		"evaluationScore_06 FLOAT," +
		"evaluationScore_07 FLOAT," +
		"evaluationScore_08 FLOAT," +
		"evaluationScore_09 FLOAT," +
		"evaluationScore_10 FLOAT," +
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
