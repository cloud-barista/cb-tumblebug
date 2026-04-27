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

// Package main provides a CLI tool for batch-testing Object Storage (bucket)
// lifecycle (create → get → delete) across multiple CSPs via CB-Tumblebug API.
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
	RequestPayload  interface{}
	ResponsePayload interface{}
	ResponseStatus  string
	ElapsedTime     string
}

// TestCase represents a single CSP test case from the config file.
type TestCase struct {
	OsId           string `mapstructure:"osId"`
	ConnectionName string `mapstructure:"connectionName"`
	Execute        bool   `mapstructure:"execute"`
}

// TestResult holds the outcome of each lifecycle step for one CSP.
type TestResult struct {
	OsId                  string
	ConnectionName        string
	CreateStatus          string
	CheckExistenceStatus  string
	GetStatus             string
	DeleteStatus          string
	SupportStatus         string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "./app",
		Short: "Object Storage batch test CLI",
		Long: `
##########################################################################
## Object Storage batch test CLI for CB-Tumblebug                       ##
## Runs create → get → delete lifecycle for each enabled CSP test case  ##
##########################################################################`,
	}

	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Run create/get/delete lifecycle test for all enabled CSPs",
		Run:   runBatchTest,
	}
	testCmd.Flags().StringP("nsId", "n", "", "Namespace ID (overrides config)")
	testCmd.Flags().Bool("parallel", false, "Run test cases in parallel")

	rootCmd.AddCommand(testCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Error executing object storage batch test CLI")
	}
}

// runBatchTest executes the full lifecycle test for every enabled test case.
// With --parallel the cases run concurrently; without it they run sequentially.
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
			log.Info().Msgf("Skipping (execute=false): osId=%s", tc.OsId)
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
	log.Info().Msgf("Running %d test case(s) in %s mode", len(cases), mode)

	results := make([]TestResult, len(cases))

	if parallel {
		var wg sync.WaitGroup
		for i, tc := range cases {
			wg.Add(1)
			go func(idx int, tc TestCase) {
				defer wg.Done()
				results[idx] = runLifecycle(nsId, tc, tbAuth)
			}(i, tc)
		}
		wg.Wait()
	} else {
		for i, tc := range cases {
			results[i] = runLifecycle(nsId, tc, tbAuth)
		}
	}

	generateSummaryReport("test-results/summary.md", results)

	log.Info().Msg("\n========== BATCH TEST SUMMARY ==========")
	for _, r := range results {
		log.Info().Msgf("  %-45s | Create: %-25s | CheckExistence: %-25s | Get: %-25s | Delete: %-25s | Support: %s",
			r.OsId+"("+r.ConnectionName+")", r.CreateStatus, r.CheckExistenceStatus, r.GetStatus, r.DeleteStatus, r.SupportStatus)
	}
	log.Info().Msg("=========================================")
	log.Info().Msg("Detailed report saved to test-results/summary.md")
}

