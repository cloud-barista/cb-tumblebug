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
var MYDB *sql.DB
var err error

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
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Database access info set successfully")
	}

	return err
}

func SelectDatabase(database string) error {
	query := "USE " + database
	_, err = MYDB.Exec(query)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("DB selected successfully..")
	}

	return err
}

func CreateSpecTable() error {
	stmt, err := MYDB.Prepare("CREATE Table IF NOT EXISTS spec(" +
		"id varchar(50) NOT NULL," +
		"connectionName varchar(50) NOT NULL," +
		"cspSpecName varchar(50) NOT NULL," +
		"name varchar(50)," +
		"os_type varchar(50)," +
		"num_vCPU varchar(50)," +
		"num_core varchar(50)," +
		"mem_GiB varchar(50)," +
		"mem_MiB varchar(50)," +
		"storage_GiB varchar(50)," +
		"description varchar(50)," +
		"cost_per_hour varchar(50)," +
		"num_storage varchar(50)," +
		"max_num_storage varchar(50)," +
		"max_total_storage_TiB varchar(50)," +
		"net_bw_Gbps varchar(50)," +
		"ebs_bw_Mbps varchar(50)," +
		"gpu_model varchar(50)," +
		"num_gpu varchar(50)," +
		"gpumem_GiB varchar(50)," +
		"gpu_p2p varchar(50)," +
		"PRIMARY KEY (id));")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table spec created successfully..")
	}

	return err
}

func CreateImageTable() error {
	stmt, err := MYDB.Prepare("CREATE Table IF NOT EXISTS image(" +
		"id varchar(50) NOT NULL," +
		"name varchar(50)," +
		"connectionName varchar(50) NOT NULL," +
		"cspImageId varchar(400) NOT NULL," +
		"cspImageName varchar(400) NOT NULL," +
		"creationDate varchar(50) NOT NULL," +
		"description varchar(400) NOT NULL," +
		"guestOS varchar(50) NOT NULL," +
		"status varchar(50) NOT NULL," +
		"PRIMARY KEY (id));")
	if err != nil {
		fmt.Println(err.Error())
	}
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table image created successfully..")
	}

	return err
}
