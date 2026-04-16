package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	_ "github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"

	"github.com/go-resty/resty/v2"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tbApiBase string

func init() {
	setConfig()
	tbApiBase = viper.GetString("tumblebug.endpoint") + "/tumblebug" // ex) "http://localhost:1323/tumblebug"
}

// setConfig get cloud settings from a config file
func setConfig() {
	// 1. Load test-config.yaml
	viper.SetConfigName("test-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Err(err).Msg("Error reading test-config.yaml")
	}
	log.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())

	// 2. Load .env for authentication
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	if err := viper.MergeInConfig(); err != nil {
		log.Warn().Msg("No .env file found, relying on environment variables or defaults")
	}

	// 3. Enable Environment Variables
	viper.AutomaticEnv()
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "./app",
		Short: "[Demo] VPN tunnel on Infra",
		Long: `
########################################################################
## [Demo] This program demonstrates VPN tunnel configuration on Infra. ##
########################################################################`,
	}

	var createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create resources",
	}

	var createInfraDynamicCmd = &cobra.Command{
		Use:   "infra",
		Short: "Create Infra dynamically",
		Run:   createInfra,
	}
	// Command-line flags with shorthand
	createInfraDynamicCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	createInfraDynamicCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	createInfraDynamicCmd.Flags().StringP("file", "f", "", "Specify the JSON file for the request body")

	var createVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Create GCP to AWS VPN tunnel",
		Run:   createVpnTunnel,
	}
	createVpnCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	createVpnCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	createVpnCmd.Flags().StringP("vpnId", "v", "", "VPN ID")
	createVpnCmd.Flags().StringP("targetCsp", "t", "gcp", "Target CSP (e.g., azure, gcp, alibaba, tencent, ibm, dcs)")

	createCmd.AddCommand(
		createInfraDynamicCmd,
		createVpnCmd,
	)

	var getCmd = &cobra.Command{
		Use:   "get",
		Short: "Get resources",
	}

	var getVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Get AWS to Site VPN tunnel info",
		Run:   getVpnTunnel,
	}
	getVpnCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	getVpnCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	getVpnCmd.Flags().StringP("vpnId", "v", "", "VPN ID")

	getCmd.AddCommand(
		getVpnCmd,
	)

	var deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
	}

	var terminateInfraCmd = &cobra.Command{
		Use:   "infra",
		Short: "Suspend and terminate Infra",
		Run:   terminateInfra,
	}
	terminateInfraCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	terminateInfraCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	terminateInfraCmd.Flags().StringP("option", "o", "terminate", "Option for delete Infra (terminate, force)")

	var destroyVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Destroy GCP to AWS VPN tunnel",
		Run:   destroyVpnTunnel,
	}
	// Command-line flags with shorthand
	destroyVpnCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	destroyVpnCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	destroyVpnCmd.Flags().StringP("vpnId", "v", "", "VPN ID")

	var cleanupSharedCmd = &cobra.Command{
		Use:   "shared",
		Short: "Cleanup shared resources in the namespace",
		Run:   cleanupShared,
	}
	cleanupSharedCmd.Flags().StringP("nsId", "n", "", "Namespace ID")

	deleteCmd.AddCommand(
		terminateInfraCmd,
		destroyVpnCmd,
		cleanupSharedCmd,
	)

	var testCmd = &cobra.Command{
		Use:   "test",
		Short: "Test resources",
	}

	var testVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Run batch VPN connectivity tests",
		Run:   batchTestVpn,
	}
	testVpnCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	testVpnCmd.Flags().StringP("infraId", "m", "", "Infra ID")
	testVpnCmd.Flags().StringP("file", "f", "test-target-pairs.json", "Test target pairs JSON file")

	testCmd.AddCommand(
		testVpnCmd,
	)

	// Add commands
	rootCmd.AddCommand(
		createCmd,
		getCmd,
		deleteCmd,
		testCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Error executing demo to configure vpn tunnel")
	}
}

type TestTargetPairs struct {
	TestCases []TestCase `json:"testCases"`
}

type TestCase struct {
	Site1   string `json:"site1"`
	Site2   string `json:"site2"`
	VpnId   string `json:"vpnId"`
	Execute bool   `json:"execute"`
}

type TestResult struct {
	TestCase          TestCase
	CreateRes         string
	HealthCheckStatus string
	DeleteRes         string
	ApiLogs           []ApiLog
}

type ApiLog struct {
	Step            string
	Method          string
	URL             string
	RequestPayload  interface{}
	ResponsePayload interface{}
	ResponseStatus  string
	ElapsedTime     string
}

