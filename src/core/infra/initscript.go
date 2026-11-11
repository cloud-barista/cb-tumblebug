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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/embedded"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/cloud-barista/cb-tumblebug/src/core/secret"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

const (
	InitScriptModeDefault = "default"
	InitScriptModeCustom  = "custom"
	InitScriptModeNone    = "none"
	initScriptDefaultUser = "cb-user"
)

// NotifierProvider returns the registered notifier so init scripts can report back; set by the notifier package.
var NotifierProvider func() *model.NotifierInfo

// initScriptTemplateLookup resolves a template id (request namespace, then system); tests replace it.
var initScriptTemplateLookup = common.FindInitScriptTemplate

// defaultInitScript returns the request as given, or the implicit default (mode default, cb-user, no steps)
// when the request had none. Mode none is the way to keep the CB-Spider default user data.
func defaultInitScript(req *model.InitScriptReq) *model.InitScriptReq {
	if req != nil {
		return req
	}
	return &model.InitScriptReq{Mode: InitScriptModeDefault, Implicit: true}
}

// initScriptEnabled reports whether a Node should receive a CB-Tumblebug init script instead of the Spider default.
func initScriptEnabled(req *model.InitScriptReq) bool {
	return req != nil && !strings.EqualFold(req.Mode, InitScriptModeNone)
}

// initScriptUserName returns the SSH account the init script creates or expects.
func initScriptUserName(req *model.InitScriptReq) string {
	if req != nil && strings.TrimSpace(req.UserName) != "" {
		return strings.TrimSpace(req.UserName)
	}
	return initScriptDefaultUser
}

// validateInitScriptReq checks mode, user name, env and step references before provisioning starts.
func validateInitScriptReq(req *model.InitScriptReq) error {
	if req == nil {
		return nil
	}
	switch strings.ToLower(req.Mode) {
	case "", InitScriptModeDefault, InitScriptModeCustom, InitScriptModeNone:
	default:
		return fmt.Errorf("initScript.mode must be one of default, custom, none (got %q)", req.Mode)
	}
	if u := initScriptUserName(req); !model.IsSafeShellValue(u) || strings.ContainsAny(u, " /:") {
		return fmt.Errorf("initScript.userName %q is not a valid account name", u)
	}
	if err := model.ValidateInitScriptEnv(req.Env); err != nil {
		return err
	}
	for i, s := range req.Scripts {
		if strings.TrimSpace(s.Script) == "" {
			return fmt.Errorf("initScript.scripts[%d] has an empty script", i)
		}
		if s.Name != "" && !model.IsInitScriptName(s.Name) {
			return fmt.Errorf("initScript.scripts[%d].name %q may only contain letters, digits, '.', '_' and '-'", i, s.Name)
		}
	}
	return nil
}

// resolveInitScript returns a copy with every step inline, so Nodes added later get the same content even
// if the template changes. Steps run in order: the template's scripts, then script, then scripts. Template env
// is applied first and the request env overrides it. Resolving an already resolved request is a no-op.
func resolveInitScript(nsId string, req *model.InitScriptReq) (*model.InitScriptReq, error) {
	if req == nil {
		return nil, nil
	}
	if err := validateInitScriptReq(req); err != nil {
		return nil, err
	}
	out := *req
	if !initScriptEnabled(req) {
		return &out, nil
	}
	steps := make([]model.InitScriptStep, 0, len(req.Scripts)+2)
	env := map[string]string{}
	if req.TemplateId != "" {
		t, err := initScriptTemplateLookup(nsId, req.TemplateId)
		if err != nil {
			return nil, fmt.Errorf("initScript: %w", err)
		}
		for i, st := range t.Scripts {
			if st.Name == "" {
				st.Name = t.Id
				if len(t.Scripts) > 1 {
					st.Name = t.Id + "-" + strconv.Itoa(i+1)
				}
			}
			steps = append(steps, st)
		}
		for k, v := range t.Env {
			env[k] = v
		}
		out.FromTemplateId = t.Id
	}
	if req.Script != "" {
		steps = append(steps, model.InitScriptStep{Name: "inline", Script: req.Script})
	}
	steps = append(steps, req.Scripts...)
	for i := range steps {
		if steps[i].Name == "" {
			steps[i].Name = "step-" + strconv.Itoa(i+1)
		}
	}
	for k, v := range req.Env {
		env[k] = v
	}
	if err := model.ValidateInitScriptEnv(env); err != nil {
		return nil, err
	}
	if len(env) == 0 {
		env = nil
	}
	out.TemplateId, out.Script, out.Scripts, out.Env = "", "", steps, env
	return &out, nil
}

