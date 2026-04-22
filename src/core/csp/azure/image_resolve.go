/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
*/

package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

// ResolveLatestUrn returns the URN of the latest available image version for
// the given Azure (publisher, offer, sku) tuple in the specified region.
// Returns the full URN "publisher:offer:sku:<latestVersion>" and the version
// string. The input version (if any) is irrelevant; this lookup always picks
// the highest-sorted version (Azure VirtualMachineImagesClient.List with
// "name desc", top=1).
//
// Returns an error if the SDK call fails or no version is available.
func ResolveLatestUrn(ctx context.Context, region, publisher, offer, sku string) (string, string, error) {
	region = strings.TrimSpace(region)
	publisher = strings.TrimSpace(publisher)
	offer = strings.TrimSpace(offer)
	sku = strings.TrimSpace(sku)
	if region == "" || publisher == "" || offer == "" || sku == "" {
		return "", "", fmt.Errorf("region/publisher/offer/sku are all required")
	}

	creds, err := getCreds(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get Azure credentials: %w", err)
	}
	credential, err := azidentity.NewClientSecretCredential(
		creds.TenantID, creds.ClientID, creds.ClientSecret, nil,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Azure credential: %w", err)
	}
	client, err := armcompute.NewVirtualMachineImagesClient(creds.SubscriptionID, credential, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create Azure VM image client: %w", err)
	}

	resp, err := callWithRetry(ctx, "ResolveLatestUrn", func() (armcompute.VirtualMachineImagesClientListResponse, error) {
		orderBy := "name desc"
		top := int32(1)
		return client.List(ctx, region, publisher, offer, sku, &armcompute.VirtualMachineImagesClientListOptions{
			Orderby: &orderBy,
			Top:     &top,
		})
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list versions for %s:%s:%s in %s: %w", publisher, offer, sku, region, err)
	}
	if len(resp.VirtualMachineImageResourceArray) == 0 {
		return "", "", fmt.Errorf("no versions returned for %s:%s:%s in %s", publisher, offer, sku, region)
	}
	latest := resp.VirtualMachineImageResourceArray[0]
	if latest == nil {
		return "", "", fmt.Errorf("nil version entry returned for %s:%s:%s in %s", publisher, offer, sku, region)
	}
	version := ""
	if latest.Name != nil {
		version = strings.TrimSpace(*latest.Name)
	}
	if version == "" && latest.ID != nil {
		parts := strings.Split(*latest.ID, "/")
		if len(parts) > 0 {
			version = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if version == "" {
		return "", "", fmt.Errorf("empty version returned for %s:%s:%s in %s", publisher, offer, sku, region)
	}

	return fmt.Sprintf("%s:%s:%s:%s", publisher, offer, sku, version), version, nil
}
