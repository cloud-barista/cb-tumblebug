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

// Package model is to handle object of CB-Tumblebug
package model

import (
	"database/sql"
	"sync"

	"gorm.io/gorm"
)

// SimpleMsg is struct for JSON Simple message
type SimpleMsg struct {
	Message string `json:"message" example:"Any message"`
}

// KeyValue is struct for key-value pair
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KeyWithEncryptedValue is struct for key-(encrypted)value pair
type KeyWithEncryptedValue struct {
	// Key for the value
	Key string `json:"key"`

	// Should be encrypted by the public key issued by GET /credential/publicKey
	Value string `json:"value"`
}

// AddItem adds a new item to the model.IdList
func (list *IdList) AddItem(id string) {
	list.mux.Lock()
	defer list.mux.Unlock()
	list.IdList = append(list.IdList, id)
}

type IdList struct {
	IdList []string `json:"output"`
	mux    sync.Mutex
}

// OptionalParameter is struct for optional parameter for function (ex. VmId)
type OptionalParameter struct {
	Value string
	Set   bool
}

// SystemReady is global variable for checking SystemReady status
var SystemReady bool

var SpiderRestUrl string
var DragonflyRestUrl string
var TerrariumRestUrl string
var DBUrl string
var DBDatabase string
var DBUser string
var DBPassword string
var AutocontrolDurationMs string
var DefaultNamespace string
var DefaultCredentialHolder string
var EtcdEndpoints string
var SelfEndpoint string
var MyDB *sql.DB
var err error

// var ORM *xorm.Engine
var ORM *gorm.DB

const (
	StrManager               string = "cb-tumblebug"
	StrSpiderRestUrl         string = "TB_SPIDER_REST_URL"
	StrDragonflyRestUrl      string = "TB_DRAGONFLY_REST_URL"
	StrTerrariumRestUrl      string = "TB_TERRARIUM_REST_URL"
	StrDBUrl                 string = "TB_POSTGRES_ENDPOINT"
	StrDBDatabase            string = "TB_POSTGRES_DATABASE"
	StrDBUser                string = "TB_POSTGRES_USER"
	StrDBPassword            string = "TB_POSTGRES_PASSWORD"
	StrAutocontrolDurationMs string = "TB_AUTOCONTROL_DURATION_MS"
	StrEtcdEndpoints         string = "TB_ETCD_ENDPOINTS"
	ErrStrKeyNotFound        string = "key not found"
	StrAdd                   string = "add"
	StrDelete                string = "delete"
	StrSSHKey                string = "sshKey"
	StrKeyPair               string = "keypair"
	StrImage                 string = "image"
	StrCustomImage           string = "customImage"
	StrSecurityGroup         string = "securityGroup"
	StrSG                    string = "sg"
	StrSpec                  string = "spec"
	StrVNet                  string = "vNet"
	StrSubnet                string = "subnet"
	StrVPC                   string = "vpc"
	StrVPN                   string = "vpn"
	StrSqlDB                 string = "sqlDb"
	StrObjectStorage         string = "objectStorage"
	StrDataDisk              string = "dataDisk"
	StrDisk                  string = "disk"
	StrNLB                   string = "nlb"
	StrVM                    string = "vm"
	StrMCI                   string = "mci"
	StrSubGroup              string = "subGroup"
	StrK8s                   string = "k8s"
	StrKubernetes            string = "kubernetes"
	StrNodeGroup             string = "nodegroup"
	StrCluster               string = "cluster"
	StrContainer             string = "container"
	StrNamespace             string = "ns"
	StrCommon                string = "common"
	StrEmpty                 string = "empty"
	StrSharedResourceName    string = "-shared-"
	// StrFirewallRule               string = "firewallRule"

	// SystemCommonNs is const for SystemCommon NameSpace ID
	SystemCommonNs string = "system"
)

var StartTime string

