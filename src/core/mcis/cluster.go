/*
Copyright 2023 The Cloud-Barista Authors.
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
	"fmt"
	"reflect"
	"strings"
	"time"

	cbstore_utils "github.com/cloud-barista/cb-store/utils"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/cloud-control-manager/cloud-driver/interfaces/resources/ClusterHandler.go#L1

/*
TODO: Implement Register/Unregister

// SpiderClusterRegisterReqInfoWrapper is a wrapper struct to create JSON body of 'Register Cluster request'
type SpiderClusterRegisterReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderClusterRegisterReqInfo
}

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/api-runtime/rest-runtime/ClusterRest.go#L52

// SpiderClusterRegisterReqInfo is a struct to create JSON body of 'Register Cluster request'
type SpiderClusterRegisterReqInfo struct {
	VPCName string
	Name    string
	CSPId   string
}

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/api-runtime/rest-runtime/ClusterRest.go#L86

// SpiderClusterUnregisterReqInfoWrapper is a wrapper struct to create JSON body of 'Unregister Cluster request'
type SpiderClusterUnregisterReqInfoWrapper struct {
	ConnectionName string
}
*/

/*
 * Cluster Request
 */

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/cloud-control-manager/cloud-driver/interfaces/resources/ClusterHandler.go#L1

// SpiderClusterReq is a wrapper struct to create JSON body of 'Create Cluster request'
type SpiderClusterReq struct {
	NameSpace      string // should be empty string from Tumblebug
	ConnectionName string
	ReqInfo        SpiderClusterReqInfo
}

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/api-runtime/rest-runtime/ClusterRest.go#L110

// SpiderClusterReqInfo is a struct to create JSON body of 'Create Cluster request'
type SpiderClusterReqInfo struct {
	// (1) Cluster Info
	Name    string
	Version string

	// (2) Network Info
	VPCName            string
	SubnetNames        []string
	SecurityGroupNames []string

	// (3) NodeGroupInfo List
	NodeGroupList []SpiderNodeGroupReqInfo
}

// TbClusterReq is a struct to handle 'Create cluster' request toward CB-Tumblebug.
type TbClusterReq struct { // Tumblebug
	//Namespace      string `json:"namespace" validate:"required" example:"ns01"`
	ConnectionName string `json:"connectionName" validate:"required" example:"testcloud01-seoul"`
	Description    string `json:"description"`

	// (1) Cluster Info
	Id      string `json:"id" validate:"required" example:"testcloud01-seoul-cluster"`
	Version string `json:"version" example:"1.23.4"`

	// (2) Network Info
	VNetId           string   `json:"vNetId" validate:"required"`
	SubnetIds        []string `json:"subnetIds" validate:"required"`
	SecurityGroupIds []string `json:"securityGroupIds" validate:"required"`

	// (3) NodeGroupInfo List
	NodeGroupList []TbNodeGroupReq `json:"nodeGroupList"`

	// Fields for "Register existing cluster" feature
	// CspClusterId is required to register a cluster from CSP (option=register)
	CspClusterId string `json:"cspClusterId"`
}

// 2023-11-13 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/api-runtime/rest-runtime/ClusterRest.go#L441

// SpiderNodeGroupReq is a wrapper struct to create JSON body of 'Add NodeGroup' request
type SpiderNodeGroupReq struct {
	NameSpace      string // should be empty string from Tumblebug
	ConnectionName string
	ReqInfo        SpiderNodeGroupReqInfo
}

// SpiderNodeGroupReqInfo is a wrapper struct to create JSON body of 'Add NodeGroup' request
type SpiderNodeGroupReqInfo struct {
	Name         string
	ImageName    string
	VMSpecName   string
	RootDiskType string
	RootDiskSize string
	KeyPairName  string

	// autoscale config.
	OnAutoScaling   string
	DesiredNodeSize string
	MinNodeSize     string
	MaxNodeSize     string
}

// TbNodeGroupReq is a struct to handle requests related to NodeGroup toward CB-Tumblebug.
type TbNodeGroupReq struct {
	Name         string `json:"name"`
	ImageId      string `json:"imageId"`
	SpecId       string `json:"specId"`
	RootDiskType string `json:"rootDiskType" example:"default, TYPE1, ..."`  // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize string `json:"rootDiskSize" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]
	SshKeyId     string `json:"sshKeyId"`

	// autoscale config.
	OnAutoScaling   string `json:"onAutoScaling"`
	DesiredNodeSize string `json:"desiredNodeSize"`
	MinNodeSize     string `json:"minNodeSize"`
	MaxNodeSize     string `json:"maxNodeSize"`
}

// SpiderSetAutoscalingReq is a wrapper struct to create JSON body of 'Set Autoscaling On/Off' request.
type SpiderSetAutoscalingReq struct {
	ConnectionName string
	ReqInfo        SpiderSetAutoscalingReqInfo
}

// SpiderSetAutoscalingReqInfo is a wrapper struct to create JSON body of 'Set Autoscaling On/Off' request.
type SpiderSetAutoscalingReqInfo struct {
	OnAutoScaling string
}

// TbSetAutoscalingReq is a struct to handle 'Set Autoscaling' request toward CB-Tumblebug.
type TbSetAutoscalingReq struct {
	OnAutoScaling string `json:"onAutoScaling"`
}

// SpiderChangeAutoscaleSizeReq is a wrapper struct to create JSON body of 'Change Autoscale Size' request.
type SpiderChangeAutoscaleSizeReq struct {
	ConnectionName string
	ReqInfo        SpiderChangeAutoscaleSizeReqInfo
}

// SpiderChangeAutoscaleSizeReqInfo is a wrapper struct to create JSON body of 'Change Autoscale Size' request.
type SpiderChangeAutoscaleSizeReqInfo struct {
	DesiredNodeSize string
	MinNodeSize     string
	MaxNodeSize     string
}

