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
	"regexp"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// parsePingStats extracts packet loss and RTT statistics from ping output.
func parsePingStats(output string) model.VpnPingStats {
	stats := model.VpnPingStats{}

	// Match packet loss: "3 packets transmitted, 3 received, 0% packet loss"
	lossRe := regexp.MustCompile(`(\d+(?:\.\d+)?%)\s+packet loss`)
	if m := lossRe.FindStringSubmatch(output); len(m) > 1 {
		stats.PacketLoss = m[1]
	}

	// Match RTT: "rtt min/avg/max/mdev = 1.234/2.345/3.456/0.123 ms"
	rttRe := regexp.MustCompile(`rtt min/avg/max/mdev\s*=\s*([\d.]+)/([\d.]+)/([\d.]+)/([\d.]+)\s*(\w+)`)
	if m := rttRe.FindStringSubmatch(output); len(m) > 5 {
		unit := m[5]
		stats.MinRtt = m[1] + " " + unit
		stats.AvgRtt = m[2] + " " + unit
		stats.MaxRtt = m[3] + " " + unit
	}

	return stats
}

// runPingCheck runs a ping test from sourceNode to targetNode with retry logic.
// Returns a VpnPingDirectionResult with parsed statistics.
func runPingCheck(nsId, infraId, direction string, sourceNode, targetNode *model.NodeInfo,
	userName string, pingCount, intervalSec, maxAttempts int) model.VpnPingDirectionResult {

	result := model.VpnPingDirectionResult{
		Direction: direction,
		SourceNode: model.VpnHealthCheckSourceNodeInfo{
			NodeId:    sourceNode.Id,
			PrivateIP: sourceNode.PrivateIP,
			CSP:       sourceNode.ConnectionConfig.ProviderName,
		},
		TargetNode: model.VpnHealthCheckTargetNodeInfo{
			NodeId:    targetNode.Id,
			PrivateIP: targetNode.PrivateIP,
			CSP:       targetNode.ConnectionConfig.ProviderName,
		},
	}

	cmdReq := &model.InfraCmdReq{
		UserName:       userName,
		Command:        []string{fmt.Sprintf("ping %s -c %d", targetNode.PrivateIP, pingCount)},
		TimeoutMinutes: 5,
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Info().Msgf("[VPN Health Check] [%s] Ping attempt %d/%d from %s (%s) to %s (%s)",
			direction, attempt, maxAttempts, sourceNode.Id, sourceNode.PrivateIP, targetNode.Id, targetNode.PrivateIP)

		results, err := RemoteCommandToInfra(nsId, infraId, "", sourceNode.Id, "", cmdReq, "")

		if err == nil && len(results) > 0 {
			stdout := results[0].Stdout[0]
			if strings.Contains(stdout, "bytes from") {
				result.Reachable = true
				result.Attempts = attempt
				result.PingStats = parsePingStats(stdout)
				result.Message = fmt.Sprintf("Ping succeeded on attempt %d/%d", attempt, maxAttempts)
				log.Info().Msgf("[VPN Health Check] [%s] %s", direction, result.Message)
				return result
			}
		}

		if attempt < maxAttempts {
			log.Warn().Msgf("[VPN Health Check] [%s] Ping attempt %d/%d failed, retrying in %ds...", direction, attempt, maxAttempts, intervalSec)
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}

	result.Reachable = false
	result.Attempts = maxAttempts
	result.Message = fmt.Sprintf("Ping failed after %d attempts", maxAttempts)
	log.Warn().Msgf("[VPN Health Check] [%s] %s", direction, result.Message)
	return result
}

// CheckVpnHealth performs a bidirectional ping-based health check on a site-to-site VPN
// by finding Nodes in the Infra that match the VPN's two sites and running ping tests in both directions.
func CheckVpnHealth(ctx context.Context, nsId, infraId, vpnId string, req *model.VpnHealthCheckRequest) (model.VpnHealthCheckResponse, error) {

	var resp model.VpnHealthCheckResponse

	// Get VPN info to find the two sites' connection names
	vpnInfo, err := resource.GetSiteToSiteVPN(ctx, nsId, infraId, vpnId, "refined", false)
	if err != nil {
		return resp, fmt.Errorf("VPN not found: %w", err)
	}
	if len(vpnInfo.VpnSites) < 2 {
		return resp, fmt.Errorf("VPN does not have two sites")
	}

	// Get Infra info to find Nodes matching VPN sites
	infraInfo, err := GetInfraInfo(nsId, infraId)
	if err != nil {
		return resp, fmt.Errorf("Infra not found: %w", err)
	}

	// Find Nodes matching each VPN site by ConnectionName
	site1ConnName := vpnInfo.VpnSites[0].ConnectionName
	site2ConnName := vpnInfo.VpnSites[1].ConnectionName

	var site1Node, site2Node *model.NodeInfo
	for i := range infraInfo.Node {
		vm := &infraInfo.Node[i]
		if vm.ConnectionName == site1ConnName && site1Node == nil {
			site1Node = vm
		} else if vm.ConnectionName == site2ConnName && site2Node == nil {
			site2Node = vm
		}
		if site1Node != nil && site2Node != nil {
			break
		}
	}

	if site1Node == nil || site2Node == nil {
		return resp, fmt.Errorf("could not find Nodes matching VPN sites (site1: %s, site2: %s) in Infra %s", site1ConnName, site2ConnName, infraId)
	}

	// Get effective values
	pingCount, intervalSec, maxAttempts := req.GetEffectiveValues()

	userName := req.UserName
	if userName == "" {
		userName = "cb-user"
	}

	resp.VpnId = vpnId

	// Direction 1: site1 → site2
	log.Info().Msgf("[VPN Health Check] Starting site1→site2 ping test")
	result1 := runPingCheck(nsId, infraId, "site1→site2", site1Node, site2Node, userName, pingCount, intervalSec, maxAttempts)

	// Direction 2: site2 → site1
	log.Info().Msgf("[VPN Health Check] Starting site2→site1 ping test")
	result2 := runPingCheck(nsId, infraId, "site2→site1", site2Node, site1Node, userName, pingCount, intervalSec, maxAttempts)

	resp.Results = []model.VpnPingDirectionResult{result1, result2}
	resp.Reachable = result1.Reachable && result2.Reachable

	if resp.Reachable {
		resp.Message = "Bidirectional VPN health check succeeded"
	} else if result1.Reachable || result2.Reachable {
		resp.Message = "VPN health check partially succeeded (one direction failed)"
	} else {
		resp.Message = "Bidirectional VPN health check failed"
	}

	return resp, nil
}
