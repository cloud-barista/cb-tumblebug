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

// Package model defines data structures for Tumblebug
package model

// StorageSizeRange defines the minimum and maximum storage capacity in Tumblebug format (lowerCamelCase)
type StorageSizeRange struct {
	Min int `json:"min" example:"10"`
	Max int `json:"max" example:"1000"`
}

// StorageTypeNote provides user-facing guidance for a storage type, derived from assets/rdbmsinfo.yaml.
type StorageTypeNote struct {
	StorageType         string            `json:"storageType" example:"gp3"`
	DisplayName         string            `json:"displayName" example:"General Purpose SSD v3"`
	Description         string            `json:"description" example:"Cost-effective, 3000 baseline IOPS, recommended for general workloads"`
	MinSize             int               `json:"minSize,omitempty" example:"100"`
	MaxSize             int               `json:"maxSize,omitempty" example:"65536"`
	RequiresIops        bool              `json:"requiresIops,omitempty" example:"true"`
	IopsRange           *StorageSizeRange `json:"iopsRange,omitempty"`
	Recommended         bool              `json:"recommended,omitempty" example:"true"`
	RecommendationLevel string            `json:"recommendationLevel,omitempty" example:"recommended" enums:"legacy,standard,recommended,premium"`
	CompatibleSpecs     []string          `json:"compatibleSpecs,omitempty" example:"rds.mysql.*"`
	IncompatibleSpecs   []string          `json:"incompatibleSpecs,omitempty" example:"mysql.n4.*"`
	Constraints         string            `json:"constraints,omitempty" example:"Requires 'iops' parameter (e.g., '3000')"`
}

// RDBMSInfoConfig mirrors assets/rdbmsinfo.yaml, static per-CSP reference data not carried by CB-Spider's live RDBMSMetaInfo.
type RDBMSInfoConfig struct {
	DBMS map[string]RDBMSProviderConfig `yaml:"dbms"`
}

// RDBMSProviderConfig is one CSP's entry under "dbms" in assets/rdbmsinfo.yaml.
type RDBMSProviderConfig struct {
	Description string `yaml:"description"`
	// Links holds this CSP's reference URLs by label (e.g. "docs", "console") — never exposed via the Tumblebug API.
	Links map[string]string `yaml:"links,omitempty"`
	Note  string            `yaml:"note,omitempty"`
	// SupportedDBEngines lists DB engines empirically verified for this CSP; omission means unverified, not unsupported.
	SupportedDBEngines []string `yaml:"supportedDBEngines,omitempty"`
	// DBOperationMethod is the fixed method CB-Spider uses for this CSP's internal Database CRUD API: "cspNativeApi" or "sqlFallback".
	DBOperationMethod string `yaml:"dbOperationMethod,omitempty" enums:"cspNativeApi,sqlFallback"`
	// SupportsTag is a static reference value; the live, per-connection flag is RDBMSMetaInfo.SupportsTag.
	SupportsTag bool `yaml:"supportsTag,omitempty"`
	// StorageTypeSelectable is a static reference for whether this CSP lets a caller choose a storage type; the live flag is RDBMSMetaInfo.SupportsStorageTypeSelection.
	StorageTypeSelectable bool                              `yaml:"storageTypeSelectable"`
	StorageTypes          map[string]RDBMSStorageTypeConfig `yaml:"storageTypes"`
	DBMSRequirements      map[string]RDBMSDBMSRequirement   `yaml:"dbmsRequirements,omitempty"`
	DatabaseRequirements  RDBMSDatabaseRequirement          `yaml:"databaseRequirements,omitempty"`
	// AdminUserNameRequirement/AdminUserPasswordRequirement capture CSP-level admin credential constraints (e.g. Tencent forces "root"), enforced by validateAdminCredentials.
	AdminUserNameRequirement     *RDBMSAdminUserNameRequirement     `yaml:"adminUserNameRequirement,omitempty"`
	AdminUserPasswordRequirement *RDBMSAdminUserPasswordRequirement `yaml:"adminUserPasswordRequirement,omitempty"`
}