// runLifecycle runs create → get → delete for one test case and returns the result.
// Between each step a stability sleep is applied.
// The delete step is verified and retried once on failure.
func runLifecycle(nsId string, tc TestCase, tbAuth map[string]string) TestResult {
	result := TestResult{OsId: tc.OsId, ConnectionName: tc.ConnectionName}
	logs := []ApiLog{}

	log.Info().Msgf("[%s] ====== START (connection=%s) ======", tc.OsId, tc.ConnectionName)

	// ── 1. Create ────────────────────────────────────────────────────────────
	reqBody := map[string]interface{}{
		"bucketName":     tc.OsId,
		"connectionName": tc.ConnectionName,
		"description":    "created by object-storage batch test CLI",
	}
	urlCreate := fmt.Sprintf("%s/ns/%s/resources/objectStorage", tbApiBase, nsId)
	respBytes, err := callApi("PUT", urlCreate, tbAuth, reqBody, &logs, fmt.Sprintf("[%s] Create", tc.OsId))
	if err != nil {
		result.CreateStatus = "Failed"
		log.Error().Err(err).Msgf("[%s] Create failed", tc.OsId)
	} else {
		var info model.ObjectStorageInfo
		_ = json.Unmarshal(respBytes, &info)
		result.CreateStatus = fmt.Sprintf("Success (status=%s)", info.Status)
		log.Info().Msgf("[%s] Create OK: status=%s", tc.OsId, info.Status)
	}

	// Stability sleep before Check Existence
	log.Info().Msgf("[%s] Waiting 3s before Check Existence...", tc.OsId)
	time.Sleep(3 * time.Second)

	// ── 2. Check Existence ───────────────────────────────────────────────────
	urlHead := fmt.Sprintf("%s/ns/%s/resources/objectStorage/%s", tbApiBase, nsId, tc.OsId)
	_, err = callApi("HEAD", urlHead, tbAuth, nil, &logs, fmt.Sprintf("[%s] Check Existence", tc.OsId))
	if err != nil {
		result.CheckExistenceStatus = "Failed (not found)"
		log.Warn().Err(err).Msgf("[%s] Check Existence: not found", tc.OsId)
	} else {
		result.CheckExistenceStatus = "Success (exists)"
		log.Info().Msgf("[%s] Check Existence OK: bucket exists", tc.OsId)
	}

	// Stability sleep before Get
	log.Info().Msgf("[%s] Waiting 3s before Get...", tc.OsId)
	time.Sleep(3 * time.Second)

	// ── 3. Get ───────────────────────────────────────────────────────────────
	urlGet := fmt.Sprintf("%s/ns/%s/resources/objectStorage/%s", tbApiBase, nsId, tc.OsId)
	respBytes, err = callApi("GET", urlGet, tbAuth, nil, &logs, fmt.Sprintf("[%s] Get", tc.OsId))
	if err != nil {
		result.GetStatus = "Failed"
		log.Error().Err(err).Msgf("[%s] Get failed", tc.OsId)
	} else {
		var info model.ObjectStorageInfo
		_ = json.Unmarshal(respBytes, &info)
		result.GetStatus = fmt.Sprintf("Success (status=%s)", info.Status)
		log.Info().Msgf("[%s] Get OK: status=%s", tc.OsId, info.Status)
	}

	// Stability sleep before Delete
	log.Info().Msgf("[%s] Waiting 3s before Delete...", tc.OsId)
	time.Sleep(3 * time.Second)

	// ── 4. Delete ────────────────────────────────────────────────────────────
	// The server performs DELETE→GET verification internally and returns an error
	// only when the resource still exists after all retries.
	urlDelete := fmt.Sprintf("%s/ns/%s/resources/objectStorage/%s", tbApiBase, nsId, tc.OsId)
	_, err = callApi("DELETE", urlDelete, tbAuth, nil, &logs, fmt.Sprintf("[%s] Delete", tc.OsId))
	if err != nil {
		result.DeleteStatus = "Failed"
		log.Error().Err(err).Msgf("[%s] Delete failed", tc.OsId)
	} else {
		result.DeleteStatus = "Success"
		log.Info().Msgf("[%s] Delete OK", tc.OsId)
	}


	// ── 5. Get Support Info ────────────────────────────────────────────────────────
	// Derive cspType from the first segment of connectionName (e.g. "aws-ap-northeast-2" → "aws")
	cspType := strings.SplitN(tc.ConnectionName, "-", 2)[0]
	urlSupport := fmt.Sprintf("%s/objectStorage/support?cspType=%s", tbApiBase, cspType)
	_, err = callApi("GET", urlSupport, tbAuth, nil, &logs, fmt.Sprintf("[%s] Get Support Info", tc.OsId))
	if err != nil {
		result.SupportStatus = "Failed"
		log.Warn().Err(err).Msgf("[%s] Get Support Info failed", tc.OsId)
	} else {
		result.SupportStatus = fmt.Sprintf("Success (cspType=%s)", cspType)
		log.Info().Msgf("[%s] Get Support Info OK: cspType=%s", tc.OsId, cspType)
	}

	saveDetailedReport(tc.OsId, logs)
	log.Info().Msgf("[%s] ====== END ======", tc.OsId)
	return result
}

// ============================================================
// Reporting
// ============================================================