// Spider 2024-10-05 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/IId.go
type IID struct {
	NameId   string `json:"NameId" validate:"required" example:"user-defined-name"`
	SystemId string `json:"SystemId" validate:"required" example:"csp-defined-id"`
}

type SpiderConnectionName struct {
	ConnectionName string `json:"ConnectionName"`
}

// ResourceTypeRegistry is map for Resource type
var ResourceTypeRegistry = map[string]func() interface{}{
	StrSSHKey:        func() interface{} { return &TbSshKeyInfo{} },
	StrImage:         func() interface{} { return &TbImageInfo{} },
	StrCustomImage:   func() interface{} { return &TbCustomImageInfo{} },
	StrSecurityGroup: func() interface{} { return &TbSecurityGroupInfo{} },
	StrSpec:          func() interface{} { return &TbSpecInfo{} },
	StrVNet:          func() interface{} { return &TbVNetInfo{} },
	StrSubnet:        func() interface{} { return &TbSubnetInfo{} },
	StrDataDisk:      func() interface{} { return &TbDataDiskInfo{} },
	StrNLB:           func() interface{} { return &TbNLBInfo{} },
	StrVM:            func() interface{} { return &TbVmInfo{} },
	StrMCI:           func() interface{} { return &TbMciInfo{} },
	StrK8s:           func() interface{} { return &TbK8sClusterInfo{} },
	StrNamespace:     func() interface{} { return &NsInfo{} },
	StrVPN:           func() interface{} { return &VPNInfo{} },
}

// ResourceIds is struct for containing id and name of each Resource type
type ResourceIds struct { // Tumblebug
	CspResourceId   string
	CspResourceName string
	ConnectionName  string
}

// ConnConfig is struct for containing modified CB-Spider struct for connection config
type ConnConfig struct {
	ConfigName           string         `json:"configName"`
	ProviderName         string         `json:"providerName"`
	DriverName           string         `json:"driverName"`
	CredentialName       string         `json:"credentialName"`
	CredentialHolder     string         `json:"credentialHolder"`
	RegionZoneInfoName   string         `json:"regionZoneInfoName"`
	RegionZoneInfo       RegionZoneInfo `json:"regionZoneInfo" gorm:"type:text;serializer:json"`
	RegionDetail         RegionDetail   `json:"regionDetail" gorm:"type:text;serializer:json"`
	RegionRepresentative bool           `json:"regionRepresentative"`
	Verified             bool           `json:"verified"`
}

// SpiderConnConfig is struct for containing a CB-Spider struct for connection config
type SpiderConnConfig struct {
	ConfigName     string
	ProviderName   string
	DriverName     string
	CredentialName string
	RegionName     string
}

// CloudDriverInfo is struct for containing a CB-Spider struct for cloud driver info
type CloudDriverInfo struct {
	DriverName        string
	ProviderName      string
	DriverLibFileName string
}

// CredentialReq is struct for containing a struct for credential request
// @Description CredentialReq contains the necessary information to register a credential.
// @Description This includes the AES key encrypted with the RSA public key, which is then used to decrypt the AES key on the server side.
type CredentialReq struct {

	// ProviderName specifies the cloud provider associated with the credential (e.g., AWS, GCP).
	ProviderName string `json:"providerName" example:"aws"`

	// CredentialHolder is the entity or user that holds the credential.
	CredentialHolder string `json:"credentialHolder" example:"admin"`

	// PublicKeyTokenId is the unique token ID used to retrieve the corresponding private key for decryption.
	PublicKeyTokenId string `json:"publicKeyTokenId" example:"cr31av30uphc738d7h0g"`

	// EncryptedClientAesKeyByPublicKey is the client temporary AES key encrypted with the RSA public key.
	EncryptedClientAesKeyByPublicKey string `json:"encryptedClientAesKeyByPublicKey" example:"ZzXL27hbAUDT0ohglf2Gwr60sAqdPw3+CnCsn0RJXeiZxXnHfW03mFx5RaSfbwtPYCq1h6wwv7XsiWzfFmr02..."`

	// CredentialKeyValueList contains key-(encrypted)value pairs that include the sensitive credential data.
	CredentialKeyValueList []KeyWithEncryptedValue `json:"credentialKeyValueList"`
}