// RDBMSAdminUserNameRequirement is a CSP's constraint on RDBMSCreateRequest.AdminUserName,
// from assets/rdbmsinfo.yaml's "adminUserNameRequirement".
type RDBMSAdminUserNameRequirement struct {
	// FixedValue, if set, is the only value this CSP accepts (e.g. Tencent: "root").
	FixedValue string `yaml:"fixedValue,omitempty"`
	// ReservedValues are values this CSP rejects (case-insensitive), confirmed via live CSP rejection — not necessarily exhaustive.
	ReservedValues []string `yaml:"reservedValues,omitempty"`
	Note           string   `yaml:"note,omitempty"`
}

// RDBMSAdminUserPasswordRequirement is a CSP's constraint on AdminUserPassword, from assets/rdbmsinfo.yaml's "adminUserPasswordRequirement".
type RDBMSAdminUserPasswordRequirement struct {
	MinLength           int    `yaml:"minLength,omitempty"`
	MaxLength           int    `yaml:"maxLength,omitempty"`
	RequiresSpecialChar bool   `yaml:"requiresSpecialChar,omitempty"`
	ForbidsSpecialChar  bool   `yaml:"forbidsSpecialChar,omitempty"`
	Note                string `yaml:"note,omitempty"`
}

// RDBMSStorageTypeConfig is one storage type entry under a CSP's "storageTypes" in
// assets/rdbmsinfo.yaml.
type RDBMSStorageTypeConfig struct {
	Name                    string            `yaml:"name"`
	Description             string            `yaml:"description"`
	RecommendationLevel     string            `yaml:"recommendationLevel"` // legacy|standard|recommended|premium
	RequiresIops            bool              `yaml:"requiresIops"`
	MinStorageSize          int               `yaml:"minStorageSize,omitempty"`
	MaxStorageSize          int               `yaml:"maxStorageSize,omitempty"`
	IopsRange               *StorageSizeRange `yaml:"iopsRange,omitempty"`
	CompatibleSpecs         []string          `yaml:"compatibleSpecs,omitempty"`
	IncompatibleSpecs       []string          `yaml:"incompatibleSpecs,omitempty"`
	CompatibleMachineSeries []string          `yaml:"compatibleMachineSeries,omitempty"`
	Note                    string            `yaml:"note,omitempty"`
}

// RDBMSDBMSRequirement is one DB-engine entry under a CSP's "dbmsRequirements" in
// assets/rdbmsinfo.yaml.
type RDBMSDBMSRequirement struct {
	MinStorageSize int    `yaml:"minStorageSize,omitempty"`
	MaxStorageSize int    `yaml:"maxStorageSize,omitempty"`
	DefaultPort    int    `yaml:"defaultPort,omitempty"`
	Note           string `yaml:"note,omitempty"`
	// ReferenceEngineVersion is CB-Spider's own test-verified engine version for this CSP/engine, preferred over guessing from the live SupportedVersions list.
	ReferenceEngineVersion string `yaml:"referenceEngineVersion,omitempty"`
	// ReferenceDBInstanceSpec is CB-Spider's own test-verified dbInstanceSpec for this CSP/engine, preferred over the live /dbspec catalog's "smallest" pick.
	ReferenceDBInstanceSpec string `yaml:"referenceDBInstanceSpec,omitempty"`
	ReferenceDBSpec         string `yaml:"referenceDBSpec,omitempty"`
	// DeprecatedVersions lists engine versions that are deprecated by the CSP and discouraged for new deployments.
	DeprecatedVersions []string `yaml:"deprecatedVersions,omitempty" json:"deprecatedVersions,omitempty"`
	// EndOfLifeVersions lists engine versions that have reached official End of Life (EOL) and cannot be provisioned.
	EndOfLifeVersions []string `yaml:"endOfLifeVersions,omitempty" json:"endOfLifeVersions,omitempty"`
}

// RDBMSDatabaseRequirement is a CSP's "databaseRequirements" entry in assets/rdbmsinfo.yaml.
type RDBMSDatabaseRequirement struct {
	MaxDatabaseNameLength int      `yaml:"maxDatabaseNameLength,omitempty"`
	ReservedDatabaseNames []string `yaml:"reservedDatabaseNames,omitempty"`
	Note                  string   `yaml:"note,omitempty"`
}