// buildInitScript renders the user data for a Node: exported TB_* variables followed by the embedded wrapper.
// It returns "" when the Node uses the Spider default (no initScript, mode none, or Windows).
func buildInitScript(nsId, infraId string, node *model.NodeInfo, windows bool) (string, error) {
	if !initScriptEnabled(node.InitScript) || windows {
		return "", nil
	}
	req, err := resolveInitScript(nsId, node.InitScript)
	if err != nil {
		return "", err
	}
	env := map[string]string{
		"TB_NS_ID": nsId, "TB_INFRA_ID": infraId, "TB_NODEGROUP_ID": node.NodeGroupId, "TB_NODE_ID": node.Id,
		"TB_CSP": node.ConnectionConfig.ProviderName, "TB_REGION": node.ConnectionConfig.RegionDetail.RegionName,
	}
	if !strings.EqualFold(req.Mode, InitScriptModeCustom) {
		publicKey, err := sshPublicKey(nsId, node.SshKeyId)
		if err != nil {
			return "", err
		}
		env["TB_INIT_PUBKEY"] = publicKey
	}
	script := renderInitScript(env, req)
	if len(script) > model.InitScriptMaxBytes {
		return "", fmt.Errorf("init script for node %s is %d bytes; the limit is %d bytes (move large content to a download step)",
			node.Id, len(script), model.InitScriptMaxBytes)
	}
	return script, nil
}

// renderInitScript joins the identity env, the access and step variables, the request env and the wrapper.
func renderInitScript(env map[string]string, req *model.InitScriptReq) string {
	env["TB_INIT_USER"] = initScriptUserName(req)
	env["TB_INIT_ACCESS"] = "0"
	if !strings.EqualFold(req.Mode, InitScriptModeCustom) {
		env["TB_INIT_ACCESS"] = "1"
	}
	env["TB_INIT_SCRIPT_COUNT"] = strconv.Itoa(len(req.Scripts))
	for i, s := range req.Scripts {
		prefix := "TB_INIT_SCRIPT_" + strconv.Itoa(i+1) + "_"
		env[prefix+"NAME"], env[prefix+"B64"] = s.Name, base64.StdEncoding.EncodeToString([]byte(s.Script))
	}
	if n := activeNotifier(); n != nil && initScriptNotify(req) {
		env["TB_NOTIFY_URL"], env["TB_NOTIFY_TOPIC"], env["TB_NOTIFY_TOKEN"] = n.Url, n.Topic, n.WriteToken
		if n.CaCert != "" {
			env["TB_NOTIFY_CA_B64"] = base64.StdEncoding.EncodeToString([]byte(n.CaCert))
		}
	}
	for k, v := range req.Env {
		env[k] = v
	}
	// cloud-init detects a shell script by the first line, so the shebang must come before the variables.
	return "#!/bin/bash\n" + exportLines(env) + strings.TrimPrefix(embedded.InitScript(), "#!/bin/bash\n")
}

// reviewInitScript dry-runs the init script of a NodeGroup: it resolves templates and checks the rendered
// size with placeholder identity values, and returns review lines for the Node review.
func reviewInitScript(nsId, infraName, nodeGroupName string, req *model.InitScriptReq) (info, warnings, errs []string) {
	req = defaultInitScript(req)
	if !initScriptEnabled(req) {
		return nil, nil, nil
	}
	resolved, err := resolveInitScript(nsId, req)
	if err != nil {
		return nil, nil, []string{"initScript: " + err.Error()}
	}
	env := map[string]string{
		"TB_NS_ID": nsId, "TB_INFRA_ID": infraName, "TB_NODEGROUP_ID": nodeGroupName, "TB_NODE_ID": nodeGroupName + "-10000",
		"TB_CSP": "placeholder", "TB_REGION": "placeholder-region",
	}
	if !strings.EqualFold(resolved.Mode, InitScriptModeCustom) {
		env["TB_INIT_PUBKEY"] = strings.Repeat("k", 800) // longer than an RSA-4096 authorized_keys line
	}
	size := len(renderInitScript(env, resolved))
	names := make([]string, 0, len(resolved.Scripts))
	for _, s := range resolved.Scripts {
		names = append(names, s.Name)
	}
	info = append(info, fmt.Sprintf("initScript: mode %s, user %s, %d step(s) [%s], about %d of %d bytes of user data",
		strings.ToLower(firstNonEmpty(resolved.Mode, InitScriptModeDefault)), initScriptUserName(resolved), len(names), strings.Join(names, ", "), size, model.InitScriptMaxBytes))
	if size > model.InitScriptMaxBytes {
		errs = append(errs, fmt.Sprintf("initScript: rendered user data is about %d bytes; the limit is %d bytes (move large content to a download step)", size, model.InitScriptMaxBytes))
	}
	if activeNotifier() == nil && initScriptNotify(resolved) {
		warnings = append(warnings, "initScript: no active notifier, so Nodes cannot report their init script result (initScriptStatus stays pending)")
	}
	return info, warnings, errs
}

