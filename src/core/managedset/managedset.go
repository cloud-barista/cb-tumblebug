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

// Package managedset holds what the CB-Tumblebug-managed service sets (ntfy notifier, observability stack)
// share: provisioning an Infra from a template, naming and upserting a Route53 record, running an install
// script on the VM, and cleaning the managed resources up again.
package managedset

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// ProvisionOrReuse creates the Infra from the template, or reuses an existing Infra of the same name so a
// failed install can be retried without re-provisioning.
func ProvisionOrReuse(ctx context.Context, nsId, templateId, name, description, who string) (*model.InfraInfo, error) {
	if existing, err := infra.GetInfraInfo(nsId, name); err == nil && existing != nil && len(existing.Node) > 0 {
		log.Info().Str("ns", nsId).Str("infra", name).Msgf("%s: reusing existing infra", who)
		return existing, nil
	}
	log.Info().Str("ns", nsId).Str("template", templateId).Str("name", name).Msgf("%s: provisioning infra", who)
	infraInfo, err := infra.CreateInfraDynamicFromTemplate(ctx, nsId, templateId,
		&model.TemplateApplyReq{Name: name, Description: description}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to provision infra '%s': %w", name, err)
	}
	return infraInfo, nil
}

// PublicNode returns the first Node of the Infra that has a public IP.
func PublicNode(infraInfo *model.InfraInfo) (*model.NodeInfo, error) {
	for i := range infraInfo.Node {
		if infraInfo.Node[i].PublicIP != "" {
			return &infraInfo.Node[i], nil
		}
	}
	return nil, fmt.Errorf("infra '%s' has no node with a public IP", infraInfo.Id)
}

// ChooseRecordName returns the FQDN for the server. An explicit record name is used as given; it must not
// exist yet unless it already points at ip (a retry for the same VM). Otherwise "<prefix>-<3 random chars>"
// is drawn until a free name is found, so repeated set creations never reuse a name (Let's Encrypt limits
// certificates and failed validations per exact name).
func ChooseRecordName(ctx context.Context, domain, recordName, prefix, ip string) (string, error) {
	toFQDN := func(name string) string {
		if strings.HasSuffix(name, "."+domain) || name == domain {
			return name
		}
		return name + "." + domain
	}
	if recordName != "" {
		fqdn := toFQDN(recordName)
		rec, err := findDnsRecord(ctx, domain, fqdn)
		if err != nil {
			return "", err
		}
		if rec != nil && !(len(rec.Values) == 1 && rec.Values[0] == ip) {
			return "", fmt.Errorf("DNS record '%s' already exists; choose another recordName or leave it empty", fqdn)
		}
		return fqdn, nil
	}
	for attempt := 0; attempt < 10; attempt++ {
		fqdn := toFQDN(prefix + "-" + RandomToken(3))
		rec, err := findDnsRecord(ctx, domain, fqdn)
		if err != nil {
			return "", err
		}
		if rec == nil {
			return fqdn, nil
		}
	}
	return "", fmt.Errorf("could not find a free '%s-xxx' record name under %s", prefix, domain)
}

func findDnsRecord(ctx context.Context, domain, fqdn string) (*model.GlobalDnsRecordInfo, error) {
	res, err := resource.GetGlobalDnsRecord(ctx, domain, fqdn, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query DNS records for %s: %w", domain, err)
	}
	for i := range res.Record {
		if strings.EqualFold(strings.TrimSuffix(res.Record[i].Name, "."), fqdn) {
			return &res.Record[i], nil
		}
	}
	return nil, nil
}

// DeleteDnsRecord removes an A record; used to undo UpsertDnsRecord when the install that followed failed.
func DeleteDnsRecord(domain, fqdn, who string) {
	if _, err := resource.DeleteGlobalDnsRecord(context.Background(), &model.GlobalDnsDeleteReq{
		DomainName: domain, RecordName: fqdn, RecordType: "A",
	}); err != nil {
		log.Warn().Err(err).Str("record", fqdn).Msgf("%s: left DNS record in place", who)
	}
}

// UpsertDnsRecord points fqdn at ip with a short TTL.
func UpsertDnsRecord(ctx context.Context, domain, fqdn, ip string) error {
	if _, err := resource.UpdateGlobalDnsRecord(ctx, &model.GlobalDnsRecordReq{
		DomainName: domain, RecordName: fqdn, RecordType: "A", TTL: 60, RoutingPolicy: "simple",
		SetBy: model.GlobalDnsIPSource{Ips: []string{ip}},
	}); err != nil {
		return fmt.Errorf("failed to create DNS record '%s': %w", fqdn, err)
	}
	return nil
}

