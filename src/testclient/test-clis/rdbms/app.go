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

// Package main provides a CLI tool for batch-testing the RDBMS lifecycle
// (rdbms/support once per batch → per case: vNet+subnets → securityGroup →
// rdbms/capability → RDBMS create → get → list → database create/list/dummy-data-test/delete
// → RDBMS delete → securityGroup delete → subnets delete → vNet delete) across multiple CSPs
// via the CB-Tumblebug API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var tbApiBase string

func init() {
	setConfig()
	tbApiBase = viper.GetString("tumblebug.endpoint") + "/tumblebug"
}

// setConfig loads settings from a specified config file or test-config.yaml and .env
func setConfig(cfgFile ...string) {
	if len(cfgFile) > 0 && cfgFile[0] != "" {
		viper.SetConfigFile(cfgFile[0])
	} else {
		viper.SetConfigName("test-config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Error reading config file")
	}
	log.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())

	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	if err := viper.MergeInConfig(); err != nil {
		log.Warn().Msg("No .env file found, relying on environment variables or defaults")
	}

	viper.AutomaticEnv()
}

// ApiLog records a single API call's details for the report.
type ApiLog struct {
	Step            string
	Method          string
	URL             string
	RequestPayload  any
	ResponsePayload any
	ResponseStatus  string
	ElapsedTime     string
}

// SubnetConfig is one subnet to create under the test case's vNet.
type SubnetConfig struct {
	Name string `mapstructure:"name"`
	CIDR string `mapstructure:"cidr"`
	Zone string `mapstructure:"zone"`
}

// TestCase represents a single CSP test case from the config file.
type TestCase struct {
	RdbmsId                  string         `mapstructure:"rdbmsId"`
	ConnectionName           string         `mapstructure:"connectionName"`
	VNetName                 string         `mapstructure:"vNetName"`
	CidrBlock                string         `mapstructure:"cidrBlock"`
	Subnets                  []SubnetConfig `mapstructure:"subnets"`
	SecurityGroupName        string         `mapstructure:"securityGroupName"`
	DBEngine                 string         `mapstructure:"dbEngine"`
	DBEngineVersion          string         `mapstructure:"dbEngineVersion"`
	DBInstanceSpec           string         `mapstructure:"dbInstanceSpec"`
	DBSpec                   string         `mapstructure:"dbSpec"`
	StorageType              string         `mapstructure:"storageType"`
	StorageSize              int            `mapstructure:"storageSize"`
	AutoFillDefaults         bool           `mapstructure:"autoFillDefaults"`
	AdminUserName            string         `mapstructure:"adminUserName"`
	AdminUserPassword        string         `mapstructure:"adminUserPassword"`
	PublicAccess             bool           `mapstructure:"publicAccess"`
	NHNDBSGToAllowAllInbound bool           `mapstructure:"nhnDBSGToAllowAllInbound"`
	HighAvailability         bool           `mapstructure:"highAvailability"`
	// DatabaseName is the logical database created/listed/deleted inside the RDBMS instance
	// (defaults to "sampledb" if left blank).
	DatabaseName string `mapstructure:"databaseName"`
	Execute      bool   `mapstructure:"execute"`

	// Internal VM test configuration (runs SQL from a test VM inside the same VPC via Remote Command)
	InternalDataIOTest bool   `mapstructure:"internalDataIOTest"`
	VmImageId          string `mapstructure:"vmImageId"`
	VmSpecId           string `mapstructure:"vmSpecId"`
	VmOSType           string `mapstructure:"vmOSType"`
	VmvCPU             string `mapstructure:"vmvCPU"`
	VmMemoryGiB        string `mapstructure:"vmMemoryGiB"`
}

// TestResult holds the outcome of each lifecycle step for one CSP.
type TestResult struct {
	RdbmsId                  string
	ConnectionName           string
	DBEngine                 string
	CreateVNetStatus         string
	CreateSGStatus           string
	SupportStatus            string
	CapabilityStatus         string
	ValidateStatus           string
	CreateRDBMSStatus        string
	GetRDBMSStatus           string
	ListRDBMSStatus          string
	CreateDatabaseStatus     string
	ListDatabaseStatus       string
	RemoteDataIOTestStatus   string
	InternalDataIOTestStatus string
	DeleteDatabaseStatus     string
	DeleteRDBMSStatus        string
	DeleteSGStatus           string
	DeleteSubnetsStatus      string
	DeleteVNetStatus         string
	Note                     string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "./app",
		Short: "RDBMS batch test CLI",
		Long: `
##########################################################################
## RDBMS batch test CLI for CB-Tumblebug                                ##
## Runs rdbms/support once, then per case: vNet+subnets -> securityGroup##
## -> rdbms/capability -> RDBMS create -> get -> list -> database       ##
## create/list/dummy-data-test/delete -> RDBMS delete ->                ##
## securityGroup/subnets/vNet delete                                    ##
##########################################################################`,
	}

	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Run the full RDBMS lifecycle test for all enabled CSPs",
		Run:   runBatchTest,
	}
	testCmd.Flags().StringP("config", "c", "", "Config file path (default: test-config.yaml)")
	testCmd.Flags().StringP("nsId", "n", "", "Namespace ID (overrides config)")
	testCmd.Flags().Bool("parallel", false, "Run test cases in parallel")

	rootCmd.AddCommand(testCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Error executing RDBMS batch test CLI")
	}
}

// runBatchTest executes the full lifecycle test for every enabled test case.
// With --parallel the cases run concurrently; without it they run sequentially.
// Each test case's own steps always run sequentially, since RDBMS creation
// depends on the vNet/subnet/securityGroup created earlier in the same case.
func runBatchTest(cmd *cobra.Command, args []string) {
	cfgFile, _ := cmd.Flags().GetString("config")
	if cfgFile != "" {
		setConfig(cfgFile)
		tbApiBase = viper.GetString("tumblebug.endpoint") + "/tumblebug"
	}

	nsId, _ := cmd.Flags().GetString("nsId")
	parallel, _ := cmd.Flags().GetBool("parallel")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.nsId")
	}
	if nsId == "" {
		log.Fatal().Msg("nsId is required (flag --nsId or config tumblebug.nsId)")
		return
	}

	tbAuth := getAuth()

	// Readiness check
	urlReadiness := fmt.Sprintf("%s/readyz", tbApiBase)
	if _, err := callApi("GET", urlReadiness, tbAuth, nil, nil, "Readiness Check"); err != nil {
		log.Fatal().Err(err).Msg("CB-Tumblebug is not ready")
		return
	}

	// Static, CSP-wide RDBMS support matrix (batch-level; no connection needed, so this
	// runs once rather than per test case). Best-effort: failure here does not block the
	// per-case lifecycle tests below; each case just reports SupportStatus as unknown.
	var supportMatrix model.RDBMSSupportResponse
	urlSupport := fmt.Sprintf("%s/rdbms/support", tbApiBase)
	if respBytes, err := callApi("GET", urlSupport, tbAuth, nil, nil, "Get RDBMS Support Matrix"); err != nil {
		log.Warn().Err(err).Msg("Get RDBMS Support Matrix failed (non-blocking)")
	} else {
		_ = json.Unmarshal(respBytes, &supportMatrix)
		log.Info().Msgf("Get RDBMS Support Matrix OK (%d CSPs)", len(supportMatrix.Supports))
	}

	// Load enabled test cases
	var allCases []TestCase
	if err := viper.UnmarshalKey("testCases", &allCases); err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal testCases from config")
		return
	}

	var cases []TestCase
	for _, tc := range allCases {
		if tc.Execute {
			cases = append(cases, tc)
		} else {
			log.Info().Msgf("Skipping (execute=false): rdbmsId=%s", tc.RdbmsId)
		}
	}

	if len(cases) == 0 {
		log.Warn().Msg("No test cases to run (set execute: true in test-config.yaml)")
		return
	}

	mode := "sequential"
	if parallel {
		mode = "parallel"
	}
	log.Info().Msgf("Running %d test case(s) in %s mode (RDBMS creation can take several minutes per case)", len(cases), mode)

	results := make([]TestResult, len(cases))

	if parallel {
		var wg sync.WaitGroup
		for i, tc := range cases {
			wg.Add(1)
			go func(idx int, tc TestCase) {
				defer wg.Done()
				results[idx] = runLifecycle(nsId, tc, tbAuth, supportMatrix)
			}(i, tc)
		}
		wg.Wait()
	} else {
		for i, tc := range cases {
			results[i] = runLifecycle(nsId, tc, tbAuth, supportMatrix)
		}
	}

	summaryMarkdown := buildSummaryMarkdown(results)
	if err := os.MkdirAll("test-results", 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create test-results directory")
	} else if err := os.WriteFile("test-results/summary.md", []byte(summaryMarkdown), 0644); err != nil {
		log.Warn().Err(err).Msgf("Failed to write summary report")
	}

	// One CSP-by-step pass/fail matrix per dbEngine present in this run (e.g.
	// test-results/summary-mysql.md, test-results/summary-mariadb.md), complementing
	// summary.md's per-CSP vertical view with an at-a-glance comparison across CSPs.
	byEngine := make(map[string][]TestResult)
	var engineOrder []string
	for _, r := range results {
		key := strings.ToLower(r.DBEngine)
		if key == "" {
			key = "unknown"
		}
		if _, seen := byEngine[key]; !seen {
			engineOrder = append(engineOrder, key)
		}
		byEngine[key] = append(byEngine[key], r)
	}
	for _, engine := range engineOrder {
		matrixMarkdown := buildEngineMatrixMarkdown(byEngine[engine], engine)
		filename := fmt.Sprintf("test-results/summary-%s.md", engine)
		if err := os.WriteFile(filename, []byte(matrixMarkdown), 0644); err != nil {
			log.Warn().Err(err).Msgf("Failed to write %s", filename)
		} else {
			log.Info().Msgf("Wrote CSP-by-step matrix: %s", filename)
		}
	}

	// Printed directly (not via the zerolog logger, which wraps every line in JSON) so
	// this can be copied straight into docs/PRs as-is — it's the same markdown saved to
	// test-results/summary.md.
	fmt.Println()
	fmt.Println(summaryMarkdown)
}