// RDBMSMetaInfo represents Tumblebug-style RDBMS support capability details
type RDBMSMetaInfo struct {
	ProviderName                     string           `json:"providerName" example:"aws"`
	RegionName                       string           `json:"regionName" example:"ap-northeast-2"`
	ConnectionName                   string           `json:"connectionName" example:"aws-ap-northeast-2-config"`
	DBEngine                         string           `json:"dbEngine" example:"mysql"`
	SupportedVersions                []string         `json:"supportedVersions" example:"8.0,8.4"`
	DBInstanceSpecOptions            []string         `json:"dbInstanceSpecOptions" example:"db.t3.medium"`
	StorageTypeOptions               []string         `json:"storageTypeOptions" example:"gp2,gp3"`
	StorageSizeRange                 StorageSizeRange `json:"storageSizeRange"`
	SupportsHighAvailability         bool             `json:"supportsHighAvailability" example:"true"`
	SupportsBackup                   bool             `json:"supportsBackup" example:"true"`
	BackupRetentionRange             string           `json:"backupRetentionRange" example:"1-35"`
	SupportsPublicAccess             bool             `json:"supportsPublicAccess" example:"true"`
	SupportsDeletionProtection       bool             `json:"supportsDeletionProtection" example:"true"`
	SupportsEncryption               bool             `json:"supportsEncryption" example:"true"`
	SupportsStorageTypeSelection     bool             `json:"supportsStorageTypeSelection" example:"true"`
	SupportsStorageSizeConfiguration bool             `json:"supportsStorageSizeConfiguration" example:"true"`
	SupportsTag                      bool             `json:"supportsTag" example:"true"`
	RequiresSubnet                   bool             `json:"requiresSubnet" example:"true"`
	RequiresSecurityGroup            bool             `json:"requiresSecurityGroup" example:"true"`

	// DBInstanceSpecs is the richer per-option spec catalog from live GET /dbspec (superset of DBInstanceSpecOptions); best-effort, empty if the live call failed.
	DBInstanceSpecs []RDBMSDBInstanceSpecInfo `json:"dbInstanceSpecs,omitempty"`
	// LiveSupportedEngines is this connection's live-verified engine list from GET /rdbmsengine, independent of the DBEngine queried; best-effort.
	LiveSupportedEngines []string `json:"liveSupportedEngines,omitempty"`

	// Notes carries Tumblebug's own advisory annotations (storage type guidance, static/approximate fields), unlike the live fields above.
	Notes RDBMSNotes `json:"notes,omitempty"`
}

// RDBMSDBInstanceSpecInfo is one instance spec option from live GET /dbspec, kept lean (no DataSource/KeyValueList).
type RDBMSDBInstanceSpecInfo struct {
	Name               string           `json:"name" example:"db.t3.medium"`
	VCpuCount          string           `json:"vCpuCount,omitempty" example:"2"`
	VCpuClockGHz       string           `json:"vCpuClockGHz,omitempty" example:"2.5"`
	MemSizeMiB         string           `json:"memSizeMiB,omitempty" example:"4096"`
	StorageSizeRangeGB StorageSizeRange `json:"storageSizeRangeGB,omitempty"`
}

// RDBMSNotes groups Tumblebug's advisory annotations for one RDBMSMetaInfo response.
type RDBMSNotes struct {
	// StorageTypes is per-storage-type guidance (display name, description, constraints,
	// recommendation) for each entry in StorageTypeOptions above; see buildStorageTypeNotes.
	StorageTypes []StorageTypeNote `json:"storageTypes,omitempty"`
	// StaticFields lists capability fields whose values are static/approximate rather than live;
	// see buildRDBMSStaticFields.
	StaticFields []StaticFieldNote `json:"staticFields,omitempty"`
}

// StaticFieldNote flags one RDBMSMetaInfo field whose value is fixed/approximate rather than
// live from CB-Spider, per GET /rdbmsmetainfo's DataSource/DataSourceNotes.
type StaticFieldNote struct {
	Field string `json:"field" example:"storageSizeRange"`
	Note  string `json:"note" example:"Static fallback value (CB-Spider does not query this live)"`
}

