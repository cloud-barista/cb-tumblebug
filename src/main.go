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
	model.APIUsername = common.NVL(os.Getenv("TB_API_USERNAME"), "default")
	model.APIPassword = common.NVL(os.Getenv("TB_API_PASSWORD"), "default")
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

	log.Info().Msg("init: updating system environment")
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

	// Setup and wait for internal services (PostgreSQL, CB-Spider, etcd)
	setupAndWaitForInternalServices()

	err := model.ORM.AutoMigrate(
		&model.SpecInfo{},
		&model.ImageInfo{},
		&model.LatencyInfo{},
	)

	if err != nil {
		log.Error().Err(err).Msg("init: failed to migrate database schemas")
	} else {
		log.Info().Msg("init: database schemas migrated successfully")
	}

	err = addIndexes()
	if err != nil {
		log.Error().Err(err).Msg("init: failed to add indexes to tables")
	}

	setConfig()

	_, err = common.GetNs(model.DefaultNamespace)
	if err != nil {
		if model.DefaultNamespace != "" {
			defaultNS := model.NsReq{Name: model.DefaultNamespace, Description: "Default Namespace"}
			_, err := common.CreateNs(&defaultNS)
			if err != nil {
				log.Error().Err(err).Msg("init: failed to create default namespace")
				panic(err)
			}
		} else {
			log.Error().Msg("init: default namespace is not set")
			panic("Default namespace is not set, please set TB_DEFAULT_NAMESPACE in setup.env or environment variable")
		}
	}
}