// saveDetailedReport writes a per-CSP markdown report to test-results/<osId>.md.
// Sensitive fields in request/response payloads are masked before writing.
func saveDetailedReport(osId string, logs []ApiLog) {
	if err := os.MkdirAll("test-results", 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create test-results directory")
		return
	}
	filename := fmt.Sprintf("test-results/%s.md", osId)

	md := fmt.Sprintf("# Object Storage Test: %s\n\n", osId)
	md += fmt.Sprintf("Generated: %s\n\n", time.Now().Format(time.RFC3339))

	for i, entry := range logs {
		md += fmt.Sprintf("## Step %d: %s\n\n", i+1, entry.Step)
		md += fmt.Sprintf("- **Method**: `%s`\n", entry.Method)
		md += fmt.Sprintf("- **URL**: `%s`\n", entry.URL)
		md += fmt.Sprintf("- **Status**: %s\n", entry.ResponseStatus)
		md += fmt.Sprintf("- **Elapsed**: %s\n\n", entry.ElapsedTime)

		if entry.RequestPayload != nil {
			masked := maskSensitiveFields(entry.RequestPayload)
			reqJson, _ := json.MarshalIndent(masked, "", "  ")
			md += "### Request Body\n```json\n" + string(reqJson) + "\n```\n\n"
		}
		if entry.ResponsePayload != nil {
			masked := maskSensitiveFields(entry.ResponsePayload)
			respJson, _ := json.MarshalIndent(masked, "", "  ")
			md += "### Response Body\n```json\n" + string(respJson) + "\n```\n\n"
		}
		md += "---\n\n"
	}

	if err := os.WriteFile(filename, []byte(md), 0644); err != nil {
		log.Warn().Err(err).Msgf("Failed to write report: %s", filename)
		return
	}
	log.Info().Msgf("[%s] Detailed report saved: %s", osId, filename)
}

// generateSummaryReport writes a summary markdown table to the given file.
func generateSummaryReport(filename string, results []TestResult) {
	if err := os.MkdirAll("test-results", 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create test-results directory")
		return
	}

	md := "# Object Storage Batch Test Summary\n\n"
	md += "## Test Workflow\n\n"
	md += "1. **Create** — `PUT /ns/{nsId}/resources/objectStorage`\n"
	md += "2. **Check Existence** — `HEAD /ns/{nsId}/resources/objectStorage/{osId}`\n"
	md += "3. **Get** — `GET /ns/{nsId}/resources/objectStorage/{osId}`\n"
	md += "4. **Delete** — `DELETE /ns/{nsId}/resources/objectStorage/{osId}` (with retry and verification)\n"
	md += "5. **Get Support Info** — `GET /objectStorage/support?cspType={cspType}`\n\n"
	md += fmt.Sprintf("Generated: %s\n\n---\n\n", time.Now().Format(time.RFC3339))

	md += "## Results\n\n"
	md += "| osId | Connection | Create | Check Existence | Get | Delete | Support Info | Overall |\n"
	md += "| ---- | ---------- | ------ | --------------- | --- | ------ | ------------ | ------- |\n"
	for _, r := range results {
		overall := "✅"
		if !strings.HasPrefix(r.CreateStatus, "Success") ||
			!strings.HasPrefix(r.CheckExistenceStatus, "Success") ||
			!strings.HasPrefix(r.GetStatus, "Success") ||
			!strings.HasPrefix(r.DeleteStatus, "Success") ||
			!strings.HasPrefix(r.SupportStatus, "Success") {
			overall = "❌"
		}
		md += fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.OsId, r.ConnectionName, r.CreateStatus, r.CheckExistenceStatus, r.GetStatus, r.DeleteStatus, r.SupportStatus, overall)
	}
	md += "\n---\n\n"
	md += "### Detailed Logs\n\nSee `test-results/<osId>.md` for per-CSP API trace logs.\n"

	if err := os.WriteFile(filename, []byte(md), 0644); err != nil {
		log.Warn().Err(err).Msgf("Failed to write summary report: %s", filename)
		return
	}
	log.Info().Msgf("Summary report saved: %s", filename)
}

// ============================================================
// Utilities
// ============================================================

// maskSensitiveFields recursively replaces values of known sensitive keys with "****".
// This is applied before writing request/response payloads to report files.
func maskSensitiveFields(v interface{}) interface{} {
	sensitive := map[string]bool{
		"password": true, "passwd": true, "secret": true,
		"credential": true, "credentials": true,
		"token": true, "accesstoken": true, "access_token": true,
		"apikey": true, "api_key": true, "secretkey": true, "secret_key": true,
		"privatekey": true, "private_key": true,
	}
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v2 := range val {
			if sensitive[strings.ToLower(k)] {
				out[k] = "****"
			} else {
				out[k] = maskSensitiveFields(v2)
			}
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
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
func callApi(
	method string,
	apiUrl string,
	auth map[string]string,
	reqBody interface{},
	logs *[]ApiLog,
	step string,
) ([]byte, error) {

	client := resty.New()
	client.SetTimeout(10 * time.Minute)

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
	case "HEAD":
		resp, err = req.Head(apiUrl)
	case "PUT":
		resp, err = req.Put(apiUrl)
	case "POST":
		resp, err = req.Post(apiUrl)
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
		var reqPayload interface{}
		if body != nil {
			json.Unmarshal(body, &reqPayload)
		}
		var respPayload interface{}
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
