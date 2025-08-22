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

	// StatusUndefined is const for Undefined
	StatusUndefined string = "Undefined"

	// StatusComplete is const for Complete
	StatusComplete string = "None"
)

// Provisioning failure handling policies
const (
	// PolicyContinue continues with partial MCI creation when some VMs fail
	PolicyContinue string = "continue"

	// PolicyRollback cleans up entire MCI when any VM creation fails
	PolicyRollback string = "rollback"

	// PolicyRefine marks failed VMs for refinement when creation fails
	PolicyRefine string = "refine"
)

const StrAutoGen string = "autogen"

// DefaultSystemLabel is const for string to specify the Default System Label
const DefaultSystemLabel string = "Managed by CB-Tumblebug"

// RegionInfo is struct for region information
type RegionInfo struct {
	Region string
	Zone   string
}

// TbMciReq is struct for requirements to create MCI
type TbMciReq struct {
	Name string `json:"name" validate:"required" example:"mci01"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	PlacementAlgo string `json:"placementAlgo,omitempty"`
	Description   string `json:"description" example:"Made in CB-TB"`

	SubGroups []TbCreateSubGroupReq `json:"subGroups" validate:"required"`

	// PostCommand is for the command to bootstrap the VMs
	PostCommand MciCmdReq `json:"postCommand" validate:"omitempty"`

	// PolicyOnPartialFailure determines how to handle VM creation failures
	// - "continue": Continue with partial MCI creation (default)
	// - "rollback": Cleanup entire MCI when any VM fails
	// - "refine": Mark failed VMs for refinement
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue" default:"continue" enums:"continue,rollback,refine"`
}

// ResourceStatusInfo is struct for status information of a resource
type ResourceStatusInfo struct {
	Status       string `json:"status"`
	TargetStatus string `json:"targetStatus"`
	TargetAction string `json:"targetAction"`
}

// TbMciInfo is struct for MCI info
type TbMciInfo struct {
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

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

	PlacementAlgo string     `json:"placementAlgo,omitempty"`
	Description   string     `json:"description"`
	Vm            []TbVmInfo `json:"vm"`

	// List of IDs for new VMs. Return IDs if the VMs are newly added. This field should be used for return body only.
	NewVmList []string `json:"newVmList"`

	// PostCommand is for the command to bootstrap the VMs
	PostCommand MciCmdReq `json:"postCommand"`

	// PostCommandResult is the result of the command for bootstraping the VMs
	PostCommandResult MciSshCmdResult `json:"postCommandResult"`

	// CreationErrors contains information about VM creation failures (if any)
	CreationErrors *MciCreationErrors `json:"creationErrors,omitempty"`
}

// MciCreationErrors represents errors that occurred during MCI creation
type MciCreationErrors struct {
	// VmObjectCreationErrors contains errors from VM object creation phase
	VmObjectCreationErrors []VmCreationError `json:"vmObjectCreationErrors,omitempty"`

	// VmCreationErrors contains errors from actual VM creation phase
	VmCreationErrors []VmCreationError `json:"vmCreationErrors,omitempty"`

	// TotalVmCount is the total number of VMs that were supposed to be created
	TotalVmCount int `json:"totalVmCount"`

	// SuccessfulVmCount is the number of VMs that were successfully created
	SuccessfulVmCount int `json:"successfulVmCount"`

	// FailedVmCount is the number of VMs that failed to be created
	FailedVmCount int `json:"failedVmCount"`

	// FailureHandlingStrategy indicates how failures were handled
	FailureHandlingStrategy string `json:"failureHandlingStrategy,omitempty"` // "rollback", "refine", "continue"
}

// VmCreationError represents a single VM creation error
type VmCreationError struct {
	// VmName is the name of the VM that failed
	VmName string `json:"vmName"`

	// Error is the error message
	Error string `json:"error"`

	// Phase indicates when the error occurred
	Phase string `json:"phase"` // "object_creation", "vm_creation"

	// Timestamp when the error occurred
	Timestamp string `json:"timestamp"`
}