// applyInitScriptReview merges reviewInitScript lines into a Node review and reports whether it stays viable.
func applyInitScriptReview(nsId, infraName string, ng *model.CreateNodeGroupDynamicReq, infraInitScript *model.InitScriptReq, review *model.ReviewNodeGroupDynamicReqInfo) (viable, warned bool) {
	req := ng.InitScript
	if req == nil {
		req = infraInitScript
	}
	info, warnings, errs := reviewInitScript(nsId, infraName, ng.Name, req)
	review.Info = append(review.Info, info...)
	review.Warnings = append(review.Warnings, warnings...)
	review.Errors = append(review.Errors, errs...)
	if len(errs) > 0 {
		review.Status, review.CanCreate = "Error", false
		review.Message = errs[0]
		return false, len(warnings) > 0
	}
	if len(warnings) > 0 && review.Status == "Ready" {
		review.Status = "Warning"
	}
	return review.CanCreate, len(warnings) > 0
}

func initScriptNotify(req *model.InitScriptReq) bool { return req.Notify == nil || *req.Notify }

// initResultIndex mirrors the stored results of one Infra in memory. It is filled from etcd on first use and
// kept current by RecordInitScriptResult/deleteInitScriptResult, so list views that are polled for thousands of
// Nodes never read etcd for init script results.
type initResultIndex struct {
	mu sync.RWMutex
	m  map[string]model.InitScriptResult
}

var initResults sync.Map // "ns/infra" -> *initResultIndex

func initResultKey(nsId, infraId string) string { return nsId + "/" + infraId }

// loadInitResultIndex returns the index of an Infra, reading etcd once when it is not in memory yet.
func loadInitResultIndex(nsId, infraId string) (*initResultIndex, error) {
	if v, ok := initResults.Load(initResultKey(nsId, infraId)); ok {
		return v.(*initResultIndex), nil
	}
	kvs, err := kvstore.GetKvList(common.GenInfraNodeInitScriptKey(nsId, infraId, ""))
	if err != nil {
		return nil, err
	}
	idx := &initResultIndex{m: make(map[string]model.InitScriptResult, len(kvs))}
	for _, kv := range kvs {
		var r model.InitScriptResult
		if json.Unmarshal([]byte(kv.Value), &r) == nil {
			idx.m[kv.Key[strings.LastIndex(kv.Key, "/")+1:]] = r
		}
	}
	actual, _ := initResults.LoadOrStore(initResultKey(nsId, infraId), idx)
	return actual.(*initResultIndex), nil
}

// RecordInitScriptResult stores what a Node reported. It is one write to the Node's own result key, with no
// read-modify-write of the Node record, so a burst of reports from thousands of Nodes stays cheap.
func RecordInitScriptResult(nsId, infraId, nodeId string, r model.InitScriptResult) error {
	if err := common.CheckString(nsId); err != nil {
		return err
	}
	if err := common.CheckString(infraId); err != nil {
		return err
	}
	if err := common.CheckString(nodeId); err != nil {
		return err
	}
	if _, exists, err := kvstore.GetKv(common.GenInfraKey(nsId, infraId, nodeId)); err != nil || !exists {
		return fmt.Errorf("node %s/%s/%s not found; report ignored", nsId, infraId, nodeId)
	}
	val, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := kvstore.Put(common.GenInfraNodeInitScriptKey(nsId, infraId, nodeId), string(val)); err != nil {
		return err
	}
	if v, ok := initResults.Load(initResultKey(nsId, infraId)); ok {
		idx := v.(*initResultIndex)
		idx.mu.Lock()
		idx.m[nodeId] = r
		idx.mu.Unlock()
	}
	log.Info().Str("nsId", nsId).Str("infraId", infraId).Str("nodeId", nodeId).Str("status", r.Status).Msg("init script result recorded")
	return nil
}