// setupAndWaitForInternalServices sets up and waits for all required internal services.
// This function consolidates the setup and connection logic for PostgreSQL, CB-Spider,
// and etcd in one place, ensuring all dependencies are ready before the system starts.
func setupAndWaitForInternalServices() {
	log.Info().Msg("setup: waiting for internal services (PostgreSQL, CB-Spider, etcd)")

	// Create necessary directories for metadata storage
	err := os.MkdirAll("../meta_db/dat/", os.ModePerm)
	if err != nil {
		log.Error().Err(err).Msg("setup: failed to create meta_db directory")
	}

	// Build PostgreSQL DSN
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Seoul",
		strings.Split(model.DBUrl, ":")[0],
		model.DBUser,
		model.DBPassword,
		model.DBDatabase,
		strings.Split(model.DBUrl, ":")[1],
	)

	var wg sync.WaitGroup
	wg.Add(3)

	// 1. Wait for PostgreSQL
	go func() {
		defer wg.Done()
		log.Info().Msg("setup: connecting to PostgreSQL...")
		maxRetries := 30
		retryInterval := 3 * time.Second

		for i := 0; i < maxRetries; i++ {
			db, dbErr := gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger: gormLogger.Default.LogMode(gormLogger.Silent),
			})
			if dbErr == nil {
				sqlDB, sqlErr := db.DB()
				if sqlErr == nil {
					if pingErr := sqlDB.Ping(); pingErr == nil {
						model.ORM = db
						log.Info().Msgf("setup: PostgreSQL is ready (attempt %d)", i+1)
						return
					}
				}
			}
			log.Warn().Msgf("setup: PostgreSQL not ready, retrying (%d/%d)", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
		log.Error().Msg("setup: failed to connect to PostgreSQL after maximum retries")
		panic("PostgreSQL connection failed")
	}()

	// 2. Wait for CB-Spider
	go func() {
		defer wg.Done()
		log.Info().Msg("setup: connecting to CB-Spider...")
		maxRetries := 60
		retryInterval := 3 * time.Second

		for i := 0; i < maxRetries; i++ {
			if err := common.CheckSpiderReady(); err == nil {
				log.Info().Msgf("setup: CB-Spider is ready (attempt %d)", i+1)
				return
			}
			log.Warn().Msgf("setup: CB-Spider not ready, retrying (%d/%d)", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
		log.Error().Msg("setup: failed to connect to CB-Spider after maximum retries")
		panic("CB-Spider connection failed")
	}()

	// 3. Wait for etcd and initialize kvstore
	go func() {
		defer wg.Done()
		log.Info().Msg("setup: connecting to etcd...")
		maxRetries := 10
		retryInterval := 5 * time.Second

		etcdEndpoints := strings.Split(model.EtcdEndpoints, ",")
		for i := range etcdEndpoints {
			etcdEndpoints[i] = strings.TrimSpace(etcdEndpoints[i])
		}

		for i := 0; i < maxRetries; i++ {
			etcdStore, etcdErr := etcd.NewEtcdStore(
				context.Background(),
				etcd.Config{
					Endpoints:   etcdEndpoints,
					DialTimeout: 5 * time.Second,
				},
			)
			if etcdErr == nil {
				if initErr := kvstore.InitializeStore(etcdStore); initErr == nil {
					log.Info().Msgf("setup: etcd is ready (attempt %d)", i+1)
					return
				}
			}
			log.Warn().Msgf("setup: etcd not ready, retrying (%d/%d)", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
		log.Error().Msg("setup: failed to connect to etcd after maximum retries")
		panic("etcd connection failed")
	}()

	wg.Wait()
	log.Info().Msg("setup: all internal services are ready")
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
		log.Error().Err(err).Msg("config: failed to read cloud_conf")
		panic(fmt.Errorf("fatal error reading cloud_conf: %w", err))
	}
	log.Info().Msgf("config: loaded %s", viper.ConfigFileUsed())
	err = viper.Unmarshal(&common.RuntimeConf)
	if err != nil {
		log.Error().Err(err).Msg("config: failed to unmarshal cloud_conf")
		panic(err)
	}

	// Load cloudinfo
	cloudInfoViper := viper.New()
	fileName = "cloudinfo"
	common.SetupViperPaths(cloudInfoViper)
	cloudInfoViper.SetConfigName(fileName)
	cloudInfoViper.SetConfigType("yaml")
	err = cloudInfoViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading cloudinfo config file: %w", err))
	}

	log.Info().Msgf("config: loaded %s", cloudInfoViper.ConfigFileUsed())
	err = cloudInfoViper.Unmarshal(&common.RuntimeCloudInfo)
	if err != nil {
		log.Error().Err(err).Msg("config: failed to unmarshal cloudinfo")
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
	common.SetupViperPaths(networkInfo)
	networkInfo.SetConfigName(fileName)
	networkInfo.SetConfigType("yaml")
	err = networkInfo.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading networkinfo config file: %w", err))
	}

	log.Info().Msgf("config: loaded %s", networkInfo.ConfigFileUsed())
	err = networkInfo.Unmarshal(&common.RuntimeCloudNetworkInfo)
	if err != nil {
		log.Error().Err(err).Msg("config: failed to unmarshal networkinfo")
		panic(err)
	}

	networkInfoJSON, _ := json.MarshalIndent(common.RuntimeCloudNetworkInfo.CSPs["aws"], "", "  ")
	log.Debug().Msgf("config: RuntimeNetworkInfo sample (aws): %s", string(networkInfoJSON))

	//
	// Load k8sclusterinfo
	//
	k8sClusterInfoViper := viper.New()
	fileName = "k8sclusterinfo"
	common.SetupViperPaths(k8sClusterInfoViper)
	k8sClusterInfoViper.SetConfigName(fileName)
	k8sClusterInfoViper.SetConfigType("yaml")
	err = k8sClusterInfoViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading cloudinfo config file: %w", err))
	}

	log.Info().Msgf("config: loaded %s", k8sClusterInfoViper.ConfigFileUsed())
	err = k8sClusterInfoViper.Unmarshal(&common.RuntimeK8sClusterInfo)
	if err != nil {
		log.Error().Err(err).Msg("config: failed to unmarshal k8sclusterinfo")
		panic(err)
	}

	//
	// Load extractionpatterns
	//
	extractPatternsViper := viper.New()
	fileName = "extractionpatterns"
	common.SetupViperPaths(extractPatternsViper)
	extractPatternsViper.SetConfigName(fileName)
	extractPatternsViper.SetConfigType("yaml")
	err = extractPatternsViper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading extractionpatterns config file: %w", err))
	}

	log.Info().Msgf("config: loaded %s", extractPatternsViper.ConfigFileUsed())
	err = extractPatternsViper.Unmarshal(&common.RuntimeExtractPatternsInfo)
	if err != nil {
		log.Error().Err(err).Msg("config: failed to unmarshal extractionpatterns")
		panic(err)
	}

	// Register all cloud info
	err = common.RegisterAllCloudInfo()
	if err != nil {
		log.Error().Err(err).Msg("config: failed to register cloud info")
		panic(err)
	}

	// // Load credentials
	// usr, err := user.Current()
	// if err != nil {
	// 	log.Error().Err(err).Msg("config: failed to get current user")
	// }
	// credPath := usr.HomeDir + "/.cloud-barista"
	// credViper := viper.New()
	// fileName = "credentials"
	// credViper.AddConfigPath(credPath)
	// credViper.SetConfigName(fileName)
	// credViper.SetConfigType("yaml")
	// err = credViper.ReadInConfig()
	// if err != nil {
	// 	log.Info().Msg("config: local credentials file not found, continuing without it")
	// } else {
	// 	log.Info().Msgf("config: loaded %s", credViper.ConfigFileUsed())
	// 	err = credViper.Unmarshal(&common.RuntimeCredential)
	// 	if err != nil {
	// 		log.Error().Err(err).Msg("config: failed to unmarshal credentials")
	// 		panic(err)
	// 	}
	// 	// common.PrintCredentialInfo(common.RuntimeCredential)
	// }

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
		log.Error().Err(err).Msg("config: failed to migrate latency data from CSV")
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
		log.Info().Msg("migrate: latency data already exists in database, skipping CSV migration")
		return nil
	}

	log.Info().Msg("migrate: starting latency data migration from CSV")

	// Read CSV file
	csvPath := common.GetAssetsFilePath("cloudlatencymap.csv")
	file, err := os.Open(csvPath)
	if err != nil {
		log.Error().
			Err(err).
			Str("attempted_path", csvPath).
			Msg("migrate: failed to open cloudlatencymap.csv")
		return fmt.Errorf("failed to open cloudlatencymap.csv at %s: %w", csvPath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("migrate: failed to close CSV file")
		}
	}()

	rdr := csv.NewReader(bufio.NewReader(file))
	records, err := rdr.ReadAll()
	if err != nil {
		log.Error().Err(err).Msg("migrate: failed to read CSV data")
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
				log.Debug().Err(err).Msgf("migrate: skipping invalid latency value '%s' for %s->%s", latencyStr, sourceRegion, targetRegion)
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
		log.Info().Msgf("migrate: successfully migrated %d latency records to database", len(latencyData))
	}

	return nil
}

