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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/apierr"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// spiderStorageSizeRange represents Spider's StorageSizeRange (PascalCase)
type spiderStorageSizeRange struct {
	Min int `json:"Min"`
	Max int `json:"Max"`
}

// spiderVCpuInfo represents Spider's VCpuInfo (PascalCase; shared by VMSpec/DBSpec)
type spiderVCpuInfo struct {
	Count    string `json:"Count"`
	ClockGHz string `json:"ClockGHz,omitempty"`
}

// spiderDBSpecInfo represents Spider's DBSpecInfo (PascalCase), from GET /dbspec (CB-Spider
// v0.12.45+). Count/MemSizeMiB are "-1" placeholders when a driver can't determine them.
type spiderDBSpecInfo struct {
	Region             string                 `json:"Region"`
	DBEngine           string                 `json:"DBEngine"`
	Name               string                 `json:"Name"`
	VCpu               spiderVCpuInfo         `json:"VCpu"`
	MemSizeMiB         string                 `json:"MemSizeMiB"`
	StorageSizeRangeGB spiderStorageSizeRange `json:"StorageSizeRangeGB,omitempty"`
}

// spiderDBSpecListResponse represents Spider's response body for GET /dbspec (PascalCase)
type spiderDBSpecListResponse struct {
	Result []spiderDBSpecInfo `json:"dbspec"`
}

// spiderRDBMSEngineListResponse represents Spider's response body for GET /rdbmsengine
type spiderRDBMSEngineListResponse struct {
	Result []string `json:"rdbmsengine"`
}

// spiderRDBMSMetaInfo represents Spider's RDBMSMetaInfo (PascalCase)
type spiderRDBMSMetaInfo struct {
	DBEngine                         string                 `json:"DBEngine"`
	SupportedVersions                []string               `json:"SupportedVersions"`
	DBSpecOptions                    []string               `json:"DBSpecOptions"`
	StorageTypeOptions               []string               `json:"StorageTypeOptions"`
	StorageSizeRangeGB               spiderStorageSizeRange `json:"StorageSizeRangeGB"`
	SupportsHighAvailability         bool                   `json:"SupportsHighAvailability"`
	SupportsBackup                   bool                   `json:"SupportsBackup"`
	BackupRetentionRange             string                 `json:"BackupRetentionRange"`
	SupportsPublicAccess             bool                   `json:"SupportsPublicAccess"`
	SupportsDeletionProtection       bool                   `json:"SupportsDeletionProtection"`
	SupportsEncryption               bool                   `json:"SupportsEncryption"`
	SupportsStorageTypeSelection     bool                   `json:"SupportsStorageTypeSelection"`
	SupportsStorageSizeConfiguration bool                   `json:"SupportsStorageSizeConfiguration"`
	SupportsTag                      bool                   `json:"SupportsTag"`
	RequiresSubnet                   bool                   `json:"RequiresSubnet"`
	RequiresSecurityGroup            bool                   `json:"RequiresSecurityGroup"`
	DataSource                       map[string]string      `json:"DataSource,omitempty"`
	DataSourceNotes                  map[string]string      `json:"DataSourceNotes,omitempty"`
}

// spiderRDBMSCreateReqInfo represents Spider's RDBMSCreateRequest.ReqInfo (PascalCase)
type spiderRDBMSCreateReqInfo struct {
	Name                       string           `json:"Name"`
	VPCName                    string           `json:"VPCName"`
	DBEngine                   string           `json:"DBEngine"`
	DBEngineVersion            string           `json:"DBEngineVersion"`
	DBSpec                     string           `json:"DBSpec"`
	StorageSize                string           `json:"StorageSize,omitempty"`
	StorageType                string           `json:"StorageType,omitempty"`
	Iops                       string           `json:"Iops,omitempty"`
	SubnetNames                []string         `json:"SubnetNames,omitempty"`
	SecurityGroupNames         []string         `json:"SecurityGroupNames,omitempty"`
	MasterUserName             string           `json:"MasterUserName"`
	MasterUserPassword         string           `json:"MasterUserPassword"`
	HighAvailability           bool             `json:"HighAvailability"`
	BackupRetentionDays        int              `json:"BackupRetentionDays,omitempty"`
	PublicAccess               bool             `json:"PublicAccess"`
	NHNAutoOpenDBSecurityGroup bool             `json:"NHNAutoOpenDBSecurityGroup,omitempty"`
	DeletionProtection         bool             `json:"DeletionProtection"`
	TagList                    []model.KeyValue `json:"TagList,omitempty"`
}

// spiderRDBMSCreateRequest represents Spider's RDBMS create request body (PascalCase)
type spiderRDBMSCreateRequest struct {
	ConnectionName string                   `json:"ConnectionName"`
	ReqInfo        spiderRDBMSCreateReqInfo `json:"ReqInfo"`
}

// spiderRDBMSDeleteRequest represents Spider's RDBMS delete request body (PascalCase)
type spiderRDBMSDeleteRequest struct {
	ConnectionName string `json:"ConnectionName"`
}

// spiderRDBMSDatabaseCreateRequest represents Spider's create-database request body
// (PascalCase; spider.RDBMSDatabaseRequest).
type spiderRDBMSDatabaseCreateRequest struct {
	ConnectionName     string `json:"ConnectionName"`
	DatabaseName       string `json:"DatabaseName"`
	MasterUserPassword string `json:"MasterUserPassword,omitempty"`
}

// spiderRDBMSDatabaseCredentialRequest is Spider's list/delete-database request body (no DatabaseName; List needs none, Delete's travels in the URL).
type spiderRDBMSDatabaseCredentialRequest struct {
	ConnectionName     string `json:"ConnectionName"`
	MasterUserPassword string `json:"MasterUserPassword,omitempty"`
}

// spiderRDBMSDatabaseListResponse represents Spider's response body for GET .../databases
// (ListRDBMSDatabases) — confirmed against CB-Spider v0.12.45's RDBMSRest.go.
type spiderRDBMSDatabaseListResponse struct {
	Databases []string `json:"Databases"`
}

// spiderSimpleMsgResp is Spider's {"message": "created"/"deleted"} response for CreateRDBMSDatabase/DeleteRDBMSDatabase.
type spiderSimpleMsgResp struct {
	Message string `json:"message"`
}

// spiderRDBMSInfo represents Spider's RDBMSInfo response (PascalCase)
type spiderRDBMSInfo struct {
	IId                        model.IID        `json:"IId"`
	VpcIID                     model.IID        `json:"VpcIID"`
	DBEngine                   string           `json:"DBEngine"`
	DBEngineVersion            string           `json:"DBEngineVersion"`
	DBSpec                     string           `json:"DBSpec"`
	DBInstanceType             string           `json:"DBInstanceType,omitempty"`
	StorageSize                string           `json:"StorageSize"`
	StorageType                string           `json:"StorageType,omitempty"`
	Iops                       string           `json:"Iops,omitempty"`
	SubnetIIDs                 []model.IID      `json:"SubnetIIDs,omitempty"`
	SecurityGroupIIDs          []model.IID      `json:"SecurityGroupIIDs,omitempty"`
	MasterUserName             string           `json:"MasterUserName"`
	PublicAccess               bool             `json:"PublicAccess"`
	NHNAutoOpenDBSecurityGroup bool             `json:"NHNAutoOpenDBSecurityGroup,omitempty"`
	HighAvailability           bool             `json:"HighAvailability"`
	BackupRetentionDays        int              `json:"BackupRetentionDays,omitempty"`
	BackupTime                 string           `json:"BackupTime,omitempty"`
	DeletionProtection         bool             `json:"DeletionProtection"`
	Encryption                 bool             `json:"Encryption,omitempty"`
	Endpoint                   string           `json:"Endpoint,omitempty"`
	Status                     string           `json:"Status"`
	CreatedTime                string           `json:"CreatedTime,omitempty"`
	KeyValueList               []model.KeyValue `json:"KeyValueList,omitempty"`
	TagList                    []model.KeyValue `json:"TagList,omitempty"`
}

// rdbmsDataSourceKeyNames maps Spider's PascalCase DataSource/DataSourceNotes keys to this API's camelCase field names.
var rdbmsDataSourceKeyNames = map[string]string{
	"SupportedVersions":      "supportedVersions",
	"DBSpecOptions":          "dbInstanceSpecOptions",
	"StorageTypeOptions":     "storageTypeOptions",
	"StorageSizeRangeGB":     "storageSizeRange",
	"StorageSizeRangeGB.Min": "storageSizeRange.min",
	"StorageSizeRangeGB.Max": "storageSizeRange.max",
	"BackupRetentionRange":   "backupRetentionRange",
}

// translateRDBMSDataSourceKey renames a Spider DataSource/DataSourceNotes key via rdbmsDataSourceKeyNames, else lowercases its first letter.
func translateRDBMSDataSourceKey(key string) string {
	if mapped, ok := rdbmsDataSourceKeyNames[key]; ok {
		return mapped
	}
	if key == "" {
		return key
	}
	return strings.ToLower(key[:1]) + key[1:]
}

// normalizeStorageTypeKey canonicalizes a storage type string for lookup (e.g. "General_HDD" vs "General HDD").
func normalizeStorageTypeKey(s string) string {
	s = strings.ToLower(s)
	for _, sep := range []string{"_", " ", "-"} {
		s = strings.ReplaceAll(s, sep, "")
	}
	return s
}

// getStorageTypeConfig looks up a provider's storage type in assets/rdbmsinfo.yaml, exact match first then normalized.
func getStorageTypeConfig(providerName, storageType string) (model.RDBMSStorageTypeConfig, bool) {
	provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]
	if !exists {
		return model.RDBMSStorageTypeConfig{}, false
	}
	if st, found := provider.StorageTypes[storageType]; found {
		return st, true
	}

	target := normalizeStorageTypeKey(storageType)
	for key, st := range provider.StorageTypes {
		if normalizeStorageTypeKey(key) == target {
			return st, true
		}
	}

	// Fallback to the sole configured storage type when Spider returns "NA"/empty for non-selectable providers.
	if (target == "na" || target == "" || !provider.StorageTypeSelectable) && len(provider.StorageTypes) == 1 {
		for _, st := range provider.StorageTypes {
			return st, true
		}
	}
	return model.RDBMSStorageTypeConfig{}, false
}

