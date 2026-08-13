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

// StorageTypeNote provides user-facing guidance for a specific storage type, derived from
// assets/rdbmsinfo.yaml (itself sourced from CB-Spider's storage-type-test results and CSP
// documentation; see resource/rdbms.go).
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

// RDBMSInfoConfig mirrors assets/rdbmsinfo.yaml, the static per-CSP RDBMS reference data
// (storage type knowledge base, DBMS/database requirements) that CB-Spider's live
// RDBMSMetaInfo query does not itself carry. See resource/rdbms.go for how it is loaded
// and applied.
type RDBMSInfoConfig struct {
	DBMS map[string]RDBMSProviderConfig `yaml:"dbms"`
}

// RDBMSProviderConfig is one CSP's entry under "dbms" in assets/rdbmsinfo.yaml.
type RDBMSProviderConfig struct {
	Description string `yaml:"description"`
	Link        string `yaml:"link"`
	Note        string `yaml:"note,omitempty"`
	// SupportedDBEngines lists DB engines empirically verified for this CSP (per CB-Spider's
	// rdbms-mysql-test / rdbms-mariadb-test suites). Omission does not mean unsupported —
	// it means unverified (e.g. postgresql has no dedicated CB-Spider test suite yet).
	SupportedDBEngines []string `yaml:"supportedDBEngines,omitempty"`
	// SupportedDBOperationMethod is how CB-Spider implements the internal Database CRUD API
	// for this CSP, per the CB-Spider wiki's RDBMS Management Guide §4.3 (database-test
	// results): "cspApi" ("CSP Native API" in the wiki) uses a CSP-native database
	// management API (e.g. Google Cloud SQL Admin API); "conventionalSqlExec" ("SQL Direct
	// Execution" in the wiki) connects directly to the instance endpoint with
	// MasterUserPassword (dbConn/mysql-cli-equivalent) and runs CREATE/DROP DATABASE.
	SupportedDBOperationMethod string `yaml:"supportedDBOperationMethod,omitempty" enums:"cspApi,conventionalSqlExec"`
	// SupportsTag is a static reference value for whether this CSP supports CB-Spider's
	// generic RDBMS tag API. The live, per-connection flag is RDBMSMetaInfo.SupportsTag.
	SupportsTag bool `yaml:"supportsTag,omitempty"`
	// StorageTypeSelectable is a static reference value (per CB-Spider's
	// storage-type-test results) for whether this CSP lets a caller choose a storage
	// type at all: false covers both "not supported" (e.g. azure, ibm, ncp auto-assign
	// storage) and "only one type exists" CSPs. This documents CB-Spider's tested
	// behavior; the live, authoritative flag for a given connection is
	// RDBMSMetaInfo.SupportsStorageTypeSelection (see resource/rdbms.go).
	StorageTypeSelectable bool                              `yaml:"storageTypeSelectable"`
	StorageTypes          map[string]RDBMSStorageTypeConfig `yaml:"storageTypes"`
	DBMSRequirements      map[string]RDBMSDBMSRequirement   `yaml:"dbmsRequirements,omitempty"`
	DatabaseRequirements  RDBMSDatabaseRequirement          `yaml:"databaseRequirements,omitempty"`
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

	// Notes carries Tumblebug's own advisory annotations about this response (storage
	// type guidance, which fields above are static/approximate), as opposed to the live
	// capability fields above, which come straight from CB-Spider.
	Notes RDBMSNotes `json:"notes,omitempty"`
}

// RDBMSNotes groups Tumblebug's advisory annotations for one RDBMSMetaInfo response.
type RDBMSNotes struct {
	// StorageTypes is per-storage-type guidance (display name, description, constraints,
	// recommendation) for each entry in StorageTypeOptions above; see buildStorageTypeNotes.
	StorageTypes []StorageTypeNote `json:"storageTypes,omitempty"`
	// StaticFields lists which fields of the surrounding RDBMSMetaInfo are fixed/approximate
	// reference values right now, and why; a field not listed there is live.
	StaticFields []RDBMSStaticField `json:"staticFields,omitempty"`
}

// RDBMSStaticField documents one RDBMSMetaInfo field whose value is a fixed/approximate reference rather than a live query result
type RDBMSStaticField struct {
	Field string `json:"field" example:"storageTypeOptions"`
	Note  string `json:"note,omitempty" example:"NCP G3 generation sets storage type (SSD) automatically; not user-selectable or queryable via API."`
}

// RDBMSCapabilityResponse represents the Tumblebug API response containing a live RDBMS
// capability query for a single connection (providerName+regionName+dbEngine resolves to
// one connection; see resource/rdbms.go's GetRDBMSCapability). As opposed to
// RDBMSSupportResponse, which is a static, CSP-wide reference matrix requiring no CB-Spider
// call.
type RDBMSCapabilityResponse struct {
	ResourceType string        `json:"resourceType" example:"rdbms"`
	Supports     RDBMSMetaInfo `json:"supports"`
}

// RDBMSSupportResponse represents the Tumblebug API response for the static, CSP-wide RDBMS
// support matrix (assets/rdbmsinfo.yaml), listing which capabilities each CSP supports in
// general — as opposed to RDBMSCapabilityResponse, which is a live, per-connection query.
type RDBMSSupportResponse struct {
	ResourceType string                         `json:"resourceType" example:"rdbms"`
	Supports     map[string]RDBMSCSPSupportInfo `json:"supports"`
}

