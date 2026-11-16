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

package notifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/embedded"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/managedset"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSetNs       = model.SystemCommonNs
	defaultSetTemplate = "infra-ntfy"
	defaultSetName     = "ntfy"
	defaultRecordName  = "ntfy"
	fallbackVersion    = "2.37.2"
	defaultSelfPort    = "9443"
	caBegin            = "-----TB-CA-BEGIN-----"
	caEnd              = "-----TB-CA-END-----"
)

var readerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

// installScript returns the ntfy install script, preferring an override on disk
// (TB_ROOT_PATH/scripts/notifier/install-ntfy.sh) so it can be changed without a rebuild.
func installScript() string {
	if root := os.Getenv("TB_ROOT_PATH"); root != "" {
		if b, err := os.ReadFile(filepath.Join(root, "scripts", "notifier", "install-ntfy.sh")); err == nil && len(b) > 0 {
			log.Info().Msg("notifier: using install-ntfy.sh override from TB_ROOT_PATH")
			return string(b)
		}
	}
	return embedded.InstallNtfy()
}

// CreateNtfySet provisions one or more ntfy servers from an Infra template, configures them with the same
// topic and tokens, and registers them together as the notifier.
func CreateNtfySet(ctx context.Context, req model.NotifierSetReq) (*model.NotifierInfo, error) {
	if Current() != nil {
		return nil, errors.New("a notifier is already registered; unregister it first")
	}
	nsId := firstNonEmpty(req.NsId, defaultSetNs)
	templateId := firstNonEmpty(req.TemplateId, defaultSetTemplate)
	name := firstNonEmpty(req.Name, defaultSetName)
	description := firstNonEmpty(req.Description, "ntfy server for CB-Tumblebug notifications")
	domain := strings.TrimSuffix(strings.TrimSpace(req.DomainName), ".")
	recordName := strings.TrimSpace(req.RecordName)

	topic := "tb-" + randomToken(8)
	writeToken, readToken := newAccessToken(), newAccessToken()
	writerHash, err := bcrypt.GenerateFromPassword([]byte(randomToken(24)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	readerHash, err := bcrypt.GenerateFromPassword([]byte(randomToken(24)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	infraInfo, err := provisionOrReuse(ctx, nsId, templateId, name, description)
	if err != nil {
		return nil, err
	}
	nodes, err := managedset.PublicNodes(infraInfo)
	if err != nil {
		return nil, err
	}
	// GetInfraInfo returns nodes sorted alphabetically by ID (e.g. "ntfy-cn-1" before "ntfy-kr-1").
	// Place DNS/Let's-Encrypt-eligible (non-Mainland-China) nodes first so the primary URL uses :443 Let's Encrypt
	// when a domain is given, while Mainland China nodes (where inbound 80/443 is blocked without ICP filing)
	// automatically use pinned self-signed TLS on :9443.
	sortNodesForNotifier(nodes)

	nodeFqdns := make([]string, len(nodes))
	var createdFqdns []string
	cleanupDns := func() {
		for _, rec := range createdFqdns {
			managedset.DeleteDnsRecord(domain, rec, "notifier")
		}
	}
	if domain != "" {
		firstDns := true
		for i, n := range nodes {
			if !canUsePublicDns(n) {
				log.Info().Str("node", n.Id).Str("ip", n.PublicIP).Msg("notifier: Mainland China region detected; skipping Route53/Let's Encrypt and using self-signed :9443")
				continue
			}
			wantedRecord := ""
			if firstDns {
				wantedRecord = recordName
				firstDns = false
			}
			fqdn, err := chooseRecordName(ctx, domain, wantedRecord, n.PublicIP)
			if err != nil {
				cleanupDns()
				return nil, err
			}
			log.Info().Str("node", n.Id).Str("record", fqdn).Str("ip", n.PublicIP).Msg("notifier: upserting Route53 record")
			if err := managedset.UpsertDnsRecord(ctx, domain, fqdn, n.PublicIP); err != nil {
				cleanupDns()
				return nil, err
			}
			nodeFqdns[i] = fqdn
			createdFqdns = append(createdFqdns, fqdn)
		}
	}

	type nodeOutcome struct {
		url    string
		caCert string
		err    error
	}
	outcomes := make([]nodeOutcome, len(nodes))
	baseScript := installScript()

	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, node *model.NodeInfo, nodeFqdn string) {
			defer wg.Done()
			env := map[string]string{
				"TB_NTFY_IP": node.PublicIP, "TB_NTFY_FQDN": nodeFqdn, "TB_NTFY_TOPIC": topic,
				"TB_NTFY_WRITE_TOKEN": writeToken, "TB_NTFY_READ_TOKEN": readToken,
				"TB_NTFY_WRITER_HASH": string(writerHash), "TB_NTFY_READER_HASH": string(readerHash),
				"TB_NTFY_VERSION": fallbackVersion,
			}
			if nodeFqdn == "" {
				env["TB_NTFY_PORT"] = defaultSelfPort
			}
			log.Info().Str("infra", infraInfo.Id).Str("node", node.Id).Str("ip", node.PublicIP).Str("fqdn", nodeFqdn).Msg("notifier: installing ntfy")
			stdout, runErr := managedset.RunScriptOnNode(nsId, infraInfo.Id, node.Id, shellCommand(withEnv(env, baseScript)), 6, "notifier")
			if runErr != nil {
				outcomes[idx].err = fmt.Errorf("node '%s' (%s): %w", node.Id, node.PublicIP, runErr)
				return
			}
			ca, caErr := extractCA(stdout)
			if caErr != nil {
				outcomes[idx].err = fmt.Errorf("node '%s' (%s): %w", node.Id, node.PublicIP, caErr)
				return
			}
			if nodeFqdn != "" {
				outcomes[idx].url = "https://" + nodeFqdn
			} else {
				outcomes[idx].url = "https://" + node.PublicIP + ":" + defaultSelfPort
				outcomes[idx].caCert = ca
			}
		}(i, n, nodeFqdns[i])
	}
	wg.Wait()

	var urls, caCerts []string
	for _, o := range outcomes {
		if o.err != nil {
			cleanupDns()
			return nil, fmt.Errorf("ntfy install failed on infra '%s': %w", infraInfo.Id, o.err)
		}
		urls = append(urls, o.url)
		if o.caCert != "" {
			caCerts = append(caCerts, o.caCert)
		}
	}

	var vNetIds, sgIds, sshKeyIds []string
	for _, n := range nodes {
		if n.VNetId != "" {
			vNetIds = append(vNetIds, n.VNetId)
		}
		sgIds = append(sgIds, n.SecurityGroupIds...)
		if n.SshKeyId != "" {
			sshKeyIds = append(sshKeyIds, n.SshKeyId)
		}
	}

	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: urls[0], Urls: urls, Topic: topic,
		WriteToken: writeToken, ReadToken: readToken, CaCert: strings.Join(caCerts, ""),
		Source: model.NotifierSourceManaged, ManagedNsId: nsId, ManagedInfraId: infraInfo.Id,
		ManagedVNetId: nodes[0].VNetId, ManagedVNetIds: vNetIds,
		ManagedSecurityGroupIds: sgIds,
		ManagedSshKeyId:         nodes[0].SshKeyId, ManagedSshKeyIds: sshKeyIds,
		RegisteredAt: time.Now(),
	}
	if len(createdFqdns) > 0 {
		info.ManagedDnsDomain = domain
		info.ManagedDnsRecord = createdFqdns[0]
		info.ManagedDnsRecords = createdFqdns
	}
	if _, err := register(info); err != nil {
		return nil, err
	}
	result := Test(20 * time.Second)
	if !result.Success {
		SetStatus(model.NotifierStatusError, "round-trip test failed: "+result.Message)
	}
	return Get(), nil
}

// DeleteNtfySet unregisters the notifier and, when requested, deletes the managed Infra with its resources.
func DeleteNtfySet(deleteInfra bool) error {
	info := Current()
	if info == nil {
		return errors.New("no notifier is registered")
	}
	if err := Unregister(); err != nil {
		return err
	}
	if deleteInfra && info.Source == model.NotifierSourceManaged && info.ManagedInfraId != "" {
		if _, err := infra.DelInfra(info.ManagedNsId, info.ManagedInfraId, model.ActionTerminate); err != nil {
			return fmt.Errorf("notifier unregistered but failed to delete infra '%s': %w", info.ManagedInfraId, err)
		}
		deleteManagedResources(info)
	}
	return nil
}

// AddReader creates a read-only ntfy user on every managed server of the notifier. The password is returned once.
func AddReader(req model.NotifierReaderReq) (*model.NotifierReaderInfo, error) {
	info, err := managedNotifier()
	if err != nil {
		return nil, err
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !readerNamePattern.MatchString(name) || name == "tb-writer" || name == "tb-reader" {
		return nil, errors.New("name must be 2-32 lowercase letters, digits, '-' or '_' and not a reserved account")
	}
	for _, r := range info.Readers {
		if r == name {
			return nil, fmt.Errorf("reader '%s' already exists", name)
		}
	}
	password := req.Password
	if password == "" {
		password = randomToken(16)
	}
	if strings.ContainsAny(password, "'\n") {
		return nil, errors.New("password must not contain quotes or newlines")
	}
	script := fmt.Sprintf("set -e\nNTFY_PASSWORD='%s' ntfy user add --role=user '%s'\nntfy access '%s' '%s' ro\n", password, name, name, info.Topic)
	if err := runOnAllNodes(info.ManagedNsId, info.ManagedInfraId, shellCommand(script)); err != nil {
		return nil, fmt.Errorf("failed to create reader '%s': %w", name, err)
	}
	updateInfo(func(i *model.NotifierInfo) { i.Readers = append(i.Readers, name) })
	return &model.NotifierReaderInfo{
		Name: name, Password: password, ServerUrl: info.Url, Topic: info.Topic, CaCert: info.CaCert,
		Note: readerNote(info),
	}, nil
}

// ListReaders returns the reader account names created through AddReader.
func ListReaders() ([]string, error) {
	info, err := managedNotifier()
	if err != nil {
		return nil, err
	}
	if info.Readers == nil {
		return []string{}, nil
	}
	return info.Readers, nil
}

// RemoveReader deletes a reader account from every managed server of the notifier.
func RemoveReader(name string) error {
	info, err := managedNotifier()
	if err != nil {
		return err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	found := false
	for _, r := range info.Readers {
		if r == name {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("reader '%s' not found", name)
	}
	if err := runOnAllNodes(info.ManagedNsId, info.ManagedInfraId, shellCommand("ntfy user remove '"+name+"'")); err != nil {
		return fmt.Errorf("failed to remove reader '%s': %w", name, err)
	}
	updateInfo(func(i *model.NotifierInfo) {
		kept := i.Readers[:0]
		for _, r := range i.Readers {
			if r != name {
				kept = append(kept, r)
			}
		}
		i.Readers = kept
	})
	return nil
}

func managedNotifier() (*model.NotifierInfo, error) {
	info := Current()
	if info == nil {
		return nil, errors.New("no notifier is registered")
	}
	if info.Source != model.NotifierSourceManaged {
		return nil, errors.New("reader accounts are managed only for an ntfy server provisioned by CB-Tumblebug")
	}
	return info, nil
}

func readerNote(info *model.NotifierInfo) string {
	if info.CaCert == "" || info.ManagedDnsRecord != "" {
		return "Add the server in the ntfy app or web UI, log in with this account, then subscribe to the topic."
	}
	return "Self-signed certificate: phone apps cannot subscribe; use the web UI or curl with caCert as a trusted CA."
}

var (
	withEnv       = managedset.WithEnv
	shellCommand  = managedset.ShellCommand
	firstNonEmpty = managedset.FirstNonEmpty
)

func provisionOrReuse(ctx context.Context, nsId, templateId, name, description string) (*model.InfraInfo, error) {
	return managedset.ProvisionOrReuse(ctx, nsId, templateId, name, description, "notifier")
}

func chooseRecordName(ctx context.Context, domain, recordName, ip string) (string, error) {
	return managedset.ChooseRecordName(ctx, domain, recordName, defaultRecordName, ip)
}

func runOnAllNodes(nsId, infraId, command string) error {
	return managedset.RunOnceOnAllNodes(nsId, infraId, command, 20)
}

func extractCA(stdout string) (string, error) {
	return managedset.ExtractMarked(stdout, caBegin, caEnd)
}

func deleteManagedResources(info *model.NotifierInfo) {
	for _, extraRec := range info.ManagedDnsRecords {
		if extraRec != "" && extraRec != info.ManagedDnsRecord {
			managedset.DeleteDnsRecord(info.ManagedDnsDomain, extraRec, "notifier")
		}
	}
	vNets := append([]string{info.ManagedVNetId}, info.ManagedVNetIds...)
	keys := append([]string{info.ManagedSshKeyId}, info.ManagedSshKeyIds...)
	managedset.DeleteManagedResourceSet(info.ManagedNsId, vNets, info.ManagedSecurityGroupIds,
		keys, info.ManagedDnsDomain, info.ManagedDnsRecord, "notifier")
}

func sortNodesForNotifier(nodes []*model.NodeInfo) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return canUsePublicDns(nodes[i]) && !canUsePublicDns(nodes[j])
	})
}

// canUsePublicDns reports whether a Node can bind an un-filed Route53 domain and pass Let's Encrypt HTTP-01 (:80).
// Mainland China CSP regions block inbound TCP 80/443 without ICP Beian filing, whereas Hong Kong/Macau/Taiwan and
// all global regions allow it.
func canUsePublicDns(n *model.NodeInfo) bool {
	if n == nil {
		return false
	}
	provider := strings.ToLower(strings.TrimSpace(n.ConnectionConfig.ProviderName))
	region := strings.ToLower(strings.TrimSpace(firstNonEmpty(n.Region.Region, n.ConnectionConfig.RegionZoneInfo.AssignedRegion)))
	if parts := strings.Split(strings.ToLower(n.SpecId), "+"); len(parts) >= 2 {
		if provider == "" {
			provider = parts[0]
		}
		if region == "" {
			region = parts[1]
		}
	}
	if (provider == "" || region == "") && n.ConnectionName != "" {
		if idx := strings.IndexByte(n.ConnectionName, '-'); idx > 0 {
			if provider == "" {
				provider = strings.ToLower(n.ConnectionName[:idx])
			}
			if region == "" {
				region = strings.ToLower(n.ConnectionName[idx+1:])
			}
		}
	}
	if strings.Contains(region, "hongkong") || strings.Contains(region, "macau") || strings.Contains(region, "taipei") || strings.Contains(region, "taiwan") {
		return true
	}
	switch provider {
	case "alibaba", "aws":
		if strings.HasPrefix(region, "cn-") {
			return false
		}
	case "tencent":
		switch {
		case region == "ap-beijing", region == "ap-shanghai", region == "ap-guangzhou",
			region == "ap-chengdu", region == "ap-chongqing", region == "ap-nanjing",
			strings.HasPrefix(region, "ap-shanghai-"), strings.HasPrefix(region, "ap-beijing-"), strings.HasPrefix(region, "ap-shenzhen"):
			return false
		}
	case "azure":
		if strings.HasPrefix(region, "china") {
			return false
		}
	}
	return !strings.HasPrefix(region, "cn-")
}
