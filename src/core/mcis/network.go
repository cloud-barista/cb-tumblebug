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
	"sync"
	"time"

	model "github.com/cloud-barista/cb-larva/poc-cb-net/pkg/cb-network/model"
	nethelper "github.com/cloud-barista/cb-larva/poc-cb-net/pkg/network-helper"
	ruletype "github.com/cloud-barista/cb-larva/poc-cb-net/pkg/rule-type"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/go-resty/resty/v2"
)

// NetworkReq is a struct for a request to configure Cloud Adaptive Network
type NetworkReq struct {
	ServiceEndpoint string   `json:"serviceEndpoint" example:"localhost:8053" default:""`
	EtcdEndpoints   []string `json:"etcdEndpoints" example:"PUBLIC_IP_1:2379,PUBLIC_IP_2:2379,..." default:""`
}

// ConfigureCloudAdaptiveNetwork configures a cloud adaptive network to VMs in an MCIS
func ConfigureCloudAdaptiveNetwork(nsId string, mcisId string, netReq *NetworkReq) (AgentInstallContentWrapper, error) {
	common.CBLog.Debug("Start.........")

	if err := common.CheckString(nsId); err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	if err := common.CheckString(mcisId); err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	if _, err := CheckMcis(nsId, mcisId); err != nil {
		return AgentInstallContentWrapper{}, err
	}

	// Get a list of VM ID
	vmIdList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	serviceEndpoint := netReq.ServiceEndpoint
	// if the parameter is not passed, try to read from the environment variable
	if serviceEndpoint == "" {
		common.CBLog.Printf("read env for CB_NETWORK_SERVICE_ENDPOINT")
		// Get an endpoint of cb-network service
		serviceEndpoint = os.Getenv("CB_NETWORK_SERVICE_ENDPOINT")
		if serviceEndpoint == "" {
			return AgentInstallContentWrapper{}, errors.New("there is no CB_NETWORK_SERVICE_ENDPOINT")
		}
	}
	common.CBLog.Printf("Network service endpoint: %+v", serviceEndpoint)

	etcdEndpoints := netReq.EtcdEndpoints
	// if the parameter is not passed, try to read from the environment variable
	if len(etcdEndpoints) == 0 {
		common.CBLog.Printf("read env for CB_NETWORK_ETCD_ENDPOINTS")
		// Get endpoints of cb-network etcd which should be accessible from the remote
		etcdEndpoints = strings.Split(os.Getenv("CB_NETWORK_ETCD_ENDPOINTS"), ",")
		if len(etcdEndpoints) == 0 {
			return AgentInstallContentWrapper{}, errors.New("there is no CB_NETWORK_ETCD_ENDPOINTS")
		}
	}
	common.CBLog.Printf("etcd endpoints: %+v", etcdEndpoints)

	// Get Cloud Adaptive Network
	cladnetSpec, err := getCloudAdaptiveNetwork(serviceEndpoint, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	// if not exist
	if cladnetSpec.CladnetID == "" {

		// Get subnet list
		ipNetworksInMCIS, err := getSubnetsInMCIS(nsId, mcisId, vmIdList)
		if err != nil {
			return AgentInstallContentWrapper{}, err
		}

		// Get a propoer address space
		cladnetDescription := fmt.Sprintf("A cladnet for %s", mcisId)
		cladnetSpec, err = createProperCloudAdaptiveNetwork(serviceEndpoint, ipNetworksInMCIS, mcisId, cladnetDescription)
		if err != nil {
			common.CBLog.Printf("could not create a cloud adaptive network: %v\n", err)
			return AgentInstallContentWrapper{}, err
		}
	}

	common.CBLog.Printf("CLADNet spec: %#v\n", cladnetSpec)

	// Prepare the installation command
	etcdEndpointsJSON, _ := json.Marshal(etcdEndpoints)
	command, err := getAgentInstallationCommand(string(etcdEndpointsJSON), cladnetSpec.CladnetID)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	common.CBLog.Printf("Command: %#v\n", command)

	// Replace given parameter with the installation cmd
	mcisCmdReq := McisCmdReq{}
	mcisCmdReq.UserName = "cb-user" // this MCIS user name is temporal code. Need to improve.
	mcisCmdReq.Command = command

	//// Install the cb-network agent to MCIS
	// sshCmdResults, err := installCBNetworkAgentToMcis(nsId, mcisId, mcisCmdReq)

	// Install cb-network agents in parallel
	var wg sync.WaitGroup
	chanResults := make(chan SshCmdResult)

	var sshCmdResults []SshCmdResult

	common.CBLog.Printf("VM list: %v\n", vmIdList)

	for _, vmId := range vmIdList {
		wg.Add(1)
		go func(nsId, mcisId, vmId string, mcisCmdReq McisCmdReq, chanResults chan SshCmdResult) {
			defer wg.Done()

			// Check NetworkAgentStatus
			vmObject, _ := GetVmObject(nsId, mcisId, vmId)
			common.CBLog.Printf("NetworkAgentStatus: %+v\n" + vmObject.NetworkAgentStatus)

			// Skip if in installing or installed status)
			if vmObject.NetworkAgentStatus != "installed" && vmObject.NetworkAgentStatus != "installing" {

				vmObject.NetworkAgentStatus = "installing"

				sshCmdResult, err := installCBNetworkAgentToVM(nsId, mcisId, vmId, mcisCmdReq)
				if err != nil {
					vmObject.NetworkAgentStatus = "installed"
				} else {
					vmObject.NetworkAgentStatus = "failed"
				}

				chanResults <- sshCmdResult
			}

		}(nsId, mcisId, vmId, mcisCmdReq, chanResults)

		// Temporarily sleep 3 sec, to assign IPs consecutively to VMs in a subGroup for a Cloud Adaptive Network
		time.Sleep(3 * time.Second)
	}

	go func() {
		wg.Wait()
		close(chanResults)
	}()

	// Collect the results of installing the cb-network agents in parallel
	contents := AgentInstallContentWrapper{}
	for result := range chanResults {
		tempContent := AgentInstallContent{}
		tempContent.McisId = mcisId
		tempContent.VmId = result.VmId
		tempContent.VmIp = result.VmIp
		tempContent.Result = result.Result

		contents.ResultArray = append(contents.ResultArray, tempContent)
	}
	common.PrintJsonPretty(sshCmdResults)

	common.CBLog.Debug("End.........")
	return contents, nil
}

// // readCBNetworkEndpoints checks endpoints of cb-network service and etcd.
// func readCBNetworkEndpoints() (string, string, error) {
// 	common.CBLog.Debug("Start.........")

// 	// Get an endpoint of cb-network service
// 	serviceEndpoint := os.Getenv("CB_NETWORK_SERVICE_ENDPOINT")
// 	if serviceEndpoint == "" {
// 		return "", "", errors.New("could not load CB_NETWORK_SERVICE_ENDPOINT")
// 	}

// 	// Get endpoints of cb-network etcd which should be accessible from the remote
// 	etcdEndpoints := os.Getenv("CB_NETWORK_ETCD_ENDPOINTS")
// 	if etcdEndpoints == "" {
// 		return "", "", errors.New("could not load CB_NETWORK_ETCD_ENDPOINTS")
// 	}

// 	common.CBLog.Debug("End.........")
// 	return serviceEndpoint, etcdEndpoints, nil
// }

// getCloudAdaptiveNetwork retrieves a Cloud Adaptive Network
func getCloudAdaptiveNetwork(networkServiceEndpoint string, cladnetId string) (model.CLADNetSpecification, error) {
	common.CBLog.Debug("Start.........")
	var cladnetSpec model.CLADNetSpecification

	client := resty.New()

	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetPathParams(map[string]string{
			"cladnetId": cladnetId,
		}).
		Get(fmt.Sprintf("http://%s/v1/cladnet/{cladnetId}", networkServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Error(err)
		return model.CLADNetSpecification{}, err
	}

	json.Unmarshal(resp.Body(), &cladnetSpec)
	common.CBLog.Printf("%+v\n", cladnetSpec)
	common.CBLog.Printf("The specification of a Cloud Adaptive Network: %+v", cladnetSpec)

	common.CBLog.Debug("End.........")
	return cladnetSpec, nil
}

// getSubnetsInMCIS extracts all subnets in MCIS.
func getSubnetsInMCIS(nsId string, mcisId string, vmList []string) ([]string, error) {
	common.CBLog.Debug("Start.........")

	ipNetsInMCIS := make([]string, 0)

	for _, vmId := range vmList {

		// Get vNet info
		tbVmInfo, err := GetVmObject(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
			return ipNetsInMCIS, err
		}

		// getVNet
		res, err := mcir.GetResource(nsId, common.StrVNet, tbVmInfo.VNetId)
		if err != nil {
			common.CBLog.Error(err)
			return ipNetsInMCIS, err
		}

		// type casting
		tempVNetInfo, ok := res.(mcir.TbVNetInfo)
		if !ok {
			common.CBLog.Error(err)
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
func createProperCloudAdaptiveNetwork(networkServiceEndpoint string, ipCIDRs []string, cladnetName string, cladnetDescription string) (model.CLADNetSpecification, error) {
	common.CBLog.Debug("Start.........")

	ipv4CidrsHolder := `{"ipv4Cidrs": %s}`
	tempJSON, _ := json.Marshal(ipCIDRs)
	ipv4CidrsString := fmt.Sprintf(ipv4CidrsHolder, string(tempJSON))
	fmt.Println(ipv4CidrsString)

	client := resty.New()

	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(ipv4CidrsString).
		Post(fmt.Sprintf("http://%s/v1/cladnet/availableIPv4AddressSpaces", networkServiceEndpoint))
	// Output print
	common.CBLog.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Printf("Could not request: %v\n", err)
		return model.CLADNetSpecification{}, err
	}

	var availableIPv4PrivateAddressSpaces nethelper.AvailableIPv4PrivateAddressSpaces

	json.Unmarshal(resp.Body(), &availableIPv4PrivateAddressSpaces)
	common.CBLog.Printf("%+v\n", availableIPv4PrivateAddressSpaces)
	common.CBLog.Printf("RecommendedIpv4PrivateAddressSpace: %#v", availableIPv4PrivateAddressSpaces.RecommendedIPv4PrivateAddressSpace)

	// if the cladnetName is unique, it can be used CladnetID.
	reqSpec := &model.CLADNetSpecification{
		CladnetID:        cladnetName,
		Name:             cladnetName,
		Ipv4AddressSpace: availableIPv4PrivateAddressSpaces.RecommendedIPv4PrivateAddressSpace,
		Description:      cladnetDescription,
	}
	// cladnetSpecHolder := `{"cladnetID": "", "name": "%s", "ipv4AddressSpace": "%s", "description": "%s", ruleType": ""}`
	// cladnetSpecString := fmt.Sprintf(cladnetSpecHolder,
	// 	cladnetName, availableIPv4PrivateAddressSpaces.RecommendedIPv4PrivateAddressSpace, cladnetDescription)
	cladnetSpecByte, errMarshal := json.Marshal(reqSpec)
	cladnetSpecString := string(cladnetSpecByte)
	if errMarshal != nil {
		return model.CLADNetSpecification{}, err
	}
	common.CBLog.Printf("%#v\n", cladnetSpecString)

	// Request to create a Cloud Adaptive Network
	resp, err = client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(cladnetSpecString).
		Post(fmt.Sprintf("http://%s/v1/cladnet", networkServiceEndpoint))
	// Output print
	common.CBLog.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Printf("Could not request: %v\n", err)
		return model.CLADNetSpecification{}, err
	}

	var tempSpec model.CLADNetSpecification
	err = json.Unmarshal(resp.Body(), &tempSpec)
	if err != nil {
		return model.CLADNetSpecification{}, err
	}

	common.CBLog.Debug("End.........")
	return tempSpec, nil
}

func getAgentInstallationCommand(etcdEndpoints, cladnetId string) (string, error) {

	if etcdEndpoints == "" || cladnetId == "" {
		err := fmt.Sprintf("no enough parameters etcdEndpoints(%+v), cladnetId(%+v)", etcdEndpoints, cladnetId)
		return "", errors.New(err)
	}

	// SSH command to install cb-network agents
	placeHolderCommand := `wget https://raw.githubusercontent.com/cloud-barista/cb-larva/v0.0.15/poc-cb-net/scripts/deploy-the-released-cb-network-agent.sh -O ~/deploy-the-released-cb-network-agent.sh; chmod +x ~/deploy-the-released-cb-network-agent.sh; source ~/deploy-the-released-cb-network-agent.sh '%s' %s`
	// placeHolderCommand := `wget https://raw.githubusercontent.com/cloud-barista/cb-larva/main/poc-cb-net/scripts/1.deploy-cb-network-agent.sh -O ~/1.deploy-cb-network-agent.sh -O ~/1.deploy-cb-network-agent.sh; chmod +x ~/1.deploy-cb-network-agent.sh; source ~/1.deploy-cb-network-agent.sh '%s' %s`

	// additionalEncodedString := strings.Replace(etcdEndpoints, "\"", "\\\"", -1)
	command := fmt.Sprintf(placeHolderCommand, etcdEndpoints, cladnetId)

	return command, nil
}

// installCBNetworkAgentToMcis installs cb-network agent to VMs in an MCIS by the remote command
func installCBNetworkAgentToVM(nsId, mcisId, vmId string, mcisCmdReq McisCmdReq) (SshCmdResult, error) {
	common.CBLog.Debug("Start.........")

	vmIp, sshPort := GetVmIp(nsId, mcisId, vmId)

	// find vaild username
	userName, sshKey, err := VerifySshUserName(nsId, mcisId, vmId, vmIp, sshPort, mcisCmdReq.UserName)
	// Eventhough VerifySshUserName is not complete, Try RunRemoteCommand
	// With RunRemoteCommand, error will be checked again
	if err == nil {
		// Just logging the error (but it is net a faultal )
		common.CBLog.Info(err)
	}
	fmt.Println("")
	fmt.Println("[SSH] " + mcisId + "." + vmId + "(" + vmIp + ")" + " with userName: " + userName)
	fmt.Println("[CMD] " + mcisCmdReq.Command)
	fmt.Println("")

	result, err := RunRemoteCommand(vmIp, sshPort, userName, sshKey, mcisCmdReq.Command)

	sshResultTmp := SshCmdResult{}
	sshResultTmp.McisId = ""
	sshResultTmp.VmId = vmId
	sshResultTmp.VmIp = vmIp

	if err != nil {
		sshResultTmp.Result = ("[ERROR: " + err.Error() + "]\n " + *result)
		sshResultTmp.Err = err
	} else {
		fmt.Println("[Begin] SSH Output")
		fmt.Println(*result)
		fmt.Println("[end] SSH Output")

		sshResultTmp.Result = *result
		sshResultTmp.Err = nil
	}

	common.CBLog.Debug("End.........")
	return sshResultTmp, err
}

// // installCBNetworkAgentToMcis installs cb-network agent to VMs in an MCIS by the remote command
// func installCBNetworkAgentToMcis(nsId, mcisId string, mcisCmdReq McisCmdReq) ([]SshCmdResult, error) {
// 	common.CBLog.Debug("Start.........")

// 	sshCmdResult, err := RemoteCommandToMcis(nsId, mcisId, &mcisCmdReq)

// 	if err != nil {
// 		temp := []SshCmdResult{}
// 		common.CBLog.Error(err)
// 		return temp, err
// 	}

// 	common.CBLog.Debug("End.........")
// 	return sshCmdResult, nil
// }

// InjectCloudInformationForCloudAdaptiveNetwork injects cloud information for a cloud adaptive network
func InjectCloudInformationForCloudAdaptiveNetwork(nsId string, mcisId string, netReq *NetworkReq) (AgentInstallContentWrapper, error) {
	common.CBLog.Debug("Start.........")

	if err := common.CheckString(nsId); err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	if err := common.CheckString(mcisId); err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}

	if _, err := CheckMcis(nsId, mcisId); err != nil {
		return AgentInstallContentWrapper{}, err
	}

	serviceEndpoint := netReq.ServiceEndpoint
	// if the parameter is not passed, try to read from the environment variable
	if serviceEndpoint == "" {
		common.CBLog.Printf("read env for CB_NETWORK_SERVICE_ENDPOINT")
		// Get an endpoint of cb-network service
		serviceEndpoint = os.Getenv("CB_NETWORK_SERVICE_ENDPOINT")
		if serviceEndpoint == "" {
			return AgentInstallContentWrapper{}, errors.New("there is no CB_NETWORK_SERVICE_ENDPOINT")
		}
	}
	common.CBLog.Printf("Network service endpoint: %+v", serviceEndpoint)

	// etcdEndpoints := netReq.EtcdEndpoints
	// // if the parameter is not passed, try to read from the environment variable
	// if len(etcdEndpoints) == 0 {
	// 	common.CBLog.Printf("read env for CB_NETWORK_ETCD_ENDPOINTS")
	// 	// Get endpoints of cb-network etcd which should be accessible from the remote
	// 	etcdEndpoints = strings.Split(os.Getenv("CB_NETWORK_ETCD_ENDPOINTS"), ",")
	// 	if len(etcdEndpoints) == 0 {
	// 		return AgentInstallContentWrapper{}, errors.New("there is no CB_NETWORK_ETCD_ENDPOINTS")
	// 	}
	// }
	// common.CBLog.Printf("etcd endpoints: %+v", etcdEndpoints)

	// Get Cloud Adaptive Network
	cladnetSpec, err := getCloudAdaptiveNetwork(serviceEndpoint, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}
	common.CBLog.Printf("CLADNet spec: %#v\n", cladnetSpec)

	// Get Peers in Cloud Adaptive Network (NOTE - mcisId is equal to cladnetID)
	peers, err := getPeersInCloudAdaptiveNetwork(serviceEndpoint, cladnetSpec.CladnetID)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}
	common.CBLog.Printf("Peers: %#v\n", peers)

	// Get a list of VM ID
	vmIdList, err := ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}
	common.CBLog.Printf("VM list: %v\n", vmIdList)

	// Change the rule type of cloud adaptive network
	cladnetSpec.RuleType = ruletype.CostPrioritized
	cladnetSpec, err = updateCloudAdaptiveNetwork(serviceEndpoint, cladnetSpec)
	if err != nil {
		common.CBLog.Error(err)
		return AgentInstallContentWrapper{}, err
	}
	common.CBLog.Printf("CLADNet spec: %#v\n", cladnetSpec)

	//// Inject cloud information to each peer in the Cloud Adaptive network
	contents := AgentInstallContentWrapper{}

	for _, vmId := range vmIdList {
		vmObject, _ := GetVmObject(nsId, mcisId, vmId)
		// jsonBytes, _ := json.Marshal(vmObject)
		// doc := string(jsonBytes)
		// common.CBLog.Printf("## vmObject ==> %+v\n", doc)

		for _, peer := range peers.Peers {

			// Public IP seem to be unique currently (or when installing agent, vmId shoud be passed.)
			if peer.HostPublicIP == vmObject.PublicIP {

				// Set cloud information
				tempCloudInfo := model.CloudInformation{
					ProviderName:       vmObject.Location.CloudType,
					RegionID:           vmObject.CspViewVmDetail.Region.Region,
					AvailabilityZoneID: vmObject.CspViewVmDetail.Region.Zone,
					VirtualNetworkID:   vmObject.CspViewVmDetail.VpcIID.SystemId,
					SubnetID:           vmObject.CspViewVmDetail.SubnetIID.SystemId,
				}
				common.CBLog.Printf("## vmId: %+v\n", vmId)
				common.CBLog.Printf("## %#v\n", tempCloudInfo)

				// Update the peer
				updatedPeer, err := updateDetailsOfPeer(serviceEndpoint, cladnetSpec.CladnetID, peer.HostID, tempCloudInfo)
				if err != nil {
					common.CBLog.Error(err)
				}
				common.CBLog.Printf("The updated peer: %#v\n", updatedPeer)

				updatedPeerBytes, err := json.Marshal(updatedPeer)
				if err != nil {
					common.CBLog.Error(err)
					tempPeer := model.Peer{}
					updatedPeerBytes, _ = json.Marshal(tempPeer)
				}
				updatedPeerString := string(updatedPeerBytes)

				tempContent := AgentInstallContent{}
				tempContent.McisId = mcisId
				tempContent.VmId = vmId
				tempContent.VmIp = vmObject.PublicIP
				tempContent.Result = updatedPeerString

				contents.ResultArray = append(contents.ResultArray, tempContent)
			}
		}
	}

	common.CBLog.Debug("End.........")
	return contents, nil
}