// TbChangeAutoscaleSizeReq is a struct to handle 'Change Autoscale Size' request toward CB-Tumblebug.
type TbChangeAutoscaleSizeReq struct {
	DesiredNodeSize string `json:"desiredNodeSize"`
	MinNodeSize     string `json:"minNodeSize"`
	MaxNodeSize     string `json:"maxNodeSize"`
}

// SpiderChangeAutoscaleSizeRes is a wrapper struct to get JSON body of 'Change Autoscale Size' response
type SpiderChangeAutoscaleSizeRes struct {
	ConnectionName string
	NodeGroupInfo  SpiderNodeGroupInfo
}

// TbChangeAutoscaleSizeRes is a struct to handle 'Change Autoscale Size' response from CB-Tumblebug.
type TbChangeAutoscaleSizeRes struct {
	TbClusterNodeGroupInfo
}

// SpiderUpgradeClusterReq is a wrapper struct to create JSON body of 'Upgrade Cluster' request
type SpiderUpgradeClusterReq struct {
	NameSpace      string // should be empty string from Tumblebug
	ConnectionName string
	ReqInfo        SpiderUpgradeClusterReqInfo
}

// SpiderUpgradeClusterReqInfo is a wrapper struct to create JSON body of 'Upgrade Cluster' request
type SpiderUpgradeClusterReqInfo struct {
	Version string
}

// TbUpgradeClusterReq is a struct to handle 'Upgrade Cluster' request toward CB-Tumblebug.
type TbUpgradeClusterReq struct {
	Version string `json:"version"`
}

// TbClusterReqStructLevelValidation is a function to validate 'TbClusterReq' object.
func TbClusterReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(TbClusterReq)

	err := common.CheckString(u.Id)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Id, "id", "Id", err.Error(), "")
	}
}

/*
 * Cluster Const
 */

// 2023-11-14 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/cloud-control-manager/cloud-driver/interfaces/resources/ClusterHandler.go#L15

type ClusterStatus string

const (
	ClusterCreating ClusterStatus = "Creating"
	ClusterActive   ClusterStatus = "Active"
	ClusterInactive ClusterStatus = "Inactive"
	ClusterUpdating ClusterStatus = "Updating"
	ClusterDeleting ClusterStatus = "Deleting"
)

type NodeGroupStatus string

const (
	NodeGroupCreating NodeGroupStatus = "Creating"
	NodeGroupActive   NodeGroupStatus = "Active"
	NodeGroupInactive NodeGroupStatus = "Inactive"
	NodeGroupUpdating NodeGroupStatus = "Updating"
	NodeGroupDeleting NodeGroupStatus = "Deleting"
)

/*
 * Cluster Info Structure
 */

// 2023-11-14 https://github.com/cloud-barista/cb-spider/blob/fa4bd91fdaa6bb853ea96eca4a7b4f58a2abebf2/cloud-control-manager/cloud-driver/interfaces/resources/ClusterHandler.go#L37

// SpiderClusterRes is a wrapper struct to handle a Cluster information from the CB-Spider's REST API response
type SpiderClusterRes struct {
	ConnectionName string
	ClusterInfo    SpiderClusterInfo
}

// SpiderClusterInfo is a struct to handle Cluster information from the CB-Spider's REST API response
type SpiderClusterInfo struct {
	IId common.IID // {NameId, SystemId}

	Version string // Kubernetes Version, ex) 1.23.3
	Network SpiderNetworkInfo

	// ---

	NodeGroupList []SpiderNodeGroupInfo
	AccessInfo    SpiderAccessInfo
	Addons        SpiderAddonsInfo

	Status ClusterStatus

	CreatedTime  time.Time
	KeyValueList []common.KeyValue
}

// TbClusterInfo is a struct that represents TB cluster object.
type TbClusterInfo struct { // Tumblebug
	Id             string `json:"id"`
	Name           string `json:"name"`
	ConnectionName string `json:"connectionName"`

	Version string `json:"version" example:"1.23.3"` // Kubernetes Version, ex) 1.23.3
	Network TbClusterNetworkInfo

	// ---

	NodeGroupList []TbClusterNodeGroupInfo
	AccessInfo    TbClusterAccessInfo
	Addons        TbClusterAddonsInfo

	Status ClusterStatus `json:"status" example:"Creating"` // Creating, Active, Inactive, Updating, Deleting

	CreatedTime  time.Time         `json:"createdTime" example:"1970-01-01T00:00:00.00Z"`
	KeyValueList []common.KeyValue `json:"keyValueList"`

	Description    string `json:"description"`
	CspClusterId   string `json:"cspClusterId"`
	CspClusterName string `json:"cspClusterName"`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// SystemLabel is for describing the MCIR in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`
}

// SpiderNetworkInfo is a struct to handle Cluster Network information from the CB-Spider's REST API response
type SpiderNetworkInfo struct {
	VpcIID            common.IID // {NameId, SystemId}
	SubnetIIDs        []common.IID
	SecurityGroupIIDs []common.IID

	// ---

	KeyValueList []common.KeyValue
}

