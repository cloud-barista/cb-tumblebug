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

package openstackcommon

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	nhnsdk "github.com/cloud-barista/nhncloud-sdk-go"
	nhnostack "github.com/cloud-barista/nhncloud-sdk-go/openstack"
	"github.com/cloud-barista/nhncloud-sdk-go/openstack/compute/v2/servers"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.OpenStack, FindInstancesByUIDs)
}

type openstackCreds struct {
	IdentityEndpoint, Username, Password, DomainName, ProjectID string
}

func getOpenStackCreds(ctx context.Context) (*openstackCreds, error) {
	path := csp.BuildSecretPath(ctx, csptypes.OpenStack)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, err
	}
	c := &openstackCreds{
		IdentityEndpoint: csp.GetString(data, "OS_AUTH_URL"),
		Username:         csp.GetString(data, "OS_USERNAME"),
		Password:         csp.GetString(data, "OS_PASSWORD"),
		DomainName:       csp.GetString(data, "OS_DOMAIN_NAME"),
		ProjectID:        csp.GetString(data, "OS_PROJECT_ID"),
	}
	if c.IdentityEndpoint == "" || c.Username == "" || c.Password == "" {
		return nil, fmt.Errorf("OpenStack credentials incomplete at %s", path)
	}
	return c, nil
}

func newOpenStackComputeClient(ctx context.Context, region string) (*nhnsdk.ServiceClient, error) {
	c, err := getOpenStackCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("OpenStack: cannot get credentials: %w", err)
	}
	provider, err := nhnostack.AuthenticatedClient(nhnsdk.AuthOptions{
		IdentityEndpoint: c.IdentityEndpoint,
		Username:         c.Username,
		Password:         c.Password,
		DomainName:       c.DomainName,
		TenantID:         c.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("OpenStack: authentication failed: %w", err)
	}
	client, err := nhnostack.NewComputeV2(provider, nhnsdk.EndpointOpts{Region: region})
	if err != nil && region != strings.ToUpper(region) {
		client, err = nhnostack.NewComputeV2(provider, nhnsdk.EndpointOpts{Region: strings.ToUpper(region)})
	}
	if err != nil {
		return nil, fmt.Errorf("OpenStack: compute endpoint not found (region=%s): %w", region, err)
	}
	return client, nil
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on OpenStack
// by querying servers.List with Name matching uids.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	client, err := newOpenStackComputeClient(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("OpenStack vmfind: cannot create compute client: %w", err)
	}

	result := make(map[string]string)

	for _, uid := range uids {
		pages, err := servers.List(client, servers.ListOpts{Name: uid}).AllPages()
		if err != nil {
			return nil, fmt.Errorf("OpenStack servers.List by Name failed (region=%s, uid=%s): %w", region, uid, err)
		}
		serverList, err := servers.ExtractServers(pages)
		if err != nil {
			return nil, fmt.Errorf("OpenStack ExtractServers failed: %w", err)
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
		Msg("[OpenStack] FindInstancesByUIDs completed")

	return result, nil
}
