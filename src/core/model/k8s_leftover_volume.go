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

// Package model is to handle object of CB-Tumblebug
package model

import "time"

/*
 * CB-Spider disk listing (GET /alldiskinfo) and CSP disk deletion (DELETE /cspdisk/{Id})
 */

// SpiderAllDiskInfoList is the CB-Spider response of 'GET /alldiskinfo'
type SpiderAllDiskInfoList struct {
	ResourceType string
	AllListInfo  struct {
		MappedInfoList  []SpiderDiskInfo
		OnlySpiderList  []IID
		OnlyCSPInfoList []SpiderDiskInfo
	}
}

/*
 * Leftover K8s volumes (CSI-created CSP disks left after their cluster is gone)
 */

// K8sLeftoverVolume is a CSP disk that looks created by a cluster's CSI driver (a PV-backed disk)
type K8sLeftoverVolume struct {
	// CspDiskId is the CSP disk ID (use it with DELETE .../k8sLeftoverVolume/{cspDiskId})
	CspDiskId string `json:"cspDiskId" example:"vol-0abc1234def567890"`
	// Name is the CSP disk name (NameId reported by CB-Spider)
	Name       string `json:"name,omitempty" example:"pvc-0f9e6a3c-2b1d-4c5e-8f7a-1234567890ab"`
	Zone       string `json:"zone,omitempty" example:"ap-northeast-2a"`
	DiskType   string `json:"diskType,omitempty" example:"gp3"`
	DiskSizeGB string `json:"diskSizeGB,omitempty" example:"10"`
	// Status is the CB-Spider disk status (Creating, Available, Attached, Deleting, Error)
	Status      string     `json:"status" example:"Available"`
	CreatedTime *time.Time `json:"createdTime,omitempty"`
	// MatchedBy lists the rules that classified the disk as CSI-created (e.g., name:pvc-uuid, tagKey:ebs.csi.aws.com/cluster)
	MatchedBy []string `json:"matchedBy" example:"tagKey:ebs.csi.aws.com/cluster"`
	// PvcName and PvcNamespace come from CSI tags when the CSP records them
	PvcName      string `json:"pvcName,omitempty" example:"data-postgres-0"`
	PvcNamespace string `json:"pvcNamespace,omitempty" example:"default"`
	// OwnerClusterHint is the owner cluster name or ID found in the tags (empty when the CSP records none)
	OwnerClusterHint string `json:"ownerClusterHint,omitempty" example:"k8scluster01-abc123"`
	// OwnerClusterAlive is true when the hint matches a K8sCluster currently known to CB-Tumblebug (any namespace)
	OwnerClusterAlive bool `json:"ownerClusterAlive"`
	// RegisteredInSpider is true when the disk is registered in CB-Spider (e.g., a TB dataDisk); such disks cannot be deleted by this API
	RegisteredInSpider bool `json:"registeredInSpider,omitempty"`
}

// K8sLeftoverVolumeListResponse is the response of GET /ns/{nsId}/k8sLeftoverVolume
type K8sLeftoverVolumeListResponse struct {
	ConnectionName  string              `json:"connectionName" example:"aws-ap-northeast-2"`
	LeftoverVolumes []K8sLeftoverVolume `json:"leftoverVolumes"`
	// Scanned is the number of CSP disks examined
	Scanned int `json:"scanned" example:"12"`
}
