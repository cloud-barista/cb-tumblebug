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
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

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

// vmIPLocation holds a VM's public IP and geographic location.
type vmIPLocation struct {
	PublicIP   string
	Latitude   float64
	Longitude  float64
	Identifier string // used as SetIdentifier
}

// UpdateGlobalDnsRecord updates a DNS record in Route53
func UpdateGlobalDnsRecord(req *model.GlobalDnsRecordReq) (model.SimpleMsg, error) {
	log.Debug().Str("domainName", req.DomainName).Str("recordName", req.RecordName).Str("recordType", req.RecordType).Int64("ttl", req.TTL).Str("routingPolicy", req.RoutingPolicy).Msg("[DNS] UpdateGlobalDnsRecord called")
	req.DomainName = strings.TrimSpace(req.DomainName)
	req.RecordName = strings.TrimSuffix(strings.TrimSpace(req.RecordName), ".")

	if req.RecordName == "" {
		req.RecordName = req.DomainName
	} else if !strings.HasSuffix(req.RecordName, "."+req.DomainName) && req.RecordName != req.DomainName {
		req.RecordName = req.RecordName + "." + req.DomainName
		log.Debug().Str("adjustedRecordName", req.RecordName).Msg("[DNS] Record name adjusted to FQDN under the domain")
	}
	if req.RecordType == "" {
		req.RecordType = "A"
	}
	if req.RoutingPolicy == "" {
		req.RoutingPolicy = "simple"
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

	// Geoproximity requires MCI or Label (need location data)
	if req.RoutingPolicy == "geoproximity" && len(req.SetBy.Ips) > 0 {
		return model.SimpleMsg{}, fmt.Errorf("geoproximity routing requires MCI or Label source (location data needed); manual IPs are not supported")
	}

	// 2. Resolve IPs (and locations for geoproximity)
	var ips []string
	var vmLocs []vmIPLocation

	if req.SetBy.Mci != nil {
		if req.RoutingPolicy == "geoproximity" {
			locs, err := getVmIPLocsByMci(req.SetBy.Mci.NsId, req.SetBy.Mci.MciId)
			if err != nil {
				return model.SimpleMsg{}, err
			}
			vmLocs = append(vmLocs, locs...)
			for _, loc := range locs {
				ips = append(ips, loc.PublicIP)
			}
		} else {
			mciIps, err := getVmIpsByMci(req.SetBy.Mci.NsId, req.SetBy.Mci.MciId)
			if err != nil {
				return model.SimpleMsg{}, err
			}
			ips = append(ips, mciIps...)
		}
	} else if req.SetBy.Label != nil {
		if req.RoutingPolicy == "geoproximity" {
			locs, err := getVmIPLocsByLabel(req.SetBy.Label.NsId, req.SetBy.Label.LabelSelector)
			if err != nil {
				return model.SimpleMsg{}, err
			}
			vmLocs = append(vmLocs, locs...)
			for _, loc := range locs {
				ips = append(ips, loc.PublicIP)
			}
		} else {
			labelIps, err := getVmIpsByLabel(req.SetBy.Label.NsId, req.SetBy.Label.LabelSelector)
			if err != nil {
				return model.SimpleMsg{}, err
			}
			ips = append(ips, labelIps...)
		}
	} else {
		ips = append(ips, req.SetBy.Ips...)
	}

	if req.RoutingPolicy == "geoproximity" {
		log.Debug().Int("count", len(vmLocs)).Msg("[DNS] VM IP+Location entries resolved for geoproximity")
		if len(vmLocs) == 0 {
			return model.SimpleMsg{}, fmt.Errorf("no VMs with public IP found for geoproximity routing")
		}
	} else {
		ips = uniqueStringSlice(ips)
		log.Debug().Strs("resolvedIPs", ips).Int("count", len(ips)).Msg("[DNS] IP addresses resolved")
		if len(ips) == 0 {
			log.Warn().Msg("[DNS] No IP addresses resolved from the provided source")
			return model.SimpleMsg{}, fmt.Errorf("no IP addresses found or resolved from the provided source")
		}
	}

	// 3. Fetch AWS credentials from OpenBao
	r53, err := getRoute53Client()
	if err != nil {
		return model.SimpleMsg{}, err
	}

	ctx := context.Background()
	zoneID, _, err := findHostedZone(ctx, r53, req.DomainName)
	if err != nil {
		log.Error().Err(err).Str("domain", req.DomainName).Msg("[DNS] Failed to find hosted zone")
		return model.SimpleMsg{}, fmt.Errorf("failed to find hosted zone for %s: %w", req.DomainName, err)
	}
	log.Debug().Str("zoneID", zoneID).Str("domain", req.DomainName).Msg("[DNS] Hosted zone found")

	// 4. Upsert based on routing policy
	if req.RoutingPolicy == "geoproximity" {
		log.Debug().Str("recordName", req.RecordName).Int("vmCount", len(vmLocs)).Msg("[DNS] Upserting geoproximity records")
		err = upsertGeoproximityRecords(ctx, r53, zoneID, req.RecordName, req.RecordType, req.TTL, vmLocs)
		if err != nil {
			log.Error().Err(err).Msg("[DNS] Failed to upsert geoproximity records")
			return model.SimpleMsg{}, fmt.Errorf("failed to update geoproximity records: %w", err)
		}
		log.Info().Str("recordName", req.RecordName).Int("vmCount", len(vmLocs)).Msg("[DNS] Successfully updated geoproximity Route53 records")
		return model.SimpleMsg{Message: fmt.Sprintf("Successfully updated %d geoproximity records for %s", len(vmLocs), req.RecordName)}, nil
	}

	// Simple routing
	log.Debug().Str("zoneID", zoneID).Str("recordName", req.RecordName).Str("recordType", req.RecordType).Int64("ttl", req.TTL).Strs("ips", ips).Msg("[DNS] Upserting Route53 record")
	err = upsertRoute53Record(ctx, r53, zoneID, req.RecordName, req.RecordType, req.TTL, ips)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to upsert Route53 record")
		return model.SimpleMsg{}, fmt.Errorf("failed to update record: %w", err)
	}

	log.Info().Str("recordName", req.RecordName).Strs("ips", ips).Msg("[DNS] Successfully updated Route53 record")
	return model.SimpleMsg{Message: "Successfully updated record " + req.RecordName}, nil
}

