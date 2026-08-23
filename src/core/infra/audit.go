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

package infra

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	cspdirect "github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	csptypes "github.com/cloud-barista/cb-tumblebug/src/core/model/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"

	// Register direct-SDK truth-surface handlers (inventory / terminate / residuals).
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/alibaba"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/aws"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/azure"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/gcp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/ibm"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/kt"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/ncp"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/nhn"
	_ "github.com/cloud-barista/cb-tumblebug/src/core/csp/tencent"
)

// AuditOptions controls what an audit does beyond reporting.
type AuditOptions struct {
	// Remediate terminates VMs that are alive at the CSP but unknown to TB or recorded as Terminated/Failed.
	Remediate bool `json:"remediate"`
	// CleanResiduals: "" | "none" (report only), "attributed" (delete residuals named after this infra's nodes), "all" (also unnamed ones).
	CleanResiduals string `json:"cleanResiduals"`
}

// AuditVM is one VM as seen by the CSP and/or TB.
type AuditVM struct {
	NodeId        string            `json:"nodeId,omitempty"`
	CspResourceId string            `json:"cspResourceId"`
	Name          string            `json:"name,omitempty"`
	CspStatus     string            `json:"cspStatus,omitempty"`
	TbStatus      string            `json:"tbStatus,omitempty"`
	Action        string            `json:"action,omitempty"`
	Error         string            `json:"error,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"` // CSP tags (untracked VMs only; reveals origin)
}

// AuditResidual is a VM sub-resource left at the CSP.
type AuditResidual struct {
	cspdirect.ResidualResource
	Attributed bool   `json:"attributed"` // named after a node of the audited scope
	Action     string `json:"action,omitempty"`
	Error      string `json:"error,omitempty"`
}

// AuditConnectionResult is the CSP-vs-TB comparison for one connection.
type AuditConnectionResult struct {
	ConnectionName     string          `json:"connectionName"`
	Provider           string          `json:"provider"`
	Region             string          `json:"region"`
	Zone               string          `json:"zone,omitempty"`
	Supported          bool            `json:"supported"`
	Error              string          `json:"error,omitempty"`
	CspVMTotal         int             `json:"cspVmTotal"`
	TrackedAlive       []AuditVM       `json:"trackedAlive,omitempty"`   // TB node exists and CSP VM is alive (expected)
	GhostAlive         []AuditVM       `json:"ghostAlive,omitempty"`     // TB says Terminated/Failed but CSP VM is alive
	TrackedGone        []AuditVM       `json:"trackedGone,omitempty"`    // TB node has a CSP id but the CSP has no such VM
	UntrackedAlive     []AuditVM       `json:"untrackedAlive,omitempty"` // CSP VM belongs to the scope but TB has no record
	OtherTBManaged     int             `json:"otherTbManaged"`           // TB-managed VMs outside the audited scope (ignored)
	TrackedNetworks    int             `json:"trackedNetworks"`          // network residual candidates skipped because TB still tracks them
	RegisteredExternal int             `json:"registeredExternal"`       // VMs TB only imported (sys.registered=true); never remediated
	Residuals          []AuditResidual `json:"residuals,omitempty"`
}

// AuditSummary aggregates counts across connections.
type AuditSummary struct {
	Connections        int `json:"connections"`
	Unsupported        int `json:"unsupported"`
	Errors             int `json:"errors"`
	TrackedAlive       int `json:"trackedAlive"`
	GhostAlive         int `json:"ghostAlive"`
	TrackedGone        int `json:"trackedGone"`
	UntrackedAlive     int `json:"untrackedAlive"`
	ResidualsFound     int `json:"residualsFound"`
	TerminateRequested int `json:"terminateRequested"`
	ResidualsDeleted   int `json:"residualsDeleted"`
}

// AuditResult is the full report of an infra- or connection-scoped audit.
type AuditResult struct {
	NsId        string                  `json:"nsId,omitempty"`
	InfraId     string                  `json:"infraId,omitempty"`
	Scope       string                  `json:"scope"` // infra | connection
	Options     AuditOptions            `json:"options"`
	Clean       bool                    `json:"clean"` // no ghost/untracked VMs, no attributed residuals, no errors
	Summary     AuditSummary            `json:"summary"`
	Connections []AuditConnectionResult `json:"connections"`
	ElapsedTime string                  `json:"elapsedTime"`
}

