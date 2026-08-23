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

// Package kt provides direct KT Cloud VPC (D1, OpenStack-based) SDK calls (truth surface).
// Public IPs on KT are managed through a separate NSM API and are not covered as residuals;
// leftover custom tiers (KT's subnet equivalent, account-wide and persistent) are.
package kt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	oscommon "github.com/cloud-barista/cb-tumblebug/src/core/csp/openstackcommon"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	ktsdk "github.com/cloud-barista/ktcloudvpc-sdk-go"
	ktostack "github.com/cloud-barista/ktcloudvpc-sdk-go/openstack"
	ktpip "github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/compute/v2/extensions/floatingips"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/compute/v2/servers"
	ktrules "github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/networking/v2/extensions/fwaas_v2/rules"
	ktpf "github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/networking/v2/extensions/layer3/portforwarding"
	ktnat "github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/networking/v2/extensions/layer3/staticnat"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/openstack/networking/v2/subnets"
	"github.com/cloud-barista/ktcloudvpc-sdk-go/pagination"
	"github.com/rs/zerolog/log"
)

func init() {
	csp.RegisterInventoryHandlers(csptypes.KT, csp.InventoryHandlers{
		ListVMs:         ListVMs,
		ListResiduals:   ListResiduals,
		DeleteResiduals: DeleteResiduals,
	})
	csp.RegisterRemediationTerminateHandler(csptypes.KT, BatchTerminateInstances)
	csp.RegisterBatchVMStatusHandler(csptypes.KT, BatchDescribeInstanceStatuses)
}

const concurrency = 3

type creds struct {
	IdentityEndpoint, Username, Password, DomainName, ProjectID string
}

func getCreds(ctx context.Context) (*creds, error) {
	path := csp.BuildSecretPath(ctx, csptypes.KT)
	data, err := csp.ReadOpenBaoSecret(ctx, path)
	if err != nil {
		return nil, err
	}
	c := &creds{
		IdentityEndpoint: csp.GetString(data, "KT_IDENTITY_ENDPOINT"),
		Username:         csp.GetString(data, "KT_USERNAME"),
		Password:         csp.GetString(data, "KT_PASSWORD"),
		DomainName:       csp.GetString(data, "KT_DOMAIN_NAME"),
		ProjectID:        csp.GetString(data, "KT_PROJECT_ID"),
	}
	if c.IdentityEndpoint == "" || c.Username == "" || c.Password == "" || c.ProjectID == "" {
		return nil, fmt.Errorf("KT credentials incomplete at %s", path)
	}
	return c, nil
}

// newComputeClient authenticates and returns a Nova client; KT scopes compute endpoints by zone.
func newComputeClient(ctx context.Context, zone string) (*ktsdk.ServiceClient, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("KT: cannot get credentials: %w", err)
	}
	provider, err := ktostack.AuthenticatedClient(ktsdk.AuthOptions{
		IdentityEndpoint: c.IdentityEndpoint, Username: c.Username, Password: c.Password,
		DomainName: c.DomainName, TenantID: c.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("KT: authentication failed: %w", err)
	}
	client, err := ktostack.NewComputeV2(provider, ktsdk.EndpointOpts{Region: zone})
	if err != nil {
		// The hint may be TB's region name rather than the zone; use the catalog's compute endpoint.
		client, err = ktostack.NewComputeV2(provider, ktsdk.EndpointOpts{})
	}
	if err != nil {
		return nil, fmt.Errorf("KT: compute endpoint not found (zone=%s): %w", zone, err)
	}
	return client, nil
}

// zoneArg prefers the explicit zone; callers passing only a region get it used as the zone.
func zoneArg(region, zone string) string {
	if zone != "" {
		return zone
	}
	return region
}

func listServers(ctx context.Context, zone string) ([]servers.Server, error) {
	client, err := newComputeClient(ctx, zone)
	if err != nil {
		return nil, err
	}
	pages, err := servers.List(client, servers.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("KT servers.List failed (zone=%s): %w", zone, err)
	}
	return servers.ExtractServers(pages)
}