// GetGlobalDnsRecord lists DNS records from Route53
func GetGlobalDnsRecord(domainName string, recordName string, recordType string) (model.RestGetGlobalDnsRecordResponse, error) {
	log.Debug().Str("domainName", domainName).Str("recordName", recordName).Str("recordType", recordType).Msg("[DNS] GetGlobalDnsRecord called")
	domainName = strings.TrimSpace(domainName)

	r53, err := getRoute53Client()
	if err != nil {
		return model.RestGetGlobalDnsRecordResponse{}, err
	}

	ctx := context.Background()
	zoneID, _, err := findHostedZone(ctx, r53, domainName)
	if err != nil {
		log.Error().Err(err).Str("domain", domainName).Msg("[DNS] Failed to find hosted zone")
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to find hosted zone for %s: %w", domainName, err)
	}
	log.Debug().Str("zoneID", zoneID).Str("domain", domainName).Msg("[DNS] Hosted zone found")

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	}
	if recordName != "" {
		input.StartRecordName = aws.String(recordName)
	}
	if recordType != "" {
		input.StartRecordType = types.RRType(recordType)
	}

	log.Debug().Str("zoneID", zoneID).Msg("[DNS] Listing Route53 record sets")
	out, err := r53.ListResourceRecordSets(ctx, input)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to list Route53 record sets")
		return model.RestGetGlobalDnsRecordResponse{}, fmt.Errorf("failed to list records: %w", err)
	}
	log.Debug().Int("totalRecordSets", len(out.ResourceRecordSets)).Msg("[DNS] Route53 record sets retrieved")

	var res model.RestGetGlobalDnsRecordResponse
	for _, rs := range out.ResourceRecordSets {
		if recordName != "" && !strings.Contains(aws.ToString(rs.Name), recordName) {
			continue
		}
		if recordType != "" && rs.Type != types.RRType(recordType) {
			continue
		}

		info := model.GlobalDnsRecordInfo{
			Name:          aws.ToString(rs.Name),
			Type:          string(rs.Type),
			TTL:           aws.ToInt64(rs.TTL),
			SetIdentifier: aws.ToString(rs.SetIdentifier),
		}

		// Detect routing policy
		if rs.GeoProximityLocation != nil {
			info.RoutingPolicy = "geoproximity"
			if rs.GeoProximityLocation.Coordinates != nil {
				info.GeoLatitude = aws.ToString(rs.GeoProximityLocation.Coordinates.Latitude)
				info.GeoLongitude = aws.ToString(rs.GeoProximityLocation.Coordinates.Longitude)
			}
		} else if rs.SetIdentifier != nil {
			info.RoutingPolicy = "other"
		} else {
			info.RoutingPolicy = "simple"
		}

		for _, val := range rs.ResourceRecords {
			info.Values = append(info.Values, aws.ToString(val.Value))
		}
		res.Record = append(res.Record, info)
	}

	log.Debug().Int("matchedRecords", len(res.Record)).Msg("[DNS] GetGlobalDnsRecord completed")
	return res, nil
}