// addIndexes adds indexes to the tables for faster search
func addIndexes() error {
	// Existing single column indexes
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_namespace ON spec_infos (namespace)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_vcpu ON spec_infos (v_cpu)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_memorygib ON spec_infos (memory_gi_b)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_cspspecname ON spec_infos (csp_spec_name)").Error; err != nil {
		return err
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_costperhour ON spec_infos (cost_per_hour)").Error; err != nil {
		return err
	}

	// Most important: Composite index optimized for the common query pattern
	// This index covers the most frequent query: namespace + architecture + v_cpu + memory_gi_b
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_main_filter 
        ON spec_infos(namespace, architecture, v_cpu, memory_gi_b, cost_per_hour)
    `).Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create main filter composite index")
	}

	// Partial index for x86_64 architecture (most common case)
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_x86_64 
        ON spec_infos(namespace, v_cpu, memory_gi_b, cost_per_hour) 
        WHERE architecture = 'x86_64'
    `).Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create x86_64 partial index")
	}

	// Partial index for arm64 architecture
	if err := model.ORM.Exec(`
        CREATE INDEX IF NOT EXISTS idx_spec_arm64 
        ON spec_infos(namespace, v_cpu, memory_gi_b, cost_per_hour) 
        WHERE architecture = 'arm64'
    `).Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create arm64 partial index")
	}

	// Latency table indexes for fast lookups
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_source ON latency_infos (source_region)").Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create latency source region index")
	}

	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_target ON latency_infos (target_region)").Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create latency target region index")
	}

	// Composite index for latency queries (source + target)
	if err := model.ORM.Exec("CREATE INDEX IF NOT EXISTS idx_latency_regions ON latency_infos (source_region, target_region)").Error; err != nil {
		log.Warn().Err(err).Msg("init: failed to create latency composite index")
	}

	log.Info().Msg("init: all database indexes created successfully")
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