// listInitScriptResults returns a snapshot of every Node's result of an Infra from the in-memory index.
func listInitScriptResults(nsId, infraId string) (map[string]model.InitScriptResult, error) {
	idx, err := loadInitResultIndex(nsId, infraId)
	if err != nil {
		return nil, err
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make(map[string]model.InitScriptResult, len(idx.m))
	for k, v := range idx.m {
		out[k] = v
	}
	return out, nil
}

// stepNamesOf returns the step names of a Node's blueprint for the compact list view.
func stepNamesOf(req *model.InitScriptReq) []string {
	if req == nil || len(req.Scripts) == 0 {
		return nil
	}
	names := make([]string, 0, len(req.Scripts))
	for _, st := range req.Scripts {
		names = append(names, st.Name)
	}
	return names
}

// attachInitScriptSummaries fills the init script fields of list-view Nodes. Results come from the memory
// index and the blueprint from the NodeGroup cache, so polling the list costs no extra etcd reads.
func attachInitScriptSummaries(nsId, infraId string, nodes []model.NodeSummary) {
	if len(nodes) == 0 {
		return
	}
	results, err := listInitScriptResults(nsId, infraId)
	if err != nil {
		return
	}
	ngInit := map[string]*model.InitScriptReq{}
	initOf := func(ngId string) *model.InitScriptReq {
		if req, ok := ngInit[ngId]; ok {
			return req
		}
		var req *model.InitScriptReq
		if ng, err := GetNodeGroupCached(nsId, infraId, ngId); err == nil {
			req = ng.InitScript
		}
		ngInit[ngId] = req
		return req
	}
	for i := range nodes {
		req := initOf(nodes[i].NodeGroupId)
		if r, ok := results[nodes[i].Id]; ok {
			nodes[i].InitScriptStatus, nodes[i].InitScriptMessage = r.Status, r.Summary()
			if r.Status != "ok" {
				nodes[i].InitScriptSteps = stepNamesOf(req)
			}
		} else if initScriptExpected(&model.NodeInfo{InitScript: req}) {
			nodes[i].InitScriptStatus = model.InitScriptStatusPending
		}
	}
}

// initScriptExpected reports whether a Node is supposed to report an init script result: the hydrated
// blueprint has an init script, reporting is on and a notifier is registered to receive it. Such Nodes are
// "pending" until their result arrives; without a notifier no report can ever come, so they are not.
func initScriptExpected(node *model.NodeInfo) bool {
	return initScriptEnabled(node.InitScript) && initScriptNotify(node.InitScript) && activeNotifier() != nil
}

// attachInitScriptResults fills initScriptStatus/initScriptMessage of listed Nodes from the stored results.
func attachInitScriptResults(nsId, infraId string, nodes []model.NodeInfo) {
	results, err := listInitScriptResults(nsId, infraId)
	if err != nil {
		return
	}
	for i := range nodes {
		if r, ok := results[nodes[i].Id]; ok {
			nodes[i].InitScriptStatus, nodes[i].InitScriptMessage = r.Status, r.Summary()
		} else if initScriptExpected(&nodes[i]) {
			nodes[i].InitScriptStatus = model.InitScriptStatusPending
		}
	}
}

// attachInitScriptResult fills the fields of a single Node read.
func attachInitScriptResult(nsId, infraId string, node *model.NodeInfo) {
	kv, exists, err := kvstore.GetKv(common.GenInfraNodeInitScriptKey(nsId, infraId, node.Id))
	if err != nil {
		return
	}
	if !exists {
		if initScriptExpected(node) {
			node.InitScriptStatus = model.InitScriptStatusPending
		}
		return
	}
	var r model.InitScriptResult
	if json.Unmarshal([]byte(kv.Value), &r) == nil {
		node.InitScriptStatus, node.InitScriptMessage = r.Status, r.Summary()
	}
}

func deleteInitScriptResult(nsId, infraId, nodeId string) {
	_ = kvstore.Delete(common.GenInfraNodeInitScriptKey(nsId, infraId, nodeId))
	if v, ok := initResults.Load(initResultKey(nsId, infraId)); ok {
		idx := v.(*initResultIndex)
		idx.mu.Lock()
		delete(idx.m, nodeId)
		empty := len(idx.m) == 0
		idx.mu.Unlock()
		if empty {
			initResults.Delete(initResultKey(nsId, infraId))
		}
	}
}

// GetInfraInitScriptStatus summarises the init script results of an Infra from the Node id list, the NodeGroup
// blueprints and one range read of the results; Node records are not loaded.
func GetInfraInitScriptStatus(nsId, infraId, option, statusFilter string) (*model.InfraInitScriptStatus, error) {
	if exists, err := CheckInfra(nsId, infraId); err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("the infra %s does not exist", infraId)
	}
	nodeIds, err := ListNodeId(nsId, infraId)
	if err != nil {
		return nil, err
	}
	results, err := listInitScriptResults(nsId, infraId)
	if err != nil {
		return nil, err
	}
	ngMap := LoadInfraNodeGroupMap(nsId, infraId)
	ngIds := make([]string, 0, len(ngMap))
	for id := range ngMap {
		ngIds = append(ngIds, id)
	}
	sort.Slice(ngIds, func(i, j int) bool { return len(ngIds[i]) > len(ngIds[j]) }) // longest prefix wins
	groupOf := func(nodeId string) string {
		for _, id := range ngIds {
			if strings.HasPrefix(nodeId, id+"-") {
				return id
			}
		}
		return ""
	}
	out := &model.InfraInitScriptStatus{InfraId: infraId, Total: len(nodeIds)}
	if option != "summary" {
		out.Nodes = make([]model.NodeInitScriptStatus, 0, len(nodeIds))
	}
	sort.Strings(nodeIds)
	for _, nodeId := range nodeIds {
		ngId := groupOf(nodeId)
		entry := model.NodeInitScriptStatus{NodeId: nodeId, NodeGroupId: ngId}
		if r, ok := results[nodeId]; ok {
			entry.InitScriptResult = r
			out.Reported++
			switch r.Status {
			case "ok":
				out.Ok++
			case model.InitScriptStatusInitializing:
				out.Initializing++
			default:
				out.Fail++
			}
		} else if ng, ok := ngMap[ngId]; ok && !initScriptExpected(&model.NodeInfo{InitScript: ng.InitScript}) {
			entry.Status = model.InitScriptStatusNotApplicable
			out.NotApplicable++
		} else {
			entry.Status = model.InitScriptStatusPending
			out.Pending++
		}
		if out.Nodes != nil && (statusFilter == "" || entry.Status == statusFilter) {
			out.Nodes = append(out.Nodes, entry)
		}
	}
	return out, nil
}

