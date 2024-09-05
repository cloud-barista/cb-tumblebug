package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type CloudInfo struct {
	CSPs map[string]CSPDetail `mapstructure:"cloud" json:"csps"`
}

// CSPDetail is structure for CSP information
type CSPDetail struct {
	Network NetworkDetail `mapstructure:"network" json:"networks"`
	// Description string                  `mapstructure:"description" json:"description"`
	// Driver      string                  `mapstructure:"driver" json:"driver"`
	// Links       []string                `mapstructure:"link" json:"links"`
	// Regions     map[string]RegionDetail `mapstructure:"region" json:"regions"`
}

// NetworkDetail is structure for network information
type NetworkDetail struct {
	CIDRPrefixLength CIDRPrefixLength `mapstructure:"cidr-prefix-length" json:"cidrPrefixLength"`
}

// CIDRPrefixLength 구조체 정의
type CIDRPrefixLength struct {
	Min int `mapstructure:"min" json:"min"`
	Max int `mapstructure:"max" json:"max"`
}

func main() {
	// Sample input request
	requestJSON := `{	
		"targetPrivateNetwork": "10.0.0.0/8",
		"supernettingEnabled": "true",
		"cspRegions": [
			{
				"connectionName": "aws-ap-northeast-2",
				"neededVNets": [
					{
						"subnetCount": 5,
						"subnetSize": 260,
						"zoneSelectionMethod": "firstTwoZones"
					},
					{
						"subnetCount": 2,
						"subnetSize": 100,
						"zoneSelectionMethod": "firstTwoZones"
					}
				]
			},
			{
				"connectionName": "gcp-asia-northeast2",
				"neededVNets": [
					{
						"subnetCount": 3,
						"subnetSize": 200,
						"zoneSelectionMethod": "firstTwoZones"
					}
				]
			},
			{
				"connectionName": "azure-koreacentral",
				"neededVNets": [
					{
						"subnetCount": 2,
						"subnetSize": 1000,
						"zoneSelectionMethod": "firstTwoZones"
					}
				]
			}
		]
	}`

	var request model.VNetDesignRequest
	err := json.Unmarshal([]byte(requestJSON), &request)
	if err != nil {
		fmt.Printf("Error parsing request: %v\n", err)
		return
	}

	// Design vNets
	response, err := resource.DesignVNets(&request)

	// Output the result
	if err != nil {
		fmt.Printf("Error designing VNets: %v\n", err)
	} else {
		fmt.Printf("Designed VNets: %v\n", response)
	}

	// TODO: Network validation function will be developed here (for the time being)
	// TODO: 1. Validate that the designed vNets meet the network characteristics or requirements of the CSP.
	// TODO: 2. And then, apply validation factors and functions to the Tumblebug main line code.

	// Init Viper
	v := viper.New()

	// Sample YAML data
	yamlData := `
cloud:
  aws:
    network: 
      cidr-prefix-length:
        min: 28
        max: 16
  gcp:
    network:
      cidr-prefix-length:
        min: 29
        max: 8
  azure:
    network:
      cidr-prefix-length:
        min: 29
        max: 8
`

	// Set configuration type and read the data
	v.SetConfigType("yaml")
	err = v.ReadConfig(bytes.NewReader([]byte(yamlData)))
	if err != nil {
		log.Fatal().Err(err).Msgf("")
	}

	// Unmarshal the data into the CloudInfo struct
	var cloudInfo CloudInfo
	err = v.Unmarshal(&cloudInfo)
	if err != nil {
		log.Fatal().Err(err).Msgf("")
	}

	// Print the min and max CIDR prefix length for each CSP
	fmt.Printf("AWS Min CIDR: %d, Max CIDR: %d\n", cloudInfo.CSPs["aws"].Network.CIDRPrefixLength.Min, cloudInfo.CSPs["aws"].Network.CIDRPrefixLength.Max)
	fmt.Printf("GCP Min CIDR: %d, Max CIDR: %d\n", cloudInfo.CSPs["gcp"].Network.CIDRPrefixLength.Min, cloudInfo.CSPs["gcp"].Network.CIDRPrefixLength.Max)
	fmt.Printf("Azure Min CIDR: %d, Max CIDR: %d\n", cloudInfo.CSPs["azure"].Network.CIDRPrefixLength.Min, cloudInfo.CSPs["azure"].Network.CIDRPrefixLength.Max)
}