// buildStorageTypeConstraints renders a storage type's constraints (iops, size, spec compatibility) as one sentence.
func buildStorageTypeConstraints(st model.RDBMSStorageTypeConfig) string {
	var parts []string
	if st.RequiresIops {
		if st.IopsRange != nil {
			parts = append(parts, fmt.Sprintf("Requires 'iops' parameter (range: %d-%d).", st.IopsRange.Min, st.IopsRange.Max))
		} else {
			parts = append(parts, "Requires 'iops' parameter.")
		}
	}
	if st.MinStorageSize > 0 {
		parts = append(parts, fmt.Sprintf("Minimum %dGB storage.", st.MinStorageSize))
	}
	if len(st.CompatibleSpecs) > 0 {
		parts = append(parts, fmt.Sprintf("Requires dbInstanceSpec matching one of: %s.", strings.Join(st.CompatibleSpecs, ", ")))
	}
	if len(st.IncompatibleSpecs) > 0 {
		parts = append(parts, fmt.Sprintf("Not compatible with dbInstanceSpec(s): %s.", strings.Join(st.IncompatibleSpecs, ", ")))
	}
	if len(st.CompatibleMachineSeries) > 0 {
		parts = append(parts, fmt.Sprintf("Only available on machine series: %s.", strings.Join(st.CompatibleMachineSeries, ", ")))
	}
	return strings.Join(parts, " ")
}

// filterSupportedVersions filters out engine versions marked as EndOfLife (EOL) in assets/rdbmsinfo.yaml.
// This prevents provisioning failures when CSP metadata APIs return deprecated/EOL versions that the CSP no longer permits creating.
func filterSupportedVersions(providerName, dbEngine string, liveVersions []string) []string {
	if len(liveVersions) == 0 {
		return liveVersions
	}

	provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]
	if !exists {
		return liveVersions
	}

	reqmt, ok := provider.DBMSRequirements[strings.ToLower(dbEngine)]
	if !ok || len(reqmt.EndOfLifeVersions) == 0 {
		return liveVersions
	}

	eolSet := make(map[string]bool, len(reqmt.EndOfLifeVersions))
	for _, v := range reqmt.EndOfLifeVersions {
		eolSet[strings.ToLower(strings.TrimSpace(v))] = true
	}

	filtered := make([]string, 0, len(liveVersions))
	for _, v := range liveVersions {
		if !eolSet[strings.ToLower(strings.TrimSpace(v))] {
			filtered = append(filtered, v)
		}
	}

	// Fallback to live versions if all were filtered out to avoid an empty list
	if len(filtered) == 0 {
		return liveVersions
	}
	return filtered
}

// buildStorageTypeNotes enriches Spider's storageTypeOptions with display/description/constraint metadata from assets/rdbmsinfo.yaml.
func buildStorageTypeNotes(providerName string, storageTypeOptions []string) []model.StorageTypeNote {
	if _, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]; !exists {
		return nil
	}

	var notes []model.StorageTypeNote
	for _, storageType := range storageTypeOptions {
		if st, found := getStorageTypeConfig(providerName, storageType); found {
			notes = append(notes, model.StorageTypeNote{
				StorageType:         storageType,
				DisplayName:         st.Name,
				Description:         st.Description,
				MinSize:             st.MinStorageSize,
				MaxSize:             st.MaxStorageSize,
				RequiresIops:        st.RequiresIops,
				IopsRange:           st.IopsRange,
				Recommended:         strings.EqualFold(st.RecommendationLevel, "recommended"),
				RecommendationLevel: st.RecommendationLevel,
				CompatibleSpecs:     st.CompatibleSpecs,
				IncompatibleSpecs:   st.IncompatibleSpecs,
				Constraints:         buildStorageTypeConstraints(st),
			})
		} else {
			notes = append(notes, model.StorageTypeNote{
				StorageType: storageType,
				DisplayName: storageType,
				Description: "Storage type details not yet documented.",
			})
		}
	}
	return notes
}

// buildRDBMSStaticFields converts Spider's DataSource/DataSourceNotes to []model.StaticFieldNote, sorted by Field.
func buildRDBMSStaticFields(dataSource, dataSourceNotes map[string]string) []model.StaticFieldNote {
	if len(dataSource) == 0 {
		return nil
	}
	fields := make([]model.StaticFieldNote, 0, len(dataSource))
	for rawField, source := range dataSource {
		if strings.EqualFold(source, "live") {
			continue
		}
		field := translateRDBMSDataSourceKey(rawField)
		note := dataSourceNotes[rawField]
		if note == "" {
			note = fmt.Sprintf("Source: %s", source)
		}
		fields = append(fields, model.StaticFieldNote{
			Field: field,
			Note:  note,
		})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
	return fields
}

// GetRDBMSCapability queries Spider live for one connection/engine's capability details (unlike the static GetRDBMSSupport).
func GetRDBMSCapability(providerName, regionName, dbEngine string) (model.RDBMSCapabilityResponse, error) {
	var response model.RDBMSCapabilityResponse
	response.ResourceType = model.StrRDBMS

	providerName = strings.TrimSpace(providerName)
	regionName = strings.TrimSpace(regionName)
	dbEngine = strings.TrimSpace(strings.ToLower(dbEngine))

	if providerName == "" || regionName == "" || dbEngine == "" {
		return response, fmt.Errorf("providerName, regionName, and dbEngine are required")
	}

	// 1. Resolve the target connection (direct match first, fallback to provider/region query)
	directConnName := fmt.Sprintf("%s-%s", providerName, regionName)
	connConfig, err := common.GetConnConfig(directConnName)
	if err != nil {
		connNames, listErr := common.GetConnConfigListByProviderRegionZone(providerName, regionName, "")
		if listErr != nil || len(connNames) == 0 {
			return response, fmt.Errorf("no matching connection config found (provider: '%s', region: '%s')", providerName, regionName)
		}
		sort.Strings(connNames)
		connConfig, err = common.GetConnConfig(connNames[0])
		if err != nil {
			return response, fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
		}
	}

	// 2. Query Spider for this connection/engine and transform the response
	spiderUrl := fmt.Sprintf("%s/rdbmsmetainfo?ConnectionName=%s&DBEngine=%s",
		model.SpiderRestUrl,
		url.QueryEscape(connConfig.ConfigName),
		url.QueryEscape(dbEngine),
	)

	var spiderMeta spiderRDBMSMetaInfo
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		spiderUrl,
		nil,
		clientManager.SetUseBody(noBody),
		&noBody,
		&spiderMeta,
		clientManager.MediumDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		return response, apierr.Wrap(err, fmt.Sprintf("Spider RDBMS meta info query failed for connection '%s'", connConfig.ConfigName))
	}
	if spiderMeta.DBEngine == "" {
		return response, fmt.Errorf("dbEngine '%s' is not supported for connection '%s'", dbEngine, connConfig.ConfigName)
	}

	// Map Spider response to Tumblebug lowerCamelCase model
	response.Supports = model.RDBMSMetaInfo{
		ProviderName:   connConfig.ProviderName,
		RegionName:     connConfig.RegionZoneInfo.AssignedRegion,
		ConnectionName: connConfig.ConfigName,
		DBEngine:       spiderMeta.DBEngine,
		// Filter out versions marked as EndOfLife (EOL) in assets/rdbmsinfo.yaml
		SupportedVersions:                filterSupportedVersions(connConfig.ProviderName, spiderMeta.DBEngine, spiderMeta.SupportedVersions),
		DBInstanceSpecOptions:            spiderMeta.DBSpecOptions,
		StorageTypeOptions:               spiderMeta.StorageTypeOptions,
		StorageSizeRange:                 model.StorageSizeRange{Min: spiderMeta.StorageSizeRangeGB.Min, Max: spiderMeta.StorageSizeRangeGB.Max},
		SupportsHighAvailability:         spiderMeta.SupportsHighAvailability,
		SupportsBackup:                   spiderMeta.SupportsBackup,
		BackupRetentionRange:             spiderMeta.BackupRetentionRange,
		SupportsPublicAccess:             spiderMeta.SupportsPublicAccess,
		SupportsDeletionProtection:       spiderMeta.SupportsDeletionProtection,
		SupportsEncryption:               spiderMeta.SupportsEncryption,
		SupportsStorageTypeSelection:     spiderMeta.SupportsStorageTypeSelection,
		SupportsStorageSizeConfiguration: spiderMeta.SupportsStorageSizeConfiguration,
		SupportsTag:                      spiderMeta.SupportsTag,
		RequiresSubnet:                   spiderMeta.RequiresSubnet,
		RequiresSecurityGroup:            spiderMeta.RequiresSecurityGroup,
		Notes: model.RDBMSNotes{
			StorageTypes: buildStorageTypeNotes(connConfig.ProviderName, spiderMeta.StorageTypeOptions),
			StaticFields: buildRDBMSStaticFields(spiderMeta.DataSource, spiderMeta.DataSourceNotes),
		},
	}

	// 3. Best-effort enrichment from /dbspec and /rdbmsengine (CB-Spider v0.12.45+) — a failure here must not fail the whole capability call; the fields just stay empty.
	if dbSpecs, dbSpecErr := getSpiderDBSpecs(connConfig.ConfigName, dbEngine); dbSpecErr != nil {
		log.Warn().Err(dbSpecErr).Msgf("GetRDBMSCapability: /dbspec enrichment failed for connection '%s' (non-fatal)", connConfig.ConfigName)
	} else {
		usable := filterUsableDBSpecs(dbSpecs)
		specs := make([]model.RDBMSDBInstanceSpecInfo, 0, len(usable))
		for _, s := range usable {
			specs = append(specs, model.RDBMSDBInstanceSpecInfo{
				Name:               s.Name,
				VCpuCount:          s.VCpu.Count,
				VCpuClockGHz:       s.VCpu.ClockGHz,
				MemSizeMiB:         s.MemSizeMiB,
				StorageSizeRangeGB: model.StorageSizeRange{Min: s.StorageSizeRangeGB.Min, Max: s.StorageSizeRangeGB.Max},
			})
		}
		response.Supports.DBInstanceSpecs = specs
	}
	if engines, engineErr := getSpiderRDBMSEngines(connConfig.ConfigName); engineErr != nil {
		log.Warn().Err(engineErr).Msgf("GetRDBMSCapability: /rdbmsengine enrichment failed for connection '%s' (non-fatal)", connConfig.ConfigName)
	} else {
		response.Supports.LiveSupportedEngines = engines
	}

	return response, nil
}

