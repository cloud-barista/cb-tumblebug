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
	"time"

	"gorm.io/gorm"
)

// SimpleMsg is struct for JSON Simple message
type SimpleMsg struct {
	Message string `json:"message" example:"Any message"`
}

// ReadyzResponse is struct for readyz API response
type ReadyzResponse struct {
	Message     string `json:"message" example:"CB-Tumblebug is ready"`
	Ready       bool   `json:"ready" example:"true"`
	Initialized bool   `json:"initialized" example:"false"`
}

// ProviderAssetSummary is a provider-level summary of fetched assets.
type ProviderAssetSummary struct {
	ProviderName      string `json:"providerName" example:"aws"`
	SpecCount         int64  `json:"specCount" example:"11234"`
	PricedSpecCount   int64  `json:"pricedSpecCount" example:"11000"`
	UnpricedSpecCount int64  `json:"unpricedSpecCount" example:"234"`
	ImageCount        int64  `json:"imageCount" example:"5234"`
}

// AssetsSummaryResponse is a namespace-level summary of specs and images.
type AssetsSummaryResponse struct {
	NamespaceID       string                 `json:"namespaceId" example:"system"`
	TotalSpecCount    int64                  `json:"totalSpecCount" example:"45000"`
	PricedSpecCount   int64                  `json:"pricedSpecCount" example:"43000"`
	UnpricedSpecCount int64                  `json:"unpricedSpecCount" example:"2000"`
	TotalImageCount   int64                  `json:"totalImageCount" example:"18000"`
	Providers         []ProviderAssetSummary `json:"providers"`
}

// KeyValue is struct for key-value pair
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ScheduleJobRequest is struct for creating a scheduled job
type ScheduleJobRequest struct {
	JobType         string `json:"jobType" validate:"required" example:"registerCspResources"` // Job type: registerCspResources, registerCspResourcesAll
	NsId            string `json:"nsId" validate:"required" example:"default"`                 // Namespace ID
	IntervalSeconds int    `json:"intervalSeconds" validate:"required,min=10" example:"60"`    // Execution interval in seconds

	// Job-specific parameters (for registerCspResources)
	ConnectionName  string `json:"connectionName,omitempty" example:"aws-ap-northeast-2"` // (Deprecated) Connection configuration name. Use Provider/Region/Zone instead
	Provider        string `json:"provider,omitempty" example:"aws"`                      // Cloud provider name. Empty: all providers
	Region          string `json:"region,omitempty" example:"ap-northeast-2"`             // Region name. Requires Provider. Empty: all regions for the provider
	Zone            string `json:"zone,omitempty" example:"ap-northeast-2a"`              // Zone name. Requires Provider and Region. Empty: all zones for the region
	InfraNamePrefix string `json:"infraNamePrefix,omitempty" example:"infra-01"`          // Infra name prefix
	Option          string `json:"option,omitempty" example:"vNet,securityGroup"`         // Resource types (csv): vNet, securityGroup, sshKey, vm, dataDisk, customImage. Empty: all
	InfraFlag       string `json:"infraFlag,omitempty" example:"y"`                       // Infra flag: y or n
}

// UpdateScheduleJobRequest is struct for updating a scheduled job
type UpdateScheduleJobRequest struct {
	IntervalSeconds *int  `json:"intervalSeconds,omitempty" example:"60"` // New execution interval in seconds
	Enabled         *bool `json:"enabled,omitempty" example:"true"`       // Enable or disable the job
}