// DeleteGlobalDnsRecord deletes DNS records from Route53
func DeleteGlobalDnsRecord(req *model.GlobalDnsDeleteReq) (model.SimpleMsg, error) {
	log.Debug().Str("domainName", req.DomainName).Str("recordName", req.RecordName).Str("recordType", req.RecordType).Str("setIdentifier", req.SetIdentifier).Msg("[DNS] DeleteGlobalDnsRecord called")

	req.DomainName = strings.TrimSpace(req.DomainName)
	req.RecordName = strings.TrimSuffix(strings.TrimSpace(req.RecordName), ".")

	if !strings.HasSuffix(req.RecordName, "."+req.DomainName) && req.RecordName != req.DomainName {
		req.RecordName = req.RecordName + "." + req.DomainName
	}
	if req.RecordType == "" {
		req.RecordType = "A"
	}

	r53, err := getRoute53Client()
	if err != nil {
		return model.SimpleMsg{}, err
	}

	ctx := context.Background()
	zoneID, _, err := findHostedZone(ctx, r53, req.DomainName)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to find hosted zone for %s: %w", req.DomainName, err)
	}
	log.Debug().Str("zoneID", zoneID).Msg("[DNS] Hosted zone found for delete")

	// List existing records to get exact match for DELETE
	listInput := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(zoneID),
		StartRecordName: aws.String(req.RecordName),
		StartRecordType: types.RRType(req.RecordType),
	}
	out, err := r53.ListResourceRecordSets(ctx, listInput)
	if err != nil {
		return model.SimpleMsg{}, fmt.Errorf("failed to list records for deletion: %w", err)
	}

	// Collect matching records
	var changes []types.Change
	fqdn := req.RecordName
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	for _, rs := range out.ResourceRecordSets {
		rsName := aws.ToString(rs.Name)
		if rsName != fqdn {
			continue
		}
		if rs.Type != types.RRType(req.RecordType) {
			continue
		}
		// If specific SetIdentifier requested, only delete that one
		if req.SetIdentifier != "" && aws.ToString(rs.SetIdentifier) != req.SetIdentifier {
			continue
		}

		change := types.Change{
			Action:            types.ChangeActionDelete,
			ResourceRecordSet: &rs,
		}
		changes = append(changes, change)
		log.Debug().Str("name", rsName).Str("setId", aws.ToString(rs.SetIdentifier)).Msg("[DNS] Marked record for deletion")
	}

	if len(changes) == 0 {
		return model.SimpleMsg{}, fmt.Errorf("no matching records found for deletion: %s %s", req.RecordName, req.RecordType)
	}

	log.Debug().Int("count", len(changes)).Msg("[DNS] Deleting Route53 records")
	deleteInput := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}
	_, err = r53.ChangeResourceRecordSets(ctx, deleteInput)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to delete records")
		return model.SimpleMsg{}, fmt.Errorf("failed to delete records: %w", err)
	}

	msg := fmt.Sprintf("Successfully deleted %d record(s) for %s", len(changes), req.RecordName)
	log.Info().Int("count", len(changes)).Str("recordName", req.RecordName).Msg("[DNS] Records deleted successfully")
	return model.SimpleMsg{Message: msg}, nil
}

