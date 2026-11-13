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
	"strings"
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
	fallbackVersion    = "2.37.2" // used when the GitHub releases API is unavailable
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

// CreateNtfySet provisions an ntfy server from an Infra template, configures it, and registers it as the notifier.
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
	node, err := managedset.PublicNode(infraInfo)
	if err != nil {
		return nil, err
	}
	publicIP := node.PublicIP

	fqdn := ""
	if domain != "" {
		fqdn, err = chooseRecordName(ctx, domain, recordName, publicIP)
		if err != nil {
			return nil, err
		}
		log.Info().Str("record", fqdn).Str("ip", publicIP).Msg("notifier: upserting Route53 record")
		if err := managedset.UpsertDnsRecord(ctx, domain, fqdn, publicIP); err != nil {
			return nil, err
		}
	}

	script := withEnv(map[string]string{
		"TB_NTFY_IP": publicIP, "TB_NTFY_FQDN": fqdn, "TB_NTFY_TOPIC": topic,
		"TB_NTFY_WRITE_TOKEN": writeToken, "TB_NTFY_READ_TOKEN": readToken,
		"TB_NTFY_WRITER_HASH": string(writerHash), "TB_NTFY_READER_HASH": string(readerHash),
		"TB_NTFY_VERSION": fallbackVersion,
	}, installScript())

	log.Info().Str("infra", infraInfo.Id).Str("ip", publicIP).Str("fqdn", fqdn).Msg("notifier: installing ntfy")
	stdout, err := runWithRetry(nsId, infraInfo.Id, shellCommand(script))
	if err == nil {
		_, err = extractCA(stdout)
	}
	if err != nil {
		if fqdn != "" {
			managedset.DeleteDnsRecord(domain, fqdn, "notifier") // a retry draws a fresh name
		}
		return nil, fmt.Errorf("ntfy install failed on infra '%s': %w", infraInfo.Id, err)
	}
	caCert, _ := extractCA(stdout)

	info := &model.NotifierInfo{
		Type: model.NotifierTypeNtfy, Url: "https://" + publicIP, Topic: topic,
		WriteToken: writeToken, ReadToken: readToken, CaCert: caCert,
		Source: model.NotifierSourceManaged, ManagedNsId: nsId, ManagedInfraId: infraInfo.Id,
		ManagedVNetId: node.VNetId, ManagedSecurityGroupIds: node.SecurityGroupIds, ManagedSshKeyId: node.SshKeyId,
		RegisteredAt: time.Now(),
	}
	if fqdn != "" {
		// Publicly trusted certificate: clients need no CA.
		info.Url, info.CaCert = "https://"+fqdn, ""
		info.ManagedDnsDomain, info.ManagedDnsRecord = domain, fqdn
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

// AddReader creates a read-only ntfy user on the managed server. The password is returned once.
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
	if _, err := runOnce(info.ManagedNsId, info.ManagedInfraId, shellCommand(script)); err != nil {
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

// RemoveReader deletes a reader account from the managed server.
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
	if _, err := runOnce(info.ManagedNsId, info.ManagedInfraId, shellCommand("ntfy user remove '"+name+"'")); err != nil {
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
	if info.CaCert == "" {
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

func runWithRetry(nsId, infraId, command string) (string, error) {
	return managedset.RunScript(nsId, infraId, command, 6, "notifier")
}

func runOnce(nsId, infraId, command string) (string, error) {
	return managedset.RunOnce(nsId, infraId, command, 20)
}

func extractCA(stdout string) (string, error) {
	return managedset.ExtractMarked(stdout, caBegin, caEnd)
}

func deleteManagedResources(info *model.NotifierInfo) {
	managedset.DeleteManagedResources(info.ManagedNsId, info.ManagedVNetId, info.ManagedSecurityGroupIds,
		info.ManagedSshKeyId, info.ManagedDnsDomain, info.ManagedDnsRecord, "notifier")
}