// RDBMSCapabilityResponse wraps the Tumblebug API response for GET /rdbms/capability.
type RDBMSCapabilityResponse struct {
	ResourceType string        `json:"resourceType" example:"rdbms"`
	Supports     RDBMSMetaInfo `json:"supports"`
}

// RDBMSSupportResponse wraps the Tumblebug API response for GET /rdbms/support (static reference matrix).
type RDBMSSupportResponse struct {
	ResourceType string                         `json:"resourceType" example:"rdbms"`
	Supports     map[string]RDBMSCSPSupportInfo `json:"supports"`
}

// RDBMSCSPSupportInfo is one CSP's entry in GET /rdbms/support, derived statically from assets/rdbmsinfo.yaml.
type RDBMSCSPSupportInfo struct {
	Supported             bool     `json:"supported" example:"true"`
	SupportedDBEngines    []string `json:"supportedDBEngines,omitempty" example:"mysql,mariadb"`
	DBOperationMethod     string   `json:"dbOperationMethod,omitempty" example:"cspNativeApi" enums:"cspNativeApi,sqlFallback"`
	SupportsTag           bool     `json:"supportsTag,omitempty" example:"true"`
	StorageTypeSelectable bool     `json:"storageTypeSelectable" example:"true"`
	Note                  string   `json:"note,omitempty" example:"Storage type selection is not supported on this CSP."`
}

// RDBMSCreateRequest is the Tumblebug-facing request to create an RDBMS instance.
type RDBMSCreateRequest struct {
	Name             string   `json:"name" validate:"required" example:"rdbms-01"`
	ConnectionName   string   `json:"connectionName" validate:"required" example:"aws-ap-northeast-2"`
	VNetId           string   `json:"vNetId" validate:"required" example:"vnet-01"`
	SubnetIds        []string `json:"subnetIds,omitempty" example:"subnet-01"`
	SecurityGroupIds []string `json:"securityGroupIds,omitempty" example:"sg-01"`
	DBEngine         string   `json:"dbEngine" validate:"required" example:"mysql" enums:"mysql,mariadb"`
	// DBEngineVersion may be left empty when AutoFillDefaults is true.
	DBEngineVersion string `json:"dbEngineVersion,omitempty" example:"8.0"`
	// DBInstanceSpec may be left empty when AutoFillDefaults is true.
	DBInstanceSpec string `json:"dbInstanceSpec,omitempty" example:"db.t3.medium"`
	StorageType    string `json:"storageType,omitempty" example:"gp3"`
	// StorageSize may be left as 0 when AutoFillDefaults is true.
	StorageSize         int    `json:"storageSize,omitempty" example:"100"`
	Iops                string `json:"iops,omitempty" example:"3000"`
	AdminUserName       string `json:"adminUserName" validate:"required" example:"admin"`
	AdminUserPassword   string `json:"adminUserPassword" validate:"required" example:"Password123!"`
	HighAvailability    bool   `json:"highAvailability,omitempty" example:"false"`
	BackupRetentionDays int    `json:"backupRetentionDays,omitempty" example:"7"`
	PublicAccess        bool   `json:"publicAccess,omitempty" example:"false"`
	// NHNDBSGToAllowAllInbound (NHN only): when true with publicAccess=true, auto-creates/attaches a 0.0.0.0/0 DB SG.
	NHNDBSGToAllowAllInbound bool   `json:"nhnDBSGToAllowAllInbound,omitempty" example:"false"`
	DeletionProtection       bool   `json:"deletionProtection,omitempty" example:"false"`
	Description              string `json:"description,omitempty" example:"managed by CB-Tumblebug"`
	// AutoFillDefaults fills DBEngineVersion/DBInstanceSpec/StorageType/StorageSize from GET /tumblebug/rdbms/capability when left empty/zero.
	AutoFillDefaults bool       `json:"autoFillDefaults,omitempty" example:"false"`
	TagList          []KeyValue `json:"tagList,omitempty"`
}

