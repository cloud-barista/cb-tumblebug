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

package aws

// EC2 limits running On-Demand instances by vCPU, per region and per group of instance families.
// The limit comes from Service Quotas and the usage from the CloudWatch metric that the quota
// itself names. Both need permissions a credential may not have, so every failure here means
// "no quota information", never an error for the availability check.

import (
	"context"
	"strings"
	"sync"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/cloud-barista/cb-tumblebug/src/core/csp"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

const (
	awsQuotaTTL         = 5 * time.Minute
	awsQuotaCallTimeout = 10 * time.Second
)

// onDemandQuotaCode returns the Service Quotas code of the On-Demand vCPU limit an instance type
// counts against, or "" when it has none (for example Mac, which is limited by dedicated hosts)
func onDemandQuotaCode(instanceType string) string {
	family := strings.ToLower(strings.TrimSpace(instanceType))
	if i := strings.Index(family, "."); i > 0 {
		family = family[:i]
	}
	switch {
	case family == "":
		return ""
	case strings.HasPrefix(family, "mac"):
		return ""
	case strings.HasPrefix(family, "u-"), strings.HasPrefix(family, "u7"):
		return "L-43DA4232" // High Memory
	case strings.HasPrefix(family, "hpc"):
		return "L-F7808C92"
	case strings.HasPrefix(family, "dl"):
		return "L-6E869C2A"
	case strings.HasPrefix(family, "inf"):
		return "L-1945791B"
	case strings.HasPrefix(family, "trn"):
		return "L-2C3B7624"
	case strings.HasPrefix(family, "vt"), strings.HasPrefix(family, "g"):
		return "L-DB2E81BA" // G and VT
	case strings.HasPrefix(family, "p"):
		return "L-417A185B"
	case strings.HasPrefix(family, "f"):
		return "L-74FC7D96"
	case strings.HasPrefix(family, "x"):
		return "L-7295265B"
	case strings.ContainsRune("acdhimrtz", rune(family[0])):
		return "L-1216C47A" // Standard (A, C, D, H, I, M, R, T, Z)
	}
	return ""
}

type awsQuotaEntry struct {
	name      string
	limit     int64 // 0 when the quota could not be read
	used      int64 // -1 when usage could not be read
	fetchedAt time.Time
}

var (
	awsQuotaCache sync.Map // accessKey|region|code -> *awsQuotaEntry
	awsQuotaSF    singleflight.Group
)

// regionQuota reads one On-Demand vCPU quota of a region, cached so that every instance type of
// the same family group in the region shares one lookup
func regionQuota(ctx context.Context, region, accessKey, secretKey, code string) *awsQuotaEntry {
	key := accessKey + "|" + region + "|" + code
	if v, ok := awsQuotaCache.Load(key); ok {
		if e := v.(*awsQuotaEntry); time.Since(e.fetchedAt) < awsQuotaTTL {
			return e
		}
	}
	v, _, _ := awsQuotaSF.Do(key, func() (any, error) {
		e := fetchRegionQuota(ctx, region, accessKey, secretKey, code)
		awsQuotaCache.Store(key, e)
		return e, nil
	})
	return v.(*awsQuotaEntry)
}

func fetchRegionQuota(ctx context.Context, region, accessKey, secretKey, code string) *awsQuotaEntry {
	e := &awsQuotaEntry{used: -1, fetchedAt: time.Now()}
	cfg := newConfig(region, accessKey, secretKey)

	qctx, cancel := context.WithTimeout(ctx, awsQuotaCallTimeout)
	defer cancel()
	out, err := servicequotas.NewFromConfig(cfg).GetServiceQuota(qctx, &servicequotas.GetServiceQuotaInput{
		ServiceCode: awssdk.String("ec2"),
		QuotaCode:   awssdk.String(code),
	})
	if err != nil || out.Quota == nil || out.Quota.Value == nil {
		log.Debug().Err(err).Str("region", region).Str("quotaCode", code).Msg("[AWS] quota not readable; reporting none")
		return e
	}
	e.name = awssdk.ToString(out.Quota.QuotaName)
	e.limit = int64(*out.Quota.Value)

	m := out.Quota.UsageMetric
	if m == nil || m.MetricNamespace == nil || m.MetricName == nil {
		return e
	}
	dims := make([]cwtypes.Dimension, 0, len(m.MetricDimensions))
	for k, v := range m.MetricDimensions {
		dims = append(dims, cwtypes.Dimension{Name: awssdk.String(k), Value: awssdk.String(v)})
	}
	uctx, ucancel := context.WithTimeout(ctx, awsQuotaCallTimeout)
	defer ucancel()
	now := time.Now()
	stats, err := cloudwatch.NewFromConfig(cfg).GetMetricStatistics(uctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  m.MetricNamespace,
		MetricName: m.MetricName,
		Dimensions: dims,
		StartTime:  awssdk.Time(now.Add(-15 * time.Minute)),
		EndTime:    awssdk.Time(now),
		Period:     awssdk.Int32(300),
		Statistics: []cwtypes.Statistic{cwtypes.StatisticMaximum},
	})
	if err != nil {
		log.Debug().Err(err).Str("region", region).Str("quotaCode", code).Msg("[AWS] quota usage not readable; reporting the limit only")
		return e
	}
	// the usage metric is published only while something is running, so no datapoint means none in use
	e.used = 0
	var latest time.Time
	for _, dp := range stats.Datapoints {
		if dp.Timestamp != nil && dp.Maximum != nil && dp.Timestamp.After(latest) {
			latest = *dp.Timestamp
			e.used = int64(*dp.Maximum)
		}
	}
	return e
}

// instanceQuota returns the On-Demand vCPU quota for one instance type in a region, or nil when
// the type has no such quota, its vCPU count is unknown, or the credential cannot read it
func instanceQuota(ctx context.Context, region, accessKey, secretKey, instanceType string, vCPU int) *model.AvailabilityQuota {
	code := onDemandQuotaCode(instanceType)
	if code == "" || vCPU <= 0 {
		return nil
	}
	e := regionQuota(ctx, region, accessKey, secretKey, code)
	if c, ok := csp.NewQuota(e.name, "region", "vCPU", e.limit, e.used, int64(vCPU)); ok {
		return &c
	}
	return nil
}