// BulkDeleteGlobalDnsRecords deletes multiple DNS records in a single batch per domain.
// Records are grouped by domain and submitted as one ChangeBatch per domain to Route53.
func BulkDeleteGlobalDnsRecords(req *model.GlobalDnsBulkDeleteReq) (model.GlobalDnsBulkDeleteResponse, error) {
	log.Debug().Int("count", len(req.Records)).Msg("[DNS] BulkDeleteGlobalDnsRecords called")

	if len(req.Records) == 0 {
		return model.GlobalDnsBulkDeleteResponse{}, fmt.Errorf("no records provided for deletion")
	}

	r53, err := getRoute53Client()
	if err != nil {
		return model.GlobalDnsBulkDeleteResponse{}, err
	}
	ctx := context.Background()

	// Group records by domain
	domainGroups := make(map[string][]model.GlobalDnsDeleteReq)
	var skippedCount int
	for _, rec := range req.Records {
		rec.DomainName = strings.TrimSpace(rec.DomainName)
		rec.RecordName = strings.TrimSpace(rec.RecordName)
		if rec.DomainName == "" {
			skippedCount++
			continue
		}
		domainGroups[rec.DomainName] = append(domainGroups[rec.DomainName], rec)
	}

	resp := model.GlobalDnsBulkDeleteResponse{
		TotalRequested: len(req.Records),
	}
	// Count skipped records (empty domainName) as failures
	if skippedCount > 0 {
		resp.Failed += skippedCount
		for i := 0; i < skippedCount; i++ {
			resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
				Success: false, Message: "domainName is empty",
			})
		}
	}

	for domain, recs := range domainGroups {
		zoneID, _, err := findHostedZone(ctx, r53, domain)
		if err != nil {
			for _, rec := range recs {
				resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
					RecordName: rec.RecordName, RecordType: rec.RecordType, SetIdentifier: rec.SetIdentifier,
					Success: false, Message: fmt.Sprintf("hosted zone not found for %s", domain),
				})
				resp.Failed++
			}
			continue
		}

		// List all records in the zone once for matching
		listInput := &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}
		out, err := r53.ListResourceRecordSets(ctx, listInput)
		if err != nil {
			for _, rec := range recs {
				resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
					RecordName: rec.RecordName, RecordType: rec.RecordType, SetIdentifier: rec.SetIdentifier,
					Success: false, Message: "failed to list records: " + err.Error(),
				})
				resp.Failed++
			}
			continue
		}

		var changes []types.Change
		matchedRecs := make(map[int]bool) // track which request records matched

		for i, rec := range recs {
			recName := strings.TrimSuffix(rec.RecordName, ".")
			if !strings.HasSuffix(recName, "."+domain) && recName != domain {
				recName = recName + "." + domain
			}
			if rec.RecordType == "" {
				rec.RecordType = "A"
			}
			fqdn := recName
			if !strings.HasSuffix(fqdn, ".") {
				fqdn += "."
			}

			for _, rs := range out.ResourceRecordSets {
				rsName := aws.ToString(rs.Name)
				if rsName != fqdn || rs.Type != types.RRType(rec.RecordType) {
					continue
				}
				if rec.SetIdentifier != "" && aws.ToString(rs.SetIdentifier) != rec.SetIdentifier {
					continue
				}
				changes = append(changes, types.Change{
					Action:            types.ChangeActionDelete,
					ResourceRecordSet: &rs,
				})
				matchedRecs[i] = true
			}
		}

		// Mark unmatched records as failed
		for i, rec := range recs {
			if !matchedRecs[i] {
				resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
					RecordName: rec.RecordName, RecordType: rec.RecordType, SetIdentifier: rec.SetIdentifier,
					Success: false, Message: "no matching record found",
				})
				resp.Failed++
			}
		}

		if len(changes) == 0 {
			continue
		}

		// Submit single ChangeBatch for this domain
		log.Debug().Int("changeCount", len(changes)).Str("domain", domain).Msg("[DNS] Submitting bulk delete ChangeBatch")
		_, err = r53.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
			ChangeBatch:  &types.ChangeBatch{Changes: changes},
		})
		if err != nil {
			log.Error().Err(err).Str("domain", domain).Msg("[DNS] Bulk delete ChangeBatch failed")
			for i, rec := range recs {
				if matchedRecs[i] {
					resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
						RecordName: rec.RecordName, RecordType: rec.RecordType, SetIdentifier: rec.SetIdentifier,
						Success: false, Message: "batch delete failed: " + err.Error(),
					})
					resp.Failed++
				}
			}
		} else {
			log.Info().Int("count", len(changes)).Str("domain", domain).Msg("[DNS] Bulk delete succeeded")
			for i, rec := range recs {
				if matchedRecs[i] {
					resp.Results = append(resp.Results, model.GlobalDnsBulkDeleteResult{
						RecordName: rec.RecordName, RecordType: rec.RecordType, SetIdentifier: rec.SetIdentifier,
						Success: true, Message: "deleted successfully",
					})
					resp.Succeeded++
				}
			}
		}
	}

	return resp, nil
}

