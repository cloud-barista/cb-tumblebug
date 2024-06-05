package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/api/rest/server/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/common/logger"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/rs/zerolog/log"

	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
	"github.com/go-resty/resty/v2"

	terrariumModel "github.com/cloud-barista/mc-terrarium/pkg/api/rest/model"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tbApiBase string
var epTerrarium string

func init() {
	setConfig()
	tbApiBase = viper.GetString("tumblebug.endpoint") + "/tumblebug"   // ex) "http://localhost:1323/tumblebug"
	epTerrarium = viper.GetString("terrarium.endpoint") + "/terrarium" // ex) "http://localhost:8888/terrarium"
}

// setConfig get cloud settings from a config file
func setConfig() {
	fileName := "config"
	viper.AddConfigPath(".")
	viper.SetConfigType("yaml")
	viper.SetConfigName(fileName)

	err := viper.ReadInConfig()
	if err != nil {
		log.Error().Err(err).Msg("")
		log.Fatal().Err(err).Msg("Error reading config file (config.yaml)")
	}

	log.Info().Msg(viper.ConfigFileUsed())

	// Map environment variable names to config file key names
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "./app",
		Short: "[Demo] VPN tunnel on MCIS",
		Long: `
########################################################################
## [Demo] This program demonstrates VPN tunnel configuration on MCIS. ##
########################################################################`,
	}

	var createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create resources",
	}

	var createMcisDynamicCmd = &cobra.Command{
		Use:   "mcis",
		Short: "Create MCIS dynamically",
		Run:   createMcis,
	}
	// Command-line flags with shorthand
	createMcisDynamicCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	createMcisDynamicCmd.Flags().StringP("mcisId", "m", "", "MCIS ID")
	createMcisDynamicCmd.Flags().StringP("file", "f", "", "Specify the JSON file for the request body")

	var createVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Create GCP to AWS VPN tunnel",
		Run:   createVpnTunnel,
	}
	// Command-line flags with shorthand
	createVpnCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	createVpnCmd.Flags().StringP("mcisId", "m", "", "MCIS ID")
	createVpnCmd.Flags().StringP("trId", "t", "", "Terrarium ID")

	createCmd.AddCommand(
		createMcisDynamicCmd,
		createVpnCmd,
	)

	var deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
	}

	var terminateMcisCmd = &cobra.Command{
		Use:   "mcis",
		Short: "Suspend and terminate MCIS",
		Run:   terminateMcis,
	}
	terminateMcisCmd.Flags().StringP("nsId", "n", "", "Namespace ID")
	terminateMcisCmd.Flags().StringP("mcisId", "m", "", "MCIS ID")

	var destroyVpnCmd = &cobra.Command{
		Use:   "vpn",
		Short: "Destroy GCP to AWS VPN tunnel",
		Run:   destroyVpnTunnel,
	}
	// Command-line flags with shorthand
	destroyVpnCmd.Flags().StringP("trId", "t", "", "Terrarium ID")

	deleteCmd.AddCommand(
		terminateMcisCmd,
		destroyVpnCmd,
	)

	// Add commands
	rootCmd.AddCommand(
		createCmd,
		deleteCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Error executing demo to configure vpn tunnel")
	}
}

