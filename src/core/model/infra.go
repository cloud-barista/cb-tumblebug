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
	"time"
)

const (

	// ActionCreate is const for Create
	ActionCreate string = "Create"

	// ActionTerminate is const for Terminate
	ActionTerminate string = "Terminate"

	// ActionSuspend is const for Suspend
	ActionSuspend string = "Suspend"

	// ActionResume is const for Resume
	ActionResume string = "Resume"

	// ActionReboot is const for Reboot
	ActionReboot string = "Reboot"

	// ActionRefine is const for Refine
	ActionRefine string = "Refine"

	// ActionComplete is const for Complete
	ActionComplete string = "None"
)
const (

	// StatusPreparing is const for Preparing
	StatusPreparing string = "Preparing"

	// StatusPrepared is const for Prepared
	StatusPrepared string = "Prepared"

	// StatusRunning is const for Running
	StatusRunning string = "Running"

	// StatusSuspended is const for Suspended
	StatusSuspended string = "Suspended"

	// StatusFailed is const for Failed
	StatusFailed string = "Failed"

	// StatusTerminated is const for Terminated
	StatusTerminated string = "Terminated"

	// StatusCreating is const for Creating
	StatusCreating string = "Creating"

	// StatusSuspending is const for Suspending
	StatusSuspending string = "Suspending"

	// StatusResuming is const for Resuming
	StatusResuming string = "Resuming"

	// StatusRebooting is const for Rebooting
	StatusRebooting string = "Rebooting"

	// StatusTerminating is const for Terminating
	StatusTerminating string = "Terminating"

	// StatusRegistering is const for Registering (when registering existing CSP Node)
	StatusRegistering string = "Registering"

	// StatusUndefined is const for Undefined
	StatusUndefined string = "Undefined"

	// StatusEmpty is const for Empty (Infra has no Nodes)
	StatusEmpty string = "Empty"

	// StatusComplete is const for Complete
	StatusComplete string = "None"
)

// Provisioning failure handling policies
const (
	// PolicyContinue continues with partial Infra creation when some Nodes fail
	PolicyContinue string = "continue"

	// PolicyRollback cleans up entire Infra when any Node creation fails
	PolicyRollback string = "rollback"

	// PolicyRefine marks failed Nodes for refinement when creation fails
	PolicyRefine string = "refine"
)

const StrAutoGen string = "autogen"

// DefaultSystemLabel is const for string to specify the Default System Label
const DefaultSystemLabel string = "Managed by CB-Tumblebug"

// RegionInfo is struct for region information
type RegionInfo struct {
	Region string `json:"region" example:"us-east-1"`
	Zone   string `json:"zone,omitempty" example:"us-east-1a"`
}

// InfraReq is struct for requirements to create Infra
type InfraReq struct {
	Name string `json:"name" validate:"required" example:"infra01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// SystemLabel is for describing the infra in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	PlacementAlgo string `json:"placementAlgo,omitempty"`
	Description   string `json:"description" example:"Made in CB-TB"`

	NodeGroups []CreateNodeGroupReq `json:"nodeGroups" validate:"required"`

	// PostCommand is for the command to bootstrap the Nodes
	PostCommand InfraCmdReq `json:"postCommand" validate:"omitempty"`

	// PolicyOnPartialFailure determines how to handle Node creation failures
	// - "continue": Continue with partial Infra creation (default)
	// - "rollback": Cleanup entire Infra when any Node fails
	// - "refine": Mark failed Nodes for refinement
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue" default:"continue" enums:"continue,rollback,refine"`
}

// ResourceStatusInfo is struct for status information of a resource
type ResourceStatusInfo struct {
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`
}

// InfraInfo is struct for Infra info
type InfraInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`

	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:no)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// ConfigureCloudAdaptiveNetwork is an option to configure Cloud Adaptive Network (CLADNet) ([yes/no] default:yes)
	ConfigureCloudAdaptiveNetwork string `json:"configureCloudAdaptiveNetwork" example:"yes" default:"no" enums:"yes,no"` // yes or no

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// SystemLabel is for describing the infra in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Latest system message such as error message
	SystemMessage []string `json:"systemMessage"` // systeam-given string message

	PlacementAlgo string     `json:"placementAlgo,omitempty"`
	Description   string     `json:"description"`
	Node          []NodeInfo `json:"node"`

	// Cluster is the list of implicit clusters synthesized at query-time from Nodes.
	Cluster []InfraClusterInfo `json:"cluster,omitempty"`

	// List of IDs for new nodes. Return IDs if the nodes are newly added. This field should be used for return body only.
	NewNodeList []string `json:"newNodeList"`

	// PostCommand is for the command to bootstrap the Nodes
	PostCommand InfraCmdReq `json:"postCommand"`

	// PostCommandResult is the result of the command for bootstraping the Nodes
	PostCommandResult InfraSshCmdResult `json:"postCommandResult"`

	// CreationErrors contains information about Node creation failures (if any)
	CreationErrors *InfraCreationErrors `json:"creationErrors,omitempty"`
}

// InfraCreationErrors represents errors that occurred during Infra creation
type InfraCreationErrors struct {
	// NodeObjectCreationErrors contains errors from Node object creation phase
	NodeObjectCreationErrors []NodeCreationError `json:"nodeObjectCreationErrors,omitempty"`

	// NodeCreationErrors contains errors from actual Node creation phase
	NodeCreationErrors []NodeCreationError `json:"nodeCreationErrors,omitempty"`

	// TotalNodeCount is the total number of Nodes that were supposed to be created
	TotalNodeCount int `json:"totalNodeCount"`

	// SuccessfulNodeCount is the number of Nodes that were successfully created
	SuccessfulNodeCount int `json:"successfulNodeCount"`

	// FailedNodeCount is the number of Nodes that failed to be created
	FailedNodeCount int `json:"failedNodeCount"`

	// FailureHandlingStrategy indicates how failures were handled
	FailureHandlingStrategy string `json:"failureHandlingStrategy,omitempty"` // "rollback", "refine", "continue"
}

// NodeCreationError represents a single Node creation error
type NodeCreationError struct {
	// NodeName is the name of the Node that failed
	NodeName string `json:"nodeName"`

	// Error is the error message
	Error string `json:"error"`

	// Phase indicates when the error occurred
	Phase string `json:"phase"` // "object_creation", "vm_creation"

	// Timestamp when the error occurred
	Timestamp string `json:"timestamp"`
}