// ScheduleJobStatus is struct for scheduled job status response
type ScheduleJobStatus struct {
	JobId               string    `json:"jobId" example:"registerCspResources-default-1698765432"`
	JobType             string    `json:"jobType" example:"registerCspResources"`
	NsId                string    `json:"nsId" example:"default"`
	Status              string    `json:"status" example:"Scheduled"`
	IntervalSeconds     int       `json:"intervalSeconds" example:"60"`
	Enabled             bool      `json:"enabled" example:"true"`
	CreatedAt           time.Time `json:"createdAt" example:"2023-10-27T10:30:00Z"`
	LastExecutedAt      time.Time `json:"lastExecutedAt" example:"2023-10-27T11:30:00Z"`
	NextExecutionAt     time.Time `json:"nextExecutionAt" example:"2023-10-27T12:30:00Z"`
	ExecutionCount      int       `json:"executionCount" example:"5"`
	SuccessCount        int       `json:"successCount" example:"4"`        // Total successful executions
	FailureCount        int       `json:"failureCount" example:"1"`        // Total failed executions
	ConsecutiveFailures int       `json:"consecutiveFailures" example:"0"` // Current consecutive failures
	AutoDisabled        bool      `json:"autoDisabled" example:"false"`    // Whether job was auto-disabled due to failures
	LastError           string    `json:"lastError,omitempty" example:""`
	LastResult          string    `json:"lastResult,omitempty" example:"Success (execution #5)"`

	// Job-specific parameters
	ConnectionName  string `json:"connectionName,omitempty" example:"aws-ap-northeast-2"` // (Deprecated)
	Provider        string `json:"provider,omitempty" example:"aws"`
	Region          string `json:"region,omitempty" example:"ap-northeast-2"`
	Zone            string `json:"zone,omitempty" example:"ap-northeast-2a"`
	InfraNamePrefix string `json:"infraNamePrefix,omitempty" example:"infra-01"`
	Option          string `json:"option,omitempty" example:""`
	InfraFlag       string `json:"infraFlag,omitempty" example:"y"`
}

// ScheduleJobListResponse is struct for list of scheduled jobs
type ScheduleJobListResponse struct {
	Jobs []ScheduleJobStatus `json:"jobs"`
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
	if list.mux == nil {
		list.mux = &sync.Mutex{}
	}
	list.mux.Lock()
	defer list.mux.Unlock()
	list.IdList = append(list.IdList, id)
}

type IdList struct {
	IdList []string `json:"output"`
	mux    *sync.Mutex
}

// ResourceDeleteResult represents the result of a single resource deletion operation.
type ResourceDeleteResult struct {
	// Resource type (e.g., "securityGroup", "sshKey", "vNet")
	ResourceType string `json:"resourceType" example:"securityGroup"`
	// Resource ID
	ResourceId string `json:"resourceId" example:"default-shared-aws-ap-northeast-2"`
	// Whether the deletion was successful
	Success bool `json:"success" example:"true"`
	// Descriptive message (error detail on failure, empty on success)
	Message string `json:"message" example:"Cannot delete resource because it is still referenced by ..."`
}

// ResourceDeleteResults represents the aggregated results of a batch resource deletion operation.
type ResourceDeleteResults struct {
	// Total number of resources processed
	Total int `json:"total" example:"10"`
	// Number of successfully deleted resources
	SuccessCount int `json:"successCount" example:"8"`
	// Number of failed deletions
	FailedCount int `json:"failedCount" example:"2"`
	// Individual results per resource
	Results []ResourceDeleteResult `json:"results"`
}

// ResourceReconcileResult represents the result of a single resource reconciliation operation.
type ResourceReconcileResult struct {
	// Resource type (e.g., "vNet", "securityGroup", "sshKey")
	ResourceType string `json:"resourceType" example:"vNet"`
	// Resource ID
	ResourceId string `json:"resourceId" example:"vnet00"`
	// Connection name
	ConnectionName string `json:"connectionName" example:"aws-ap-northeast-2"`
	// Whether the reconciliation was successful
	Success bool `json:"success" example:"true"`
	// Elapsed time in seconds (numeric, 2 decimal places)
	ElapsedSeconds float64 `json:"elapsedSeconds" example:"2.31"`
	// Human-readable elapsed time
	Elapsed string `json:"elapsed" example:"2.3s"`
	// Descriptive message about the reconciliation outcome
	Message string `json:"message,omitempty" example:"vNet (vnet00) on CSP (aws-ap-northeast-2) reconciled; 2 subnet(s): 2 consistent / 0 restored / 0 cleaned / 0 csp-only / 0 error(s)"`
	// Error detail if reconciliation failed
	Error string `json:"error,omitempty" example:"failed to get CSP status: connection timeout"`
}

