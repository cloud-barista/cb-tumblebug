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

package observability

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/embedded"
	"github.com/cloud-barista/cb-tumblebug/src/core/infra"
	"github.com/cloud-barista/cb-tumblebug/src/core/managedset"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

const (
	defaultSetNs         = model.SystemCommonNs
	defaultSetTemplate   = "infra-obs"
	defaultSetName       = "obs"
	defaultRecordName    = "obs"
	defaultRetentionDays = 15
	caBegin              = "-----TB-CA-BEGIN-----"
	caEnd                = "-----TB-CA-END-----"
)

var readerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)

// installScript returns the stack install script, preferring an override on disk
// (TB_ROOT_PATH/scripts/observability/install-otel-stack.sh) so it can be changed without a rebuild.
func installScript() string {
	if root := os.Getenv("TB_ROOT_PATH"); root != "" {
		if b, err := os.ReadFile(filepath.Join(root, "scripts", "observability", "install-otel-stack.sh")); err == nil && len(b) > 0 {
			log.Info().Msg("observability: using install-otel-stack.sh override from TB_ROOT_PATH")
			return string(b)
		}
	}
	return embedded.InstallOtelStack()
}

// CreateStack provisions the observability stack from an Infra template, installs it, and registers it.
func CreateStack(ctx context.Context, req model.ObservabilityStackReq) (*model.ObservabilityInfo, error) {
	if Current() != nil {
		return nil, errors.New("an observability stack is already registered; unregister it first")
	}
	nsId := managedset.FirstNonEmpty(req.NsId, defaultSetNs)
	templateId := managedset.FirstNonEmpty(req.TemplateId, defaultSetTemplate)
	name := managedset.FirstNonEmpty(req.Name, defaultSetName)
	description := managedset.FirstNonEmpty(req.Description, "observability stack for CB-Tumblebug nodes")
	domain := strings.TrimSuffix(strings.TrimSpace(req.DomainName), ".")
	retention := req.RetentionDays
	if retention <= 0 {
		retention = defaultRetentionDays
	}
	token := "tk_" + managedset.RandomToken(29)
	grafanaPassword := managedset.RandomToken(20)

	infraInfo, err := managedset.ProvisionOrReuse(ctx, nsId, templateId, name, description, "observability")
	if err != nil {
		return nil, err
	}
	node, err := managedset.PublicNode(infraInfo)
	if err != nil {
		return nil, err
	}
	publicIP := node.PublicIP

	fqdn := ""
	if domain != "" {
		if fqdn, err = managedset.ChooseRecordName(ctx, domain, strings.TrimSpace(req.RecordName), defaultRecordName, publicIP); err != nil {
			return nil, err
		}
		log.Info().Str("record", fqdn).Str("ip", publicIP).Msg("observability: upserting Route53 record")
		if err := managedset.UpsertDnsRecord(ctx, domain, fqdn, publicIP); err != nil {
			return nil, err
		}
	}

	script := managedset.WithEnv(map[string]string{
		"TB_OBS_IP": publicIP, "TB_OBS_FQDN": fqdn, "TB_OBS_TOKEN": token, "TB_OBS_GRAFANA_PASSWORD": grafanaPassword,
		"TB_OBS_RETENTION_DAYS":      strconv.Itoa(retention),
		"TB_OBS_AGENT_INSTALLER_B64": base64.StdEncoding.EncodeToString([]byte(embedded.InstallOtelAgent())),
		"TB_OBS_DASHBOARDS_TGZ_B64":  base64.StdEncoding.EncodeToString(embedded.DashboardsTarGz()),
	}, installScript())

	log.Info().Str("infra", infraInfo.Id).Str("ip", publicIP).Str("fqdn", fqdn).Msg("observability: installing the stack (several minutes)")
	stdout, err := managedset.RunScript(nsId, infraInfo.Id, managedset.ShellCommand(script), 6, "observability")
	host := publicIP
	caCert := ""
	if err == nil {
		if fqdn != "" {
			host = fqdn // publicly trusted certificate: clients need no CA
		} else {
			caCert, err = managedset.ExtractMarked(stdout, caBegin, caEnd)
		}
	}
	if err != nil {
		if fqdn != "" {
			managedset.DeleteDnsRecord(domain, fqdn, "observability") // a retry draws a fresh name
		}
		return nil, fmt.Errorf("stack install failed on infra '%s': %w", infraInfo.Id, err)
	}
	info := &model.ObservabilityInfo{
		OtlpEndpoint: "https://" + host, WriteToken: token, CaCert: caCert,
		GrafanaUrl: "https://" + host, GrafanaAdminUser: "admin", GrafanaAdminPassword: grafanaPassword,
		Source: model.ObservabilitySourceManaged, ManagedNsId: nsId, ManagedInfraId: infraInfo.Id,
		ManagedVNetId: node.VNetId, ManagedSecurityGroupIds: node.SecurityGroupIds, ManagedSshKeyId: node.SshKeyId,
		ManagedDnsDomain: domain, ManagedDnsRecord: fqdn, RetentionDays: retention, RegisteredAt: time.Now(),
	}
	if _, err := register(info); err != nil {
		return nil, err
	}
	if result := Test(90 * time.Second); !result.Success {
		SetStatus(model.ObservabilityStatusError, "round-trip test failed: "+result.Message)
	}
	return Get(), nil
}