// GetRDBMSSupport returns the static, CSP-wide RDBMS support matrix from assets/rdbmsinfo.yaml, covering every CSP in csp.AllCSPs (Supported: false for undocumented ones) unless filtered by providerName.
func GetRDBMSSupport(providerName string) (model.RDBMSSupportResponse, error) {
	response := model.RDBMSSupportResponse{
		ResourceType: model.StrRDBMS,
		Supports:     map[string]model.RDBMSCSPSupportInfo{},
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName != "" {
		if !slices.Contains(csp.AllCSPs, providerName) {
			return response, fmt.Errorf("unknown provider '%s'", providerName)
		}
		response.Supports[providerName] = buildCSPSupportInfo(providerName)
		return response, nil
	}

	for _, cspKey := range csp.AllCSPs {
		response.Supports[cspKey] = buildCSPSupportInfo(cspKey)
	}
	return response, nil
}

// buildCSPSupportInfo builds one CSP's RDBMSCSPSupportInfo, returning Supported: false if cspKey has no assets/rdbmsinfo.yaml entry.
func buildCSPSupportInfo(cspKey string) model.RDBMSCSPSupportInfo {
	provider, exists := common.RuntimeRDBMSInfo.DBMS[cspKey]
	if !exists {
		return model.RDBMSCSPSupportInfo{
			Supported: false,
			Note:      "RDBMS is not supported on this CSP.",
		}
	}

	return model.RDBMSCSPSupportInfo{
		Supported:             isRDBMSSupported(cspKey),
		SupportedDBEngines:    provider.SupportedDBEngines,
		DBOperationMethod:     provider.DBOperationMethod,
		SupportsTag:           provider.SupportsTag,
		StorageTypeSelectable: provider.StorageTypeSelectable,
		Note:                  provider.Note,
	}
}

// ========== RDBMS Instance Lifecycle (Create/List/Get/Delete) ==========

// CSP-side teardown (e.g. Alibaba's DependencyViolation.Rds) lags Spider's own record,
// so the CSP-gone confirmation gets its own, more patient budget (vars for tests).
var (
	rdbmsSpiderMaxAttempts  = 30
	rdbmsSpiderInterval     = 10 * time.Second
	rdbmsCSPGoneMaxAttempts = 20
	rdbmsCSPGoneInterval    = 30 * time.Second

	rdbmsPostDeleteWaitDefault = 10 * time.Second
	rdbmsPostDeleteWaitAlibaba = 510 * time.Second
	rdbmsPostDeleteWaitTencent = 90 * time.Second
)

// cspSupportingRDBMS lists CSPs verified against CB-Spider's rdbms-mysql-test suite (v0.12.42).
// KT is not covered by that suite and is marked unsupported until verified.
var cspSupportingRDBMS = map[string]bool{
	csp.AWS:       true,
	csp.Azure:     true,
	csp.GCP:       true,
	csp.Alibaba:   true,
	csp.Tencent:   true,
	csp.IBM:       true,
	csp.OpenStack: true,
	csp.NCP:       true,
	csp.NHN:       true,
	csp.KT:        false,
}

func isRDBMSSupported(cspType string) bool {
	cspType = csp.ResolveCloudPlatform(cspType)
	supported, exists := cspSupportingRDBMS[cspType]
	if !exists {
		return false
	}
	return supported
}

// getSpiderRDBMSMetaInfo queries Spider live for a connection/engine's RDBMSMetaInfo (see docs/feature_guide/rdbms-management.md's CSP-Specific Capability Reference intro).
func getSpiderRDBMSMetaInfo(connectionName, dbEngine string) (spiderRDBMSMetaInfo, error) {
	var spiderMeta spiderRDBMSMetaInfo
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	spiderUrl := fmt.Sprintf("%s/rdbmsmetainfo?ConnectionName=%s&DBEngine=%s",
		model.SpiderRestUrl, url.QueryEscape(connectionName), url.QueryEscape(dbEngine))

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		spiderUrl,
		nil,
		clientManager.SetUseBody(noBody),
		&noBody,
		&spiderMeta,
		clientManager.MediumDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		return spiderMeta, apierr.Wrap(err, fmt.Sprintf("failed to query RDBMS metainfo for connection '%s'", connectionName))
	}
	return spiderMeta, nil
}

// getSpiderDBSpecs queries GET /dbspec, the richer per-spec catalog behind rdbmsmetainfo's flat DBSpecOptions name list.
func getSpiderDBSpecs(connectionName, dbEngine string) ([]spiderDBSpecInfo, error) {
	var specResp spiderDBSpecListResponse
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	spiderUrl := fmt.Sprintf("%s/dbspec?ConnectionName=%s&DBEngine=%s",
		model.SpiderRestUrl, url.QueryEscape(connectionName), url.QueryEscape(dbEngine))

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		spiderUrl,
		nil,
		clientManager.SetUseBody(noBody),
		&noBody,
		&specResp,
		clientManager.MediumDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		return nil, apierr.Wrap(err, fmt.Sprintf("failed to query DB specs for connection '%s'", connectionName))
	}
	return specResp.Result, nil
}

// getSpiderRDBMSEngines queries GET /rdbmsengine for which engines this connection's driver claims to support.
func getSpiderRDBMSEngines(connectionName string) ([]string, error) {
	var engineResp spiderRDBMSEngineListResponse
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	spiderUrl := fmt.Sprintf("%s/rdbmsengine?ConnectionName=%s", model.SpiderRestUrl, url.QueryEscape(connectionName))

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		spiderUrl,
		nil,
		clientManager.SetUseBody(noBody),
		&noBody,
		&engineResp,
		clientManager.ShortDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		return nil, apierr.Wrap(err, fmt.Sprintf("failed to query supported RDBMS engines for connection '%s'", connectionName))
	}
	return engineResp.Result, nil
}

// filterUsableDBSpecs drops entries with no real spec data (vCPU/memory reported as "-1").
func filterUsableDBSpecs(specs []spiderDBSpecInfo) []spiderDBSpecInfo {
	usable := make([]spiderDBSpecInfo, 0, len(specs))
	for _, s := range specs {
		if s.VCpu.Count == "-1" && s.MemSizeMiB == "-1" {
			continue
		}
		usable = append(usable, s)
	}
	return usable
}

// dbSpecSortKey parses a DBSpecInfo's vCPU count/memory for size-ascending sorting; an
// unparseable value sorts last rather than first, so a malformed entry never wins by default.
func dbSpecSortKey(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return math.MaxInt32
	}
	return n
}

// pickSmallestDBSpec returns the smallest usable spec by (vCPU count, memory) ascending, or "" if specs is empty.
func pickSmallestDBSpec(specs []spiderDBSpecInfo) string {
	usable := filterUsableDBSpecs(specs)
	if len(usable) == 0 {
		return ""
	}
	sort.Slice(usable, func(i, j int) bool {
		ci, cj := dbSpecSortKey(usable[i].VCpu.Count), dbSpecSortKey(usable[j].VCpu.Count)
		if ci != cj {
			return ci < cj
		}
		return dbSpecSortKey(usable[i].MemSizeMiB) < dbSpecSortKey(usable[j].MemSizeMiB)
	})
	return usable[0].Name
}

// newestSupportedVersion returns the numerically-greatest dot-separated version string (e.g. "8.0" > "5.5"); fallback when no referenceEngineVersion is set.
func newestSupportedVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	newest := versions[0]
	for _, v := range versions[1:] {
		if compareVersionStrings(v, newest) > 0 {
			newest = v
		}
	}
	return newest
}

// compareVersionStrings compares two dot-separated version strings segment by segment
// (numerically); an unparseable segment is treated as 0.
func compareVersionStrings(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var an, bn int
		if i < len(as) {
			an, _ = strconv.Atoi(strings.TrimSpace(as[i]))
		}
		if i < len(bs) {
			bn, _ = strconv.Atoi(strings.TrimSpace(bs[i]))
		}
		if an != bn {
			return an - bn
		}
	}
	return 0
}

// resolveRDBMSNetwork resolves Tumblebug vNetId/subnetIds/securityGroupIds to their CSP names.
func resolveRDBMSNetwork(nsId, vNetId string, subnetIds, securityGroupIds []string) (vpcName string, subnetNames []string, sgNames []string, err error) {
	vNetData, err := GetResource(nsId, model.StrVNet, vNetId)
	if err != nil {
		return "", nil, nil, apierr.Wrap(err, fmt.Sprintf("failed to resolve vNet '%s'", vNetId))
	}
	vNetInfo := vNetData.(model.VNetInfo)

	for _, subnetId := range subnetIds {
		found := false
		for _, subnet := range vNetInfo.SubnetInfoList {
			if subnet.Id == subnetId {
				subnetNames = append(subnetNames, subnet.CspResourceName)
				found = true
				break
			}
		}
		if !found {
			return "", nil, nil, fmt.Errorf("subnet '%s' not found in vNet '%s'", subnetId, vNetId)
		}
	}

	for _, sgId := range securityGroupIds {
		sgData, err := GetResource(nsId, model.StrSecurityGroup, sgId)
		if err != nil {
			return "", nil, nil, apierr.Wrap(err, fmt.Sprintf("failed to resolve securityGroup '%s'", sgId))
		}
		sgInfo := sgData.(model.SecurityGroupInfo)
		sgNames = append(sgNames, sgInfo.CspResourceName)
	}

	return vNetInfo.CspResourceName, subnetNames, sgNames, nil
}