func providerFromSpecID(specID string) string {
	parts := strings.Split(strings.ToLower(specID), "+")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func getEnabledCSPs(testPairs TestTargetPairs) map[string]bool {
	enabled := map[string]bool{}
	for _, tc := range testPairs.TestCases {
		if !tc.Execute {
			continue
		}
		s1 := strings.ToLower(strings.TrimSpace(tc.Site1))
		s2 := strings.ToLower(strings.TrimSpace(tc.Site2))
		if s1 != "" {
			enabled[s1] = true
		}
		if s2 != "" {
			enabled[s2] = true
		}
	}
	return enabled
}

func createInfra(cmd *cobra.Command, args []string) {
	var err error
	var respBytes []byte

	// Set namespace ID, Infra ID, and request body file
	nsId, _ := cmd.Flags().GetString("namespaceId")
	infraId, _ := cmd.Flags().GetString("infraId")
	filePath, _ := cmd.Flags().GetString("file")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("File path", filePath).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	if filePath == "" {
		filePath = viper.GetString("tumblebug.api.infraDynamic.reqBody")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("File path", filePath).
		Msg("[config.yaml]")

	if nsId == "" || infraId == "" || filePath == "" {
		err = fmt.Errorf("bad request: nsId, infraId, or file path is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("Infra ID", infraId).
			Str("File path", filePath).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting creating an Infra dynamically...")

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: check readiness

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil, nil, "Readiness Check")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(model.SimpleMsg)
	if err := json.Unmarshal(respBytes, resTbReadiness); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResTbReadiness, err := json.MarshalIndent(resTbReadiness, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}
	log.Debug().Msgf("[Response] %+v", string(prettyResTbReadiness))

	log.Info().Msg(resTbReadiness.Message)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: infraDynamic

	infraInfo, err := createInfraInternal(nsId, infraId, filePath, tbAuth, nil, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Infra")
		return
	}

	prettyInfraInfo, _ := json.MarshalIndent(infraInfo, "", "   ")
	log.Debug().Msgf("[Response] %+v", string(prettyInfraInfo))
}

func createInfraInternal(nsId, infraId, filePath string, tbAuth map[string]string, logs *[]ApiLog, enabledCSPs map[string]bool) (*model.InfraInfo, error) {
	// Set the API path
	urlPostInfraDynamic := fmt.Sprintf("%s/ns/%s/infraDynamic", tbApiBase, nsId)

	// Read the request body written in infraDynamic.json
	infraDynamicFile, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %v", filePath, err)
	}
	defer infraDynamicFile.Close()

	infraDynamicData, err := io.ReadAll(infraDynamicFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", filePath, err)
	}

	reqInfraDynamic := new(model.InfraDynamicReq)
	err = json.Unmarshal(infraDynamicData, &reqInfraDynamic)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %v", filePath, err)
	}

	if len(enabledCSPs) > 0 {
		filtered := make([]model.CreateNodeGroupDynamicReq, 0, len(reqInfraDynamic.NodeGroups))
		for _, sg := range reqInfraDynamic.NodeGroups {
			provider := providerFromSpecID(sg.SpecId)
			if enabledCSPs[provider] {
				filtered = append(filtered, sg)
			}
		}

		if len(filtered) == 0 {
			return nil, fmt.Errorf("no nodeGroups matched enabled CSPs: %v", enabledCSPs)
		}

		log.Info().Msgf("Filtered Infra nodeGroups by enabled CSPs: %d -> %d", len(reqInfraDynamic.NodeGroups), len(filtered))
		reqInfraDynamic.NodeGroups = filtered
	}

	// Set Infra ID
	reqInfraDynamic.Name = infraId

	respBytes, err := callApi("POST", urlPostInfraDynamic, tbAuth, reqInfraDynamic, logs, "Provision Infra")
	if err != nil {
		return nil, fmt.Errorf("failed to create Infra: %s", string(respBytes))
	}

	infraInfo := new(model.InfraInfo)
	if err := json.Unmarshal(respBytes, infraInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Infra info: %v", err)
	}

	// Check if all VMs reached Running state and have IPs
	allRunning := true
	for _, node := range infraInfo.Node {
		if strings.ToLower(node.Status) != "running" {
			log.Error().Msgf("VM %s is in state %s. System Message: %s", node.Id, node.Status, node.SystemMessage)
			allRunning = false
		} else if node.PrivateIP == "" {
			log.Warn().Msgf("VM %s is Running but missing PrivateIP", node.Id)
			allRunning = false
		}
	}

	if !allRunning {
		return infraInfo, fmt.Errorf("some VMs failed to initialize correctly")
	}

	log.Info().Msg("Infra project created successfully and all VMs are Running.")
	return infraInfo, nil
}

func createVpnTunnel(cmd *cobra.Command, args []string) {
	var err error
	var respBytes []byte

	// Set namespace ID, Infra ID, VPN ID, and Target CSP
	nsId, _ := cmd.Flags().GetString("namespaceId")
	infraId, _ := cmd.Flags().GetString("infraId")
	vpnId, _ := cmd.Flags().GetString("vpnId")
	targetCsp, _ := cmd.Flags().GetString("targetCsp")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("VPN ID", vpnId).
		Str("Target CSP", targetCsp).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	if vpnId == "" {
		vpnId = viper.GetString("tumblebug.demo.vpnId")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("VPN ID", vpnId).
		Msg("[config.yaml]")

	if nsId == "" || infraId == "" || vpnId == "" {
		err = fmt.Errorf("bad request: nsId, infraId, or vpnId is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("Infra ID", infraId).
			Str("VPN ID", vpnId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting a demo of vpn tunnel configuration...")

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: readiness check

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil, nil, "Readiness Check")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(model.SimpleMsg)
	if err := json.Unmarshal(respBytes, resTbReadiness); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResTbReadiness, err := json.MarshalIndent(resTbReadiness, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}
	log.Debug().Msgf("[Response] %+v", string(prettyResTbReadiness))

	log.Info().Msg(resTbReadiness.Message)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Get Infra

	// Set the API path
	queryParams := "" //"option=status"
	urlGetInfraStatus := fmt.Sprintf("%s/ns/%s/infra/%s", tbApiBase, nsId, infraId)
	if queryParams != "" {
		urlGetInfraStatus += "?" + queryParams
	}

	// Request to create an infra dynamically
	respBytes, err = callApi("GET", urlGetInfraStatus, tbAuth, nil, nil, "Get Infra Status")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	infraInfo := new(model.InfraInfo)
	if err := json.Unmarshal(respBytes, infraInfo); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyInfraInfo, err := json.MarshalIndent(infraInfo, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}

	log.Debug().Msgf("[Response] %+v", string(prettyInfraInfo))

	// Print the infraInfo
	for _, node := range infraInfo.Node {
		log.Debug().
			Str("ProviderName", node.ConnectionConfig.ProviderName).
			Str("ConfigName", node.ConnectionConfig.ConfigName).
			Msg("ConnectionConfig managed by Cloud-Barista system")

		log.Debug().
			Str("VM ID", node.Id).
			Str("VNet ID", node.VNetId).
			Str("Subnet ID", node.SubnetId).
			Msg("IDs managed by Cloud-Barista system")

		log.Debug().
			Str("VPC/vNet ID", node.CspVNetId).
			Str("Subnet ID", node.CspSubnetId).
			Str("Region", node.Region.Region).
			Str("Region", node.Region.Zone).
			Msg("IDs managed by CSPs")

	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare information needed to configure VPN tunnel
	awsVNetId := ""
	targetVNetId := ""

	// Extract info from infraInfo
	targetCspLower := strings.ToLower(targetCsp)
	for _, node := range infraInfo.Node {
		providerName := strings.ToLower(node.ConnectionConfig.ProviderName)
		if providerName == "aws" {
			awsVNetId = node.VNetId
		} else if providerName == targetCspLower {
			targetVNetId = node.VNetId
		}
	}

	log.Debug().Msgf("AWS VNet ID: %s", awsVNetId)
	log.Debug().Msgf("Target (%s) VNet ID: %s", targetCsp, targetVNetId)

	if targetVNetId == "" {
		log.Error().Msgf("Could not find a VM with provider '%s' in Infra", targetCsp)
		return
	}

	log.Info().Msg("Information needed to configure VPN tunnel is ready")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Configure VPN tunnel via Tumblebug

	urlPostVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn", tbApiBase, nsId, infraId)

	// Set properties for site2 based on targetCsp
	var site2Props map[string]interface{}
	switch targetCspLower {
	case "gcp":
		site2Props = map[string]interface{}{
			"cspSpecificProperty": map[string]interface{}{
				"gcp": map[string]string{
					"bgpAsn": "65530",
				},
			},
			"vNetId": targetVNetId,
		}
	case "azure":
		site2Props = map[string]interface{}{
			"cspSpecificProperty": map[string]interface{}{
				"azure": map[string]string{
					"bgpAsn":            "65531",
					"gatewaySubnetCidr": "",
					"vpnSku":            "VpnGw1AZ",
				},
			},
			"vNetId": targetVNetId,
		}
	case "alibaba":
		site2Props = map[string]interface{}{
			"cspSpecificProperty": map[string]interface{}{
				"alibaba": map[string]string{
					"bgpAsn": "65532",
				},
			},
			"vNetId": targetVNetId,
		}
	case "tencent":
		site2Props = map[string]interface{}{
			"vNetId": targetVNetId,
		}
	case "ibm", "dcs":
		site2Props = map[string]interface{}{
			"vNetId": targetVNetId,
		}
	default:
		site2Props = map[string]interface{}{
			"vNetId": targetVNetId,
		}
	}

	reqVpn := map[string]interface{}{
		"name": vpnId,
		"site1": map[string]interface{}{
			"cspSpecificProperty": map[string]interface{}{
				"aws": map[string]string{
					"bgpAsn": "64512",
				},
			},
			"vNetId": awsVNetId,
		},
		"site2": site2Props,
	}

	respBytes, err = callApi("POST", urlPostVpn, tbAuth, reqVpn, nil, "Create VPN")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	vpnResp := new(model.VpnInfo)
	if err := json.Unmarshal(respBytes, vpnResp); err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	prettyVpnResp, _ := json.MarshalIndent(vpnResp, "", "   ")
	log.Debug().Msgf("[POST VPN Response] \n%s", string(prettyVpnResp))

	// GET VPN
	urlGetVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, vpnId)
	respBytes, err = callApi("GET", urlGetVpn, tbAuth, nil, nil, "Get VPN Info")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}
	vpnGetResp := new(model.VpnInfo)
	if err := json.Unmarshal(respBytes, vpnGetResp); err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	prettyVpnGetResp, _ := json.MarshalIndent(vpnGetResp, "", "   ")
	log.Debug().Msgf("[GET VPN Response] \n%s", string(prettyVpnGetResp))

	// GET all VPNs - IdList
	urlGetVpnIds := fmt.Sprintf("%s/ns/%s/infra/%s/vpn?option=IdList", tbApiBase, nsId, infraId)
	respBytes, err = callApi("GET", urlGetVpnIds, tbAuth, nil, nil, "List VPN IDs")
	if err == nil {
		var vpnIds map[string]interface{}
		json.Unmarshal(respBytes, &vpnIds)
		prettyVpnIds, _ := json.MarshalIndent(vpnIds, "", "   ")
		log.Debug().Msgf("[GET all VPNs IdList Response] \n%s", string(prettyVpnIds))
	}

	// GET all VPNs - InfoList
	urlGetVpnInfos := fmt.Sprintf("%s/ns/%s/infra/%s/vpn?option=InfoList", tbApiBase, nsId, infraId)
	respBytes, err = callApi("GET", urlGetVpnInfos, tbAuth, nil, nil, "List VPN Infos")
	if err == nil {
		var vpnInfos map[string]interface{}
		json.Unmarshal(respBytes, &vpnInfos)
		prettyVpnInfos, _ := json.MarshalIndent(vpnInfos, "", "   ")
		log.Debug().Msgf("[GET all VPNs InfoList Response] \n%s", string(prettyVpnInfos))
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// VPN Health Check (bidirectional ping test)

	urlHealthCheck := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s/health", tbApiBase, nsId, infraId, vpnId)
	reqHealth := map[string]interface{}{
		"userName":    "cb-user",
		"pingCount":   4,
		"intervalSec": 15,
		"maxAttempts": 20,
	}

	log.Info().Msgf("Running VPN health check for %s (bidirectional ping test)", vpnId)
	respBytes, err = callApi("POST", urlHealthCheck, tbAuth, reqHealth, nil, "VPN Health Check")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
	} else {
		var healthResp model.VpnHealthCheckResponse
		if jsonErr := json.Unmarshal(respBytes, &healthResp); jsonErr != nil {
			log.Error().Err(jsonErr).Msg("Failed to parse health check response")
		} else {
			prettyResp, _ := json.MarshalIndent(healthResp, "", "   ")
			log.Debug().Msgf("[VPN Health Check Response] \n%s", string(prettyResp))

			if healthResp.Reachable {
				log.Info().Msgf("VPN health check succeeded: %s", healthResp.Message)
			} else {
				log.Warn().Msgf("VPN health check failed: %s", healthResp.Message)
				for _, r := range healthResp.Results {
					log.Warn().Msgf("  [%s] reachable=%v, attempts=%d, packetLoss=%s",
						r.Direction, r.Reachable, r.Attempts, r.PingStats.PacketLoss)
				}
			}
		}
	}

}

func getVpnTunnel(cmd *cobra.Command, args []string) {
	var err error
	var respBytes []byte

	// Set namespace ID, Infra ID, and VPN ID
	nsId, _ := cmd.Flags().GetString("namespaceId")
	infraId, _ := cmd.Flags().GetString("infraId")
	vpnId, _ := cmd.Flags().GetString("vpnId")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("VPN ID", vpnId).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	if vpnId == "" {
		vpnId = viper.GetString("tumblebug.demo.vpnId")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("VPN ID", vpnId).
		Msg("[config.yaml]")

	if nsId == "" || infraId == "" || vpnId == "" {
		err = fmt.Errorf("bad request: nsId, infraId, or vpnId is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("Infra ID", infraId).
			Str("VPN ID", vpnId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting getting VPN tunnel info...")

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Get VPN

	urlGetVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, vpnId)
	respBytes, err = callApi("GET", urlGetVpn, tbAuth, nil, nil, "Get VPN Info")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}
	vpnGetResp := new(model.VpnInfo)
	if err := json.Unmarshal(respBytes, vpnGetResp); err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	prettyVpnGetResp, _ := json.MarshalIndent(vpnGetResp, "", "   ")
	log.Debug().Msgf("[GET VPN Response] \n%s", string(prettyVpnGetResp))

}

func destroyVpnTunnel(cmd *cobra.Command, args []string) {
	var err error
	var respBytes []byte

	// Set VPN ID
	vpnId, _ := cmd.Flags().GetString("vpnId")
	nsId, _ := cmd.Flags().GetString("namespaceId")
	infraId, _ := cmd.Flags().GetString("infraId")

	log.Debug().
		Str("VPN ID", vpnId).
		Msg("[args]")

	if vpnId == "" {
		vpnId = viper.GetString("tumblebug.demo.vpnId")
	}
	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}
	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	log.Debug().
		Str("VPN ID", vpnId).
		Msg("[config.yaml]")

	if vpnId == "" {
		err = fmt.Errorf("bad request: nsId, infraId, or vpnId is not set")
		log.Fatal().Err(err).
			Str("VPN ID", vpnId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting deleting a VPN tunnel...")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	// Tumblebug: Delete VPN tunnel

	urlDeleteVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, vpnId)

	respBytes, err = callApi("DELETE", urlDeleteVpn, tbAuth, nil, nil, "Delete VPN")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	var res map[string]interface{}
	json.Unmarshal(respBytes, &res)

	prettyResDestroy, _ := json.MarshalIndent(res, "", "   ")
	log.Debug().Msgf("[DELETE VPN Response] \n%s", string(prettyResDestroy))

}

func terminateInfraInternal(nsId, infraId, option string, tbAuth map[string]string, logs *[]ApiLog) error {
	// Set the API path
	urlDeleteInfra := fmt.Sprintf("%s/ns/%s/infra/%s?option=%s", tbApiBase, nsId, infraId, option)

	respBytes, err := callApi("DELETE", urlDeleteInfra, tbAuth, nil, logs, "Delete Infra")
	if err != nil {
		log.Warn().Msgf("Failed to delete Infra: %s. Retrying in 3 seconds...", string(respBytes))
		time.Sleep(3 * time.Second)
		respBytes, err = callApi("DELETE", urlDeleteInfra, tbAuth, nil, logs, "Delete Infra (Retry)")
		if err != nil {
			return fmt.Errorf("failed to delete Infra: %s", string(respBytes))
		}
	}

	log.Info().Msg("Verifying Infra deletion...")
	time.Sleep(15 * time.Second)
	urlGetInfraStatus := fmt.Sprintf("%s/ns/%s/infra/%s", tbApiBase, nsId, infraId)
	respBytes, err = callApi("GET", urlGetInfraStatus, tbAuth, nil, nil, "Check Infra Deletion Status")
	if !isDeleted(err, respBytes) {
		log.Warn().Msgf("Infra(id: %s) deletion verification failed or Infra still exists", infraId)
		if err != nil {
			return fmt.Errorf("failed to check Infra status: %v", err)
		}
		return fmt.Errorf("timeout waiting for Infra %s deletion", infraId)
	}

	log.Info().Msgf("Infra(id: %s) is successfully deleted.", infraId)
	return nil
}

func deleteAllVpnsInternal(nsId, infraId string, tbAuth map[string]string, logs *[]ApiLog) error {
	log.Info().Msg("Cleaning up all VPN resources in the Infra...")

	// 1. Get List of all VPN IDs
	urlListVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn?option=IdList", tbApiBase, nsId, infraId)
	respBytes, err := callApi("GET", urlListVpn, tbAuth, nil, logs, "List all VPN IDs for cleanup")
	if err != nil {
		return fmt.Errorf("failed to list VPNs for cleanup: %v", err)
	}

	var res struct {
		VpnIdList []string `json:"vpnIdList"`
	}
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return fmt.Errorf("failed to unmarshal VPN list: %v", err)
	}

	// 2. Delete each VPN
	for _, vpnId := range res.VpnIdList {
		log.Info().Msgf("Deleting orphan VPN: %s", vpnId)
		urlDeleteVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, vpnId)
		_, err := callApi("DELETE", urlDeleteVpn, tbAuth, nil, logs, fmt.Sprintf("Delete orphan VPN %s", vpnId))
		if err != nil {
			log.Warn().Msgf("Failed to delete VPN %s: %v. Retrying in 3 seconds...", vpnId, err)
			time.Sleep(3 * time.Second)
			_, err = callApi("DELETE", urlDeleteVpn, tbAuth, nil, logs, fmt.Sprintf("Delete orphan VPN %s (Retry)", vpnId))
			if err != nil {
				log.Warn().Msgf("Retry failed to delete VPN %s: %v", vpnId, err)
			}
		}
	}

	// 3. Verify VPN deletion
	log.Info().Msg("Verifying all VPNs are deleted...")
	time.Sleep(15 * time.Second)
	respBytes, err = callApi("GET", urlListVpn, tbAuth, nil, nil, "Verify VPN deletion")
	if err != nil {
		log.Warn().Msgf("Failed to check VPN list: %v", err)
	} else {
		var checkRes struct {
			VpnIdList []string `json:"vpnIdList"`
		}
		json.Unmarshal(respBytes, &checkRes)
		if len(checkRes.VpnIdList) == 0 {
			log.Info().Msg("All VPNs successfully deleted.")
			return nil
		}
		log.Warn().Msgf("Warning: %d VPNs still exist after deletion attempt", len(checkRes.VpnIdList))
	}
	return nil
}

func cleanupSharedResourcesInternal(nsId string, tbAuth map[string]string, logs *[]ApiLog) error {
	log.Info().Msg("Starting cleaning up shared resources in the namespace...")
	urlDeleteSharedResources := fmt.Sprintf("%s/ns/%s/sharedResources", tbApiBase, nsId)

	respBytes, err := callApi("DELETE", urlDeleteSharedResources, tbAuth, nil, logs, "Cleanup Shared Resources")
	if err != nil {
		log.Warn().Msgf("Failed to cleanup shared resources: %s. Retrying in 3 seconds...", string(respBytes))
		time.Sleep(3 * time.Second)
		respBytes, err = callApi("DELETE", urlDeleteSharedResources, tbAuth, nil, logs, "Cleanup Shared Resources (Retry)")
		if err != nil {
			return fmt.Errorf("failed to cleanup shared resources: %s", string(respBytes))
		}
	}

	sharedResResults := new(model.ResourceDeleteResults)
	if err := json.Unmarshal(respBytes, sharedResResults); err != nil {
		return fmt.Errorf("failed to unmarshal shared resource deletion results: %v", err)
	}

	prettySharedResResults, _ := json.MarshalIndent(sharedResResults, "", "   ")
	log.Info().Msgf("[Shared Resource Cleanup Results] \n%+v", string(prettySharedResResults))

	// Verify Shared Resource deletion (check if VPCs are gone)
	log.Info().Msg("Verifying Shared Resources (VPCs) are deleted...")
	urlGetVpcs := fmt.Sprintf("%s/ns/%s/resources/vNet", tbApiBase, nsId)
	time.Sleep(15 * time.Second)
	respBytes, err = callApi("GET", urlGetVpcs, tbAuth, nil, nil, "Verify VPC deletion")
	if err != nil {
		log.Warn().Msgf("Failed to check VPC list: %v", err)
	} else {
		var checkRes struct {
			VNet []model.VNetInfo `json:"vNet"`
		}
		json.Unmarshal(respBytes, &checkRes)
		if len(checkRes.VNet) == 0 {
			log.Info().Msg("All Shared Resources (VPCs) successfully deleted.")
			return nil
		}
		log.Warn().Msgf("Warning: %d VPCs still exist after deletion attempt", len(checkRes.VNet))
	}
	return nil
}

func cleanupShared(cmd *cobra.Command, args []string) {
	nsId, _ := cmd.Flags().GetString("nsId")
	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if nsId == "" {
		log.Fatal().Msg("nsId is not set")
		return
	}

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	log.Info().Msgf("Starting manual cleanup of shared resources in namespace: %s", nsId)
	err := cleanupSharedResourcesInternal(nsId, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg("Manual cleanup failed")
	} else {
		log.Info().Msg("Manual cleanup completed successfully")
	}
}

func terminateInfra(cmd *cobra.Command, args []string) {
	var err error
	var respBytes []byte

	// Set namespace ID, Infra ID, and request body file
	nsId, _ := cmd.Flags().GetString("namespaceId")
	infraId, _ := cmd.Flags().GetString("infraId")
	option, _ := cmd.Flags().GetString("option")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("Option", option).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("Infra ID", infraId).
		Str("Option", option).
		Msg("[config.yaml]")

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: readiness check

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil, nil, "Readiness Check")
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(model.SimpleMsg)
	if err := json.Unmarshal(respBytes, resTbReadiness); err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	log.Info().Msg(resTbReadiness.Message)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Delete Infra with option

	err = terminateInfraInternal(nsId, infraId, option, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Delete Shared Resources

	err = cleanupSharedResourcesInternal(nsId, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
}

func batchTestVpn(cmd *cobra.Command, args []string) {
	nsId, _ := cmd.Flags().GetString("nsId")
	infraId, _ := cmd.Flags().GetString("infraId")
	filePath, _ := cmd.Flags().GetString("file")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}
	if infraId == "" {
		infraId = viper.GetString("tumblebug.demo.infraId")
	}

	if nsId == "" || infraId == "" || filePath == "" {
		log.Fatal().Msg("nsId, infraId, or file path is not set")
		return
	}

	tbAuth := map[string]string{
		"username": viper.GetString("TB_API_USERNAME"),
		"password": viper.GetString("TB_API_PASSWORD"),
	}

	// Readiness Check
	urlReadiness := fmt.Sprintf("%s/readyz", tbApiBase)
	_, err := callApi("GET", urlReadiness, tbAuth, nil, nil, "Readiness Check")
	if err != nil {
		log.Fatal().Err(err).Msg("Tumblebug is not ready")
		return
	}

	summaryResults := []TestResult{}
	provisionLogs := []ApiLog{}
	cleanupLogs := []ApiLog{}

	var testPairs TestTargetPairs
	err = viper.UnmarshalKey("testTargetPairs", &testPairs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal testTargetPairs from config")
		return
	}

	enabledCSPs := getEnabledCSPs(testPairs)
	if len(enabledCSPs) == 0 {
		log.Fatal().Msg("No test cases enabled (execute: true). Enable at least one test case.")
		return
	}

	// Keep AWS as hub for aws-to-site VPN scenarios.
	enabledCSPs["aws"] = true
	log.Info().Msgf("Enabled CSPs for Infra provisioning: %v", enabledCSPs)

	// Phase 1: Infrastructure Provisioning
	log.Info().Msg("Phase 1: Infrastructure Provisioning")
	infraInfo, err := createInfraInternal(nsId, infraId, "infraDynamic.json", tbAuth, &provisionLogs, enabledCSPs)
	if err != nil {
		log.Error().Err(err).Msg("Infra Creation failed. Rolling back...")
		// Use "terminate" option to ensure VMs are cleaned up before shared resources
		terminateInfraInternal(nsId, infraId, "terminate", tbAuth, &cleanupLogs)
		cleanupSharedResourcesInternal(nsId, tbAuth, &cleanupLogs)
		saveDetailedReport("test-results/provision.md", "Phase 1: Infrastructure Provisioning (Failed)", provisionLogs, "")
		return
	}
	saveDetailedReport("test-results/provision.md", "Phase 1: Infrastructure Provisioning (Success)", provisionLogs, "")

	// Phase 2: Batch VPN Tests
	log.Info().Msg("Phase 2: Batch VPN Tests")
	interrupted := false
	for _, tc := range testPairs.TestCases {
		if !tc.Execute {
			log.Info().Msgf("--- Skipping Disabled Case: %s to %s ---", tc.Site1, tc.Site2)
			continue
		}
		log.Info().Msgf("--- Testing Case: %s to %s ---", tc.Site1, tc.Site2)
		result := TestResult{TestCase: tc, ApiLogs: []ApiLog{}}

		err := runVpnTestCase(nsId, infraId, infraInfo, tc, tbAuth, &result)
		summaryResults = append(summaryResults, result)

		// Save individual report
		reportFilename := fmt.Sprintf("test-results/%s-to-%s-vpn.md", tc.Site1, tc.Site2)
		infraRef := "Refer to [Provisioning Report](provision.md) and [Cleanup Report](cleanup.md) for infrastructure details."
		saveDetailedReport(reportFilename, fmt.Sprintf("VPN Test: %s to %s", tc.Site1, tc.Site2), result.ApiLogs, infraRef)

		if err != nil {
			log.Error().Err(err).Msg("Test case failed. Interrupting batch...")
			interrupted = true
			break
		}
	}

	// Phase 3: Cleanup
	log.Info().Msg("Phase 3: Cleanup")

	// Ensure ALL VPNs are deleted first to avoid DependencyViolation in Tumblebug/Spider
	err = deleteAllVpnsInternal(nsId, infraId, tbAuth, &cleanupLogs)
	if err != nil {
		log.Error().Err(err).Msg("Error during VPN orphan cleanup")
	}

	err = terminateInfraInternal(nsId, infraId, "terminate", tbAuth, &cleanupLogs)
	if err != nil {
		log.Error().Err(err).Msg("Error during Infra termination")
	}

	err = cleanupSharedResourcesInternal(nsId, tbAuth, &cleanupLogs)
	if err != nil {
		log.Error().Err(err).Msg("Error during Shared Resource cleanup")
	}
	saveDetailedReport("test-results/cleanup.md", "Phase 3: Cleanup Phase", cleanupLogs, "")

	// Final Summary Report
	generateSummaryReport("test-results/summary.md", summaryResults, interrupted)
	log.Info().Msg("Batch VPN testing completed.")
}

func runVpnTestCase(nsId, infraId string, infraInfo *model.InfraInfo, tc TestCase, tbAuth map[string]string, result *TestResult) error {
	awsVNetId := ""
	targetVNetId := ""

	for _, node := range infraInfo.Node {
		providerName := strings.ToLower(node.ConnectionConfig.ProviderName)
		if providerName == "aws" {
			awsVNetId = node.VNetId
		} else if providerName == strings.ToLower(tc.Site2) {
			targetVNetId = node.VNetId
		}
	}

	if awsVNetId == "" || targetVNetId == "" {
		result.CreateRes = "Failed: Missing VNet Info"
		return fmt.Errorf("missing VNets for %s to %s", tc.Site1, tc.Site2)
	}

	// 1. POST VPN
	site2Props := map[string]interface{}{"vNetId": targetVNetId}
	tcSite2Lower := strings.ToLower(tc.Site2)
	switch tcSite2Lower {
	case "gcp":
		site2Props["cspSpecificProperty"] = map[string]interface{}{"gcp": map[string]string{"bgpAsn": "65530"}}
	case "azure":
		site2Props["cspSpecificProperty"] = map[string]interface{}{"azure": map[string]string{"bgpAsn": "65531", "gatewaySubnetCidr": "", "vpnSku": "VpnGw1AZ"}}
	case "alibaba":
		site2Props["cspSpecificProperty"] = map[string]interface{}{"alibaba": map[string]string{"bgpAsn": "65532"}}
	}

	reqVpn := map[string]interface{}{
		"name": tc.VpnId,
		"site1": map[string]interface{}{
			"cspSpecificProperty": map[string]interface{}{"aws": map[string]string{"bgpAsn": "64512"}},
			"vNetId":              awsVNetId,
		},
		"site2": site2Props,
	}

	urlPostVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn", tbApiBase, nsId, infraId)
	_, err := callApi("POST", urlPostVpn, tbAuth, reqVpn, &result.ApiLogs, "Create VPN")
	if err != nil {
		result.CreateRes = "Failed"
		return err
	}
	result.CreateRes = "Success"

	// 2. GET VPN Info
	urlGetVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, tc.VpnId)
	callApi("GET", urlGetVpn, tbAuth, nil, &result.ApiLogs, "Get VPN Info")

	// 3. GET all VPNs
	urlListVpnId := fmt.Sprintf("%s/ns/%s/infra/%s/vpn?option=IdList", tbApiBase, nsId, infraId)
	callApi("GET", urlListVpnId, tbAuth, nil, &result.ApiLogs, "List VPN IDs")
	urlListVpnInfo := fmt.Sprintf("%s/ns/%s/infra/%s/vpn?option=InfoList", tbApiBase, nsId, infraId)
	callApi("GET", urlListVpnInfo, tbAuth, nil, &result.ApiLogs, "List VPN Infos")

	// Wait for BGP propagation
	log.Info().Msg("Waiting 60 seconds before health check... (e.g., waiting for BGP propagation)")
	time.Sleep(60 * time.Second)

	// 4. VPN Health Check (bidirectional ping test via health API)
	urlHealthCheck := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s/health", tbApiBase, nsId, infraId, tc.VpnId)
	reqHealth := map[string]interface{}{
		"userName":    "cb-user",
		"pingCount":   4,
		"intervalSec": 15,
		"maxAttempts": 20,
	}

	log.Info().Msgf("Running VPN health check for %s (bidirectional ping test)", tc.VpnId)
	respBytes, err := callApi("POST", urlHealthCheck, tbAuth, reqHealth, &result.ApiLogs, "VPN Health Check")
	if err != nil {
		result.HealthCheckStatus = "Failed"
		log.Error().Err(err).Msg("VPN health check API call failed")
	} else {
		var healthResp model.VpnHealthCheckResponse
		if jsonErr := json.Unmarshal(respBytes, &healthResp); jsonErr != nil {
			result.HealthCheckStatus = "Failed"
			log.Error().Err(jsonErr).Msg("Failed to parse health check response")
		} else {
			prettyResp, _ := json.MarshalIndent(healthResp, "", "  ")
			log.Debug().Msgf("[VPN Health Check Response]\n%s", string(prettyResp))

			if healthResp.Reachable {
				result.HealthCheckStatus = "Success"
				log.Info().Msgf("VPN health check succeeded: %s", healthResp.Message)
			} else {
				result.HealthCheckStatus = "Failed"
				log.Warn().Msgf("VPN health check failed: %s", healthResp.Message)
				for _, r := range healthResp.Results {
					log.Warn().Msgf("  [%s] reachable=%v, attempts=%d, packetLoss=%s",
						r.Direction, r.Reachable, r.Attempts, r.PingStats.PacketLoss)
				}
			}
		}
	}

	// 5. DELETE VPN
	urlDeleteVpn := fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, tc.VpnId)
	_, err = callApi("DELETE", urlDeleteVpn, tbAuth, nil, &result.ApiLogs, "Delete VPN")
	if err != nil {
		log.Warn().Msgf("Failed to delete VPN %s: %v. Retrying in 3 seconds...", tc.VpnId, err)
		time.Sleep(3 * time.Second)
		_, err = callApi("DELETE", urlDeleteVpn, tbAuth, nil, &result.ApiLogs, "Delete VPN (Retry)")
		if err != nil {
			result.DeleteRes = "Failed"
			return err
		}
	}

	// Verify VPN deletion for this specific case
	log.Info().Msgf("Verifying deletion of VPN: %s", tc.VpnId)
	time.Sleep(15 * time.Second)
	urlGetVpn = fmt.Sprintf("%s/ns/%s/infra/%s/vpn/%s", tbApiBase, nsId, infraId, tc.VpnId)
	delCheckResp, err := callApi("GET", urlGetVpn, tbAuth, nil, nil, "Verify individual VPN deletion")
	if !isDeleted(err, delCheckResp) {
		log.Warn().Msgf("VPN %s deletion verification failed or timed out", tc.VpnId)
		result.DeleteRes = "Timeout/Failed"
		if err != nil {
			return fmt.Errorf("failed to check VPN status: %v", err)
		}
		return fmt.Errorf("timeout waiting for VPN %s deletion", tc.VpnId)
	}

	log.Info().Msgf("VPN %s successfully deleted.", tc.VpnId)
	result.DeleteRes = "Success"
	return nil
}

func saveDetailedReport(filename, title string, logs []ApiLog, extraInfo string) {
	os.MkdirAll("test-results", 0755)
	md := fmt.Sprintf("# %s\n\n", title)
	if extraInfo != "" {
		md += fmt.Sprintf("%s\n\n", extraInfo)
	}
	for i, log := range logs {
		md += fmt.Sprintf("## Step %d: %s\n\n", i+1, log.Step)
		md += fmt.Sprintf("- **Method**: %s\n", log.Method)
		md += fmt.Sprintf("- **URL**: %s\n", log.URL)
		md += fmt.Sprintf("- **Status**: %s\n", log.ResponseStatus)
		md += fmt.Sprintf("- **Elapsed**: %s\n\n", log.ElapsedTime)

		if log.RequestPayload != nil {
			reqJson, _ := json.MarshalIndent(log.RequestPayload, "", "  ")
			md += "### Request Body\n```json\n" + string(reqJson) + "\n```\n\n"
		}
		if log.ResponsePayload != nil {
			respJson, _ := json.MarshalIndent(log.ResponsePayload, "", "  ")
			md += "### Response Body\n```json\n" + string(respJson) + "\n```\n\n"
		}
		md += "---\n\n"
	}
	os.WriteFile(filename, []byte(md), 0644)
}

func generateSummaryReport(filename string, results []TestResult, interrupted bool) {
	os.MkdirAll("test-results", 0755)
	md := "# VPN Batch Test Summary\n\n"
	md += "## Test Workflow\n\n"
	md += "1. **Phase 1: Infrastructure Provisioning** (Infra creation with multi-cloud VMs)\n"
	md += "2. **Phase 2: VPN Tests** (Sequential VPN creation > View > Health Check (ping) > Deletion)\n"
	md += "3. **Phase 3: Cleanup** (Infra termination and Shared Resource deletion)\n\n"
	md += "--- \n\n"

	if interrupted {
		md += "> [!WARNING]\n> The batch test was interrupted due to a failure.\n\n"
	}

	md += "## Step-by-Step VPN Test Results\n\n"
	md += "| Test Case | Create | Health Check (ping) | Delete | Result |\n"
	md += "| --- | --- | --- | --- | --- |\n"
	for _, res := range results {
		status := "✅"
		if res.CreateRes != "Success" || res.HealthCheckStatus != "Success" || res.DeleteRes != "Success" {
			status = "❌"
		}
		md += fmt.Sprintf("| %s to %s | %s | %s | %s | %s |\n", res.TestCase.Site1, res.TestCase.Site2, res.CreateRes, res.HealthCheckStatus, res.DeleteRes, status)
	}
	md += "\n---\n\n"
	md += "### Detailed Logs\n\n"
	md += "- For infrastructure provisioning details, see `provision.md`.\n"
	md += "- For detailed VPN test traces, see corresponding `<site1>-to-<site2>-vpn.md` files.\n"
	md += "- For cleanup operation details, see `cleanup.md`.\n"

	os.WriteFile(filename, []byte(md), 0644)
}

// Helper to check if Tumblebug returned a "not found" / "does not exist" error
func isDeleted(err error, respBytes []byte) bool {
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "404") || strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "not found") {
			return true
		}
	}
	if respBytes != nil {
		respStr := strings.ToLower(string(respBytes))
		if strings.Contains(respStr, "does not exist") || strings.Contains(respStr, "not found") {
			return true
		}
	}
	return false
}
func callApi(
	method string,
	apiUrl string,
	auth map[string]string,
	reqBody interface{},
	logs *[]ApiLog,
	step string,
) ([]byte, error) {

	// Add 5-second sleep for API stability as requested by USER
	time.Sleep(5 * time.Second)

	client := resty.New()
	client.SetTimeout(1 * time.Hour)

	// Prepare the request
	// Set header and basic auth
	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBasicAuth(auth["username"], auth["password"])

	// Set the request body
	var body []byte
	var err error
	if reqBody != nil {
		body, err = json.Marshal(reqBody)
		if err != nil {
			log.Printf("Error marshalling request body: %v", err)
			return nil, err
		}
		req.SetBody(body)
	}

	var resp *resty.Response

	// Log the request
	log.Debug().Msgf("Request '%s %s'", method, apiUrl)

	// Make the request based on the method and measure elapsed time
	start := time.Now()
	switch method {
	case "GET":
		resp, err = req.Get(apiUrl)
	case "POST":
		resp, err = req.Post(apiUrl)
	case "DELETE":
		resp, err = req.Delete(apiUrl)
	default:
		log.Error().Msgf("Unsupported request method: %s", method)
		return nil, fmt.Errorf("unsupported request method: %s", method)
	}
	elapsed := time.Since(start)

	// Check the request
	if err != nil {
		log.Error().Err(err).Msg("failed to make the request")
		return nil, err
	}

	// Record logs if requested
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
			RequestPayload:  reqPayload,
			ResponsePayload: respPayload,
			ResponseStatus:  resp.Status(),
			ElapsedTime:     elapsed.Round(time.Millisecond).String(),
		})
	}

	// Check the response
	if resp.IsError() {
		err = fmt.Errorf("%s", resp.Status())
	}

	return resp.Body(), err
}
