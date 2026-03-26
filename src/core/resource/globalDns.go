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

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/openbao/openbao/api/v2"
)

// UpdateGlobalDnsRecord updates a DNS record in Route53
func UpdateGlobalDnsRecord(req *model.GlobalDnsRecordReq) (model.SimpleMsg, error) {
	req.DomainName = strings.TrimSpace(req.DomainName)

	if req.RecordName == "" {
		req.RecordName = req.DomainName
	}
	if req.RecordType == "" {
		req.RecordType = "A"
	}

	// 1. Validate mutual exclusivity of IP sources
	count := 0
	if req.SetBy.Mci != nil {
		count++
	}
	if req.SetBy.Label != nil {
		count++
	}
	if len(req.SetBy.Ips) > 0 {
		count++
	}

	if count == 0 {
		return model.SimpleMsg{}, fmt.Errorf("at least one IP source (Mci, Label, or Ips) must be provided in 'setBy'")
	}
	if count > 1 {
		return model.SimpleMsg{}, fmt.Errorf("only one IP source (Mci, Label, or Ips) can be provided at a time in 'setBy'")
	}

	// 2. Resolve IPs
	var ips []string
	if req.SetBy.Mci != nil {
		mciIps, err := getVmIpsByMci(req.SetBy.Mci.NsId, req.SetBy.Mci.MciId)
		if err != nil {
			return model.SimpleMsg{}, err
		}
		ips = append(ips, mciIps...)
	} else if req.SetBy.Label != nil {
		labelIps, err := getVmIpsByLabel(req.SetBy.Label.NsId, req.SetBy.Label.LabelSelector)
		if err != nil {
			return model.SimpleMsg{}, err
		}
		ips = append(ips, labelIps...)
	} else {
		ips = append(ips, req.SetBy.Ips...)
	}

	// Remove duplicates
	ips = uniqueStringSlice(ips)

	if len(ips) == 0 {
		return model.SimpleMsg{}, fmt.Errorf("no IP addresses found or resolved from the provided source")
	}

	// 2. Fetch AWS credentials from OpenBao
	if model.VaultToken == "" {
		return model.SimpleMsg{}, fmt.Errorf("VAULT_TOKEN is not set")
	}

	awsCreds, err := fetchAWSCredsFromOpenBao(model.VaultAddr, model.VaultToken)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to fetch AWS credentials from OpenBao: %w", err)
	}

	// 3. Update Route53
	ctx := context.Background()
	r53, err := newRoute53Client(ctx, awsCreds)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to create AWS client: %w", err)
	}

	zoneID, _, err := findHostedZone(ctx, r53, req.DomainName)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to find hosted zone for %s: %w", req.DomainName, err)
	}

	err = upsertRoute53Record(ctx, r53, zoneID, req.RecordName, req.RecordType, req.TTL, ips)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to update record: %w", err)
	}

	return model.SimpleMsg{Message: "Successfully updated record " + req.RecordName}, nil
}

// GetGlobalDnsRecord lists DNS records from Route53
func GetGlobalDnsRecord(domainName string, recordName string, recordType string) (model.RestGetGlobalDnsRecordResponse, error) {
	domainName = strings.TrimSpace(domainName)

	// 1. Fetch AWS credentials from OpenBao
	if model.VaultToken == "" {
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("VAULT_TOKEN is not set")
	}

	awsCreds, err := fetchAWSCredsFromOpenBao(model.VaultAddr, model.VaultToken)
	if err != nil {
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to fetch AWS credentials from OpenBao: %w", err)
	}

	// 2. Query Route53
	ctx := context.Background()
	r53, err := newRoute53Client(ctx, awsCreds)
	if err != nil {
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to create AWS client: %w", err)
	}

	zoneID, _, err := findHostedZone(ctx, r53, domainName)
	if err != nil {
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to find hosted zone for %s: %w", domainName, err)
	}

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	}
	if recordName != "" {
		input.StartRecordName = aws.String(recordName)
	}
	if recordType != "" {
		input.StartRecordType = types.RRType(recordType)
	}

	out, err := r53.ListResourceRecordSets(ctx, input)
	if err != nil {
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to list records: %w", err)
	}

	var res model.RestGetGlobalDnsRecordResponse
	for _, rs := range out.ResourceRecordSets {
		if recordName != "" && !strings.Contains(aws.ToString(rs.Name), recordName) {
			continue
		}
		if recordType != "" && rs.Type != types.RRType(recordType) {
			continue
		}

		info := model.GlobalDnsRecordInfo{
			Name: aws.ToString(rs.Name),
			Type: string(rs.Type),
			TTL:  aws.ToInt64(rs.TTL),
		}
		for _, val := range rs.ResourceRecords {
			info.Values = append(info.Values, aws.ToString(val.Value))
		}
		res.Record = append(res.Record, info)
	}

	return res, nil
}