// applyRDBMSCreateDefaults fills DBEngineVersion/DBSpec/StorageType/StorageSize from live RDBMSMetaInfo when AutoFillDefaults is set and the field is empty.
// safeStorageTypePreference orders preferred storage types for autoFillDefaults, favoring ones needing no extra params (e.g. iops).
var safeStorageTypePreference = map[string][]string{
	"aws":       {"gp3", "gp2", "standard"},   // avoid io1/io2 (requires iops, min 100GB)
	"alibaba":   {"cloud_auto", "cloud_essd"}, // avoid cloud_essd2/3 (500GB/1500GB min), local_ssd (spec constraint)
	"gcp":       {"PD_SSD", "PD_HDD"},         // avoid HYPERDISK_BALANCED (machine series dependency)
	"azure":     {},                           // SupportsStorageTypeSelection=false
	"ibm":       {},                           // SupportsStorageTypeSelection=false
	"ncp":       {},                           // SupportsStorageTypeSelection=false
	"openstack": {"__DEFAULT__", "RBD"},
	"tencent":   {"CLOUD_SSD", "CLOUD_PREMIUM", "CLOUD_HSSD", "local_ssd"},
	"nhn":       {"General SSD", "General HDD"},
}

func applyRDBMSCreateDefaults(meta spiderRDBMSMetaInfo, req *model.RDBMSCreateRequest, providerName string) {
	if !req.AutoFillDefaults {
		return
	}
	if req.DBEngineVersion == "" {
		// Prefer the CB-Spider-verified reference version (assets/rdbmsinfo.yaml) over the live list, since some CSPs restrict valid dbInstanceSpec/version pairs.
		if provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]; exists {
			if reqmt, ok := provider.DBMSRequirements[strings.ToLower(req.DBEngine)]; ok && reqmt.ReferenceEngineVersion != "" {
				req.DBEngineVersion = reqmt.ReferenceEngineVersion
			}
		}
		if req.DBEngineVersion == "" && len(meta.SupportedVersions) > 0 {
			req.DBEngineVersion = newestSupportedVersion(meta.SupportedVersions)
		}
	}
	if req.DBInstanceSpec == "" {
		// Prefer the CB-Spider-verified reference dbInstanceSpec (assets/rdbmsinfo.yaml) over the live catalog's "smallest" pick, which CreateRDBMS can still reject.
		if provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]; exists {
			if reqmt, ok := provider.DBMSRequirements[strings.ToLower(req.DBEngine)]; ok {
				if reqmt.ReferenceDBInstanceSpec != "" {
					req.DBInstanceSpec = reqmt.ReferenceDBInstanceSpec
				} else if reqmt.ReferenceDBSpec != "" {
					req.DBInstanceSpec = reqmt.ReferenceDBSpec
				}
			}
		}
	}
	if req.DBInstanceSpec == "" {
		// Pick the smallest usable spec from the live /dbspec catalog instead of index 0 of the flat DBSpecOptions list (see §5's Azure/IBM picks); falls back to the old behavior on failure.
		if specs, specErr := getSpiderDBSpecs(req.ConnectionName, req.DBEngine); specErr != nil {
			log.Warn().Err(specErr).Msg("AutoFillDefaults: /dbspec lookup failed, falling back to DBSpecOptions[0]")
			if len(meta.DBSpecOptions) > 0 {
				req.DBInstanceSpec = meta.DBSpecOptions[0]
			}
		} else if picked := pickSmallestDBSpec(specs); picked != "" {
			req.DBInstanceSpec = picked
			log.Info().Msgf("AutoFillDefaults: selected smallest usable dbInstanceSpec=%s", picked)
		} else if len(meta.DBSpecOptions) > 0 {
			log.Warn().Msg("AutoFillDefaults: /dbspec returned no usable entries, falling back to DBSpecOptions[0]")
			req.DBInstanceSpec = meta.DBSpecOptions[0]
		}
	}

	// StorageType: prefer safe defaults with no iops/size constraints that are also compatible with the resolved DBInstanceSpec's machine series (isStorageTypeCompatibleWithDBSpec; prevents GCP's PD_SSD-vs-C4A mismatch).
	if req.StorageType == "" && meta.SupportsStorageTypeSelection && len(meta.StorageTypeOptions) > 0 {
		providerKey := strings.ToLower(providerName)
		if preferences, exists := safeStorageTypePreference[providerKey]; exists {
			for _, preferred := range preferences {
				for _, available := range meta.StorageTypeOptions {
					if strings.EqualFold(preferred, available) && isStorageTypeCompatibleWithDBSpec(providerName, available, req.DBInstanceSpec) {
						req.StorageType = available
						log.Info().Msgf("AutoFillDefaults: selected safe storageType=%s", available)
						break
					}
				}
				if req.StorageType != "" {
					break
				}
			}
		}
		// fallback: first available storage type compatible with the resolved DBInstanceSpec, if no safe preference matched (or none were compatible)
		if req.StorageType == "" {
			for _, available := range meta.StorageTypeOptions {
				if isStorageTypeCompatibleWithDBSpec(providerName, available, req.DBInstanceSpec) {
					req.StorageType = available
					log.Warn().Msgf("AutoFillDefaults: no safe preference, using first compatible storageType=%s", req.StorageType)
					break
				}
			}
		}
	}

	// StorageSize: fill from the engine minimum only when configurable.
	if req.StorageSize <= 0 && meta.SupportsStorageSizeConfiguration {
		minSize := meta.StorageSizeRangeGB.Min
		if req.StorageType != "" {
			if st, found := getStorageTypeConfig(providerName, req.StorageType); found && st.MinStorageSize > minSize {
				minSize = st.MinStorageSize
			}
		}
		if minSize > 0 {
			req.StorageSize = minSize
		}
	}

	// Omit unsupported parameters automatically when autoFillDefaults=true (Permissive handling)
	if !meta.SupportsStorageTypeSelection && req.StorageType != "" {
		log.Info().Msgf("AutoFillDefaults: %s does not support storageType selection; auto-clearing storageType '%s'", providerName, req.StorageType)
		req.StorageType = ""
	}
	if strings.EqualFold(providerName, "ibm") && req.BackupRetentionDays > 0 {
		log.Info().Msgf("AutoFillDefaults: IBM does not support BackupRetentionDays during provisioning; auto-clearing it")
		req.BackupRetentionDays = 0
	}
}

// matchesAnySpecPattern reports whether spec matches any glob pattern (e.g. "mysql.n4.*") from assets/rdbmsinfo.yaml; an empty list matches nothing.
func matchesAnySpecPattern(spec string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := filepath.Match(pattern, spec); err == nil && matched {
			return true
		}
	}
	return false
}