// ListHostedZones returns all hosted zones from Route53
func ListHostedZones() (model.RestGetHostedZonesResponse, error) {
	log.Debug().Msg("[DNS] ListHostedZones called")

	r53, err := getRoute53Client()
	if err != nil {
		return model.RestGetHostedZonesResponse{}, err
	}

	ctx := context.Background()
	out, err := r53.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to list hosted zones")
		return model.RestGetHostedZonesResponse{}, fmt.Errorf("failed to list hosted zones: %w", err)
	}

	var res model.RestGetHostedZonesResponse
	for _, zone := range out.HostedZones {
		res.HostedZones = append(res.HostedZones, model.HostedZoneInfo{
			ZoneId:      aws.ToString(zone.Id),
			Name:        aws.ToString(zone.Name),
			RecordCount: aws.ToInt64(zone.ResourceRecordSetCount),
		})
	}

	log.Debug().Int("count", len(res.HostedZones)).Msg("[DNS] Hosted zones listed")
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

func getVmIPLocsByMci(nsId, mciId string) ([]vmIPLocation, error) {
	key := "/ns/" + nsId + "/mci/" + mciId + "/vm/"
	kvList, err := kvstore.GetKvList(key)
	if err != nil {
		return nil, err
	}

	var locs []vmIPLocation
	for _, kv := range kvList {
		var vm model.VmInfo
		err = json.Unmarshal([]byte(kv.Value), &vm)
		if err == nil && vm.PublicIP != "" {
			locs = append(locs, vmIPLocation{
				PublicIP:   vm.PublicIP,
				Latitude:   vm.Location.Latitude,
				Longitude:  vm.Location.Longitude,
				Identifier: vm.Id,
			})
			log.Debug().Str("vmId", vm.Id).Str("ip", vm.PublicIP).Float64("lat", vm.Location.Latitude).Float64("lng", vm.Location.Longitude).Msg("[DNS] VM location resolved")
		}
	}
	return locs, nil
}

func getVmIpsByLabel(nsId, labelSelector string) ([]string, error) {
	log.Debug().Str("nsId", nsId).Str("labelSelector", labelSelector).Msg("[DNS] Getting VM IPs by label")

	if nsId != "" {
		labelSelector = model.LabelNamespace + "=" + nsId + "," + labelSelector
	}

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

func getVmIPLocsByLabel(nsId, labelSelector string) ([]vmIPLocation, error) {
	log.Debug().Str("nsId", nsId).Str("labelSelector", labelSelector).Msg("[DNS] Getting VM IP+Locations by label")

	if nsId != "" {
		labelSelector = model.LabelNamespace + "=" + nsId + "," + labelSelector
	}

	resources, err := label.GetResourcesByLabelSelector(model.StrVM, labelSelector)
	if err != nil {
		return nil, err
	}

	var locs []vmIPLocation
	for _, res := range resources {
		if vm, ok := res.(*model.VmInfo); ok {
			if vm.PublicIP != "" {
				locs = append(locs, vmIPLocation{
					PublicIP:   vm.PublicIP,
					Latitude:   vm.Location.Latitude,
					Longitude:  vm.Location.Longitude,
					Identifier: vm.Id,
				})
			}
		}
	}
	return locs, nil
}

type awsCreds struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

// getRoute53Client creates a Route53 client using OpenBao credentials (shared helper).
func getRoute53Client() (*route53.Client, error) {
	log.Debug().Str("vaultAddr", model.VaultAddr).Bool("vaultTokenSet", model.VaultToken != "").Msg("[DNS] Checking Vault credentials")
	if model.VaultToken == "" {
		log.Error().Msg("[DNS] VAULT_TOKEN is not set")
		return nil, fmt.Errorf("VAULT_TOKEN is not set")
	}

	awsCreds, err := fetchAWSCredsFromOpenBao(model.VaultAddr, model.VaultToken)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to fetch AWS credentials from OpenBao")
		return nil, fmt.Errorf("failed to fetch AWS credentials from OpenBao: %w", err)
	}
	log.Debug().Str("region", awsCreds.Region).Msg("[DNS] AWS credentials fetched successfully")

	ctx := context.Background()
	r53, err := newRoute53Client(ctx, awsCreds)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to create Route53 client")
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}
	log.Debug().Msg("[DNS] Route53 client created")
	return r53, nil
}