// TbCreateSubGroupReq is struct to get requirements to create a new server instance
type TbCreateSubGroupReq struct {
	// SubGroup name of VMs. Actual VM name will be generated with -N postfix.
	Name string `json:"name" validate:"required" example:"g1-1"`

	// CspResourceId is resource identifier managed by CSP (required for option=register)
	CspResourceId string `json:"cspResourceId,omitempty" example:"i-014fa6ede6ada0b2c"`

	// if subGroupSize is (not empty) && (> 0), subGroup will be generated. VMs will be created accordingly.
	SubGroupSize string `json:"subGroupSize" example:"3" default:""`

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
	VmUserName       string   `json:"vmUserName,omitempty"`
	VmUserPassword   string   `json:"vmUserPassword,omitempty"`
	RootDiskType     string   `json:"rootDiskType,omitempty" example:"default, TYPE1, ..."`  // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_ssd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize     string   `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]
	DataDiskIds      []string `json:"dataDiskIds"`
}

// TbCreateSubGroupReq is struct to get requirements to create a new server instance
type TbScaleOutSubGroupReq struct {
	// Define addtional VMs to scaleOut
	NumVMsToAdd string `json:"numVMsToAdd" validate:"required" example:"2"`

	//tobe added accoring to new future capability
}

// TbMciDynamicReq is struct for requirements to create MCI dynamically (with default resource option)
type TbMciDynamicReq struct {
	Name string `json:"name" validate:"required" example:"mci01"`

	// PolicyOnPartialFailure determines how to handle VM creation failures
	// - "continue": Continue with partial MCI creation (default)
	// - "rollback": Cleanup entire MCI when any VM fails
	// - "refine": Mark failed VMs for refinement
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue" default:"continue" enums:"continue,rollback,refine"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:no)
	InstallMonAgent string `json:"installMonAgent" example:"no" default:"no" enums:"yes,no"` // yes or no

	// SubGroups is array of VM requests for multi-cloud infrastructure
	// Example: Multiple VM groups across different CSPs
	// [
	//   {
	//     "name": "aws-group",
	//     "subGroupSize": "3",
	//     "specId": "aws+ap-northeast-2+t3.nano",
	//     "imageId": "ami-01f71f215b23ba262",
	//     "rootDiskSize": "50",
	//     "label": {"role": "worker", "csp": "aws"}
	//   },
	//   {
	//     "name": "azure-group",
	//     "subGroupSize": "2",
	//     "specId": "azure+koreasouth+standard_b1s",
	//     "imageId": "Canonical:0001-com-ubuntu-server-jammy:22_04-lts:22.04.202505210",
	//     "rootDiskSize": "50",
	//     "label": {"role": "head", "csp": "azure"}
	//   },
	//   {
	//     "name": "gcp-group",
	//     "subGroupSize": "1",
	//     "specId": "gcp+asia-northeast3+g1-small",
	//     "imageId": "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/ubuntu-2204-jammy-v20250712",
	//     "rootDiskSize": "50",
	//     "label": {"role": "test", "csp": "gcp"}
	//   }
	// ]
	SubGroups []TbCreateSubGroupDynamicReq `json:"subGroups" validate:"required"`

	// PostCommand is for the command to bootstrap the VMs
	PostCommand MciCmdReq `json:"postCommand"`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"" default:""`

	Description string `json:"description" example:"Made in CB-TB"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`
}

// TbCreateSubGroupDynamicReq is struct to get requirements to create a new server instance dynamically (with default resource option)
type TbCreateSubGroupDynamicReq struct {
	// SubGroup name, actual VM name will be generated with -N postfix.
	Name string `json:"name" example:"g1"`

	// if subGroupSize is (not empty) && (> 0), subGroup will be generated. VMs will be created accordingly.
	SubGroupSize string `json:"subGroupSize" example:"3" default:"1"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label" example:"{\"role\":\"worker\",\"env\":\"test\"}"`

	Description string `json:"description" example:"Created via CB-Tumblebug"`

	// SpecId is field for id of a spec in common namespace
	SpecId string `json:"specId" validate:"required" example:"aws+ap-northeast-2+t3.nano"`
	// ImageId is field for id of a image in common namespace
	ImageId string `json:"imageId" validate:"required" example:"ami-01f71f215b23ba262"`

	RootDiskType string `json:"rootDiskType,omitempty" example:"gp3" default:"default"` // "", "default", "TYPE1", AWS: ["standard", "gp2", "gp3"], Azure: ["PremiumSSD", "StandardSSD", "StandardHDD"], GCP: ["pd-standard", "pd-balanced", "pd-ssd", "pd-extreme"], ALIBABA: ["cloud_efficiency", "cloud", "cloud_essd"], TENCENT: ["CLOUD_PREMIUM", "CLOUD_SSD"]
	RootDiskSize string `json:"rootDiskSize,omitempty" example:"50" default:"default"`  // "default", Integer (GB): ["50", ..., "1000"]

	VmUserPassword string `json:"vmUserPassword,omitempty" example:"" default:""`
	// if ConnectionName is given, the VM tries to use associtated credential.
	// if not, it will use predefined ConnectionName in Spec objects
	ConnectionName string `json:"connectionName,omitempty" example:"aws-ap-northeast-2" default:""`
}

// MciConnectionConfigCandidatesReq is struct for a request to check requirements to create a new MCI instance dynamically (with default resource option)
type MciConnectionConfigCandidatesReq struct {
	// SpecId is field for id of a spec in common namespace
	SpecIds []string `json:"specId" validate:"required" example:"aws+ap-northeast-2+t2.small,gcp+us-west1+g1-small"`
}

// CheckMciDynamicReqInfo is struct to check requirements to create a new MCI instance dynamically (with default resource option)
type CheckMciDynamicReqInfo struct {
	ReqCheck []CheckVmDynamicReqInfo `json:"reqCheck" validate:"required"`
}

// CheckVmDynamicReqInfo is struct to check requirements to create a new server instance dynamically (with default resource option)
type CheckVmDynamicReqInfo struct {

	// ConnectionConfigCandidates will provide ConnectionConfig options
	ConnectionConfigCandidates []string `json:"connectionConfigCandidates" default:""`

	//RootDiskSize string `json:"rootDiskSize,omitempty" example:"default, 30, 42, ..."` // "default", Integer (GB): ["50", ..., "1000"]

	Spec   TbSpecInfo    `json:"spec" default:""`
	Image  []TbImageInfo `json:"image" default:""`
	Region RegionDetail  `json:"region" default:""`

	// Latest system message such as error message
	SystemMessage string `json:"systemMessage" example:"Failed because ..." default:""` // systeam-given string message

}

// ReviewMciDynamicReqInfo is struct for review result of MCI dynamic request
type ReviewMciDynamicReqInfo struct {
	// Overall assessment of the MCI request
	OverallStatus  string `json:"overallStatus" example:"Ready/Warning/Error"`
	OverallMessage string `json:"overallMessage" example:"All VMs can be created successfully"`
	CreationViable bool   `json:"creationViable"`
	EstimatedCost  string `json:"estimatedCost,omitempty" example:"$0.50/hour"`

	// MCI-level information
	MciName      string `json:"mciName"`
	TotalVmCount int    `json:"totalVmCount"`

	// Failure policy analysis
	PolicyOnPartialFailure string `json:"policyOnPartialFailure" example:"continue"`
	PolicyDescription      string `json:"policyDescription" example:"If some VMs fail during creation, MCI will be created with successfully provisioned VMs only"`
	PolicyRecommendation   string `json:"policyRecommendation,omitempty" example:"Consider 'refine' policy for automatic cleanup of failed VMs"`

	// VM-level validation results
	VmReviews []ReviewVmDynamicReqInfo `json:"vmReviews"`

	// Resource availability summary
	ResourceSummary ReviewResourceSummary `json:"resourceSummary"`

	// Recommendations for improvement
	Recommendations []string `json:"recommendations,omitempty"`
}

// ReviewVmDynamicReqInfo is struct for review result of individual VM in MCI dynamic request
type ReviewVmDynamicReqInfo struct {
	// VM request information
	VmName       string `json:"vmName"`
	SubGroupSize string `json:"subGroupSize"`

	// Validation status
	Status    string `json:"status" example:"Ready/Warning/Error"`
	Message   string `json:"message" example:"VM can be created successfully"`
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
// SpiderVMReqInfo is struct from CB-Spider for VM request information
type SpiderVMReqInfo struct {
	// Fields for request
	Name               string
	ImageName          string
	VPCName            string
	SubnetName         string
	SecurityGroupNames []string
	KeyPairName        string
	CSPid              string // VM ID given by CSP (required for registering VM)
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
	RootDiskName      string // "/dev/sda1", ...
	SSHAccessPoint    string
	KeyValueList      []KeyValue
}

// TbSubGroupInfo is struct to define an object that includes homogeneous VMs
type TbSubGroupInfo struct {
	// ResourceType is the type of the resource
	ResourceType string `json:"resourceType"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"aws-ap-southeast-1"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// Name is human-readable string to represent the object
	Name string `json:"name" example:"aws-ap-southeast-1"`

	VmId         []string `json:"vmId"`
	SubGroupSize string   `json:"subGroupSize"`
}

// MciAssociatedResourceList is struct for associated resource IDs of an MCI
type MciAssociatedResourceList struct {
	ConnectionNames []string `json:"connectionNames"`
	ProviderNames   []string `json:"providerNames"`

	SubGroupIds []string `json:"subGroupIds"`
	VmIds       []string `json:"vmIds"`
	CspVmNames  []string `json:"cspVmNames"`
	CspVmIds    []string `json:"cspVmIds"`
	ImageIds    []string `json:"imageIds"`
	SpecIds     []string `json:"specIds"`

	VNetIds          []string `json:"vNetIds"`
	CspVNetIds       []string `json:"cspVNetIds"`
	SubnetIds        []string `json:"subnetIds"`
	CspSubnetIds     []string `json:"cspSubnetIds"`
	SecurityGroupIds []string `json:"securityGroupIds"`
	DataDiskIds      []string `json:"dataDiskIds"`
	SSHKeyIds        []string `json:"sshKeyIds"`
}

type TbVmInfo struct {
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

	// defined if the VM is in a group
	SubGroupId string `json:"subGroupId"`

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

	Region       RegionInfo `json:"region"` // AWS, ex) {us-east1, us-east1-c} or {ap-northeast-2}
	PublicIP     string     `json:"publicIP"`
	SSHPort      string     `json:"sshPort"`
	PublicDNS    string     `json:"publicDNS"`
	PrivateIP    string     `json:"privateIP"`
	PrivateDNS   string     `json:"privateDNS"`
	RootDiskType string     `json:"rootDiskType"`
	RootDiskSize string     `json:"rootDiskSize"`
	RootDiskName string     `json:"rootDiskName"`

	ConnectionName   string     `json:"connectionName"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`
	SpecId           string     `json:"specId"`
	CspSpecName      string     `json:"cspSpecName"`
	ImageId          string     `json:"imageId"`
	CspImageName     string     `json:"cspImageName"`
	VNetId           string     `json:"vNetId"`
	CspVNetId        string     `json:"cspVNetId"`
	SubnetId         string     `json:"subnetId"`
	CspSubnetId      string     `json:"cspSubnetId"`
	NetworkInterface string     `json:"networkInterface"`
	SecurityGroupIds []string   `json:"securityGroupIds"`
	DataDiskIds      []string   `json:"dataDiskIds"`
	SshKeyId         string     `json:"sshKeyId"`
	CspSshKeyId      string     `json:"cspSshKeyId"`
	VmUserName       string     `json:"vmUserName,omitempty"`
	VmUserPassword   string     `json:"vmUserPassword,omitempty"`

	AddtionalDetails []KeyValue `json:"addtionalDetails,omitempty"`
}

// MciAccessInfo is struct to retrieve overall access information of a MCI
type MciAccessInfo struct {
	MciId                 string
	MciNlbListener        *MciAccessInfo `json:"mciNlbListener,omitempty"`
	MciSubGroupAccessInfo []MciSubGroupAccessInfo
}

// MciSubGroupAccessInfo is struct for MciSubGroupAccessInfo
type MciSubGroupAccessInfo struct {
	SubGroupId      string
	NlbListener     *TbNLBListenerInfo `json:"nlbListener,omitempty"`
	BastionVmId     string
	MciVmAccessInfo []MciVmAccessInfo
}

// MciVmAccessInfo is struct for MciVmAccessInfo
type MciVmAccessInfo struct {
	VmId             string     `json:"vmId"`
	PublicIP         string     `json:"publicIP"`
	PrivateIP        string     `json:"privateIP"`
	SSHPort          string     `json:"sshPort"`
	PrivateKey       string     `json:"privateKey,omitempty"`
	VmUserName       string     `json:"vmUserName,omitempty"`
	VmUserPassword   string     `json:"vmUserPassword,omitempty"`
	ConnectionConfig ConnConfig `json:"connectionConfig"`
}

// TbVmIdNameInDetailInfo is struct for details related with ID and Name
type TbIdNameInDetailInfo struct {
	IdInTb    string `json:"idInTb"`
	IdInSp    string `json:"idInSp"`
	IdInCsp   string `json:"idInCsp"`
	NameInCsp string `json:"nameInCsp"`
}

// StatusCountInfo is struct to count the number of VMs in each status. ex: Running=4, Suspended=8.
type StatusCountInfo struct {

	// CountTotal is for Total VMs
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

	// CountUndefined is for counting Undefined
	CountUndefined int `json:"countUndefined"`
}

// MciRecommendReq is struct for MciRecommendReq
type MciRecommendReq struct {
	VmReq          []TbVmRecommendReq `json:"vmReq"`
	PlacementAlgo  string             `json:"placementAlgo"`
	PlacementParam []KeyValue         `json:"placementParam"`
	MaxResultNum   string             `json:"maxResultNum"`
}

// TbVmRecommendReq is struct for TbVmRecommendReq
type TbVmRecommendReq struct {
	RequestName  string `json:"requestName"`
	MaxResultNum string `json:"maxResultNum"`

	VcpuSize   string `json:"vcpuSize"`
	MemorySize string `json:"memorySize"`
	DiskSize   string `json:"diskSize"`
	//Disk_type   string `json:"disk_type"`

	PlacementAlgo  string     `json:"placementAlgo"`
	PlacementParam []KeyValue `json:"placementParam"`
}

// TbVmPriority is struct for TbVmPriority
type TbVmPriority struct {
	Priority string     `json:"priority"`
	VmSpec   TbSpecInfo `json:"vmSpec"`
}

// TbVmRecommendInfo is struct for TbVmRecommendInfo
type TbVmRecommendInfo struct {
	VmReq          TbVmRecommendReq `json:"vmReq"`
	VmPriority     []TbVmPriority   `json:"vmPriority"`
	PlacementAlgo  string           `json:"placementAlgo"`
	PlacementParam []KeyValue       `json:"placementParam"`
}

// MciStatusInfo is struct to define simple information of MCI with updated status of all VMs
type MciStatusInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`

	Status       string          `json:"status"`
	StatusCount  StatusCountInfo `json:"statusCount"`
	TargetStatus string          `json:"targetStatus"`
	TargetAction string          `json:"targetAction"`

	// InstallMonAgent Option for CB-Dragonfly agent installation ([yes/no] default:yes)
	InstallMonAgent string `json:"installMonAgent" example:"[yes, no]"` // yes or no

	MasterVmId    string `json:"masterVmId" example:"vm-asiaeast1-cb-01"`
	MasterIp      string `json:"masterIp" example:"32.201.134.113"`
	MasterSSHPort string `json:"masterSSHPort"`

	// Label is for describing the object by keywords
	Label map[string]string `json:"label"`

	// SystemLabel is for describing the mci in a keyword (any string can be used) for special System purpose
	SystemLabel string `json:"systemLabel" example:"Managed by CB-Tumblebug" default:""`

	Vm []TbVmStatusInfo `json:"vm"`
}

// ControlVmResult is struct for result of VM control
type ControlVmResult struct {
	VmId   string `json:"vmId"`
	Status string `json:"Status"`
	Error  error  `json:"Error"`
}

// ControlVmResultWrapper is struct for array of results of VM control
type ControlVmResultWrapper struct {
	ResultArray []ControlVmResult `json:"resultarray"`
}

// TbVmStatusInfo is to define simple information of VM with updated status
type TbVmStatusInfo struct {

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
	SSHPort   string `json:"sshPort"`

	Location Location `json:"location"`
}

// Status for mci automation
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

// Action for mci automation
const (
	// AutoActionScaleOut is const for "ScaleOut" action.
	AutoActionScaleOut string = "ScaleOut"

	// AutoActionScaleIn is const for "ScaleIn" action.
	AutoActionScaleIn string = "ScaleIn"
)

// AutoCondition is struct for MCI auto-control condition.
type AutoCondition struct {
	Metric           string   `json:"metric" example:"cpu"`
	Operator         string   `json:"operator" example:">=" enums:"<,<=,>,>="`
	Operand          string   `json:"operand" example:"80"`
	EvaluationPeriod string   `json:"evaluationPeriod" example:"10"`
	EvaluationValue  []string `json:"evaluationValue"`
	//InitTime	   string 	  `json:"initTime"`  // to check start of duration
	//Duration	   string 	  `json:"duration"`  // duration for checking
}

// AutoAction is struct for MCI auto-control action.
type AutoAction struct {
	ActionType   string                     `json:"actionType" example:"ScaleOut" enums:"ScaleOut,ScaleIn"`
	VmDynamicReq TbCreateSubGroupDynamicReq `json:"vmDynamicReq"`

	// PostCommand is field for providing command to VMs after its creation. example:"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
	PostCommand   MciCmdReq `json:"postCommand"`
	PlacementAlgo string    `json:"placementAlgo" example:"random"`
}

// Policy is struct for MCI auto-control Policy request that includes AutoCondition, AutoAction, Status.
type Policy struct {
	AutoCondition AutoCondition `json:"autoCondition"`
	AutoAction    AutoAction    `json:"autoAction"`
	Status        string        `json:"status"`
}

// MciPolicyInfo is struct for MCI auto-control Policy object.
type MciPolicyInfo struct {
	Name   string   `json:"Name"` //MCI Name (for request)
	Id     string   `json:"Id"`   //MCI Id (generated ID by the Name)
	Policy []Policy `json:"policy"`

	ActionLog   string `json:"actionLog"`
	Description string `json:"description" example:"Description"`
}

// MciPolicyReq is struct for MCI auto-control Policy Request.
type MciPolicyReq struct {
	Policy      []Policy `json:"policy"`
	Description string   `json:"description" example:"Description"`
}

// SshDefaultUserName is array for temporal constants
var SshDefaultUserName = []string{"cb-user", "ubuntu", "root", "ec2-user"}

// MciCmdReq is struct for remote command
type MciCmdReq struct {
	UserName string   `json:"userName" example:"cb-user" default:""`
	Command  []string `json:"command" validate:"required" example:"client_ip=$(echo $SSH_CLIENT | awk '{print $1}'); echo SSH client IP is: $client_ip"`
}

// SshCmdResult is struct for SshCmd Result
type SshCmdResult struct { // Tumblebug
	MciId   string         `json:"mciId"`
	VmId    string         `json:"vmId"`
	VmIp    string         `json:"vmIp"`
	Command map[int]string `json:"command"`
	Stdout  map[int]string `json:"stdout"`
	Stderr  map[int]string `json:"stderr"`
	Err     error          `json:"err"`
}

// MciSshCmdResult is struct for Set of SshCmd Results in terms of MCI
type MciSshCmdResult struct {
	Results []SshCmdResult `json:"results"`
}

// SshInfo is struct for ssh info
type SshInfo struct {
	UserName   string // ex) root
	PrivateKey []byte // ex) -----BEGIN RSA PRIVATE KEY-----
	EndPoint   string // ex) node12:22
}

// BastionInfo is struct for bastion info
type BastionInfo struct {
	VmId []string `json:"vmId"`
}

// RecommendSpecReq is struct for .
type RecommendSpecReq struct {
	Filter   FilterInfo   `json:"filter"`
	Priority PriorityInfo `json:"priority"`
	Limit    string       `json:"limit" example:"5" enums:"1,2,30"`
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
	Operand  string `json:"operand" example:"4" enums:"4,8,.."`     // 10, 70, 80, 98, ...
}

// PriorityInfo is struct for .
type PriorityInfo struct {
	Policy []PriorityCondition `json:"policy"`
}

// FilterCondition is struct for .
type PriorityCondition struct {
	Metric    string            `json:"metric" example:"location" enums:"location,cost,random,performance,latency"`
	Weight    string            `json:"weight" example:"0.3" enums:"0.1,0.2,..."`
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
	MciId  string `json:"mciId"`
	VmId   string `json:"vmId"`
	VmIp   string `json:"vmIp"`
	Result string `json:"result"`
}

// ProvisioningLog represents provisioning history for a specific VM spec
type ProvisioningLog struct {
	// SpecId is the VM specification ID
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
	// SpecId is the VM specification ID
	SpecId string `json:"specId"`

	// CspImageName is the CSP-specific image name used in this provisioning attempt
	CspImageName string `json:"cspImageName"`

	// IsSuccess indicates if the provisioning was successful
	IsSuccess bool `json:"isSuccess"`

	// ErrorMessage contains the error message if provisioning failed
	ErrorMessage string `json:"errorMessage"`

	// Timestamp is when this provisioning event occurred
	Timestamp time.Time `json:"timestamp"`

	// VmName is the name of the VM that was being provisioned
	VmName string `json:"vmName"`

	// MciId is the MCI ID that this VM belongs to
	MciId string `json:"mciId"`
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
