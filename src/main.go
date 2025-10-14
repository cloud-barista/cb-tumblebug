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
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/etcd"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"

	restServer "github.com/cloud-barista/cb-tumblebug/src/interface/rest/server"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// init for main
func init() {
	model.SystemReady = false

	model.SelfEndpoint = common.NVL(os.Getenv("TB_SELF_ENDPOINT"), "localhost:1323")
	model.SpiderRestUrl = common.NVL(os.Getenv("TB_SPIDER_REST_URL"), "http://localhost:1024/spider")
	model.DragonflyRestUrl = common.NVL(os.Getenv("TB_DRAGONFLY_REST_URL"), "http://localhost:9090/dragonfly")
	model.TerrariumRestUrl = common.NVL(os.Getenv("TB_TERRARIUM_REST_URL"), "http://localhost:8055/terrarium")
	model.DBUrl = common.NVL(os.Getenv("TB_POSTGRES_ENDPOINT"), "localhost:3306")
	model.DBDatabase = common.NVL(os.Getenv("TB_POSTGRES_DATABASE"), "tumblebug")
	model.DBUser = common.NVL(os.Getenv("TB_POSTGRES_USER"), "tumblebug")
	model.DBPassword = common.NVL(os.Getenv("TB_POSTGRES_PASSWORD"), "tumblebug")
	model.AutocontrolDurationMs = common.NVL(os.Getenv("TB_AUTOCONTROL_DURATION_MS"), "10000")
	model.DefaultNamespace = common.NVL(os.Getenv("TB_DEFAULT_NAMESPACE"), "default")
	model.DefaultCredentialHolder = common.NVL(os.Getenv("TB_DEFAULT_CREDENTIALHOLDER"), "admin")

	// Etcd
	model.EtcdEndpoints = common.NVL(os.Getenv("TB_ETCD_ENDPOINTS"), "localhost:2379")

	// load the latest configuration from DB (if exist)

	log.Info().Msg("[Update system environment]")
	common.UpdateGlobalVariable(model.StrDragonflyRestUrl)
	common.UpdateGlobalVariable(model.StrSpiderRestUrl)
	common.UpdateGlobalVariable(model.TerrariumRestUrl)
	common.UpdateGlobalVariable(model.StrAutocontrolDurationMs)

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

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Seoul",
		strings.Split(model.DBUrl, ":")[0],
		model.DBUser,
		model.DBPassword,
		model.DBDatabase,
		strings.Split(model.DBUrl, ":")[1],
	)

	// Use GORM's default logger in silent mode to avoid verbose output
	model.ORM, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to PostgreSQL database")
	} else {
		log.Info().Msg("PostgreSQL database connected successfully")
	}
	err = model.ORM.AutoMigrate(
		&model.SpecInfo{},
		&model.ImageInfo{},
		&model.LatencyInfo{},
	)

	if err != nil {
		log.Error().Err(err).Msg("")
	} else {
		log.Info().Msg("Database schemas migrated successfully")
	}

	err = addIndexes()
	if err != nil {
		log.Error().Err(err).Msg("Cannot add indexes to the tables (ORM)")
	}

	setConfig()

	_, err = common.GetNs(model.DefaultNamespace)
	if err != nil {
		if model.DefaultNamespace != "" {
			defaultNS := model.NsReq{Name: model.DefaultNamespace, Description: "Default Namespace"}
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
	// common.PrintCloudInfoTable(common.RuntimeCloudInfo)

	//
	// Load networkinfo
	//
	networkInfo := viper.New()
	fileName = "networkinfo"
	networkInfo.AddConfigPath(".")
	networkInfo.AddConfigPath("./assets/")
	networkInfo.AddConfigPath("../assets/")
	networkInfo.SetConfigName(fileName)
	networkInfo.SetConfigType("yaml")
	err = networkInfo.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading networkinfo config file: %w", err))
	}

	log.Info().Msg(networkInfo.ConfigFileUsed())
	err = networkInfo.Unmarshal(&common.RuntimeCloudNetworkInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		panic(err)
	}

	networkInfoJSON, _ := json.MarshalIndent(common.RuntimeCloudNetworkInfo.CSPs["aws"], "", "  ")
	log.Debug().Msgf("common.RuntimeNetworkInfo: %s", string(networkInfoJSON))

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
	// Load extractionpatterns
	//
	extractPatternsViper := viper.New()
	fileName = "extractionpatterns"
	extractPatternsViper.AddConfigPath(".")
	extractPatternsViper.AddConfigPath("./assets/")
	extractPatternsViper.AddConfigPath("../assets/")
	extractPatternsViper.SetConfigName(fileName)
	extractPatternsViper.SetConfigType("yaml")
	err = extractPatternsViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading extractionpatterns config file: %w", err))
	}

	log.Info().Msg(extractPatternsViper.ConfigFileUsed())
	err = extractPatternsViper.Unmarshal(&common.RuntimeExtractPatternsInfo)
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
		log.Info().Msgf("CB-Spider at %s is not ready. Attempt %d/%d", model.SpiderRestUrl, attempt+1, maxAttempts)
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

	etcdEndpoints := strings.Split(model.EtcdEndpoints, ",")

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
		log.Warn().Err(err2).Msgf("etcd at %s is not ready. Attempt %d/%d", model.EtcdEndpoints, etcdAttempt, maxAttempts)
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

	// const mrttArrayYMax = 300
	// common.RuntimeLatancyMap = make([][]string, mrttArrayXMax)

	// Migrate latency data from CSV to database if database is empty
	if err := migrateLatencyDataFromCSV(); err != nil {
		log.Error().Err(err).Msg("Failed to migrate latency data from CSV to database")
	}
}

