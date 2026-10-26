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

// Package resource is to handle REST API for resource
package resource

import (
	"net/http"
	"strings"

	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func k8sStorageErrorResponse(c echo.Context, err error) error {
	code := resource.K8sStorageErrorCode(err)
	log.Warn().Err(err).Int("status", code).Msg("")
	return clientManager.EndRequestWithLogAndStatus(c, err, nil, code)
}

func splitK8sStorageQuery(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.Split(v, ",")
}

// RestGetAllK8sClusterStorageDriver godoc
// @ID GetAllK8sClusterStorageDriver
// @Summary List storage drivers of a K8sCluster
// @Description List the CSP-managed CSI storage drivers (Block and File) of a K8sCluster
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Success 200 {object} model.K8sStorageDriverListResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storageDriver [get]
func RestGetAllK8sClusterStorageDriver(c echo.Context) error {
	res, err := resource.ListK8sClusterStorageDrivers(c.Param("nsId"), c.Param("k8sClusterId"))
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestPutK8sClusterStorageDriver godoc
// @ID PutK8sClusterStorageDriver
// @Summary Enable a storage driver of a K8sCluster
// @Description Enable a CSP-managed CSI storage driver (Block or File) of a K8sCluster (idempotent; status may be "Updating", check with GET).
// @Description Usually not needed: PUT .../storage enables the drivers that the requested storage types need.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param storageDriverReq body model.K8sStorageDriverEnableReq true "Storage driver kind (Block or File) and optional version"
// @Success 200 {object} model.K8sStorageDriverInfo
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Failure 422 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storageDriver [put]
func RestPutK8sClusterStorageDriver(c echo.Context) error {
	req := &model.K8sStorageDriverEnableReq{}
	if err := c.Bind(req); err != nil {
		return clientManager.EndRequestWithLogAndStatus(c, err, nil, http.StatusBadRequest)
	}
	res, err := resource.EnableK8sClusterStorageDriver(c.Param("nsId"), c.Param("k8sClusterId"), req)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestDeleteK8sClusterStorageDriver godoc
// @ID DeleteK8sClusterStorageDriver
// @Summary Disable a storage driver of a K8sCluster
// @Description Disable a CSP-managed CSI storage driver (Block or File) of a K8sCluster.
// @Description WARNING: existing PersistentVolumes of this driver can no longer be attached or mounted (volumes and data are kept).
// @Description Requires option=force (400 otherwise). BuiltIn drivers cannot be disabled (400).
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param kind path string true "Storage driver kind" Enums(Block, File)
// @Param option query string true "Option for storage driver deletion (required to confirm)" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Failure 422 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storageDriver/{kind} [delete]
func RestDeleteK8sClusterStorageDriver(c echo.Context) error {
	k8sClusterId := c.Param("k8sClusterId")
	kind := c.Param("kind")
	force := c.QueryParam("option") == "force"
	if err := resource.DisableK8sClusterStorageDriver(c.Param("nsId"), k8sClusterId, kind, force); err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "The " + kind + " storage driver of K8sCluster(" + k8sClusterId + ") has been disabled"})
}

// RestPutK8sClusterStorage godoc
// @ID PutK8sClusterStorage
// @Summary Prepare abstract storage types on a K8sCluster (async)
// @Description Prepare abstract storage types (e.g., ssd-general, shared-file; see GET /k8sStorageInfo) on a K8sCluster:
// @Description enable the required storage drivers and, with fileSystem.create=true or fileSystem.name, prepare the file system a File driver needs.
// @Description Returns 202 immediately; poll GET .../storage for the result. 409 when a preparation is already running.
// @Description StorageClasses are not created; get them from GET .../storageClassManifest.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param storageReq body model.K8sClusterStorageReq false "Storage types to prepare (empty: [ssd-general]) and optional file system option"
// @Success 202 {object} model.K8sClusterStoragePrepareResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storage [put]
func RestPutK8sClusterStorage(c echo.Context) error {
	req := &model.K8sClusterStorageReq{}
	if c.Request().ContentLength != 0 {
		if err := c.Bind(req); err != nil {
			return clientManager.EndRequestWithLogAndStatus(c, err, nil, http.StatusBadRequest)
		}
	}
	res, err := resource.StartK8sClusterStoragePreparation(c.Param("nsId"), c.Param("k8sClusterId"), req)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLogAndStatus(c, nil, res, http.StatusAccepted)
}

// RestGetK8sClusterStorage godoc
// @ID GetK8sClusterStorage
// @Summary Get the storage state of a K8sCluster
// @Description Get the readiness (Ready / NotReady / Preparing) of the abstract storage types of a K8sCluster.
// @Description When no preparation is running, the driver status is refreshed from the CSP.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Success 200 {object} model.K8sClusterStorageStatusResponse
// @Failure 404 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storage [get]
func RestGetK8sClusterStorage(c echo.Context) error {
	res, err := resource.GetK8sClusterStorage(c.Param("nsId"), c.Param("k8sClusterId"))
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestPutK8sMultiClusterStorage godoc
// @ID PutK8sMultiClusterStorage
// @Summary Prepare abstract storage types on many K8sClusters (async)
// @Description Run PUT .../k8sCluster/{k8sClusterId}/storage on the listed K8sClusters (empty: all in the namespace) in parallel.
// @Description Returns 202 with a per-cluster result; with more than one cluster, fileSystem.name is ignored (use fileSystem.create=true).
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param storageReq body model.K8sMultiClusterStorageReq false "Target clusters and storage types"
// @Success 202 {object} model.K8sMultiClusterStoragePrepareResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sStorage [put]
func RestPutK8sMultiClusterStorage(c echo.Context) error {
	req := &model.K8sMultiClusterStorageReq{}
	if c.Request().ContentLength != 0 {
		if err := c.Bind(req); err != nil {
			return clientManager.EndRequestWithLogAndStatus(c, err, nil, http.StatusBadRequest)
		}
	}
	res, err := resource.StartK8sMultiClusterStoragePreparation(c.Param("nsId"), req)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLogAndStatus(c, nil, res, http.StatusAccepted)
}

// RestGetK8sMultiClusterStorage godoc
// @ID GetK8sMultiClusterStorage
// @Summary Get the storage state of many K8sClusters
// @Description Aggregate GET .../k8sCluster/{k8sClusterId}/storage over the K8sClusters of the namespace (done when summary.preparing is 0).
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterIds query string false "Comma-separated K8sCluster IDs (default: all)"
// @Success 200 {object} model.K8sMultiClusterStorageStatusResponse
// @Failure 400 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sStorage [get]
func RestGetK8sMultiClusterStorage(c echo.Context) error {
	res, err := resource.GetK8sMultiClusterStorage(c.Param("nsId"), splitK8sStorageQuery(c.QueryParam("k8sClusterIds")))
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestGetK8sClusterStorageClassManifest godoc
// @ID GetK8sClusterStorageClassManifest
// @Summary Generate StorageClass manifests for a K8sCluster
// @Description Generate the abstract StorageClasses (cb-<storageType>, e.g. cb-ssd-general) of a K8sCluster; apply them to the cluster.
// @Description Default response is a multi-document YAML; send "Accept: application/json" for a JSON Kubernetes List.
// @Description 412 when a storage type is not Ready (prepare it with PUT .../storage first); 422 when the catalog entry is not verified.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  application/yaml
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param k8sClusterId path string true "K8sCluster ID" default(k8scluster01)
// @Param storageTypes query string false "Comma-separated abstract storage types (default: ssd-general)" default(ssd-general)
// @Param setDefault query boolean false "Annotate the StorageClass as the cluster default" default(false)
// @Success 200 {object} model.K8sStorageClassManifestList
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 412 {object} model.SimpleMsg
// @Failure 422 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sCluster/{k8sClusterId}/storageClassManifest [get]
func RestGetK8sClusterStorageClassManifest(c echo.Context) error {
	setDefault := strings.EqualFold(c.QueryParam("setDefault"), "true")
	manifests, err := resource.GetK8sClusterStorageClassManifests(c.Param("nsId"), c.Param("k8sClusterId"), splitK8sStorageQuery(c.QueryParam("storageTypes")), setDefault)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	if strings.Contains(c.Request().Header.Get(echo.HeaderAccept), echo.MIMEApplicationJSON) {
		return clientManager.EndRequestWithLog(c, nil, resource.NewK8sStorageClassManifestList(manifests))
	}
	y, err := resource.MarshalK8sStorageClassManifestsYAML(manifests)
	if err != nil {
		return clientManager.EndRequestWithLogAndStatus(c, err, nil, http.StatusInternalServerError)
	}
	return c.Blob(http.StatusOK, "application/yaml", []byte(y))
}

// RestGetK8sMultiClusterStorageClassManifest godoc
// @ID GetK8sMultiClusterStorageClassManifest
// @Summary Generate StorageClass manifests for many K8sClusters
// @Description Generate the cb-<storageType> StorageClass YAML for each listed K8sCluster (default: all in the namespace).
// @Description Clusters whose storage types are not Ready or not verified are returned with status NotReady and a reason.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param storageTypes query string false "Comma-separated abstract storage types (default: ssd-general)" default(ssd-general)
// @Param k8sClusterIds query string false "Comma-separated K8sCluster IDs (default: all)"
// @Param setDefault query boolean false "Annotate the StorageClass as the cluster default" default(false)
// @Success 200 {object} model.K8sMultiClusterStorageClassManifestResponse
// @Failure 400 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sStorageClassManifest [get]
func RestGetK8sMultiClusterStorageClassManifest(c echo.Context) error {
	setDefault := strings.EqualFold(c.QueryParam("setDefault"), "true")
	res, err := resource.GetK8sMultiClusterStorageClassManifests(c.Param("nsId"),
		splitK8sStorageQuery(c.QueryParam("k8sClusterIds")), splitK8sStorageQuery(c.QueryParam("storageTypes")), setDefault)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestGetAllK8sLeftoverVolume godoc
// @ID GetAllK8sLeftoverVolume
// @Summary List leftover K8s volumes (CSI-created CSP disks)
// @Description List CSP disks of a connection that look created by a Kubernetes CSI driver (PV-backed disks), e.g. left after cluster deletion with reclaimPolicy Retain.
// @Description A disk matches by its name (pvc-<uuid>) or CSI tags (see matchedBy). Attached disks and disks of K8sClusters known to CB-Tumblebug are excluded by default.
// @Description Disks without an owner hint (GCP, Azure) may still belong to a cluster outside CB-Tumblebug: check before deleting.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param connectionName query string true "Connection name to scan" default(aws-ap-northeast-2)
// @Param includeAttached query boolean false "Include disks in Attached status" default(false)
// @Param includeAlive query boolean false "Include disks whose owner cluster is known to CB-Tumblebug" default(false)
// @Success 200 {object} model.K8sLeftoverVolumeListResponse
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sLeftoverVolume [get]
func RestGetAllK8sLeftoverVolume(c echo.Context) error {
	includeAttached := strings.EqualFold(c.QueryParam("includeAttached"), "true")
	includeAlive := strings.EqualFold(c.QueryParam("includeAlive"), "true")
	res, err := resource.ListK8sLeftoverVolumes(c.Param("nsId"), c.QueryParam("connectionName"), includeAttached, includeAlive)
	if err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, res)
}

// RestDeleteK8sLeftoverVolume godoc
// @ID DeleteK8sLeftoverVolume
// @Summary Delete a leftover K8s volume (CSI-created CSP disk)
// @Description Delete a CSP disk created by a Kubernetes CSI driver that was left after its cluster was deleted.
// @Description WARNING: the disk and all its data are deleted permanently. Requires option=force (400 otherwise).
// @Description 409 when the disk no longer looks CSI-created, is attached, its owner cluster is known to CB-Tumblebug, or it is a registered dataDisk.
// @Tags [Kubernetes] Storage Management
// @Accept  json
// @Produce  json
// @Param nsId path string true "Namespace ID" default(default)
// @Param cspDiskId path string true "CSP disk ID (cspDiskId from GET .../k8sLeftoverVolume)"
// @Param connectionName query string true "Connection name of the disk" default(aws-ap-northeast-2)
// @Param option query string true "Option for leftover volume deletion (required to confirm)" Enums(force)
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 409 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Failure 502 {object} model.SimpleMsg
// @Param x-request-id header string false "Custom request ID for tracking"
// @Router /ns/{nsId}/k8sLeftoverVolume/{cspDiskId} [delete]
func RestDeleteK8sLeftoverVolume(c echo.Context) error {
	cspDiskId := c.Param("cspDiskId")
	connectionName := c.QueryParam("connectionName")
	force := c.QueryParam("option") == "force"
	if err := resource.DeleteK8sLeftoverVolume(c.Param("nsId"), connectionName, cspDiskId, force); err != nil {
		return k8sStorageErrorResponse(c, err)
	}
	return clientManager.EndRequestWithLog(c, nil, model.SimpleMsg{Message: "The leftover K8s volume (disk " + cspDiskId + ") of connection(" + connectionName + ") has been deleted"})
}