// auditScope is the TB-side knowledge the CSP inventory is compared against.
type auditScope struct {
	infraId string
	nodes   map[string][]model.NodeInfo // connectionName -> nodes
	// uids (node.Uid / cspResourceName) that identify VMs and name-prefixed residuals of this scope
	uids map[string]struct{}
	// whole-connection mode: every TB-managed VM not tracked is an orphan
	connectionWide bool
}

const auditConnConcurrency = 4

func loadNodesParallel(nsId, infraId string, nodeIds []string) []model.NodeInfo {
	out := make([]model.NodeInfo, 0, len(nodeIds))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)
	for _, id := range nodeIds {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			n, err := GetNodeObject(nsId, infraId, id)
			if err != nil {
				return
			}
			mu.Lock()
			out = append(out, n)
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return out
}

func newScope(infraId string, nodes []model.NodeInfo, connectionWide bool) *auditScope {
	s := &auditScope{infraId: infraId, nodes: map[string][]model.NodeInfo{}, uids: map[string]struct{}{}, connectionWide: connectionWide}
	for _, n := range nodes {
		s.nodes[n.ConnectionName] = append(s.nodes[n.ConnectionName], n)
		if n.Uid != "" {
			s.uids[strings.ToLower(n.Uid)] = struct{}{}
		}
		if n.CspResourceName != "" {
			s.uids[strings.ToLower(n.CspResourceName)] = struct{}{}
		}
	}
	return s
}

// isRegisteredExternal reports whether TB only registered (imported) this VM rather than creating it.
func isRegisteredExternal(tags map[string]string) bool {
	for k, v := range tags {
		if cspdirect.NormalizeTagKey(k) == "sys.registered" && strings.EqualFold(v, "true") {
			return true
		}
	}
	return false
}

// belongs reports whether a CSP record (not tracked by TB) belongs to the audited scope.
func (s *auditScope) belongs(rec cspdirect.VMRecord) bool {
	if isRegisteredExternal(rec.Tags) {
		return false // imported VMs are never TB's to terminate
	}
	if s.connectionWide {
		return cspdirect.IsManagedByTB(rec.Name, rec.Tags)
	}
	if _, ok := s.uids[strings.ToLower(rec.Name)]; ok {
		return true
	}
	for k, v := range rec.Tags {
		if cspdirect.NormalizeTagKey(k) == strings.ToLower(model.LabelInfraId) && strings.EqualFold(v, s.infraId) {
			return true
		}
	}
	return false
}

// attributed reports whether a residual is named after a node of the scope (or, connection-wide,
// after any TB uid or TB shared-resource naming such as "<ns>-shared-<connection>").
func (s *auditScope) attributed(r cspdirect.ResidualResource, connName string) bool {
	name := strings.ToLower(r.Name)
	if s.connectionWide {
		return cspdirect.IsTBUid(name) || strings.Contains(name, "-shared-"+strings.ToLower(connName))
	}
	if len(name) >= 20 {
		if _, ok := s.uids[name[:20]]; ok {
			return true
		}
	}
	_, ok := s.uids[name]
	return ok
}

func tbConsidersGone(n model.NodeInfo) bool {
	return strings.EqualFold(n.Status, model.StatusTerminated) || strings.EqualFold(n.Status, model.StatusFailed)
}