// matchesAnyMachineSeries reports whether dbSpec contains any machine-series code (e.g. "C4A") case-insensitively, as a substring check.
func matchesAnyMachineSeries(dbSpec string, series []string) bool {
	lowerSpec := strings.ToLower(dbSpec)
	for _, s := range series {
		if strings.Contains(lowerSpec, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// isStorageTypeCompatibleWithDBSpec checks assets/rdbmsinfo.yaml's compatibleMachineSeries: a restricted type needs a series match; an unrestricted type is incompatible only if another type reserves that series (e.g. GCP's C4A/N4 reserved by HYPERDISK_BALANCED).
func isStorageTypeCompatibleWithDBSpec(providerName, storageType, dbSpec string) bool {
	if storageType == "" || dbSpec == "" {
		return true
	}
	provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]
	if !exists {
		return true
	}
	target, found := getStorageTypeConfig(providerName, storageType)
	if !found {
		return true
	}
	if len(target.CompatibleMachineSeries) > 0 {
		return matchesAnyMachineSeries(dbSpec, target.CompatibleMachineSeries)
	}
	for key, other := range provider.StorageTypes {
		if strings.EqualFold(key, storageType) {
			continue
		}
		if len(other.CompatibleMachineSeries) > 0 && matchesAnyMachineSeries(dbSpec, other.CompatibleMachineSeries) {
			return false
		}
	}
	return true
}

// validateRDBMSCreateRequest checks the request against live capability flags and assets/rdbmsinfo.yaml's storage type constraints before provisioning.
func validateRDBMSCreateRequest(meta spiderRDBMSMetaInfo, req model.RDBMSCreateRequest, providerName string) error {
	// Azure requires subnetIds in VPC-private mode (publicAccess=false).
	if strings.EqualFold(providerName, "azure") && !req.PublicAccess && len(req.SubnetIds) == 0 {
		return fmt.Errorf("subnetIds required for azure when publicAccess is false")
	}
	if req.NHNDBSGToAllowAllInbound {
		if !strings.EqualFold(providerName, "nhn") {
			return fmt.Errorf("nhnDBSGToAllowAllInbound is only supported for NHN Cloud")
		}
		if !req.PublicAccess {
			return fmt.Errorf("nhnDBSGToAllowAllInbound requires publicAccess=true")
		}
	}
	if meta.RequiresSubnet && len(req.SubnetIds) == 0 {
		return fmt.Errorf("subnetIds required for %s", providerName)
	}
	if meta.RequiresSecurityGroup && len(req.SecurityGroupIds) == 0 {
		return fmt.Errorf("securityGroupIds required for %s", providerName)
	}
	if !meta.SupportsStorageTypeSelection && req.StorageType != "" {
		return fmt.Errorf("storageType is not configurable for %s; omit it", providerName)
	}
	if !meta.SupportsStorageSizeConfiguration && req.StorageSize > 0 {
		log.Info().Msgf("validateRDBMSCreateRequest: %s uses auto-scaling storage; user-provided storageSize (%dGB) will be ignored", providerName, req.StorageSize)
	}

	// General storage size range check
	if meta.SupportsStorageSizeConfiguration {
		if req.StorageSize < meta.StorageSizeRangeGB.Min || (meta.StorageSizeRangeGB.Max > 0 && req.StorageSize > meta.StorageSizeRangeGB.Max) {
			return fmt.Errorf("storageSize %d out of range [%d-%d] for %s", req.StorageSize, meta.StorageSizeRangeGB.Min, meta.StorageSizeRangeGB.Max, providerName)
		}
	}

	// StorageType-specific constraint validation (assets/rdbmsinfo.yaml)
	if req.StorageType != "" {
		if st, found := getStorageTypeConfig(providerName, req.StorageType); found {
			if st.MinStorageSize > 0 && req.StorageSize < st.MinStorageSize {
				return fmt.Errorf("storageType '%s' requires minimum %dGB (requested: %dGB)", req.StorageType, st.MinStorageSize, req.StorageSize)
			}
			if st.MaxStorageSize > 0 && req.StorageSize > st.MaxStorageSize {
				return fmt.Errorf("storageType '%s' allows maximum %dGB (requested: %dGB)", req.StorageType, st.MaxStorageSize, req.StorageSize)
			}
			if st.RequiresIops && req.Iops == "" {
				return fmt.Errorf("storageType '%s' requires 'iops' parameter (e.g., '3000')", req.StorageType)
			}
			if req.DBInstanceSpec != "" {
				if len(st.CompatibleSpecs) > 0 && !matchesAnySpecPattern(req.DBInstanceSpec, st.CompatibleSpecs) {
					return fmt.Errorf("storageType '%s' requires dbInstanceSpec matching one of %v (got: %s)", req.StorageType, st.CompatibleSpecs, req.DBInstanceSpec)
				}
				if matchesAnySpecPattern(req.DBInstanceSpec, st.IncompatibleSpecs) {
					return fmt.Errorf("storageType '%s' is not compatible with dbInstanceSpec '%s'", req.StorageType, req.DBInstanceSpec)
				}
			}
		}
		if req.DBInstanceSpec != "" && !isStorageTypeCompatibleWithDBSpec(providerName, req.StorageType, req.DBInstanceSpec) {
			return fmt.Errorf("storageType '%s' is not compatible with dbInstanceSpec '%s' for %s (its machine series requires a different storage type — see assets/rdbmsinfo.yaml's compatibleMachineSeries)", req.StorageType, req.DBInstanceSpec, providerName)
		}
	}

	return nil
}

// validateAdminCredentials checks AdminUserName/AdminUserPassword against assets/rdbmsinfo.yaml's per-CSP requirement (e.g. Tencent forces "root") before provisioning.
func validateAdminCredentials(providerName string, req model.RDBMSCreateRequest) error {
	provider, exists := common.RuntimeRDBMSInfo.DBMS[strings.ToLower(providerName)]
	if !exists {
		return nil
	}

	if nameReq := provider.AdminUserNameRequirement; nameReq != nil {
		if nameReq.FixedValue != "" && !strings.EqualFold(req.AdminUserName, nameReq.FixedValue) {
			return fmt.Errorf("adminUserName must be '%s' for %s (got: '%s')", nameReq.FixedValue, providerName, req.AdminUserName)
		}
		for _, reserved := range nameReq.ReservedValues {
			if strings.EqualFold(req.AdminUserName, reserved) {
				return fmt.Errorf("adminUserName '%s' is reserved on %s; choose a different value", req.AdminUserName, providerName)
			}
		}
	}

	if pwReq := provider.AdminUserPasswordRequirement; pwReq != nil {
		pwLen := len(req.AdminUserPassword)
		if pwReq.MinLength > 0 && pwLen < pwReq.MinLength {
			return fmt.Errorf("adminUserPassword must be at least %d characters for %s (got: %d)", pwReq.MinLength, providerName, pwLen)
		}
		if pwReq.MaxLength > 0 && pwLen > pwReq.MaxLength {
			return fmt.Errorf("adminUserPassword must be at most %d characters for %s (got: %d)", pwReq.MaxLength, providerName, pwLen)
		}
		hasSpecial := hasSpecialChar(req.AdminUserPassword)
		if pwReq.RequiresSpecialChar && !hasSpecial {
			return fmt.Errorf("adminUserPassword requires at least one special character for %s", providerName)
		}
		if pwReq.ForbidsSpecialChar && hasSpecial {
			return fmt.Errorf("adminUserPassword must not contain special characters for %s", providerName)
		}
	}

	return nil
}

// hasSpecialChar reports whether s contains any character outside letters and digits.
func hasSpecialChar(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// updateRDBMSInfoFromSpider copies Spider's response fields (including Status, verbatim) into a Tumblebug RDBMSInfo.
func updateRDBMSInfoFromSpider(rdbmsInfo *model.RDBMSInfo, sp spiderRDBMSInfo) {
	rdbmsInfo.CspResourceName = sp.IId.NameId
	rdbmsInfo.CspResourceId = sp.IId.SystemId
	rdbmsInfo.DBEngine = sp.DBEngine
	rdbmsInfo.DBEngineVersion = sp.DBEngineVersion
	rdbmsInfo.DBInstanceSpec = sp.DBSpec
	rdbmsInfo.DBInstanceType = sp.DBInstanceType
	rdbmsInfo.StorageType = sp.StorageType
	rdbmsInfo.Iops = sp.Iops
	if size, err := strconv.Atoi(sp.StorageSize); err == nil {
		rdbmsInfo.StorageSize = size
	}
	rdbmsInfo.AdminUserName = sp.MasterUserName
	rdbmsInfo.PublicAccess = sp.PublicAccess
	rdbmsInfo.NHNDBSGToAllowAllInbound = sp.NHNAutoOpenDBSecurityGroup
	rdbmsInfo.HighAvailability = sp.HighAvailability
	rdbmsInfo.BackupRetentionDays = sp.BackupRetentionDays
	rdbmsInfo.BackupTime = sp.BackupTime
	rdbmsInfo.DeletionProtection = sp.DeletionProtection
	rdbmsInfo.Encryption = sp.Encryption
	rdbmsInfo.Endpoint = sp.Endpoint
	rdbmsInfo.TagList = sp.TagList
	rdbmsInfo.Status = sp.Status
}

const (
	rdbmsCreationPollInterval = 20 * time.Second
	rdbmsCreationMaxAttempts  = 45 // 15 minutes total
	rdbmsCreationTimeout      = rdbmsCreationMaxAttempts * rdbmsCreationPollInterval
)

// ConfirmRDBMSCreated polls Spider GET until Status reaches "Available" or attempts run out, retrying regardless of the status seen in between — Creating, or a possibly-transient Error (e.g. Alibaba's driver mis-mapping ACCOUNT_MODE_UPGRADING) — persisting progress to kvstore so a concurrent GetRDBMS observes it.
func ConfirmRDBMSCreated(rdbmsKey string, rdbmsInfo *model.RDBMSInfo) (spiderRDBMSInfo, error) {
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	getUrl := fmt.Sprintf("%s/rdbms/%s?ConnectionName=%s", model.SpiderRestUrl, rdbmsInfo.Uid, rdbmsInfo.ConnectionName)
	log.Info().Msgf("Waiting for RDBMS %s to reach Available state (polling every %s up to %s)...", rdbmsInfo.Id, rdbmsCreationPollInterval, rdbmsCreationTimeout)

	for attempt := 1; attempt <= rdbmsCreationMaxAttempts; attempt++ {
		var spResp spiderRDBMSInfo
		restyResp, err := clientManager.ExecuteHttpRequest(
			client,
			"GET",
			getUrl,
			nil,
			clientManager.SetUseBody(noBody),
			&noBody,
			&spResp,
			clientManager.ShortDuration,
		)

		if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
			log.Warn().Err(err).Msgf("RDBMS %s status poll failed on attempt %d/%d; retrying", rdbmsInfo.Uid, attempt, rdbmsCreationMaxAttempts)
		} else if spResp.Status == "Available" {
			log.Info().Msgf("RDBMS %s reached Available state on attempt %d/%d", rdbmsInfo.Id, attempt, rdbmsCreationMaxAttempts)
			return spResp, nil
		} else {
			log.Info().Msgf("RDBMS %s not yet Available (status: %s), attempt %d/%d; will poll again in %s", rdbmsInfo.Uid, spResp.Status, attempt, rdbmsCreationMaxAttempts, rdbmsCreationPollInterval)
			updateRDBMSInfoFromSpider(rdbmsInfo, spResp)
			if val, mErr := json.Marshal(rdbmsInfo); mErr == nil {
				_ = kvstore.Put(rdbmsKey, string(val))
			}
		}

		if attempt < rdbmsCreationMaxAttempts {
			time.Sleep(rdbmsCreationPollInterval)
		}
	}
	return spiderRDBMSInfo{}, fmt.Errorf("timed out after %s waiting for RDBMS %s to become Available", rdbmsCreationTimeout, rdbmsInfo.Uid)
}

// resolveAndValidateRDBMSCreateRequest resolves IDs, validates against live RDBMSMetaInfo, and applies defaults — the single shared core behind CreateRDBMS and the dry-run ValidateRDBMSCreateRequest.
func resolveAndValidateRDBMSCreateRequest(nsId string, req model.RDBMSCreateRequest) (
	resolvedReq model.RDBMSCreateRequest,
	connConfig model.ConnConfig,
	vpcName string,
	subnetNames []string,
	sgNames []string,
	err error,
) {
	if err = common.CheckString(nsId); err != nil {
		return
	}
	if err = validate.Struct(req); err != nil {
		return
	}
	if err = common.CheckString(req.Name); err != nil {
		return
	}

	connConfig, err = common.GetConnConfig(req.ConnectionName)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
		return
	}

	if !isRDBMSSupported(connConfig.ProviderName) {
		err = fmt.Errorf("RDBMS is not supported for CSP: %s", connConfig.ProviderName)
		return
	}

	// Resolve Tumblebug vNet/subnet/securityGroup IDs to CSP names
	vpcName, subnetNames, sgNames, err = resolveRDBMSNetwork(nsId, req.VNetId, req.SubnetIds, req.SecurityGroupIds)
	if err != nil {
		return
	}

	// Validate against live RDBMSMetaInfo (always live; see §2.2)
	meta, metaErr := getSpiderRDBMSMetaInfo(req.ConnectionName, req.DBEngine)
	if metaErr != nil {
		err = metaErr
		return
	}
	if meta.DBEngine == "" {
		liveHint := ""
		if engines, engineErr := getSpiderRDBMSEngines(req.ConnectionName); engineErr == nil && len(engines) > 0 {
			liveHint = fmt.Sprintf("; this connection's live-supported engines are: %v", engines)
		}
		err = fmt.Errorf("dbEngine '%s' is not supported for connection '%s' (see GET /tumblebug/rdbms/support for %s's supportedDBEngines)%s",
			req.DBEngine, req.ConnectionName, connConfig.ProviderName, liveHint)
		return
	}

	resolvedReq = req
	applyRDBMSCreateDefaults(meta, &resolvedReq, connConfig.ProviderName)
	if resolvedReq.DBEngineVersion == "" {
		err = fmt.Errorf("dbEngineVersion required (or set autoFillDefaults=true)")
		return
	}
	if resolvedReq.DBInstanceSpec == "" {
		err = fmt.Errorf("dbInstanceSpec required (or set autoFillDefaults=true)")
		return
	}
	if meta.SupportsStorageSizeConfiguration {
		if resolvedReq.StorageSize <= 0 {
			err = fmt.Errorf("storageSize required (or set autoFillDefaults=true)")
			return
		}
	} else {
		resolvedReq.StorageSize = 0
	}
	if err = validateRDBMSCreateRequest(meta, resolvedReq, connConfig.ProviderName); err != nil {
		return
	}
	if err = validateAdminCredentials(connConfig.ProviderName, resolvedReq); err != nil {
		return
	}

	return resolvedReq, connConfig, vpcName, subnetNames, sgNames, nil
}

// ValidateRDBMSCreateRequest runs CreateRDBMS's exact validation as a pure dry run — no Spider create call, no kvstore writes.
func ValidateRDBMSCreateRequest(nsId string, req model.RDBMSCreateRequest) (model.RDBMSCreateRequest, error) {
	resolvedReq, _, _, _, _, err := resolveAndValidateRDBMSCreateRequest(nsId, req)
	return resolvedReq, err
}

// CreateRDBMS creates a managed RDBMS instance via CB-Spider, polling until it leaves "Creating" so it returns the final state directly (see §4.1).
func CreateRDBMS(ctx context.Context, nsId string, req model.RDBMSCreateRequest) (model.RDBMSInfo, error) {
	var emptyRet model.RDBMSInfo
	var rdbmsInfo model.RDBMSInfo

	resolvedReq, connConfig, vpcName, subnetNames, sgNames, err := resolveAndValidateRDBMSCreateRequest(nsId, req)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	req = resolvedReq

	// 4. Set the resource type and base info
	resourceType := model.StrRDBMS
	rdbmsInfo.ResourceType = resourceType
	rdbmsInfo.Id = req.Name
	rdbmsInfo.Name = req.Name
	rdbmsInfo.ConnectionName = req.ConnectionName
	rdbmsInfo.ConnectionConfig = connConfig
	rdbmsInfo.Description = req.Description
	rdbmsInfo.VNetId = req.VNetId
	rdbmsInfo.SubnetIds = req.SubnetIds
	rdbmsInfo.SecurityGroupIds = req.SecurityGroupIds
	rdbmsInfo.DBEngine = req.DBEngine
	rdbmsInfo.DBEngineVersion = req.DBEngineVersion
	rdbmsInfo.DBInstanceSpec = req.DBInstanceSpec
	rdbmsInfo.StorageType = req.StorageType
	rdbmsInfo.StorageSize = req.StorageSize
	rdbmsInfo.Iops = req.Iops
	rdbmsInfo.AdminUserName = req.AdminUserName
	rdbmsInfo.HighAvailability = req.HighAvailability
	rdbmsInfo.BackupRetentionDays = req.BackupRetentionDays
	rdbmsInfo.PublicAccess = req.PublicAccess
	rdbmsInfo.NHNDBSGToAllowAllInbound = req.NHNDBSGToAllowAllInbound
	rdbmsInfo.DeletionProtection = req.DeletionProtection
	rdbmsInfo.TagList = req.TagList

	rdbmsKey := common.GenResourceKey(nsId, resourceType, rdbmsInfo.Id)

	// 5. Check if the RDBMS instance already exists
	exists, err := CheckResource(nsId, resourceType, rdbmsInfo.Id)
	if exists {
		err := fmt.Errorf("already exists, RDBMS: %s", rdbmsInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to check if the RDBMS (%s) exists or not", rdbmsInfo.Id))
	}

	// 6. [Conditions] Mark as not ready (creating) before calling Spider API
	rdbmsInfo.Uid = common.GenUid() // set before the Spider call so a Failed record still records the attempted CSP name
	model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreating, "RDBMS creation in progress")
	model.SetCondition(&rdbmsInfo.Conditions, model.ConditionSynced, model.ConditionFalse, model.ReasonCreating, "")
	rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
	if val, err := json.Marshal(rdbmsInfo); err == nil {
		if err := kvstore.Put(rdbmsKey, string(val)); err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	} else {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	var storageSizeStr string
	if req.StorageSize > 0 {
		storageSizeStr = strconv.Itoa(req.StorageSize)
	}

	// 7. Call Spider API to create the RDBMS instance
	spReq := spiderRDBMSCreateRequest{
		ConnectionName: req.ConnectionName,
		ReqInfo: spiderRDBMSCreateReqInfo{
			Name:                       rdbmsInfo.Uid,
			VPCName:                    vpcName,
			DBEngine:                   req.DBEngine,
			DBEngineVersion:            req.DBEngineVersion,
			DBSpec:                     req.DBInstanceSpec,
			StorageSize:                storageSizeStr,
			StorageType:                req.StorageType,
			Iops:                       req.Iops,
			SubnetNames:                subnetNames,
			SecurityGroupNames:         sgNames,
			MasterUserName:             req.AdminUserName,
			MasterUserPassword:         req.AdminUserPassword,
			HighAvailability:           req.HighAvailability,
			BackupRetentionDays:        req.BackupRetentionDays,
			PublicAccess:               req.PublicAccess,
			NHNAutoOpenDBSecurityGroup: req.NHNDBSGToAllowAllInbound,
			DeletionProtection:         req.DeletionProtection,
			TagList:                    req.TagList,
		},
	}

	client := clientManager.NewHttpClient()
	spResp := spiderRDBMSInfo{}
	url := fmt.Sprintf("%s/rdbms", model.SpiderRestUrl)
	logReq := spReq
	logReq.ReqInfo.MasterUserPassword = "********"
	log.Debug().Msgf("[Request to Spider] Creating RDBMS (url: %s, request: %+v)", url, logReq)

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"POST",
		url,
		nil,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.VeryLongDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, err.Error())
		rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
		rdbmsInfo.SystemMessage = err.Error()
		if failVal, marshalErr := json.Marshal(rdbmsInfo); marshalErr == nil {
			_ = kvstore.Put(rdbmsKey, string(failVal))
		}
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to create RDBMS '%s'", rdbmsInfo.Id))
	}

	log.Debug().Msgf("[Response from Spider] Creating RDBMS: %+v", spResp)

	// 8. If Spider returned before the instance reached "Available", poll until it does or
	// times out — this call blocks so the caller receives the final Available/Failed state
	// directly, retrying through any status in between (Creating, or a possibly-transient
	// Error such as Alibaba's driver mis-mapping ACCOUNT_MODE_UPGRADING).
	if spResp.Status != "Available" {
		log.Info().Msgf("RDBMS %s not yet Available (status: %s); confirming (timeout: %s)", rdbmsInfo.Id, spResp.Status, rdbmsCreationTimeout)
		updateRDBMSInfoFromSpider(&rdbmsInfo, spResp)
		if val, mErr := json.Marshal(rdbmsInfo); mErr == nil {
			_ = kvstore.Put(rdbmsKey, string(val))
		}

		confirmed, pollErr := ConfirmRDBMSCreated(rdbmsKey, &rdbmsInfo)
		if pollErr != nil {
			log.Error().Err(pollErr).Msg("")
			model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, pollErr.Error())
			rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
			rdbmsInfo.SystemMessage = pollErr.Error()
			if failVal, marshalErr := json.Marshal(rdbmsInfo); marshalErr == nil {
				_ = kvstore.Put(rdbmsKey, string(failVal))
			}
			return emptyRet, apierr.Wrap(pollErr, fmt.Sprintf("RDBMS '%s' did not become available", rdbmsInfo.Id))
		}
		spResp = confirmed
	}

	// 9. Map Spider's final response and mark the create operation as succeeded
	updateRDBMSInfoFromSpider(&rdbmsInfo, spResp)
	model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionTrue, model.ReasonAvailable, "")
	model.SetCondition(&rdbmsInfo.Conditions, model.ConditionSynced, model.ConditionTrue, model.ReasonAvailable, "")
	rdbmsInfo.SystemMessage = ""

	val, err := json.Marshal(rdbmsInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err := kvstore.Put(rdbmsKey, string(val)); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 10. Store label info
	labels := map[string]string{
		model.LabelManager:         model.StrManager,
		model.LabelNamespace:       nsId,
		model.LabelLabelType:       model.StrRDBMS,
		model.LabelId:              rdbmsInfo.Id,
		model.LabelName:            rdbmsInfo.Name,
		model.LabelUid:             rdbmsInfo.Uid,
		model.LabelCspResourceId:   rdbmsInfo.CspResourceId,
		model.LabelCspResourceName: rdbmsInfo.CspResourceName,
		model.LabelDescription:     rdbmsInfo.Description,
		model.LabelConnectionName:  rdbmsInfo.ConnectionName,
	}
	if err := label.CreateOrUpdateLabel(ctx, model.StrRDBMS, rdbmsInfo.Uid, rdbmsKey, labels); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	log.Info().Msgf("RDBMS created: id=%s, status=%s, endpoint=%s, engine=%s/%s",
		rdbmsInfo.Id, rdbmsInfo.Status, rdbmsInfo.Endpoint, rdbmsInfo.DBEngine, rdbmsInfo.DBEngineVersion)
	return rdbmsInfo, nil
}

