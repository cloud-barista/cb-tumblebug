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

	// Black import (_) is for running a package's init() function without using its other contents.
	_ "github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/rs/zerolog/log"

	//_ "github.com/go-sql-driver/mysql"
	"github.com/fsnotify/fsnotify"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"

	restServer "github.com/cloud-barista/cb-tumblebug/src/api/rest/server"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// init for main
func init() {

	common.SpiderRestUrl = common.NVL(os.Getenv("SPIDER_REST_URL"), "http://localhost:1024/spider")
	common.DragonflyRestUrl = common.NVL(os.Getenv("DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
	common.DBUrl = common.NVL(os.Getenv("DB_URL"), "localhost:3306")
	common.DBDatabase = common.NVL(os.Getenv("DB_DATABASE"), "cb_tumblebug")
	common.DBUser = common.NVL(os.Getenv("DB_USER"), "cb_tumblebug")
	common.DBPassword = common.NVL(os.Getenv("DB_PASSWORD"), "cb_tumblebug")
	common.AutocontrolDurationMs = common.NVL(os.Getenv("AUTOCONTROL_DURATION_MS"), "10000")
	common.DefaultNamespace = common.NVL(os.Getenv("DEFAULT_NAMESPACE"), "ns01")
	common.DefaultCredentialHolder = common.NVL(os.Getenv("DEFAULT_CREDENTIALHOLDER"), "admin")

	// load the latest configuration from DB (if exist)

	log.Info().Msg("[Update system environment]")
	common.UpdateGlobalVariable(common.StrDragonflyRestUrl)
	common.UpdateGlobalVariable(common.StrSpiderRestUrl)
	common.UpdateGlobalVariable(common.StrAutocontrolDurationMs)

	// load config
	//masterConfigInfos = confighandler.GetMasterConfigInfos()

	//Setup database (meta_db/dat/cbtumblebug.s3db)

	log.Info().Msg("[Setup SQL Database]")

	err := os.MkdirAll("../meta_db/dat/", os.ModePerm)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	//err = common.OpenSQL("../meta_db/dat/cbtumblebug.s3db") // commented out to move to use XORM
	common.ORM, err = xorm.NewEngine("sqlite3", "../meta_db/dat/cbtumblebug.s3db")
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Database access info set successfully")
	}
	//common.ORM.SetMapper(names.SameMapper{})
	common.ORM.SetTableMapper(names.SameMapper{})
	common.ORM.SetColumnMapper(names.SameMapper{})

	// "CREATE Table IF NOT EXISTS spec(...)"
	//err = common.CreateSpecTable() // commented out to move to use XORM
	err = common.ORM.Sync2(new(mcir.TbSpecInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table spec set successfully..")
	}

	// "CREATE Table IF NOT EXISTS image(...)"
	//err = common.CreateImageTable() // commented out to move to use XORM
	err = common.ORM.Sync2(new(mcir.TbImageInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table image set successfully..")
	}

	err = common.ORM.Sync2(new(mcir.TbCustomImageInfo))
	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Table customImage set successfully..")
	}

	setConfig()

	_, err = common.GetNs(common.DefaultNamespace)
	if err != nil {
		if common.DefaultNamespace != "" {
			defaultNS := common.NsReq{Name: common.DefaultNamespace, Description: "Default Namespace"}
			_, err := common.CreateNs(&defaultNS)
			if err != nil {
				log.Error().Err(err).Msg("")
				panic(err)
			}
		} else {
			log.Error().Msg("Default namespace is not set")
			panic("Default namespace is not set, please set DEFAULT_NAMESPACE in setup.env or environment variable")
		}
	}
}

// setConfig get cloud settings from a config file
func setConfig() {
	fileName := "cloud_conf"
	viper.AddConfigPath(".")
	viper.AddConfigPath("./conf/")
	viper.AddConfigPath("../conf/")
	viper.SetConfigName(fileName)
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(fmt.Errorf("fatal error reading cloud_conf: %w", err))
	}
	log.Info().Msg(viper.ConfigFileUsed())
	err = viper.Unmarshal(&common.RuntimeConf)
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(err)
	}

	// Load cloudinfo
	cloudInfoViper := viper.New()
	fileName = "cloudinfo"
	cloudInfoViper.AddConfigPath(".")
	cloudInfoViper.AddConfigPath("./assets/")
	cloudInfoViper.AddConfigPath("../assets/")
	cloudInfoViper.SetConfigName(fileName)
	cloudInfoViper.SetConfigType("yaml")
	err = cloudInfoViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading cloudinfo config file: %w", err))
	}

	log.Info().Msg(cloudInfoViper.ConfigFileUsed())
	err = cloudInfoViper.Unmarshal(&common.RuntimeCloudInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(err)
	}
	// make all map keys lowercase
	common.AdjustKeysToLowercase(&common.RuntimeCloudInfo)
	// fmt.Printf("%+v\n", common.RuntimeCloudInfo)
	common.PrintCloudInfoTable(common.RuntimeCloudInfo)

	err = common.RegisterAllCloudInfo()
	if err != nil {
		log.Error().Err(err).Msg("Failed to register cloud info")
		panic(err)
	}

	// Load credentials
	credViper := viper.New()
	fileName = "cred"
	credViper.AddConfigPath(".")
	credViper.AddConfigPath("./conf/.cred/")
	credViper.AddConfigPath("../conf/.cred/")
	credViper.SetConfigName(fileName)
	credViper.SetConfigType("yaml")
	err = credViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading credential file: %w", err))
	}

	log.Info().Msg(credViper.ConfigFileUsed())
	err = credViper.Unmarshal(&common.RuntimeCredential)
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(err)
	}

	common.PrintCredentialInfo(common.RuntimeCredential)

	// err = common.RegisterAllCloudInfo()
	// if err != nil {
	// 	log.Error().Err(err).Msg("Failed to register credentials")
	// 	panic(err)
	// }

	// const mrttArrayXMax = 300
	// const mrttArrayYMax = 300
	// common.RuntimeLatancyMap = make([][]string, mrttArrayXMax)

	// cloudlatencymap.csv
	file, fileErr := os.Open("../assets/cloudlatencymap.csv")
	defer file.Close()
	if fileErr != nil {
		log.Error().Err(fileErr).Msg("")
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
	//fmt.Printf("[RuntimeLatancyMapIndex]\n %v\n", common.RuntimeLatancyMapIndex)

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

	//Ticker for MCIS Orchestration Policy

	log.Info().Msg("[Initiate Multi-Cloud Orchestration]")

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

	go func() {
		viper.WatchConfig()
		viper.OnConfigChange(func(e fsnotify.Event) {
			log.Info().Msgf("Config file changed: %s", e.Name)
			err := viper.ReadInConfig()
			if err != nil { // Handle errors reading the config file
				log.Error().Err(err).Msg("")
				panic(fmt.Errorf("fatal error config file: %w", err))
			}
			err = viper.Unmarshal(&common.RuntimeConf)
			if err != nil {
				log.Error().Err(err).Msg("")
				panic(err)
			}
		})
	}()

	// Launch API servers (REST)
	wg := new(sync.WaitGroup)
	wg.Add(1)

	// Start REST Server
	go func() {
		restServer.RunServer(*port)
		wg.Done()
	}()

	// Note: Deprecated gRPC server
	// Start gRPC Server
	// go func() {
	// 	grpcServer.RunServer()
	// 	wg.Done()
	// }()
	// fmt.Println("RuntimeConf: ", common.RuntimeConf.Cloud)

	wg.Wait()
}