// migrateLatencyDataFromCSV migrates latency data from CSV file to database
func migrateLatencyDataFromCSV() error {
	// Check if latency data already exists in database
	var count int64
	if err := model.ORM.Model(&model.LatencyInfo{}).Count(&count).Error; err != nil {
		return err
	}

	// If data already exists, skip migration
	if count > 0 {
		log.Info().Msg("Latency data already exists in database, skipping CSV migration")
		return nil
	}

	log.Info().Msg("Starting latency data migration from CSV to database")

	// Read CSV file
	file, err := os.Open("../assets/cloudlatencymap.csv")
	if err != nil {
		log.Error().Err(err).Msg("Failed to open cloudlatencymap.csv file")
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close CSV file")
		}
	}()

	rdr := csv.NewReader(bufio.NewReader(file))
	records, err := rdr.ReadAll()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read CSV data")
		return err
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV file has insufficient data")
	}

	// Extract header (target regions)
	header := records[0]
	var latencyData []model.LatencyInfo

	// Process each row (source region)
	for _, row := range records[1:] {
		if len(row) == 0 || row[0] == "" {
			break
		}

		sourceRegion := row[0]

		// Process each column (target region)
		for j, latencyStr := range row[1:] {
			if j >= len(header)-1 || latencyStr == "" {
				continue
			}

			targetRegion := header[j+1]
			if targetRegion == "" {
				continue
			}

			// Parse latency value
			latencyValue, err := strconv.ParseFloat(latencyStr, 64)
			if err != nil {
				log.Debug().Err(err).Msgf("Skipping invalid latency value '%s' for %s->%s", latencyStr, sourceRegion, targetRegion)
				continue // Skip invalid values
			}

			latencyData = append(latencyData, model.LatencyInfo{
				SourceRegion: sourceRegion,
				TargetRegion: targetRegion,
				LatencyMs:    latencyValue,
			})
		}
	}

	// Batch store to database
	if len(latencyData) > 0 {
		if err := model.BatchStoreLatencyInfo(latencyData); err != nil {
			return err
		}
		log.Info().Msgf("Successfully migrated %d latency records to database", len(latencyData))
	}

	return nil
}

// addIndexes adds indexes to the tables for faster search
func addIndexes() error {
	// Existing single column indexes
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_namespace ON tb_spec_infos (namespace)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_vcpu ON tb_spec_infos (v_cpu)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_memorygib ON tb_spec_infos (memory_gi_b)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_cspspecname ON tb_spec_infos (csp_spec_name)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_costperhour ON tb_spec_infos (cost_per_hour)").Error; err != nil {
		return err
	}

	// Most important: Composite index optimized for the common query pattern
	// This index covers the most frequent query: namespace + architecture + v_cpu + memory_gi_b
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_main_filter 
        ON tb_spec_infos(namespace, architecture, v_cpu, memory_gi_b, cost_per_hour)
    `).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create main filter composite index")
	}

	// Partial index for x86_64 architecture (most common case)
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_x86_64 
        ON tb_spec_infos(namespace, v_cpu, memory_gi_b, cost_per_hour) 
        WHERE architecture = 'x86_64'
    `).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create x86_64 partial index")
	}

	// Partial index for arm64 architecture
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_arm64 
        ON tb_spec_infos(namespace, v_cpu, memory_gi_b, cost_per_hour) 
        WHERE architecture = 'arm64'
    `).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create arm64 partial index")
	}

	// Latency table indexes for fast lookups
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_source ON tb_latency_infos (source_region)").Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create latency source region index")
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_target ON tb_latency_infos (target_region)").Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create latency target region index")
	}

	// Composite index for latency queries (source + target)
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_regions ON tb_latency_infos (source_region, target_region)").Error; err != nil {
		log.Warn().Err(err).Msg("Failed to create latency composite index")
	}

	log.Info().Msg("All indexes created successfully")
	return nil
}

// Main Body

// @title CB-Tumblebug REST API
// @version latest
// @description CB-Tumblebug is an open source system for managing multi-cloud infrastructure consisting of resources from multiple cloud service providers. (Cloud-Barista)
// @termsOfService  https://github.com/cloud-barista/cb-tumblebug/blob/main/README.md

// @contact.name API Support
// @contact.url https://github.com/cloud-barista/cb-tumblebug/issues/new/choose

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath /tumblebug

// @securityDefinitions.basic BasicAuth

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token ([TBD] Get token in http://xxx.xxx.xxx.xxx:xxx/auth)
func main() {

	//Ticker for MCI Orchestration Policy
	log.Info().Msg("[Initiate Multi-Cloud Orchestration]")
	autoControlDuration, _ := strconv.Atoi(model.AutocontrolDurationMs) //ms
	ticker := time.NewTicker(time.Millisecond * time.Duration(autoControlDuration))
	go func() {
		for t := range ticker.C {
			//display ticker if you need (remove '_ = t')
			_ = t
			//fmt.Println("- Orchestration Controller ", t.Format("2006-01-02 15:04:05"))
			infra.OrchestrationController()
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
		restServer.RunServer()
		wg.Done()
	}()

	wg.Wait()
}