// auditConnection compares one connection's CSP inventory with the scope and optionally remediates.
func auditConnection(connName string, nodes []model.NodeInfo, scope *auditScope, opts AuditOptions) AuditConnectionResult {
	res := AuditConnectionResult{ConnectionName: connName}
	var conn model.ConnConfig
	if len(nodes) > 0 && nodes[0].ConnectionConfig.ConfigName != "" {
		conn = nodes[0].ConnectionConfig
	} else {
		c, err := common.GetConnConfig(connName)
		if err != nil {
			res.Error = fmt.Sprintf("connection config not found: %v", err)
			return res
		}
		conn = c
	}
	res.Provider = strings.ToLower(conn.ProviderName)
	// AssignedRegion carries the CSP-native code (e.g. NHN "KR1", NCP "KR"); RegionName is TB's lowercase form.
	res.Region = conn.RegionZoneInfo.AssignedRegion
	res.Zone = conn.RegionZoneInfo.AssignedZone
	if res.Region == "" {
		res.Region = conn.RegionDetail.RegionName
	}
	handlers, ok := cspdirect.GetInventoryHandlers(res.Provider)
	if !ok || handlers.ListVMs == nil {
		res.Error = "direct inventory not supported for provider " + res.Provider
		return res
	}
	res.Supported = true
	ctx := context.WithValue(context.Background(), model.CtxKeyCredentialHolder, conn.CredentialHolder)

	records, err := handlers.ListVMs(ctx, res.Region, res.Zone)
	if err != nil && isTransientNetworkError(err) {
		time.Sleep(3 * time.Second)
		records, err = handlers.ListVMs(ctx, res.Region, res.Zone)
	}
	if err != nil {
		res.Error = err.Error()
		return res
	}
	res.CspVMTotal = len(records)
	byId := make(map[string]cspdirect.VMRecord, len(records))
	byName := make(map[string]cspdirect.VMRecord, len(records))
	for _, r := range records {
		byId[strings.ToLower(r.CspResourceId)] = r
		if r.Name != "" {
			byName[strings.ToLower(r.Name)] = r
		}
	}
	matched := map[string]bool{}
	for _, n := range nodes {
		rec, found := byId[strings.ToLower(n.CspResourceId)]
		if !found && n.CspResourceName != "" {
			rec, found = byName[strings.ToLower(n.CspResourceName)]
		}
		if !found && n.Uid != "" {
			rec, found = byName[strings.ToLower(n.Uid)]
		}
		if !found {
			if n.CspResourceId != "" || n.CspResourceName != "" {
				res.TrackedGone = append(res.TrackedGone, AuditVM{NodeId: n.Id, CspResourceId: n.CspResourceId, Name: n.CspResourceName, TbStatus: n.Status})
			}
			continue
		}
		matched[strings.ToLower(rec.CspResourceId)] = true
		vm := AuditVM{NodeId: n.Id, CspResourceId: rec.CspResourceId, Name: rec.Name, CspStatus: rec.Status, TbStatus: n.Status}
		if tbConsidersGone(n) && !strings.EqualFold(rec.Status, model.StatusTerminated) && !strings.EqualFold(rec.Status, model.StatusTerminating) {
			res.GhostAlive = append(res.GhostAlive, vm)
		} else {
			res.TrackedAlive = append(res.TrackedAlive, vm)
		}
	}
	for _, r := range records {
		if matched[strings.ToLower(r.CspResourceId)] {
			continue
		}
		if strings.EqualFold(r.Status, model.StatusTerminated) || strings.EqualFold(r.Status, model.StatusTerminating) {
			continue
		}
		if isRegisteredExternal(r.Tags) {
			res.RegisteredExternal++
		} else if scope.belongs(r) {
			res.UntrackedAlive = append(res.UntrackedAlive, AuditVM{CspResourceId: r.CspResourceId, Name: r.Name, CspStatus: r.Status, Tags: r.Tags})
		} else if cspdirect.IsManagedByTB(r.Name, r.Tags) {
			res.OtherTBManaged++
		}
	}

	if handlers.ListResiduals != nil {
		tracked := trackedNetworkKeys(connName)
		items, rerr := handlers.ListResiduals(ctx, res.Region, res.Zone)
		if rerr != nil && isTransientNetworkError(rerr) {
			time.Sleep(3 * time.Second) // DNS/connection blips are common on this path; one retry
			items, rerr = handlers.ListResiduals(ctx, res.Region, res.Zone)
		}
		if rerr != nil {
			log.Warn().Err(rerr).Msgf("[Audit] residual listing failed for %s", connName)
			res.Error = "residual listing failed: " + rerr.Error()
		}
		for _, it := range items {
			if networkResidualType(it.Type) && (tracked[strings.ToLower(it.Id)] || tracked[strings.ToLower(it.Name)]) {
				res.TrackedNetworks++
				continue
			}
			res.Residuals = append(res.Residuals, AuditResidual{ResidualResource: it, Attributed: scope.attributed(it, connName)})
		}
	}

	if opts.Remediate && (len(res.GhostAlive) > 0 || len(res.UntrackedAlive) > 0) {
		terminateRegion := res.Region
		if res.Provider == csptypes.KT && res.Zone != "" {
			terminateRegion = res.Zone
		}
		term, ok := cspdirect.GetRemediationTerminateHandler(res.Provider)
		targets := append(res.GhostAlive, res.UntrackedAlive...)
		ids := make([]string, 0, len(targets))
		for _, t := range targets {
			ids = append(ids, t.CspResourceId)
		}
		var accepted map[string]string
		var terr error
		if !ok {
			terr = fmt.Errorf("direct terminate not supported for provider %s", res.Provider)
		} else {
			accepted, terr = term(ctx, terminateRegion, ids)
		}
		apply := func(list []AuditVM) {
			for i := range list {
				if terr != nil && accepted == nil {
					list[i].Action, list[i].Error = "error", terr.Error()
					continue
				}
				if _, okk := accepted[list[i].CspResourceId]; okk {
					list[i].Action = "terminate-requested"
				} else {
					list[i].Action = "error"
					list[i].Error = "terminate not accepted by CSP"
					if terr != nil {
						list[i].Error = terr.Error()
					}
				}
			}
		}
		apply(res.GhostAlive)
		apply(res.UntrackedAlive)
	}

	mode := strings.ToLower(opts.CleanResiduals)
	if (mode == "attributed" || mode == "all") && handlers.DeleteResiduals != nil && len(res.Residuals) > 0 {
		var toDelete []cspdirect.ResidualResource
		for _, r := range res.Residuals {
			if r.Attributed || (mode == "all" && eligibleForAllMode(r.ResidualResource)) {
				toDelete = append(toDelete, r.ResidualResource)
			}
		}
		if len(toDelete) > 0 {
			outcome := handlers.DeleteResiduals(ctx, res.Region, res.Zone, toDelete)
			for i := range res.Residuals {
				e, acted := outcome[res.Residuals[i].Key()]
				if !acted {
					continue
				}
				if e != nil {
					res.Residuals[i].Action, res.Residuals[i].Error = "error", e.Error()
				} else {
					res.Residuals[i].Action = "deleted"
				}
			}
		}
	}
	return res
}