// ResourceReconcileResults represents the aggregated results of a batch resource reconciliation operation.
type ResourceReconcileResults struct {
	// Total number of resources processed
	Total int `json:"total" example:"10"`
	// Number of successfully reconciled resources
	SuccessCount int `json:"successCount" example:"9"`
	// Number of failed reconciliations
	FailedCount int `json:"failedCount" example:"1"`
	// Total elapsed time in seconds (numeric, 2 decimal places)
	ElapsedSeconds float64 `json:"elapsedSeconds" example:"106.43"`
	// Human-readable total elapsed time
	Elapsed string `json:"elapsed" example:"1m 46s"`
	// Individual results per resource
	Results []ResourceReconcileResult `json:"results"`
}

// OptionalParameter is struct for optional parameter for function (ex. NodeId)
type OptionalParameter struct {
	Value string
	Set   bool
}

// SystemReady is global variable for checking SystemReady status
var SystemReady bool

// SystemInitialized indicates whether the system has been fully initialized
// (e.g., credentials and connection configs registered via init.py)
// This is set to true when PUT /readyz/init is called after init.py completes
var SystemInitialized bool

var SpiderRestUrl string
var DragonflyRestUrl string
var TerrariumRestUrl string
var APIUsername string
var APIPassword string
var SpiderAPIUsername string
var SpiderAPIPassword string
var DBUrl string
var DBDatabase string
var DBUser string
var DBPassword string
var AutocontrolDurationMs string
var DefaultNamespace string
var DefaultCredentialHolder string
var EtcdEndpoints string
var SelfEndpoint string
var VaultAddr string
var VaultToken string
var MyDB *sql.DB
var err error

// var ORM *xorm.Engine
var ORM *gorm.DB

