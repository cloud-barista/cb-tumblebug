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

// Package mcis is to manage multi-cloud infra service
package mcis

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	model "github.com/cloud-barista/cb-larva/poc-cb-net/pkg/cb-network/model"
	nethelper "github.com/cloud-barista/cb-larva/poc-cb-net/pkg/network-helper"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/go-resty/resty/v2"
)

// configureCloudAdaptiveNetwork configures a cloud adaptive network to VMs in an MCIS
func configureCloudAdaptiveNetwork(nsId string, mcisInfo TbMcisInfo) error {
	common.CBLog.Debug("Start.........")

	// Check cb-network endpoints
	gRPCServiceEndpoint, etcdEndpoints, err := checkCBNetworkEndpoints()
	if err != nil {
		return err
	}

	// Get subnet list
	ipNetworksInMCIS, err := getSubnetsInMCIS(nsId, mcisInfo)
	if err != nil {
		return err
	}

	// Get a propoer address space
	cladnetDescription := fmt.Sprintf("A cladnet for %s", mcisInfo.Id)
	cladnetSpec, err := createProperCloudAdaptiveNetwork(gRPCServiceEndpoint, ipNetworksInMCIS, mcisInfo.Id, cladnetDescription)
	if err != nil {
		log.Printf("Could not create a cloud adaptive network: %v\n", err)
		return err
	}

	log.Printf("Struct: %#v\n", cladnetSpec)

	// Install the cb-network agent
	content, err := installCBNetworkAgentToMcis(nsId, mcisInfo.Id, etcdEndpoints, cladnetSpec.ID)

	if err != nil {
		return err
	}
	common.PrintJsonPretty(content)

	common.CBLog.Debug("End.........")
	return nil
}

// checkCBNetworkEndpoints checks endpoints of cb-network service and etcd.
func checkCBNetworkEndpoints() (string, string, error) {
	common.CBLog.Debug("Start.........")

	// Get an endpoint of cb-network service
	serviceEndpoint := os.Getenv("CB_NETWORK_SERVICE_ENDPOINT")
	if serviceEndpoint == "" {
		return "", "", errors.New("could not load CB_NETWORK_SERVICE_ENDPOINT")
	}

	// Get endpoints of cb-network etcd which should be accessible from the remote
	etcdEndpoints := os.Getenv("CB_NETWORK_ETCD_ENDPOINTS")
	if etcdEndpoints == "" {
		return "", "", errors.New("could not load CB_NETWORK_ETCD_ENDPOINTS")
	}

	common.CBLog.Debug("End.........")
	return serviceEndpoint, etcdEndpoints, nil
}

// getSubnetsInMCIS extracts all subnets in MCIS.
func getSubnetsInMCIS(nsId string, mcisInfo TbMcisInfo) ([]string, error) {
	common.CBLog.Debug("Start.........")

	// Get IP Networks in MCIS
	ipNetsInMCIS := make([]string, 0)

	for _, tbVmInfo := range mcisInfo.Vm {
		// Get vNet info
		res, err := mcir.GetResource(nsId, common.StrVNet, tbVmInfo.VNetId)
		if err != nil {
			return ipNetsInMCIS, err
		}

		tempVNetInfo, ok := res.(mcir.TbVNetInfo)
		if !ok {
			return ipNetsInMCIS, err
		}

		// Get IP Networks in a vNet
		for _, SubnetInfo := range tempVNetInfo.SubnetInfoList {
			ipNetsInMCIS = append(ipNetsInMCIS, SubnetInfo.IPv4_CIDR)
		}
	}

	// Trace
	common.CBLog.Tracef("%#v", ipNetsInMCIS)

	common.CBLog.Debug("End.........")
	return ipNetsInMCIS, nil
}

// createProperCloudAdaptiveNetwork requests available IPv4 private address spaces and uses the recommended address space.
func createProperCloudAdaptiveNetwork(gRPCServiceEndpoint string, ipNetworks []string, cladnetName string, cladnetDescription string) (model.CLADNetSpecification, error) {
	common.CBLog.Debug("Start.........")

	var spec model.CLADNetSpecification

	ipNetworksHolder := `{"ipNetworks": %s}`
	tempJSON, _ := json.Marshal(ipNetworks)
	ipNetworksString := fmt.Sprintf(ipNetworksHolder, string(tempJSON))
	fmt.Println(ipNetworksString)

	client := resty.New()

	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(ipNetworksString).
		Post(fmt.Sprintf("http://%s/v1/cladnet/available-ipv4-address-spaces", gRPCServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	log.Printf("Time: %v\n", resp.Time())
	log.Printf("Body: %v\n", resp)

	if err != nil {
		log.Printf("Could not request: %v\n", err)
		return model.CLADNetSpecification{}, err
	}

	var availableIPv4PrivateAddressSpaces nethelper.AvailableIPv4PrivateAddressSpaces

	json.Unmarshal(resp.Body(), &availableIPv4PrivateAddressSpaces)
	log.Printf("%+v\n", availableIPv4PrivateAddressSpaces)
	log.Printf("RecommendedIpv4PrivateAddressSpace: %#v", availableIPv4PrivateAddressSpaces.RecommendedIPv4PrivateAddressSpace)

	cladnetSpecHolder := `{"id": "", "name": "%s", "ipv4AddressSpace": "%s", "description": "%s"}`
	cladnetSpecString := fmt.Sprintf(cladnetSpecHolder,
		cladnetName, availableIPv4PrivateAddressSpaces.RecommendedIPv4PrivateAddressSpace, cladnetDescription)
	log.Printf("%#v\n", cladnetSpecString)

	// Request to create a Cloud Adaptive Network
	resp, err = client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(cladnetSpecString).
		Post(fmt.Sprintf("http://%s/v1/cladnet", gRPCServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	log.Printf("Time: %v\n", resp.Time())
	log.Printf("Body: %v\n", resp)

	if err != nil {
		log.Printf("Could not request: %v\n", err)
		return model.CLADNetSpecification{}, err
	}

	json.Unmarshal(resp.Body(), &spec)
	log.Printf("%#v\n", spec)

	common.CBLog.Debug("End.........")
	return spec, nil
}

// installCBNetworkAgentToMcis installs cb-network agent to VMs in an MCIS by the remote command
func installCBNetworkAgentToMcis(nsId string, mcisId string, etcdEndpoints string, cladnetID string) ([]SshCmdResult, error) {
	common.CBLog.Debug("Start.........")

	// SSH command to install cb-network agents
	placeHolderCommand := `wget https://raw.githubusercontent.com/cloud-barista/cb-larva/develop/poc-cb-net/scripts/1.deploy-cb-network-agent.sh -O ~/1.deploy-cb-network-agent.sh; chmod +x ~/1.deploy-cb-network-agent.sh; source ~/1.deploy-cb-network-agent.sh '%s' %s`

	additionalEncodedString := strings.Replace(etcdEndpoints, "\"", "\\\"", -1)
	command := fmt.Sprintf(placeHolderCommand, additionalEncodedString, cladnetID)

	// Replace given parameter with the installation cmd
	mcisCmdReq := &McisCmdReq{}
	mcisCmdReq.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.
	mcisCmdReq.Command = command

	sshCmdResult, err := RemoteCommandToMcis(nsId, mcisId, mcisCmdReq)

	if err != nil {
		temp := []SshCmdResult{}
		common.CBLog.Error(err)
		return temp, err
	}

	common.CBLog.Debug("End.........")
	return sshCmdResult, nil
}