// CredentialInfo is struct for containing a struct for credential info
type CredentialInfo struct {
	CredentialName   string         `json:"credentialName"`
	CredentialHolder string         `json:"credentialHolder"`
	ProviderName     string         `json:"providerName"`
	KeyValueInfoList []KeyValue     `json:"keyValueInfoList"`
	AllConnections   ConnConfigList `json:"allConnections"`
}

// ConnConfigList is struct for containing a CB-Spider struct for connection config list
type ConnConfigList struct { // Spider
	Connectionconfig []ConnConfig `json:"connectionconfig"`
}

// SpiderRegionZoneInfo is struct for containing region struct of CB-Spider
type SpiderRegionZoneInfo struct {
	RegionName        string     // ex) "region01"
	ProviderName      string     // ex) "GCP"
	KeyValueInfoList  []KeyValue // ex) { {region, us-east1}, {zone, us-east1-c} }
	AvailableZoneList []string
}

// RegionZoneInfo is struct for containing region struct
type RegionZoneInfo struct {
	AssignedRegion string `json:"assignedRegion"`
	AssignedZone   string `json:"assignedZone"`
}

// RetrievedRegionList is array struct for Region
type RetrievedRegionList struct {
	Region []SpiderRegionZoneInfo `json:"region"`
}

// PublicKeyResponse is struct for containing the public key response
type PublicKeyResponse struct {
	PublicKeyTokenId string `json:"publicKeyTokenId"`
	PublicKey        string `json:"publicKey"`
}

// InspectResourceAllResult is struct for Inspect Resource Result for All Clouds
type InspectResourceAllResult struct {
	ElapsedTime          int                     `json:"elapsedTime"`
	RegisteredConnection int                     `json:"registeredConnection"`
	AvailableConnection  int                     `json:"availableConnection"`
	TumblebugOverview    inspectOverview         `json:"tumblebugOverview"`
	CspOnlyOverview      inspectOverview         `json:"cspOnlyOverview"`
	InspectResult        []InspectResourceResult `json:"inspectResult"`
}

// InspectResourceResult is struct for Inspect Resource Result
type InspectResourceResult struct {
	ConnectionName    string          `json:"connectionName"`
	SystemMessage     string          `json:"systemMessage"`
	ElapsedTime       int             `json:"elapsedTime"`
	TumblebugOverview inspectOverview `json:"tumblebugOverview"`
	CspOnlyOverview   inspectOverview `json:"cspOnlyOverview"`
}

type inspectOverview struct {
	VNet          int `json:"vNet"`
	SecurityGroup int `json:"securityGroup"`
	SshKey        int `json:"sshKey"`
	DataDisk      int `json:"dataDisk"`
	CustomImage   int `json:"customImage"`
	Vm            int `json:"vm"`
	NLB           int `json:"nlb"`
}

type SpiderNameIdSystemId struct {
	NameId   string
	SystemId string
}

type SpiderAllListWrapper struct {
	AllList SpiderAllList
}

type SpiderAllList struct {
	MappedList     []SpiderNameIdSystemId
	OnlySpiderList []SpiderNameIdSystemId
	OnlyCSPList    []SpiderNameIdSystemId
}

// TbInspectResourcesResponse is struct for response of InspectResources request
type TbInspectResourcesResponse struct {
	InspectResources []InspectResource `json:"inspectResources"`
}

