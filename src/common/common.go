package common

import (
	"database/sql"
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

var SPIDER_URL string
var DRAGONFLY_URL string
var DB_URL string
var DB_DATABASE string
var DB_USER string
var DB_PASSWORD string
var MYDB *sql.DB

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
