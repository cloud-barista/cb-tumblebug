package main

import (
	"fmt"
	"os"
	"strconv"
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
// @version latest
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

	fmt.Println("")

	common.SPIDER_REST_URL = common.NVL(os.Getenv("SPIDER_REST_URL"), "http://localhost:1024/spider")
	common.DRAGONFLY_REST_URL = common.NVL(os.Getenv("DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
	common.DB_URL = common.NVL(os.Getenv("DB_URL"), "localhost:3306")
	common.DB_DATABASE = common.NVL(os.Getenv("DB_DATABASE"), "cb_tumblebug")
	common.DB_USER = common.NVL(os.Getenv("DB_USER"), "cb_tumblebug")
	common.DB_PASSWORD = common.NVL(os.Getenv("DB_PASSWORD"), "cb_tumblebug")
	common.AUTOCONTROL_DURATION_MS = common.NVL(os.Getenv("AUTOCONTROL_DURATION_MS"), "10000")

	// load the latest configuration from DB (if exist)
	fmt.Println("")
	fmt.Println("[Update system environment]")
	lowerizedName := common.ToLower("DRAGONFLY_REST_URL")
	common.UpdateEnv(lowerizedName)
	lowerizedName = common.ToLower("SPIDER_REST_URL")
	common.UpdateEnv(lowerizedName)
	lowerizedName = common.ToLower("AUTOCONTROL_DURATION_MS")
	common.UpdateEnv(lowerizedName)

	// load config
	//masterConfigInfos = confighandler.GetMasterConfigInfos()

	//Setup database (meta_db/dat/cbtumblebug.s3db)
	fmt.Println("")
	fmt.Println("[Setup SQL Database]")
	err := common.OpenSQL("../meta_db/dat/cbtumblebug.s3db")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Database access info set successfully")
	}

	/* // Required if using MySQL // Not required if using SQLite
	err = common.SelectDatabase(common.DB_DATABASE)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("DB selected successfully..")
	}
	*/

	err = common.CreateSpecTable()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table spec set successfully..")
	}

	err = common.CreateImageTable()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table image set successfully..")
	}

	//defer db.Close()

	//Ticker for MCIS Orchestration Policy
	fmt.Println("")
	fmt.Println("[Initiate Multi-Cloud Orchestration]")

	autoControlDuration, _ := strconv.Atoi(common.AUTOCONTROL_DURATION_MS) //ms
	ticker := time.NewTicker(time.Millisecond * time.Duration(autoControlDuration))
	go func() {
		for t := range ticker.C {
			//display ticker if you need (remove '_ = t')
			_ = t
			//fmt.Println("- Orchestration Controller ", t.Format("2006-01-02 15:04:05"))
			mcis.OrchestrationController()
		}
	}()
	defer ticker.Stop()

	// Launch API servers (REST and gRPC)
	wg := new(sync.WaitGroup)
	wg.Add(2)

	// Start REST Server
	go func() {
		restapiserver.ApiServer()
		wg.Done()
	}()

	// Start gRPC Server
	go func() {
		grpcserver.RunServer()
		//fmt.Println("gRPC server started on " + grpcserver.Port)
		wg.Done()
	}()

	wg.Wait()
}