func fetchAWSCredsFromOpenBao(vaultAddr, vaultToken string) (*awsCreds, error) {
	log.Debug().Str("vaultAddr", vaultAddr).Msg("[DNS] Connecting to OpenBao")
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = vaultAddr
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to create OpenBao client")
		return nil, err
	}
	client.SetToken(vaultToken)

	log.Debug().Msg("[DNS] Reading secret at secret/data/csp/aws")
	secret, err := client.Logical().Read("secret/data/csp/aws")
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to read secret from OpenBao")
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		log.Error().Bool("secretNil", secret == nil).Msg("[DNS] Secret not found at secret/data/csp/aws")
		return nil, fmt.Errorf("secret not found at secret/data/csp/aws")
	}
	log.Debug().Msg("[DNS] Secret read successfully from OpenBao")

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		log.Error().Msg("[DNS] Invalid secret format: 'data' field missing or not a map")
		return nil, fmt.Errorf("invalid secret format: 'data' field missing or not a map")
	}

	keyID, _ := data["AWS_ACCESS_KEY_ID"].(string)
	secretKey, _ := data["AWS_SECRET_ACCESS_KEY"].(string)
	region, _ := data["AWS_DEFAULT_REGION"].(string)
	if region == "" {
		region = "us-east-1"
	}

	log.Debug().Bool("accessKeyIDSet", keyID != "").Bool("secretAccessKeySet", secretKey != "").Str("region", region).Msg("[DNS] Parsed AWS credentials from secret")

	if keyID == "" || secretKey == "" {
		log.Error().Bool("accessKeyIDEmpty", keyID == "").Bool("secretAccessKeyEmpty", secretKey == "").Msg("[DNS] AWS credentials incomplete")
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
	log.Debug().Str("lookup", lookup).Msg("[DNS] Searching hosted zones by name")
	out, err := r53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{DNSName: aws.String(lookup)})
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Failed to list hosted zones")
		return "", "", err
	}
	log.Debug().Int("zonesReturned", len(out.HostedZones)).Msg("[DNS] Hosted zones retrieved")
	for _, zone := range out.HostedZones {
		name := aws.ToString(zone.Name)
		log.Debug().Str("zoneName", name).Str("zoneId", aws.ToString(zone.Id)).Msg("[DNS] Checking zone")
		if strings.HasSuffix(lookup, strings.TrimSuffix(name, ".")+".") || lookup == name {
			log.Debug().Str("matchedZone", name).Str("zoneId", aws.ToString(zone.Id)).Msg("[DNS] Hosted zone matched")
			return aws.ToString(zone.Id), name, nil
		}
	}
	log.Warn().Str("domain", domain).Msg("[DNS] No hosted zone found")
	return "", "", fmt.Errorf("no hosted zone found for domain %q", domain)
}