// RDBMSCSPSupportInfo is one CSP's entry in the static RDBMS support matrix
// (assets/rdbmsinfo.yaml), returned by GET /tumblebug/rdbms/support. Every CSP in
// csp.AllCSPs gets an entry, even ones with no RDBMS support at all (e.g. KT) — those
// appear with Supported: false and every other field at its zero value, rather than being
// omitted, matching GetObjectStorageSupport's pattern of always covering the full CSP list.
// Deliberately brief — full storage type guidance (descriptions, constraints, recommendation)
// lives in RDBMSMetaInfo.Notes.StorageTypes (GET /tumblebug/rdbms/capability), not here.
type RDBMSCSPSupportInfo struct {
	// Supported is whether RDBMS is available on this CSP at all (per cspSupportingRDBMS in
	// resource/rdbms.go). false here means every other field below is a zero value, not that
	// they weren't populated.
	Supported bool `json:"supported" example:"true"`
	// SupportedDBEngines lists DB engines empirically verified for this CSP. Omission does
	// not mean unsupported — it means unverified.
	SupportedDBEngines []string `json:"supportedDBEngines" example:"mysql,mariadb"`
	// SupportedDBOperationMethod is how CB-Spider implements the internal Database CRUD API
	// for this CSP, per the CB-Spider wiki's RDBMS Management Guide §4.3: a CSP-native
	// database API ("cspApi"), or connecting directly to the instance endpoint with
	// MasterUserPassword (dbConn/mysql-cli-equivalent) and running CREATE/DROP DATABASE
	// ("conventionalSqlExec").
	SupportedDBOperationMethod string `json:"supportedDBOperationMethod" example:"conventionalSqlExec" enums:"cspApi,conventionalSqlExec"`
	SupportsTag                bool   `json:"supportsTag" example:"true"`
	StorageTypeSelectable      bool   `json:"storageTypeSelectable" example:"true"`
	Note                       string `json:"note,omitempty"`
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
	MasterUserName      string `json:"masterUserName" validate:"required" example:"admin"`
	MasterUserPassword  string `json:"masterUserPassword" validate:"required" example:"Password123!"`
	HighAvailability    bool   `json:"highAvailability,omitempty" example:"false"`
	BackupRetentionDays int    `json:"backupRetentionDays,omitempty" example:"7"`
	PublicAccess        bool   `json:"publicAccess,omitempty" example:"true"`
	DeletionProtection  bool   `json:"deletionProtection,omitempty" example:"false"`
	Description         string `json:"description,omitempty" example:"managed by CB-Tumblebug"`
	// AutoFillDefaults fills DBEngineVersion/DBInstanceSpec/StorageType/StorageSize from
	// GET /tumblebug/rdbms/capability when left empty/zero. Selection is "first supported
	// option that passes live capability checks" — not a cost/performance recommendation.
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

	VNetId           string   `json:"vNetId"`
	SubnetIds        []string `json:"subnetIds,omitempty"`
	SecurityGroupIds []string `json:"securityGroupIds,omitempty"`

	DBEngine        string `json:"dbEngine" example:"mysql"`
	DBEngineVersion string `json:"dbEngineVersion" example:"8.0"`
	DBInstanceSpec  string `json:"dbInstanceSpec" example:"db.t3.medium"`
	// DBInstanceType is "Primary" or "ReadReplica" (CB-Spider v0.12.44+). Informational
	// only for now — CB-Spider has no API yet to create a read replica; this only reflects
	// what a registered/discovered instance already is.
	DBInstanceType      string     `json:"dbInstanceType,omitempty" example:"Primary" enums:"Primary,ReadReplica"`
	StorageType         string     `json:"storageType,omitempty" example:"gp3"`
	StorageSize         int        `json:"storageSize" example:"100"`
	Iops                string     `json:"iops,omitempty" example:"3000"`
	MasterUserName      string     `json:"masterUserName" example:"admin"`
	HighAvailability    bool       `json:"highAvailability" example:"false"`
	BackupRetentionDays int        `json:"backupRetentionDays,omitempty" example:"7"`
	BackupTime          string     `json:"backupTime,omitempty" example:"03:00"`
	PublicAccess        bool       `json:"publicAccess" example:"true"`
	DeletionProtection  bool       `json:"deletionProtection" example:"false"`
	Encryption          bool       `json:"encryption,omitempty"`
	Endpoint            string     `json:"endpoint,omitempty" example:"rdbms-01.xxxx.rds.amazonaws.com:3306"`
	TagList             []KeyValue `json:"tagList,omitempty"`
}

// RDBMSListResponse is the Tumblebug API response wrapper for listing RDBMS instances.
type RDBMSListResponse struct {
	RDBMS []RDBMSInfo `json:"rdbms"`
}

// RDBMSDatabaseCreateReq is the Tumblebug-facing request to create a logical database inside
// an Available RDBMS instance (see resource/rdbms.go's CreateRDBMSDatabase). MasterUserPassword
// is required and forwarded to CB-Spider as-is; Tumblebug never persists it (§1.6).
type RDBMSDatabaseCreateReq struct {
	DatabaseName       string `json:"databaseName" validate:"required" example:"sampledb"`
	MasterUserPassword string `json:"masterUserPassword" validate:"required" example:"Password123!"`
}

// RDBMSDatabaseInfo represents one logical database inside an RDBMS instance. Not a tracked
// Tumblebug resource — it has no kvstore entry of its own and is always queried live from
// CB-Spider.
type RDBMSDatabaseInfo struct {
	DatabaseName string `json:"databaseName" example:"sampledb"`
}

// RDBMSDatabaseListResponse wraps the Tumblebug API response for GET .../database (list).
type RDBMSDatabaseListResponse struct {
	Databases []string `json:"databases" example:"sampledb"`
}