// RDBMSInfo is the Tumblebug-facing RDBMS instance resource (persisted and returned to callers).
type RDBMSInfo struct {
	// ResourceType is the type of this resource
	ResourceType string `json:"resourceType" example:"rdbms"`

	// Id is unique identifier for the object
	Id string `json:"id" example:"rdbms-01"`
	// Uid is universally unique identifier for the object, used for labelSelector
	Uid string `json:"uid,omitempty" example:"wef12awefadf1221edcf"`
	// CspResourceName is name assigned to the CSP resource. This name is internally used to handle the resource.
	CspResourceName string `json:"cspResourceName,omitempty"`
	// CspResourceId is resource identifier managed by CSP
	CspResourceId string `json:"cspResourceId,omitempty"`

	// Name is human-readable string to represent the object
	Name             string      `json:"name" example:"rdbms-01"`
	ConnectionName   string      `json:"connectionName"`
	ConnectionConfig ConnConfig  `json:"connectionConfig"`
	Description      string      `json:"description,omitempty"`
	Status           string      `json:"status"`
	SystemMessage    string      `json:"systemMessage,omitempty"`
	Conditions       []Condition `json:"conditions,omitempty"`

	// DeletionRequestedAt (RFC3339) marks a deletion tombstone: non-empty means the
	// record is kept until CSP-side removal is confirmed
	DeletionRequestedAt string `json:"deletionRequestedAt,omitempty"`

	VNetId           string   `json:"vNetId"`
	SubnetIds        []string `json:"subnetIds,omitempty"`
	SecurityGroupIds []string `json:"securityGroupIds,omitempty"`

	DBEngine        string `json:"dbEngine" example:"mysql"`
	DBEngineVersion string `json:"dbEngineVersion" example:"8.0"`
	DBInstanceSpec  string `json:"dbInstanceSpec" example:"db.t3.medium"`
	// DBInstanceType is "Primary" or "ReadReplica"; informational only — CB-Spider has no API yet to create a read replica.
	DBInstanceType           string     `json:"dbInstanceType,omitempty" example:"Primary" enums:"Primary,ReadReplica"`
	StorageType              string     `json:"storageType,omitempty" example:"gp3"`
	StorageSize              int        `json:"storageSize" example:"100"`
	Iops                     string     `json:"iops,omitempty" example:"3000"`
	AdminUserName            string     `json:"adminUserName" example:"admin"`
	HighAvailability         bool       `json:"highAvailability" example:"false"`
	BackupRetentionDays      int        `json:"backupRetentionDays,omitempty" example:"7"`
	BackupTime               string     `json:"backupTime,omitempty" example:"03:00"`
	PublicAccess             bool       `json:"publicAccess" example:"false"`
	NHNDBSGToAllowAllInbound bool       `json:"nhnDBSGToAllowAllInbound,omitempty"`
	DeletionProtection       bool       `json:"deletionProtection" example:"false"`
	Encryption               bool       `json:"encryption,omitempty"`
	Endpoint                 string     `json:"endpoint,omitempty" example:"rdbms-01.xxxx.rds.amazonaws.com:3306"`
	TagList                  []KeyValue `json:"tagList,omitempty"`
}

// RDBMSListResponse is the Tumblebug API response wrapper for listing RDBMS instances.
type RDBMSListResponse struct {
	RDBMS []RDBMSInfo `json:"rdbms"`
}

// RDBMSDatabaseCreateReq creates a logical database inside an Available RDBMS instance; AdminUserPassword is forwarded as-is, never persisted (§1.6).
type RDBMSDatabaseCreateReq struct {
	DatabaseName      string `json:"databaseName" validate:"required" example:"sampledb"`
	AdminUserPassword string `json:"adminUserPassword" validate:"required" example:"Password123!"`
}

// RDBMSDatabaseInfo represents one logical database inside an RDBMS instance; not a tracked Tumblebug resource, always queried live.
type RDBMSDatabaseInfo struct {
	DatabaseName string `json:"databaseName" example:"sampledb"`
}

// RDBMSDatabaseListResponse wraps the Tumblebug API response for GET .../database (list).
type RDBMSDatabaseListResponse struct {
	Databases []string `json:"databases" example:"sampledb"`
}