// isRDBMSInfoUpdated reports whether any Spider-derived field changed, to avoid
// unnecessary kvstore writes on every GetRDBMS poll.
func isRDBMSInfoUpdated(oldInfo, newInfo model.RDBMSInfo) bool {
	return oldInfo.Status != newInfo.Status ||
		oldInfo.Endpoint != newInfo.Endpoint ||
		oldInfo.StorageSize != newInfo.StorageSize ||
		oldInfo.StorageType != newInfo.StorageType ||
		oldInfo.BackupTime != newInfo.BackupTime ||
		oldInfo.Encryption != newInfo.Encryption
}

// GetRDBMS retrieves the RDBMS instance info, refreshed live from CB-Spider.
func GetRDBMS(nsId, rdbmsId string) (model.RDBMSInfo, error) {
	var emptyRet model.RDBMSInfo

	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err := common.CheckString(rdbmsId); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	resourceType := model.StrRDBMS
	rdbmsData, err := GetResource(nsId, resourceType, rdbmsId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, RDBMS: %s", rdbmsId)
		return emptyRet, err
	}
	oldInfo := rdbmsData.(model.RDBMSInfo)

	if !isRDBMSSupported(oldInfo.ConnectionConfig.ProviderName) {
		err = fmt.Errorf("RDBMS is not supported for CSP: %s", oldInfo.ConnectionConfig.ProviderName)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if oldInfo.Uid == "" {
		// Create never reached Spider (failed before the CSP name was assigned); nothing to refresh.
		return oldInfo, nil
	}

	client := clientManager.NewHttpClient()
	spResp := spiderRDBMSInfo{}
	noBody := clientManager.NoBody
	getUrl := fmt.Sprintf("%s/rdbms/%s?ConnectionName=%s", model.SpiderRestUrl, oldInfo.Uid, oldInfo.ConnectionName)

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		getUrl,
		nil,
		clientManager.SetUseBody(noBody),
		&noBody,
		&spResp,
		clientManager.ShortDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to get RDBMS '%s'", rdbmsId))
	}

	data, _ := json.Marshal(oldInfo)
	var newInfo model.RDBMSInfo
	_ = json.Unmarshal(data, &newInfo)
	updateRDBMSInfoFromSpider(&newInfo, spResp)

	if isRDBMSInfoUpdated(oldInfo, newInfo) {
		rdbmsKey := common.GenResourceKey(nsId, resourceType, rdbmsId)
		if val, err := json.Marshal(newInfo); err == nil {
			if err := kvstore.Put(rdbmsKey, string(val)); err != nil {
				log.Error().Err(err).Msg("Failed to update RDBMS info in kvstore")
				return emptyRet, err
			}
		}
	}

	return newInfo, nil
}

