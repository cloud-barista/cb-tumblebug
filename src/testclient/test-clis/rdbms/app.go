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
// rdbms/capability → RDBMS create → get → list → delete → securityGroup delete →
// subnets delete → vNet delete) across multiple CSPs via the CB-Tumblebug API.
package main

import (
	"encoding/json"
	"fmt"
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
)

var tbApiBase string

func init() {
	setConfig()
	tbApiBase = viper.GetString("tumblebug.endpoint") + "/tumblebug"
}

// setConfig loads settings from test-config.yaml and .env
func setConfig() {
	viper.SetConfigName("test-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Error reading test-config.yaml")
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
	RdbmsId            string         `mapstructure:"rdbmsId"`
	ConnectionName     string         `mapstructure:"connectionName"`
	VNetName           string         `mapstructure:"vNetName"`
	CidrBlock          string         `mapstructure:"cidrBlock"`
	Subnets            []SubnetConfig `mapstructure:"subnets"`
	SecurityGroupName  string         `mapstructure:"securityGroupName"`
	DBEngine           string         `mapstructure:"dbEngine"`
	DBEngineVersion    string         `mapstructure:"dbEngineVersion"`
	DBInstanceSpec     string         `mapstructure:"dbInstanceSpec"`
	StorageType        string         `mapstructure:"storageType"`
	StorageSize        int            `mapstructure:"storageSize"`
	AutoFillDefaults   bool           `mapstructure:"autoFillDefaults"`
	MasterUserName     string         `mapstructure:"masterUserName"`
	MasterUserPassword string         `mapstructure:"masterUserPassword"`
	PublicAccess       bool           `mapstructure:"publicAccess"`
	HighAvailability   bool           `mapstructure:"highAvailability"`
	Execute            bool           `mapstructure:"execute"`
}

// TestResult holds the outcome of each lifecycle step for one CSP.
type TestResult struct {
	RdbmsId             string
	ConnectionName      string
	CreateVNetStatus    string
	CreateSGStatus      string
	SupportStatus       string
	CapabilityStatus    string
	ValidateStatus      string
	CreateRDBMSStatus   string
	GetRDBMSStatus      string
	ListRDBMSStatus     string
	DeleteRDBMSStatus   string
	DeleteSGStatus      string
	DeleteSubnetsStatus string
	DeleteVNetStatus    string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "./app",
		Short: "RDBMS batch test CLI",
		Long: `
##########################################################################
## RDBMS batch test CLI for CB-Tumblebug                                ##
## Runs rdbms/support once, then per case: vNet+subnets -> securityGroup##
## -> rdbms/capability -> RDBMS create -> get -> list -> delete ->      ##
## securityGroup/subnets/vNet delete                                    ##
##########################################################################`,
	}

	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Run the full RDBMS lifecycle test for all enabled CSPs",
		Run:   runBatchTest,
	}
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

	// Printed directly (not via the zerolog logger, which wraps every line in JSON) so
	// this can be copied straight into docs/PRs as-is — it's the same markdown saved to
	// test-results/summary.md.
	fmt.Println()
	fmt.Println(summaryMarkdown)
}