func createMcis(cmd *cobra.Command, args []string) {

	// Set namespace ID, MCIS ID, and request body file
	nsId, _ := cmd.Flags().GetString("namespaceId")
	mcisId, _ := cmd.Flags().GetString("mcisId")
	filePath, _ := cmd.Flags().GetString("file")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Str("File path", filePath).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if mcisId == "" {
		mcisId = viper.GetString("tumblebug.demo.mcisId")
	}

	if filePath == "" {
		filePath = viper.GetString("tumblebug.api.mcisDynamic.reqBody")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Str("File path", filePath).
		Msg("[config.yaml]")

	if nsId == "" || mcisId == "" || filePath == "" {
		err := fmt.Errorf("bad request: nsId, mcisId, or file path is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("MCIS ID", mcisId).
			Str("File path", filePath).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting creating an MCIS dynamically...")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare to call Tumblebug APIs
	authInfoFilename := viper.GetString("tumblebug.auth.info")
	authInfoFile, err := os.Open(authInfoFilename)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer authInfoFile.Close()

	tbAuthData, err := io.ReadAll(authInfoFile)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	tbAuth := map[string]string{}
	err = json.Unmarshal(tbAuthData, &tbAuth)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal AuthData for Tumblebug")
		return
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: check readiness

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	var respBytes []byte
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(common.SimpleMsg)
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
	// Tumblebug API: mcisDynamic

	// Set the API path
	urlPostMcisDynamic := fmt.Sprintf("%s/ns/%s/mcisDynamic", tbApiBase, nsId)

	// Read the request body written in mcisDynamic.json
	mcisDynamicFile, err := os.Open(filePath)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to open %s", filePath)
		return
	}
	defer mcisDynamicFile.Close()

	mcisDynamicData, err := io.ReadAll(mcisDynamicFile)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read %s", filePath)
		return
	}

	reqMcisDynamic := new(mcis.TbMcisDynamicReq)
	err = json.Unmarshal(mcisDynamicData, &reqMcisDynamic)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to unmarshal %s", filePath)
		return
	}

	// Set MCIS ID
	reqMcisDynamic.Name = mcisId

	respBytes, err = callApi("POST", urlPostMcisDynamic, tbAuth, reqMcisDynamic)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	mcisInfo := new(mcis.TbMcisInfo)
	if err := json.Unmarshal(respBytes, mcisInfo); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyMcisInfo, err := json.MarshalIndent(mcisInfo, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}

	log.Debug().Msgf("[Response] %+v", string(prettyMcisInfo))
}

