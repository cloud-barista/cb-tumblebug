/*
Copyright 2019 The Cloud-Barista Authors.
<!-- SPDX-License-Identifier: Apache-2.0 -->
*/

package infra

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// bastionMaxConcurrency is the maximum number of concurrent SSH connections
// allowed per bastion host. It matches the OpenSSH default MaxStartups value (10)
// so that parallel file transfers and remote commands do not exceed the bastion's
// built-in limit and trigger "unexpected packet in response to channel open" errors.
// Override with the TB_BASTION_MAX_CONCURRENCY environment variable.
var bastionMaxConcurrency = func() int {
	if v := os.Getenv("TB_BASTION_MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10 // matches OpenSSH default MaxStartups
}()

// bastionSemaphores holds one channel-based semaphore per bastion endpoint.
// Key: bastionEndpoint (host:port string), Value: chan struct{}
var bastionSemaphores sync.Map

// acquireBastionSlot acquires a concurrency slot for the given bastion endpoint.
// It creates the semaphore channel on first use. Call releaseBastionSlot when done.
func acquireBastionSlot(bastionEndpoint string) {
	sem, _ := bastionSemaphores.LoadOrStore(bastionEndpoint, make(chan struct{}, bastionMaxConcurrency))
	sem.(chan struct{}) <- struct{}{}
}

// releaseBastionSlot releases a previously acquired concurrency slot.
func releaseBastionSlot(bastionEndpoint string) {
	if sem, ok := bastionSemaphores.Load(bastionEndpoint); ok {
		<-sem.(chan struct{})
	}
}

// SetBastionNodes func sets bastion nodes
func SetBastionNodes(nsId string, infraId string, targetNodeId string, bastionNsId string, bastionInfraId string, bastionNodeId string) (string, error) {

	// Default bastionNsId/bastionInfraId to the target's values when not specified
	if bastionNsId == "" {
		bastionNsId = nsId
	}
	if bastionInfraId == "" {
		bastionInfraId = infraId
	}

	// Check if bastion node already exists for the target VM (for random assignment)
	currentBastion, err := GetBastionNodes(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if len(currentBastion) > 0 && bastionNodeId == "" {
		return "", fmt.Errorf("bastion node already exists for VM (ID: %s) in Infra (ID: %s) under namespace (ID: %s)",
			targetNodeId, infraId, nsId)
	}

	nodeObj, err := GetNodeObject(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	res, err := resource.GetResource(nsId, model.StrVNet, nodeObj.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}

	tempVNetInfo, ok := res.(model.VNetInfo)
	if !ok {
		log.Error().Err(err).Msg("")
		return "", err
	}

	// An explicitly named bastion is recorded as manual; an auto-selected one as auto.
	// pickBastion relies on this to prefer what an operator asked for.
	assignedBy := model.BastionAssignedManual
	if bastionNodeId == "" {
		assignedBy = model.BastionAssignedAuto
	}

	// find subnet and append bastion node
	for i, subnetInfo := range tempVNetInfo.SubnetInfoList {
		if subnetInfo.Id == nodeObj.SubnetId {

			if bastionNodeId == "" {
				// Auto-select: find a VM with a public IP.
				// For same-Infra, prefer VMs in the same subnet (original behaviour).
				// For cross-Infra/cross-NS, search all VMs in bastionNsId/bastionInfraId.
				isSameInfra := bastionNsId == nsId && bastionInfraId == infraId
				var candidateNodes []string
				var listErr error
				if isSameInfra {
					candidateNodes, listErr = ListNodeByFilter(nsId, infraId, "subnetId", nodeObj.SubnetId)
					if listErr != nil || len(candidateNodes) == 0 {
						// Fall back to all VMs in the Infra if no VM found in the subnet
						candidateNodes, listErr = ListNodeByFilter(nsId, infraId, "", "")
					}
				} else {
					candidateNodes, listErr = ListNodeByFilter(bastionNsId, bastionInfraId, "", "")
				}
				if listErr != nil {
					log.Error().Err(listErr).Msg("")
					return "", fmt.Errorf("failed to list VMs in Infra (ID: %s): %w", bastionInfraId, listErr)
				}

				// Find a Running VM with public IP to use as bastion — a stopped
				// VM cannot relay SSH even if it still holds a public IP.
				for _, v := range candidateNodes {
					candidateObj, err := GetNodeObject(bastionNsId, bastionInfraId, v)
					if err != nil || !strings.EqualFold(candidateObj.Status, model.StatusRunning) {
						continue
					}
					tmpPublicIp, _, _, err := GetNodeIp(bastionNsId, bastionInfraId, v)
					if err != nil {
						log.Error().Err(err).Msgf("failed to get IP for VM %s", v)
						continue
					}
					if tmpPublicIp != "" {
						bastionNodeId = v
						log.Info().Msgf("Selected VM %s in NS %s / Infra %s as bastion (public IP: %s)", v, bastionNsId, bastionInfraId, tmpPublicIp)
						break
					}
				}

				// If no suitable bastion VM found, return error
				if bastionNodeId == "" {
					return "", fmt.Errorf("no Running VM with public IP found in NS (ID: %s) Infra (ID: %s) to use as bastion", bastionNsId, bastionInfraId)
				}
			} else {
				// Validate that the specified bastion VM exists in bastionNsId/bastionInfraId
				_, err := GetNodeObject(bastionNsId, bastionInfraId, bastionNodeId)
				if err != nil {
					return "", fmt.Errorf("bastion VM (ID: %s) not found in NS (ID: %s) Infra (ID: %s): %w", bastionNodeId, bastionNsId, bastionInfraId, err)
				}

				// Duplicate check: normalize legacy BastionNode entries that have empty NsId
				// (they were stored before cross-namespace support was added and implicitly
				// belong to the target namespace).
				for _, existingNode := range subnetInfo.BastionNodes {
					if isSameBastion(existingNode, nsId, bastionNsId, bastionInfraId, bastionNodeId) {
						// The entry is already there, but registering it by hand still carries
						// intent: it promotes an auto entry to manual and retires the auto
						// entries it supersedes. Re-registering is how an operator repairs a
						// subnet whose stored list drifted, so this path cannot be a no-op.
						changed := false
						if assignedBy == model.BastionAssignedManual {
							pruned, dropped := pruneAutoBastions(subnetInfo.BastionNodes)
							for _, d := range dropped {
								if isSameBastion(d, nsId, bastionNsId, bastionInfraId, bastionNodeId) {
									continue // the entry being re-registered; it comes back as manual below
								}
								log.Info().Msgf("Removing auto-assigned bastion %s from subnet %s: bastion (NS: %s, Infra: %s, VM: %s) was registered by hand",
									d.NodeId, subnetInfo.Id, bastionNsId, bastionInfraId, bastionNodeId)
								changed = true
							}

							// Drop every copy of this bastion and re-add exactly one, so the
							// re-registration repairs duplicates rather than adding to them.
							kept := []model.BastionNode{}
							for _, k := range pruned {
								if isSameBastion(k, nsId, bastionNsId, bastionInfraId, bastionNodeId) {
									continue
								}
								kept = append(kept, k)
							}
							if len(kept) != len(pruned)-1 || existingNode.Assigned != model.BastionAssignedManual {
								changed = true // duplicates collapsed, or an auto/legacy entry promoted
							}
							kept = append(kept, model.BastionNode{NsId: bastionNsId, InfraId: bastionInfraId, NodeId: bastionNodeId, Assigned: model.BastionAssignedManual})
							subnetInfo.BastionNodes = kept
						}
						if changed {
							tempVNetInfo.SubnetInfoList[i] = subnetInfo
							resource.UpdateResourceObject(nsId, model.StrVNet, tempVNetInfo)
							return fmt.Sprintf("Bastion (NS: %s, Infra: %s, VM: %s) is now the manual bastion for subnet (ID: %s) in VNet (ID: %s); superseded auto-assigned entries were removed.",
								bastionNsId, bastionInfraId, bastionNodeId, subnetInfo.Id, nodeObj.VNetId), nil
						}
						return fmt.Sprintf("Bastion (NS: %s, Infra: %s, VM: %s) already exists in subnet (ID: %s) in VNet (ID: %s).",
							bastionNsId, bastionInfraId, bastionNodeId, subnetInfo.Id, nodeObj.VNetId), nil
					}
				}
			}

			// Registering a bastion by hand answers the question auto-assignment was
			// guessing at, so every auto entry in this subnet is now obsolete. They are
			// already inert - pickBastion prefers manual entries - but leaving them makes
			// the stored list, and anything drawn from it, disagree with how commands
			// actually route.
			//
			// Only entries explicitly marked auto are dropped. An entry predating the
			// Assigned field may well have been an operator's choice, and losing that
			// silently would be worse than leaving a redundant one behind.
			if assignedBy == model.BastionAssignedManual {
				kept, dropped := pruneAutoBastions(subnetInfo.BastionNodes)
				for _, d := range dropped {
					log.Info().Msgf("Removing auto-assigned bastion %s from subnet %s: bastion (NS: %s, Infra: %s, VM: %s) was registered by hand",
						d.NodeId, subnetInfo.Id, bastionNsId, bastionInfraId, bastionNodeId)
				}
				subnetInfo.BastionNodes = kept
			}

			bastionCandidate := model.BastionNode{NsId: bastionNsId, InfraId: bastionInfraId, NodeId: bastionNodeId, Assigned: assignedBy}
			subnetInfo.BastionNodes = append(subnetInfo.BastionNodes, bastionCandidate)
			tempVNetInfo.SubnetInfoList[i] = subnetInfo
			resource.UpdateResourceObject(nsId, model.StrVNet, tempVNetInfo)

			return fmt.Sprintf("Successfully set the bastion (NS: %s, Infra: %s, VM: %s) for subnet (ID: %s) in vNet (ID: %s) for VM (ID: %s) in Infra (ID: %s).",
				bastionNsId, bastionInfraId, bastionNodeId, subnetInfo.Id, nodeObj.VNetId, targetNodeId, infraId), nil
		}
	}
	return "", fmt.Errorf("failed to set bastion. Subnet (ID: %s) not found in VNet (ID: %s) for VM (ID: %s) in Infra (ID: %s) under namespace (ID: %s)",
		nodeObj.SubnetId, nodeObj.VNetId, targetNodeId, infraId, nsId)
}

// RemoveBastionNodes func removes existing bastion nodes info.
// bastionNsId and bastionInfraId narrow the match to a specific bastion identity;
// pass empty strings to match by bastionNodeId alone (legacy / cleanup on VM deletion).
func RemoveBastionNodes(nsId string, infraId string, bastionNsId string, bastionInfraId string, bastionNodeId string) (string, error) {
	resourceListInNs, err := resource.ListResource(nsId, model.StrVNet, "infraId", infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	} else {
		vNets := resourceListInNs.([]model.VNetInfo) // type assertion
		for _, vNet := range vNets {
			removed := false
			for i, subnet := range vNet.SubnetInfoList {
				for j := len(subnet.BastionNodes) - 1; j >= 0; j-- {
					node := subnet.BastionNodes[j]
					if node.NodeId != bastionNodeId {
						continue
					}
					// When bastionNsId/bastionInfraId are provided, also match on them
					// so that two bastions with the same NodeId but different Infras are
					// not accidentally conflated.
					if bastionInfraId != "" {
						effectiveNsId := node.NsId
						if effectiveNsId == "" {
							effectiveNsId = nsId
						}
						effectiveBastionNsId := bastionNsId
						if effectiveBastionNsId == "" {
							effectiveBastionNsId = nsId
						}
						if node.InfraId != bastionInfraId || effectiveNsId != effectiveBastionNsId {
							continue
						}
					}
					subnet.BastionNodes = append(subnet.BastionNodes[:j], subnet.BastionNodes[j+1:]...)
					removed = true
				}
				vNet.SubnetInfoList[i] = subnet
			}
			if removed {
				resource.UpdateResourceObject(nsId, model.StrVNet, vNet)
			}
		}
	}
	return fmt.Sprintf("Successfully removed the bastion (ID: %s) in Infra (ID: %s) from all subnets", bastionNodeId, infraId), nil
}

// GetBastionNodes func retrieves bastion nodes for a given VM
func GetBastionNodes(nsId string, infraId string, targetNodeId string) ([]model.BastionNode, error) {
	returnValue := []model.BastionNode{}
	// Fetch VM object based on nsId, infraId, and targetNodeId
	nodeObj, err := GetNodeObject(nsId, infraId, targetNodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Fetch VNet resource information
	res, err := resource.GetResource(nsId, model.StrVNet, nodeObj.VNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Type assertion for VNet information
	tempVNetInfo, ok := res.(model.VNetInfo)
	if !ok {
		log.Error().Err(err).Msg("")
		return returnValue, err
	}

	// Find the subnet corresponding to the VM and return the BastionNodeIds
	for _, subnetInfo := range tempVNetInfo.SubnetInfoList {
		if subnetInfo.Id == nodeObj.SubnetId {
			if subnetInfo.BastionNodes == nil {
				return returnValue, nil
			}
			returnValue = subnetInfo.BastionNodes
			return returnValue, nil
		}
	}

	return returnValue, fmt.Errorf("failed to get bastion in Subnet (ID: %s) of VNet (ID: %s) for VM (ID: %s)",
		nodeObj.SubnetId, nodeObj.VNetId, targetNodeId)
}

// GetUsableBastionNodes returns only usable bastion nodes (existing and
// Running) for the target VM, so callers can safely use the first entry.
// Every registered bastion is validated — not just the first — and stale
// ones (e.g. suspended after assignment; terminate already removes its
// registration) are dropped. When none survives, a fresh bastion is
// auto-selected; the re-read result is validated again, so an unusable
// entry can never be returned (e.g. when removal failed and the stale
// registration is still stored). Tolerates concurrent auto-assignment by
// re-reading after an assignment error.
func GetUsableBastionNodes(nsId string, infraId string, targetNodeId string) ([]model.BastionNode, error) {
	// filterUsable keeps Running bastions and removes stale registrations (best-effort).
	filterUsable := func(list []model.BastionNode) []model.BastionNode {
		usable := []model.BastionNode{}
		for _, b := range list {
			effNsId := b.NsId
			if effNsId == "" {
				effNsId = nsId
			}
			bObj, bErr := GetNodeObject(effNsId, b.InfraId, b.NodeId)
			if bErr == nil && strings.EqualFold(bObj.Status, model.StatusRunning) {
				usable = append(usable, b)
				continue
			}
			status := "not found"
			if bErr == nil {
				status = bObj.Status
			}
			log.Warn().Msgf("Bastion %s (NS: %s, Infra: %s) for VM %s is not usable (status: %s); dropping it",
				b.NodeId, effNsId, b.InfraId, targetNodeId, status)
			if _, rmErr := RemoveBastionNodes(nsId, infraId, b.NsId, b.InfraId, b.NodeId); rmErr != nil {
				log.Warn().Err(rmErr).Msg("failed to remove stale bastion registration")
			}
		}
		return usable
	}

	bastionNodes, err := GetBastionNodes(nsId, infraId, targetNodeId)
	if err != nil {
		return nil, err
	}
	usable := filterUsable(bastionNodes)
	if len(usable) > 0 {
		return usable, nil
	}

	// Auto-select a bastion. Concurrent callers may race here — one wins,
	// the others get an "already exists" error; re-read instead of failing.
	if _, asgErr := SetBastionNodes(nsId, infraId, targetNodeId, "", "", ""); asgErr != nil {
		log.Info().Err(asgErr).Msgf("bastion auto-assignment for VM %s returned error; re-checking (may have been assigned concurrently)", targetNodeId)
	}
	bastionNodes, err = GetBastionNodes(nsId, infraId, targetNodeId)
	if err != nil {
		return nil, err
	}
	usable = filterUsable(bastionNodes)
	if len(usable) == 0 {
		return nil, fmt.Errorf("no usable bastion node available for VM (ID: %s) in Infra (ID: %s)", targetNodeId, infraId)
	}
	return usable, nil
}

// pickBastion selects one bastion for a given target from a subnet's list of
// bastions, spreading load across bastions when an operator has registered
// more than one for the same subnet (manually and/or auto-assigned).
//
// It uses Rendezvous (Highest-Random-Weight) hashing keyed on the STABLE
// bastion NodeId — deliberately NOT `hash(target) % len(list)`. The bastion
// set is volatile: GetUsableBastionNodes re-checks Running state and drops
// stale registrations on every call, and operators add/remove bastions over
// time. With index-modulo, removing a single bastion (or a change in storage
// order) reshuffles almost every target's assignment. With HRW:
//   - selection depends only on the SET of present NodeIds, not their order;
//   - removing a bastion rehomes ONLY the targets that were on it (~1/K move
//     when one is added), so churn under a changing set is minimal;
//   - it is stateless/lock-free, so the concurrent fan-out needs no shared
//     counter, and the same target maps to the same bastion across the
//     command / upload / download call sites as long as that bastion stays
//     usable.
//
// Any bastion in the target's subnet can reach the target's private network,
// so if the chosen bastion later drops out, rehoming to the next-best one is
// functionally safe.
// Before hashing, the candidate set is narrowed, because a subnet can hold two
// kinds of registration that are NOT interchangeable:
//
//   - a MANUAL entry means an operator said "reach the target through here",
//     which only makes sense if the target is not directly reachable;
//   - an AUTO entry only means "this VM in the subnet has a public IP", and for a
//     single-VM Infra that VM is the target itself.
//
// Treating both as equal let the hash pick a self-entry over an operator's real
// bastion — intermittently, since it depends on the node ids. That is what broke
// SSH to an OpenStack VM whose floating IP (172.24.4.86, RFC1918 space) is only
// routable inside its own hypervisor host: the self-entry short-circuits to a
// direct dial, which can never work, while the registered host node could reach it.
//
// Narrowing rules:
//  1. If any manual entry exists, use only manual entries. This keeps the
//     self-bastion optimization for operators who deliberately register a
//     public-IP VM as its own subnet's bastion.
//  2. Otherwise (all auto, or entries predating the Assigned field), drop the
//     self-entry when some other bastion is registered. Routing through another
//     bastion always works — it costs at most one extra hop when the target was
//     directly reachable anyway — whereas a self-entry fails outright when it was
//     not. For un-annotated legacy data the safe choice is the correct one.
//
// ResolveSshUserName picks the SSH account for a node from what the SSH key resource
// carries, falling back to the platform default. It is the same order VerifySshUserName
// applies when it actually connects, so reported access info matches how commands are
// really delivered. VerifiedUsername is currently never written (the verification step
// is disabled), which is why the fallback carries the common case.
func ResolveSshUserName(verifiedUserName, userName string) string {
	if verifiedUserName != "" {
		return verifiedUserName
	}
	if userName != "" {
		return userName
	}
	return model.SshDefaultUserName[0] // cb-user
}

// isSameBastion reports whether a stored entry names the given bastion. A legacy entry
// with an empty NsId predates cross-namespace support and implicitly belongs to the
// target's namespace, so defaultNsId stands in for it.
func isSameBastion(entry model.BastionNode, defaultNsId, nsId, infraId, nodeId string) bool {
	entryNsId := entry.NsId
	if entryNsId == "" {
		entryNsId = defaultNsId
	}
	return entryNsId == nsId && entry.InfraId == infraId && entry.NodeId == nodeId
}

// pruneAutoBastions removes the auto-assigned entries from a subnet's bastion list,
// returning the entries to keep and the ones dropped. It is applied when an operator
// registers a bastion by hand: pickBastion would ignore the auto entries from then on,
// so keeping them only lets the stored list contradict how commands actually route.
//
// Entries predating the Assigned field are kept. One of those may itself have been an
// operator's choice, and discarding it silently is worse than a redundant entry.
func pruneAutoBastions(bastions []model.BastionNode) (kept []model.BastionNode, dropped []model.BastionNode) {
	for _, b := range bastions {
		if b.Assigned == model.BastionAssignedAuto {
			dropped = append(dropped, b)
			continue
		}
		kept = append(kept, b)
	}
	return kept, dropped
}

func pickBastion(bastions []model.BastionNode, nsId string, infraId string, targetNodeId string) model.BastionNode {
	isSelf := func(b model.BastionNode) bool {
		bNsId := b.NsId
		if bNsId == "" {
			bNsId = nsId
		}
		return bNsId == nsId && b.InfraId == infraId && b.NodeId == targetNodeId
	}

	candidates := []model.BastionNode{}
	for _, b := range bastions {
		if b.Assigned == model.BastionAssignedManual {
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		nonSelf := []model.BastionNode{}
		for _, b := range bastions {
			if !isSelf(b) {
				nonSelf = append(nonSelf, b)
			}
		}
		if len(nonSelf) > 0 {
			candidates = nonSelf
		} else {
			candidates = bastions
		}
	}
	if len(candidates) == 0 {
		return model.BastionNode{}
	}

	best := candidates[0]
	var bestScore uint64
	for i, b := range candidates {
		sum := sha256.Sum256([]byte(targetNodeId + "|" + b.NodeId))
		score := binary.BigEndian.Uint64(sum[:8])
		if i == 0 || score > bestScore {
			bestScore = score
			best = b
		}
	}
	return best
}
