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
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

// spiderRDBMSMetaInfo represents Spider's RDBMSMetaInfo (PascalCase)
type spiderRDBMSMetaInfo struct {
	DBEngine                         string                 `json:"DBEngine"`
	SupportedVersions                []string               `json:"SupportedVersions"`
	DBInstanceSpecOptions            []string               `json:"DBInstanceSpecOptions"`
	StorageTypeOptions               []string               `json:"StorageTypeOptions"`
	StorageSizeRange                 spiderStorageSizeRange `json:"StorageSizeRange"`
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
	DataSource                       map[string]string      `json:"DataSource,omitempty"`
	DataSourceNotes                  map[string]string      `json:"DataSourceNotes,omitempty"`
}

// spiderRDBMSCreateReqInfo represents Spider's RDBMSCreateRequest.ReqInfo (PascalCase)
type spiderRDBMSCreateReqInfo struct {
	Name                string           `json:"Name"`
	VPCName             string           `json:"VPCName"`
	DBEngine            string           `json:"DBEngine"`
	DBEngineVersion     string           `json:"DBEngineVersion"`
	DBInstanceSpec      string           `json:"DBInstanceSpec"`
	StorageSize         string           `json:"StorageSize"`
	StorageType         string           `json:"StorageType,omitempty"`
	Iops                string           `json:"Iops,omitempty"`
	SubnetNames         []string         `json:"SubnetNames,omitempty"`
	SecurityGroupNames  []string         `json:"SecurityGroupNames,omitempty"`
	MasterUserName      string           `json:"MasterUserName"`
	MasterUserPassword  string           `json:"MasterUserPassword"`
	HighAvailability    bool             `json:"HighAvailability"`
	BackupRetentionDays int              `json:"BackupRetentionDays,omitempty"`
	PublicAccess        bool             `json:"PublicAccess"`
	DeletionProtection  bool             `json:"DeletionProtection"`
	TagList             []model.KeyValue `json:"TagList,omitempty"`
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

// spiderRDBMSInfo represents Spider's RDBMSInfo response (PascalCase)
type spiderRDBMSInfo struct {
	IId                 model.IID        `json:"IId"`
	VpcIID              model.IID        `json:"VpcIID"`
	DBEngine            string           `json:"DBEngine"`
	DBEngineVersion     string           `json:"DBEngineVersion"`
	DBInstanceSpec      string           `json:"DBInstanceSpec"`
	StorageSize         string           `json:"StorageSize"`
	StorageType         string           `json:"StorageType,omitempty"`
	Iops                string           `json:"Iops,omitempty"`
	SubnetIIDs          []model.IID      `json:"SubnetIIDs,omitempty"`
	SecurityGroupIIDs   []model.IID      `json:"SecurityGroupIIDs,omitempty"`
	MasterUserName      string           `json:"MasterUserName"`
	PublicAccess        bool             `json:"PublicAccess"`
	HighAvailability    bool             `json:"HighAvailability"`
	BackupRetentionDays int              `json:"BackupRetentionDays,omitempty"`
	BackupTime          string           `json:"BackupTime,omitempty"`
	DeletionProtection  bool             `json:"DeletionProtection"`
	Encryption          bool             `json:"Encryption,omitempty"`
	Endpoint            string           `json:"Endpoint,omitempty"`
	Status              string           `json:"Status"`
	CreatedTime         string           `json:"CreatedTime,omitempty"`
	KeyValueList        []model.KeyValue `json:"KeyValueList,omitempty"`
	TagList             []model.KeyValue `json:"TagList,omitempty"`
}

// rdbmsDataSourceKeyNames maps Spider's PascalCase RDBMSMetaInfo field names (as used in
// DataSource/DataSourceNotes map keys) to this API's own camelCase field names, so a
// caller reading dataSource never sees a key that doesn't match any field in the same
// response.
var rdbmsDataSourceKeyNames = map[string]string{
	"SupportedVersions":       "supportedVersions",
	"DBInstanceSpecOptions":   "dbInstanceSpecOptions",
	"StorageTypeOptions":      "storageTypeOptions",
	"StorageSizeRange":        "storageSizeRange",
	"StorageSizeRange.Min":    "storageSizeRange.min",
	"StorageSizeRange.Max":    "storageSizeRange.max",
	"BackupRetentionRange":    "backupRetentionRange",
	"DBInstanceSpecOptionsV2": "dbInstanceSpecOptions", // defensive: tolerate a future renamed variant
}

// translateRDBMSDataSourceKey renames a Spider-side DataSource/DataSourceNotes map key to
// this API's own field name via rdbmsDataSourceKeyNames, falling back to a lowercased
// first letter for any key not in that table.
func translateRDBMSDataSourceKey(key string) string {
	if mapped, ok := rdbmsDataSourceKeyNames[key]; ok {
		return mapped
	}
	if key == "" {
		return key
	}
	return strings.ToLower(key[:1]) + key[1:]
}

// normalizeStorageTypeKey canonicalizes a storage type identifier for lookup, so that
// formatting differences between assets/rdbmsinfo.yaml's YAML keys and CB-Spider's literal
// StorageTypeOptions strings (e.g. "General_HDD" vs "General HDD") don't cause a real,
// documented entry to be missed just because it isn't a byte-for-byte match.
func normalizeStorageTypeKey(s string) string {
	s = strings.ToLower(s)
	for _, sep := range []string{"_", " ", "-"} {
		s = strings.ReplaceAll(s, sep, "")
	}
	return s
}

// getStorageTypeConfig retrieves the assets/rdbmsinfo.yaml entry (loaded into
// common.RuntimeRDBMSInfo at server startup; see main.go's setConfig) for a specific
// provider/storage type. Shared by buildStorageTypeNotes (user-facing notes) and
// validateRDBMSCreateRequest (size/iops/spec constraint checks). Tries an exact key match
// first, then falls back to a normalized match (see normalizeStorageTypeKey).
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
	return model.RDBMSStorageTypeConfig{}, false
}