func runAudit(scope *auditScope, connNames []string, opts AuditOptions, result *AuditResult) {
	start := time.Now()
	sort.Strings(connNames)
	results := make([]AuditConnectionResult, len(connNames))
	var wg sync.WaitGroup
	sem := make(chan struct{}, auditConnConcurrency)
	for i, cn := range connNames {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, cn string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = auditConnection(cn, scope.nodes[cn], scope, opts)
		}(i, cn)
	}
	wg.Wait()
	result.Connections = results
	s := &result.Summary
	s.Connections = len(results)
	clean := true
	for _, r := range results {
		if !r.Supported {
			s.Unsupported++
			clean = false
		}
		if r.Error != "" {
			s.Errors++
			clean = false
		}
		s.TrackedAlive += len(r.TrackedAlive)
		s.GhostAlive += len(r.GhostAlive)
		s.TrackedGone += len(r.TrackedGone)
		s.UntrackedAlive += len(r.UntrackedAlive)
		if len(r.GhostAlive) > 0 || len(r.UntrackedAlive) > 0 {
			clean = false
		}
		for _, v := range append(r.GhostAlive, r.UntrackedAlive...) {
			if v.Action == "terminate-requested" {
				s.TerminateRequested++
			}
		}
		for _, x := range r.Residuals {
			s.ResidualsFound++
			if x.Action == "deleted" {
				s.ResidualsDeleted++
			} else if x.Attributed {
				clean = false
			}
		}
	}
	result.Clean = clean
	result.ElapsedTime = time.Since(start).String()
}

// AuditInfra compares the CSP-side truth with TB's records for every connection used by an infra.
func AuditInfra(nsId, infraId string, opts AuditOptions) (*AuditResult, error) {
	if _, exists, _ := GetInfraObject(nsId, infraId); !exists {
		return nil, fmt.Errorf("infra %s/%s not found", nsId, infraId)
	}
	nodeIds, err := ListNodeId(nsId, infraId)
	if err != nil {
		return nil, err
	}
	nodes := loadNodesParallel(nsId, infraId, nodeIds)
	scope := newScope(infraId, nodes, false)
	connNames := make([]string, 0, len(scope.nodes))
	for cn := range scope.nodes {
		if cn != "" {
			connNames = append(connNames, cn)
		}
	}
	result := &AuditResult{NsId: nsId, InfraId: infraId, Scope: "infra", Options: opts}
	runAudit(scope, connNames, opts, result)
	log.Info().Msgf("[Audit] infra %s/%s: clean=%t summary=%+v", nsId, infraId, result.Clean, result.Summary)
	return result, nil
}

