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
	"fmt"
	"net/url"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// Default supported DB engines in Spider
var defaultRDBMSDBEngines = []string{"mysql", "mariadb", "postgresql"}

// GetRDBMSSupport retrieves Tumblebug-style RDBMS support capability details by querying Spider
func GetRDBMSSupport(providerName, regionName, dbEngine string) (model.RDBMSSupportResponse, error) {
	var response model.RDBMSSupportResponse
	response.ResourceType = model.StrRDBMS
	response.Supports = []model.RDBMSMetaInfo{}

	providerName = strings.TrimSpace(providerName)
	regionName = strings.TrimSpace(regionName)
	dbEngine = strings.TrimSpace(strings.ToLower(dbEngine))

	var targetConnConfigs []model.ConnConfig

	// 1. Resolve Target Connections using providerName and regionName
	connNames, err := common.GetConnConfigListByProviderRegionZone(providerName, regionName, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to list connection configs for RDBMS support")
		return response, fmt.Errorf("failed to list connection configs: %w", err)
	}

	for _, name := range connNames {
		connConfig, err := common.GetConnConfig(name)
		if err != nil {
			log.Warn().Err(err).Msgf("Skipping invalid connection config: %s", name)
			continue
		}
		targetConnConfigs = append(targetConnConfigs, connConfig)
	}

	if len(targetConnConfigs) == 0 {
		return response, fmt.Errorf("no matching connection configs found (provider: '%s', region: '%s')", providerName, regionName)
	}

	// 2. Resolve Target DB Engines
	var targetEngines []string
	if dbEngine != "" {
		targetEngines = []string{dbEngine}
	} else {
		targetEngines = defaultRDBMSDBEngines
	}

	client := clientManager.NewHttpClient()
	method := "GET"
	noBody := clientManager.NoBody

	// 3. Query Spider for each connection and engine, and transform response
	for _, connConfig := range targetConnConfigs {
		for _, engine := range targetEngines {
			spiderUrl := fmt.Sprintf("%s/rdbmsmetainfo?ConnectionName=%s&DBEngine=%s",
				model.SpiderRestUrl,
				url.QueryEscape(connConfig.ConfigName),
				url.QueryEscape(engine),
			)

			var spiderMeta model.SpiderRDBMSMetaInfo
			restyResp, err := clientManager.ExecuteHttpRequest(
				client,
				method,
				spiderUrl,
				nil,
				clientManager.SetUseBody(noBody),
				&noBody,
				&spiderMeta,
				clientManager.MediumDuration,
			)

			if err = clientManager.HandleHttpResponse(restyResp, err); err != nil {
				log.Warn().Err(err).Msgf("Spider RDBMS meta info query failed for connection '%s', engine '%s'", connConfig.ConfigName, engine)
				continue
			}

			if spiderMeta.DBEngine == "" {
				continue
			}

			// Map Spider response to Tumblebug lowerCamelCase model
			metaInfo := model.RDBMSMetaInfo{
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
			}

			response.Supports = append(response.Supports, metaInfo)
		}
	}

	return response, nil
}