// buildStorageTypeConstraints renders a storage type's machine-checkable constraints
// (iops range, size floor, spec/machine-series compatibility) as one human-readable
// sentence, for the StorageTypeNote.Constraints field.
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

// buildStorageTypeNotes enriches Spider's raw storageTypeOptions list with user-facing
// metadata from assets/rdbmsinfo.yaml (display names, descriptions, constraints,
// recommendations) to help Portal/UI users and automation scripts make informed storage
// type selections before RDBMS creation.
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

// buildRDBMSStaticFields collapses Spider's DataSource/DataSourceNotes maps into a single
// list of only the fields marked "Static", so callers only ever see fields worth distrusting.
func buildRDBMSStaticFields(dataSource, dataSourceNotes map[string]string) []model.RDBMSStaticField {
	var fields []model.RDBMSStaticField
	for k, v := range dataSource {
		if !strings.EqualFold(v, "Static") {
			continue
		}
		fields = append(fields, model.RDBMSStaticField{
			Field: translateRDBMSDataSourceKey(k),
			Note:  dataSourceNotes[k],
		})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Field < fields[j].Field })
	return fields
}

// GetRDBMSSupport retrieves Tumblebug-style RDBMS support capability details for a single
// connection/engine by querying Spider live. providerName/regionName/dbEngine are all
// required to keep this endpoint to one Spider call: unlike ObjectStorage's support
// endpoint (a static in-memory map), each call here is a live query, so an unfiltered
// "all connections/all engines" mode would fan out into many slow Spider round trips per
// request.
func GetRDBMSSupport(providerName, regionName, dbEngine string) (model.RDBMSSupportResponse, error) {
	var response model.RDBMSSupportResponse
	response.ResourceType = model.StrRDBMS

	providerName = strings.TrimSpace(providerName)
	regionName = strings.TrimSpace(regionName)
	dbEngine = strings.TrimSpace(strings.ToLower(dbEngine))

	if providerName == "" || regionName == "" || dbEngine == "" {
		return response, fmt.Errorf("providerName, regionName, and dbEngine are required")
	}

	// 1. Resolve the target connection (providerName + regionName normally resolves to a
	// single connection within the caller's credential-holder scope; if more than one
	// matches, the first (name-sorted) is used deterministically).
	connNames, err := common.GetConnConfigListByProviderRegionZone(providerName, regionName, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to list connection configs for RDBMS support")
		return response, fmt.Errorf("failed to list connection configs: %w", err)
	}
	if len(connNames) == 0 {
		return response, fmt.Errorf("no matching connection config found (provider: '%s', region: '%s')", providerName, regionName)
	}
	sort.Strings(connNames)
	if len(connNames) > 1 {
		log.Warn().Msgf("Multiple connections matched provider '%s' region '%s'; using '%s'", providerName, regionName, connNames[0])
	}

	connConfig, err := common.GetConnConfig(connNames[0])
	if err != nil {
		return response, fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
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
		ProviderName:                     connConfig.ProviderName,
		RegionName:                       connConfig.RegionZoneInfo.AssignedRegion,
		ConnectionName:                   connConfig.ConfigName,
		DBEngine:                         spiderMeta.DBEngine,
		SupportedVersions:                spiderMeta.SupportedVersions,
		DBInstanceSpecOptions:            spiderMeta.DBInstanceSpecOptions,
		StorageTypeOptions:               spiderMeta.StorageTypeOptions,
		StorageSizeRange:                 model.StorageSizeRange{Min: spiderMeta.StorageSizeRange.Min, Max: spiderMeta.StorageSizeRange.Max},
		SupportsHighAvailability:         spiderMeta.SupportsHighAvailability,
		SupportsBackup:                   spiderMeta.SupportsBackup,
		BackupRetentionRange:             spiderMeta.BackupRetentionRange,
		SupportsPublicAccess:             spiderMeta.SupportsPublicAccess,
		SupportsDeletionProtection:       spiderMeta.SupportsDeletionProtection,
		SupportsEncryption:               spiderMeta.SupportsEncryption,
		SupportsStorageTypeSelection:     spiderMeta.SupportsStorageTypeSelection,
		SupportsStorageSizeConfiguration: spiderMeta.SupportsStorageSizeConfiguration,
		RequiresSubnet:                   spiderMeta.RequiresSubnet,
		RequiresSecurityGroup:            spiderMeta.RequiresSecurityGroup,
		Notes: model.RDBMSNotes{
			StorageTypes: buildStorageTypeNotes(connConfig.ProviderName, spiderMeta.StorageTypeOptions),
			StaticFields: buildRDBMSStaticFields(spiderMeta.DataSource, spiderMeta.DataSourceNotes),
		},
	}

	return response, nil
}