const (
	StrManager               string = "cb-tumblebug"
	StrUidPrefix             string = "tb"
	StrSpiderRestUrl         string = "TB_SPIDER_REST_URL"
	StrDragonflyRestUrl      string = "TB_DRAGONFLY_REST_URL"
	StrTerrariumRestUrl      string = "TB_TERRARIUM_REST_URL"
	StrAPIUsername           string = "TB_API_USERNAME"
	StrAPIPassword           string = "TB_API_PASSWORD"
	StrSpiderAPIUsername     string = "TB_SPIDER_USERNAME"
	StrSpiderAPIPassword     string = "TB_SPIDER_PASSWORD"
	StrDBUrl                 string = "TB_POSTGRES_ENDPOINT"
	StrDBDatabase            string = "TB_POSTGRES_DATABASE"
	StrDBUser                string = "TB_POSTGRES_USER"
	StrDBPassword            string = "TB_POSTGRES_PASSWORD"
	StrAutocontrolDurationMs string = "TB_AUTOCONTROL_DURATION_MS"
	StrEtcdEndpoints         string = "TB_ETCD_ENDPOINTS"
	StrVaultAddr             string = "VAULT_ADDR"
	StrVaultToken            string = "VAULT_TOKEN"
	StrFromAssets            string = "from-assets"
	ErrStrKeyNotFound        string = "key not found"
	StrAdd                   string = "add"
	StrDelete                string = "delete"
	StrSSHKey                string = "sshKey"
	StrKeypair               string = "keypair"
	StrImage                 string = "image"
	StrCustomImage           string = "customImage"
	StrMyImage               string = "myimage"
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
	StrNode                  string = "node"
	StrSpiderVM              string = "vm" // CB-Spider uses "vm" as the resource type for VMs
	StrInfra                 string = "infra"
	StrNodeGroup             string = "nodeGroup"
	StrK8s                   string = "k8s"
	StrK8sCluster            string = "k8sCluster"
	StrKubernetes            string = "kubernetes"
	StrCluster               string = "cluster"
	StrContainer             string = "container"
	StrNamespace             string = "ns"
	StrTemplate              string = "template"
	StrCommon                string = "common"
	StrGlobalDns             string = "globalDns"
	StrEmpty                 string = ""
	StrSharedResourceName    string = "-shared-"
	// StrFirewallRule               string = "firewallRule"

	// SystemCommonNs is const for SystemCommon NameSpace ID
	SystemCommonNs string = "system"

	// DefaultSecurityGroupTemplateId is the SecurityGroup template ID (loaded into
	// the system namespace from init/templates/) used to create the default shared
	// SecurityGroup when no SgTemplateId is explicitly requested during dynamic
	// provisioning. Keeping the default firewall policy in an editable template
	// (rather than hard-coded in Go) makes it the single source of truth and lets
	// it be changed without recompiling the server.
	DefaultSecurityGroupTemplateId string = "sg-default"

	// DefaultVNetTemplateId is the VNet template (loaded into the system namespace from
	// init/templates/) used to create the default shared VNet when no VNetTemplateId is
	// requested during dynamic provisioning. Keeping the default network policy in an
	// editable template makes it the single source of truth (no hard-coded default path).
	DefaultVNetTemplateId string = "vnet-default"

	// K8sSecurityGroupTemplateId is the SecurityGroup template (loaded into the system
	// namespace from init/templates/) used for the shared SecurityGroup of managed
	// Kubernetes (dynamic) node groups. It is intentionally permissive by default because
	// required ports vary by CSP; edit sg-k8s.json to change the K8s policy without
	// recompiling.
	K8sSecurityGroupTemplateId string = "sg-k8s"

	// PurposeInfraDynamic is the value stored in the sys.purpose label of a resource
	// (SecurityGroup or SSHKey) that was auto-generated for a specific Infra during
	// dynamic provisioning. Such resources are dedicated to one Infra (named
	// "{infraId}-{connection}") rather than shared across the connection, and are targeted
	// (together with the "{ns}-shared-" named resources) by the unused-resource release
	// operation once no VMs reference them.
	PurposeInfraDynamic string = "infra-dynamic"

	// FirewallCidrKeywordInternal is a placeholder CIDR keyword usable in a
	// SecurityGroup template's firewallRules. At dynamic provisioning time it is
	// replaced with the target VNet's own CIDR block, so a static template can
	// express node-to-node (internal) access without hard-coding the address range.
	FirewallCidrKeywordInternal string = "internal"

	// CredentialHolderHeaderKey is the HTTP header key for specifying credential holder
	CredentialHolderHeaderKey string = "x-credential-holder"
)

// ContextKey is a typed key for context values to prevent collisions with
// keys defined in other packages, following Go best practices.
type ContextKey string