// CreateNodeGroupReq is struct to get requirements to create a new server instance
type CreateNodeGroupReq struct {
	// NodeGroup name of Nodes. Actual Node name will be generated with -N postfix.
	Name string `json:"name" validate:"required" example:"g1-1"`

	// CspResourceId is resource identifier managed by CSP (required for option=register)
	CspResourceId string `json:"cspResourceId,omitempty" example:"i-014fa6ede6ada0b2c"`

	// NodeGroupSize is the number of Nodes to create in this NodeGroup. If > 0, nodeGroup will be generated.
	NodeGroupSize int `json:"nodeGroupSize" example:"3"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	Description string `json:"description" example:"Description"`

	ConnectionName string `json:"connectionName" validate:"required" example:"testcloud01-seoul"`
	SpecId         string `json:"specId" validate:"required"`
	// ImageType        string   `json:"imageType"`
	ImageId          string   `json:"imageId" validate:"required"`
	VNetId           string   `json:"vNetId" validate:"required"`
	SubnetId         string   `json:"subnetId" validate:"required"`
	SecurityGroupIds []string `json:"securityGroupIds" validate:"required"`
	SshKeyId         string   `json:"sshKeyId" validate:"required"`
	NodeUserName     string   `json:"nodeUserName,omitempty"`
	NodeUserPassword string   `json:"nodeUserPassword,omitempty"`
	RootDiskType     string   `json:"rootDiskType,omitempty" example:"default, TYPE1, ..."` // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize     int      `json:"rootDiskSize,omitempty" example:"50"`                  // Root disk size in GB. 0 = use CSP default.
	DataDiskIds      []string `json:"dataDiskIds"`
}

// CreateNodeGroupReq is struct to get requirements to create a new server instance
type ScaleOutNodeGroupReq struct {
	// Define additional Nodes to scaleOut
	NumNodesToAdd int `json:"numNodesToAdd" validate:"required" example:"2"`

	//tobe added accoring to new future capability
}

// InfraDynamicReq is struct for requirements to create Infra dynamically (with default resource option)
type InfraDynamicReq struct {
	Name string `json:"name" validate:"required" example:"infra01"`

	// PolicyOnPartialFailure determines how to handle Node creation failures
	// - "continue": Continue with partial Infra creation (default)
	// - "rollback": Cleanup entire Infra when any Node fails
	// - "refine": Mark failed Nodes for refinement
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue" default:"continue" enums:"continue,rollback,refine"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:no)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// NodeGroups is array of Node requests for multi-cloud infrastructure
	// Example: Multiple Node groups across different CSPs
	// [
	//   {
	//     "name": "aws-group",
	//     "nodeGroupSize": "3",
	//     "specId": "aws+ap-northeast-2+t3.nano",
	//     "imageId": "ami-01f71f215b23ba262",
	//     "rootDiskSize": "50",
	//     "label": {"role": "worker", "csp": "aws"}
	//   },
	//   {
	//     "name": "azure-group",
	//     "nodeGroupSize": "2",
	//     "specId": "azure+koreasouth+standard_b1s",
	//     "imageId": "Canonical:0001-com-ubuntu-server-jammy:22_04-lts:22.04.202505210",
	//     "rootDiskSize": "50",
	//     "label": {"role": "head", "csp": "azure"}
	//   },
	//   {
	//     "name": "gcp-group",
	//     "nodeGroupSize": "1",
	//     "specId": "gcp+asia-northeast3+g1-small",
	//     "imageId": "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20250712",
	//     "rootDiskSize": "50",
	//     "label": {"role": "test", "csp": "gcp"}
	//   }
	// ]
	NodeGroups []CreateNodeGroupDynamicReq `json:"nodeGroups" validate:"required"`

	// PostCommand is for the command to bootstrap the Nodes
	PostCommand InfraCmdReq `json:"postCommand"`

	// SystemLabel is for describing the infra in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	Description string `json:"description" example:"Made in CB-TB"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// VNetTemplateId specifies the vNet template ID (from system namespace) to use when
	// auto-creating shared vNet resources. Propagates to all NodeGroups unless overridden
	// at the NodeGroup level. If empty, the default hard-coded CIDR behavior is used.
	VNetTemplateId string `json:"vNetTemplateId,omitempty" example:"default-vnet"`

	// SgTemplateId specifies the SecurityGroup template ID (from system namespace) to use
	// when auto-creating shared SecurityGroup resources. Propagates to all NodeGroups unless
	// overridden at the NodeGroup level. If empty, the default all-open behavior is used.
	SgTemplateId string `json:"sgTemplateId,omitempty" example:"default-sg"`
}

// CreateNodeGroupDynamicReq is struct to get requirements to create a new server instance dynamically (with default resource option)
type CreateNodeGroupDynamicReq struct {
	// NodeGroup name, actual Node name will be generated with -N postfix.
	Name string `json:"name" example:"g1"`

	// NodeGroupSize is the number of Nodes to create in this NodeGroup. If > 0, nodeGroup will be generated. Default is 1.
	NodeGroupSize int `json:"nodeGroupSize" example:"3"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label" example:"{\"role\":\"worker\",\"env\":\"test\"}"`

	Description string `json:"description" example:"Created via CB-Tumblebug"`

	// SpecId is field for id of a spec in common namespace
	SpecId string `json:"specId" validate:"required" example:"aws+ap-northeast-2+t3.nano"`
	// ImageId is field for id of a image in common namespace
	ImageId string `json:"imageId" validate:"required" example:"ami-01f71f215b23ba262"`

	RootDiskType string `json:"rootDiskType,omitempty" example:"gp3" default:"default"` // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize int    `json:"rootDiskSize,omitempty" example:"50"`                    // Root disk size in GB. 0 = use CSP default.

	NodeUserPassword string `json:"nodeUserPassword,omitempty" example:"" default:""`
	// if ConnectionName is given, the Node tries to use associtated credential.
	// if not, it will use predefined ConnectionName in Spec objects
	ConnectionName string `json:"connectionName,omitempty" example:"aws-ap-northeast-2" default:""`
	// Zone is an optional field to specify the availability zone for Node placement.
	// If specified, subnet will be created in this zone for resources like GPU Nodes
	// that may only be available in specific zones. If empty, auto-selection applies.
	Zone string `json:"zone,omitempty" example:"ap-northeast-2a" default:""`

	// VNetTemplateId overrides the Infra-level VNetTemplateId for this NodeGroup.
	// If empty, inherits the VNetTemplateId from the parent InfraDynamicReq.
	VNetTemplateId string `json:"vNetTemplateId,omitempty" example:""`

	// SgTemplateId overrides the Infra-level SgTemplateId for this NodeGroup.
	// If empty, inherits the SgTemplateId from the parent InfraDynamicReq.
	SgTemplateId string `json:"sgTemplateId,omitempty" example:""`
}

// InfraConnectionConfigCandidatesReq is struct for a request to check requirements to create a new Infra instance dynamically (with default resource option)
type InfraConnectionConfigCandidatesReq struct {
	// SpecId is field for id of a spec in common namespace
	SpecIds []string `json:"specId" validate:"required" example:"aws+ap-northeast-2+t2.small,gcp+us-west1+g1-small"`
}

// CheckInfraDynamicReqInfo is struct to check requirements to create a new Infra instance dynamically (with default resource option)
type CheckInfraDynamicReqInfo struct {
	ReqCheck []CheckNodeGroupDynamicReqInfo `json:"reqCheck" validate:"required"`
}

// CheckNodeGroupDynamicReqInfo is struct to check requirements to create a new server instance dynamically (with default resource option)
type CheckNodeGroupDynamicReqInfo struct {

	// ConnectionConfigCandidates will provide ConnectionConfig options
	ConnectionConfigCandidates []string `json:"connectionConfigCandidates" default:""`

	//RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]

	Spec   SpecInfo     `json:"spec" default:""`
	Image  []ImageInfo  `json:"image" default:""`
	Region RegionDetail `json:"region" default:""`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

}

// ReviewInfraDynamicReqInfo is struct for review result of Infra dynamic request
type ReviewInfraDynamicReqInfo struct {
	// Overall assessment of the Infra request
	OverallStatus  string `json:"overallStatus" example:"Ready/Warning/Error"`
	OverallMessage string `json:"overallMessage" example:"All Nodes can be created successfully"`
	CreationViable bool   `json:"creationViable"`
	EstimatedCost  string `json:"estimatedCost,omitempty" example:"$0.50/hour"`

	// Infra-level information
	InfraName      string `json:"infraName"`
	TotalNodeCount int    `json:"totalNodeCount"`

	// Failure policy analysis
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue"`
	PolicyDescription      string `json:"policyDescription" example:"If some Nodes fail during creation, Infra will be created with successfully provisioned Nodes only"`
	PolicyRecommendation   string `json:"policyRecommendation,omitempty" example:"Consider 'refine' policy for automatic cleanup of failed Nodes"`

	// Node-level validation results
	NodeReviews []ReviewNodeGroupDynamicReqInfo `json:"nodeReviews"`

	// Resource availability summary
	ResourceSummary ReviewResourceSummary `json:"resourceSummary"`

	// Recommendations for improvement
	Recommendations []string `json:"recommendations,omitempty"`
}