// InspectResource is struct for InspectResource per Cloud Connection
type InspectResource struct {
	// ResourcesOnCsp       interface{} `json:"resourcesOnCsp"`
	// ResourcesOnSpider    interface{} `json:"resourcesOnSpider"`
	// ResourcesOnTumblebug interface{} `json:"resourcesOnTumblebug"`

	ConnectionName   string                `json:"connectionName"`
	ResourceType     string                `json:"resourceType"`
	SystemMessage    string                `json:"systemMessage"`
	ResourceOverview ResourceCountOverview `json:"resourceOverview"`
	Resources        ResourcesByManageType `json:"resources"`
}

// ResourceCountOverview is struct for Resource Count Overview
type ResourceCountOverview struct {
	OnTumblebug int `json:"onTumblebug"`
	OnSpider    int `json:"onSpider"`
	OnCspTotal  int `json:"onCspTotal"`
	OnCspOnly   int `json:"onCspOnly"`
}

// ResourcesByManageType is struct for Resources by Manage Type
type ResourcesByManageType struct {
	OnTumblebug ResourceOnTumblebug `json:"onTumblebug"`
	OnSpider    ResourceOnSpider    `json:"onSpider"`
	OnCspTotal  ResourceOnCsp       `json:"onCspTotal"`
	OnCspOnly   ResourceOnCsp       `json:"onCspOnly"`
}

// ResourceOnSpider is struct for Resource on Spider
type ResourceOnSpider struct {
	Count int                    `json:"count"`
	Info  []ResourceOnSpiderInfo `json:"info"`
}

// ResourceOnSpiderInfo is struct for Resource on Spider Info
type ResourceOnSpiderInfo struct {
	IdBySp        string `json:"idBySp"`
	CspResourceId string `json:"cspResourceId"`
}

// ResourceOnCsp is struct for Resource on CSP
type ResourceOnCsp struct {
	Count int                 `json:"count"`
	Info  []ResourceOnCspInfo `json:"info"`
}

// ResourceOnCspInfo is struct for Resource on CSP Info
type ResourceOnCspInfo struct {
	CspResourceId string `json:"cspResourceId"`
	RefNameOrId   string `json:"refNameOrId"`
}

// ResourceOnTumblebug is struct for Resource on Tumblebug
type ResourceOnTumblebug struct {
	Count int                       `json:"count"`
	Info  []ResourceOnTumblebugInfo `json:"info"`
}

// ResourceOnTumblebugInfo is struct for Resource on Tumblebug Info
type ResourceOnTumblebugInfo struct {
	IdByTb        string `json:"idByTb"`
	CspResourceId string `json:"cspResourceId"`
	NsId          string `json:"nsId"`
	MciId         string `json:"mciId,omitempty"`
	ObjectKey     string `json:"objectKey"`
}

// RegisterResourceAllResult is struct for Register Resource Result for All Clouds
type RegisterResourceAllResult struct {
	ElapsedTime           int                      `json:"elapsedTime"`
	RegisteredConnection  int                      `json:"registeredConnection"`
	AvailableConnection   int                      `json:"availableConnection"`
	RegisterationOverview RegisterationOverview    `json:"registerationOverview"`
	RegisterationResult   []RegisterResourceResult `json:"registerationResult"`
}

// RegisterResourceResult is struct for Register Resource Result
type RegisterResourceResult struct {
	ConnectionName        string                `json:"connectionName"`
	SystemMessage         string                `json:"systemMessage"`
	ElapsedTime           int                   `json:"elapsedTime"`
	RegisterationOverview RegisterationOverview `json:"registerationOverview"`
	RegisterationOutputs  IdList                `json:"registerationOutputs"`
}

// RegisterResource is struct for Register Resource
type RegisterationOverview struct {
	VNet          int `json:"vNet"`
	SecurityGroup int `json:"securityGroup"`
	SshKey        int `json:"sshKey"`
	DataDisk      int `json:"dataDisk"`
	CustomImage   int `json:"customImage"`
	Vm            int `json:"vm"`
	NLB           int `json:"nlb"`
	Failed        int `json:"failed"`
}