func upsertGeoproximityRecords(ctx context.Context, r53 *route53.Client, zoneID, name, rtype string, ttl int64, vmLocs []vmIPLocation) error {
	if ttl == 0 {
		ttl = 300
	}

	// Group VMs by coordinates (same region VMs share one record)
	type coordKey struct{ lat, lng string }
	type coordGroup struct {
		lat, lng string
		ips      []string
	}
	groupOrder := []coordKey{}
	groups := map[coordKey]*coordGroup{}
	for _, vm := range vmLocs {
		lat := strconv.FormatFloat(vm.Latitude, 'f', 2, 64)
		lng := strconv.FormatFloat(vm.Longitude, 'f', 2, 64)
		key := coordKey{lat, lng}
		if g, ok := groups[key]; ok {
			g.ips = append(g.ips, vm.PublicIP)
		} else {
			groups[key] = &coordGroup{lat: lat, lng: lng, ips: []string{vm.PublicIP}}
			groupOrder = append(groupOrder, key)
		}
	}

	var changes []types.Change

	// Build set of new SetIdentifiers to detect stale records
	newSetIDs := make(map[string]bool)
	for i, key := range groupOrder {
		g := groups[key]
		setId := fmt.Sprintf("%s-geo-%d", name, i+1)
		newSetIDs[setId] = true
		var resRecords []types.ResourceRecord
		for _, ip := range g.ips {
			resRecords = append(resRecords, types.ResourceRecord{Value: aws.String(ip)})
		}

		log.Debug().Str("setId", setId).Strs("ips", g.ips).Str("lat", g.lat).Str("lng", g.lng).Msg("[DNS] Preparing geoproximity record")

		change := types.Change{
			Action: types.ChangeActionUpsert,
			ResourceRecordSet: &types.ResourceRecordSet{
				Name:          aws.String(name),
				Type:          types.RRType(rtype),
				TTL:           aws.Int64(ttl),
				SetIdentifier: aws.String(setId),
				GeoProximityLocation: &types.GeoProximityLocation{
					Coordinates: &types.Coordinates{
						Latitude:  aws.String(g.lat),
						Longitude: aws.String(g.lng),
					},
					Bias: aws.Int32(0),
				},
				ResourceRecords: resRecords,
			},
		}
		changes = append(changes, change)
	}

	// Delete stale geoproximity records from previous calls
	fqdn := name
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}
	listOut, listErr := r53.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(zoneID),
		StartRecordName: aws.String(name),
		StartRecordType: types.RRType(rtype),
	})
	if listErr == nil {
		for _, rs := range listOut.ResourceRecordSets {
			rsName := aws.ToString(rs.Name)
			if rsName != fqdn || rs.Type != types.RRType(rtype) {
				continue
			}
			sid := aws.ToString(rs.SetIdentifier)
			if rs.GeoProximityLocation != nil && sid != "" && !newSetIDs[sid] {
				log.Debug().Str("staleSetId", sid).Msg("[DNS] Deleting stale geoproximity record")
				changes = append(changes, types.Change{
					Action:            types.ChangeActionDelete,
					ResourceRecordSet: &rs,
				})
			}
		}
	} else {
		log.Warn().Err(listErr).Msg("[DNS] Failed to list existing records for stale cleanup; proceeding with upsert only")
	}

	log.Debug().Int("changeCount", len(changes)).Msg("[DNS] Sending geoproximity ChangeResourceRecordSets (UPSERT)")
	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}
	_, err := r53.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		log.Error().Err(err).Msg("[DNS] Geoproximity ChangeResourceRecordSets failed")
	} else {
		log.Debug().Msg("[DNS] Geoproximity ChangeResourceRecordSets succeeded")
	}
	return err
}

func upsertRoute53Record(ctx context.Context, r53 *route53.Client, zoneID, name, rtype string, ttl int64, values []string) error {
	if ttl == 0 {
		ttl = 300
	}
	var resRecords []types.ResourceRecord
	for _, v := range values {
		resRecords = append(resRecords, types.ResourceRecord{Value: aws.String(v)})
	}

	log.Debug().Str("zoneID", zoneID).Str("name", name).Str("type", rtype).Int64("ttl", ttl).Int("valueCount", len(values)).Msg("[DNS] Sending ChangeResourceRecordSets (UPSERT)")

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
	if err != nil {
		log.Error().Err(err).Msg("[DNS] ChangeResourceRecordSets failed")
	} else {
		log.Debug().Msg("[DNS] ChangeResourceRecordSets succeeded")
	}
	return err
}