// @tag.name [Admin] System Management
// @tag.description System configuration, namespace management, and administrative operations

// @tag.name [Admin] Cloud Credential Management
// @tag.description Cloud credential and authentication management

// @tag.name [Admin] Multi-Cloud Information
// @tag.description Multi-cloud provider information and metadata

// @tag.name [Admin] Provisioning History and Analytics
// @tag.description Provisioning event history and risk analytics

// @tag.name [MC-Infra] MCI Provisioning and Management
// @tag.description Multi-Cloud Infrastructure provisioning, lifecycle management, and operations

// @tag.name [MC-Infra] MCI Remote Command
// @tag.description Execute commands or transfer files remotely on VMs in MCI via SSH

// @tag.name [Kubernetes] Cluster Management
// @tag.description Kubernetes cluster provisioning and management

// @tag.name [Kubernetes] Cluster's Container Remote Command
// @tag.description Execute commands in Kubernetes cluster containers

// @tag.name [Job Scheduler] (WIP) CSP Resource Registration
// @tag.description Scheduled CSP resource registration jobs (Work In Progress)

// @tag.name [Infra Resource] Common Utility
// @tag.description Common utility functions for infrastructure resources

// @tag.name [Infra Resource] Spec Management
// @tag.description VM specification recommendation and management

// @tag.name [Infra Resource] Image Management
// @tag.description VM image lookup, registration, and management

// @tag.name [Infra Resource] Network Management
// @tag.description Virtual network (VNet/VPC) and subnet management

// @tag.name [Infra Resource] Security Group Management
// @tag.description Security group and firewall rule management

// @tag.name [Infra Resource] Access Key Management
// @tag.description SSH key pair management for VM access

// @tag.name [Infra Resource] Data Disk Management
// @tag.description Additional data disk management for VMs

// @tag.name [Infra Resource] Object Storage Management
// @tag.description Object storage bucket and object management

// @tag.name [Infra Resource] SQL Database Management (under development)
// @tag.description Managed SQL database service operations

// @tag.name [Infra Resource] NLB Management
// @tag.description Network Load Balancer management

// @tag.name [Infra Resource] NLB Management (for developer)
// @tag.description Advanced NLB management operations for developers

// @tag.name [Infra Resource] Site-to-site VPN Management (under development)
// @tag.description Site-to-site VPN tunnel management

// @tag.name [Admin] System Configuration
// @tag.description System settings and configuration management

// @tag.name [Admin] API Request Management
// @tag.description API request tracking and management

// @tag.name [MC-Infra] MCI Performance Benchmarking (WIP)
// @tag.description Performance benchmark operations for MCI (Work In Progress)

// @tag.name [MC-Infra] MCI Orchestration Management (WIP)
// @tag.description MCI orchestration policy and automation (Work In Progress)

// @tag.name [MC-Infra] MCI Resource Monitor (for developer)
// @tag.description MCI resource monitoring operations for developers

// @tag.name [Test] Stream Response
// @tag.description Test endpoints for streaming responses

func main() {

	//Ticker for MCI Orchestration Policy
	log.Info().Msg("main: initiating multi-cloud orchestration")
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
			log.Info().Msgf("main: config file changed: %s", e.Name)
			err := viper.ReadInConfig()
			if err != nil { // Handle errors reading the config file
				log.Error().Err(err).Msg("main: failed to reload config file")
				panic(fmt.Errorf("fatal error config file: %w", err))
			}
			err = viper.Unmarshal(&common.RuntimeConf)
			if err != nil {
				log.Error().Err(err).Msg("main: failed to unmarshal reloaded config")
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