const (
	// CtxKeyCredentialHolder is the context.Context key for credential holder
	CtxKeyCredentialHolder ContextKey = "credentialHolder"

	// CtxKeyRequestID is the context.Context key for request ID
	CtxKeyRequestID ContextKey = "requestID"
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
var ResourceTypeRegistry = map[string]func() any{
	StrSSHKey:        func() any { return &SshKeyInfo{} },
	StrImage:         func() any { return &ImageInfo{} },
	StrCustomImage:   func() any { return &ImageInfo{} },
	StrSecurityGroup: func() any { return &SecurityGroupInfo{} },
	StrSpec:          func() any { return &SpecInfo{} },
	StrVNet:          func() any { return &VNetInfo{} },
	StrSubnet:        func() any { return &SubnetInfo{} },
	StrDataDisk:      func() any { return &DataDiskInfo{} },
	StrNLB:           func() any { return &NLBInfo{} },
	StrNode:          func() any { return &NodeInfo{} },
	StrInfra:         func() any { return &InfraInfo{} },
	StrK8s:           func() any { return &K8sClusterInfo{} },
	StrNamespace:     func() any { return &NsInfo{} },
	StrVPN:           func() any { return &VpnInfo{} },
	StrGlobalDns:     func() any { return &GlobalDnsRecordInfo{} },
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
	// OpenBaoStatus reports whether this credential was also stored in OpenBao
	// (used for direct CSP API calls). Values: "registered at <path>",
	// "skipped: <reason>", or "failed: <reason>".
	OpenBaoStatus string `json:"openBaoStatus,omitempty"`
}

// OpenBaoStatusInfo reports whether the OpenBao credential store is usable by CB-Tumblebug.
// @Description OpenBao (secret store) connectivity and readiness status
type OpenBaoStatusInfo struct {
	// VaultAddr is the OpenBao endpoint CB-Tumblebug is configured with (VAULT_ADDR)
	VaultAddr string `json:"vaultAddr" example:"http://openbao:8200"`
	// VaultTokenSet indicates whether VAULT_TOKEN is set in the CB-Tumblebug environment
	VaultTokenSet bool `json:"vaultTokenSet" example:"true"`
	// Reachable indicates whether the OpenBao endpoint responded
	Reachable bool `json:"reachable" example:"true"`
	// Initialized indicates whether OpenBao itself has been initialized
	Initialized bool `json:"initialized" example:"true"`
	// Sealed indicates whether OpenBao is sealed (secrets inaccessible until unsealed)
	Sealed bool `json:"sealed" example:"false"`
	// TokenValid indicates whether the configured VAULT_TOKEN was accepted by OpenBao
	TokenValid bool `json:"tokenValid" example:"true"`
	// Available is true only when credentials can actually be stored and read
	Available bool `json:"available" example:"true"`
	// Message describes the first detected problem, or confirms availability
	Message string `json:"message" example:"OpenBao is available for credential storage"`
}

// ConnConfigList is struct for containing a CB-Spider struct for connection config list
type ConnConfigList struct { // Spider
	Connectionconfig []ConnConfig `json:"connectionconfig"`
}

// CredentialHolderInfo is struct for credential holder summary derived from registered connections.
// @Description Credential holder summary with associated providers and connection counts.
type CredentialHolderInfo struct {
	// CredentialHolder is the holder identifier (e.g., "admin", "team-a")
	CredentialHolder string `json:"credentialHolder" example:"admin"`
	// Providers is the list of cloud providers registered under this holder
	Providers []string `json:"providers" example:"aws,gcp,azure"`
	// ConnectionCount is the total number of connection configs for this holder
	ConnectionCount int `json:"connectionCount" example:"42"`
	// VerifiedConnectionCount is the number of verified connections for this holder
	VerifiedConnectionCount int `json:"verifiedConnectionCount" example:"38"`
	// IsDefault indicates whether this holder is the system default
	IsDefault bool `json:"isDefault" example:"true"`
}

// CredentialHolderList is struct for containing a list of credential holder summaries
// @Description List of credential holder summaries
type CredentialHolderList struct {
	CredentialHolderList []CredentialHolderInfo `json:"credentialHolderList"`
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
	Node          int `json:"node"`
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

// InspectResourcesResponse is struct for response of InspectResources request
type InspectResourcesResponse struct {
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
	OnTumblebug          int `json:"onTumblebug"`
	OnSpider             int `json:"onSpider"`
	OnCspTotal           int `json:"onCspTotal"`
	OnCspOnly            int `json:"onCspOnly"`
	OnSpiderNotTumblebug int `json:"onSpiderNotTumblebug"`
}

// ResourcesByManageType is struct for Resources by Manage Type
type ResourcesByManageType struct {
	OnTumblebug ResourceOnTumblebug `json:"onTumblebug"`
	OnSpider    ResourceOnSpider    `json:"onSpider"`
	OnCspTotal  ResourceOnCsp       `json:"onCspTotal"`
	OnCspOnly   ResourceOnCsp       `json:"onCspOnly"`
	// OnSpiderNotTumblebug holds resources CB-Spider already has mapped (so they exist
	// on the CSP and are not caught by OnCspOnly) but that are missing from CB-TB's own
	// KV store — e.g. orphaned by a race in shared-resource creation.
	OnSpiderNotTumblebug ResourceOnCsp `json:"onSpiderNotTumblebug"`
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
	InfraId       string `json:"infraId,omitempty"`
	ObjectKey     string `json:"objectKey"`
}

// RegisterResourceAllResult is struct for Register Resource Result for All Clouds
type RegisterResourceAllResult struct {
	ElapsedTime          int                      `json:"elapsedTime"`
	RegisteredConnection int                      `json:"registeredConnection"`
	AvailableConnection  int                      `json:"availableConnection"`
	RegistrationOverview RegistrationOverview     `json:"registerationOverview"`
	RegisterationResult  []RegisterResourceResult `json:"registerationResult"`
}

// RegisterResourceResult is struct for Register Resource Result
type RegisterResourceResult struct {
	ConnectionName       string               `json:"connectionName"`
	SystemMessage        string               `json:"systemMessage"`
	ElapsedTime          int                  `json:"elapsedTime"`
	RegistrationOverview RegistrationOverview `json:"registerationOverview"`
	RegisterationOutputs IdList               `json:"registerationOutputs"`
}

// RegisterResource is struct for Register Resource
type RegistrationOverview struct {
	VNet          int `json:"vNet"`
	SecurityGroup int `json:"securityGroup"`
	SshKey        int `json:"sshKey"`
	DataDisk      int `json:"dataDisk"`
	CustomImage   int `json:"customImage"`
	Node          int `json:"node"`
	NLB           int `json:"nlb"`
	Failed        int `json:"failed"`
}

// InspectResourcesRequest struct for Inspect Resources Request
type InspectResourcesRequest struct {
	ConnectionName string `json:"connectionName" example:"aws-ap-southeast-1"`
	ResourceType   string `json:"resourceType" example:"vNet" enums:"vNet,subnet,securityGroup,sshKey,node"`
}

// CspResourceStatusRequest is struct for requesting CSP resource status from CB-Spider
type CspResourceStatusRequest struct {
	ConnectionName string `json:"ConnectionName"`
}

// CspResourceStatusResponse is struct for CSP resource status response from CB-Spider
type CspResourceStatusResponse struct {
	ConnectionName string        `json:"connectionName"`
	ResourceType   string        `json:"resourceType"`
	AllList        SpiderAllList `json:"allList"`
	SystemMessage  string        `json:"systemMessage,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// SpiderVpcInfo is struct for VPC information from CB-Spider
type SpiderVpcInfo struct {
	IId            IID                `json:"iId"`
	IPv4_CIDR      string             `json:"ipv4_CIDR"`
	SubnetInfoList []SpiderSubnetInfo `json:"subnetInfoList"`
	KeyValueList   []KeyValue         `json:"keyValueList"`
	TagList        []KeyValue         `json:"tagList"`
}

// SpiderSubnetInfo is struct for Subnet information from CB-Spider
type SpiderSubnetInfo struct {
	IId          IID        `json:"iId"`
	IPv4_CIDR    string     `json:"ipv4_CIDR"`
	KeyValueList []KeyValue `json:"keyValueList"`
	TagList      []KeyValue `json:"tagList"`
}

// SpiderBooleanInfo is struct for boolean type response from CB-Spider DELETE operations
// Spider returns {"Result": "true"} or {"Result": "false"} for delete operations
type SpiderBooleanInfo struct {
	Result string `json:"Result"`
}

// SpiderAllVpcInfoWrapper is struct for wrapping VPC info response from CB-Spider
type SpiderAllVpcInfoWrapper struct {
	ResourceType string               `json:"resourceType"`
	AllListInfo  SpiderAllVpcListInfo `json:"allListInfo"`
}

// SpiderAllVpcListInfo is struct for VPC list info from CB-Spider
type SpiderAllVpcListInfo struct {
	MappedInfoList  []SpiderVpcInfo `json:"mappedInfoList"`
	OnlySpiderList  []SpiderVpcInfo `json:"onlySpiderList"`
	OnlyCSPInfoList []SpiderVpcInfo `json:"onlyCSPInfoList"`
}