// AuditConnection compares every TB-managed VM at a connection's CSP region with all TB records (all namespaces).
func AuditConnection(connectionName string, opts AuditOptions) (*AuditResult, error) {
	if _, err := common.GetConnConfig(connectionName); err != nil {
		return nil, err
	}
	var nodes []model.NodeInfo
	nsIds, err := common.ListNsId()
	if err != nil {
		return nil, err
	}
	for _, ns := range nsIds {
		infraIds, err := ListInfraId(ns)
		if err != nil {
			continue
		}
		for _, inf := range infraIds {
			ids, err := ListNodeId(ns, inf)
			if err != nil {
				continue
			}
			for _, n := range loadNodesParallel(ns, inf, ids) {
				if strings.EqualFold(n.ConnectionName, connectionName) {
					nodes = append(nodes, n)
				}
			}
		}
	}
	scope := newScope("", nodes, true)
	if _, ok := scope.nodes[connectionName]; !ok {
		scope.nodes[connectionName] = nil
	}
	result := &AuditResult{Scope: "connection", Options: opts}
	runAudit(scope, []string{connectionName}, opts, result)
	log.Info().Msgf("[Audit] connection %s: clean=%t summary=%+v", connectionName, result.Clean, result.Summary)
	return result, nil
}

// deleteGuardWait bounds how long DelInfra(option=terminate) waits for remediated VMs to disappear.
const deleteGuardWait = 3 * time.Minute

// guardOrphansBeforeDelete refuses to drop TB records while the CSP still runs VMs of the infra.
// option=terminate remediates (direct terminate + attributed residual cleanup) and waits; option=force skips the guard.
func guardOrphansBeforeDelete(nsId, infraId, option string) error {
	if option == "force" {
		return nil
	}
	remediate := strings.EqualFold(option, model.ActionTerminate)
	opts := AuditOptions{Remediate: remediate}
	if remediate {
		opts.CleanResiduals = "attributed"
	}
	res, err := AuditInfra(nsId, infraId, opts)
	if err != nil {
		log.Warn().Err(err).Msgf("[DelInfra] audit failed for %s/%s; proceeding without CSP verification", nsId, infraId)
		return nil
	}
	alive := res.Summary.GhostAlive + res.Summary.UntrackedAlive
	if alive == 0 {
		return nil
	}
	if !remediate {
		return fmt.Errorf("infra %s still has %d VM(s) alive at the CSP (ghost=%d, untracked=%d); use option=terminate to terminate them directly, or option=force to drop records anyway",
			infraId, alive, res.Summary.GhostAlive, res.Summary.UntrackedAlive)
	}
	deadline := time.Now().Add(deleteGuardWait)
	for time.Now().Before(deadline) {
		time.Sleep(15 * time.Second)
		chk, err := AuditInfra(nsId, infraId, AuditOptions{})
		if err != nil {
			continue
		}
		if chk.Summary.GhostAlive+chk.Summary.UntrackedAlive == 0 {
			log.Info().Msgf("[DelInfra] %s/%s: remediated %d VM(s) at the CSP before deletion", nsId, infraId, alive)
			if _, cerr := AuditInfra(nsId, infraId, AuditOptions{CleanResiduals: "attributed"}); cerr != nil {
				log.Warn().Err(cerr).Msg("[DelInfra] residual cleanup after remediation failed")
			}
			return nil
		}
	}
	return fmt.Errorf("infra %s: %d VM(s) still alive at the CSP after direct terminate; retry deletion later or use option=force", infraId, alive)
}

// eligibleForAllMode limits cleanResiduals=all to unnamed VM-adjacent leftovers (IPs, NICs, disks).
// Network objects and firewall rules that are not TB-named are never deleted unattributed.
func eligibleForAllMode(r cspdirect.ResidualResource) bool {
	if networkResidualType(r.Type) || r.Type == "firewallRule" {
		return cspdirect.IsTBUid(r.Name)
	}
	return true
}

// networkResidualType reports residual types that are network objects TB itself may still track.
func networkResidualType(t string) bool {
	switch t {
	case "tier", "subnet", "vnet":
		return true
	}
	return false
}

// trackedNetworkKeys returns lower-cased CSP ids/names of vNets and subnets TB still tracks on a connection (all namespaces).
func trackedNetworkKeys(connName string) map[string]bool {
	keys := map[string]bool{}
	add := func(v ...string) {
		for _, x := range v {
			if x != "" {
				keys[strings.ToLower(x)] = true
			}
		}
	}
	nsIds, err := common.ListNsId()
	if err != nil {
		return keys
	}
	for _, ns := range nsIds {
		ids, err := resource.ListResourceId(ns, model.StrVNet)
		if err != nil {
			continue
		}
		for _, id := range ids {
			v, err := resource.GetVNet(ns, id)
			if err != nil || !strings.EqualFold(v.ConnectionName, connName) {
				continue
			}
			add(v.CspResourceId, v.CspResourceName, v.Uid)
			for _, sn := range v.SubnetInfoList {
				add(sn.CspResourceId, sn.CspResourceName, sn.Uid)
			}
		}
	}
	return keys
}
