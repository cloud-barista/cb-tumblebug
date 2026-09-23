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

package nhn

import (
	"context"
	"fmt"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/servers"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.NHN, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on NHN Cloud
// by querying servers.List with Name matching uids.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	client, err := newComputeClient(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("NHN vmfind: cannot create compute client: %w", err)
	}

	result := make(map[string]string)

	for _, uid := range uids {
		pages, err := servers.List(client, servers.ListOpts{Name: uid}).AllPages()
		if err != nil {
			return nil, fmt.Errorf("NHN servers.List by Name failed (region=%s, uid=%s): %w", region, uid, err)
		}
		serverList, err := servers.ExtractServers(pages)
		if err != nil {
			return nil, fmt.Errorf("NHN ExtractServers failed: %w", err)
		}
		for _, s := range serverList {
			if s.Name == uid {
				result[uid] = s.ID
				break
			}
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[NHN] FindInstancesByUIDs completed")

	return result, nil
}