// DeleteStack unregisters the stack and, when requested, deletes the managed Infra with its resources.
func DeleteStack(deleteInfra bool) error {
	info := Current()
	if info == nil {
		return errors.New("no observability stack is registered")
	}
	if err := Unregister(); err != nil {
		return err
	}
	if deleteInfra && info.Source == model.ObservabilitySourceManaged && info.ManagedInfraId != "" {
		if _, err := infra.DelInfra(info.ManagedNsId, info.ManagedInfraId, model.ActionTerminate); err != nil {
			return fmt.Errorf("stack unregistered but failed to delete infra '%s': %w", info.ManagedInfraId, err)
		}
		managedset.DeleteManagedResources(info.ManagedNsId, info.ManagedVNetId, info.ManagedSecurityGroupIds,
			info.ManagedSshKeyId, info.ManagedDnsDomain, info.ManagedDnsRecord, "observability")
	}
	return nil
}

// AddReader creates a Grafana viewer account. The password is returned once.
func AddReader(req model.ObservabilityReaderReq) (*model.ObservabilityReaderInfo, error) {
	info, err := grafanaAdmin()
	if err != nil {
		return nil, err
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !readerNamePattern.MatchString(name) || name == "admin" {
		return nil, errors.New("name must be 2-32 lowercase letters, digits, '-' or '_' and not 'admin'")
	}
	for _, r := range info.Readers {
		if r == name {
			return nil, fmt.Errorf("reader '%s' already exists", name)
		}
	}
	password := req.Password
	if password == "" {
		password = managedset.RandomToken(16)
	}
	body, _ := json.Marshal(map[string]any{"name": name, "login": name, "password": password, "OrgId": 1})
	if _, err := grafanaDo(info, http.MethodPost, "/api/admin/users", string(body)); err != nil {
		return nil, fmt.Errorf("failed to create reader '%s': %w", name, err)
	}
	updateInfo(func(i *model.ObservabilityInfo) { i.Readers = append(i.Readers, name) })
	return &model.ObservabilityReaderInfo{
		Name: name, Password: password, GrafanaUrl: info.GrafanaUrl,
		Note: "Viewer account: log in to Grafana with it; dashboards are in the CB-Tumblebug folder.",
	}, nil
}

// ListReaders returns the reader account names created through AddReader.
func ListReaders() ([]string, error) {
	info, err := grafanaAdmin()
	if err != nil {
		return nil, err
	}
	if info.Readers == nil {
		return []string{}, nil
	}
	return info.Readers, nil
}

// RemoveReader deletes a reader account from Grafana.
func RemoveReader(name string) error {
	info, err := grafanaAdmin()
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
	raw, err := grafanaGet(info, "/api/users/lookup?loginOrEmail="+url.QueryEscape(name))
	if err != nil {
		return fmt.Errorf("failed to look up reader '%s': %w", name, err)
	}
	var user struct {
		Id int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &user); err != nil || user.Id == 0 {
		return fmt.Errorf("failed to look up reader '%s'", name)
	}
	if _, err := grafanaDo(info, http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(user.Id, 10), ""); err != nil {
		return fmt.Errorf("failed to remove reader '%s': %w", name, err)
	}
	updateInfo(func(i *model.ObservabilityInfo) {
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

func grafanaAdmin() (*model.ObservabilityInfo, error) {
	info := Current()
	if info == nil {
		return nil, errors.New("no observability stack is registered")
	}
	if info.GrafanaUrl == "" || info.GrafanaAdminPassword == "" {
		return nil, errors.New("reader accounts need Grafana admin access (grafanaUrl and grafanaAdminPassword)")
	}
	return info, nil
}
