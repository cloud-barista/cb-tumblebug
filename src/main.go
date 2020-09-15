package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	//_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"

	grpcserver "github.com/cloud-barista/cb-tumblebug/src/api/grpc/server"
	restapiserver "github.com/cloud-barista/cb-tumblebug/src/api/rest/server"
)

// Main Body

// @title CB-Tumblebug REST API
// @version 0.2.0
// @description CB-Tumblebug REST API
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://cloud-barista.github.io
// @contact.email contact-to-cloud-barista@googlegroups.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:1323
// @BasePath /tumblebug

// @securityDefinitions.basic BasicAuth

func main() {
	common.SPIDER_REST_URL = os.Getenv("SPIDER_REST_URL")
	common.DRAGONFLY_REST_URL = os.Getenv("DRAGONFLY_REST_URL")
	common.DB_URL = os.Getenv("DB_URL")
	common.DB_DATABASE = os.Getenv("DB_DATABASE")
	common.DB_USER = os.Getenv("DB_USER")
	common.DB_PASSWORD = os.Getenv("DB_PASSWORD")

	// load config
	//masterConfigInfos = confighandler.GetMasterConfigInfos()

	//Ticker for MCIS status validation
	validationDuration := 60000 //ms
	ticker := time.NewTicker(time.Millisecond * time.Duration(validationDuration))
	go func() {
		for t := range ticker.C {
			fmt.Println("Tick at", t)
			mcis.ValidateStatus()
		}
	}()
	defer ticker.Stop()

	var err error

	err = common.OpenSQL("../meta_db/dat/cbtumblebug.s3db")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Database access info set successfully")
	}

	err = common.SelectDatabase(common.DB_DATABASE)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("DB selected successfully..")
	}

	err = common.CreateSpecTable()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table spec created successfully..")
	}

	err = common.CreateImageTable()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table image created successfully..")
	}

	//defer db.Close()

	wg := new(sync.WaitGroup)

	wg.Add(2)

	go func() {
		restapiserver.ApiServer()
		wg.Done()
	}()

	go func() {
		grpcserver.RunServer()
		//fmt.Println("gRPC server started on " + grpcserver.Port)
		wg.Done()
	}()

	wg.Wait()
}