// runLifecycle runs the full chain for one test case and returns the result:
//  1. Create vNet (with embedded subnets)
//  2. Create SecurityGroup (inbound rule for the DB engine's port)
//  3. Get RDBMS support info for the resolved provider/region
//  4. Create RDBMS (blocks server-side until Available/Failed)
//  5. Get RDBMS (single)
//  6. List RDBMS (verify the instance appears)
//  7. Delete RDBMS
//  8. Delete SecurityGroup
//  9. Delete each Subnet
//  10. Delete vNet
//
// Steps 7-10 always run (best-effort, in reverse-dependency order) even if an
// earlier step failed, so a failed run doesn't leave billed CSP resources behind.
func runLifecycle(nsId string, tc TestCase, tbAuth map[string]string, supportMatrix model.RDBMSSupportResponse) TestResult {
	result := TestResult{RdbmsId: tc.RdbmsId, ConnectionName: tc.ConnectionName}
	logs := []ApiLog{}

	log.Info().Msgf("[%s] ====== START (connection=%s) ======", tc.RdbmsId, tc.ConnectionName)

	var vNetId string
	var subnetIds []string
	var sgId string
	var rdbmsCreated bool

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
		sgReqBody := map[string]any{
			"name":           tc.SecurityGroupName,
			"connectionName": tc.ConnectionName,
			"vNetId":         vNetId,
			"description":    "created by RDBMS batch test CLI",
			"firewallRules": []map[string]any{
				{"Ports": port, "Protocol": "TCP", "Direction": "inbound", "CIDR": "0.0.0.0/0"},
			},
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
		_, err = callApi("GET", urlCapability, tbAuth, nil, &logs, fmt.Sprintf("[%s] Get RDBMS Capability", tc.RdbmsId))
		if err != nil {
			result.CapabilityStatus = "Failed"
			log.Warn().Err(err).Msgf("[%s] Get RDBMS Capability failed", tc.RdbmsId)
		} else {
			result.CapabilityStatus = "Success"
			log.Info().Msgf("[%s] Get RDBMS Capability OK", tc.RdbmsId)
		}
	} else {
		result.SupportStatus = "Skipped (no VNet)"
		result.CapabilityStatus = "Skipped (no VNet)"
	}

	// 4. Validate, then create RDBMS (only if VNet succeeded; SecurityGroup is best-effort)
	if vNetId != "" {
		rdbmsReqBody := map[string]any{
			"name":               tc.RdbmsId,
			"connectionName":     tc.ConnectionName,
			"vNetId":             vNetId,
			"subnetIds":          subnetIds,
			"dbEngine":           tc.DBEngine,
			"dbEngineVersion":    tc.DBEngineVersion,
			"dbInstanceSpec":     tc.DBInstanceSpec,
			"storageType":        tc.StorageType,
			"storageSize":        tc.StorageSize,
			"masterUserName":     tc.MasterUserName,
			"masterUserPassword": tc.MasterUserPassword,
			"highAvailability":   tc.HighAvailability,
			"publicAccess":       tc.PublicAccess,
			"autoFillDefaults":   tc.AutoFillDefaults,
			"description":        "created by RDBMS batch test CLI",
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

	// 7. Delete RDBMS (best-effort cleanup; runs whenever creation was attempted,
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

	// 8. Delete SecurityGroup
	if sgId != "" {
		urlDeleteSG := fmt.Sprintf("%s/ns/%s/resources/securityGroup/%s", tbApiBase, nsId, sgId)
		_, err = callApi("DELETE", urlDeleteSG, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete SecurityGroup", tc.RdbmsId))
		switch {
		case err == nil:
			result.DeleteSGStatus = "Success"
			log.Info().Msgf("[%s] Delete SecurityGroup OK", tc.RdbmsId)
		case isNotFoundErr(err):
			result.DeleteSGStatus = "Success (nothing to delete)"
			log.Info().Msgf("[%s] Delete SecurityGroup: nothing to delete", tc.RdbmsId)
		default:
			result.DeleteSGStatus = "Failed"
			log.Error().Err(err).Msgf("[%s] Delete SecurityGroup failed", tc.RdbmsId)
		}
	} else {
		result.DeleteSGStatus = "Skipped (not created)"
	}

	// 9. Delete each Subnet
	if vNetId != "" && len(subnetIds) > 0 {
		failed := 0
		for _, subnetId := range subnetIds {
			urlDeleteSubnet := fmt.Sprintf("%s/ns/%s/resources/vNet/%s/subnet/%s", tbApiBase, nsId, vNetId, subnetId)
			_, err = callApi("DELETE", urlDeleteSubnet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete Subnet %s", tc.RdbmsId, subnetId))
			if err != nil && !isNotFoundErr(err) {
				failed++
				log.Error().Err(err).Msgf("[%s] Delete Subnet %s failed", tc.RdbmsId, subnetId)
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

	// 10. Delete vNet
	if vNetId != "" {
		urlDeleteVNet := fmt.Sprintf("%s/ns/%s/resources/vNet/%s", tbApiBase, nsId, vNetId)
		_, err = callApi("DELETE", urlDeleteVNet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete VNet", tc.RdbmsId))
		switch {
		case err == nil:
			result.DeleteVNetStatus = "Success"
			log.Info().Msgf("[%s] Delete VNet OK", tc.RdbmsId)
		case isNotFoundErr(err):
			result.DeleteVNetStatus = "Success (nothing to delete)"
			log.Info().Msgf("[%s] Delete VNet: nothing to delete", tc.RdbmsId)
		default:
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
	md.WriteString("8. **Delete RDBMS** — `DELETE /ns/{nsId}/resources/rdbms/{rdbmsId}`\n")
	md.WriteString("9. **Delete SecurityGroup** — `DELETE /ns/{nsId}/resources/securityGroup/{sgId}`\n")
	md.WriteString("10. **Delete Subnets** — `DELETE /ns/{nsId}/resources/vNet/{vNetId}/subnet/{subnetId}`\n")
	md.WriteString("11. **Delete VNet** — `DELETE /ns/{nsId}/resources/vNet/{vNetId}`\n\n")
	md.WriteString(fmt.Sprintf("Generated: %s\n\n---\n\n", time.Now().Format(time.RFC3339)))

	md.WriteString("## Results\n\n")
	for _, r := range results {
		steps := []string{
			r.CreateVNetStatus, r.CreateSGStatus, r.SupportStatus, r.CapabilityStatus, r.ValidateStatus,
			r.CreateRDBMSStatus, r.GetRDBMSStatus, r.ListRDBMSStatus, r.DeleteRDBMSStatus, r.DeleteSGStatus,
			r.DeleteSubnetsStatus, r.DeleteVNetStatus,
		}
		overall := "✅"
		for _, s := range steps {
			if strings.HasPrefix(s, "Failed") {
				overall = "❌"
				break
			}
		}

		md.WriteString(fmt.Sprintf("### %s (%s) %s\n\n", r.RdbmsId, r.ConnectionName, overall))
		md.WriteString("| Step | Result |\n")
		md.WriteString("| ---- | ------ |\n")
		md.WriteString(fmt.Sprintf("| VNet | %s |\n", r.CreateVNetStatus))
		md.WriteString(fmt.Sprintf("| SecurityGroup | %s |\n", r.CreateSGStatus))
		md.WriteString(fmt.Sprintf("| Support | %s |\n", r.SupportStatus))
		md.WriteString(fmt.Sprintf("| Capability | %s |\n", r.CapabilityStatus))
		md.WriteString(fmt.Sprintf("| Validate | %s |\n", r.ValidateStatus))
		md.WriteString(fmt.Sprintf("| Create RDBMS | %s |\n", r.CreateRDBMSStatus))
		md.WriteString(fmt.Sprintf("| Get RDBMS | %s |\n", r.GetRDBMSStatus))
		md.WriteString(fmt.Sprintf("| List RDBMS | %s |\n", r.ListRDBMSStatus))
		md.WriteString(fmt.Sprintf("| Delete RDBMS | %s |\n", r.DeleteRDBMSStatus))
		md.WriteString(fmt.Sprintf("| Delete SecurityGroup | %s |\n", r.DeleteSGStatus))
		md.WriteString(fmt.Sprintf("| Delete Subnets | %s |\n", r.DeleteSubnetsStatus))
		md.WriteString(fmt.Sprintf("| Delete VNet | %s |\n", r.DeleteVNetStatus))
		md.WriteString(fmt.Sprintf("| **Overall** | %s |\n\n", overall))
	}
	md.WriteString("---\n\n")
	md.WriteString("### Detailed Logs\n\nSee `test-results/<rdbmsId>.md` for per-CSP API trace logs.\n")

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
		"masteruserpassword": true,
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
) ([]byte, error) {

	client := resty.New()
	client.SetTimeout(30 * time.Minute)

	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBasicAuth(auth["username"], auth["password"])

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
