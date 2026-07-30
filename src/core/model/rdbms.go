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

// SpiderStorageSizeRange represents Spider's StorageSizeRange (PascalCase)
type SpiderStorageSizeRange struct {
	Min int `json:"Min"`
	Max int `json:"Max"`
}

// SpiderRDBMSMetaInfo represents Spider's RDBMSMetaInfo (PascalCase)
type SpiderRDBMSMetaInfo struct {
	DBEngine                         string                 `json:"DBEngine"`
	SupportedVersions                []string               `json:"SupportedVersions"`
	DBInstanceSpecOptions            []string               `json:"DBInstanceSpecOptions"`
	StorageTypeOptions               []string               `json:"StorageTypeOptions"`
	StorageSizeRange                 SpiderStorageSizeRange `json:"StorageSizeRange"`
	SupportsHighAvailability         bool                   `json:"SupportsHighAvailability"`
	SupportsBackup                   bool                   `json:"SupportsBackup"`
	BackupRetentionRange             string                 `json:"BackupRetentionRange"`
	SupportsPublicAccess             bool                   `json:"SupportsPublicAccess"`
	SupportsDeletionProtection       bool                   `json:"SupportsDeletionProtection"`
	SupportsEncryption               bool                   `json:"SupportsEncryption"`
	SupportsStorageTypeSelection     bool                   `json:"SupportsStorageTypeSelection"`
	SupportsStorageSizeConfiguration bool                   `json:"SupportsStorageSizeConfiguration"`
	RequiresSubnet                   bool                   `json:"RequiresSubnet"`
	RequiresSecurityGroup            bool                   `json:"RequiresSecurityGroup"`
}

// StorageSizeRange defines the minimum and maximum storage capacity in Tumblebug format (lowerCamelCase)
type StorageSizeRange struct {
	Min int `json:"min" example:"10"`
	Max int `json:"max" example:"1000"`
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
	RequiresSubnet                   bool             `json:"requiresSubnet" example:"true"`
	RequiresSecurityGroup            bool             `json:"requiresSecurityGroup" example:"true"`
}

// RDBMSSupportResponse represents the Tumblebug API response containing RDBMS support metadata
type RDBMSSupportResponse struct {
	ResourceType string          `json:"resourceType" example:"rdbms"`
	Supports     []RDBMSMetaInfo `json:"supports"`
}