// Helpers (Internal)

func uniqueStringSlice(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func getVmIpsByMci(nsId, mciId string) ([]string, error) {
	key := "/ns/" + nsId + "/mci/" + mciId + "/vm/"
	kvList, err := kvstore.GetKvList(key)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, kv := range kvList {
		var vm model.VmInfo
		err = json.Unmarshal([]byte(kv.Value), &vm)
		if err == nil && vm.PublicIP != "" {
			ips = append(ips, vm.PublicIP)
		}
	}
	return ips, nil
}

func getVmIpsByLabel(nsId, labelSelector string) ([]string, error) {
	resources, err := label.GetResourcesByLabelSelector(model.StrVM, labelSelector)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, res := range resources {
		if vm, ok := res.(*model.VmInfo); ok {
			if vm.PublicIP != "" {
				ips = append(ips, vm.PublicIP)
			}
		}
	}
	return ips, nil
}

type awsCreds struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

func fetchAWSCredsFromOpenBao(vaultAddr, vaultToken string) (*awsCreds, error) {
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = vaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, err
	}
	client.SetToken(vaultToken)

	secret, err := client.Logical().Read("secret/data/csp/aws")
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found at secret/data/csp/aws")
	}
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format: 'data' field missing or not a map")
	}

	keyID, _ := data["AWS_ACCESS_KEY_ID"].(string)
	secretKey, _ := data["AWS_SECRET_ACCESS_KEY"].(string)
	region, _ := data["AWS_DEFAULT_REGION"].(string)
	if region == "" {
		region = "us-east-1"
	}

	if keyID == "" || secretKey == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID or AWS_SECRET_ACCESS_KEY is missing in secret")
	}

	return &awsCreds{AccessKeyID: keyID, SecretAccessKey: secretKey, Region: region}, nil
}

func newRoute53Client(ctx context.Context, creds *awsCreds) (*route53.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}
	return route53.NewFromConfig(cfg), nil
}

func findHostedZone(ctx context.Context, r53 *route53.Client, domain string) (string, string, error) {
	lookup := domain
	if !strings.HasSuffix(lookup, ".") {
		lookup += "."
	}
	out, err := r53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{DNSName: aws.String(lookup)})
	if err != nil {
		return "", "", err
	}
	for _, zone := range out.HostedZones {
		name := aws.ToString(zone.Name)
		if strings.HasSuffix(lookup, strings.TrimSuffix(name, ".")+".") || lookup == name {
			return aws.ToString(zone.Id), name, nil
		}
	}
	return "", "", fmt.Errorf("no hosted zone found for domain %q", domain)
}

func upsertRoute53Record(ctx context.Context, r53 *route53.Client, zoneID, name, rtype string, ttl int64, values []string) error {
	if ttl == 0 {
		ttl = 300
	}
	var resRecords []types.ResourceRecord
	for _, v := range values {
		resRecords = append(resRecords, types.ResourceRecord{Value: aws.String(v)})
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name:            aws.String(name),
						Type:            types.RRType(rtype),
						TTL:             aws.Int64(ttl),
						ResourceRecords: resRecords,
					},
				},
			},
		},
	}
	_, err := r53.ChangeResourceRecordSets(ctx, input)
	return err
}