// ListVMs lists every server in the zone directly from KT Cloud.
func ListVMs(ctx context.Context, region, zone string) ([]csp.VMRecord, error) {
	list, err := listServers(ctx, zoneArg(region, zone))
	if err != nil {
		return nil, err
	}
	out := make([]csp.VMRecord, 0, len(list))
	for _, s := range list {
		out = append(out, csp.VMRecord{CspResourceId: s.ID, Name: s.Name, Status: oscommon.StateToTBStatus(s.Status),
			PublicIP: oscommon.PublicIPOf(s.Addresses), Tags: s.Metadata})
	}
	return out, nil
}

// BatchDescribeInstanceStatuses returns TB statuses for the given server IDs (region argument = zone).
func BatchDescribeInstanceStatuses(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	list, err := listServers(ctx, region)
	if err != nil {
		return nil, err
	}
	want := make(map[string]struct{}, len(instanceIds))
	for _, id := range instanceIds {
		want[id] = struct{}{}
	}
	for _, s := range list {
		if _, ok := want[s.ID]; ok {
			result[s.ID] = oscommon.StateToTBStatus(s.Status)
		}
	}
	return result, nil
}

// BatchTerminateInstances deletes the given servers (region argument = zone). Like Spider, it first
// removes the servers' firewall policies, port forwarding, and public IPs — KT refuses to delete
// those once the server is gone.
func BatchTerminateInstances(ctx context.Context, region string, instanceIds []string) (map[string]string, error) {
	result := make(map[string]string, len(instanceIds))
	if len(instanceIds) == 0 {
		return result, nil
	}
	client, err := newComputeClient(ctx, region)
	if err != nil {
		return nil, err
	}
	if err := cleanupServerNetworking(ctx, region, instanceIds); err != nil {
		log.Warn().Err(err).Msg("[KT] pre-terminate networking cleanup incomplete; continuing with server deletion")
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, id := range instanceIds {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			if derr := servers.Delete(client, id).ExtractErr(); derr != nil {
				log.Warn().Err(derr).Msgf("[KT] servers.Delete failed for %s", id)
				return
			}
			mu.Lock()
			result[id] = model.StatusTerminating
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return result, nil
}

// systemTiers are KT's built-in tiers that must never be treated as residuals.
var systemTiers = map[string]bool{"private": true, "dmz": true, "external": true, "nlb-subnet": true}

// newNetworkClient returns a Neutron-style client for KT tiers (zone-scoped like compute).
func newNetworkClient(ctx context.Context, zone string) (*ktsdk.ServiceClient, error) {
	c, err := getCreds(ctx)
	if err != nil {
		return nil, fmt.Errorf("KT: cannot get credentials: %w", err)
	}
	provider, err := ktostack.AuthenticatedClient(ktsdk.AuthOptions{
		IdentityEndpoint: c.IdentityEndpoint, Username: c.Username, Password: c.Password,
		DomainName: c.DomainName, TenantID: c.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("KT: authentication failed: %w", err)
	}
	client, err := ktostack.NewNetworkV2(provider, ktsdk.EndpointOpts{Name: "neutron", Region: zone})
	if err != nil {
		client, err = ktostack.NewNetworkV2(provider, ktsdk.EndpointOpts{Name: "neutron"})
	}
	if err != nil {
		return nil, fmt.Errorf("KT: network endpoint not found (zone=%s): %w", zone, err)
	}
	return client, nil
}

var ipv4Pattern = regexp.MustCompile(`(?:\d{1,3}\.){3}\d{1,3}`)

// addrKeys returns the lookup keys of a KT firewall address: the bare name/IP plus any IPv4
// embedded in object names such as "PF_211.34.246.123_1_65535_TCP" (inbound rules reference
// the port-forwarding object, not the IP).
func addrKeys(name string) []string {
	name = strings.TrimSpace(name)
	keys := []string{strings.TrimSuffix(name, "/32")}
	return append(keys, ipv4Pattern.FindAllString(name, -1)...)
}

// ListCustomTiers returns KT custom tiers (NetworkID -> name) in the zone; system tiers are excluded.
// KT has no deletable VPC: a TB vNet is "gone" once none of its tiers remain.
func ListCustomTiers(ctx context.Context, region, zone string) (map[string]string, error) {
	client, err := newNetworkClient(ctx, zoneArg(region, zone))
	if err != nil {
		return nil, err
	}
	tiers := map[string]string{}
	pager := subnets.List(client, subnets.ListOpts{Page: 1, Size: 2000, NetworkType: "ALL"})
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := subnets.ExtractSubnets(page)
		if err != nil {
			return false, err
		}
		for _, t := range list {
			name := t.RefName
			if name == "" {
				name = t.NetworkName
			}
			if t.IsCustom && !systemTiers[strings.ToLower(name)] {
				tiers[t.NetworkID] = name
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("KT tier listing failed: %w", err)
	}
	return tiers, nil
}

// ListResiduals lists custom tiers (non-system, isCustom) — leftovers of failed or half-cleaned vNet
// runs — and the Fortigate firewall policies that still reference TB-named tiers (KT refuses to delete
// a tier while a policy references it). Firewall residuals carry the referenced tier's name so the
// caller's attribution rule (TB uid / "-shared-") applies to both.
func ListResiduals(ctx context.Context, region, zone string) ([]csp.ResidualResource, error) {
	client, err := newNetworkClient(ctx, zoneArg(region, zone))
	if err != nil {
		return nil, err
	}
	var out []csp.ResidualResource
	tierNames := map[string]string{} // NetworkID -> tier name (custom tiers only)
	pager := subnets.List(client, subnets.ListOpts{Page: 1, Size: 2000, NetworkType: "ALL"})
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := subnets.ExtractSubnets(page)
		if err != nil {
			return false, err
		}
		for _, t := range list {
			name := t.RefName
			if name == "" {
				name = t.NetworkName
			}
			if !t.IsCustom || systemTiers[strings.ToLower(name)] {
				continue
			}
			tierNames[t.NetworkID] = name
			out = append(out, csp.ResidualResource{Type: "tier", Id: t.NetworkID, Name: name, Zone: zoneArg(region, zone),
				Detail: strings.TrimSpace(t.CIDR + " " + t.Status)})
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("KT tier listing failed: %w", err)
	}
	if len(tierNames) == 0 {
		return out, nil
	}
	// tier CIDRs, to attribute NAT/port-forwarding entries by their mapped private IP
	tierByCIDR := map[*net.IPNet]string{}
	for _, t := range out {
		cidr := strings.Fields(t.Detail)
		if len(cidr) == 0 {
			continue
		}
		if _, n, perr := net.ParseCIDR(cidr[0]); perr == nil {
			tierByCIDR[n] = t.Name
		}
	}
	tierOfIP := func(ip string) string {
		addr := net.ParseIP(strings.TrimSuffix(ip, "/32"))
		if addr == nil {
			return ""
		}
		for n, name := range tierByCIDR {
			if n.Contains(addr) {
				return name
			}
		}
		return ""
	}
	// IPs of live servers: their NAT/PF/firewall entries are in use, not residuals.
	liveIPs := map[string]bool{}
	if servers, lerr := listServers(ctx, zoneArg(region, zone)); lerr == nil {
		for _, sv := range servers {
			for _, v := range sv.Addresses {
				if list, ok := v.([]interface{}); ok {
					for _, a := range list {
						if m, ok := a.(map[string]interface{}); ok {
							if addr, _ := m["addr"].(string); addr != "" {
								liveIPs[addr] = true
							}
						}
					}
				}
			}
		}
	} else {
		return nil, fmt.Errorf("KT server listing failed (needed to protect live VMs): %w", lerr)
	}
	var chain []csp.ResidualResource // port forwarding + static NAT (+ their public IPs), deleted after firewall rules
	ipTier := map[string]string{}    // private/public IPs of attributed entries -> tier name (for firewall matching)
	publicIPs := map[string]string{} // public IP id -> tier name
	publicIPAddr := map[string]string{}
	inUsePublicIPs := map[string]bool{} // public IP ids referenced by any PF/NAT (live or not)
	pfPager := ktpf.List(client, ktpf.ListOpts{Page: 1, Size: 2000})
	err = pfPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktpf.ExtractPFs(page)
		if err != nil {
			return false, err
		}
		for _, pf := range list {
			inUsePublicIPs[pf.PublicIPID] = true
			if liveIPs[pf.MappedIP] {
				continue
			}
			if ref := tierOfIP(pf.MappedIP); ref != "" {
				chain = append(chain, csp.ResidualResource{Type: "portForwarding", Id: pf.ID, Name: ref, Zone: zoneArg(region, zone),
					Detail: pf.PublicIP + ":" + pf.StartPublicPort + " -> " + pf.MappedIP + ":" + pf.StartPrivatePort + " " + pf.Protocol})
				ipTier[pf.MappedIP], ipTier[pf.PublicIP] = ref, ref
				if pf.PublicIPID != "" {
					publicIPs[pf.PublicIPID], publicIPAddr[pf.PublicIPID] = ref, pf.PublicIP
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("KT port-forwarding listing failed: %w", err)
	}
	natPager := ktnat.List(client, ktnat.ListOpts{Page: 1, Size: 2000})
	err = natPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktnat.ExtractStaticNats(page)
		if err != nil {
			return false, err
		}
		for _, n := range list {
			inUsePublicIPs[n.PublicIpID] = true
			if liveIPs[n.MappedIP] {
				continue
			}
			if ref := tierOfIP(n.MappedIP); ref != "" {
				chain = append(chain, csp.ResidualResource{Type: "staticNat", Id: n.StaticNatID, Name: ref, Zone: zoneArg(region, zone),
					Detail: n.PublicIP + " -> " + n.MappedIP})
				ipTier[n.MappedIP], ipTier[n.PublicIP] = ref, ref
				if n.PublicIpID != "" {
					publicIPs[n.PublicIpID], publicIPAddr[n.PublicIpID] = ref, n.PublicIP
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("KT static NAT listing failed: %w", err)
	}
	for id, ref := range publicIPs {
		chain = append(chain, csp.ResidualResource{Type: "publicIp", Id: id, Name: ref, Zone: zoneArg(region, zone), Detail: publicIPAddr[id]})
	}
	// Public IPs no PF/NAT references any more (e.g. after a partial cleanup) cannot be attributed; list them unnamed.
	pipPager := ktpip.List(client, ktpip.ListOpts{Page: 1, Size: 2000})
	err = pipPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktpip.ExtractFloatingIPs(page)
		if err != nil {
			return false, err
		}
		for _, ip := range list {
			if ip.PublicIpID == "" || inUsePublicIPs[ip.PublicIpID] {
				continue
			}
			if _, known := publicIPs[ip.PublicIpID]; known {
				continue
			}
			chain = append(chain, csp.ResidualResource{Type: "publicIp", Id: ip.PublicIpID, Zone: zoneArg(region, zone), Detail: ip.PublicIP + " (unreferenced)"})
		}
		return true, nil
	})
	if err != nil {
		log.Warn().Err(err).Msg("[KT] public IP listing failed; unreferenced public IPs not reported")
	}
	// addrTier attributes a firewall address (IP or IP/32) to a TB tier by private range or by NAT/PF IP.
	addrTier := func(a string) string {
		for _, k := range addrKeys(a) {
			if ref, ok := ipTier[k]; ok {
				return ref
			}
		}
		return tierOfIP(strings.TrimSpace(a))
	}
	var fw []csp.ResidualResource
	fwPager := ktrules.List(client, ktrules.ListOpts{Page: 1, Size: 2000})
	err = fwPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktrules.ExtractRules(page)
		if err != nil {
			return false, err
		}
		for _, r := range list {
			ref := ""
			for _, ifc := range append(r.SrcInterface, r.DstInterface...) {
				if n, ok := tierNames[ifc.NetworkID]; ok {
					ref = n
					break
				}
			}
			live := false
			for _, a := range append(r.SrcAddress, r.DstAddress...) {
				for _, k := range addrKeys(a.Name) {
					if liveIPs[k] {
						live = true
					}
				}
			}
			if live {
				continue
			}
			if ref == "" {
				for _, a := range append(r.SrcAddress, r.DstAddress...) {
					if n := addrTier(a.Name); n != "" {
						ref = n
						break
					}
				}
			}
			if ref == "" {
				continue
			}
			var addrs []string
			for _, a := range r.SrcAddress {
				addrs = append(addrs, a.Name)
			}
			addrs = append(addrs, "->")
			for _, a := range r.DstAddress {
				addrs = append(addrs, a.Name)
			}
			fw = append(fw, csp.ResidualResource{Type: "firewallRule", Id: r.PolicyID, Name: ref, Zone: zoneArg(region, zone),
				Detail: strings.Join(addrs, " ")})
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("KT firewall rule listing failed: %w", err)
	}
	return append(append(fw, chain...), out...), nil
}

// DeleteResiduals deletes the given custom tiers. KT returns an odd error body containing ":true"
// on success (same quirk cb-spider tolerates).
func DeleteResiduals(ctx context.Context, region, zone string, items []csp.ResidualResource) map[string]error {
	result := make(map[string]error, len(items))
	client, err := newNetworkClient(ctx, zoneArg(region, zone))
	if err != nil {
		for _, it := range items {
			result[it.Key()] = err
		}
		return result
	}
	// KT dependency order: firewall policies -> port forwarding -> static NAT -> tiers.
	phases := []string{"firewallRule", "portForwarding", "staticNat", "publicIp", "tier"}
	for _, it := range items {
		known := false
		for _, ph := range phases {
			if it.Type == ph {
				known = true
			}
		}
		if !known {
			result[it.Key()] = fmt.Errorf("unsupported residual type %q", it.Type)
		}
	}
	for _, phase := range phases {
		for _, it := range items {
			if it.Type != phase {
				continue
			}
			switch it.Type {
			case "firewallRule":
				result[it.Key()] = normalizeKTDeleteErr(ktrules.Delete(client, it.Id).ExtractErr())
			case "portForwarding":
				result[it.Key()] = normalizeKTDeleteErr(ktpf.Delete(client, it.Id).ExtractErr())
			case "staticNat":
				result[it.Key()] = normalizeKTDeleteErr(ktnat.Delete(client, it.Id).ExtractErr())
			case "publicIp":
				result[it.Key()] = normalizeKTDeleteErr(ktpip.Delete(client, it.Id).ExtractErr())
			case "tier":
				result[it.Key()] = normalizeKTDeleteErr(subnets.Delete(client, it.Id).ExtractErr())
			}
		}
	}
	return result
}

// normalizeKTDeleteErr maps KT's quirky "HTTP 500 with {...:true}" success body to nil and
// otherwise surfaces the response body (the SDK's default error text is just "Internal Server Error").
func normalizeKTDeleteErr(err error) error {
	if err == nil {
		return nil
	}
	var e500 ktsdk.ErrDefault500
	if errors.As(err, &e500) {
		body := string(e500.Body)
		if strings.Contains(body, ":true") {
			return nil
		}
		return fmt.Errorf("KT API 500: %s", strings.TrimSpace(body))
	}
	var eCode ktsdk.ErrUnexpectedResponseCode
	if errors.As(err, &eCode) && strings.Contains(string(eCode.Body), ":true") {
		return nil
	}
	return err
}

// cleanupServerNetworking deletes firewall policies, port-forwarding entries, and public IPs that
// belong to the given servers' addresses (KT dependency order), before the servers are deleted.
func cleanupServerNetworking(ctx context.Context, zone string, serverIds []string) error {
	servers, err := listServers(ctx, zone)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, id := range serverIds {
		want[id] = true
	}
	ips := map[string]bool{}
	for _, sv := range servers {
		if !want[sv.ID] {
			continue
		}
		for _, v := range sv.Addresses {
			if list, ok := v.([]interface{}); ok {
				for _, a := range list {
					if m, ok := a.(map[string]interface{}); ok {
						if addr, _ := m["addr"].(string); addr != "" {
							ips[addr] = true
						}
					}
				}
			}
		}
	}
	if len(ips) == 0 {
		return nil
	}
	net, err := newNetworkClient(ctx, zone)
	if err != nil {
		return err
	}
	// port forwarding + public IPs of those servers
	var pfIDs []string
	pubIPs := map[string]string{}
	pfPager := ktpf.List(net, ktpf.ListOpts{Page: 1, Size: 2000})
	_ = pfPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktpf.ExtractPFs(page)
		if err != nil {
			return false, err
		}
		for _, pf := range list {
			if ips[pf.MappedIP] {
				pfIDs = append(pfIDs, pf.ID)
				ips[pf.PublicIP] = true
				if pf.PublicIPID != "" {
					pubIPs[pf.PublicIPID] = pf.PublicIP
				}
			}
		}
		return true, nil
	})
	natPager := ktnat.List(net, ktnat.ListOpts{Page: 1, Size: 2000})
	var natIDs []string
	_ = natPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktnat.ExtractStaticNats(page)
		if err != nil {
			return false, err
		}
		for _, n := range list {
			if ips[n.MappedIP] {
				natIDs = append(natIDs, n.StaticNatID)
				ips[n.PublicIP] = true
				if n.PublicIpID != "" {
					pubIPs[n.PublicIpID] = n.PublicIP
				}
			}
		}
		return true, nil
	})
	// firewall policies referencing any of those IPs
	var fwIDs []string
	fwPager := ktrules.List(net, ktrules.ListOpts{Page: 1, Size: 2000})
	_ = fwPager.EachPage(func(page pagination.Page) (bool, error) {
		list, err := ktrules.ExtractRules(page)
		if err != nil {
			return false, err
		}
		for _, r := range list {
			matched := false
			for _, a := range append(r.SrcAddress, r.DstAddress...) {
				for _, k := range addrKeys(a.Name) {
					if ips[k] {
						matched = true
					}
				}
			}
			if matched {
				fwIDs = append(fwIDs, r.PolicyID)
			}
		}
		return true, nil
	})
	var firstErr error
	note := func(e error) {
		if e != nil && firstErr == nil {
			firstErr = e
		}
	}
	for _, id := range fwIDs {
		note(normalizeKTDeleteErr(ktrules.Delete(net, id).ExtractErr()))
	}
	for _, id := range pfIDs {
		note(normalizeKTDeleteErr(ktpf.Delete(net, id).ExtractErr()))
	}
	for _, id := range natIDs {
		note(normalizeKTDeleteErr(ktnat.Delete(net, id).ExtractErr()))
	}
	for id := range pubIPs {
		note(normalizeKTDeleteErr(ktpip.Delete(net, id).ExtractErr()))
	}
	log.Info().Msgf("[KT] pre-terminate cleanup: firewall=%d portForwarding=%d staticNat=%d publicIp=%d (servers=%d)", len(fwIDs), len(pfIDs), len(natIDs), len(pubIPs), len(serverIds))
	return firstErr
}