// TbClusterNetworkInfo is a struct to handle Cluster Network information from the CB-Tumblebug's REST API response
type TbClusterNetworkInfo struct {
	VNetId           string   `json:"vNetId"`
	SubnetIds        []string `json:"subnetIds"`
	SecurityGroupIds []string `json:"securityGroupIds"`

	// ---

	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// SpiderNodeGroupInfo is a struct to handle Cluster Node Group information from the CB-Spider's REST API response
type SpiderNodeGroupInfo struct {
	IId common.IID // {NameId, SystemId}

	// VM config.
	ImageIID     common.IID
	VMSpecName   string
	RootDiskType string // "SSD(gp2)", "Premium SSD", ...
	RootDiskSize string // "", "default", "50", "1000" (GB)
	KeyPairIID   common.IID

	// Scaling config.
	OnAutoScaling   bool // default: true
	DesiredNodeSize int
	MinNodeSize     int
	MaxNodeSize     int

	// ---

	Status NodeGroupStatus
	Nodes  []common.IID

	KeyValueList []common.KeyValue
}

// TbClusterNodeGroupInfo is a struct to handle Cluster Node Group information from the CB-Tumblebug's REST API response
type TbClusterNodeGroupInfo struct {
	Id string `json:"id"`
	//Name string `json:"name"`

	// VM config.
	ImageId      string `json:"imageId"`
	SpecId       string `json:"specId"`
	RootDiskType string `json:"rootDiskType"`
	RootDiskSize string `json:"rootDiskSize"`
	SshKeyId     string `json:"sshKeyId"`

	// Scaling config.
	OnAutoScaling   bool `json:"onAutoScaling"`
	DesiredNodeSize int  `json:"desiredNodeSize"`
	MinNodeSize     int  `json:"minNodeSize"`
	MaxNodeSize     int  `json:"maxNodeSize"`

	// ---
	Status NodeGroupStatus `json:"status" example:"Creating"` // Creating, Active, Inactive, Updating, Deleting
	Nodes  []string        `json:"nodes"`                     // id for nodes

	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// SpiderAccessInfo is a struct to handle Cluster Access information from the CB-Spider's REST API response
type SpiderAccessInfo struct {
	Endpoint   string // ex) https://1.2.3.4:6443
	Kubeconfig string
}

// TbClusterAccessInfo is a struct to handle Cluster Access information from the CB-Tumblebug's REST API response
type TbClusterAccessInfo struct {
	Endpoint   string `json:"endpoint" example:"http://1.2.3.4:6443"`
	Kubeconfig string `json:"kubeconfig"`
}

// SpiderAddonsInfo is a struct to handle Cluster Addons information from the CB-Spider's REST API response
type SpiderAddonsInfo struct {
	KeyValueList []common.KeyValue
}

// TbClusterAddonsInfo is a struct to handle Cluster Addons information from the CB-Tumblebug's REST API response
type TbClusterAddonsInfo struct {
	KeyValueList []common.KeyValue `json:"keyValueList"`
}

// CreateCluster create a cluster
func CreateCluster(nsId string, u *TbClusterReq, option string) (TbClusterInfo, error) {
	log.Info().Msg("CreateCluster")

	emptyObj := TbClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(u.Id)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckCluster(nsId, u.Id)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if check {
		err := fmt.Errorf("The cluster " + u.Id + " already exists.")
		return emptyObj, err
	}

	/*
	 * Check for Cluster Enablement from ClusterSetting
	 */

	connConfig, err := common.GetConnConfig(u.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get the connConfig " + u.ConnectionName + ": " + err.Error())
		return emptyObj, err
	}

	cloudType := connConfig.ProviderName

	// Convert cloud type to field name (e.g., AWS to Aws, OPENSTACK to Openstack)
	lowercase := strings.ToLower(cloudType)
	fnCloudType := strings.ToUpper(string(lowercase[0])) + lowercase[1:]

	// Get cloud setting with field name
	cloudSetting := common.CloudSetting{}

	getCloudSetting := func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("%v", err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(common.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fnCloudType).Interface().(common.CloudSetting)
	}

	getCloudSetting()

	if cloudSetting.Cluster.Enable != "y" {
		err := fmt.Errorf("The Cluster Management function is not enabled for Cloud(" + fnCloudType + ")")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for SpiderClusterReq{}
	 */

	spName := fmt.Sprintf("%s-%s", nsId, u.Id)
	spVersion := u.Version

	spVPCName, err := common.GetCspResourceId(nsId, common.StrVNet, u.VNetId)
	if spVPCName == "" {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
		var spSubnetNames []string
		for _, v := range u.SubnetIds {
			spSnName, err := common.GetCspResourceId(nsId, common.StrSubnet, v)
			if spSnName == "" {
				log.Error().Err(err).Msg("")
				return emptyObj, err
			}

			spSubnetNames = append(spSubnetNames, spSnName)
		}
	*/

	var spSnName string
	var spSubnetNames []string
	var found bool

	tmpInf, err := mcir.GetResource(nsId, common.StrVNet, u.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}
	tbVNetInfo := mcir.TbVNetInfo{}
	err = common.CopySrcToDest(&tmpInf, &tbVNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	for _, v := range u.SubnetIds {
		found = false
		for _, w := range tbVNetInfo.SubnetInfoList {
			if v == w.Name {
				spSnName = w.Name
				found = true
				break
			}
		}

		if found == true {
			spSubnetNames = append(spSubnetNames, spSnName)
		}
	}
	if len(spSubnetNames) == 0 {
		err := fmt.Errorf("No valid subnets in VNetId(%s)", u.VNetId)
		return emptyObj, err
	}

	var spSecurityGroupNames []string
	for _, v := range u.SecurityGroupIds {
		spSgName, err := common.GetCspResourceId(nsId, common.StrSecurityGroup, v)
		if spSgName == "" {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		spSecurityGroupNames = append(spSecurityGroupNames, spSgName)
	}

	var spNodeGroupList []SpiderNodeGroupReqInfo
	for _, v := range u.NodeGroupList {
		err := common.CheckString(v.Name)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		spImgName := "" // Some CSPs do not require ImageName for creating a cluster
		if v.ImageId != "" {
			spImgName, err = common.GetCspResourceId(nsId, common.StrImage, v.ImageId)
			if spImgName == "" {
				log.Error().Err(err).Msg("")
				return emptyObj, err
			}
		}

		spSpecName, err := common.GetCspResourceId(nsId, common.StrSpec, v.SpecId)
		if spSpecName == "" {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		spKpName, err := common.GetCspResourceId(nsId, common.StrSSHKey, v.SshKeyId)
		if spKpName == "" {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		spNodeGroupList = append(spNodeGroupList, SpiderNodeGroupReqInfo{
			Name:            v.Name,
			ImageName:       spImgName,
			VMSpecName:      spSpecName,
			RootDiskType:    v.RootDiskType,
			RootDiskSize:    v.RootDiskSize,
			KeyPairName:     spKpName,
			OnAutoScaling:   v.OnAutoScaling,
			DesiredNodeSize: v.DesiredNodeSize,
			MinNodeSize:     v.MinNodeSize,
			MaxNodeSize:     v.MaxNodeSize,
		})
	}

	requestBody := SpiderClusterReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: u.ConnectionName,
		ReqInfo: SpiderClusterReqInfo{
			Name:               spName,
			Version:            spVersion,
			VPCName:            spVPCName,
			SubnetNames:        spSubnetNames,
			SecurityGroupNames: spSecurityGroupNames,
			NodeGroupList:      spNodeGroupList,
		},
	}

	// Randomly sleep within 20 Secs to avoid rateLimit from CSP
	//common.RandomSleep(0, 20)
	client := resty.New()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := common.SpiderRestUrl

	if option == "register" {
		url = url + "/regcluster"
	} else { // option != "register"
		url = url + "/cluster"
	}

	var spClusterRes SpiderClusterRes

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build TbClusterInfo object
	 */

	tbCInfo := convertSpiderClusterInfoToTbClusterInfo(&spClusterRes.ClusterInfo, u.Id, u.ConnectionName, u.Description)

	if option == "register" && u.CspClusterId == "" {
		tbCInfo.SystemLabel = "Registered from CB-Spider resource"
		// TODO: check to handle something to register
	} else if option == "register" && u.CspClusterId != "" {
		tbCInfo.SystemLabel = "Registered from CSP resource"
	}

	/*
	 * Put/Get TbClusterInfo to/from cb-store
	 */
	k := GenClusterKey(nsId, tbCInfo.Id)
	Val, _ := json.Marshal(tbCInfo)

	err = common.CBStore.Put(k, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return tbCInfo, err
	}

	kv, err := common.CBStore.Get(k)
	if err != nil {
		err = fmt.Errorf("In CreateCluster(); CBStore.Get() returned an error: " + err.Error())
		log.Error().Err(err).Msg("")
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	return storedTbCInfo, nil
}

// AddNodeGroup adds a NodeGroup
func AddNodeGroup(nsId string, clusterId string, u *TbNodeGroupReq) (TbClusterInfo, error) {
	log.Info().Msg("AddNodeGroup")

	emptyObj := TbClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckCluster(nsId, clusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The cluster " + clusterId + " does not exist.")
		return emptyObj, err
	}

	/*
	 * Get TbClusterInfo from cb-store
	 */
	oldTbCInfo := TbClusterInfo{}
	k := GenClusterKey(nsId, clusterId)
	kv, err := common.CBStore.Get(k)
	if err != nil {
		err = fmt.Errorf("In AddNodeGroup(); CBStore.Get() returned an error: " + err.Error())
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	err = json.Unmarshal([]byte(kv.Value), &oldTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Check for Cluster Enablement from ClusterSetting
	 */

	connConfig, err := common.GetConnConfig(oldTbCInfo.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get the connConfig " + oldTbCInfo.ConnectionName + ": " + err.Error())
		return emptyObj, err
	}

	cloudType := connConfig.ProviderName

	// Convert cloud type to field name (e.g., AWS to Aws, OPENSTACK to Openstack)
	lowercase := strings.ToLower(cloudType)
	fnCloudType := strings.ToUpper(string(lowercase[0])) + lowercase[1:]

	// Get cloud setting with field name
	cloudSetting := common.CloudSetting{}

	getCloudSetting := func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("%v", err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(common.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fnCloudType).Interface().(common.CloudSetting)
	}

	getCloudSetting()

	if cloudSetting.Cluster.Enable != "y" {
		err := fmt.Errorf("The Cluster Management function is not enabled for Cloud(" + fnCloudType + ")")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for SpiderNodeGroupReq{}
	 */

	spName := u.Name
	err = common.CheckString(spName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	spImgName := "" // Some CSPs do not require ImageName for creating a cluster
	if u.ImageId != "" {
		spImgName, err = common.GetCspResourceId(nsId, common.StrImage, u.ImageId)
		if spImgName == "" {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	}

	spSpecName, err := common.GetCspResourceId(nsId, common.StrSpec, u.SpecId)
	if spSpecName == "" {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	spKpName, err := common.GetCspResourceId(nsId, common.StrSSHKey, u.SshKeyId)
	if spKpName == "" {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	requestBody := SpiderNodeGroupReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: oldTbCInfo.ConnectionName,
		ReqInfo: SpiderNodeGroupReqInfo{
			Name:         spName,
			ImageName:    spImgName,
			VMSpecName:   spSpecName,
			RootDiskType: u.RootDiskType,
			RootDiskSize: u.RootDiskSize,
			KeyPairName:  spKpName,

			// autoscale config.
			OnAutoScaling:   u.OnAutoScaling,
			DesiredNodeSize: u.DesiredNodeSize,
			MinNodeSize:     u.MinNodeSize,
			MaxNodeSize:     u.MaxNodeSize,
		},
	}

	client := resty.New()
	method := "POST"
	client.SetTimeout(20 * time.Minute)

	url := common.SpiderRestUrl + "/cluster/" + oldTbCInfo.CspClusterName + "/nodegroup"

	var spClusterRes SpiderClusterRes

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build TbClusterInfo object
	 */

	newTbCInfo := convertSpiderClusterInfoToTbClusterInfo(&spClusterRes.ClusterInfo, oldTbCInfo.Id, oldTbCInfo.ConnectionName, oldTbCInfo.Description)

	/*
	 * Put/Get TbClusterInfo to/from cb-store
	 */
	k = GenClusterKey(nsId, newTbCInfo.Id)
	Val, _ := json.Marshal(newTbCInfo)

	err = common.CBStore.Put(k, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return newTbCInfo, err
	}

	kv, err = common.CBStore.Get(k)
	if err != nil {
		err = fmt.Errorf("In AddNodeGroup(); CBStore.Get() returned an error: " + err.Error())
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	return storedTbCInfo, nil

}

// RemoveNodeGroup removes a specified NodeGroup
func RemoveNodeGroup(nsId string, clusterId string, nodeGroupName string, forceFlag string) (bool, error) {
	log.Info().Msg("RemoveNodeGroup")
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckCluster(nsId, clusterId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	if !check {
		err := fmt.Errorf("The Cluster " + clusterId + " does not exist.")
		return false, err
	}

	k := GenClusterKey(nsId, clusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := common.CBStore.Get(k)

	// Create Req body
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	tbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	requestBody.NameSpace = "" // should be empty string from Tumblebug
	requestBody.ConnectionName = tbCInfo.ConnectionName

	client := resty.New()
	url := common.SpiderRestUrl + "/cluster/" + tbCInfo.CspClusterName + "/nodegroup/" + nodeGroupName
	if forceFlag == "true" {
		url += "?force=true"
	}
	method := "DELETE"

	var ifRes interface{}
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				return true, nil
			}
		}
	}

	return false, nil
}

// SetAutoscaling set NodeGroup's Autoscaling On/Off
func SetAutoscaling(nsId string, clusterId string, nodeGroupName string, u *TbSetAutoscalingReq) (bool, error) {
	log.Info().Msg("SetAutoscaling")
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckCluster(nsId, clusterId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	if !check {
		err := fmt.Errorf("The Cluster " + clusterId + " does not exist.")
		return false, err
	}

	err = common.CheckString(nodeGroupName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	/*
	 * Get TbClusterInfo object from cb-store
	 */

	k := GenClusterKey(nsId, clusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := common.CBStore.Get(k)

	tbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	requestBody := SpiderSetAutoscalingReq{
		ConnectionName: tbCInfo.ConnectionName,
		ReqInfo: SpiderSetAutoscalingReqInfo{
			OnAutoScaling: u.OnAutoScaling,
		},
	}

	client := resty.New()
	url := common.SpiderRestUrl + "/cluster/" + tbCInfo.CspClusterName + "/nodegroup/" + nodeGroupName + "/onautoscaling"
	method := "PUT"

	var ifRes interface{}
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	return true, nil
}

// ChangeAutoscaleSize change NodeGroup's Autoscaling Size
func ChangeAutoscaleSize(nsId string, clusterId string, nodeGroupName string, u *TbChangeAutoscaleSizeReq) (TbChangeAutoscaleSizeRes, error) {
	log.Info().Msg("ChangeAutoscaleSize")

	emptyObj := TbChangeAutoscaleSizeRes{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckCluster(nsId, clusterId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The Cluster " + clusterId + " does not exist.")
		return emptyObj, err
	}

	err = common.CheckString(nodeGroupName)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Get TbClusterInfo object from cb-store
	 */

	k := GenClusterKey(nsId, clusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := common.CBStore.Get(k)

	tbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	requestBody := SpiderChangeAutoscaleSizeReq{
		ConnectionName: tbCInfo.ConnectionName,
		ReqInfo: SpiderChangeAutoscaleSizeReqInfo{
			DesiredNodeSize: u.DesiredNodeSize,
			MinNodeSize:     u.MinNodeSize,
			MaxNodeSize:     u.MaxNodeSize,
		},
	}

	client := resty.New()
	url := common.SpiderRestUrl + "/cluster/" + tbCInfo.CspClusterName + "/nodegroup/" + nodeGroupName + "/autoscalesize"
	method := "PUT"

	var spChangeAutoscaleSizeRes SpiderChangeAutoscaleSizeRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spChangeAutoscaleSizeRes,
		common.VeryShortDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	var tbCAutoscaleSizeRes TbChangeAutoscaleSizeRes
	tbCAutoscaleSizeRes.TbClusterNodeGroupInfo = convertSpiderNodeGroupInfoToTbClusterNodeGroupInfo(&spChangeAutoscaleSizeRes.NodeGroupInfo)

	return tbCAutoscaleSizeRes, nil
}

// GetCluster retrives a cluster information
func GetCluster(nsId string, clusterId string) (TbClusterInfo, error) {

	emptyObj := TbClusterInfo{}
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyObj, err
		}
	*/
	check, err := CheckCluster(nsId, clusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The cluster " + clusterId + " does not exist.")
		return emptyObj, err
	}

	log.Debug().Msg("[Get Cluster] " + clusterId)

	/*
	 * Get TbClusterInfo object from cb-store
	 */
	k := GenClusterKey(nsId, clusterId)

	kv, err := common.CBStore.Get(k)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	storedTbCInfo := TbClusterInfo{}
	if kv == nil {
		err = fmt.Errorf("Cannot get the cluster " + clusterId + ".")
		return storedTbCInfo, err
	}

	err = json.Unmarshal([]byte(kv.Value), &storedTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return storedTbCInfo, err
	}

	/*
	 * Get TbClusterInfo object from CB-Spider
	 */

	client := resty.New()
	client.SetTimeout(10 * time.Minute)
	url := common.SpiderRestUrl + "/cluster/" + nsId + "-" + clusterId
	method := "GET"

	// Create Request body for GetCluster of CB-Spider
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: storedTbCInfo.ConnectionName,
	}

	var spClusterRes SpiderClusterRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	tbCInfo := convertSpiderClusterInfoToTbClusterInfo(&spClusterRes.ClusterInfo, clusterId, storedTbCInfo.ConnectionName, storedTbCInfo.Description)

	/*
	 * FIXME: Do not compare, just store?
	 * Compare tbCInfo with storedTbCInfo
	 */
	if !isEqualTbClusterInfoExceptStatus(storedTbCInfo, tbCInfo) {
		err := fmt.Errorf("The cluster " + clusterId + " has been changed something.")
		return emptyObj, err
	}

	return tbCInfo, nil
}

func isEqualTbClusterInfoExceptStatus(info1 TbClusterInfo, info2 TbClusterInfo) bool {

	// FIX: now compare some fields only

	if info1.Id != info2.Id ||
		info1.Name != info2.Name ||
		info1.ConnectionName != info2.ConnectionName ||
		info1.Description != info2.Description ||
		info1.CspClusterId != info2.CspClusterId ||
		info1.CspClusterName != info2.CspClusterName ||
		info1.CreatedTime != info2.CreatedTime {
		return false

	}

	return true
}

// CheckCluster returns the existence of the TB Cluster object in bool form.
func CheckCluster(nsId string, clusterId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckCluster failed; nsId given is empty.")
		return false, err
	} else if clusterId == "" {
		err := fmt.Errorf("CheckCluster failed; clusterId given is empty.")
		return false, err
	}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	err = common.CheckString(clusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	log.Debug().Msg("[Check Cluster] " + clusterId)

	key := GenClusterKey(nsId, clusterId)

	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil
}

// GenClusterKey is func to generate a key from Cluster id
func GenClusterKey(nsId string, clusterId string) string {
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	err = common.CheckString(clusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "/invalidKey"
	}

	return fmt.Sprintf("/ns/%s/cluster/%s", nsId, clusterId)
}

// ListClusterId returns the list of TB Cluster object IDs of given nsId
func ListClusterId(nsId string) ([]string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[ListClusterId] ns: " + nsId)
	// key := "/ns/" + nsId + "/"
	k := fmt.Sprintf("/ns/%s/", nsId)
	log.Debug().Msg(k)

	kv, err := common.CBStore.GetList(k, true)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	/* if keyValue == nil, then for-loop below will not be executed, and the empty array will be returned in `resourceList` placeholder.
	if keyValue == nil {
		err = fmt.Errorf("ListResourceId(); %s is empty.", key)
		log.Error().Err(err).Msg("")
		return nil, err
	}
	*/

	var clusterIds []string
	for _, v := range kv {
		trimmed := strings.TrimPrefix(v.Key, (k + "cluster/"))
		// prevent malformed key (if key for cluster ID includes '/', the key does not represent cluster ID)
		if !strings.Contains(trimmed, "/") {
			clusterIds = append(clusterIds, trimmed)
		}
	}

	return clusterIds, nil
}

// ListCluster returns the list of TB Cluster objects of given nsId
func ListCluster(nsId string, filterKey string, filterVal string) (interface{}, error) {
	log.Info().Msg("ListCluster")

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	log.Debug().Msg("[Get] Cluster list")
	k := fmt.Sprintf("/ns/%s/cluster", nsId)
	log.Debug().Msg(k)

	/*
	 * Get TbClusterInfo objects from cb-store
	 */

	kv, err := common.CBStore.GetList(k, true)
	kv = cbstore_utils.GetChildList(kv, k)

	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}

	tbCInfoList := []TbClusterInfo{}

	if kv != nil {
		for _, v := range kv {
			tbCInfo := TbClusterInfo{}
			err = json.Unmarshal([]byte(v.Value), &tbCInfo)
			if err != nil {
				log.Error().Err(err).Msg("")
				return nil, err
			}
			// Check the JSON body includes both filterKey and filterVal strings. (assume key and value)
			if filterKey != "" {
				// If not includes both, do not append current item to the list result.
				itemValueForCompare := strings.ToLower(v.Value)
				if !(strings.Contains(itemValueForCompare, strings.ToLower(filterKey)) &&
					strings.Contains(itemValueForCompare, strings.ToLower(filterVal))) {
					continue
				}
			}
			tbCInfoList = append(tbCInfoList, tbCInfo)
		}
	}

	return tbCInfoList, nil
}

// DeleteCluster deletes a cluster
func DeleteCluster(nsId string, clusterId string, forceFlag string) (bool, error) {
	log.Info().Msg("DeleteCluster")
	/*
		err := common.CheckString(nsId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

		err = common.CheckString(clusterId)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	*/
	check, err := CheckCluster(nsId, clusterId)

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	if !check {
		err := fmt.Errorf("The Cluster " + clusterId + " does not exist.")
		return false, err
	}

	/*
	 * Get TbClusterInfo object from cb-store
	 */

	k := GenClusterKey(nsId, clusterId)
	log.Debug().Msg("key: " + k)

	kv, _ := common.CBStore.Get(k)

	// Create Req body
	type JsonTemplate struct {
		NameSpace      string
		ConnectionName string
	}
	requestBody := JsonTemplate{}

	tbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &tbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	requestBody.NameSpace = "" // should be empty string from Tumblebug
	requestBody.ConnectionName = tbCInfo.ConnectionName

	client := resty.New()
	url := common.SpiderRestUrl + "/cluster/" + tbCInfo.CspClusterName
	if forceFlag == "true" {
		url += "?force=true"
	}
	method := "DELETE"

	var ifRes interface{}
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&ifRes,
		common.VeryShortDuration,
	)

	if forceFlag == "true" {
		err = common.CBStore.Delete(k)
		if err != nil {
			log.Error().Err(err).Msg("")
			return false, err
		}
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return false, err
	}

	if ifRes != nil {
		if mapRes, ok := ifRes.(map[string]interface{}); ok {
			result := mapRes["Result"]
			if result == "true" {
				if forceFlag != "true" {
					err = common.CBStore.Delete(k)
					if err != nil {
						log.Error().Err(err).Msg("")
						return false, err
					}
				}

				return true, nil
			}
		}
	}

	return false, nil
}

// DeleteAllCluster deletes all clusters
func DeleteAllCluster(nsId string, subString string, forceFlag string) (common.IdList, error) {
	log.Info().Msg("DeleteAllCluster")

	deletedClusters := common.IdList{}

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return deletedClusters, err
	}

	clusterIdList, err := ListClusterId(nsId)
	if err != nil {
		return deletedClusters, err
	}

	for _, v := range clusterIdList {
		// if subString is provided, check the clusterId contains the subString.
		if subString == "" || strings.Contains(v, subString) {
			deleteStatus := ""

			res, err := DeleteCluster(nsId, v, forceFlag)

			if err != nil {
				deleteStatus = err.Error()
			} else {
				deleteStatus = " [" + fmt.Sprintf("%t", res) + "]"
			}

			deletedClusters.IdList = append(deletedClusters.IdList, "Cluster: "+v+deleteStatus)
		}
	}
	return deletedClusters, nil
}

// UpgradeCluster upgrades an existing cluster to the specified version
func UpgradeCluster(nsId string, clusterId string, u *TbUpgradeClusterReq) (TbClusterInfo, error) {
	log.Info().Msg("UpgradeCluster")

	emptyObj := TbClusterInfo{}

	err := validate.Struct(u)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Err(err).Msg("")
			return emptyObj, err
		}

		return emptyObj, err
	}

	check, err := CheckCluster(nsId, clusterId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	if !check {
		err := fmt.Errorf("The cluster " + clusterId + " does not exist.")
		return emptyObj, err
	}

	/*
	 * Get TbClusterInfo from cb-store
	 */
	oldTbCInfo := TbClusterInfo{}
	k := GenClusterKey(nsId, clusterId)
	kv, err := common.CBStore.Get(k)
	if err != nil {
		err = fmt.Errorf("In UpgradeCluster(); CBStore.Get() returned an error: " + err.Error())
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	err = json.Unmarshal([]byte(kv.Value), &oldTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Check for Cluster Enablement from ClusterSetting
	 */

	connConfig, err := common.GetConnConfig(oldTbCInfo.ConnectionName)
	if err != nil {
		err := fmt.Errorf("Failed to get the connConfig " + oldTbCInfo.ConnectionName + ": " + err.Error())
		return emptyObj, err
	}

	cloudType := connConfig.ProviderName

	// Convert cloud type to field name (e.g., AWS to Aws, OPENSTACK to Openstack)
	lowercase := strings.ToLower(cloudType)
	fnCloudType := strings.ToUpper(string(lowercase[0])) + lowercase[1:]

	// Get cloud setting with field name
	cloudSetting := common.CloudSetting{}

	getCloudSetting := func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error().Msgf("%v", err)
				cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName("Common").Interface().(common.CloudSetting)
			}
		}()

		cloudSetting = reflect.ValueOf(&common.RuntimeConf.Cloud).Elem().FieldByName(fnCloudType).Interface().(common.CloudSetting)
	}

	getCloudSetting()

	if cloudSetting.Cluster.Enable != "y" {
		err := fmt.Errorf("The Cluster Management function is not enabled for Cloud(" + fnCloudType + ")")
		return emptyObj, err
	}

	/*
	 * Build RequestBody for SpiderUpgradeClusterReq{}
	 */
	requestBody := SpiderUpgradeClusterReq{
		NameSpace:      "", // should be empty string from Tumblebug
		ConnectionName: oldTbCInfo.ConnectionName,
		ReqInfo: SpiderUpgradeClusterReqInfo{
			Version: u.Version,
		},
	}

	client := resty.New()
	url := common.SpiderRestUrl + "/cluster/" + oldTbCInfo.CspClusterName + "/upgrade"
	method := "PUT"

	var spClusterRes SpiderClusterRes
	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(requestBody),
		&requestBody,
		&spClusterRes,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	/*
	 * Extract SpiderClusterInfo from Response & Build TbClusterInfo object
	 */

	newTbCInfo := convertSpiderClusterInfoToTbClusterInfo(&spClusterRes.ClusterInfo, oldTbCInfo.Id, oldTbCInfo.ConnectionName, oldTbCInfo.Description)

	/*
	 * Put/Get TbClusterInfo to/from cb-store
	 */
	k = GenClusterKey(nsId, newTbCInfo.Id)
	Val, _ := json.Marshal(newTbCInfo)

	err = common.CBStore.Put(k, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyObj, err
	}

	kv, err = common.CBStore.Get(k)
	if err != nil {
		err = fmt.Errorf("In UpgradeCluster(); CBStore.Get() returned an error: " + err.Error())
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<" + kv.Key + "> \n" + kv.Value)

	storedTbCInfo := TbClusterInfo{}
	err = json.Unmarshal([]byte(kv.Value), &storedTbCInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	return storedTbCInfo, nil
}

func convertSpiderNetworkInfoToTbClusterNetworkInfo(spNetworkInfo SpiderNetworkInfo) TbClusterNetworkInfo {
	tbVNetId := spNetworkInfo.VpcIID.SystemId

	var tbSubnetIds []string
	for _, v := range spNetworkInfo.SubnetIIDs {
		tbSubnetIds = append(tbSubnetIds, v.SystemId)
	}

	var tbSecurityGroupIds []string
	for _, v := range spNetworkInfo.SecurityGroupIIDs {
		tbSecurityGroupIds = append(tbSecurityGroupIds, v.SystemId)
	}

	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spNetworkInfo.KeyValueList)

	tbClusterNetworkInfo := TbClusterNetworkInfo{
		VNetId:           tbVNetId,
		SubnetIds:        tbSubnetIds,
		SecurityGroupIds: tbSecurityGroupIds,
		KeyValueList:     tbKeyValueList,
	}

	return tbClusterNetworkInfo
}

func convertSpiderNodeGroupInfoToTbClusterNodeGroupInfo(spNodeGroupInfo *SpiderNodeGroupInfo) TbClusterNodeGroupInfo {
	tbNodeId := spNodeGroupInfo.IId.SystemId
	tbImageId := spNodeGroupInfo.ImageIID.SystemId
	tbSpecId := spNodeGroupInfo.VMSpecName
	tbRootDiskType := spNodeGroupInfo.RootDiskType
	tbRootDiskSize := spNodeGroupInfo.RootDiskSize
	tbSshKeyId := spNodeGroupInfo.KeyPairIID.SystemId
	tbOnAutoScaling := spNodeGroupInfo.OnAutoScaling
	tbDesiredNodeSize := spNodeGroupInfo.DesiredNodeSize
	tbMinNodeSize := spNodeGroupInfo.MinNodeSize
	tbMaxNodeSize := spNodeGroupInfo.MaxNodeSize
	tbStatus := spNodeGroupInfo.Status

	var tbNodes []string
	for _, v := range spNodeGroupInfo.Nodes {
		tbNodes = append(tbNodes, v.SystemId)
	}

	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spNodeGroupInfo.KeyValueList)
	tbClusterNodeGroupInfo := TbClusterNodeGroupInfo{
		Id:              tbNodeId,
		ImageId:         tbImageId,
		SpecId:          tbSpecId,
		RootDiskType:    tbRootDiskType,
		RootDiskSize:    tbRootDiskSize,
		SshKeyId:        tbSshKeyId,
		OnAutoScaling:   tbOnAutoScaling,
		DesiredNodeSize: tbDesiredNodeSize,
		MinNodeSize:     tbMinNodeSize,
		MaxNodeSize:     tbMaxNodeSize,
		Status:          tbStatus,
		Nodes:           tbNodes,
		KeyValueList:    tbKeyValueList,
	}

	return tbClusterNodeGroupInfo
}

func convertSpiderNodeGroupListToTbClusterNodeGroupList(spNodeGroupList []SpiderNodeGroupInfo) []TbClusterNodeGroupInfo {
	var tbClusterNodeGroupList []TbClusterNodeGroupInfo
	for _, v := range spNodeGroupList {
		tbClusterNodeGroupInfo := convertSpiderNodeGroupInfoToTbClusterNodeGroupInfo(&v)
		tbClusterNodeGroupList = append(tbClusterNodeGroupList, tbClusterNodeGroupInfo)
	}

	return tbClusterNodeGroupList
}

func convertSpiderClusterAccessInfoToTbClusterAccessInfo(spAccessInfo SpiderAccessInfo) TbClusterAccessInfo {
	return TbClusterAccessInfo{spAccessInfo.Endpoint, spAccessInfo.Kubeconfig}
}

func convertSpiderClusterAddonsInfoToTbClusterAddonsInfo(spAddonsInfo SpiderAddonsInfo) TbClusterAddonsInfo {
	tbKeyValueList := convertSpiderKeyValueListToTbKeyValueList(spAddonsInfo.KeyValueList)
	return TbClusterAddonsInfo{tbKeyValueList}
}

func convertSpiderKeyValueListToTbKeyValueList(spKeyValueList []common.KeyValue) []common.KeyValue {
	var tbKeyValueList []common.KeyValue
	for _, v := range spKeyValueList {
		tbKeyValueList = append(tbKeyValueList, v)
	}
	return tbKeyValueList
}

func convertSpiderClusterInfoToTbClusterInfo(spClusterInfo *SpiderClusterInfo, id string, connectionName string, description string) TbClusterInfo {
	tbCNInfo := convertSpiderNetworkInfoToTbClusterNetworkInfo(spClusterInfo.Network)
	tbNGList := convertSpiderNodeGroupListToTbClusterNodeGroupList(spClusterInfo.NodeGroupList)
	tbCAccInfo := convertSpiderClusterAccessInfoToTbClusterAccessInfo(spClusterInfo.AccessInfo)
	tbCAddInfo := convertSpiderClusterAddonsInfoToTbClusterAddonsInfo(spClusterInfo.Addons)
	//tbCStatus := spClusterInfo.Status
	tbKVList := convertSpiderKeyValueListToTbKeyValueList(spClusterInfo.KeyValueList)
	tbCInfo := TbClusterInfo{
		Id:             id,
		Name:           id,
		ConnectionName: connectionName,
		Version:        spClusterInfo.Version,
		Network:        tbCNInfo,
		NodeGroupList:  tbNGList,
		AccessInfo:     tbCAccInfo,
		Addons:         tbCAddInfo,
		Status:         spClusterInfo.Status,
		CreatedTime:    spClusterInfo.CreatedTime,
		KeyValueList:   tbKVList,
		Description:    description,
		CspClusterId:   spClusterInfo.IId.SystemId,
		CspClusterName: spClusterInfo.IId.NameId,
	}

	return tbCInfo
}
