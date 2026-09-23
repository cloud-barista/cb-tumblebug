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

package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterFindVMsByUIDHandler(csptypes.Azure, FindInstancesByUIDs)
}

// FindInstancesByUIDs looks up whether instances with the given UIDs exist on Azure ARM
// by matching vm.Name against uids in the given region.
func FindInstancesByUIDs(ctx context.Context, region string, uids []string) (map[string]string, error) {
	if len(uids) == 0 {
		return map[string]string{}, nil
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("Azure vmfind: cannot get credentials: %w", err)
	}

	vmClient, err := newVMClient(creds)
	if err != nil {
		return nil, fmt.Errorf("Azure vmfind: cannot create client: %w", err)
	}

	want := make(map[string]struct{}, len(uids))
	for _, id := range uids {
		want[strings.ToLower(id)] = struct{}{}
	}

	result := make(map[string]string)
	pager := vmClient.NewListAllPager(nil)

	for pager.More() {
		page, perr := pager.NextPage(ctx)
		if perr != nil {
			return nil, fmt.Errorf("Azure ListAll VMs failed: %w", perr)
		}
		for _, vm := range page.Value {
			if vm == nil || vm.ID == nil || vm.Name == nil {
				continue
			}
			if region != "" && !strings.EqualFold(ptrStr(vm.Location), region) {
				continue
			}
			vmName := *vm.Name
			if _, ok := want[strings.ToLower(vmName)]; ok {
				result[vmName] = *vm.ID
			}
		}
		if len(result) == len(want) {
			break
		}
	}

	log.Debug().
		Str("region", region).
		Int("queried", len(uids)).
		Int("found", len(result)).
		Msg("[Azure] FindInstancesByUIDs completed")

	return result, nil
}
