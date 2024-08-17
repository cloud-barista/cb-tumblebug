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
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/etcd"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"

	//_ "github.com/go-sql-driver/mysql"
	"github.com/fsnotify/fsnotify"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mci"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"

	restServer "github.com/cloud-barista/cb-tumblebug/src/api/rest/server"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// init for main
func init() {
	common.SystemReady = false

	common.SpiderRestUrl = common.NVL(os.Getenv("TB_SPIDER_REST_URL"), "http://localhost:1024/spider")
	common.DragonflyRestUrl = common.NVL(os.Getenv("TB_DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
	common.TerrariumRestUrl = common.NVL(os.Getenv("TB_TERRARIUM_REST_URL"), "http://localhost:8888/terrarium")
	common.DBUrl = common.NVL(os.Getenv("TB_SQLITE_URL"), "localhost:3306")
	common.DBDatabase = common.NVL(os.Getenv("TB_SQLITE_DATABASE"), "cb_tumblebug")
	common.DBUser = common.NVL(os.Getenv("TB_SQLITE_USER"), "cb_tumblebug")
	common.DBPassword = common.NVL(os.Getenv("TB_SQLITE_PASSWORD"), "cb_tumblebug")
	common.AutocontrolDurationMs = common.NVL(os.Getenv("TB_AUTOCONTROL_DURATION_MS"), "10000")
	common.DefaultNamespace = common.NVL(os.Getenv("TB_DEFAULT_NAMESPACE"), "default")
	common.DefaultCredentialHolder = common.NVL(os.Getenv("TB_DEFAULT_CREDENTIALHOLDER"), "admin")
	// Etcd
	common.EtcdEndpoints = common.NVL(os.Getenv("TB_ETCD_ENDPOINTS"), "localhost:2379")

	// load the latest configuration from DB (if exist)

	log.Info().Msg("[Update system environment]")
	common.UpdateGlobalVariable(common.StrDragonflyRestUrl)
	common.UpdateGlobalVariable(common.StrSpiderRestUrl)
	common.UpdateGlobalVariable(common.TerrariumRestUrl)
	common.UpdateGlobalVariable(common.StrAutocontrolDurationMs)

	// Initialize the logger
	logLevel := common.NVL(os.Getenv("TB_LOGLEVEL"), "debug")
	logWriter := common.NVL(os.Getenv("TB_LOGWRITER"), "both")
	logFilePath := common.NVL(os.Getenv("TB_LOGFILE_PATH"), "./log/tumblebug.log")
	logMaxSizeStr := common.NVL(os.Getenv("TB_LOGFILE_MAXSIZE"), "10")
	logMaxSize, _ := strconv.Atoi(logMaxSizeStr)
	logMaxBackupsStr := common.NVL(os.Getenv("TB_LOGFILE_MAXBACKUPS"), "3")
	logMaxBackups, _ := strconv.Atoi(logMaxBackupsStr)
	logMaxAgeStr := common.NVL(os.Getenv("TB_LOGFILE_MAXAGE"), "3")
	logMaxAge, _ := strconv.Atoi(logMaxAgeStr)
	logCompressStr := common.NVL(os.Getenv("TB_LOGFILE_COMPRESS"), "false")
	logCompress := (logCompressStr == "true")

	logger := logger.NewLogger(logger.Config{
		LogLevel:    logLevel,
		LogWriter:   logWriter,
		LogFilePath: logFilePath,
		MaxSize:     logMaxSize,
		MaxBackups:  logMaxBackups,
		MaxAge:      logMaxAge,
		Compress:    logCompress,
	})

	// Set the global logger
	log.Logger = *logger

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
			panic("Default namespace is not set, please set TB_DEFAULT_NAMESPACE in setup.env or environment variable")
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

	//
	// Load k8sclusterinfo
	//
	k8sClusterInfoViper := viper.New()
	fileName = "k8sclusterinfo"
	k8sClusterInfoViper.AddConfigPath(".")
	k8sClusterInfoViper.AddConfigPath("./assets/")
	k8sClusterInfoViper.AddConfigPath("../assets/")
	k8sClusterInfoViper.SetConfigName(fileName)
	k8sClusterInfoViper.SetConfigType("yaml")
	err = k8sClusterInfoViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading cloudinfo config file: %w", err))
	}

	log.Info().Msg(k8sClusterInfoViper.ConfigFileUsed())
	err = k8sClusterInfoViper.Unmarshal(&common.RuntimeK8sClusterInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(err)
	}

	//
	// Wait until CB-Spider is ready
	//
	maxAttempts := 60 // (3 mins)
	attempt := 0

	for attempt < maxAttempts {
		if common.CheckSpiderReady() == nil {
			log.Info().Msg("CB-Spider is now ready. Initializing CB-Tumblebug...")
			break
		}
		log.Info().Msgf("CB-Spider at %s is not ready. Attempt %d/%d", common.SpiderRestUrl, attempt+1, maxAttempts)
		time.Sleep(3 * time.Second)
		attempt++
	}

	if attempt == maxAttempts {
		panic("Failed to confirm CB-Spider readiness within the allowed time. \nCheck the connection to CB-Spider.")
	}

	// Setup etcd and kvstore
	var etcdAuthEnabled bool
	var etcdUsername string
	var etcdPassword string
	etcdAuthEnabled = os.Getenv("TB_ETCD_AUTH_ENABLED") == "true"
	if etcdAuthEnabled {
		etcdUsername = os.Getenv("TB_ETCD_USERNAME")
		etcdPassword = os.Getenv("TB_ETCD_PASSWORD")
	}

	etcdEndpoints := strings.Split(common.EtcdEndpoints, ",")

	ctx := context.Background()
	config := etcd.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: 5 * time.Second,
	}
	if etcdAuthEnabled && etcdUsername != "" && etcdPassword != "" {
		config.Username = etcdUsername
		config.Password = etcdPassword
	}

	// Wait until etcd is ready
	var etcdStore kvstore.Store
	var err2 error
	etcdMaxAttempts := 10 // (50 sec)
	etcdAttempt := 1
	for ; etcdAttempt <= etcdMaxAttempts; etcdAttempt++ {
		etcdStore, err2 = etcd.NewEtcdStore(ctx, config)
		if err2 == nil {
			log.Info().Msg("etcd is now available.")
			break
		}
		log.Warn().Err(err2).Msgf("etcd at %s is not ready. Attempt %d/%d", common.EtcdEndpoints, etcdAttempt, maxAttempts)
		time.Sleep(5 * time.Second)
	}

	if err2 != nil {
		log.Fatal().Err(err2).Msg("failed to initialize etcd")
	}

	err2 = kvstore.InitializeStore(etcdStore)
	if err2 != nil {
		log.Fatal().Err(err2).Msg("")
	}
	log.Info().Msg("kvstore is initialized successfully. Initializing CB-Tumblebug...")

	// Register all cloud info
	err = common.RegisterAllCloudInfo()
	if err != nil {
		log.Error().Err(err).Msg("Failed to register cloud info")
		panic(err)
	}

	// Load credentials
	usr, err := user.Current()
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	credPath := usr.HomeDir + "/.cloud-barista"
	credViper := viper.New()
	fileName = "credentials"
	credViper.AddConfigPath(credPath)
	credViper.SetConfigName(fileName)
	credViper.SetConfigType("yaml")
	err = credViper.ReadInConfig()
	if err != nil {
		log.Info().Msg("Local credentials file not found. Continue.")
	} else {
		log.Info().Msg(credViper.ConfigFileUsed())
		err = credViper.Unmarshal(&common.RuntimeCredential)
		if err != nil {
			log.Error().Err(err).Msg("")
			panic(err)
		}
		// common.PrintCredentialInfo(common.RuntimeCredential)
	}

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

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token ([TBD] Get token in http://xxx.xxx.xxx.xxx:xxx/auth)
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

	//Ticker for MCI Orchestration Policy

	log.Info().Msg("[Initiate Multi-Cloud Orchestration]")

	autoControlDuration, _ := strconv.Atoi(common.AutocontrolDurationMs) //ms
	ticker := time.NewTicker(time.Millisecond * time.Duration(autoControlDuration))
	go func() {
		for t := range ticker.C {
			//display ticker if you need (remove '_ = t')
			_ = t
			//fmt.Println("- Orchestration Controller ", t.Format("2006-01-02 15:04:05"))
			mci.OrchestrationController()
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
