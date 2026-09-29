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

// Package infra is to manage multi-cloud infra
package infra

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// isPortAllowedByFirewallRules evaluates whether the specified port is allowed for inbound TCP traffic
// by any of the provided firewall rules.
func isPortAllowedByFirewallRules(rules []model.FirewallRuleInfo, targetPort int) bool {
	for _, r := range rules {
		// Only inbound rules apply
		if !strings.EqualFold(r.Direction, "inbound") && !strings.EqualFold(r.Direction, "in") {
			continue
		}

		// Protocol must be TCP or ALL
		proto := strings.ToUpper(strings.TrimSpace(r.Protocol))
		if proto != "TCP" && proto != "ALL" && proto != "" {
			continue
		}

		// ALL protocol with empty or "-1" port allows everything
		portStr := strings.TrimSpace(r.Port)
		if (proto == "ALL" || proto == "") && (portStr == "" || portStr == "-1" || strings.EqualFold(portStr, "ALL")) {
			return true
		}
		if portStr == "" || portStr == "-1" || strings.EqualFold(portStr, "ALL") {
			return true
		}

		// Handle comma-separated ports or ranges (e.g., "22,80,443", "20-80,443")
		ports := strings.Split(portStr, ",")
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if p == "" || p == "-1" || strings.EqualFold(p, "ALL") {
				return true
			}
			if strings.Contains(p, "-") {
				parts := strings.SplitN(p, "-", 2)
				from, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				to, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && targetPort >= from && targetPort <= to {
					return true
				}
			} else {
				portNum, err := strconv.Atoi(p)
				if err == nil && portNum == targetPort {
					return true
				}
			}
		}
	}
	return false
}

// checkNodeSshPortOpen checks if any of the Security Groups associated with the node allow inbound traffic to targetSshPort.
// If the node has no Security Groups registered or if fetching fails, it logs a warning and allows (fail-open)
// to prevent breaking unmanaged/legacy nodes.
// If Security Groups are found and NONE allow the port, it returns a descriptive error (fail-fast).
func checkNodeSshPortOpen(nsId, infraId, nodeId string, targetSshPort int, role string) error {
	nodeObj, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Warn().Err(err).Msgf("[SG Pre-check] Failed to get %s node %s/%s/%s, skipping pre-check", role, nsId, infraId, nodeId)
		return nil
	}

	if len(nodeObj.SecurityGroupIds) == 0 {
		log.Debug().Msgf("[SG Pre-check] No security groups registered for %s node %s, skipping pre-check", role, nodeId)
		return nil
	}

	var checkedSGs []string
	for _, sgId := range nodeObj.SecurityGroupIds {
		checkedSGs = append(checkedSGs, sgId)
		sgInfo, err := resource.GetSecurityGroup(nsId, sgId)
		if err != nil {
			log.Warn().Err(err).Msgf("[SG Pre-check] Failed to get security group %s in ns %s, skipping rule check for this SG", sgId, nsId)
			continue
		}
		if isPortAllowedByFirewallRules(sgInfo.FirewallRules, targetSshPort) {
			log.Debug().Msgf("[SG Pre-check] Inbound port %d is allowed by security group %s for %s node %s", targetSshPort, sgId, role, nodeId)
			return nil
		}
	}

	if len(checkedSGs) > 0 {
		return fmt.Errorf("security group pre-check failed: inbound port %d/TCP is not allowed in %s node %q security groups %v. Please open inbound port %d in security group rules",
			targetSshPort, role, nodeId, checkedSGs, targetSshPort)
	}

	return nil
}