// ReviewNodeGroupDynamicReqInfo is struct for review result of individual Node in Infra dynamic request
type ReviewNodeGroupDynamicReqInfo struct {
	// Node request information
	NodeName      string `json:"nodeName"`
	NodeGroupSize int    `json:"nodeGroupSize"`

	// Validation status
	Status    string `json:"status" example:"Ready/Warning/Error"`
	Message   string `json:"message" example:"Node can be created successfully"`
	CanCreate bool   `json:"canCreate"`

	// Resource validation details
	SpecValidation  ReviewResourceValidation `json:"specValidation"`
	ImageValidation ReviewResourceValidation `json:"imageValidation"`

	// Connection and region info
	ConnectionName string `json:"connectionName"`
	ProviderName   string `json:"providerName"`
	RegionName     string `json:"regionName"`

	// Cost estimation
	EstimatedCost string `json:"estimatedCost,omitempty" example:"$0.10/hour"`

	// General information and configuration notes
	Info []string `json:"info,omitempty"`

	// Warnings and errors
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// ReviewResourceValidation is struct for resource validation details
type ReviewResourceValidation struct {
	ResourceId    string `json:"resourceId"`
	ResourceName  string `json:"resourceName,omitempty"`
	IsAvailable   bool   `json:"isAvailable"`
	Status        string `json:"status" example:"Available/Unavailable/Unknown"`
	Message       string `json:"message,omitempty"`
	CspResourceId string `json:"cspResourceId,omitempty"`
}

// ReviewResourceSummary is struct for overall resource summary
type ReviewResourceSummary struct {
	TotalProviders  int      `json:"totalProviders"`
	TotalRegions    int      `json:"totalRegions"`
	UniqueSpecs     []string `json:"uniqueSpecs"`
	UniqueImages    []string `json:"uniqueImages"`
	ConnectionNames []string `json:"connectionNames"`

	// Provider and region details
	ProviderNames []string `json:"providerNames"`
	RegionNames   []string `json:"regionNames"`

	// Resource availability counts
	AvailableSpecs    int `json:"availableSpecs"`
	UnavailableSpecs  int `json:"unavailableSpecs"`
	AvailableImages   int `json:"availableImages"`
	UnavailableImages int `json:"unavailableImages"`
}

// SpecImagePairReviewReq is struct for spec-image pair review request.
//
// RootDiskType and Zone are OPTIONAL and used to make the availability
// pre-check more precise:
//   - RootDiskType: when empty or "default", the checker queries availability
//     across all disk categories supported in the region (CSP/Spider will
//     pick its own default at provision time). When a specific category is
//     given (e.g., "cloud_essd"), the result reflects stock for that exact
//     disk type, allowing the UI to flag combinations that will fail.
//   - Zone: when set, the checker scopes the availability query to that
//     single zone (so SuggestedSystemDisk and Available reflect that zone
//     only). When empty, the result spans all zones in the region.
type SpecImagePairReviewReq struct {
	SpecId       string `json:"specId" validate:"required" example:"aws+ap-northeast-2+t3.nano"`
	ImageId      string `json:"imageId" validate:"required" example:"ami-01f71f215b23ba262"`
	RootDiskType string `json:"rootDiskType,omitempty" example:"default"` // "", "default", or CSP-native disk category (e.g., "cloud_essd")
	Zone         string `json:"zone,omitempty" example:""`                // optional CSP-native zone id; empty = all zones in region
}

// SpecImagePairReviewResult is struct for spec-image pair review result
type SpecImagePairReviewResult struct {
	// Review summary
	IsValid bool   `json:"isValid"`
	Status  string `json:"status" example:"OK/Warning/Error"`
	Message string `json:"message" example:"Spec and image pair is valid for provisioning"`

	// Input parameters
	SpecId  string `json:"specId"`
	ImageId string `json:"imageId"`

	// Spec details
	SpecValidation ReviewResourceValidation `json:"specValidation"`
	SpecDetails    *SpecInfo                `json:"specDetails,omitempty"`

	// Image details
	ImageValidation ReviewResourceValidation `json:"imageValidation"`
	ImageDetails    *ImageInfo               `json:"imageDetails,omitempty"`

	// Connection info
	ConnectionName string `json:"connectionName,omitempty"`
	ProviderName   string `json:"providerName,omitempty"`
	RegionName     string `json:"regionName,omitempty"`

	// Cost estimation
	EstimatedCost string `json:"estimatedCost,omitempty" example:"$0.0052/hour"`

	// Pre-flight availability (zone × disk-category) from CSP-specific checker.
	// Populated when a checker is registered for the provider; nil otherwise.
	Availability *AvailabilityResult `json:"availability,omitempty"`

	// SuggestedZone is a zone picked from Availability.Zones that has stock and
	// supports the requested (or any) system-disk category. Empty when no
	// suggestion is possible (e.g., provider has no checker, or every zone is
	// out of stock). Callers can use this to pre-fill ZoneId for VM creation
	// to improve 1-shot success rate.
	SuggestedZone string `json:"suggestedZone,omitempty"`

	// SuggestedSystemDisk is a system-disk category that is currently
	// available in SuggestedZone. Empty when no suggestion is possible.
	SuggestedSystemDisk string `json:"suggestedSystemDisk,omitempty"`

	// RequestedRootDiskType echoes the input RootDiskType (after normalization:
	// empty/"default" -> ""). Useful for UI to confirm what was checked.
	RequestedRootDiskType string `json:"requestedRootDiskType,omitempty"`

	// RequestedZone echoes the input Zone. Empty means region-wide check.
	RequestedZone string `json:"requestedZone,omitempty"`

	// Additional info
	Info     []string `json:"info,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

//

// SpiderVMReqInfoWrapper is struct from CB-Spider (VMHandler.go) for wrapping SpiderVMReqInfo
type SpiderVMReqInfoWrapper struct {
	ConnectionName string
	ReqInfo        SpiderVMReqInfo
}

type SpiderImageType string

const (
	PublicImage SpiderImageType = "PublicImage"
	MyImage     SpiderImageType = "MyImage"
)

// Ref: cb-spider/cloud-control-manager/cloud-driver/interfaces/resources/VMHandler.go
// SpiderVMReqInfo is struct from CB-Spider for Node request information
type SpiderVMReqInfo struct {
	// Fields for request
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string
	CSPid              string // Node ID given by CSP (required for registering Node)
	DataDiskNames      []string

	// Fields for both request and response
	VMSpecName   string // instance type or flavour, etc... ex) t2.micro or f1.micro
	VMUserId     string // ex) user1
	VMUserPasswd string
	RootDiskType string // "SSD(gp2)", "Premium SSD", ...
	RootDiskSize string // "default", "50", "1000" (GB)
	ImageType    SpiderImageType
}

// Ref: cb-spider/cloud-control-manager/cloud-driver/interfaces/resources/VMHandler.go
// SpiderVMInfo is struct from CB-Spider for VM information
type SpiderVMInfo struct {

	// Fields for both request and response
	VMSpecName   string // instance type or flavour, etc... ex) t2.micro or f1.micro
	VMUserId     string // ex) user1
	VMUserPasswd string
	RootDiskType string // "SSD(gp2)", "Premium SSD", ...
	RootDiskSize string // "default", "50", "1000" (GB)
	ImageType    SpiderImageType

	// Fields for response
	IId               IID // {NameId, SystemId}
	ImageIId          IID
	VpcIID            IID
	SubnetIID         IID   // AWS, ex) subnet-8c4a53e4
	SecurityGroupIIds []IID // AWS, ex) sg-0b7452563e1121bb6
	KeyPairIId        IID
	DataDiskIIDs      []IID
	StartTime         time.Time
	Region            RegionInfo //  ex) {us-east1, us-east1-c} or {ap-northeast-2}
	NetworkInterface  string     // ex) eth0
	PublicIP          string
	PublicDNS         string
	PrivateIP         string
	PrivateDNS        string
	RootDeviceName    string // "/dev/sda1", ...
	SSHAccessPoint    string
	KeyValueList      []KeyValue
}

// NodeGroupInfo is struct to define an object that includes homogeneous Nodes
type NodeGroupInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	NodeId        []string `json:"nodeId"`
	NodeGroupSize int      `json:"nodeGroupSize"`
}

// InfraClusterInfo is a lightweight, on-demand cluster view synthesized from Infra NodeGroups and Nodes.
// A cluster is implicitly formed by NodeGroups that share the same network boundary (currently VNet-centric grouping).
type InfraClusterInfo struct {
	// Id is a deterministic cluster identifier generated from grouping attributes.
	Id string `json:"id"`

	// Name is a human-readable cluster name. Currently same as Id.
	Name string `json:"name"`

	// InfraId is the parent Infra ID.
	InfraId string `json:"infraId"`

	// VNetId is the shared VNet boundary used for implicit clustering.
	VNetId string `json:"vNetId,omitempty"`

	// ConnectionNames are unique connection names included in this cluster.
	ConnectionNames []string `json:"connectionNames"`

	// ProviderNames are unique CSP providers included in this cluster.
	ProviderNames []string `json:"providerNames"`

	// RegionNames are unique regions included in this cluster.
	RegionNames []string `json:"regionNames"`

	// NodeGroupIds are NodeGroups that belong to this implicit cluster.
	NodeGroupIds []string `json:"nodeGroupIds"`

	// NodeIds are Nodes that belong to this implicit cluster.
	NodeIds []string `json:"nodeIds"`

	// NodeGroupCount is the number of NodeGroups in this cluster.
	NodeGroupCount int `json:"nodeGroupCount"`

	// NodeCount is the number of Nodes in this cluster.
	NodeCount int `json:"nodeCount"`

	// RepresentativeNodeGroupId is a representative NodeGroup ID for quick inspection.
	RepresentativeNodeGroupId string `json:"representativeNodeGroupId,omitempty"`

	// RepresentativeNodeId is a representative Node ID for quick inspection.
	RepresentativeNodeId string `json:"representativeNodeId,omitempty"`
}

// InfraClusterList is a response wrapper for listing implicit clusters in an Infra.
type InfraClusterList struct {
	Cluster []InfraClusterInfo `json:"cluster"`
}

// InfraAssociatedResourceList is struct for associated resource IDs of an Infra
type InfraAssociatedResourceList struct {
	ConnectionNames []string `json:"connectionNames"`
	ProviderNames   []string `json:"providerNames"`

	NodeGroupIds []string `json:"nodeGroupIds"`
	NodeIds      []string `json:"nodeIds"`
	CspNodeNames []string `json:"cspNodeNames"`
	CspNodeIds   []string `json:"cspNodeIds"`
	ImageIds     []string `json:"imageIds"`
	SpecIds      []string `json:"specIds"`

	VNetIds          []string `json:"vNetIds"`
	CspVNetIds       []string `json:"cspVNetIds"`
	SubnetIds        []string `json:"subnetIds"`
	CspSubnetIds     []string `json:"cspSubnetIds"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	DataDiskIds      []string `json:"dataDiskIds"`
	SSHKeyIds        []string `json:"sshKeyIds"`
}

type NodeInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	// defined if the Node is in a group
	NodeGroupId string `json:"nodeGroupId"`

	Location Location `json:"location"`

	// Required by CB-Tumblebug
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// NetworkAgent status
	NetworkAgentStatus string `json:"networkAgentStatus" example:"[notInstalled, installing, installed, failed]"` // notInstalled, installing, installed, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	Label       map[string]string `json:"label"`
	Description string            `json:"description"`

	Region         RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP       string     `json:"publicIP"`
	SSHPort        int        `json:"sshPort"`
	PublicDNS      string     `json:"publicDNS"`
	PrivateIP      string     `json:"privateIP"`
	PrivateDNS     string     `json:"privateDNS"`
	RootDiskType   string     `json:"rootDiskType"`
	RootDiskSize   int        `json:"rootDiskSize"`
	RootDeviceName string     `json:"RootDeviceName"`

	ConnectionName   string       `json:"connectionName"`
	ConnectionConfig ConnConfig   `json:"connectionConfig"`
	SpecId           string       `json:"specId"`
	CspSpecName      string       `json:"cspSpecName"`
	Spec             SpecSummary  `json:"spec,omitempty"`
	ImageId          string       `json:"imageId"`
	CspImageName     string       `json:"cspImageName"`
	Image            ImageSummary `json:"image,omitempty"`
	VNetId           string       `json:"vNetId"`
	CspVNetId        string       `json:"cspVNetId"`
	SubnetId         string       `json:"subnetId"`
	CspSubnetId      string       `json:"cspSubnetId"`
	NetworkInterface string       `json:"networkInterface"`
	SecurityGroupIds []string     `json:"securityGroupIds"`
	DataDiskIds      []string     `json:"dataDiskIds"`
	SshKeyId         string       `json:"sshKeyId"`
	CspSshKeyId      string       `json:"cspSshKeyId"`
	NodeUserName     string       `json:"nodeUserName,omitempty"`
	NodeUserPassword string       `json:"nodeUserPassword,omitempty"`

	// SshHostKeyInfo contains SSH host key information for TOFU (Trust On First Use) verification
	SshHostKeyInfo *SshHostKeyInfo `json:"sshHostKeyInfo,omitempty"`

	// CommandStatus stores the status and history of remote commands executed on this Node
	CommandStatus []CommandStatusInfo `json:"commandStatus,omitempty"`

	AddtionalDetails []KeyValue `json:"addtionalDetails,omitempty"`
}