func createVpnTunnel(cmd *cobra.Command, args []string) {

	// Set namespace ID, MCIS ID, and terrarium ID
	nsId, _ := cmd.Flags().GetString("namespaceId")
	mcisId, _ := cmd.Flags().GetString("mcisId")
	trId, _ := cmd.Flags().GetString("trId")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Str("Terrarium ID", trId).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if mcisId == "" {
		mcisId = viper.GetString("tumblebug.demo.mcisId")
	}

	if trId == "" {
		trId = viper.GetString("terrarium.demo.terrariumId")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Str("Terrarium ID", trId).
		Msg("[config.yaml]")

	if nsId == "" || mcisId == "" || trId == "" {
		err := fmt.Errorf("bad request: nsId, mcisId, or rgId is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("MCIS ID", mcisId).
			Str("Terrarium ID", trId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting a demo of vpn tunnel configuration...")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare to call Tumblebug APIs
	authInfoFilename := viper.GetString("tumblebug.auth.info")
	authInfoFile, err := os.Open(authInfoFilename)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer authInfoFile.Close()

	tbAuthData, err := io.ReadAll(authInfoFile)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	tbAuth := map[string]string{}
	err = json.Unmarshal(tbAuthData, &tbAuth)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal AuthData for Tumblebug")
		return
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: readiness check

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	var respBytes []byte
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(common.SimpleMsg)
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
	// Tumblebug API: Get MCIS

	// Set the API path
	queryParams := "" //"option=status"
	urlGetMcisStatus := fmt.Sprintf("%s/ns/%s/mcis/%s", tbApiBase, nsId, mcisId)
	if queryParams != "" {
		urlGetMcisStatus += "?" + queryParams
	}

	// Request to create an mcis dynamically
	respBytes, err = callApi("GET", urlGetMcisStatus, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	mcisInfo := new(mcis.TbMcisInfo)
	if err := json.Unmarshal(respBytes, mcisInfo); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyMcisInfo, err := json.MarshalIndent(mcisInfo, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}

	log.Debug().Msgf("[Response] %+v", string(prettyMcisInfo))

	// Print the mcisInfo
	for _, vm := range mcisInfo.Vm {
		log.Debug().
			Str("ProviderName", vm.ConnectionConfig.ProviderName).
			Str("ConfigName", vm.ConnectionConfig.ConfigName).
			Msg("ConnectionConfig managed by Cloud-Barista system")

		log.Debug().
			Str("VM ID", vm.Id).
			Str("VNet ID", vm.VNetId).
			Str("Subnet ID", vm.SubnetId).
			Msg("IDs managed by Cloud-Barista system")

		log.Debug().
			Str("VPC/vNet ID", vm.CspViewVmDetail.VpcIID.SystemId).
			Str("Subnet ID", vm.CspViewVmDetail.SubnetIID.SystemId).
			Str("Region", vm.CspViewVmDetail.Region.Region).
			Str("Region", vm.CspViewVmDetail.Region.Zone).
			Msg("IDs managed by CSPs")

	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare information needed to configure VPN tunnel

	awsRegion := ""
	awsVpcId := ""
	awsSubnetId := ""
	gcpRegion := ""
	gcpVpcNetworkName := ""
	azureRegion := ""
	azureResourceGroupName := ""
	azureVirtualNetworkName := ""
	azureGatewaySubnetCidrBlock := ""

	vNetId := ""
	// Print the mcisInfo
	for _, vm := range mcisInfo.Vm {
		providerName := strings.ToLower(vm.ConnectionConfig.ProviderName)
		switch providerName {
		case "aws":
			awsRegion = vm.CspViewVmDetail.Region.Region
			awsVpcId = vm.CspViewVmDetail.VpcIID.SystemId
			awsSubnetId = vm.CspViewVmDetail.SubnetIID.SystemId
		case "gcp":
			gcpRegion = vm.CspViewVmDetail.Region.Region
			gcpVpcNetworkName = vm.CspViewVmDetail.VpcIID.SystemId
		case "azure":
			azureRegion = vm.CspViewVmDetail.Region.Region

			// Sample
			// /subscriptions/xxxxxxxxx/resourceGroups/cb-tb-az-krc-tb/providers/xxxxxxx \
			// /virtualNetworks/kdemo-ns01-kdemo-ns01-systemdefault-az-krc-cobrrejpr52be711ok20
			parts := strings.Split(vm.CspViewVmDetail.VpcIID.SystemId, "/")
			log.Debug().Msgf("parts: %+v", parts)
			azureResourceGroupName = parts[4]
			azureVirtualNetworkName = parts[8]

			// azureGatewaySubnetCidrBlock = ""
			vNetId = vm.VNetId
		}
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Get VNet

	urlGetVNet := fmt.Sprintf("%s/ns/%s/resources/vNet/%s", tbApiBase, nsId, vNetId)
	// Request
	respBytes, err = callApi("GET", urlGetVNet, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	vNetInfo := new(mcir.TbVNetInfo)
	if err := json.Unmarshal(respBytes, vNetInfo); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyVNetInfo, err := json.MarshalIndent(vNetInfo, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}
	log.Debug().Msgf("[Response] %+v", string(prettyVNetInfo))

	// Find the next subnet CIDR block (temporarily use)
	subnetCount := len(vNetInfo.SubnetInfoList)
	lastSubnetCidr := vNetInfo.SubnetInfoList[subnetCount-1].IPv4_CIDR

	nextCidr, err := netutil.NextSubnet(lastSubnetCidr, vNetInfo.CidrBlock)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get the next subnet CIDR")
		return
	}

	azureGatewaySubnetCidrBlock = nextCidr

	// Print information needed to configure VPN tunnel
	log.Debug().Msgf("AWS Region: %s", awsRegion)
	log.Debug().Msgf("AWS VPC ID: %s", awsVpcId)
	log.Debug().Msgf("AWS Subnet ID: %s", awsSubnetId)
	log.Debug().Msgf("GCP Region: %s", gcpRegion)
	log.Debug().Msgf("GCP VPC Network Name: %s", gcpVpcNetworkName)
	log.Debug().Msgf("Azure Region: %s", azureRegion)
	log.Debug().Msgf("Azure Resource Group Name: %s", azureResourceGroupName)
	log.Debug().Msgf("Azure Virtual Network Name: %s", azureVirtualNetworkName)
	log.Debug().Msgf("Azure Gateway Subnet CIDR Block: %s", azureGatewaySubnetCidrBlock)

	log.Info().Msg("Information needed to configure VPN tunnel is ready")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Configure VPN tunnel

	// Prepare to call mc-terrarium APIs
	authTerrarium := tbAuth

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: readiness check

	urlTerrariumReadiness := fmt.Sprintf("%s/readyz", epTerrarium)

	// Request readiness check
	respBytes, err = callApi("GET", urlTerrariumReadiness, authTerrarium, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTerrariumReadiness := new(terrariumModel.Response)
	if err := json.Unmarshal(respBytes, resTerrariumReadiness); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResTerrariumReadiness, err := json.MarshalIndent(resTerrariumReadiness, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}
	log.Debug().Msgf("[Response] %+v", string(prettyResTerrariumReadiness))

	log.Info().Msg(resTerrariumReadiness.Message)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: Initialize a multi-cloud terrarium for GCP to AWS VPN tunnel

	urlInitTerrarium := fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/env", epTerrarium, trId)

	// Request init
	respBytes, err = callApi("POST", urlInitTerrarium, authTerrarium, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	initRes := new(model.Response)
	if err := json.Unmarshal(respBytes, initRes); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	log.Trace().Msgf("[Response] %+v", initRes.Message)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: Create blueprints of GCP-AWS VPN tunnel
	urlInfracode := fmt.Sprintf("%s/rg/%s/vpn/gcp-aws/infracode", epTerrarium, trId)

	reqBody := terrariumModel.CreateInfracodeOfGcpAwsVpnRequest{
		TfVars: terrariumModel.TfVarsGcpAwsVpnTunnel{
			TerrariumId:       trId,
			AwsRegion:         awsRegion,
			AwsVpcId:          awsVpcId,
			AwsSubnetId:       awsSubnetId,
			GcpRegion:         gcpRegion,
			GcpVpcNetworkName: gcpVpcNetworkName,
		},
	}

	respBytes, err = callApi("POST", urlInfracode, authTerrarium, reqBody)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resInfracode := new(terrariumModel.Response)
	if err := json.Unmarshal(respBytes, resInfracode); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	log.Trace().Msgf("[Response] %+v", resInfracode.Detail)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: Create GCP-AWS VPN tunnel

	urlCreateGcpToAwsVpnTunnel := fmt.Sprintf("%s/rg/%s/vpn/gcp-aws", epTerrarium, trId)

	respBytes, err = callApi("POST", urlCreateGcpToAwsVpnTunnel, authTerrarium, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resCreateGcpToAwsVpnTunnel := new(terrariumModel.Response)
	if err := json.Unmarshal(respBytes, resCreateGcpToAwsVpnTunnel); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	log.Trace().Msgf("[Response] %+v", resCreateGcpToAwsVpnTunnel.Detail)
	fmt.Printf("[Response] %+v\n", resCreateGcpToAwsVpnTunnel.Detail)

	// type TfVarsGcpAzureVpnTunnel struct {
	// 	AzureRegion                 string `json:"azure-region" default:"koreacentral"`
	// 	AzureResourceGroupName      string `json:"azure-resource-group-name" default:"tofu-rg-01"`
	// 	AzureVirtualNetworkName     string `json:"azure-virtual-network-name" default:"tofu-azure-vnet"`
	// 	AzureGatewaySubnetCidrBlock string `json:"azure-gateway-subnet-cidr-block" default:"192.168.130.0/24"`
	// 	GcpRegion                   string `json:"gcp-region" default:"asia-northeast3"`
	// 	GcpVpcNetworkName           string `json:"gcp-vpc-network-name" default:"tofu-gcp-vpc"`
	// 	// AzureBgpAsn				 	string `json:"azure-bgp-asn" default:"65515"`
	// 	// GcpBgpAsn                   string `json:"gcp-bgp-asn" default:"65534"`
	// 	// AzureSubnetName             string `json:"azure-subnet-name" default:"tofu-azure-subnet-0"`
	// 	// GcpVpcSubnetworkName    string `json:"gcp-vpc-subnetwork-name" default:"tofu-gcp-subnet-1"`
	// }

}

func destroyVpnTunnel(cmd *cobra.Command, args []string) {

	// Set Terrarium ID
	rgId, _ := cmd.Flags().GetString("trId")

	log.Debug().
		Str("Terrarium ID", rgId).
		Msg("[args]")

	if rgId == "" {
		rgId = viper.GetString("terrarium.demo.terrariumId")
	}

	log.Debug().
		Str("Terrarium ID", rgId).
		Msg("[config.yaml]")

	if rgId == "" {
		err := fmt.Errorf("bad request: nsId, mcisId, or rgId is not set")
		log.Fatal().Err(err).
			Str("Terrarium ID", rgId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting deleting a VPN tunnel...")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare to call Tumblebug APIs
	authInfoFilename := viper.GetString("tumblebug.auth.info")
	authInfoFile, err := os.Open(authInfoFilename)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer authInfoFile.Close()

	tbAuthData, err := io.ReadAll(authInfoFile)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	tbAuth := map[string]string{}
	err = json.Unmarshal(tbAuthData, &tbAuth)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal AuthData for Tumblebug")
		return
	}

	// Prepare to call mc-terrarium APIs
	authTerrarium := tbAuth

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: readiness check

	urlTerrariumReadiness := fmt.Sprintf("%s/readyz", epTerrarium)

	// Request health check
	respBytes, err := callApi("GET", urlTerrariumReadiness, authTerrarium, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	type Response struct {
		Success bool   `json:"success" example:"true"`
		Text    string `json:"text" example:"Any text"`
	}

	// Print the response
	resTerrariumReadiness := new(Response)
	if err := json.Unmarshal(respBytes, resTerrariumReadiness); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResTerrariumReadiness, err := json.MarshalIndent(resTerrariumReadiness, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}
	log.Debug().Msgf("[Response] %+v", string(prettyResTerrariumReadiness))

	log.Info().Msg(resTerrariumReadiness.Text)

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// mc-terrarium: Destory providers for VPN tunnel (i.e, GCP and AWS)

	urlDestroy := fmt.Sprintf("%s/rg/%s/vpn/gcp-aws", epTerrarium, rgId)

	respBytes, err = callApi("DELETE", urlDestroy, authTerrarium, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resDestroy := new(Response)
	if err := json.Unmarshal(respBytes, resDestroy); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResDestroy, err := json.MarshalIndent(resDestroy, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}

	log.Trace().Msgf("[Response] %+v", string(prettyResDestroy))

}

func terminateMcis(cmd *cobra.Command, args []string) {

	// Set namespace ID, MCIS ID, and request body file
	nsId, _ := cmd.Flags().GetString("namespaceId")
	mcisId, _ := cmd.Flags().GetString("mcisId")

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Msg("[args]")

	if nsId == "" {
		nsId = viper.GetString("tumblebug.demo.nsId")
	}

	if mcisId == "" {
		mcisId = viper.GetString("tumblebug.demo.mcisId")
	}

	log.Debug().
		Str("Namespace ID", nsId).
		Str("MCIS ID", mcisId).
		Msg("[config.yaml]")

	if nsId == "" || mcisId == "" {
		err := fmt.Errorf("bad request: nsId or mcisId is not set")
		log.Fatal().Err(err).
			Str("Namespace ID", nsId).
			Str("MCIS ID", mcisId).
			Msg("Please set the values in the config file or pass them as arguments")
		return
	}

	log.Info().Msg("Starting terminating an MCIS dynamically...")

	///////////////////////////////////////////////////////////////////////////////////////////////////
	// Prepare to call Tumblebug APIs
	authInfoFilename := viper.GetString("tumblebug.auth.info")
	authInfoFile, err := os.Open(authInfoFilename)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}
	defer authInfoFile.Close()

	tbAuthData, err := io.ReadAll(authInfoFile)
	if err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	tbAuth := map[string]string{}
	err = json.Unmarshal(tbAuthData, &tbAuth)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal AuthData for Tumblebug")
		return
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: readiness check

	// Set the API path
	urlTumblebugReadiness := fmt.Sprintf("%s/readyz", tbApiBase)

	// Request readiness check
	var respBytes []byte
	respBytes, err = callApi("GET", urlTumblebugReadiness, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resTbReadiness := new(common.SimpleMsg)
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
	// Tumblebug API: Suspend MCIS

	// Set the API path
	queryParams := "action=suspend"
	urlGetMcisControlLifecycle := fmt.Sprintf("%s/ns/%s/control/mcis/%s", tbApiBase, nsId, mcisId)

	if queryParams != "" {
		urlGetMcisControlLifecycle += "?" + queryParams
	}

	respBytes, err = callApi("GET", urlGetMcisControlLifecycle, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	respText := string(respBytes)

	// // Print the response
	// resp := new(common.SimpleMsg)
	// if err := json.Unmarshal(respBytes, resp); err != nil {
	// 	log.Error().Err(err).Msg(string(respBytes))
	// 	return
	// }

	// prettyResp, err := json.MarshalIndent(resp, "", "   ")
	// if err != nil {
	// 	log.Error().Err(err).Msgf("")
	// 	return
	// }

	log.Debug().Msgf("[Response] %+v", respText)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Keep checking the status of MCIS until it is suspended

	for {
		time.Sleep(5 * time.Second)

		// Set the API path
		queryParams = "" //"option=status"
		urlGetMcisStatus := fmt.Sprintf("%s/ns/%s/mcis/%s", tbApiBase, nsId, mcisId)
		if queryParams != "" {
			urlGetMcisStatus += "?" + queryParams
		}

		// Request to create an mcis dynamically
		respBytes, err = callApi("GET", urlGetMcisStatus, tbAuth, nil)
		if err != nil {
			log.Error().Err(err).Msg(string(respBytes))
			return
		}

		// Print the response
		mcisInfo := new(mcis.TbMcisInfo)
		if err := json.Unmarshal(respBytes, mcisInfo); err != nil {
			log.Error().Err(err).Msg("")
			return
		}

		if strings.Contains(mcisInfo.Status, "Suspended") && !strings.Contains(mcisInfo.Status, "Partial") {
			log.Info().Msgf("MCIS(id: %s) status: ", mcisInfo.Status)
			break
		} else {
			log.Debug().Msgf("MCIS(id: %s) status: ", mcisInfo.Status)
		}
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Terminate MCIS

	// Set the API path
	queryParams = "action=terminate"
	urlGetMcisControlLifecycle = fmt.Sprintf("%s/ns/%s/control/mcis/%s", tbApiBase, nsId, mcisId)

	if queryParams != "" {
		urlGetMcisControlLifecycle += "?" + queryParams
	}

	respBytes, err = callApi("GET", urlGetMcisControlLifecycle, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	respText = string(respBytes)
	// Print the response
	// resp = new(common.SimpleMsg)
	// if err := json.Unmarshal(respBytes, resp); err != nil {
	// 	log.Error().Err(err).Msg("")
	// 	return
	// }

	// prettyResp, err = json.MarshalIndent(resp, "", "   ")
	// if err != nil {
	// 	log.Error().Err(err).Msgf("")
	// 	return
	// }

	log.Debug().Msgf("[Response] %+v", respText)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Keep checking the status of MCIS until it is terminated

	for {
		time.Sleep(5 * time.Second)

		// Set the API path
		queryParams = "" //"option=status"
		urlGetMcisStatus := fmt.Sprintf("%s/ns/%s/mcis/%s", tbApiBase, nsId, mcisId)
		if queryParams != "" {
			urlGetMcisStatus += "?" + queryParams
		}

		// Request to create an mcis dynamically
		respBytes, err = callApi("GET", urlGetMcisStatus, tbAuth, nil)
		if err != nil {
			log.Error().Err(err).Msg(string(respBytes))
			return
		}

		// Print the response
		mcisInfo := new(mcis.TbMcisInfo)
		if err := json.Unmarshal(respBytes, mcisInfo); err != nil {
			log.Error().Err(err).Msg("")
			return
		}

		if strings.Contains(mcisInfo.Status, "Terminated") && !strings.Contains(mcisInfo.Status, "Partial") {
			log.Info().Msgf("MCIS(id: %s) status: %s", mcisInfo.Id, mcisInfo.Status)
			break
		} else {
			log.Debug().Msgf("MCIS(id: %s) status: %s", mcisInfo.Id, mcisInfo.Status)
		}
	}

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Tumblebug API: Delete MCIS

	// Set the API path
	queryParams = ""
	urlDeleteMcis := fmt.Sprintf("%s/ns/%s/mcis/%s", tbApiBase, nsId, mcisId)

	if queryParams != "" {
		urlDeleteMcis += "?" + queryParams
	}

	respBytes, err = callApi("DELETE", urlDeleteMcis, tbAuth, nil)
	if err != nil {
		log.Error().Err(err).Msg(string(respBytes))
		return
	}

	// Print the response
	resp := new(common.SimpleMsg)
	if err := json.Unmarshal(respBytes, resp); err != nil {
		log.Error().Err(err).Msg("")
		return
	}

	prettyResp, err := json.MarshalIndent(resp, "", "   ")
	if err != nil {
		log.Error().Err(err).Msgf("")
		return
	}

	log.Debug().Msgf("[Response] %+v", string(prettyResp))
}

func callApi(
	method string,
	apiUrl string,
	auth map[string]string,
	reqBody interface{},
) ([]byte, error) {

	client := resty.New()

	// Prepare the request
	// Set header and basic auth
	req := client.R().
		SetHeader("Content-Type", "application/json").
		SetBasicAuth(auth["username"], auth["password"])

	// Set the request body
	if reqBody != nil {
		body, err := json.Marshal(reqBody)
		if err != nil {
			log.Printf("Error marshalling request body: %v", err)
			return nil, err
		}
		req.SetBody(body)
	}

	var resp *resty.Response
	var err error

	// Log the request
	log.Debug().Msgf("Request '%s %s'", method, apiUrl)

	// Make the request based on the method
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

	// Check the request
	if err != nil {
		log.Error().Err(err).Msg("failed to make the request")
		return nil, err
	}

	// Check the response
	if resp.IsError() {
		err = fmt.Errorf(resp.Status())
	}

	return resp.Body(), err
}