// DeleteRDBMS deletes the specified RDBMS instance from the specified namespace.
// If force is true, Spider force-deletes the instance despite DeletionProtection.
func DeleteRDBMS(nsId, rdbmsId string, force bool) error {
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if err := common.CheckString(rdbmsId); err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	resourceType := model.StrRDBMS
	rdbmsData, err := GetResource(nsId, resourceType, rdbmsId)
	if err != nil {
		log.Error().Err(err).Msgf("not found, RDBMS: %s", rdbmsId)
		return err
	}
	rdbmsInfo := rdbmsData.(model.RDBMSInfo)

	if !isRDBMSSupported(rdbmsInfo.ConnectionConfig.ProviderName) {
		err = fmt.Errorf("RDBMS is not supported for CSP: %s", rdbmsInfo.ConnectionConfig.ProviderName)
		log.Error().Err(err).Msg("")
		return err
	}

	// [Conditions] Mark as not ready before calling Spider API
	model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeleting, "RDBMS deletion in progress")
	rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
	rdbmsInfo.SystemMessage = ""
	rdbmsKey := common.GenResourceKey(nsId, resourceType, rdbmsInfo.Id)
	if val, err := json.Marshal(rdbmsInfo); err == nil {
		if err := kvstore.Put(rdbmsKey, string(val)); err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	if rdbmsInfo.Uid != "" {
		// Retry the DELETE call itself only on failure; confirming eventual consistency is PollResourceFullyDeleted's job, kept separate so budgets don't nest.
		const maxDeleteCallAttempts = 3
		const deleteCallRetryWait = 20 * time.Second

		var lastErr error
		for attempt := 1; attempt <= maxDeleteCallAttempts; attempt++ {
			if attempt > 1 {
				log.Warn().Msgf("RDBMS %s DELETE call failed; retrying (attempt %d/%d) after %s...", rdbmsInfo.Uid, attempt, maxDeleteCallAttempts, deleteCallRetryWait)
				time.Sleep(deleteCallRetryWait)
			}

			client := clientManager.NewHttpClient()
			deleteURL := fmt.Sprintf("%s/rdbms/%s", model.SpiderRestUrl, rdbmsInfo.Uid)
			if force {
				deleteURL += "?force=true"
			}
			spReq := spiderRDBMSDeleteRequest{ConnectionName: rdbmsInfo.ConnectionName}
			var spResp spiderBooleanInfoResp

			restyResp, delErr := clientManager.ExecuteHttpRequest(
				client,
				"DELETE",
				deleteURL,
				nil,
				clientManager.SetUseBody(spReq),
				&spReq,
				&spResp,
				clientManager.ShortDuration,
			)
			delErr = clientManager.HandleHttpResponse(restyResp, delErr)

			if delErr != nil && !apierr.IsNotFound(delErr) {
				lastErr = fmt.Errorf("DELETE failed for RDBMS %s: %w", rdbmsInfo.Uid, delErr)
				log.Warn().Err(lastErr).Msgf("RDBMS %s delete attempt %d/%d failed", rdbmsInfo.Uid, attempt, maxDeleteCallAttempts)
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			err = lastErr
			log.Error().Err(err).Msg("")
			if markErr := patchKvTombstone(nsId, resourceType, rdbmsInfo.Id, model.ResourceStatusFailed, model.ReasonDeletionFailed, err.Error()); markErr != nil {
				log.Error().Err(markErr).Msgf("Failed to mark RDBMS %q as DeletionFailed", rdbmsId)
			}
			return err
		}

		// Confirm the RDBMS is completely gone from Spider and CSP
		var deleted bool
		var pollErr error
		getUrl := fmt.Sprintf("%s/rdbms/%s?ConnectionName=%s", model.SpiderRestUrl, rdbmsInfo.Uid, rdbmsInfo.ConnectionName)
		deleted, pollErr = PollResourceFullyDeleted(getUrl, rdbmsInfo.ConnectionName, model.StrRDBMS, rdbmsInfo.CspResourceId,
			rdbmsSpiderMaxAttempts, rdbmsSpiderInterval, rdbmsCSPGoneMaxAttempts, rdbmsCSPGoneInterval)

		if !deleted && !force {
			// Fail-closed: a billed RDBMS must not be purged on an unconfirmed deletion.
			// The record stays Deleting (not Failed) so the reconciler keeps retrying;
			// slow CSP teardown routinely outlives the poll budget.
			cause := fmt.Errorf("RDBMS %q deletion unconfirmed: %v; record retained — retry DELETE, or delete with force to discard it (%w)", rdbmsId, pollErr, ErrDeletionInProgress)
			if markErr := patchKvTombstone(nsId, resourceType, rdbmsInfo.Id, model.ResourceStatusDeleting, model.ReasonDeleting, cause.Error()); markErr != nil {
				log.Error().Err(markErr).Msgf("Failed to mark RDBMS %q deletion outcome", rdbmsId)
			}
			log.Warn().Err(cause).Msg("Fail-closed deletion: record retained")
			return cause
		}
		if !deleted {
			log.Warn().Err(pollErr).Msgf("Force deletion of RDBMS %q: deletion unconfirmed; removing the record anyway (the CSP resource may remain as an orphan)", rdbmsId)
		}

		// Wait to allow the CSP to stabilize before a caller's likely-next Subnet/VNet delete.
		// Tencent: 90s confirmed sufficient. Alibaba: 90s was still observed failing, so it gets
		// its own, longer wait — its VPC-level dependency clears on an even slower timeline.
		postDeleteWait := rdbmsPostDeleteWaitDefault
		switch {
		case strings.EqualFold(rdbmsInfo.ConnectionConfig.ProviderName, csp.Alibaba):
			postDeleteWait = rdbmsPostDeleteWaitAlibaba
		case strings.EqualFold(rdbmsInfo.ConnectionConfig.ProviderName, csp.Tencent):
			postDeleteWait = rdbmsPostDeleteWaitTencent
		}
		if !deleted {
			// nothing was confirmed torn down, so there is no dependency release to wait for
			postDeleteWait = 0
		}
		if postDeleteWait > 0 {
			log.Info().Msgf("Waiting %s for CSP post-delete stabilization (%s) to release background network bindings...", postDeleteWait, rdbmsInfo.ConnectionConfig.ProviderName)
			time.Sleep(postDeleteWait)
			log.Info().Msgf("Post-delete stabilization wait of %s completed for %s (%s)", postDeleteWait, rdbmsInfo.ConnectionConfig.ProviderName, rdbmsId)
		}

	} else {
		log.Warn().Msgf("RDBMS %s has no CSP resource (Uid is empty). Skipping Spider DELETE and removing metadata only.", rdbmsId)
	}

	if err := kvstore.Delete(rdbmsKey); err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	if err := label.DeleteLabelObject(model.StrRDBMS, rdbmsInfo.Uid); err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// PruneRDBMS purges Tumblebug metadata for RDBMS instances Reconcile diagnosed as missing on CSP.
func PruneRDBMS(nsId string) (model.ResourcePruneResults, error) {
	err := common.CheckString(nsId)
	if err != nil {
		return model.ResourcePruneResults{}, err
	}

	resList, err := ListResource(nsId, model.StrRDBMS, "", "")
	if err != nil {
		return model.ResourcePruneResults{}, err
	}

	rdbmsList, ok := resList.([]model.RDBMSInfo)
	if !ok {
		return model.ResourcePruneResults{}, fmt.Errorf("unexpected type from ListResource")
	}

	pruneResults := model.ResourcePruneResults{
		Results: []model.ResourcePruneResult{},
	}

	for _, rdbmsItem := range rdbmsList {
		condSynced := model.GetCondition(rdbmsItem.Conditions, model.ConditionSynced)
		// Only ReasonCspResourceMissing is Prune's concern — a broader Status==Failed check would also
		// match SpMetaMissing (CSP still alive) and delete metadata for a resource that isn't actually gone.
		if condSynced == nil || condSynced.Reason != model.ReasonCspResourceMissing {
			continue
		}

		rdbmsKey := common.GenResourceKey(nsId, model.StrRDBMS, rdbmsItem.Id)

		// The list above is a snapshot; re-read and re-verify right before acting, since a
		// concurrent Reconcile may have already restored this item in the meantime.
		freshKv, exists, fErr := kvstore.GetKv(rdbmsKey)
		if fErr != nil || !exists {
			continue
		}
		if json.Unmarshal([]byte(freshKv.Value), &rdbmsItem) != nil {
			continue
		}
		condSynced = model.GetCondition(rdbmsItem.Conditions, model.ConditionSynced)
		if condSynced == nil || condSynced.Reason != model.ReasonCspResourceMissing {
			continue
		}

		// Purge Spider's own orphaned IID too, or it's stranded forever once TB's record is gone.
		if rdbmsItem.Uid != "" {
			forceDelURL := fmt.Sprintf("%s/rdbms/%s?force=true", model.SpiderRestUrl, rdbmsItem.Uid)
			spForceDelReq := spiderRDBMSDeleteRequest{ConnectionName: rdbmsItem.ConnectionName}
			var spForceDelResp spiderBooleanInfoResp
			restyForceDelResp, forceDelErr := clientManager.ExecuteHttpRequest(
				clientManager.NewHttpClient(),
				"DELETE",
				forceDelURL,
				nil,
				clientManager.SetUseBody(spForceDelReq),
				&spForceDelReq,
				&spForceDelResp,
				clientManager.ShortDuration,
			)
			if forceDelErr = clientManager.HandleHttpResponse(restyForceDelResp, forceDelErr); forceDelErr != nil && !apierr.IsNotFound(forceDelErr) {
				log.Warn().Err(forceDelErr).Msgf("Prune: failed to purge Spider metadata for RDBMS %s (continuing)", rdbmsItem.Id)
			}
		}

		// Remove label
		if lErr := label.DeleteLabelObject(model.StrRDBMS, rdbmsItem.Uid); lErr != nil {
			log.Warn().Err(lErr).Msgf("failed to delete label during prune for RDBMS %s", rdbmsItem.Id)
		}

		// Delete KV metadata
		delErr := kvstore.Delete(rdbmsKey)
		res := model.ResourcePruneResult{
			ResourceType:   model.StrRDBMS,
			ResourceId:     rdbmsItem.Id,
			ConnectionName: rdbmsItem.ConnectionName,
		}

		if delErr != nil {
			res.Success = false
			res.Error = delErr.Error()
			pruneResults.FailedCount++
		} else {
			res.Success = true
			res.Message = fmt.Sprintf("Orphaned metadata for RDBMS (%s) pruned successfully", rdbmsItem.Id)
			pruneResults.SuccessCount++
		}
		pruneResults.TotalPruned++
		pruneResults.Results = append(pruneResults.Results, res)
	}

	return pruneResults, nil
}

// ========== RDBMS Internal Logical Database CRUD ==========
// Databases aren't tracked as Tumblebug resources (no kvstore/label); AdminUserPassword is never persisted, only forwarded per call (see §1.6).

// CreateRDBMSDatabase creates a logical database inside an Available RDBMS instance via
// CB-Spider.
func CreateRDBMSDatabase(nsId, rdbmsId string, req model.RDBMSDatabaseCreateReq) (model.RDBMSDatabaseInfo, error) {
	var emptyRet model.RDBMSDatabaseInfo

	if err := validate.Struct(req); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	rdbmsInfo, err := GetRDBMS(nsId, rdbmsId)
	if err != nil {
		return emptyRet, err
	}
	if rdbmsInfo.Status != model.StorageStatusAvailable {
		err = fmt.Errorf("RDBMS '%s' is not Available (status: %s); databases can only be created on an Available instance", rdbmsId, rdbmsInfo.Status)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	spReq := spiderRDBMSDatabaseCreateRequest{
		ConnectionName:     rdbmsInfo.ConnectionName,
		DatabaseName:       req.DatabaseName,
		MasterUserPassword: req.AdminUserPassword,
	}
	logReq := spReq
	logReq.MasterUserPassword = "********"

	client := clientManager.NewHttpClient()
	spResp := spiderSimpleMsgResp{}
	spiderUrl := fmt.Sprintf("%s/rdbms/%s/databases", model.SpiderRestUrl, rdbmsInfo.Uid)
	log.Debug().Msgf("[Request to Spider] Creating RDBMS database (url: %s, request: %+v)", spiderUrl, logReq)

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"POST",
		spiderUrl,
		nil,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to create database '%s' in RDBMS '%s'", req.DatabaseName, rdbmsId))
	}
	log.Debug().Msgf("[Response from Spider] Creating RDBMS database: %+v", spResp)
	if spResp.Message != "created" {
		err = fmt.Errorf("unexpected response creating database '%s' in RDBMS '%s': message=%q", req.DatabaseName, rdbmsId, spResp.Message)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return model.RDBMSDatabaseInfo{DatabaseName: req.DatabaseName}, nil
}

// ListRDBMSDatabases lists the logical databases inside an RDBMS instance; adminUserPassword may be empty (some drivers don't require it, see §1.3).
func ListRDBMSDatabases(nsId, rdbmsId, adminUserPassword string) (model.RDBMSDatabaseListResponse, error) {
	var emptyRet model.RDBMSDatabaseListResponse

	rdbmsInfo, err := GetRDBMS(nsId, rdbmsId)
	if err != nil {
		return emptyRet, err
	}

	spReq := spiderRDBMSDatabaseCredentialRequest{
		ConnectionName:     rdbmsInfo.ConnectionName,
		MasterUserPassword: adminUserPassword,
	}
	logReq := spReq
	logReq.MasterUserPassword = "********"

	client := clientManager.NewHttpClient()
	spResp := spiderRDBMSDatabaseListResponse{}
	spiderUrl := fmt.Sprintf("%s/rdbms/%s/databases", model.SpiderRestUrl, rdbmsInfo.Uid)
	log.Debug().Msgf("[Request to Spider] Listing RDBMS databases (url: %s, request: %+v)", spiderUrl, logReq)

	restyResp, err := clientManager.ExecuteHttpRequest(
		client,
		"GET",
		spiderUrl,
		nil,
		clientManager.SetUseBody(spReq),
		&spReq,
		&spResp,
		clientManager.ShortDuration,
	)
	if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, apierr.Wrap(err, fmt.Sprintf("failed to list databases in RDBMS '%s'", rdbmsId))
	}
	log.Debug().Msgf("[Response from Spider] Listing RDBMS databases: %+v", spResp)

	return model.RDBMSDatabaseListResponse{Databases: spResp.Databases}, nil
}

// DeleteRDBMSDatabase deletes a logical database inside an RDBMS instance; an already-gone database is tolerated as success.
func DeleteRDBMSDatabase(nsId, rdbmsId, dbName, adminUserPassword string) error {
	// dbName is a SQL identifier (may contain underscores), not a Tumblebug resource name, so only non-empty is required here.
	if dbName == "" {
		err := fmt.Errorf("dbName is required")
		log.Error().Err(err).Msg("")
		return err
	}

	rdbmsInfo, err := GetRDBMS(nsId, rdbmsId)
	if err != nil {
		return err
	}

	// Repeat delete-then-confirm a few times rather than trusting a single DELETE response —
	// mirrors DeleteRDBMS's own retry cycle for the same class of CSP-side delay.
	const maxDeleteCycles = 10
	const deleteCycleWait = 10 * time.Second

	var lastErr error
	confirmed := false
	for cycle := 1; cycle <= maxDeleteCycles; cycle++ {
		if cycle > 1 {
			log.Warn().Msgf("Database '%s' in RDBMS '%s' not yet confirmed deleted; retrying delete (cycle %d/%d) after %s...", dbName, rdbmsId, cycle, maxDeleteCycles, deleteCycleWait)
			time.Sleep(deleteCycleWait)
		}

		spReq := spiderRDBMSDatabaseCredentialRequest{
			ConnectionName:     rdbmsInfo.ConnectionName,
			MasterUserPassword: adminUserPassword,
		}
		logReq := spReq
		logReq.MasterUserPassword = "********"

		client := clientManager.NewHttpClient()
		spResp := spiderSimpleMsgResp{}
		spiderUrl := fmt.Sprintf("%s/rdbms/%s/databases/%s", model.SpiderRestUrl, rdbmsInfo.Uid, url.PathEscape(dbName))
		log.Debug().Msgf("[Request to Spider] Deleting RDBMS database (url: %s, request: %+v)", spiderUrl, logReq)

		restyResp, delErr := clientManager.ExecuteHttpRequest(
			client,
			"DELETE",
			spiderUrl,
			nil,
			clientManager.SetUseBody(spReq),
			&spReq,
			&spResp,
			clientManager.ShortDuration,
		)
		if delErr = clientManager.HandleHttpResponse(restyResp, delErr); delErr != nil && !apierr.IsNotFound(delErr) {
			lastErr = apierr.Wrap(delErr, fmt.Sprintf("failed to delete database '%s' in RDBMS '%s'", dbName, rdbmsId))
			log.Warn().Err(lastErr).Msgf("Delete attempt %d/%d failed", cycle, maxDeleteCycles)
			continue
		}
		log.Debug().Msgf("[Response from Spider] Deleting RDBMS database: %+v", spResp)
		if delErr == nil && spResp.Message != "deleted" {
			lastErr = fmt.Errorf("unexpected response deleting database '%s' in RDBMS '%s': message=%q", dbName, rdbmsId, spResp.Message)
			log.Warn().Err(lastErr).Msgf("Delete attempt %d/%d failed", cycle, maxDeleteCycles)
			continue
		}
		lastErr = nil

		listResp, listErr := ListRDBMSDatabases(nsId, rdbmsId, adminUserPassword)
		if listErr != nil {
			log.Warn().Err(listErr).Msgf("Delete verify: failed to list databases on cycle %d/%d", cycle, maxDeleteCycles)
			continue
		}
		if !slices.Contains(listResp.Databases, dbName) {
			confirmed = true
			break
		}
		log.Warn().Msgf("Database '%s' still present after delete attempt %d/%d", dbName, cycle, maxDeleteCycles)
	}

	if lastErr != nil {
		log.Error().Err(lastErr).Msg("")
		return lastErr
	}
	if !confirmed {
		log.Warn().Msgf("Database '%s' in RDBMS '%s' not confirmed deleted after %d cycles; trusting last DELETE response", dbName, rdbmsId, maxDeleteCycles)
	}
	return nil
}
