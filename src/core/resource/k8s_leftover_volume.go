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

// Leftover K8s volumes: CSP disks created by a cluster's CSI driver (PV-backed disks) that remain
// after the cluster is gone (reclaimPolicy Retain, or a cluster deleted without cleaning up).
// While the cluster lives, Kubernetes owns these disks; once it is gone, the platform may help
// find and delete them.
//
// Like the other storage APIs, this never calls the in-cluster Kubernetes API: it only reads the
// CSP disk list (name and tags) through CB-Spider and compares the owner hints with the
// K8sClusters known to CB-Tumblebug.

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/rs/zerolog/log"
)

// K8sLeftoverVolumeDeleteWarning is returned when DELETE k8sLeftoverVolume is called without option=force.
const K8sLeftoverVolumeDeleteWarning = "deleting a leftover K8s volume permanently deletes the CSP disk and all its data; this cannot be undone. Retry with option=force to proceed"

// k8sCsiDiskNamePattern is the PV name the CSI driver uses as the disk name (pvc-<PV UID>).
// Used by GCP, Azure and NHN.
var k8sCsiDiskNamePattern = regexp.MustCompile(`^pvc-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// k8sCsiDiskCompactNamePattern is the NCP NKS block CSI disk name: the PV UID without hyphens,
// truncated (e.g., pvc-0c48da33dbf94af8acb5aec151).
var k8sCsiDiskCompactNamePattern = regexp.MustCompile(`^pvc-[0-9a-f]{24,32}$`)

// k8sCsiDiskTagRule is one rule for reading CSI metadata from a CSP disk tag (TagList or KeyValueList).
type k8sCsiDiskTagRule struct {
	// key is matched against the tag key, case-insensitively
	key string
	// prefix: key is a prefix and the rest of the tag key is the value (AWS kubernetes.io/cluster/<name>)
	prefix bool
	// detect: a match classifies the disk as CSI-created
	detect bool
	// field: the tag value (or key suffix) gives this field: "pvcName", "pvcNamespace" or "cluster"
	field string
	// confirmed: verified against CSP or CSI driver documentation/source; otherwise assumed
	confirmed bool
}

// k8sCsiDiskTagRules is the single table of CSP-specific CSI disk tags.
// Detection keys are matched with "contains" (case-insensitive) so CSP-specific key mangling
// (e.g., Azure replaces '/' with '-') still matches; field keys are matched exactly (case-insensitive).
var k8sCsiDiskTagRules = []k8sCsiDiskTagRule{
	// AWS EBS CSI (--extra-create-metadata, default in the EKS add-on). Confirmed.
	{key: "kubernetes.io/created-for/pvc/name", detect: true, field: "pvcName", confirmed: true},
	{key: "kubernetes.io/created-for/pvc/namespace", field: "pvcNamespace", confirmed: true},
	// AWS EBS CSI always tags its volumes with ebs.csi.aws.com/cluster=true and CSIVolumeName=<PV name>. Confirmed.
	{key: "ebs.csi.aws.com/cluster", detect: true, confirmed: true},
	{key: "CSIVolumeName", detect: true, confirmed: true},
	// AWS EBS CSI with --k8s-tag-cluster-id (set by the EKS add-on): KubernetesCluster=<cluster> and
	// kubernetes.io/cluster/<cluster>=owned. Confirmed for the CSI driver; that EKS always sets it is assumed.
	{key: "KubernetesCluster", field: "cluster", confirmed: true},
	{key: "kubernetes.io/cluster/", prefix: true, field: "cluster", confirmed: true},
	// Azure Disk CSI: tags cannot contain '/', so the keys are written with '-'. Confirmed.
	{key: "kubernetes.io-created-for-pvc-name", detect: true, field: "pvcName", confirmed: true},
	{key: "kubernetes.io-created-for-pvc-namespace", field: "pvcNamespace", confirmed: true},
	{key: "k8s-azure-created-by", detect: true, confirmed: true},
	// OpenStack Cinder CSI (NHN): volume metadata. Assumed (whether CB-Spider exposes it is unverified).
	{key: "csi.storage.k8s.io/pvc/name", detect: true, field: "pvcName"},
	{key: "csi.storage.k8s.io/pvc/namespace", field: "pvcNamespace"},
	{key: "cinder.csi.openstack.org/cluster", detect: true, field: "cluster"},
	// Alibaba ACK disk CSI: ack.aliyun.com=<cluster ID>. Assumed.
	{key: "ack.aliyun.com", field: "cluster"},
	// GKE: cluster name label on PDs. Assumed.
	{key: "goog-k8s-cluster-name", field: "cluster"},
}

// k8sCsiDiskTagValueMarker classifies a disk when any tag value contains it (e.g., the GCP PD
// description JSON written by the GKE PD CSI driver). Assumed: CB-Spider may not expose the description.
const k8sCsiDiskTagValueMarker = "created-for/pvc"

// k8sCsiDiskMatch is the classification of one CSP disk.
type k8sCsiDiskMatch struct {
	matchedBy    []string
	pvcName      string
	pvcNamespace string
	clusterHint  string
}

func (m k8sCsiDiskMatch) isCsi() bool { return len(m.matchedBy) > 0 }

// matchK8sCsiDisk applies the name rule, the tag table and the value marker to a CSP disk.
// Name rule: confirmed for GCP and Azure (the disk is named after the PV); assumed for others.
func matchK8sCsiDisk(d model.SpiderDiskInfo) k8sCsiDiskMatch {
	m := k8sCsiDiskMatch{}
	if k8sCsiDiskNamePattern.MatchString(d.IId.NameId) || k8sCsiDiskNamePattern.MatchString(d.IId.SystemId) {
		m.matchedBy = append(m.matchedBy, "name:pvc-uuid")
	} else if k8sCsiDiskCompactNamePattern.MatchString(d.IId.NameId) {
		m.matchedBy = append(m.matchedBy, "name:pvc-compact")
	}
	seen := map[string]bool{}
	add := func(rule string) {
		if !seen[rule] {
			seen[rule] = true
			m.matchedBy = append(m.matchedBy, rule)
		}
	}
	tags := append(append([]model.KeyValue{}, d.TagList...), d.KeyValueList...)
	valueMarked := false
	for _, kv := range tags {
		key := strings.ToLower(strings.TrimSpace(kv.Key))
		for _, r := range k8sCsiDiskTagRules {
			rk := strings.ToLower(r.key)
			if r.prefix {
				if strings.HasPrefix(key, rk) && len(key) > len(rk) {
					if r.detect {
						add("tagKey:" + r.key)
					}
					if r.field == "cluster" && m.clusterHint == "" {
						m.clusterHint = strings.TrimSpace(kv.Key)[len(rk):]
					}
				}
				continue
			}
			if r.detect && strings.Contains(key, rk) {
				add("tagKey:" + r.key)
			}
			if r.field != "" && key == rk && kv.Value != "" {
				switch r.field {
				case "pvcName":
					if m.pvcName == "" {
						m.pvcName = kv.Value
					}
				case "pvcNamespace":
					if m.pvcNamespace == "" {
						m.pvcNamespace = kv.Value
					}
				case "cluster":
					if m.clusterHint == "" {
						m.clusterHint = kv.Value
					}
				}
			}
		}
		if strings.Contains(strings.ToLower(kv.Value), k8sCsiDiskTagValueMarker) {
			valueMarked = true
			// GCP PD description: {"kubernetes.io/created-for/pvc/name":"...","kubernetes.io/created-for/pvc/namespace":"..."}
			var desc map[string]string
			if json.Unmarshal([]byte(kv.Value), &desc) == nil {
				if m.pvcName == "" {
					m.pvcName = desc["kubernetes.io/created-for/pvc/name"]
				}
				if m.pvcNamespace == "" {
					m.pvcNamespace = desc["kubernetes.io/created-for/pvc/namespace"]
				}
			}
		}
	}
	if valueMarked {
		add("tagValue:" + k8sCsiDiskTagValueMarker)
	}
	return m
}

// k8sKnownClusterKeys returns the lowercase CSP names/IDs of all K8sClusters known to TB in any
// namespace (stored objects only; no CB-Spider calls). A package variable so tests can replace it.
var k8sKnownClusterKeys = func() (map[string]bool, error) {
	keys := map[string]bool{}
	nsIds, err := common.ListNsId()
	if err != nil {
		return nil, err
	}
	for _, nsId := range nsIds {
		ids, err := ListK8sClusterId(nsId)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if id == "" {
				continue
			}
			info, err := getK8sClusterInfo(nsId, id)
			if err != nil {
				log.Debug().Err(err).Msgf("Skipping K8sCluster(%s/%s)", nsId, id)
				continue
			}
			addK8sClusterKeys(keys, info)
		}
	}
	return keys, nil
}

func addK8sClusterKeys(keys map[string]bool, info *model.K8sClusterInfo) {
	for _, v := range []string{info.CspResourceName, info.CspResourceId} {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		keys[v] = true
		// ARNs and Azure resource IDs: also the last path segment (the CSP cluster name).
		if i := strings.LastIndex(v, "/"); i >= 0 && i < len(v)-1 {
			keys[v[i+1:]] = true
		}
	}
}

func isK8sOwnerClusterAlive(hint string, alive map[string]bool) bool {
	h := strings.ToLower(strings.TrimSpace(hint))
	return h != "" && alive[h]
}

func isSpiderDiskAttached(d model.SpiderDiskInfo) bool {
	return strings.EqualFold(string(d.Status), string(model.DiskAttached)) || d.OwnerNode.NameId != "" || d.OwnerNode.SystemId != ""
}

func toK8sLeftoverVolume(d model.SpiderDiskInfo, m k8sCsiDiskMatch, alive map[string]bool, registered bool) model.K8sLeftoverVolume {
	v := model.K8sLeftoverVolume{
		CspDiskId:          d.IId.SystemId,
		Name:               d.IId.NameId,
		Zone:               d.Zone,
		DiskType:           d.DiskType,
		DiskSizeGB:         d.DiskSize,
		Status:             string(d.Status),
		MatchedBy:          m.matchedBy,
		PvcName:            m.pvcName,
		PvcNamespace:       m.pvcNamespace,
		OwnerClusterHint:   m.clusterHint,
		OwnerClusterAlive:  isK8sOwnerClusterAlive(m.clusterHint, alive),
		RegisteredInSpider: registered,
	}
	if v.CspDiskId == "" {
		v.CspDiskId = d.IId.NameId
	}
	if !d.CreatedTime.IsZero() {
		t := d.CreatedTime
		v.CreatedTime = &t
	}
	return v
}

// scanK8sLeftoverVolumes lists the CSI-created disks of a connection with the given filters.
func scanK8sLeftoverVolumes(api k8sStorageSpiderAPI, connectionName string, alive map[string]bool, includeAttached, includeAlive bool) (*model.K8sLeftoverVolumeListResponse, error) {
	all, err := api.ListAllDisks(connectionName)
	if err != nil {
		return nil, spiderK8sStorageError(err, "failed to list disks of connection(%s) from CB-Spider", connectionName)
	}
	res := &model.K8sLeftoverVolumeListResponse{ConnectionName: connectionName, LeftoverVolumes: []model.K8sLeftoverVolume{}}
	scan := func(list []model.SpiderDiskInfo, registered bool) {
		for _, d := range list {
			res.Scanned++
			m := matchK8sCsiDisk(d)
			if !m.isCsi() {
				continue
			}
			v := toK8sLeftoverVolume(d, m, alive, registered)
			if !includeAttached && isSpiderDiskAttached(d) {
				continue
			}
			if !includeAlive && v.OwnerClusterAlive {
				continue
			}
			res.LeftoverVolumes = append(res.LeftoverVolumes, v)
		}
	}
	scan(all.AllListInfo.OnlyCSPInfoList, false)
	scan(all.AllListInfo.MappedInfoList, true)
	return res, nil
}

// ListK8sLeftoverVolumes lists CSP disks of a connection that look created by a K8s CSI driver,
// excluding (unless requested) attached disks and disks whose owner cluster is still known to TB.
func ListK8sLeftoverVolumes(nsId, connectionName string, includeAttached, includeAlive bool) (*model.K8sLeftoverVolumeListResponse, error) {
	if err := common.CheckString(nsId); err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if strings.TrimSpace(connectionName) == "" {
		return nil, newK8sStorageError(http.StatusBadRequest, "connectionName is required")
	}
	alive, err := k8sKnownClusterKeys()
	if err != nil {
		return nil, newK8sStorageError(http.StatusInternalServerError, "failed to list K8sClusters known to CB-Tumblebug: %v", err)
	}
	return scanK8sLeftoverVolumes(k8sStorageSpider, connectionName, alive, includeAttached, includeAlive)
}

// deleteK8sLeftoverVolume re-checks the disk and deletes it through CB-Spider.
// 404: disk not found; 409: not CSI-created, attached, owner cluster alive, or registered in CB-Spider.
func deleteK8sLeftoverVolume(api k8sStorageSpiderAPI, connectionName, cspDiskId string, alive map[string]bool) error {
	all, err := api.ListAllDisks(connectionName)
	if err != nil {
		return spiderK8sStorageError(err, "failed to list disks of connection(%s) from CB-Spider", connectionName)
	}
	var disk *model.SpiderDiskInfo
	registered := false
	find := func(list []model.SpiderDiskInfo, reg bool) {
		for i := range list {
			if disk == nil && (list[i].IId.SystemId == cspDiskId || (list[i].IId.SystemId == "" && list[i].IId.NameId == cspDiskId)) {
				disk, registered = &list[i], reg
			}
		}
	}
	find(all.AllListInfo.OnlyCSPInfoList, false)
	find(all.AllListInfo.MappedInfoList, true)
	if disk == nil {
		return newK8sStorageError(http.StatusNotFound, "disk(%s) not found in connection(%s)", cspDiskId, connectionName)
	}
	m := matchK8sCsiDisk(*disk)
	switch {
	case !m.isCsi():
		return newK8sStorageError(http.StatusConflict, "disk(%s) does not look created by a K8s CSI driver; use the dataDisk API", cspDiskId)
	case registered:
		return newK8sStorageError(http.StatusConflict, "disk(%s) is registered in CB-Spider; delete it through the dataDisk API", cspDiskId)
	case isSpiderDiskAttached(*disk):
		return newK8sStorageError(http.StatusConflict, "disk(%s) is attached (status: %s, ownerVM: %s); it may still be in use", cspDiskId, disk.Status, disk.OwnerNode.NameId+disk.OwnerNode.SystemId)
	case isK8sOwnerClusterAlive(m.clusterHint, alive):
		return newK8sStorageError(http.StatusConflict, "the owner cluster(%s) of disk(%s) is still known to CB-Tumblebug; Kubernetes owns this disk, delete its PVC/PV instead", m.clusterHint, cspDiskId)
	}
	if err := api.DeleteCSPDisk(cspDiskId, connectionName); err != nil {
		return spiderK8sStorageError(err, "failed to delete disk(%s) through CB-Spider", cspDiskId)
	}
	log.Info().Msgf("Deleted leftover K8s volume disk(%s) of connection(%s) (matchedBy: %v, pvc: %s/%s)", cspDiskId, connectionName, m.matchedBy, m.pvcNamespace, m.pvcName)
	return nil
}

// DeleteK8sLeftoverVolume deletes a leftover CSI-created CSP disk. Requires force (option=force; 400 otherwise).
func DeleteK8sLeftoverVolume(nsId, connectionName, cspDiskId string, force bool) error {
	if err := common.CheckString(nsId); err != nil {
		return newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if strings.TrimSpace(connectionName) == "" {
		return newK8sStorageError(http.StatusBadRequest, "connectionName is required")
	}
	if strings.TrimSpace(cspDiskId) == "" {
		return newK8sStorageError(http.StatusBadRequest, "cspDiskId is required")
	}
	if !force {
		return newK8sStorageError(http.StatusBadRequest, "WARNING: %s", K8sLeftoverVolumeDeleteWarning)
	}
	alive, err := k8sKnownClusterKeys()
	if err != nil {
		return newK8sStorageError(http.StatusInternalServerError, "failed to list K8sClusters known to CB-Tumblebug: %v", err)
	}
	return deleteK8sLeftoverVolume(k8sStorageSpider, connectionName, cspDiskId, alive)
}