// DeleteManagedResources removes the DNS record, SecurityGroups, SSHKey and vNet created for a managed Infra.
// A resource still used by another Infra fails to delete and is left in place.
func DeleteManagedResources(ns, vNetId string, sgIds []string, sshKeyId, dnsDomain, dnsRecord, who string) {
	if dnsRecord != "" {
		DeleteDnsRecord(dnsDomain, dnsRecord, who)
	}
	for _, sgId := range sgIds {
		if err := resource.DelResource(ns, model.StrSecurityGroup, sgId, "false"); err != nil {
			log.Warn().Err(err).Str("securityGroup", sgId).Msgf("%s: left managed securityGroup in place", who)
		}
	}
	if sshKeyId != "" {
		if err := resource.DelResource(ns, model.StrSSHKey, sshKeyId, "false"); err != nil {
			log.Warn().Err(err).Str("sshKey", sshKeyId).Msgf("%s: left managed sshKey in place", who)
		}
	}
	if vNetId != "" {
		if _, err := resource.DeleteVNet(ns, vNetId, string(resource.ActionWithSubnets)); err != nil {
			log.Warn().Err(err).Str("vNet", vNetId).Msgf("%s: left managed vNet in place", who)
		}
	}
}

// WithEnv prefixes a script with exported variables. Values are single-quoted; a quote in a value is rejected upstream.
func WithEnv(env map[string]string, script string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "export %s='%s'\n", k, env[k])
	}
	return b.String() + script
}

// ShellCommand wraps a script so it runs as root regardless of quoting.
func ShellCommand(script string) string {
	return "echo " + base64.StdEncoding.EncodeToString([]byte(script)) + " | base64 -d | sudo bash"
}

// RunScript runs the command on the Infra, retrying while SSH is not yet reachable after boot.
func RunScript(nsId, infraId, command string, attempts int, who string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		stdout, err := RunOnce(nsId, infraId, command, 30)
		if err == nil {
			return stdout, nil
		}
		lastErr = err
		if strings.Contains(err.Error(), "Process exited with status") {
			return "", err // the script ran and failed; running it again would only repeat the failure
		}
		log.Warn().Err(err).Int("attempt", attempt).Msgf("%s: install attempt failed, retrying", who)
		time.Sleep(15 * time.Second)
	}
	return "", lastErr
}

// RunOnce runs one command on the first Node of the Infra and returns its stdout.
func RunOnce(nsId, infraId, command string, timeoutMinutes int) (string, error) {
	results, err := infra.RemoteCommandToInfra(nsId, infraId, "", "", "",
		&model.InfraCmdReq{Command: []string{command}, TimeoutMinutes: timeoutMinutes}, "")
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New("no command result")
	}
	if results[0].Err != nil {
		return "", fmt.Errorf("%v (stderr: %s)", results[0].Err, strings.TrimSpace(results[0].Stderr[0]))
	}
	return results[0].Stdout[0], nil
}

// ExtractMarked returns the text between two marker lines of a script's output.
func ExtractMarked(stdout, begin, end string) (string, error) {
	start := strings.Index(stdout, begin)
	stop := strings.Index(stdout, end)
	if start < 0 || stop < 0 || stop <= start {
		return "", fmt.Errorf("%s block not found in install output", strings.Trim(begin, "-"))
	}
	return strings.TrimSpace(stdout[start+len(begin):stop]) + "\n", nil
}

// RandomToken returns n random lowercase letters and digits.
func RandomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// FirstNonEmpty returns the first non-empty value.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ValidateServerUrl accepts only an http(s) server URL (optional base path, no userinfo/query/fragment) that
// does not point at a link-local or cloud metadata address, and returns it without a trailing slash.
// CB-Tumblebug sends requests to such addresses on behalf of its callers, so the shape is restricted even
// though the caller is an administrator.
func ValidateServerUrl(rawUrl string) (string, error) {
	u, err := url.Parse(rawUrl) // url.Parse (not ParseRequestURI) so a fragment is seen and rejected
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid url: scheme must be http or https")
	}
	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid url: expected scheme://host[:port][/path] without credentials, query or fragment")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return "", fmt.Errorf("invalid url: link-local and metadata addresses are not allowed")
	}
	if strings.EqualFold(u.Hostname(), "metadata.google.internal") || strings.EqualFold(u.Hostname(), "metadata") {
		return "", fmt.Errorf("invalid url: metadata hosts are not allowed")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