// InfraAccessInfo is struct to retrieve overall access information of a Infra
type InfraAccessInfo struct {
	InfraId                  string
	InfraNlbListener         *InfraAccessInfo `json:"infraNlbListener,omitempty"`
	InfraNodeGroupAccessInfo []InfraNodeGroupAccessInfo
}

// InfraNodeGroupAccessInfo is struct for InfraNodeGroupAccessInfo
type InfraNodeGroupAccessInfo struct {
	NodeGroupId    string
	NlbListener    *NLBListenerInfo `json:"nlbListener,omitempty"`
	BastionNodeId  string
	NodeAccessInfo []InfraNodeAccessInfo
}

// InfraNodeAccessInfo is struct for InfraNodeAccessInfo
type InfraNodeAccessInfo struct {
	NodeId           string     `json:"nodeId"`
	PublicIP         string     `json:"publicIP"`
	PrivateIP        string     `json:"privateIP"`
	SSHPort          int        `json:"sshPort"`
	PrivateKey       string     `json:"privateKey,omitempty"`
	NodeUserName     string     `json:"nodeUserName,omitempty"`
	NodeUserPassword string     `json:"nodeUserPassword,omitempty"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`
}

// SshHostKeyInfo is struct for SSH host key information (TOFU verification)
type SshHostKeyInfo struct {
	// HostKey is the SSH host public key (base64 encoded)
	HostKey string `json:"hostKey,omitempty"`
	// KeyType is the type of the SSH host key (e.g., ssh-rsa, ssh-ed25519, ecdsa-sha2-nistp256)
	KeyType string `json:"keyType,omitempty" example:"ssh-ed25519"`
	// Fingerprint is the SHA256 fingerprint of the SSH host key
	Fingerprint string `json:"fingerprint,omitempty" example:"SHA256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
	// FirstUsedAt is the timestamp when the host key was first stored (TOFU moment)
	FirstUsedAt string `json:"firstUsedAt,omitempty" example:"2024-01-15T10:30:00Z"`
}

// IdNameInDetailInfo is struct for details related with ID and Name
type IdNameInDetailInfo struct {
	IdInTb    string `json:"idInTb"`
	IdInSp    string `json:"idInSp"`
	IdInCsp   string `json:"idInCsp"`
	NameInCsp string `json:"nameInCsp"`
}

// StatusCountInfo is struct to count the number of Nodes in each status. ex: Running=4, Suspended=8.
type StatusCountInfo struct {

	// CountTotal is for Total Nodes
	CountTotal int `json:"countTotal"`

	// CountCreating is for counting Creating
	CountCreating int `json:"countCreating"`

	// CountRunning is for counting Running
	CountRunning int `json:"countRunning"`

	// CountFailed is for counting Failed
	CountFailed int `json:"countFailed"`

	// CountSuspended is for counting Suspended
	CountSuspended int `json:"countSuspended"`

	// CountRebooting is for counting Rebooting
	CountRebooting int `json:"countRebooting"`

	// CountTerminated is for counting Terminated
	CountTerminated int `json:"countTerminated"`

	// CountSuspending is for counting Suspending
	CountSuspending int `json:"countSuspending"`

	// CountResuming is for counting Resuming
	CountResuming int `json:"countResuming"`

	// CountTerminating is for counting Terminating
	CountTerminating int `json:"countTerminating"`

	// CountRegistering is for counting Registering
	CountRegistering int `json:"countRegistering"`

	// CountUndefined is for counting Undefined
	CountUndefined int `json:"countUndefined"`
}

// InfraRecommendReq is struct for InfraRecommendReq
type InfraRecommendReq struct {
	NodeReq        []NodeRecommendReq `json:"nodeReq"`
	PlacementAlgo  string             `json:"placementAlgo"`
	PlacementParam []KeyValue         `json:"placementParam"`
	MaxResultNum   string             `json:"maxResultNum"`
}

// NodeRecommendReq is struct for NodeRecommendReq
type NodeRecommendReq struct {
	RequestName  string `json:"requestName"`
	MaxResultNum string `json:"maxResultNum"`

	VcpuSize   string `json:"vcpuSize"`
	MemorySize string `json:"memorySize"`
	DiskSize   string `json:"diskSize"`
	//Disk_type   string `json:"disk_type"`

	PlacementAlgo  string     `json:"placementAlgo"`
	PlacementParam []KeyValue `json:"placementParam"`
}

// NodePriority is struct for NodePriority
type NodePriority struct {
	Priority string   `json:"priority"`
	NodeSpec SpecInfo `json:"nodeSpec"`
}

// NodeRecommendInfo is struct for NodeRecommendInfo
type NodeRecommendInfo struct {
	NodeReq          NodeRecommendReq `json:"nodeReq"`
	NodePriorityList []NodePriority   `json:"nodePriority"`
	PlacementAlgo    string           `json:"placementAlgo"`
	PlacementParam   []KeyValue       `json:"placementParam"`
}

// InfraStatusInfo is struct to define simple information of Infra with updated status of all Nodes
type InfraStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no

	MasterNodeId  string `json:"masterNodeId" example:"node-asiaeast1-cb-01"`
	MasterIp      string `json:"masterIp" example:"32.201.134.113"`
	MasterSSHPort int    `json:"masterSSHPort"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// SystemLabel is for describing the infra in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	Node []NodeStatusInfo `json:"node"`
}