func activeNotifier() *model.NotifierInfo {
	if NotifierProvider == nil {
		return nil
	}
	if n := NotifierProvider(); n != nil && n.Status == model.NotifierStatusActive && n.WriteToken != "" {
		return n
	}
	return nil
}

// publicKeyCache keeps derived public keys per sshKey so that provisioning thousands of Nodes with one key
// reads OpenBao once instead of once per Node.
var publicKeyCache sync.Map // "ns/sshKeyId" -> publicKeyEntry

type publicKeyEntry struct {
	key string
	exp time.Time
}

const publicKeyCacheTTL = 10 * time.Minute

func sshPublicKey(nsId, sshKeyId string) (string, error) {
	if sshKeyId == "" {
		return "", errors.New("node has no sshKeyId; cannot render the init script access block")
	}
	cacheKey := nsId + "/" + sshKeyId
	if v, ok := publicKeyCache.Load(cacheKey); ok && time.Now().Before(v.(publicKeyEntry).exp) {
		return v.(publicKeyEntry).key, nil
	}
	key, err := loadSshPublicKey(nsId, sshKeyId)
	if err == nil {
		publicKeyCache.Store(cacheKey, publicKeyEntry{key: key, exp: time.Now().Add(publicKeyCacheTTL)})
	}
	return key, err
}

func loadSshPublicKey(nsId, sshKeyId string) (string, error) {
	res, err := resource.GetResource(nsId, model.StrSSHKey, sshKeyId)
	if err != nil {
		return "", fmt.Errorf("failed to read sshKey %s: %w", sshKeyId, err)
	}
	key, ok := res.(model.SshKeyInfo)
	if !ok {
		return "", fmt.Errorf("unexpected sshKey record type for %s", sshKeyId)
	}
	if pub := strings.TrimSpace(key.PublicKey); pub != "" {
		return pub, nil
	}
	// Some CSPs (e.g. AWS) return only the private key; derive the public key from it.
	privateKey, err := secret.GetSshKey(context.Background(), nsId, sshKeyId)
	if err != nil || privateKey == "" {
		return "", fmt.Errorf("sshKey %s has no public key and its private key is unavailable: %v", sshKeyId, err)
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", fmt.Errorf("sshKey %s: cannot parse private key: %w", sshKeyId, err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
}

// exportLines renders sorted "export K='v'" lines; values are validated to contain no single quote or newline.
func exportLines(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "export %s='%s'\n", k, env[k])
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
