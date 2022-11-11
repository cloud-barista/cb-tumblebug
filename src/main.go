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

// Package main is the starting point of CB-Tumblebug
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	//_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"

	grpcServer "github.com/cloud-barista/cb-tumblebug/src/api/grpc/server"
	restServer "github.com/cloud-barista/cb-tumblebug/src/api/rest/server"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// init for main
func init() {
	profile := "cloud_conf"
	setConfig(profile)
}

// setConfig get cloud settings from a config file
func setConfig(profile string) {
	viper.AddConfigPath(".")
	viper.AddConfigPath("./conf/")
	viper.AddConfigPath("../conf/")
	viper.SetConfigName(profile)
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	err = viper.Unmarshal(&common.RuntimeConf)
	if err != nil {
		panic(err)
	}

	// const mrttArrayXMax = 300
	// const mrttArrayYMax = 300
	// common.RuntimeLatancyMap = make([][]string, mrttArrayXMax)

	// cloudlatencymap.csv
	file, fileErr := os.Open("../assets/cloudlatencymap.csv")
	defer file.Close()
	if fileErr != nil {
		common.CBLog.Error(fileErr)
		panic(fileErr)
	}
	rdr := csv.NewReader(bufio.NewReader(file))
	common.RuntimeLatancyMap, _ = rdr.ReadAll()

	for i, v := range common.RuntimeLatancyMap {
		if i == 0 {
			continue
		}
		if v[0] == "" {
			break
		}
		common.RuntimeLatancyMapIndex[v[0]] = i
	}

	//fmt.Printf("RuntimeLatancyMap: %v\n\n", common.RuntimeLatancyMap)

	fmt.Printf("[RuntimeLatancyMapIndex]\n %v\n", common.RuntimeLatancyMapIndex)

}

// Main Body

// @title CB-Tumblebug REST API
// @version latest
// @description CB-Tumblebug REST API

// @contact.name API Support
// @contact.url http://cloud-barista.github.io
// @contact.email contact-to-cloud-barista@googlegroups.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath /tumblebug

// @securityDefinitions.basic BasicAuth
func main() {
	fmt.Println("")

	// giving a default value of "1323"
	port := flag.String("port", "1323", "port number for the restapiserver to listen to")
	flag.Parse()

	// validate arguments from flag
	validationFlag := true
	// validation: port
	// set validationFlag to false if your number is not in [1-65535] range
	if portInt, err := strconv.Atoi(*port); err == nil {
		if portInt < 1 || portInt > 65535 {
			validationFlag = false
		}
	} else {
		validationFlag = false
	}
	if !validationFlag {
		fmt.Printf("%s is not a valid port number.\n", *port)
		fmt.Printf("Please retry with a valid port number (ex: -port=[1-65535]).\n")
		os.Exit(1)
	}

	common.SpiderRestUrl = common.NVL(os.Getenv("SPIDER_REST_URL"), "http://localhost:1024/spider")
	common.DragonflyRestUrl = common.NVL(os.Getenv("DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
	common.DBUrl = common.NVL(os.Getenv("DB_URL"), "localhost:3306")
	common.DBDatabase = common.NVL(os.Getenv("DB_DATABASE"), "cb_tumblebug")
	common.DBUser = common.NVL(os.Getenv("DB_USER"), "cb_tumblebug")
	common.DBPassword = common.NVL(os.Getenv("DB_PASSWORD"), "cb_tumblebug")
	common.AutocontrolDurationMs = common.NVL(os.Getenv("AUTOCONTROL_DURATION_MS"), "10000")

	// load the latest configuration from DB (if exist)
	fmt.Println("")
	fmt.Println("[Update system environment]")
	common.UpdateGlobalVariable(common.StrDragonflyRestUrl)
	common.UpdateGlobalVariable(common.StrSpiderRestUrl)
	common.UpdateGlobalVariable(common.StrAutocontrolDurationMs)

	// load config
	//masterConfigInfos = confighandler.GetMasterConfigInfos()

	//Setup database (meta_db/dat/cbtumblebug.s3db)
	fmt.Println("")
	fmt.Println("[Setup SQL Database]")

	err := os.MkdirAll("../meta_db/dat/", os.ModePerm)
	if err != nil {
		fmt.Println(err.Error())
	}

	//err = common.OpenSQL("../meta_db/dat/cbtumblebug.s3db") // commented out to move to use XORM
	common.ORM, err = xorm.NewEngine("sqlite3", "../meta_db/dat/cbtumblebug.s3db")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Database access info set successfully")
	}
	//common.ORM.SetMapper(names.SameMapper{})
	common.ORM.SetTableMapper(names.SameMapper{})
	common.ORM.SetColumnMapper(names.SameMapper{})

	/* // Required if using MySQL // Not required if using SQLite
	err = common.SelectDatabase(common.DB_DATABASE)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("DB selected successfully..")
	}
	*/

	// "CREATE Table IF NOT EXISTS spec(...)"
	//err = common.CreateSpecTable() // commented out to move to use XORM
	err = common.ORM.Sync2(new(mcir.TbSpecInfo))
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table spec set successfully..")
	}

	// "CREATE Table IF NOT EXISTS image(...)"
	//err = common.CreateImageTable() // commented out to move to use XORM
	err = common.ORM.Sync2(new(mcir.TbImageInfo))
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table image set successfully..")
	}

	err = common.ORM.Sync2(new(mcir.TbCustomImageInfo))
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table customImage set successfully..")
	}

	//defer db.Close()

	//Ticker for MCIS Orchestration Policy
	fmt.Println("")
	fmt.Println("[Initiate Multi-Cloud Orchestration]")

	autoControlDuration, _ := strconv.Atoi(common.AutocontrolDurationMs) //ms
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
		restServer.RunServer(*port)
		wg.Done()
	}()

	// Start gRPC Server
	go func() {
		grpcServer.RunServer()
		//fmt.Println("gRPC server started on " + grpcserver.Port)
		wg.Done()
	}()

	// fmt.Println("RuntimeConf: ", common.RuntimeConf.Cloud)

	wg.Wait()
}