// getPeersInCloudAdaptiveNetwork retrieves peers in a Cloud Adaptive Network
func getPeersInCloudAdaptiveNetwork(networkServiceEndpoint string, cladnetId string) (model.Peers, error) {
	common.CBLog.Debug("Start.........")

	client := resty.New()

	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetPathParams(map[string]string{
			"cladnetId": cladnetId,
		}).
		Get(fmt.Sprintf("http://%s/v1/cladnet/{cladnetId}/peer", networkServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Error(err)
		return model.Peers{}, err
	}

	var peers model.Peers

	err = json.Unmarshal(resp.Body(), &peers)
	if err != nil {
		common.CBLog.Error(err)
		return model.Peers{}, err
	}
	common.CBLog.Printf("%+v\n", peers)
	common.CBLog.Printf("Peers in a Cloud Adaptive Network: %+v", peers)

	if len(peers.Peers) == 0 {
		return model.Peers{}, errors.New("could not find any Peers")
	}

	common.CBLog.Debug("End.........")
	return peers, nil
}

// updateCloudAdaptiveNetwork updates the specification of a Cloud Adaptive Network.
func updateCloudAdaptiveNetwork(networkServiceEndpoint string, cladnetSpec model.CLADNetSpecification) (model.CLADNetSpecification, error) {
	common.CBLog.Debug("Start.........")

	jsonBytes, errMarshal := json.Marshal(cladnetSpec)
	if errMarshal != nil {
		return model.CLADNetSpecification{}, errMarshal
	}
	doc := string(jsonBytes)
	common.CBLog.Printf("CLADNetSpecification (JSON string): %v\n", doc)

	client := resty.New()
	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetPathParams(map[string]string{
			"cladnetId": cladnetSpec.CladnetID,
		}).
		SetBody(doc).
		Put(fmt.Sprintf("http://%s/v1/cladnet/{cladnetId}", networkServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Error(err)
		return model.CLADNetSpecification{}, err
	}

	var spec model.CLADNetSpecification

	err = json.Unmarshal(resp.Body(), &spec)
	if err != nil {
		common.CBLog.Error(err)
		return model.CLADNetSpecification{}, err
	}
	common.CBLog.Printf("%+v\n", spec)
	common.CBLog.Printf("The updated CLADNetSpecification: %+v", spec)

	common.CBLog.Debug("End.........")
	return spec, nil

}

// updateDetailsOfPeer updates the peers with cloud information (i.e., details).
func updateDetailsOfPeer(networkServiceEndpoint string, cladnetId string, hostId string, details model.CloudInformation) (model.Peer, error) {
	common.CBLog.Debug("Start.........")

	cloudInformationHolder := `{"cloudInformation": %s}`
	jsonBytes, errMarshal := json.Marshal(details)
	if errMarshal != nil {
		return model.Peer{}, errMarshal
	}
	doc := fmt.Sprintf(cloudInformationHolder, string(jsonBytes))

	common.CBLog.Printf("CloudInforamtion (JSON string): %v\n", doc)

	client := resty.New()
	// Request a recommendation of available IPv4 private address spaces.
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetPathParams(map[string]string{
			"cladnetId": cladnetId,
			"hostId":    hostId,
		}).
		SetBody(doc).
		Put(fmt.Sprintf("http://%s/v1/cladnet/{cladnetId}/peer/{hostId}/details", networkServiceEndpoint))
	// Output print
	log.Printf("\nError: %v\n", err)
	common.CBLog.Printf("Time: %v\n", resp.Time())
	common.CBLog.Printf("Body: %v\n", resp)

	if err != nil {
		common.CBLog.Error(err)
		return model.Peer{}, err
	}

	var peer model.Peer

	err = json.Unmarshal(resp.Body(), &peer)
	if err != nil {
		common.CBLog.Error(err)
		return model.Peer{}, err
	}
	common.CBLog.Printf("%+v\n", peer)
	common.CBLog.Printf("The updated peer: %+v", peer)

	common.CBLog.Debug("End.........")
	return peer, nil
}