// ========== RDBMS Instance Lifecycle (Create/List/Get/Delete) ==========

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

// getSpiderRDBMSMetaInfo queries Spider live for a single connection/engine's RDBMSMetaInfo.
// Used at create time so validation always reflects the CSP's current capabilities
// (see docs/feature_guide/rdbms-management.md §2.2 for why this is not backed by static config).
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

// applyRDBMSCreateDefaults fills DBEngineVersion/DBInstanceSpec/StorageType/StorageSize from
// live RDBMSMetaInfo when req.AutoFillDefaults is set and the field was left empty/zero.
// Selection is always "first supported option" — CB-Spider's option lists carry no
// cost/performance ordering, so this is a capability-valid pick, not a recommendation.
// StorageSize falls back to StorageSizeRange.Min even when SupportsStorageSizeConfiguration
// is false, since CB-Spider's create schema still requires a StorageSize value for those CSPs.
// safeStorageTypePreference defines preferred storage type selection order for autoFillDefaults,
// prioritizing types that don't require additional parameters (e.g., iops) or have special size constraints.
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
	if req.DBEngineVersion == "" && len(meta.SupportedVersions) > 0 {
		req.DBEngineVersion = meta.SupportedVersions[0]
	}
	if req.DBInstanceSpec == "" && len(meta.DBInstanceSpecOptions) > 0 {
		req.DBInstanceSpec = meta.DBInstanceSpecOptions[0]
	}

	// StorageType: prefer safe defaults that don't require iops or have size constraints
	if req.StorageType == "" && meta.SupportsStorageTypeSelection && len(meta.StorageTypeOptions) > 0 {
		providerKey := strings.ToLower(providerName)
		if preferences, exists := safeStorageTypePreference[providerKey]; exists {
			for _, preferred := range preferences {
				for _, available := range meta.StorageTypeOptions {
					if strings.EqualFold(preferred, available) {
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
		// fallback: use first available if no safe preference matched
		if req.StorageType == "" && len(meta.StorageTypeOptions) > 0 {
			req.StorageType = meta.StorageTypeOptions[0]
			log.Warn().Msgf("AutoFillDefaults: no safe preference, using first storageType=%s", req.StorageType)
		}
	}

	if req.StorageSize <= 0 && meta.StorageSizeRange.Min > 0 {
		req.StorageSize = meta.StorageSizeRange.Min
	}
}

// matchesAnySpecPattern reports whether spec matches any of the glob patterns (e.g.
// "mysql.n4.*") from a storage type's compatibleSpecs/incompatibleSpecs in
// assets/rdbmsinfo.yaml. An empty pattern list matches nothing.
func matchesAnySpecPattern(spec string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := filepath.Match(pattern, spec); err == nil && matched {
			return true
		}
	}
	return false
}

// validateRDBMSCreateRequest checks the request against the CSP's live capability flags
// and assets/rdbmsinfo.yaml's storage type constraints, so unsupported combinations fail
// fast before provisioning.
func validateRDBMSCreateRequest(meta spiderRDBMSMetaInfo, req model.RDBMSCreateRequest, providerName string) error {
	if meta.RequiresSubnet && len(req.SubnetIds) == 0 {
		return fmt.Errorf("subnetIds required for %s", providerName)
	}
	if meta.RequiresSecurityGroup && len(req.SecurityGroupIds) == 0 {
		return fmt.Errorf("securityGroupIds required for %s", providerName)
	}
	if !meta.SupportsStorageTypeSelection && req.StorageType != "" {
		return fmt.Errorf("storageType is not configurable for %s; omit it", providerName)
	}

	// General storage size range check
	if meta.SupportsStorageSizeConfiguration {
		if req.StorageSize < meta.StorageSizeRange.Min || (meta.StorageSizeRange.Max > 0 && req.StorageSize > meta.StorageSizeRange.Max) {
			return fmt.Errorf("storageSize %d out of range [%d-%d] for %s", req.StorageSize, meta.StorageSizeRange.Min, meta.StorageSizeRange.Max, providerName)
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
	}

	return nil
}

// updateRDBMSInfoFromSpider copies Spider's response fields into a Tumblebug RDBMSInfo.
// Status is set verbatim from Spider (Creating/Available/Deleting/Stopped/Error) since,
// once CB-Spider is successfully reached, it is authoritative for the CSP-reported state.
func updateRDBMSInfoFromSpider(rdbmsInfo *model.RDBMSInfo, sp spiderRDBMSInfo) {
	rdbmsInfo.CspResourceName = sp.IId.NameId
	rdbmsInfo.CspResourceId = sp.IId.SystemId
	rdbmsInfo.DBEngine = sp.DBEngine
	rdbmsInfo.DBEngineVersion = sp.DBEngineVersion
	rdbmsInfo.DBInstanceSpec = sp.DBInstanceSpec
	rdbmsInfo.StorageType = sp.StorageType
	rdbmsInfo.Iops = sp.Iops
	if size, err := strconv.Atoi(sp.StorageSize); err == nil {
		rdbmsInfo.StorageSize = size
	}
	rdbmsInfo.MasterUserName = sp.MasterUserName
	rdbmsInfo.PublicAccess = sp.PublicAccess
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
	// rdbmsCreationPollInterval matches the poll cadence CB-Spider's own
	// rdbms-mysql-test suite uses while waiting for Status to leave "Creating".
	rdbmsCreationPollInterval = 30 * time.Second
	// rdbmsCreationTimeout is sized off observed CB-Spider test results, not the
	// suite's own MAX_WAIT_SEC=3600 (a CI safety net, not a typical duration):
	// times ranged ~2m30s (Alibaba) to ~11m24s (NCP/NHN) across 9 CSPs. 20 minutes
	// gives ~2x margin over the slowest observed CSP.
	rdbmsCreationTimeout = 20 * time.Minute
)

// waitForRDBMSAvailable polls Spider GET until Status leaves "Creating", persisting
// progress to kvstore on each poll so a concurrent GetRDBMS call observes it while
// this call blocks. Returns the terminal spiderRDBMSInfo, or an error if the poll
// times out; a non-"Available" terminal status is left for the caller to handle.
func waitForRDBMSAvailable(rdbmsKey string, rdbmsInfo *model.RDBMSInfo) (spiderRDBMSInfo, error) {
	client := clientManager.NewHttpClient()
	noBody := clientManager.NoBody
	getUrl := fmt.Sprintf("%s/rdbms/%s?ConnectionName=%s", model.SpiderRestUrl, rdbmsInfo.Uid, rdbmsInfo.ConnectionName)
	deadline := time.Now().Add(rdbmsCreationTimeout)

	for {
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
			log.Warn().Err(err).Msgf("RDBMS %s status poll failed; retrying", rdbmsInfo.Uid)
		} else if spResp.Status != "Creating" {
			return spResp, nil
		} else {
			log.Info().Msgf("RDBMS %s still creating; will poll again in %s", rdbmsInfo.Uid, rdbmsCreationPollInterval)
			updateRDBMSInfoFromSpider(rdbmsInfo, spResp)
			if val, mErr := json.Marshal(rdbmsInfo); mErr == nil {
				_ = kvstore.Put(rdbmsKey, string(val))
			}
		}

		if time.Now().After(deadline) {
			return spiderRDBMSInfo{}, fmt.Errorf("timed out after %s waiting for RDBMS %s to leave Creating", rdbmsCreationTimeout, rdbmsInfo.Uid)
		}
		time.Sleep(rdbmsCreationPollInterval)
	}
}

// CreateRDBMS creates a managed RDBMS instance via CB-Spider, polling internally until
// the instance leaves "Creating" so it returns the final Available/Failed state directly
// rather than a caller-facing "Creating" placeholder (see docs/feature_guide/rdbms-management.md §4.1).
func CreateRDBMS(ctx context.Context, nsId string, req model.RDBMSCreateRequest) (model.RDBMSInfo, error) {
	var emptyRet model.RDBMSInfo
	var rdbmsInfo model.RDBMSInfo

	// 1. Validate input parameters
	if err := common.CheckString(nsId); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err := validate.Struct(req); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err := common.CheckString(req.Name); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	connConfig, err := common.GetConnConfig(req.ConnectionName)
	if err != nil {
		err = fmt.Errorf("cannot retrieve ConnectionConfig %s", err.Error())
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if !isRDBMSSupported(connConfig.ProviderName) {
		err = fmt.Errorf("RDBMS is not supported for CSP: %s", connConfig.ProviderName)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 2. Resolve Tumblebug vNet/subnet/securityGroup IDs to CSP names
	vpcName, subnetNames, sgNames, err := resolveRDBMSNetwork(nsId, req.VNetId, req.SubnetIds, req.SecurityGroupIds)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// 3. Validate against live RDBMSMetaInfo (always live; see §2.2)
	meta, err := getSpiderRDBMSMetaInfo(req.ConnectionName, req.DBEngine)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	applyRDBMSCreateDefaults(meta, &req, connConfig.ProviderName)
	if req.DBEngineVersion == "" {
		err = fmt.Errorf("dbEngineVersion required (or set autoFillDefaults=true)")
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if req.DBInstanceSpec == "" {
		err = fmt.Errorf("dbInstanceSpec required (or set autoFillDefaults=true)")
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if req.StorageSize <= 0 {
		err = fmt.Errorf("storageSize required (or set autoFillDefaults=true)")
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if err := validateRDBMSCreateRequest(meta, req, connConfig.ProviderName); err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

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
	rdbmsInfo.MasterUserName = req.MasterUserName
	rdbmsInfo.HighAvailability = req.HighAvailability
	rdbmsInfo.BackupRetentionDays = req.BackupRetentionDays
	rdbmsInfo.PublicAccess = req.PublicAccess
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

	// 7. Call Spider API to create the RDBMS instance
	spReq := spiderRDBMSCreateRequest{
		ConnectionName: req.ConnectionName,
		ReqInfo: spiderRDBMSCreateReqInfo{
			Name:                rdbmsInfo.Uid,
			VPCName:             vpcName,
			DBEngine:            req.DBEngine,
			DBEngineVersion:     req.DBEngineVersion,
			DBInstanceSpec:      req.DBInstanceSpec,
			StorageSize:         strconv.Itoa(req.StorageSize),
			StorageType:         req.StorageType,
			Iops:                req.Iops,
			SubnetNames:         subnetNames,
			SecurityGroupNames:  sgNames,
			MasterUserName:      req.MasterUserName,
			MasterUserPassword:  req.MasterUserPassword,
			HighAvailability:    req.HighAvailability,
			BackupRetentionDays: req.BackupRetentionDays,
			PublicAccess:        req.PublicAccess,
			DeletionProtection:  req.DeletionProtection,
			TagList:             req.TagList,
		},
	}

	client := clientManager.NewHttpClient()
	spResp := spiderRDBMSInfo{}
	url := fmt.Sprintf("%s/rdbms", model.SpiderRestUrl)
	log.Debug().Msgf("[Request to Spider] Creating RDBMS (url: %s, request: %+v)", url, spReq)

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

	// 8. If Spider returned before the instance left "Creating", poll until it does;
	// this call blocks so the caller receives the final Available/Failed state directly.
	if spResp.Status == "Creating" {
		log.Info().Msgf("RDBMS %s creation started; polling until Available (timeout: %s)", rdbmsInfo.Id, rdbmsCreationTimeout)
		updateRDBMSInfoFromSpider(&rdbmsInfo, spResp)
		if val, mErr := json.Marshal(rdbmsInfo); mErr == nil {
			_ = kvstore.Put(rdbmsKey, string(val))
		}

		polled, pollErr := waitForRDBMSAvailable(rdbmsKey, &rdbmsInfo)
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
		spResp = polled
	}

	if spResp.Status != "Available" {
		err = fmt.Errorf("RDBMS '%s' reached status '%s' after creation, expected 'Available'", rdbmsInfo.Id, spResp.Status)
		log.Error().Err(err).Msg("")
		updateRDBMSInfoFromSpider(&rdbmsInfo, spResp)
		model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonCreationFailed, err.Error())
		rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
		rdbmsInfo.SystemMessage = err.Error()
		if failVal, marshalErr := json.Marshal(rdbmsInfo); marshalErr == nil {
			_ = kvstore.Put(rdbmsKey, string(failVal))
		}
		return emptyRet, err
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
			err = fmt.Errorf("DELETE failed for RDBMS %s: %w", rdbmsInfo.Uid, delErr)
			log.Error().Err(err).Msg("")
			model.SetCondition(&rdbmsInfo.Conditions, model.ConditionReady, model.ConditionFalse, model.ReasonDeletionFailed, err.Error())
			rdbmsInfo.Status = model.DeriveRDBMSStatus(rdbmsInfo.Conditions)
			rdbmsInfo.SystemMessage = err.Error()
			if failVal, marshalErr := json.Marshal(rdbmsInfo); marshalErr == nil {
				_ = kvstore.Put(rdbmsKey, string(failVal))
			}
			return err
		}

		if delErr == nil {
			// RDBMS deletion can take minutes; a bounded poll only confirms the fast
			// path, so an inconclusive result after all attempts still trusts the
			// DELETE response and proceeds with metadata cleanup (as with ObjectStorage).
			getUrl := fmt.Sprintf("%s/rdbms/%s?ConnectionName=%s", model.SpiderRestUrl, rdbmsInfo.Uid, rdbmsInfo.ConnectionName)
			deleted, verifyErr := PollResourceDeletedViaSpider(getUrl, nil, DefaultPollMaxAttempts, DefaultPollInterval)
			if !deleted {
				if verifyErr != nil {
					log.Warn().Err(verifyErr).Msgf("RDBMS %s verification GET failed; trusting DELETE response and removing metadata", rdbmsInfo.Uid)
				} else {
					log.Warn().Msgf("RDBMS %s still visible via GET after %d attempts; trusting DELETE response and removing metadata", rdbmsInfo.Uid, DefaultPollMaxAttempts)
				}
			}
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

// PruneRDBMS purges Tumblebug metadata for RDBMS instances diagnosed by Reconcile
// as missing on CSP (ConditionSynced.Reason == ReasonCspResourceMissing). This is
// the only place, besides an explicit DeleteRDBMS call, that removes RDBMS metadata.
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