// runLifecycle runs the full chain for one test case and returns the result:
//  1. Create vNet (with embedded subnets)
//  2. Create SecurityGroup (inbound rule for the DB engine's port)
//  3. Look up RDBMS support (static matrix, fetched once per batch) and RDBMS capability (live)
//  4. Validate RDBMS create request (dry run), then create RDBMS (blocks until Available/Failed)
//  5. Get RDBMS (single)
//  6. List RDBMS (verify the instance appears)
//  7. Create Database
//  8. List Database (confirm it appears)
//  9. Dummy data test (direct SQL write/read/verify/delete against the created database)
//  10. Delete Database
//  11. Delete RDBMS
//  12. Delete SecurityGroup
//  13. Delete each Subnet
//  14. Delete vNet
//
// Steps 10-14 always run (best-effort, in reverse-dependency order) even if an
// earlier step failed, so a failed run doesn't leave billed CSP resources behind.
func runLifecycle(nsId string, tc TestCase, tbAuth map[string]string, supportMatrix model.RDBMSSupportResponse) TestResult {
	result := TestResult{RdbmsId: tc.RdbmsId, ConnectionName: tc.ConnectionName, DBEngine: tc.DBEngine}
	logs := []ApiLog{}

	log.Info().Msgf("[%s] ====== START (connection=%s) ======", tc.RdbmsId, tc.ConnectionName)

	var vNetId string
	var subnetIds []string
	var sgId string
	var rdbmsCreated bool
	var rdbmsEndpoint string
	var databaseCreated bool
	dbName := tc.DatabaseName
	if dbName == "" {
		dbName = "sampledb"
	}

	// 0b. Pre-flight Spec & Image Discovery, Recommendation, and Review (if InternalDataIOTest is enabled)
	resolveAndReviewSpecAndImage(tbApiBase, nsId, &tc, tbAuth, &logs)

	// 1. Create vNet (with embedded subnets)
	subnetReqs := make([]map[string]any, 0, len(tc.Subnets))
	for _, s := range tc.Subnets {
		subnetReqs = append(subnetReqs, map[string]any{
			"name":      s.Name,
			"ipv4_CIDR": s.CIDR,
			"zone":      s.Zone,
		})
	}
	vNetReqBody := map[string]any{
		"name":           tc.VNetName,
		"connectionName": tc.ConnectionName,
		"cidrBlock":      tc.CidrBlock,
		"subnetInfoList": subnetReqs,
		"description":    "created by RDBMS batch test CLI",
	}
	urlCreateVNet := fmt.Sprintf("%s/ns/%s/resources/vNet", tbApiBase, nsId)
	respBytes, err := callApi("POST", urlCreateVNet, tbAuth, vNetReqBody, &logs, fmt.Sprintf("[%s] Create VNet", tc.RdbmsId))
	var vNetInfo model.VNetInfo
	if err != nil {
		result.CreateVNetStatus = "Failed"
		log.Error().Err(err).Msgf("[%s] Create VNet failed", tc.RdbmsId)
	} else {
		_ = json.Unmarshal(respBytes, &vNetInfo)
		vNetId = vNetInfo.Id
		for _, s := range vNetInfo.SubnetInfoList {
			subnetIds = append(subnetIds, s.Id)
		}
		result.CreateVNetStatus = fmt.Sprintf("Success (id=%s, subnets=%d)", vNetId, len(subnetIds))
		log.Info().Msgf("[%s] Create VNet OK: id=%s, subnets=%d", tc.RdbmsId, vNetId, len(subnetIds))
	}

	// 2. Create SecurityGroup (only if VNet succeeded)
	if vNetId != "" {
		port := dbPortForEngine(tc.DBEngine)
		firewallRules := []map[string]any{
			{"Ports": port, "Protocol": "TCP", "Direction": "inbound", "CIDR": "0.0.0.0/0"},
		}
		if tc.InternalDataIOTest {
			firewallRules = append(firewallRules, map[string]any{
				"Ports": "22", "Protocol": "TCP", "Direction": "inbound", "CIDR": "0.0.0.0/0",
			})
		}
		sgReqBody := map[string]any{
			"name":           tc.SecurityGroupName,
			"connectionName": tc.ConnectionName,
			"vNetId":         vNetId,
			"description":    "created by RDBMS batch test CLI",
			"firewallRules":  firewallRules,
		}
		urlCreateSG := fmt.Sprintf("%s/ns/%s/resources/securityGroup", tbApiBase, nsId)
		respBytes, err = callApi("POST", urlCreateSG, tbAuth, sgReqBody, &logs, fmt.Sprintf("[%s] Create SecurityGroup", tc.RdbmsId))
		if err != nil {
			result.CreateSGStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Create SecurityGroup failed", tc.RdbmsId)
		} else {
			var sgInfo model.SecurityGroupInfo
			_ = json.Unmarshal(respBytes, &sgInfo)
			sgId = sgInfo.Id
			result.CreateSGStatus = fmt.Sprintf("Success (id=%s, port=%s)", sgId, port)
			log.Info().Msgf("[%s] Create SecurityGroup OK: id=%s", tc.RdbmsId, sgId)
		}
	} else {
		result.CreateSGStatus = "Skipped (no VNet)"
	}

	// 3. RDBMS support (static, CSP-wide matrix fetched once per batch — just a lookup
	// here, no API call) and capability (live, per-connection/engine check; best-effort,
	// failure does not block create).
	if vNetId != "" {
		providerName := vNetInfo.ConnectionConfig.ProviderName
		regionName := vNetInfo.ConnectionConfig.RegionZoneInfo.AssignedRegion

		if info, ok := supportMatrix.Supports[strings.ToLower(providerName)]; ok {
			if info.Supported {
				result.SupportStatus = fmt.Sprintf("Supported (engines: %s)", strings.Join(info.SupportedDBEngines, ","))
			} else {
				result.SupportStatus = "Not Supported"
			}
		} else {
			result.SupportStatus = "Unknown (matrix unavailable)"
		}

		urlCapability := fmt.Sprintf("%s/rdbms/capability?providerName=%s&regionName=%s&dbEngine=%s",
			tbApiBase, providerName, regionName, tc.DBEngine)
		capBytes, err := callApi("GET", urlCapability, tbAuth, nil, &logs, fmt.Sprintf("[%s] Get RDBMS Capability", tc.RdbmsId))
		if err != nil {
			result.CapabilityStatus = "Failed"
			log.Warn().Err(err).Msgf("[%s] Get RDBMS Capability failed", tc.RdbmsId)
		} else {
			result.CapabilityStatus = "Success"
			log.Info().Msgf("[%s] Get RDBMS Capability OK", tc.RdbmsId)
			// Content sanity check, not just HTTP status — a Spider wire-format mismatch can return 200 with silently zeroed/empty data.
			var capResp model.RDBMSCapabilityResponse
			if jsonErr := json.Unmarshal(capBytes, &capResp); jsonErr == nil {
				s := capResp.Supports
				if s.SupportsStorageSizeConfiguration && s.StorageSizeRange.Min == 0 && s.StorageSizeRange.Max == 0 {
					log.Warn().Msgf("[%s] Capability response looks suspicious: supportsStorageSizeConfiguration=true but storageSizeRange is {0,0} (possible Spider wire-format mismatch)", tc.RdbmsId)
				}
				log.Info().Msgf("[%s] Capability data: dbInstanceSpecs=%d liveSupportedEngines=%v requiresSG=%v", tc.RdbmsId, len(s.DBInstanceSpecs), s.LiveSupportedEngines, s.RequiresSecurityGroup)
			}
		}
	} else {
		result.SupportStatus = "Skipped (no VNet)"
		result.CapabilityStatus = "Skipped (no VNet)"
	}

	// 4. Validate, then create RDBMS (only if VNet succeeded; SecurityGroup is best-effort)
	if vNetId != "" {
		specVal := tc.DBInstanceSpec
		if specVal == "" {
			specVal = tc.DBSpec
		}
		rdbmsReqBody := map[string]any{
			"name":              tc.RdbmsId,
			"connectionName":    tc.ConnectionName,
			"vNetId":            vNetId,
			"subnetIds":         subnetIds,
			"dbEngine":          tc.DBEngine,
			"dbEngineVersion":   tc.DBEngineVersion,
			"dbInstanceSpec":    specVal,
			"storageType":       tc.StorageType,
			"storageSize":       tc.StorageSize,
			"adminUserName":     tc.AdminUserName,
			"adminUserPassword": tc.AdminUserPassword,
			"highAvailability":  tc.HighAvailability,
			"publicAccess":      tc.PublicAccess,
			"autoFillDefaults":  tc.AutoFillDefaults,
			"description":       "created by RDBMS batch test CLI",
		}
		if tc.NHNDBSGToAllowAllInbound {
			rdbmsReqBody["nhnDBSGToAllowAllInbound"] = true
		}
		if sgId != "" {
			rdbmsReqBody["securityGroupIds"] = []string{sgId}
		}

		// 4a. Validate first (dry run, no side effects) — catches request/capability
		// problems (e.g. an autoFillDefaults pick that fails assets/rdbmsinfo.yaml's own
		// storage-type constraints) without waiting on the up-to-20-minute create call.
		urlValidate := fmt.Sprintf("%s/ns/%s/resources/rdbms/validate", tbApiBase, nsId)
		_, err = callApi("POST", urlValidate, tbAuth, rdbmsReqBody, &logs, fmt.Sprintf("[%s] Validate RDBMS", tc.RdbmsId))
		if err != nil {
			result.ValidateStatus = "Failed"
			result.CreateRDBMSStatus = "Skipped (validation failed)"
			log.Warn().Err(err).Msgf("[%s] Validate RDBMS failed; skipping create", tc.RdbmsId)
		} else {
			result.ValidateStatus = "Success"
			log.Info().Msgf("[%s] Validate RDBMS OK", tc.RdbmsId)

			// 4b. Create RDBMS
			urlCreateRDBMS := fmt.Sprintf("%s/ns/%s/resources/rdbms", tbApiBase, nsId)
			log.Info().Msgf("[%s] Creating RDBMS (this can take several minutes)...", tc.RdbmsId)
			respBytes, err = callApi("POST", urlCreateRDBMS, tbAuth, rdbmsReqBody, &logs, fmt.Sprintf("[%s] Create RDBMS", tc.RdbmsId))
			if err != nil {
				result.CreateRDBMSStatus = "Failed"
				log.Error().Err(err).Msgf("[%s] Create RDBMS failed", tc.RdbmsId)
			} else {
				var info model.RDBMSInfo
				_ = json.Unmarshal(respBytes, &info)
				rdbmsCreated = true
				rdbmsEndpoint = info.Endpoint
				result.CreateRDBMSStatus = fmt.Sprintf("Success (status=%s, endpoint=%s)", info.Status, info.Endpoint)
				log.Info().Msgf("[%s] Create RDBMS OK: status=%s", tc.RdbmsId, info.Status)
			}
		}
	} else {
		result.ValidateStatus = "Skipped (no VNet)"
		result.CreateRDBMSStatus = "Skipped (no VNet)"
	}

	// 5. Get RDBMS (single)
	if rdbmsCreated {
		urlGet := fmt.Sprintf("%s/ns/%s/resources/rdbms/%s", tbApiBase, nsId, tc.RdbmsId)
		respBytes, err = callApi("GET", urlGet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Get RDBMS", tc.RdbmsId))
		if err != nil {
			result.GetRDBMSStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Get RDBMS failed", tc.RdbmsId)
		} else {
			var info model.RDBMSInfo
			_ = json.Unmarshal(respBytes, &info)
			result.GetRDBMSStatus = fmt.Sprintf("Success (status=%s)", info.Status)
			log.Info().Msgf("[%s] Get RDBMS OK: status=%s", tc.RdbmsId, info.Status)
		}
	} else {
		result.GetRDBMSStatus = "Skipped (not created)"
	}

	// 6. List RDBMS (verify the instance appears)
	if rdbmsCreated {
		urlList := fmt.Sprintf("%s/ns/%s/resources/rdbms", tbApiBase, nsId)
		respBytes, err = callApi("GET", urlList, tbAuth, nil, &logs, fmt.Sprintf("[%s] List RDBMS", tc.RdbmsId))
		if err != nil {
			result.ListRDBMSStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] List RDBMS failed", tc.RdbmsId)
		} else {
			var list model.RDBMSListResponse
			_ = json.Unmarshal(respBytes, &list)
			found := false
			for _, r := range list.RDBMS {
				if r.Id == tc.RdbmsId {
					found = true
					break
				}
			}
			if found {
				result.ListRDBMSStatus = fmt.Sprintf("Success (found among %d)", len(list.RDBMS))
			} else {
				result.ListRDBMSStatus = fmt.Sprintf("Failed (not found among %d)", len(list.RDBMS))
			}
			log.Info().Msgf("[%s] List RDBMS: %s", tc.RdbmsId, result.ListRDBMSStatus)
		}
	} else {
		result.ListRDBMSStatus = "Skipped (not created)"
	}

	// 7. Create Database (only if RDBMS was created)
	if rdbmsCreated {
		urlCreateDB := fmt.Sprintf("%s/ns/%s/resources/rdbms/%s/database", tbApiBase, nsId, tc.RdbmsId)
		dbReqBody := map[string]any{
			"databaseName":      dbName,
			"adminUserPassword": tc.AdminUserPassword,
		}
		_, err = callApi("POST", urlCreateDB, tbAuth, dbReqBody, &logs, fmt.Sprintf("[%s] Create Database", tc.RdbmsId))
		if err != nil {
			result.CreateDatabaseStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Create Database failed", tc.RdbmsId)
		} else {
			databaseCreated = true
			result.CreateDatabaseStatus = fmt.Sprintf("Success (name=%s)", dbName)
			log.Info().Msgf("[%s] Create Database OK: name=%s", tc.RdbmsId, dbName)
		}
	} else {
		result.CreateDatabaseStatus = "Skipped (RDBMS not created)"
	}

	// 8. List Database (verify it appears; X-Admin-User-Password is optional here per
	// docs/feature_guide/rdbms-management.md's Features section, but supplied anyway since
	// we have it)
	if databaseCreated {
		urlListDB := fmt.Sprintf("%s/ns/%s/resources/rdbms/%s/database", tbApiBase, nsId, tc.RdbmsId)
		headers := map[string]string{"X-Admin-User-Password": tc.AdminUserPassword}
		respBytes, err = callApi("GET", urlListDB, tbAuth, nil, &logs, fmt.Sprintf("[%s] List Database", tc.RdbmsId), headers)
		if err != nil {
			result.ListDatabaseStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] List Database failed", tc.RdbmsId)
		} else {
			var dbList model.RDBMSDatabaseListResponse
			_ = json.Unmarshal(respBytes, &dbList)
			found := false
			for _, d := range dbList.Databases {
				if d == dbName {
					found = true
					break
				}
			}
			if found {
				result.ListDatabaseStatus = fmt.Sprintf("Success (found among %d)", len(dbList.Databases))
			} else {
				result.ListDatabaseStatus = fmt.Sprintf("Failed (not found among %d)", len(dbList.Databases))
			}
			log.Info().Msgf("[%s] List Database: %s", tc.RdbmsId, result.ListDatabaseStatus)
		}
	} else {
		result.ListDatabaseStatus = "Skipped (database not created)"
	}

	// 9. Data Tests (External Remote vs Internal VPC)
	providerKey := strings.ToLower(vNetInfo.ConnectionConfig.ProviderName)

	// 9a. Remote Data I/O Test: connect directly from local machine (MySQL wire protocol)
	if databaseCreated {
		if providerKey == "ncp" || (!tc.PublicAccess && !strings.Contains(rdbmsEndpoint, ".external.")) {
			result.RemoteDataIOTestStatus = "N/A (Private endpoint only)"
			log.Info().Msgf("[%s] Remote Data I/O Test: N/A (Private VPC endpoint only: %s)", tc.RdbmsId, rdbmsEndpoint)
		} else {
			start := time.Now()
			dummyErr := testDatabaseDummyData(rdbmsEndpoint, tc.AdminUserName, tc.AdminUserPassword, dbName)
			entry := ApiLog{
				Step:           fmt.Sprintf("[%s] Remote Data I/O Test", tc.RdbmsId),
				Method:         "SQL",
				URL:            fmt.Sprintf("%s/%s", rdbmsEndpoint, dbName),
				RequestPayload: map[string]any{"table": "tumblebug_test", "operations": "CREATE TABLE, INSERT, SELECT, DELETE"},
				ElapsedTime:    time.Since(start).Round(time.Millisecond).String(),
			}
			if dummyErr != nil {
				entry.ResponseStatus = "Failed"
				entry.ResponsePayload = map[string]any{"error": dummyErr.Error()}
				if providerKey == "nhn" && !tc.NHNDBSGToAllowAllInbound {
					result.RemoteDataIOTestStatus = "Fail (Note: Requires DB Security Group rule in NHN Console or nhnDBSGToAllowAllInbound=true)"
					result.Note = "NHN requires an inbound permit rule in NHN Console (DB 보안 그룹) or setting nhnDBSGToAllowAllInbound=true"
				} else {
					result.RemoteDataIOTestStatus = "Fail"
				}
				log.Warn().Err(dummyErr).Msgf("[%s] Remote Data I/O Test failed (non-blocking)", tc.RdbmsId)
			} else {
				entry.ResponseStatus = "OK"
				entry.ResponsePayload = map[string]any{"result": "write/read/verify/delete succeeded"}
				result.RemoteDataIOTestStatus = "Pass"
				log.Info().Msgf("[%s] Remote Data I/O Test OK (write/read/verify/delete)", tc.RdbmsId)
			}
			logs = append(logs, entry)
		}
	} else {
		result.RemoteDataIOTestStatus = "N/A (database not created)"
	}

	// 9b. Internal Data I/O Test: run SQL commands from inside the VPC using a test VM via Remote Command
	if databaseCreated && tc.InternalDataIOTest && tc.VmImageId != "" && tc.VmSpecId != "" {
		internalStatus, internalErr := runInternalDataTest(
			tbApiBase, nsId, tc, vNetId, subnetIds[0], sgId, rdbmsEndpoint, dbName, tbAuth, &logs,
		)
		result.InternalDataIOTestStatus = internalStatus
		if internalErr != nil {
			log.Warn().Err(internalErr).Msgf("[%s] Internal Data I/O Test failed: %s", tc.RdbmsId, internalStatus)
		} else {
			log.Info().Msgf("[%s] Internal Data I/O Test OK: %s", tc.RdbmsId, internalStatus)
		}
	} else if databaseCreated && tc.InternalDataIOTest {
		result.InternalDataIOTestStatus = "Skipped (vmImageId/vmSpecId not set)"
		log.Warn().Msgf("[%s] Internal Data I/O Test skipped: vmImageId/vmSpecId not set", tc.RdbmsId)
	} else {
		result.InternalDataIOTestStatus = "N/A (internal test disabled)"
	}

	// 10. Delete Database (best-effort cleanup; X-Admin-User-Password is required here)
	if databaseCreated {
		urlDeleteDB := fmt.Sprintf("%s/ns/%s/resources/rdbms/%s/database/%s", tbApiBase, nsId, tc.RdbmsId, dbName)
		headers := map[string]string{"X-Admin-User-Password": tc.AdminUserPassword}
		_, err = callApi("DELETE", urlDeleteDB, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete Database", tc.RdbmsId), headers)
		switch {
		case err == nil:
			result.DeleteDatabaseStatus = "Success"
			log.Info().Msgf("[%s] Delete Database OK", tc.RdbmsId)
		case isNotFoundErr(err):
			result.DeleteDatabaseStatus = "Success (nothing to delete)"
			log.Info().Msgf("[%s] Delete Database: nothing to delete", tc.RdbmsId)
		default:
			result.DeleteDatabaseStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Delete Database failed", tc.RdbmsId)
		}
	} else {
		result.DeleteDatabaseStatus = "Skipped (database not created)"
	}

	// 11. Delete RDBMS (best-effort cleanup; runs whenever creation was attempted,
	// not just when it reported success — a failed/timed-out create can still
	// have registered a Failed-status record, or a real CSP resource, that needs
	// cleanup. A 404 here means there was genuinely nothing to delete.
	// Note: DeletionProtection is false by default in test cases, so force is not needed.
	if vNetId != "" {
		urlDelete := fmt.Sprintf("%s/ns/%s/resources/rdbms/%s", tbApiBase, nsId, tc.RdbmsId)
		_, err = callApi("DELETE", urlDelete, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete RDBMS", tc.RdbmsId))
		switch {
		case err == nil:
			result.DeleteRDBMSStatus = "Success"
			log.Info().Msgf("[%s] Delete RDBMS OK", tc.RdbmsId)
		case isNotFoundErr(err):
			result.DeleteRDBMSStatus = "Success (nothing to delete)"
			log.Info().Msgf("[%s] Delete RDBMS: nothing to delete (create never registered a record)", tc.RdbmsId)
		default:
			result.DeleteRDBMSStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Delete RDBMS failed", tc.RdbmsId)
		}
	} else {
		result.DeleteRDBMSStatus = "Skipped (no VNet)"
	}

	// 12. Delete SecurityGroup (with retry for eventual consistency on CSPs like Tencent)
	if sgId != "" {
		urlDeleteSG := fmt.Sprintf("%s/ns/%s/resources/securityGroup/%s", tbApiBase, nsId, sgId)
		for attempt := 1; attempt <= 3; attempt++ {
			_, err = callApi("DELETE", urlDeleteSG, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete SecurityGroup", tc.RdbmsId))
			if err == nil {
				result.DeleteSGStatus = "Success"
				log.Info().Msgf("[%s] Delete SecurityGroup OK", tc.RdbmsId)
				break
			} else if isNotFoundErr(err) {
				result.DeleteSGStatus = "Success (nothing to delete)"
				log.Info().Msgf("[%s] Delete SecurityGroup: nothing to delete", tc.RdbmsId)
				break
			}
			if attempt < 3 {
				log.Info().Msgf("[%s] Delete SecurityGroup attempt %d/3 returned error; waiting 15s for CSP interface release...", tc.RdbmsId, attempt)
				time.Sleep(15 * time.Second)
			} else {
				result.DeleteSGStatus = "Failed"
				log.Error().Err(err).Msgf("[%s] Delete SecurityGroup failed", tc.RdbmsId)
			}
		}
	} else {
		result.DeleteSGStatus = "Skipped (not created)"
	}

	// 13. Delete each Subnet (with retry for eventual consistency across CSPs like Alibaba/Tencent)
	if vNetId != "" && len(subnetIds) > 0 {
		failed := 0
		for _, subnetId := range subnetIds {
			urlDeleteSubnet := fmt.Sprintf("%s/ns/%s/resources/vNet/%s/subnet/%s", tbApiBase, nsId, vNetId, subnetId)
			var subErr error
			for attempt := 1; attempt <= 6; attempt++ {
				_, subErr = callApi("DELETE", urlDeleteSubnet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete Subnet %s", tc.RdbmsId, subnetId))
				if subErr == nil || isNotFoundErr(subErr) {
					subErr = nil
					break
				}
				if attempt < 6 {
					log.Info().Msgf("[%s] Delete Subnet %s attempt %d/6 returned error; waiting 20s for CSP resource release...", tc.RdbmsId, subnetId, attempt)
					time.Sleep(20 * time.Second)
				}
			}
			if subErr != nil {
				failed++
				log.Error().Err(subErr).Msgf("[%s] Delete Subnet %s failed", tc.RdbmsId, subnetId)
			}
		}
		if failed == 0 {
			result.DeleteSubnetsStatus = fmt.Sprintf("Success (%d)", len(subnetIds))
		} else {
			result.DeleteSubnetsStatus = fmt.Sprintf("Failed (%d/%d)", failed, len(subnetIds))
		}
	} else {
		result.DeleteSubnetsStatus = "Skipped (no VNet)"
	}

	// 14. Delete vNet (with retry for eventual consistency on CSPs like Alibaba/NCP)
	if vNetId != "" {
		urlDeleteVNet := fmt.Sprintf("%s/ns/%s/resources/vNet/%s", tbApiBase, nsId, vNetId)
		for attempt := 1; attempt <= 6; attempt++ {
			_, err = callApi("DELETE", urlDeleteVNet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete VNet", tc.RdbmsId))
			if err == nil {
				result.DeleteVNetStatus = "Success"
				log.Info().Msgf("[%s] Delete VNet OK", tc.RdbmsId)
				break
			} else if isNotFoundErr(err) {
				result.DeleteVNetStatus = "Success (nothing to delete)"
				log.Info().Msgf("[%s] Delete VNet OK (already deleted)", tc.RdbmsId)
				err = nil
				break
			}
			if attempt < 6 {
				log.Info().Msgf("[%s] Delete VNet attempt %d/6 returned error; waiting 20s...", tc.RdbmsId, attempt)
				time.Sleep(20 * time.Second)
			}
		}
		if err != nil {
			result.DeleteVNetStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Delete VNet failed", tc.RdbmsId)
		}
	} else {
		result.DeleteVNetStatus = "Skipped (not created)"
	}

	saveDetailedReport(tc.RdbmsId, logs)
	log.Info().Msgf("[%s] ====== END ======", tc.RdbmsId)
	return result
}

// dbPortForEngine returns the default TCP port for the security group rule,
// keyed by dbEngine (mysql/mariadb: 3306, postgresql: 5432).
func dbPortForEngine(dbEngine string) string {
	switch strings.ToLower(dbEngine) {
	case "postgresql", "postgres":
		return "5432"
	default:
		return "3306"
	}
}

// isNotFoundErr reports whether a callApi error represents an HTTP 404 (resource
// already gone / never existed), so cleanup steps can treat it as success.
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

// tumblebugTestRecord is the scratch row used by testDatabaseDummyData.
type tumblebugTestRecord struct {
	ID    int `gorm:"primaryKey"`
	Value string
}

// TableName pins the table name so it matches across CSPs regardless of GORM's pluralization.
func (tumblebugTestRecord) TableName() string { return "tumblebug_test" }

// testDatabaseDummyData connects directly to the RDBMS instance via GORM (MySQL dialect, shared
// by mysql and mariadb) and runs a minimal write/read/verify/delete cycle against a scratch
// table — confirming the logical database created via Tumblebug's RDBMS database API is
// actually usable for SQL, not just present in the database list. Requires the instance to be
// reachable from wherever this CLI runs (publicAccess=true and a security group rule allowing
// the caller, as set up by the earlier steps in runLifecycle).
func testDatabaseDummyData(endpoint, adminUserName, adminUserPassword, dbName string) error {
	// tls=preferred: use TLS if the server offers it, fall back to plaintext if not — CSP
	// requirements vary (Azure rejects a plaintext connection outright; AWS/GCP don't require
	// TLS but support it) and this single setting adapts to either without per-CSP branching.
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&timeout=10s&tls=preferred",
		adminUserName, adminUserPassword, endpoint, dbName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db = db.WithContext(ctx)

	if err := db.AutoMigrate(&tumblebugTestRecord{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	const wantValue = "tumblebug-dummy-data"
	if err := db.Save(&tumblebugTestRecord{ID: 1, Value: wantValue}).Error; err != nil {
		return fmt.Errorf("save: %w", err)
	}

	var got tumblebugTestRecord
	if err := db.First(&got, 1).Error; err != nil {
		return fmt.Errorf("find: %w", err)
	}
	if got.Value != wantValue {
		return fmt.Errorf("find: got %q, want %q", got.Value, wantValue)
	}

	if err := db.Delete(&tumblebugTestRecord{}, 1).Error; err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// resolveAndReviewSpecAndImage uses recommendSpec, searchImage, and specImagePairReview
// to dynamically discover, recommend, and validate VM specs and images before resource creation.
func resolveAndReviewSpecAndImage(
	tbApiBase string,
	nsId string,
	tc *TestCase,
	tbAuth map[string]string,
	logs *[]ApiLog,
) {
	if !tc.InternalDataIOTest {
		return
	}

	providerName := ""
	regionName := ""
	urlConn := fmt.Sprintf("%s/connConfig/%s", tbApiBase, tc.ConnectionName)
	respBytes, err := callApi("GET", urlConn, tbAuth, nil, logs, fmt.Sprintf("[%s] Get Connection Config", tc.RdbmsId))
	if err == nil {
		var connInfo model.ConnConfig
		if jsonErr := json.Unmarshal(respBytes, &connInfo); jsonErr == nil {
			providerName = connInfo.ProviderName
			regionName = connInfo.RegionDetail.RegionName
		}
	}
	if providerName == "" {
		parts := strings.Split(tc.ConnectionName, "-")
		if len(parts) >= 2 {
			providerName = parts[0]
			regionName = strings.Join(parts[1:], "-")
		}
	}

	// 1. Recommend Spec via POST /recommendSpec if vmSpecId is empty
	if tc.VmSpecId == "" && providerName != "" {
		vCPU := tc.VmvCPU
		if vCPU == "" {
			vCPU = "2"
		}
		mem := tc.VmMemoryGiB
		if mem == "" {
			mem = "4"
		}
		recReq := map[string]any{
			"filter": map[string]any{
				"policy": []map[string]any{
					{"metric": "providerName", "condition": []map[string]any{{"operand": providerName}}},
					{"metric": "regionName", "condition": []map[string]any{{"operand": regionName}}},
					{"metric": "vCPU", "condition": []map[string]any{{"operator": ">=", "operand": vCPU}}},
					{"metric": "memoryGiB", "condition": []map[string]any{{"operator": ">=", "operand": mem}}},
				},
			},
			"priority": map[string]any{
				"policy": []map[string]any{{"metric": "cost", "weight": 1.0}},
			},
			"limit": 1,
		}
		urlRec := fmt.Sprintf("%s/recommendSpec", tbApiBase)
		respBytes, err := callApi("POST", urlRec, tbAuth, recReq, logs, fmt.Sprintf("[%s] Recommend VM Spec", tc.RdbmsId))
		if err == nil {
			var specList []model.SpecInfo
			if jsonErr := json.Unmarshal(respBytes, &specList); jsonErr == nil && len(specList) > 0 {
				tc.VmSpecId = specList[0].Id
				log.Info().Msgf("[%s] Recommended VM Spec: %s (cspSpecName=%s, vcpu=%d, mem=%.1fGB)",
					tc.RdbmsId, specList[0].Id, specList[0].CspSpecName, specList[0].VCPU, specList[0].MemoryGiB)
			}
		}
	}

	// 2. Search Image via POST /ns/{nsId}/resources/searchImage if vmImageId is empty
	if tc.VmImageId == "" && providerName != "" {
		osType := tc.VmOSType
		if osType == "" {
			osType = "ubuntu 24.04"
		}
		searchReq := map[string]any{
			"providerName":  providerName,
			"regionName":    regionName,
			"osType":        osType,
			"matchedSpecId": tc.VmSpecId,
		}
		urlSearch := fmt.Sprintf("%s/ns/%s/resources/searchImage", tbApiBase, model.SystemCommonNs)
		respBytes, err := callApi("POST", urlSearch, tbAuth, searchReq, logs, fmt.Sprintf("[%s] Search VM Image", tc.RdbmsId))
		if err == nil {
			var searchResp model.SearchImageResponse
			if jsonErr := json.Unmarshal(respBytes, &searchResp); jsonErr == nil && searchResp.ImageCount > 0 {
				for _, img := range searchResp.ImageList {
					if !img.IsKubernetesImage && !strings.Contains(strings.ToLower(img.InfraType), "k8s") && !strings.Contains(strings.ToLower(img.InfraType), "kubernetes") {
						tc.VmImageId = img.Id
						break
					}
				}
				if tc.VmImageId == "" {
					tc.VmImageId = searchResp.ImageList[0].Id
				}
				log.Info().Msgf("[%s] Discovered VM Image: %s", tc.RdbmsId, tc.VmImageId)
			}
		}
		if tc.VmImageId == "" {
			searchReqFallback := map[string]any{
				"providerName": providerName,
				"regionName":   regionName,
				"osType":       osType,
			}
			respBytes, err = callApi("POST", urlSearch, tbAuth, searchReqFallback, logs, fmt.Sprintf("[%s] Search VM Image (Fallback)", tc.RdbmsId))
			if err == nil {
				var searchResp model.SearchImageResponse
				if jsonErr := json.Unmarshal(respBytes, &searchResp); jsonErr == nil && searchResp.ImageCount > 0 {
					for _, img := range searchResp.ImageList {
						if !img.IsKubernetesImage && !strings.Contains(strings.ToLower(img.InfraType), "k8s") && !strings.Contains(strings.ToLower(img.InfraType), "kubernetes") {
							tc.VmImageId = img.Id
							break
						}
					}
					if tc.VmImageId == "" {
						tc.VmImageId = searchResp.ImageList[0].Id
					}
					log.Info().Msgf("[%s] Discovered Fallback VM Image: %s", tc.RdbmsId, tc.VmImageId)
				}
			}
		}
	}

	// 3. Review Spec & Image Pair Compatibility via POST /specImagePairReview
	if tc.VmSpecId != "" && tc.VmImageId != "" {
		zone := ""
		if len(tc.Subnets) > 0 {
			zone = tc.Subnets[0].Zone
		}
		specReviewId := tc.VmSpecId
		if !strings.Contains(specReviewId, "+") && providerName != "" && regionName != "" {
			specReviewId = fmt.Sprintf("%s+%s+%s", providerName, regionName, tc.VmSpecId)
		}
		imageReviewId := tc.VmImageId
		if !strings.Contains(imageReviewId, "+") && providerName != "" && regionName != "" {
			imageReviewId = fmt.Sprintf("%s+%s+%s", providerName, regionName, tc.VmImageId)
		}
		reviewReq := map[string]any{
			"specId":  specReviewId,
			"imageId": imageReviewId,
			"zone":    zone,
		}
		urlReview := fmt.Sprintf("%s/specImagePairReview", tbApiBase)
		respBytes, err := callApi("POST", urlReview, tbAuth, reviewReq, logs, fmt.Sprintf("[%s] Review Spec-Image Pair", tc.RdbmsId))
		if err == nil {
			var reviewResult model.SpecImagePairReviewResult
			if jsonErr := json.Unmarshal(respBytes, &reviewResult); jsonErr == nil {
				if reviewResult.ImageValidation.CspResourceId != "" {
					log.Info().Msgf("[%s] Spec-Image Pair Review succeeded (valid=%v, status=%s, resolved CSP image=%s)",
						tc.RdbmsId, reviewResult.IsValid, reviewResult.Status, reviewResult.ImageValidation.CspResourceId)
				} else {
					log.Info().Msgf("[%s] Spec-Image Pair Review result: valid=%v, status=%s, msg=%s",
						tc.RdbmsId, reviewResult.IsValid, reviewResult.Status, reviewResult.Message)
				}
				if !reviewResult.IsValid || reviewResult.Status == "Error" {
					log.Warn().Msgf("[%s] Spec-Image Pair Review flagged issues: %s (errors: %v)",
						tc.RdbmsId, reviewResult.Message, reviewResult.Errors)
				}
			}
		}
	}

}

func runInternalDataTest(
	tbApiBase string,
	nsId string,
	tc TestCase,
	vNetId string,
	subnetId string,
	sgId string,
	rdbmsEndpoint string,
	dbName string,
	tbAuth map[string]string,
	logs *[]ApiLog,
) (status string, err error) {
	infraId := fmt.Sprintf("test-rdbms-infra-%s", tc.RdbmsId)
	sshKeyId := fmt.Sprintf("test-rdbms-sshkey-%s", tc.RdbmsId)

	// Ensure cleanup of test VM and SSHKey upon completion
	defer func() {
		urlDeleteInfra := fmt.Sprintf("%s/ns/%s/infra/%s?option=terminate", tbApiBase, nsId, infraId)
		_, _ = callApi("DELETE", urlDeleteInfra, tbAuth, nil, logs, fmt.Sprintf("[%s] Teardown Test Infra", tc.RdbmsId))

		// Poll until test infra is fully terminated so CSP releases SecurityGroup/Subnet ENIs
		urlGetInfra := fmt.Sprintf("%s/ns/%s/infra/%s", tbApiBase, nsId, infraId)
		for attempt := 1; attempt <= 18; attempt++ {
			time.Sleep(5 * time.Second)
			_, getErr := callApi("GET", urlGetInfra, tbAuth, nil, logs, fmt.Sprintf("[%s] Verify Infra Termination", tc.RdbmsId))
			if getErr != nil && isNotFoundErr(getErr) {
				break
			}
		}

		// Brief stabilization pause after VM termination to allow CSP-side key unbinding
		time.Sleep(15 * time.Second)

		// Delete SSHKey with retry (normal DELETE confirms genuine CSP-side keypair deletion)
		urlDeleteSSH := fmt.Sprintf("%s/ns/%s/resources/sshKey/%s", tbApiBase, nsId, sshKeyId)
		for attempt := 1; attempt <= 4; attempt++ {
			_, delErr := callApi("DELETE", urlDeleteSSH, tbAuth, nil, logs, fmt.Sprintf("[%s] Teardown Test SSHKey", tc.RdbmsId))
			if delErr == nil || isNotFoundErr(delErr) {
				break
			}
			if attempt < 4 {
				time.Sleep(10 * time.Second)
			}
		}
	}()

	// 1. Create or retrieve SSH Key
	sshReq := map[string]any{
		"name":           sshKeyId,
		"connectionName": tc.ConnectionName,
		"description":    "created by RDBMS test-cli for internal test",
	}
	urlSSH := fmt.Sprintf("%s/ns/%s/resources/sshKey", tbApiBase, nsId)
	if _, err := callApi("POST", urlSSH, tbAuth, sshReq, logs, fmt.Sprintf("[%s] Create Test SSHKey", tc.RdbmsId)); err != nil {
		urlGetSSH := fmt.Sprintf("%s/ns/%s/resources/sshKey/%s", tbApiBase, nsId, sshKeyId)
		if _, getErr := callApi("GET", urlGetSSH, tbAuth, nil, logs, fmt.Sprintf("[%s] Get Existing SSHKey", tc.RdbmsId)); getErr != nil {
			return "Failed (SSHKey create failed)", err
		}
	}

	// 2. Create Infra (VM) in same VNet/Subnet using looked-up spec and image directly
	infraReq := model.InfraReq{
		Name:            infraId,
		Description:     "test runner VM for internal RDBMS SQL test",
		InstallMonAgent: "no",
		NodeGroups: []model.CreateNodeGroupReq{
			{
				Name:             "ng1",
				NodeGroupSize:    1,
				ConnectionName:   tc.ConnectionName,
				VNetId:           vNetId,
				SubnetId:         subnetId,
				SecurityGroupIds: []string{sgId},
				SshKeyId:         sshKeyId,
				SpecId:           tc.VmSpecId,
				ImageId:          tc.VmImageId,
				RootDiskSize:     100,
				RootDiskType:     "default",
			},
		},
	}
	urlInfra := fmt.Sprintf("%s/ns/%s/infra", tbApiBase, nsId)
	if _, err := callApi("POST", urlInfra, tbAuth, infraReq, logs, fmt.Sprintf("[%s] Create Test Infra", tc.RdbmsId)); err != nil {
		return "Failed (Infra create failed)", err
	}

	host, port, splitErr := net.SplitHostPort(rdbmsEndpoint)
	if splitErr != nil {
		host = rdbmsEndpoint
		port = "3306"
	}

	// Give sshd and cloud-init sufficient grace period to start accepting connections after VM reaches Running
	time.Sleep(50 * time.Second)

	// 3. Send Remote Command via POST /ns/{nsId}/cmd/infra/{infraId}
	sqlCmd := fmt.Sprintf("mysql -h %s -P %s -u %s -p'%s' %s -e \"DROP TABLE IF EXISTS tumblebug_internal_test; CREATE TABLE tumblebug_internal_test (id INT PRIMARY KEY, val VARCHAR(255)); INSERT INTO tumblebug_internal_test (id, val) VALUES (1, 'internal-test-ok'); SELECT val FROM tumblebug_internal_test WHERE id=1; DROP TABLE tumblebug_internal_test;\"",
		host, port, tc.AdminUserName, tc.AdminUserPassword, dbName)

	cmdUserName := "cb-user"

	cmdReq := model.InfraCmdReq{
		Command: []string{
			"command -v mysql || command -v mariadb || (command -v apt-get >/dev/null 2>&1 && sudo apt-get update -qq && sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq default-mysql-client) || (command -v dnf >/dev/null 2>&1 && sudo dnf install -y mariadb) || (command -v yum >/dev/null 2>&1 && sudo yum install -y mariadb) || sudo yum install -y mysql",
			sqlCmd,
		},
		UserName:       cmdUserName,
		TimeoutMinutes: 5,
	}

	urlCmd := fmt.Sprintf("%s/ns/%s/cmd/infra/%s", tbApiBase, nsId, infraId)
	respBytes, err := callApi("POST", urlCmd, tbAuth, cmdReq, logs, fmt.Sprintf("[%s] Execute Internal SQL via Remote Cmd", tc.RdbmsId))
	if err != nil {
		return "Failed (Remote command failed)", err
	}

	var cmdResp model.InfraSshCmdResultForAPI
	_ = json.Unmarshal(respBytes, &cmdResp)
	cmdSucceeded := false
	for _, res := range cmdResp.Results {
		for _, out := range res.Stdout {
			if strings.Contains(out, "internal-test-ok") {
				cmdSucceeded = true
				break
			}
		}
		if cmdSucceeded {
			break
		}
	}

	if cmdSucceeded {
		return "Pass", nil
	}
	return "Failed (SQL verification failed)", fmt.Errorf("SQL verification response did not contain expected output")
}

// ============================================================
// Reporting
// ============================================================

// saveDetailedReport writes a per-CSP markdown report to test-results/<rdbmsId>.md.
// Sensitive fields in request/response payloads are masked before writing.
func saveDetailedReport(rdbmsId string, logs []ApiLog) {
	if err := os.MkdirAll("test-results", 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create test-results directory")
		return
	}
	filename := fmt.Sprintf("test-results/%s.md", rdbmsId)

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# RDBMS Test: %s\n\n", rdbmsId))
	md.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	for i, entry := range logs {
		md.WriteString(fmt.Sprintf("## Step %d: %s\n\n", i+1, entry.Step))
		md.WriteString(fmt.Sprintf("- **Method**: `%s`\n", entry.Method))
		md.WriteString(fmt.Sprintf("- **URL**: `%s`\n", entry.URL))
		md.WriteString(fmt.Sprintf("- **Status**: %s\n", entry.ResponseStatus))
		md.WriteString(fmt.Sprintf("- **Elapsed**: %s\n\n", entry.ElapsedTime))

		if entry.RequestPayload != nil {
			masked := maskSensitiveFields(entry.RequestPayload)
			reqJson, _ := json.MarshalIndent(masked, "", "  ")
			md.WriteString("### Request Body\n```json\n")
			md.WriteString(string(reqJson))
			md.WriteString("\n```\n\n")
		}
		if entry.ResponsePayload != nil {
			masked := maskSensitiveFields(entry.ResponsePayload)
			respJson, _ := json.MarshalIndent(masked, "", "  ")
			md.WriteString("### Response Body\n```json\n")
			md.WriteString(string(respJson))
			md.WriteString("\n```\n\n")
		}
		md.WriteString("---\n\n")
	}

	if err := os.WriteFile(filename, []byte(md.String()), 0644); err != nil {
		log.Warn().Err(err).Msgf("Failed to write report: %s", filename)
		return
	}
	log.Info().Msgf("[%s] Detailed report saved: %s", rdbmsId, filename)
}

// stepLabels lists the row labels used by both buildSummaryMarkdown and
// buildEngineMatrixMarkdown, in the same order as stepValues.
var stepLabels = []string{
	"Create VNet", "Create SecurityGroup", "Support", "Capability", "Validate",
	"Create RDBMS", "Get RDBMS", "List RDBMS", "Create Database", "List Database",
	"Remote Data I/O Test", "Internal Data I/O Test", "Delete Database", "Delete RDBMS", "Delete SecurityGroup",
	"Delete Subnets", "Delete VNet",
}

// stepValues returns one TestResult's step statuses, in the same order as stepLabels.
func stepValues(r TestResult) []string {
	return []string{
		r.CreateVNetStatus, r.CreateSGStatus, r.SupportStatus, r.CapabilityStatus, r.ValidateStatus,
		r.CreateRDBMSStatus, r.GetRDBMSStatus, r.ListRDBMSStatus, r.CreateDatabaseStatus,
		r.ListDatabaseStatus, r.RemoteDataIOTestStatus, r.InternalDataIOTestStatus, r.DeleteDatabaseStatus, r.DeleteRDBMSStatus,
		r.DeleteSGStatus, r.DeleteSubnetsStatus, r.DeleteVNetStatus,
	}
}

// overallStatus reports "❌" if any core step status starts with "Failed" or "Fail",
// with an exception for Remote Data I/O Test on CSPs where a known console setting is required
// or where public endpoint is N/A, so that Tumblebug's own API lifecycle success is not masked.
func overallStatus(r TestResult) string {
	steps := stepValues(r)
	for i, s := range steps {
		label := stepLabels[i]
		if label == "Remote Data I/O Test" && strings.Contains(s, "Note:") {
			continue
		}
		if label == "Remote Data I/O Test" && strings.HasPrefix(s, "N/A") {
			continue
		}
		if label == "Internal Data I/O Test" && strings.HasPrefix(s, "N/A") {
			continue
		}
		if strings.HasPrefix(s, "Failed") || strings.HasPrefix(s, "Fail") {
			return "❌"
		}
	}
	return "✅"
}

// statusEmoji classifies one step's status string for the matrix view:
// ✅ success/pass, ❌ failure, "-" skipped/unknown, N/A not applicable.
func statusEmoji(s string) string {
	switch {
	case strings.HasPrefix(s, "Success"), strings.HasPrefix(s, "Supported"), strings.HasPrefix(s, "Pass"):
		return "✅"
	case strings.Contains(s, "Note:"):
		return "❌ (Note)"
	case strings.HasPrefix(s, "N/A"), strings.HasPrefix(s, "Not Supported"), strings.HasPrefix(s, "Skipped"):
		return "N/A"
	case strings.HasPrefix(s, "Failed"), strings.HasPrefix(s, "Fail"):
		return "❌"
	default:
		return "-"
	}
}

// cspLabel derives a short column header from a test case's rdbmsId (e.g.
// "test-rdbms-aws" -> "aws"), falling back to the full id if the prefix isn't present.
func cspLabel(rdbmsId string) string {
	return strings.TrimPrefix(rdbmsId, "test-rdbms-")
}

// buildSummaryMarkdown renders the batch run as a single markdown document — the same
// content is written to test-results/summary.md and printed directly to the console (see
// runBatchTest), so it can be pasted as-is into docs/PRs.
func buildSummaryMarkdown(results []TestResult) string {
	var md strings.Builder
	md.WriteString("# RDBMS Batch Test Summary\n\n")
	md.WriteString("## Test Workflow\n\n")
	md.WriteString("0. **Get RDBMS Support Matrix** (once per batch) — `GET /tumblebug/rdbms/support`\n")
	md.WriteString("1. **Create VNet (+ subnets)** — `POST /ns/{nsId}/resources/vNet`\n")
	md.WriteString("2. **Create SecurityGroup** — `POST /ns/{nsId}/resources/securityGroup`\n")
	md.WriteString("3. **Get RDBMS Capability** — `GET /tumblebug/rdbms/capability?providerName=&regionName=&dbEngine=`\n")
	md.WriteString("4. **Validate RDBMS** — `POST /ns/{nsId}/resources/rdbms/validate` (dry run, no side effects; create is skipped if this fails)\n")
	md.WriteString("5. **Create RDBMS** — `POST /ns/{nsId}/resources/rdbms` (blocks until Available/Failed)\n")
	md.WriteString("6. **Get RDBMS** — `GET /ns/{nsId}/resources/rdbms/{rdbmsId}`\n")
	md.WriteString("7. **List RDBMS** — `GET /ns/{nsId}/resources/rdbms`\n")
	md.WriteString("8. **Create Database** — `POST /ns/{nsId}/resources/rdbms/{rdbmsId}/database`\n")
	md.WriteString("9. **List Database** — `GET /ns/{nsId}/resources/rdbms/{rdbmsId}/database`\n")
	md.WriteString("10. **Remote Data I/O Test** — direct SQL write/read/verify/delete from remote client over public endpoint\n")
	md.WriteString("11. **Internal Data I/O Test** — SQL write/read/verify/delete from internal VPC network/VM\n")
	md.WriteString("12. **Delete Database** — `DELETE /ns/{nsId}/resources/rdbms/{rdbmsId}/database/{dbName}`\n")
	md.WriteString("13. **Delete RDBMS** — `DELETE /ns/{nsId}/resources/rdbms/{rdbmsId}`\n")
	md.WriteString("14. **Delete SecurityGroup** — `DELETE /ns/{nsId}/resources/securityGroup/{sgId}`\n")
	md.WriteString("15. **Delete Subnets** — `DELETE /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId}`\n")
	md.WriteString("16. **Delete VNet** — `DELETE /ns/{nsId}/resources/vNet/{vNetId}`\n\n")
	md.WriteString(fmt.Sprintf("Generated: %s\n\n---\n\n", time.Now().Format(time.RFC3339)))

	md.WriteString("## Results\n\n")
	for _, r := range results {
		steps := stepValues(r)
		overall := overallStatus(r)

		md.WriteString(fmt.Sprintf("### %s (%s) %s\n\n", r.RdbmsId, r.ConnectionName, overall))
		md.WriteString("| Step | Result |\n")
		md.WriteString("| ---- | ------ |\n")
		for i, label := range stepLabels {
			md.WriteString(fmt.Sprintf("| %s | %s |\n", label, steps[i]))
		}
		md.WriteString(fmt.Sprintf("| **Overall** | %s |\n", overall))
		if r.Note != "" {
			md.WriteString(fmt.Sprintf("| **Note** | %s |\n", r.Note))
		}
		md.WriteString("\n")
	}
	md.WriteString("---\n\n")
	md.WriteString("### Detailed Logs\n\nSee `test-results/<rdbmsId>.md` for per-CSP API trace logs.\n")

	return md.String()
}

// buildEngineMatrixMarkdown renders a CSP-by-step pass/fail matrix for one dbEngine:
// columns are CSPs, rows are test steps, cells are ✅/❌/"-" (skipped or unknown).
// Complements buildSummaryMarkdown's per-CSP vertical view with an at-a-glance
// comparison across CSPs for a single engine (e.g. test-results/summary-mysql.md).
func buildEngineMatrixMarkdown(results []TestResult, engine string) string {
	var md strings.Builder
	md.WriteString(fmt.Sprintf("# RDBMS Batch Test Summary — %s\n\n", engine))
	md.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	allSteps := make([][]string, len(results))
	for i, r := range results {
		allSteps[i] = stepValues(r)
	}

	md.WriteString("| Step |")
	for _, r := range results {
		md.WriteString(fmt.Sprintf(" %s |", cspLabel(r.RdbmsId)))
	}
	md.WriteString("\n| ---- |")
	for range results {
		md.WriteString(" :--: |")
	}
	md.WriteString("\n")

	for stepIdx, label := range stepLabels {
		md.WriteString(fmt.Sprintf("| %s |", label))
		for _, steps := range allSteps {
			md.WriteString(fmt.Sprintf(" %s |", statusEmoji(steps[stepIdx])))
		}
		md.WriteString("\n")
	}

	md.WriteString("| **Overall** |")
	for _, r := range results {
		md.WriteString(fmt.Sprintf(" %s |", overallStatus(r)))
	}
	md.WriteString("\n")

	hasNotes := false
	for _, r := range results {
		if r.Note != "" {
			hasNotes = true
			break
		}
	}
	if hasNotes {
		md.WriteString("\n### Notes\n\n")
		for _, r := range results {
			if r.Note != "" {
				md.WriteString(fmt.Sprintf("- **%s**: %s\n", cspLabel(r.RdbmsId), r.Note))
			}
		}
	}

	return md.String()
}

// ============================================================
// Utilities
// ============================================================

// maskSensitiveFields recursively replaces values of known sensitive keys with "****".
// This is applied before writing request/response payloads to report files.
func maskSensitiveFields(v any) any {
	sensitive := map[string]bool{
		"password": true, "passwd": true, "secret": true,
		"credential": true, "credentials": true,
		"token": true, "accesstoken": true, "access_token": true,
		"apikey": true, "api_key": true, "secretkey": true, "secret_key": true,
		"privatekey": true, "private_key": true,
		"adminuserpassword": true,
	}
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v2 := range val {
			if sensitive[strings.ToLower(k)] {
				out[k] = "****"
			} else {
				out[k] = maskSensitiveFields(v2)
			}
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v2 := range val {
			out[i] = maskSensitiveFields(v2)
		}
		return out
	default:
		return v
	}
}

// getAuth returns the basic-auth credentials from env/config.
// The password is never logged; it is used only via SetBasicAuth.
func getAuth() map[string]string {
	return map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}
}

// callApi executes an HTTP request, records the call in logs (if non-nil),
// and returns the response body. Returns an error on network failure or non-2xx status.
// The timeout is sized for RDBMS creation, which can block server-side for up to
// ~20 minutes (see src/core/resource/rdbms.go's rdbmsCreationTimeout).
func callApi(
	method string,
	apiUrl string,
	auth map[string]string,
	reqBody any,
	logs *[]ApiLog,
	step string,
	extraHeaders ...map[string]string,
) ([]byte, error) {

	client := resty.New()
	client.SetTimeout(30 * time.Minute)
	// The Tumblebug endpoint under test is typically plain http://localhost during local
	// development, so resty's "Basic Auth over HTTP is insecure" warning is expected noise
	// here, not an actual finding — the CLI never talks to a real endpoint over HTTP.
	client.SetDisableWarn(true)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBasicAuth(auth["username"], auth["password"])

	for _, headers := range extraHeaders {
		for k, v := range headers {
			req.SetHeader(k, v)
		}
	}

	var body []byte
	var marshalErr error
	if reqBody != nil {
		body, marshalErr = json.Marshal(reqBody)
		if marshalErr != nil {
			return nil, fmt.Errorf("[%s] failed to marshal request body: %v", step, marshalErr)
		}
		req.SetBody(body)
	}

	log.Debug().Msgf("[%s] %s %s", step, method, apiUrl)

	start := time.Now()
	var resp *resty.Response
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		resp, err = req.Get(apiUrl)
	case "POST":
		resp, err = req.Post(apiUrl)
	case "PUT":
		resp, err = req.Put(apiUrl)
	case "DELETE":
		resp, err = req.Delete(apiUrl)
	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		// A transport-level failure (connection reset, DNS, timeout) never reaches
		// tumblebug.log — record it here or it vanishes entirely from the saved report.
		if logs != nil {
			var reqPayload any
			if body != nil {
				json.Unmarshal(body, &reqPayload)
			}
			*logs = append(*logs, ApiLog{
				Step:            step,
				Method:          method,
				URL:             apiUrl,
				RequestPayload:  maskSensitiveFields(reqPayload),
				ResponsePayload: map[string]any{"error": err.Error()},
				ResponseStatus:  "transport error",
				ElapsedTime:     elapsed.String(),
			})
		}
		return nil, fmt.Errorf("[%s] request failed: %v", step, err)
	}

	log.Debug().Msgf("[%s] %s %s → HTTP %d (%s)", step, method, apiUrl, resp.StatusCode(), elapsed)

	// Record log entry (with sensitive fields masked)
	if logs != nil {
		var reqPayload any
		if body != nil {
			json.Unmarshal(body, &reqPayload)
		}
		var respPayload any
		json.Unmarshal(resp.Body(), &respPayload)

		*logs = append(*logs, ApiLog{
			Step:            step,
			Method:          method,
			URL:             apiUrl,
			RequestPayload:  maskSensitiveFields(reqPayload),
			ResponsePayload: maskSensitiveFields(respPayload),
			ResponseStatus:  resp.Status(),
			ElapsedTime:     elapsed.String(),
		})
	}

	if resp.IsError() {
		return resp.Body(), fmt.Errorf("[%s] HTTP %s: %s", step, resp.Status(), strings.TrimSpace(string(resp.Body())))
	}

	return resp.Body(), nil
}
