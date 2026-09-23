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

package ncp

import (
	"context"
	"fmt"
	"strings"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.NCP, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on NAVER Cloud Platform
// by querying GetServerInstanceList with ServerName matching uids.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	client, err := newClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("NCP vmfind: cannot create client: %w", err)
	}

	result := make(map[string]string)
	regCode := strings.ToUpper(region)

	for _, uid := range uids {
		req := &vserver.GetServerInstanceListRequest{
			RegionCode: ncloud.String(regCode),
			ServerName: ncloud.String(uid),
		}
		resp, err := client.V2Api.GetServerInstanceList(req)
		if err != nil {
			return nil, fmt.Errorf("NCP GetServerInstanceList by ServerName failed (region=%s, uid=%s): %w", region, uid, err)
		}
		if resp != nil {
			for _, inst := range resp.ServerInstanceList {
				if inst.ServerName != nil && *inst.ServerName == uid && inst.ServerInstanceNo != nil {
					result[uid] = *inst.ServerInstanceNo
					break
				}
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[NCP] FindInstancesByUIDs completed")

	return result, nil
}