// ControlNodeResult is struct for result of Node control
type ControlNodeResult struct {
	NodeId string `json:"nodeId"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}

// ControlNodeResultWrapper is struct for array of results of Node control
type ControlNodeResultWrapper struct {
	ResultArray []ControlNodeResult `json:"resultarray"`
}

// NodeStatusInfo is to define simple information of Node with updated status
type NodeStatusInfo struct {

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty" example:"we12fawefadf1221edcf"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty" example:"csp-06eb41e14121c550a"`

	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`
	NativeStatus string `json:"nativeStatus"`

	// Montoring agent status
	MonAgentStatus string `json:"monAgentStatus" example:"[installed, notInstalled, failed]"` // yes or no// installed, notInstalled, failed

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	// Created time
	CreatedTime string `json:"createdTime" example:"2022-11-10 23:00:00" default:""`

	PublicIp  string `json:"publicIp"`
	PrivateIp string `json:"privateIp"`
	SSHPort   int    `json:"sshPort"`

	Location Location `json:"location"`
}

// Status for infra automation
const (
	// AutoStatusReady is const for "Ready" status.
	AutoStatusReady string = "Ready"

	// AutoStatusChecking is const for "Checking" status.
	AutoStatusChecking string = "Checking"

	// AutoStatusDetected is const for "Detected" status.
	AutoStatusDetected string = "Detected"

	// AutoStatusOperating is const for "Operating" status.
	AutoStatusOperating string = "Operating"

	// AutoStatusStabilizing is const for "Stabilizing" status.
	AutoStatusStabilizing string = "Stabilizing"

	// AutoStatusTimeout is const for "Timeout" status.
	AutoStatusTimeout string = "Timeout"

	// AutoStatusError is const for "Failed" status.
	AutoStatusError string = "Failed"

	// AutoStatusSuspended is const for "Suspended" status.
	AutoStatusSuspended string = "Suspended"
)

// Action for infra automation
const (
	// AutoActionScaleOut is const for "ScaleOut" action.
	AutoActionScaleOut string = "ScaleOut"

	// AutoActionScaleIn is const for "ScaleIn" action.
	AutoActionScaleIn string = "ScaleIn"
)

// AutoCondition is struct for Infra auto-control condition.
type AutoCondition struct {
	Metric           string   `json:"metric" example:"cpu"`
	Operator         string   `json:"operator" example:">=" enums:"<,<=,>,>="`
	Operand          float64  `json:"operand" example:"80"`
	EvaluationPeriod int      `json:"evaluationPeriod" example:"10"`
	EvaluationValue  []string `json:"evaluationValue"`
	//InitTime	   string 	  `json:"initTime"`  // to check start of duration
	//Duration	   string 	  `json:"duration"`  // duration for checking
}

// AutoAction is struct for Infra auto-control action.
type AutoAction struct {
	ActionType          string                    `json:"actionType" example:"ScaleOut" enums:"ScaleOut,ScaleIn"`
	NodeGroupDynamicReq CreateNodeGroupDynamicReq `json:"nodeGroupDynamicReq"`

	// PostCommand is field for providing command to Nodes after their creation. example:"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
	PostCommand   InfraCmdReq `json:"postCommand"`
	PlacementAlgo string      `json:"placementAlgo" example:"random"`
}

// Policy is struct for Infra auto-control Policy request that includes AutoCondition, AutoAction, Status.
type Policy struct {
	AutoCondition AutoCondition `json:"autoCondition"`
	AutoAction    AutoAction    `json:"autoAction"`
	Status        string        `json:"status"`
}

// InfraPolicyInfo is struct for Infra auto-control Policy object.
type InfraPolicyInfo struct {
	Name   string   `json:"Name"` //Infra Name (for request)
	Id     string   `json:"Id"`   //Infra Id (generated ID by the Name)
	Policy []Policy `json:"policy"`

	ActionLog   string `json:"actionLog"`
	Description string `json:"description" example:"Description"`
}

// InfraPolicyReq is struct for Infra auto-control Policy Request.
type InfraPolicyReq struct {
	Policy      []Policy `json:"policy"`
	Description string   `json:"description" example:"Description"`
}

// SshDefaultUserName is array for temporal constants
var SshDefaultUserName = []string{"cb-user", "ubuntu", "root", "ec2-user"}

// SSH Command Timeout Constants
const (
	// SSHConnectionTimeoutSeconds is the timeout for establishing SSH connection
	SSHConnectionTimeoutSeconds = 30

	// SSHCommandDefaultTimeoutMinutes is the default timeout for SSH command execution
	SSHCommandDefaultTimeoutMinutes = 30

	// SSHCommandMaxTimeoutMinutes is the maximum allowed timeout for SSH command execution
	SSHCommandMaxTimeoutMinutes = 120

	// SSHCommandMinTimeoutMinutes is the minimum allowed timeout for SSH command execution
	SSHCommandMinTimeoutMinutes = 1
)

// InfraCmdReq is struct for remote command
type InfraCmdReq struct {
	// UserName is the SSH username to use for command execution
	UserName string `json:"userName" example:"cb-user" default:""`

	// Command is the list of commands to execute
	Command []string `json:"command" validate:"required" example:"client_ip=$(echo $SSH_CLIENT | awk '{print $1}'); echo SSH client IP is: $client_ip"`

	// TimeoutMinutes is the timeout for command execution in minutes (default: 30, min: 1, max: 120)
	// If not specified or set to 0, the default timeout (30 minutes) will be used
	TimeoutMinutes int `json:"timeoutMinutes,omitempty" example:"30" default:"30"`
}

// GetEffectiveTimeout returns the effective timeout duration for command execution
// It validates and normalizes the timeout value within allowed bounds
func (req *InfraCmdReq) GetEffectiveTimeout() int {
	if req.TimeoutMinutes <= 0 {
		return SSHCommandDefaultTimeoutMinutes
	}
	if req.TimeoutMinutes < SSHCommandMinTimeoutMinutes {
		return SSHCommandMinTimeoutMinutes
	}
	if req.TimeoutMinutes > SSHCommandMaxTimeoutMinutes {
		return SSHCommandMaxTimeoutMinutes
	}
	return req.TimeoutMinutes
}

// ExecutionTask represents a running or completed command execution task
// This is used for tracking and cancelling long-running SSH command executions
// Status uses CommandExecutionStatus (Queued, Handling, Completed, Failed, Timeout, Cancelled, Interrupted)
type ExecutionTask struct {
	// TaskId is the unique identifier for this execution task (format: xRequestId:nodeId:index)
	TaskId string `json:"taskId" example:"req-12345678:node-01:1"`

	// XRequestId is the X-Request-ID header value, the unique identifier for the request
	XRequestId string `json:"xRequestId,omitempty" example:"req-12345678"`

	// NsId is the namespace ID
	NsId string `json:"nsId" example:"default"`

	// InfraId is the Infra ID
	InfraId string `json:"infraId" example:"infra01"`

	// NodeId is the target Node ID
	NodeId string `json:"nodeId,omitempty" example:"g1-1"`

	// CommandIndex is the index of this command in the Node's command history
	CommandIndex int `json:"commandIndex,omitempty" example:"1"`

	// NodeGroupId is the target nodegroup ID (empty if not specified)
	NodeGroupId string `json:"nodeGroupId,omitempty" example:"g1"`

	// Command is the command being executed
	Command []string `json:"command" example:"apt update && apt install -y docker.io"`

	// Status is the current status of the task (uses CommandExecutionStatus: Queued, Handling, Completed, etc.)
	Status CommandExecutionStatus `json:"status" example:"Handling"`

	// StartedAt is when the task started (RFC3339 format)
	StartedAt string `json:"startedAt" example:"2024-01-15T10:30:00Z"`

	// CompletedAt is when the task completed (RFC3339 format), empty if still running
	CompletedAt string `json:"completedAt,omitempty" example:"2024-01-15T10:35:00Z"`

	// TimeoutMinutes is the timeout setting for this task
	TimeoutMinutes int `json:"timeoutMinutes" example:"30"`

	// ElapsedSeconds is the elapsed time in seconds
	ElapsedSeconds int64 `json:"elapsedSeconds" example:"120"`

	// Message provides additional status information
	Message string `json:"message,omitempty" example:"Executing command on 3 Nodes"`

	// TargetNodeCount is the number of Nodes targeted by this task
	TargetNodeCount int `json:"targetNodeCount" example:"3"`

	// CompletedNodeCount is the number of Nodes that have completed execution
	CompletedNodeCount int `json:"completedNodeCount" example:"1"`
}

// ExecutionTaskListResponse represents the response for execution task list queries
type ExecutionTaskListResponse struct {
	// Tasks is the list of execution tasks
	Tasks []ExecutionTask `json:"tasks"`

	// Total is the total number of tasks
	Total int `json:"total" example:"5"`
}

// CancelTaskRequest represents a request to cancel an execution task
type CancelTaskRequest struct {
	// Reason is an optional reason for cancellation
	Reason string `json:"reason,omitempty" example:"User requested cancellation"`
}

// CancelTaskResponse represents the response after cancelling a task
type CancelTaskResponse struct {
	// TaskId is the cancelled task ID
	TaskId string `json:"taskId" example:"cmd-g1-1-req123-1"`

	// Success indicates whether the cancellation was successful
	Success bool `json:"success" example:"true"`

	// Status is the new status after cancellation (Cancelled)
	Status CommandExecutionStatus `json:"status,omitempty" example:"Cancelled"`

	// Message provides additional information about the cancellation
	Message string `json:"message" example:"Task cancelled successfully"`

	// CancelledAt is when the task was cancelled (RFC3339 format)
	CancelledAt string `json:"cancelledAt,omitempty" example:"2024-01-15T10:32:00Z"`
}

// ExecutionTaskList represents a list of execution tasks
type ExecutionTaskList struct {
	// Tasks is the list of execution tasks
	Tasks []*ExecutionTask `json:"tasks"`
}

// CommandExecutionStatus represents the status of command execution
type CommandExecutionStatus string

const (
	// CommandStatusQueued indicates the command has been requested but not started
	CommandStatusQueued CommandExecutionStatus = "Queued"

	// CommandStatusHandling indicates the command is currently being processed
	CommandStatusHandling CommandExecutionStatus = "Handling"

	// CommandStatusCompleted indicates the command execution completed successfully
	CommandStatusCompleted CommandExecutionStatus = "Completed"

	// CommandStatusFailed indicates the command execution failed
	CommandStatusFailed CommandExecutionStatus = "Failed"

	// CommandStatusTimeout indicates the command execution timed out
	CommandStatusTimeout CommandExecutionStatus = "Timeout"

	// CommandStatusCancelled indicates the command was cancelled by user request
	CommandStatusCancelled CommandExecutionStatus = "Cancelled"

	// CommandStatusInterrupted indicates the command was interrupted (e.g., system restart)
	CommandStatusInterrupted CommandExecutionStatus = "Interrupted"
)

// CommandStatusInfo represents a single remote command execution record
type CommandStatusInfo struct {
	// Index is sequential identifier for this command execution (1, 2, 3, ...)
	Index int `json:"index" example:"1"`

	// XRequestId is the request ID from X-Request-ID header when the command was executed
	XRequestId string `json:"xRequestId,omitempty" example:"req-12345678-abcd-1234-efgh-123456789012"`

	// CommandRequested is the original command as requested by the user
	CommandRequested string `json:"commandRequested" example:"ls -la"`

	// CommandExecuted is the actual SSH command executed on the Node (may be adjusted)
	CommandExecuted string `json:"commandExecuted" example:"ls -la"`

	// Status represents the current status of the command execution
	Status CommandExecutionStatus `json:"status" example:"Completed"`

	// StartedTime is when the command execution started
	StartedTime string `json:"startedTime" example:"2024-01-15 10:30:00" default:""`

	// CompletedTime is when the command execution completed (success or failure)
	CompletedTime string `json:"completedTime,omitempty" example:"2024-01-15 10:30:05"`

	// ElapsedTime is the duration of command execution in seconds
	ElapsedTime int64 `json:"elapsedTime,omitempty" example:"120"`

	// ResultSummary provides a brief summary of the execution result
	ResultSummary string `json:"resultSummary,omitempty" example:"Command executed successfully"`

	// ErrorMessage contains error details if the execution failed
	ErrorMessage string `json:"errorMessage,omitempty" example:"SSH connection failed"`

	// Stdout contains the standard output from command execution (truncated for history)
	Stdout string `json:"stdout,omitempty" example:"total 8\ndrwxr-xr-x 2 user user 4096 Jan 15 10:30 ."`

	// Stderr contains the standard error from command execution (truncated for history)
	Stderr string `json:"stderr,omitempty" example:""`
}

// CommandStatusFilter represents filtering criteria for command status queries
type CommandStatusFilter struct {
	// Status filters by command execution status
	Status []CommandExecutionStatus `json:"status,omitempty" example:"[\"Completed\",\"Failed\"]"`

	// XRequestId filters by specific request ID
	XRequestId string `json:"xRequestId,omitempty" example:"req-12345678-abcd-1234-efgh-123456789012"`

	// CommandContains filters commands containing this text
	CommandContains string `json:"commandContains,omitempty" example:"ls"`

	// StartTimeFrom filters commands started from this time (RFC3339 format)
	StartTimeFrom string `json:"startTimeFrom,omitempty" example:"2024-01-15T10:00:00Z"`

	// StartTimeTo filters commands started until this time (RFC3339 format)
	StartTimeTo string `json:"startTimeTo,omitempty" example:"2024-01-15T11:00:00Z"`

	// IndexFrom filters commands from this index (inclusive)
	IndexFrom int `json:"indexFrom,omitempty" example:"1"`

	// IndexTo filters commands to this index (inclusive)
	IndexTo int `json:"indexTo,omitempty" example:"10"`

	// Limit limits the number of results returned
	Limit int `json:"limit,omitempty" example:"50"`

	// Offset specifies the number of results to skip
	Offset int `json:"offset,omitempty" example:"0"`
}

// CommandStatusListResponse represents the response for command status list queries
type CommandStatusListResponse struct {
	// Commands is the list of command status info matching the filter criteria
	Commands []CommandStatusInfo `json:"commands"`

	// Total is the total number of commands matching the criteria (before limit/offset)
	Total int `json:"total" example:"25"`

	// Limit is the limit applied to the query
	Limit int `json:"limit" example:"50"`

	// Offset is the offset applied to the query
	Offset int `json:"offset" example:"0"`
}

// HandlingCommandCountResponse represents the response for Node handling command count queries
type HandlingCommandCountResponse struct {
	// NodeId is the Node identifier
	NodeId string `json:"nodeId" example:"g1-1"`

	// HandlingCount is the number of commands currently in 'Handling' status
	HandlingCount int `json:"handlingCount" example:"3"`
}

// InfraHandlingCommandCountResponse represents the response for Infra handling command count queries
type InfraHandlingCommandCountResponse struct {
	// InfraId is the Infra identifier
	InfraId string `json:"infraId" example:"infra01"`

	// NodeHandlingCounts is a map of Node ID to handling command count
	NodeHandlingCounts map[string]int `json:"nodeHandlingCounts"`

	// TotalHandlingCount is the total number of handling commands across all Nodes in the Infra
	TotalHandlingCount int `json:"totalHandlingCount" example:"3"`
}

// CommandStreamEventType represents the type of SSE event for command streaming
type CommandStreamEventType string

const (
	// EventCommandStatus is sent when a command's status changes (Queued→Handling→Completed etc.)
	EventCommandStatus CommandStreamEventType = "CommandStatus"

	// EventCommandLog is sent for real-time stdout/stderr log lines during SSH execution
	EventCommandLog CommandStreamEventType = "CommandLog"

	// EventCommandDone is sent when all Nodes have finished execution (terminal event)
	EventCommandDone CommandStreamEventType = "CommandDone"
)

// CommandStreamEvent is a single SSE event sent to streaming clients
type CommandStreamEvent struct {
	// Type indicates the kind of event
	Type CommandStreamEventType `json:"type" example:"CommandLog"`

	// NodeId identifies which Node this event belongs to
	NodeId string `json:"nodeId" example:"g1-1"`

	// CommandIndex is the command status index in NodeInfo.CommandStatus
	CommandIndex int `json:"commandIndex" example:"1"`

	// Timestamp is when the event was generated (RFC3339)
	Timestamp string `json:"timestamp" example:"2024-01-15T10:30:05Z"`

	// Status is populated for EventCommandStatus events (reuses existing CommandStatusInfo)
	Status *CommandStatusInfo `json:"status,omitempty"`

	// Log is populated for EventCommandLog events
	Log *CommandLogEntry `json:"log,omitempty"`

	// Summary is populated for EventCommandDone events
	Summary *CommandDoneSummary `json:"summary,omitempty"`
}

// CommandLogEntry represents a single log line from SSH command execution
type CommandLogEntry struct {
	// Stream indicates the source: "stdout" or "stderr"
	Stream string `json:"stream" example:"stdout"`

	// Line is the log line content (truncated at 4096 chars)
	Line string `json:"line" example:"total 8"`

	// LineNumber is the sequential line number within this stream for this Node
	LineNumber int `json:"lineNumber" example:"1"`
}

// CommandDoneSummary is sent as the final SSE event when all Nodes finish
type CommandDoneSummary struct {
	// TotalNodes is the number of Nodes that were targeted
	TotalNodes int `json:"totalNodes" example:"3"`

	// CompletedNodes is the number of Nodes that completed successfully
	CompletedNodes int `json:"completedNodes" example:"2"`

	// FailedNodes is the number of Nodes that failed
	FailedNodes int `json:"failedNodes" example:"1"`

	// ElapsedSeconds is total wall-clock time for the entire command execution
	ElapsedSeconds int64 `json:"elapsedSeconds" example:"45"`

	// Error is set when the command execution failed before reaching Nodes (e.g., preprocessing error)
	Error string `json:"error,omitempty" example:"built-in function GetPublicIP error: no Node found"`
}

// SshCmdResult is struct for SshCmd Result
type SshCmdResult struct { // Tumblebug
	InfraId string         `json:"infraId"`
	NodeId  string         `json:"nodeId"`
	NodeIp  string         `json:"nodeIp"`
	Command map[int]string `json:"command"`
	Stdout  map[int]string `json:"stdout"`
	Stderr  map[int]string `json:"stderr"`
	Err     error          `json:"err"`
}

// InfraSshCmdResult is struct for Set of SshCmd Results in terms of Infra
type InfraSshCmdResult struct {
	Results []SshCmdResult `json:"results"`
}

// SshCmdResultForAPI is struct for SshCmd Result with string error for API response
type SshCmdResultForAPI struct { // For REST API response
	InfraId string         `json:"infraId"`
	NodeId  string         `json:"nodeId"`
	NodeIp  string         `json:"nodeIp"`
	Command map[int]string `json:"command"`
	Stdout  map[int]string `json:"stdout"`
	Stderr  map[int]string `json:"stderr"`
	Error   string         `json:"error"` // String representation of error for JSON serialization
}

// InfraSshCmdResultForAPI is struct for Set of SshCmd Results in terms of Infra for API response
type InfraSshCmdResultForAPI struct {
	Results []SshCmdResultForAPI `json:"results"`
}

// InfraFileTransferAndCmdResult is struct for combined file transfer and optional command execution result (internal)
type InfraFileTransferAndCmdResult struct {
	FileTransferResults []SshCmdResult `json:"fileTransferResults"`
	CmdResults          []SshCmdResult `json:"cmdResults,omitempty"`
}

// InfraFileTransferAndCmdResultForAPI is the API-friendly version of InfraFileTransferAndCmdResult
type InfraFileTransferAndCmdResultForAPI struct {
	FileTransferResults InfraSshCmdResultForAPI  `json:"fileTransferResults"`
	CmdResults          *InfraSshCmdResultForAPI `json:"cmdResults,omitempty"`
}

// FileDownloadReq is struct for file download request from a Node
type FileDownloadReq struct {
	SourcePath string `json:"sourcePath" validate:"required" example:"/home/cb-user/result.json"` // Full path of the file on the remote VM
}

// SshInfo is struct for ssh info
type SshInfo struct {
	UserName   string // ex) root
	PrivateKey []byte // ex) -----BEGIN RSA PRIVATE KEY-----
	EndPoint   string // ex) node12:22
}

// BastionInfo is struct for bastion info
type BastionInfo struct {
	NodeId []string `json:"nodeId"`
}

// RecommendSpecReq is struct for .
type RecommendSpecReq struct {
	Filter   FilterInfo   `json:"filter"`
	Priority PriorityInfo `json:"priority"`
	Limit    int          `json:"limit" example:"5"`
}

// FilterInfo is struct for .
type FilterInfo struct {
	Policy []FilterCondition `json:"policy"`
}

// FilterCondition is struct for .
type FilterCondition struct {
	Metric    string      `json:"metric" example:"vCPU" enums:"vCPU,memoryGiB,costPerHour"`
	Condition []Operation `json:"condition"`
}

// Operation is struct for .
type Operation struct {
	Operator string `json:"operator" example:"<=" enums:">=,<=,=="` // >=, <=, ==
	Operand  string `json:"operand" example:"4"`                    // 10, 70, 80, 98, ... or string values like "aws", "x86_64"
}

// PriorityInfo is struct for .
type PriorityInfo struct {
	Policy []PriorityCondition `json:"policy"`
}

// FilterCondition is struct for .
type PriorityCondition struct {
	Metric    string            `json:"metric" example:"location" enums:"location,cost,random,performance,latency"`
	Weight    float64           `json:"weight" example:"0.3"`
	Parameter []ParameterKeyVal `json:"parameter,omitempty"`
}

// Operation is struct for .
type ParameterKeyVal struct {
	Key string   `json:"key" example:"coordinateClose" enums:"coordinateClose,coordinateWithin,coordinateFair"` // coordinate
	Val []string `json:"val" example:"44.146838/-116.411403"`                                                   // ["Latitude,Longitude","12,543",..,"31,433"]
}

// SpecBenchmarkInfo is struct for SpecBenchmarkInfo
type SpecBenchmarkInfo struct {
	SpecId     string `json:"specid"`
	Cpus       string `json:"cpus"`
	Cpum       string `json:"cpum"`
	MemR       string `json:"memR"`
	MemW       string `json:"memW"`
	FioR       string `json:"fioR"`
	FioW       string `json:"fioW"`
	DbR        string `json:"dbR"`
	DbW        string `json:"dbW"`
	Rtt        string `json:"rtt"`
	EvaledTime string `json:"evaledTime"`
}

// BenchmarkInfo is struct for BenchmarkInfo
type BenchmarkInfo struct {
	Result      string          `json:"result"`
	Unit        string          `json:"unit"`
	Desc        string          `json:"desc"`
	Elapsed     string          `json:"elapsed"`
	SpecId      string          `json:"specid"`
	RegionName  string          `json:"regionName"`
	ResultArray []BenchmarkInfo `json:"resultarray"` // struct-element cycle ?
}

// BenchmarkInfoArray is struct for BenchmarkInfoArray
type BenchmarkInfoArray struct {
	ResultArray []BenchmarkInfo `json:"resultarray"`
}

// BenchmarkReq is struct for BenchmarkReq
type BenchmarkReq struct {
	Host string `json:"host"`
	Spec string `json:"spec"`
}

// MultihostBenchmarkReq is struct for MultihostBenchmarkReq
type MultihostBenchmarkReq struct {
	Multihost []BenchmarkReq `json:"multihost"`
}

// MilkywayPort is const for MilkywayPort
const MilkywayPort string = ":1324/milkyway/"

// AgentInstallContentWrapper ...
type AgentInstallContentWrapper struct {
	ResultArray []AgentInstallContent `json:"resultArray"`
}

// AgentInstallContent ...
type AgentInstallContent struct {
	InfraId string `json:"infraId"`
	NodeId  string `json:"nodeId"`
	NodeIp  string `json:"nodeIp"`
	Result  string `json:"result"`
}

// ProvisioningLog represents provisioning history for a specific Node spec
type ProvisioningLog struct {
	// SpecId is the Node specification ID
	SpecId string `json:"specId"`

	// ConnectionName is the connection configuration name
	ConnectionName string `json:"connectionName"`

	// ProviderName is the cloud service provider name
	ProviderName string `json:"providerName"`

	// RegionName is the region name
	RegionName string `json:"regionName"`

	// FailureCount is the total number of provisioning failures
	FailureCount int `json:"failureCount"`

	// SuccessCount is the total number of provisioning successes (only recorded if there were previous failures)
	SuccessCount int `json:"successCount"`

	// FailureTimestamps contains list of failure timestamps
	FailureTimestamps []time.Time `json:"failureTimestamps"`

	// SuccessTimestamps contains list of success timestamps (only recorded if there were previous failures)
	SuccessTimestamps []time.Time `json:"successTimestamps"`

	// FailureMessages contains list of failure error messages
	FailureMessages []string `json:"failureMessages"`

	// FailureImages contains list of CSP image names that failed with this spec
	FailureImages []string `json:"failureImages"`

	// SuccessImages contains list of CSP image names that succeeded with this spec (only recorded if there were previous failures)
	SuccessImages []string `json:"successImages"`

	// LastUpdated is the timestamp of the last log update
	LastUpdated time.Time `json:"lastUpdated"`

	// AdditionalInfo contains any additional information about the provisioning attempts
	AdditionalInfo map[string]string `json:"additionalInfo"`
}

// ProvisioningEvent represents a single provisioning event for logging
type ProvisioningEvent struct {
	// SpecId is the Node specification ID
	SpecId string `json:"specId"`

	// CspImageName is the CSP-specific image name used in this provisioning attempt
	CspImageName string `json:"cspImageName"`

	// IsSuccess indicates if the provisioning was successful
	IsSuccess bool `json:"isSuccess"`

	// ErrorMessage contains the error message if provisioning failed
	ErrorMessage string `json:"errorMessage"`

	// Timestamp is when this provisioning event occurred
	Timestamp time.Time `json:"timestamp"`

	// NodeName is the name of the VM that was being provisioned
	NodeName string `json:"nodeName"`

	// InfraId is the Infra ID that this VM belongs to
	InfraId string `json:"infraId"`
}

// RiskAnalysis represents detailed risk analysis for provisioning
type RiskAnalysis struct {
	// SpecRisk contains spec-specific risk analysis
	SpecRisk SpecRiskInfo `json:"specRisk"`

	// ImageRisk contains image-specific risk analysis
	ImageRisk ImageRiskInfo `json:"imageRisk"`

	// OverallRisk contains overall combined risk assessment
	OverallRisk OverallRiskInfo `json:"overallRisk"`

	// Recommendations provides actionable guidance for users
	Recommendations []string `json:"recommendations"`

	// RecentFailureMessages contains recent failure messages for context (up to 5 most recent, unique messages)
	RecentFailureMessages []string `json:"recentFailureMessages,omitempty"`
}

// SpecRiskInfo represents risk analysis specific to the VM specification
type SpecRiskInfo struct {
	// Level is the risk level: "low", "medium", "high"
	Level string `json:"level"`

	// Message explains the spec-specific risk reasoning
	Message string `json:"message"`

	// FailedImageCount is the number of different images that failed with this spec
	FailedImageCount int `json:"failedImageCount"`

	// SucceededImageCount is the number of different images that succeeded with this spec
	SucceededImageCount int `json:"succeededImageCount"`

	// TotalFailures is the total number of failures for this spec
	TotalFailures int `json:"totalFailures"`

	// TotalSuccesses is the total number of successes for this spec
	TotalSuccesses int `json:"totalSuccesses"`

	// FailureRate is the overall failure rate for this spec (0.0 to 1.0)
	FailureRate float64 `json:"failureRate"`
}

// ImageRiskInfo represents risk analysis specific to the image
type ImageRiskInfo struct {
	// Level is the risk level: "low", "medium", "high"
	Level string `json:"level"`

	// Message explains the image-specific risk reasoning
	Message string `json:"message"`

	// HasFailedWithSpec indicates if this image has failed with this spec before
	HasFailedWithSpec bool `json:"hasFailedWithSpec"`

	// HasSucceededWithSpec indicates if this image has succeeded with this spec before
	HasSucceededWithSpec bool `json:"hasSucceededWithSpec"`

	// IsNewCombination indicates if this spec+image combination has never been tried
	IsNewCombination bool `json:"isNewCombination"`
}

// OverallRiskInfo represents the combined risk assessment
type OverallRiskInfo struct {
	// Level is the overall risk level: "low", "medium", "high"
	Level string `json:"level"`

	// Message explains the overall risk reasoning
	Message string `json:"message"`

	// PrimaryRiskFactor indicates what the main risk factor is: "spec", "image", "combination", "none"
	PrimaryRiskFactor string `json:"primaryRiskFactor"`
}
