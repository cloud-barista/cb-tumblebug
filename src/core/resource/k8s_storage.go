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

// K8s storage deploy & control (abstract storage types on managed K8s clusters).
//
// TB never calls the in-cluster Kubernetes API here: no kubeconfig, client-go, exec or Helm.
// Everything goes through CB-Spider REST (storagedriver, cluster, filesystem). Cluster-internal
// objects (StorageClasses) are handed to the user as manifests only.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// Timing of the asynchronous storage preparation (package variables so tests can shorten them).
var (
	k8sStorageDriverPollInterval  = 15 * time.Second
	k8sStorageDriverPollTimeout   = 20 * time.Minute
	k8sStorageDriverFailedGrace   = 10 * time.Minute // an EKS add-on can stay DEGRADED for minutes after its nodes join
	k8sStorageClusterPollInterval = 30 * time.Second
	k8sStorageClusterPollTimeout  = 60 * time.Minute
)

/*
 * Errors with HTTP status
 */

// K8sStorageError is an error with the HTTP status the REST layer should return.
type K8sStorageError struct {
	StatusCode int
	Err        error
}

func (e *K8sStorageError) Error() string { return e.Err.Error() }
func (e *K8sStorageError) Unwrap() error { return e.Err }

func newK8sStorageError(code int, format string, args ...any) error {
	return &K8sStorageError{StatusCode: code, Err: fmt.Errorf(format, args...)}
}

// K8sStorageErrorCode returns the HTTP status of a storage API error (500 when unknown).
func K8sStorageErrorCode(err error) int {
	var se *K8sStorageError
	if errors.As(err, &se) {
		return se.StatusCode
	}
	return http.StatusInternalServerError
}

// spiderK8sStorageError classifies a CB-Spider failure: NotImplemented/NotSupported → 422, otherwise 502.
func spiderK8sStorageError(err error, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "not implemented") || strings.Contains(lower, "notimplemented") || strings.Contains(lower, "not supported") || strings.Contains(lower, "notsupported") {
		return &K8sStorageError{StatusCode: http.StatusUnprocessableEntity, Err: fmt.Errorf("%s: %w", msg, err)}
	}
	return &K8sStorageError{StatusCode: http.StatusBadGateway, Err: fmt.Errorf("%s: %w", msg, err)}
}

/*
 * CB-Spider client
 */

// k8sStorageSpiderAPI is the subset of CB-Spider REST used by the storage APIs.
type k8sStorageSpiderAPI interface {
	ListStorageDrivers(cspClusterName, connectionName string) ([]model.SpiderStorageDriverInfo, error)
	EnableStorageDriver(cspClusterName, connectionName, kind, version string) (model.SpiderStorageDriverInfo, error)
	DisableStorageDriver(cspClusterName, connectionName, kind string) error
	GetCluster(cspClusterName, connectionName string) (model.SpiderClusterInfo, error)
	CreateFileSystem(req model.SpiderFileSystemCreateReq) (model.SpiderFileSystemInfo, error)
	GetFileSystem(name, connectionName string) (model.SpiderFileSystemInfo, error)
	AddFileSystemAccessSubnet(name, connectionName string, subnet model.IID) (model.SpiderFileSystemInfo, error)
	DeleteFileSystem(name, connectionName string) error
	ListAllDisks(connectionName string) (model.SpiderAllDiskInfoList, error)
	DeleteCSPDisk(cspDiskId, connectionName string) error
}

// k8sStorageSpider is the CB-Spider client used by the storage APIs (replaced by a fake in tests).
var k8sStorageSpider k8sStorageSpiderAPI = spiderK8sStorageClient{}

type spiderK8sStorageClient struct{}

func spiderCall[B any, T any](method, reqUrl string, body B, useBody bool, result *T, timeout time.Duration) error {
	client := clientManager.NewHttpClient()
	client.SetTimeout(timeout)
	resp, err := clientManager.ExecuteHttpRequest(client, method, reqUrl, nil, useBody, &body, result, 0)
	return clientManager.HandleHttpResponse(resp, err)
}

type spiderConnectionBody struct {
	ConnectionName string
}

func (spiderK8sStorageClient) ListStorageDrivers(cspClusterName, connectionName string) ([]model.SpiderStorageDriverInfo, error) {
	u := fmt.Sprintf("%s/cluster/%s/storagedriver?ConnectionName=%s", model.SpiderRestUrl, url.PathEscape(cspClusterName), url.QueryEscape(connectionName))
	var res model.SpiderStorageDriverListRes
	if err := spiderCall("GET", u, clientManager.NoBody, false, &res, 2*time.Minute); err != nil {
		return nil, err
	}
	return res.Result, nil
}

func (spiderK8sStorageClient) EnableStorageDriver(cspClusterName, connectionName, kind, version string) (model.SpiderStorageDriverInfo, error) {
	u := fmt.Sprintf("%s/cluster/%s/storagedriver", model.SpiderRestUrl, url.PathEscape(cspClusterName))
	body := model.SpiderStorageDriverEnableReq{ConnectionName: connectionName, ReqInfo: model.SpiderStorageDriverEnableReqInfo{Kind: kind, Version: version}}
	var res model.SpiderStorageDriverInfo
	err := spiderCall("PUT", u, body, true, &res, 30*time.Minute)
	return res, err
}

func (spiderK8sStorageClient) DisableStorageDriver(cspClusterName, connectionName, kind string) error {
	u := fmt.Sprintf("%s/cluster/%s/storagedriver/%s", model.SpiderRestUrl, url.PathEscape(cspClusterName), url.PathEscape(kind))
	var res struct{ Result string }
	if err := spiderCall("DELETE", u, spiderConnectionBody{ConnectionName: connectionName}, true, &res, 30*time.Minute); err != nil {
		return err
	}
	if !strings.EqualFold(res.Result, "true") {
		return fmt.Errorf("CB-Spider returned Result=%q", res.Result)
	}
	return nil
}

func (spiderK8sStorageClient) GetCluster(cspClusterName, connectionName string) (model.SpiderClusterInfo, error) {
	u := fmt.Sprintf("%s/cluster/%s", model.SpiderRestUrl, url.PathEscape(cspClusterName))
	var res model.SpiderClusterRes
	err := spiderCall("GET", u, spiderConnectionBody{ConnectionName: connectionName}, true, &res, 10*time.Minute)
	return res.SpiderClusterInfo, err
}

func (spiderK8sStorageClient) CreateFileSystem(req model.SpiderFileSystemCreateReq) (model.SpiderFileSystemInfo, error) {
	var res model.SpiderFileSystemInfo
	err := spiderCall("POST", model.SpiderRestUrl+"/filesystem", req, true, &res, 30*time.Minute)
	return res, err
}

func (spiderK8sStorageClient) GetFileSystem(name, connectionName string) (model.SpiderFileSystemInfo, error) {
	u := fmt.Sprintf("%s/filesystem/%s?ConnectionName=%s", model.SpiderRestUrl, url.PathEscape(name), url.QueryEscape(connectionName))
	var res model.SpiderFileSystemInfo
	err := spiderCall("GET", u, clientManager.NoBody, false, &res, 2*time.Minute)
	return res, err
}

func (spiderK8sStorageClient) AddFileSystemAccessSubnet(name, connectionName string, subnet model.IID) (model.SpiderFileSystemInfo, error) {
	u := fmt.Sprintf("%s/filesystem/%s/accesssubnet", model.SpiderRestUrl, url.PathEscape(name))
	body := model.SpiderFileSystemAccessSubnetReq{ConnectionName: connectionName, SubnetIID: model.IID{NameId: subnet.NameId}}
	var res model.SpiderFileSystemInfo
	err := spiderCall("POST", u, body, true, &res, 30*time.Minute)
	return res, err
}

func (spiderK8sStorageClient) DeleteFileSystem(name, connectionName string) error {
	u := fmt.Sprintf("%s/filesystem/%s", model.SpiderRestUrl, url.PathEscape(name))
	var res model.SpiderBooleanInfo
	if err := spiderCall("DELETE", u, spiderConnectionBody{ConnectionName: connectionName}, true, &res, 10*time.Minute); err != nil {
		return err
	}
	if !strings.EqualFold(res.Result, "true") {
		return fmt.Errorf("CB-Spider returned Result=%q", res.Result)
	}
	return nil
}

func (spiderK8sStorageClient) ListAllDisks(connectionName string) (model.SpiderAllDiskInfoList, error) {
	u := fmt.Sprintf("%s/alldiskinfo?ConnectionName=%s", model.SpiderRestUrl, url.QueryEscape(connectionName))
	var res model.SpiderAllDiskInfoList
	err := spiderCall("GET", u, clientManager.NoBody, false, &res, 5*time.Minute)
	return res, err
}

func (spiderK8sStorageClient) DeleteCSPDisk(cspDiskId, connectionName string) error {
	u := fmt.Sprintf("%s/cspdisk/%s", model.SpiderRestUrl, url.PathEscape(cspDiskId))
	var res model.SpiderBooleanInfo
	if err := spiderCall("DELETE", u, spiderConnectionBody{ConnectionName: connectionName}, true, &res, 10*time.Minute); err != nil {
		return err
	}
	if !strings.EqualFold(res.Result, "true") {
		return fmt.Errorf("CB-Spider returned Result=%q", res.Result)
	}
	return nil
}

func toK8sStorageDriverInfo(d model.SpiderStorageDriverInfo) model.K8sStorageDriverInfo {
	return model.K8sStorageDriverInfo{
		Kind:               d.Kind,
		CspDriverName:      d.CSPDriverName,
		CsiDriverName:      d.CSIDriverName,
		ManagementMode:     d.ManagementMode,
		Status:             d.Status,
		Version:            d.Version,
		RequiresFileSystem: d.RequiresFileSystem,
		KeyValueList:       d.KeyValueList,
	}
}

func toK8sStorageDriverRef(d model.SpiderStorageDriverInfo) *model.K8sStorageDriverRef {
	return &model.K8sStorageDriverRef{Kind: d.Kind, CsiDriverName: d.CSIDriverName, Status: d.Status, ManagementMode: d.ManagementMode}
}

func findSpiderStorageDriver(drivers []model.SpiderStorageDriverInfo, kind string) *model.SpiderStorageDriverInfo {
	for i := range drivers {
		if strings.EqualFold(drivers[i].Kind, kind) {
			return &drivers[i]
		}
	}
	return nil
}

/*
 * Cluster lookup and storage state persistence
 */

// k8sStorageTarget is what the storage logic needs to know about a K8sCluster.
type k8sStorageTarget struct {
	NsId            string
	K8sClusterId    string
	Provider        string
	CspResourceName string
	ConnectionName  string
	Zone            string
	// FileSystemCreate is the catalog's file system creation options of the provider (set by the preparation)
	FileSystemCreate *model.K8sStorageFileSystemCreateOptions
}

func newK8sStorageTarget(nsId string, info *model.K8sClusterInfo) k8sStorageTarget {
	return k8sStorageTarget{
		NsId:            nsId,
		K8sClusterId:    info.Id,
		Provider:        strings.ToLower(info.ConnectionConfig.ProviderName),
		CspResourceName: info.CspResourceName,
		ConnectionName:  info.ConnectionName,
		Zone:            info.ConnectionConfig.RegionZoneInfo.AssignedZone,
	}
}

// loadK8sClusterForStorage reads the K8sCluster object from the KV store (404 when absent).
func loadK8sClusterForStorage(nsId, k8sClusterId string) (*model.K8sClusterInfo, error) {
	exists, err := CheckK8sCluster(nsId, k8sClusterId)
	if err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if !exists {
		return nil, newK8sStorageError(http.StatusNotFound, "K8sCluster(%s) not found in namespace(%s)", k8sClusterId, nsId)
	}
	info, err := getK8sClusterInfo(nsId, k8sClusterId)
	if err != nil {
		return nil, newK8sStorageError(http.StatusInternalServerError, "%v", err)
	}
	return info, nil
}

// requireK8sClusterAtSpider fails with 409 when the cluster is not yet known to CB-Spider.
func requireK8sClusterAtSpider(info *model.K8sClusterInfo) error {
	if info.CspResourceName == "" {
		return newK8sStorageError(http.StatusConflict, "K8sCluster(%s) is still being provisioned (status: %s)", info.Id, info.Status)
	}
	return nil
}

// updateK8sClusterStorage applies mutate to the stored K8sCluster object under the K8sCluster mutex.
// mutate returns false to skip the write. Only the storage APIs write K8sClusterInfo.Storage
// (storeK8sClusterInfo preserves it), so a slow GetK8sCluster cannot overwrite it.
func updateK8sClusterStorage(nsId, k8sClusterId string, mutate func(info *model.K8sClusterInfo) bool) error {
	k := common.GenK8sClusterKey(nsId, k8sClusterId)

	k8sClusterInfoMutex.Lock()
	defer k8sClusterInfoMutex.Unlock()

	kv, exists, err := kvstore.GetKv(k)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("K8sCluster(%s) not found", k8sClusterId)
	}
	info := &model.K8sClusterInfo{}
	if err := json.Unmarshal([]byte(kv.Value), info); err != nil {
		return err
	}
	if !mutate(info) {
		return nil
	}
	val, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return kvstore.Put(k, string(val))
}

func recordK8sClusterStorage(nsId, k8sClusterId string, state model.K8sClusterStorageInfo) {
	err := updateK8sClusterStorage(nsId, k8sClusterId, func(info *model.K8sClusterInfo) bool {
		s := state
		// A re-preparation replaces the storage state; keep the file systems TB created earlier so
		// cluster deletion still removes them; otherwise the file systems would be left on the CSP.
		s.CreatedFileSystems = mergeK8sCreatedFileSystems(info.Storage, &s)
		info.Storage = &s
		return true
	})
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to record storage state of K8sCluster(%s)", k8sClusterId)
	}
}

// mergeK8sCreatedFileSystems returns the distinct TB-created file systems recorded in prev and next.
func mergeK8sCreatedFileSystems(prev, next *model.K8sClusterStorageInfo) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, s := range []*model.K8sClusterStorageInfo{prev, next} {
		for _, name := range tumblebugCreatedK8sFileSystems(s) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

// k8sStorageJobs tracks in-flight storage preparations (key: nsId/k8sClusterId).
var k8sStorageJobs sync.Map

func k8sStorageJobKey(nsId, k8sClusterId string) string { return nsId + "/" + k8sClusterId }

func isK8sStorageJobRunning(nsId, k8sClusterId string) bool {
	_, ok := k8sStorageJobs.Load(k8sStorageJobKey(nsId, k8sClusterId))
	return ok
}

func newPreparingK8sStorageState(storageTypes []string, reason string) model.K8sClusterStorageInfo {
	state := model.K8sClusterStorageInfo{Status: model.K8sStoragePreparing, UpdatedAt: time.Now().UTC()}
	for _, t := range storageTypes {
		state.StorageTypes = append(state.StorageTypes, model.K8sStorageTypeState{StorageType: t, Status: model.K8sStoragePreparing, Reason: reason})
	}
	return state
}

/*
 * Storage driver APIs (thin wrappers over CB-Spider storagedriver)
 */

// ListK8sClusterStorageDrivers returns the CSI storage drivers of a K8sCluster from CB-Spider.
func ListK8sClusterStorageDrivers(nsId, k8sClusterId string) (*model.K8sStorageDriverListResponse, error) {
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return nil, err
	}
	if err := requireK8sClusterAtSpider(info); err != nil {
		return nil, err
	}
	drivers, err := k8sStorageSpider.ListStorageDrivers(info.CspResourceName, info.ConnectionName)
	if err != nil {
		return nil, spiderK8sStorageError(err, "failed to list storage drivers of K8sCluster(%s) from CB-Spider", k8sClusterId)
	}
	res := &model.K8sStorageDriverListResponse{K8sClusterId: k8sClusterId, StorageDrivers: []model.K8sStorageDriverInfo{}}
	for _, d := range drivers {
		res.StorageDrivers = append(res.StorageDrivers, toK8sStorageDriverInfo(d))
	}
	return res, nil
}

// EnableK8sClusterStorageDriver enables a CSI storage driver of a K8sCluster through CB-Spider.
func EnableK8sClusterStorageDriver(nsId, k8sClusterId string, req *model.K8sStorageDriverEnableReq) (*model.K8sStorageDriverInfo, error) {
	kind, ok := common.NormalizeK8sStorageDriverKind(req.Kind)
	if !ok {
		return nil, newK8sStorageError(http.StatusBadRequest, "invalid storage driver kind(%s): use Block or File", req.Kind)
	}
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return nil, err
	}
	if err := requireK8sClusterAtSpider(info); err != nil {
		return nil, err
	}
	drivers, err := k8sStorageSpider.ListStorageDrivers(info.CspResourceName, info.ConnectionName)
	if err != nil {
		return nil, spiderK8sStorageError(err, "failed to list storage drivers of K8sCluster(%s) from CB-Spider", k8sClusterId)
	}
	if d := findSpiderStorageDriver(drivers, kind); d != nil && d.ManagementMode == model.K8sStorageDriverModeNotSupported {
		return nil, newK8sStorageError(http.StatusUnprocessableEntity, "the CSP of K8sCluster(%s) offers no %s storage driver (NotSupported)", k8sClusterId, kind)
	}
	d, err := k8sStorageSpider.EnableStorageDriver(info.CspResourceName, info.ConnectionName, kind, req.Version)
	if err != nil {
		return nil, spiderK8sStorageError(err, "failed to enable %s storage driver of K8sCluster(%s) through CB-Spider", kind, k8sClusterId)
	}
	res := toK8sStorageDriverInfo(d)
	return &res, nil
}

// K8sStorageDriverDisableWarning is returned when DELETE storageDriver is called without option=force.
const K8sStorageDriverDisableWarning = "disabling a storage driver removes the CSI driver from the cluster: existing PersistentVolumes of this driver can no longer be attached or mounted, and Pods using them fail when rescheduled (volumes and data are kept). Retry with option=force to proceed"

// DisableK8sClusterStorageDriver disables a CSI storage driver of a K8sCluster through CB-Spider.
func DisableK8sClusterStorageDriver(nsId, k8sClusterId, kindParam string, force bool) error {
	kind, ok := common.NormalizeK8sStorageDriverKind(kindParam)
	if !ok {
		return newK8sStorageError(http.StatusBadRequest, "invalid storage driver kind(%s): use Block or File", kindParam)
	}
	if !force {
		return newK8sStorageError(http.StatusBadRequest, "WARNING: %s", K8sStorageDriverDisableWarning)
	}
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return err
	}
	if err := requireK8sClusterAtSpider(info); err != nil {
		return err
	}
	if isK8sStorageJobRunning(nsId, k8sClusterId) {
		return newK8sStorageError(http.StatusConflict, "storage preparation of K8sCluster(%s) is in progress", k8sClusterId)
	}
	drivers, err := k8sStorageSpider.ListStorageDrivers(info.CspResourceName, info.ConnectionName)
	if err != nil {
		return spiderK8sStorageError(err, "failed to list storage drivers of K8sCluster(%s) from CB-Spider", k8sClusterId)
	}
	if d := findSpiderStorageDriver(drivers, kind); d != nil {
		switch d.ManagementMode {
		case model.K8sStorageDriverModeBuiltIn:
			return newK8sStorageError(http.StatusBadRequest, "the %s storage driver of K8sCluster(%s) is BuiltIn and cannot be disabled", kind, k8sClusterId)
		case model.K8sStorageDriverModeNotSupported:
			return newK8sStorageError(http.StatusUnprocessableEntity, "the CSP of K8sCluster(%s) offers no %s storage driver (NotSupported)", k8sClusterId, kind)
		}
	}
	if err := k8sStorageSpider.DisableStorageDriver(info.CspResourceName, info.ConnectionName, kind); err != nil {
		return spiderK8sStorageError(err, "failed to disable %s storage driver of K8sCluster(%s) through CB-Spider", kind, k8sClusterId)
	}

	// Storage types that relied on this driver are no longer Ready.
	err = updateK8sClusterStorage(nsId, k8sClusterId, func(c *model.K8sClusterInfo) bool {
		if c.Storage == nil {
			return false
		}
		changed := false
		for i := range c.Storage.StorageTypes {
			ts := &c.Storage.StorageTypes[i]
			if ts.Driver != nil && strings.EqualFold(ts.Driver.Kind, kind) {
				ts.Status = model.K8sStorageNotReady
				ts.Reason = fmt.Sprintf("%s storage driver was disabled by the user", kind)
				ts.Driver.Status = model.K8sStorageDriverDisabled
				changed = true
			}
		}
		if changed {
			c.Storage.Actions = append(c.Storage.Actions, model.K8sStorageAction{Step: "disableStorageDriver", Kind: kind, To: model.K8sStorageDriverDisabled, Time: time.Now().UTC()})
			c.Storage.Status = aggregateK8sStorageStatus(c.Storage.StorageTypes)
			c.Storage.UpdatedAt = time.Now().UTC()
		}
		return changed
	})
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to update storage state of K8sCluster(%s) after disabling the %s driver", k8sClusterId, kind)
	}
	return nil
}

/*
 * Storage preparation (single cluster)
 */

// StartK8sClusterStoragePreparation validates the request, marks the cluster Preparing and
// prepares the abstract storage types asynchronously. Errors: 400, 404, 409.
func StartK8sClusterStoragePreparation(nsId, k8sClusterId string, req *model.K8sClusterStorageReq) (*model.K8sClusterStoragePrepareResponse, error) {
	if req == nil {
		req = &model.K8sClusterStorageReq{}
	}
	types, err := common.NormalizeK8sStorageTypes(&common.RuntimeK8sStorageInfo, req.StorageTypes)
	if err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if err := validateK8sStorageFileSystemReq(req.FileSystem); err != nil {
		return nil, err
	}
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return nil, err
	}
	if err := requireK8sClusterAtSpider(info); err != nil {
		return nil, err
	}
	if info.Status != "" && info.Status != model.K8sClusterActive {
		return nil, newK8sStorageError(http.StatusConflict, "K8sCluster(%s) is not Active (status: %s)", k8sClusterId, info.Status)
	}

	jobKey := k8sStorageJobKey(nsId, k8sClusterId)
	if _, loaded := k8sStorageJobs.LoadOrStore(jobKey, time.Now()); loaded {
		return nil, newK8sStorageError(http.StatusConflict, "storage preparation of K8sCluster(%s) is already in progress", k8sClusterId)
	}

	target := newK8sStorageTarget(nsId, info)
	fsReq := req.FileSystem
	recordK8sClusterStorage(nsId, k8sClusterId, newPreparingK8sStorageState(types, ""))

	go func() {
		defer k8sStorageJobs.Delete(jobKey)
		runAndRecordK8sStoragePreparation(target, types, fsReq)
	}()

	return &model.K8sClusterStoragePrepareResponse{K8sClusterId: k8sClusterId, Status: model.K8sStoragePreparing, StorageTypes: types}, nil
}

func validateK8sStorageFileSystemReq(fs *model.K8sStorageFileSystemReq) error {
	if fs == nil || fs.Name == "" {
		return nil
	}
	if err := common.CheckString(fs.Name); err != nil {
		return newK8sStorageError(http.StatusBadRequest, "invalid fileSystem.name(%s): %v", fs.Name, err)
	}
	return nil
}

func runAndRecordK8sStoragePreparation(target k8sStorageTarget, types []string, fsReq *model.K8sStorageFileSystemReq) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("Panic while preparing storage of K8sCluster(%s): %v", target.K8sClusterId, r)
			state := newPreparingK8sStorageState(types, "")
			for i := range state.StorageTypes {
				state.StorageTypes[i].Status = model.K8sStorageNotReady
				state.StorageTypes[i].Reason = fmt.Sprintf("internal error: %v", r)
			}
			state.Status = model.K8sStorageNotReady
			recordK8sClusterStorage(target.NsId, target.K8sClusterId, state)
		}
	}()
	log.Info().Msgf("Preparing %v on K8sCluster(%s/%s)", types, target.NsId, target.K8sClusterId)
	final := runK8sStoragePreparation(k8sStorageSpider, &common.RuntimeK8sStorageInfo, target, types, fsReq, func(s model.K8sClusterStorageInfo) {
		recordK8sClusterStorage(target.NsId, target.K8sClusterId, s)
	})
	recordK8sClusterStorage(target.NsId, target.K8sClusterId, final)
	log.Info().Msgf("K8sCluster(%s/%s) storage status: %s", target.NsId, target.K8sClusterId, final.Status)
}

// k8sStorageKindResult is the preparation result of one driver kind, shared by the storage types needing it.
type k8sStorageKindResult struct {
	status     model.K8sStorageStatus
	reason     string
	driver     *model.K8sStorageDriverRef
	fileSystem *model.K8sStorageFileSystemState
}

// runK8sStoragePreparation prepares the storage types on one cluster and returns the final state.
// onUpdate receives intermediate states (may be nil). It only talks to CB-Spider through api.
func runK8sStoragePreparation(api k8sStorageSpiderAPI, catalog *model.K8sStorageAssetInfo, target k8sStorageTarget, types []string, fsReq *model.K8sStorageFileSystemReq, onUpdate func(model.K8sClusterStorageInfo)) model.K8sClusterStorageInfo {
	if p, ok := catalog.Providers[strings.ToLower(target.Provider)]; ok {
		target.FileSystemCreate = p.FileStorage.FileSystemCreate
	}
	state := newPreparingK8sStorageState(types, "")
	publish := func() {
		state.Status = aggregateK8sStorageStatus(state.StorageTypes)
		state.UpdatedAt = time.Now().UTC()
		if onUpdate != nil {
			onUpdate(state)
		}
	}
	addAction := func(a model.K8sStorageAction) {
		a.Time = time.Now().UTC()
		state.Actions = append(state.Actions, a)
	}

	drivers, err := api.ListStorageDrivers(target.CspResourceName, target.ConnectionName)
	if err != nil {
		for i := range state.StorageTypes {
			state.StorageTypes[i].Status = model.K8sStorageNotReady
			state.StorageTypes[i].Reason = fmt.Sprintf("CB-Spider storagedriver: %v", err)
		}
		publish()
		return state
	}

	// Record the cluster VPC and subnet CSP IDs for StorageClass parameters that name them.
	if cluster, err := api.GetCluster(target.CspResourceName, target.ConnectionName); err == nil {
		state.ClusterVpcCspId = cluster.Network.VpcIID.SystemId
		for _, subnet := range cluster.Network.SubnetIIDs {
			if subnet.SystemId != "" {
				state.ClusterSubnetCspIds = append(state.ClusterSubnetCspIds, subnet.SystemId)
			}
		}
	} else {
		log.Warn().Err(err).Msgf("Failed to get the network of K8sCluster(%s)", target.K8sClusterId)
	}

	results := map[string]k8sStorageKindResult{}
	for i := range state.StorageTypes {
		ts := &state.StorageTypes[i]
		kind, err := common.ResolveK8sStorageDriverKind(catalog, ts.StorageType)
		if err != nil {
			ts.Status, ts.Reason = model.K8sStorageNotReady, err.Error()
			publish()
			continue
		}
		r, done := results[kind]
		if !done {
			r = prepareK8sStorageDriverKind(api, target, kind, drivers, fsReq, addAction)
			results[kind] = r
		}
		ts.Status, ts.Reason, ts.Driver, ts.FileSystem = r.status, r.reason, r.driver, r.fileSystem
		publish()
	}
	publish()
	return state
}

// prepareK8sStorageDriverKind makes one driver kind usable: optional file system, then driver enablement.
func prepareK8sStorageDriverKind(api k8sStorageSpiderAPI, target k8sStorageTarget, kind string, drivers []model.SpiderStorageDriverInfo, fsReq *model.K8sStorageFileSystemReq, addAction func(model.K8sStorageAction)) k8sStorageKindResult {
	d := findSpiderStorageDriver(drivers, kind)
	if d == nil {
		return k8sStorageKindResult{status: model.K8sStorageNotReady, reason: fmt.Sprintf("CB-Spider reported no %s storage driver for this cluster", kind)}
	}
	res := k8sStorageKindResult{driver: toK8sStorageDriverRef(*d)}
	if d.ManagementMode == model.K8sStorageDriverModeNotSupported {
		res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("the CSP offers no %s storage driver (NotSupported)", kind)
		return res
	}

	if kind == model.K8sStorageDriverKindFile && d.RequiresFileSystem {
		if fsReq == nil || (!fsReq.Create && fsReq.Name == "") {
			res.status, res.reason = model.K8sStorageNotReady, "file system required: the File driver of this CSP needs an existing file system; set fileSystem.create=true or fileSystem.name"
			return res
		}
		fs, err := prepareK8sStorageFileSystem(api, target, fsReq, addAction)
		if err != nil {
			res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("file system: %v", err)
			return res
		}
		res.fileSystem = fs
	}

	if d.Status == model.K8sStorageDriverEnabled {
		res.status = model.K8sStorageReady
		return res
	}
	if d.ManagementMode != model.K8sStorageDriverModeAddon {
		res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("%s driver (%s) is %s and cannot be enabled through the CSP API", kind, d.ManagementMode, d.Status)
		return res
	}

	from := d.Status
	enabled, err := api.EnableStorageDriver(target.CspResourceName, target.ConnectionName, kind, "")
	// The CSP may refuse while the cluster is still busy right after it becomes Active, e.g., NHN NKS
	// rejects add-on installs while a config stack is *_IN_PROGRESS. Retry
	// within the poll timeout unless the error is permanent.
	for retryStart := time.Now(); err != nil && !isPermanentK8sStorageDriverError(err) && time.Since(retryStart) < k8sStorageDriverPollTimeout; {
		log.Warn().Err(err).Msgf("Enabling %s driver of K8sCluster(%s) failed; retrying", kind, target.K8sClusterId)
		time.Sleep(k8sStorageDriverPollInterval)
		enabled, err = api.EnableStorageDriver(target.CspResourceName, target.ConnectionName, kind, "")
	}
	if err != nil {
		res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("failed to enable %s storage driver through CB-Spider: %v", kind, err)
		addAction(model.K8sStorageAction{Step: "enableStorageDriver", Kind: kind, From: from, To: model.K8sStorageDriverFailed})
		return res
	}
	current := enabled
	start := time.Now()
	// A Failed status can be transient: e.g., an EKS add-on is DEGRADED until the node group's nodes
	// join. Keep polling a Failed driver for a grace period.
	var failedSince time.Time
	for current.Status != model.K8sStorageDriverEnabled {
		if time.Since(start) >= k8sStorageDriverPollTimeout {
			break
		}
		if current.Status == model.K8sStorageDriverFailed {
			if failedSince.IsZero() {
				failedSince = time.Now()
			} else if time.Since(failedSince) >= k8sStorageDriverFailedGrace {
				break
			}
		} else {
			failedSince = time.Time{}
		}
		time.Sleep(k8sStorageDriverPollInterval)
		list, err := api.ListStorageDrivers(target.CspResourceName, target.ConnectionName)
		if err != nil {
			log.Warn().Err(err).Msgf("Polling %s driver of K8sCluster(%s) failed; retrying", kind, target.K8sClusterId)
			continue
		}
		if nd := findSpiderStorageDriver(list, kind); nd != nil {
			current = *nd
		}
	}
	if current.Kind == "" {
		current.Kind = kind
	}
	if current.ManagementMode == "" {
		current.ManagementMode = d.ManagementMode
	}
	res.driver = toK8sStorageDriverRef(current)
	addAction(model.K8sStorageAction{Step: "enableStorageDriver", Kind: kind, From: from, To: current.Status})

	switch current.Status {
	case model.K8sStorageDriverEnabled:
		res.status = model.K8sStorageReady
	case model.K8sStorageDriverFailed:
		res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("%s storage driver enablement failed at the CSP", kind)
	default:
		res.status, res.reason = model.K8sStorageNotReady, fmt.Sprintf("%s storage driver did not become Enabled within %s (status: %s)", kind, k8sStorageDriverPollTimeout, current.Status)
	}
	return res
}

func fsCreateCapacityGB(o *model.K8sStorageFileSystemCreateOptions) int64 {
	if o == nil {
		return 0
	}
	return o.CapacityGB
}

// fsCreatePerformanceInfos returns the PerformanceInfo candidates to try in order; a single nil
// candidate means no CSP-specific option.
func fsCreatePerformanceInfos(o *model.K8sStorageFileSystemCreateOptions) []map[string]string {
	if o == nil {
		return []map[string]string{nil}
	}
	return append([]map[string]string{o.PerformanceInfo}, o.PerformanceInfoAlternatives...)
}

// isK8sStorageZoneNotSupportedError tells whether the CSP rejected the file system type for the zone,
// so another PerformanceInfo candidate may succeed (e.g., Alibaba "InvalidAzone.NotSupport").
func isK8sStorageZoneNotSupportedError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalidazone") || strings.Contains(msg, "zone not support")
}

// isPermanentK8sStorageDriverError tells whether retrying EnableStorageDriver cannot help.
func isPermanentK8sStorageDriverError(err error) bool {
	msg := strings.ToLower(err.Error())
	// NHN phrases a busy cluster as "... status UPDATE_IN_PROGRESS is not supported".
	if strings.Contains(msg, "_in_progress") || strings.Contains(msg, "in progress") {
		return false
	}
	return strings.Contains(msg, "not implemented") || strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "notsupported") || strings.Contains(msg, "invalid storage driver kind")
}

// k8sStorageDefaultFileSystemName is the file system name TB uses when fileSystem.name is empty.
func k8sStorageDefaultFileSystemName(k8sClusterId string) string {
	return strings.ToLower(k8sClusterId) + "-shared"
}

func sameIID(a, b model.IID) bool {
	return (a.NameId != "" && a.NameId == b.NameId) || (a.SystemId != "" && a.SystemId == b.SystemId)
}

// k8sStorageMountTargets puts a mount target in every cluster subnet with the cluster security groups.
// Mount targets in the CSP default security group may not be reachable from the nodes over NFS (TCP 2049),
// e.g., on EKS. Returns nil when the cluster reports no security groups, so the CSP default applies.
func k8sStorageMountTargets(network model.SpiderNetworkInfo) []model.SpiderFileSystemMountTargetReq {
	var sgs []model.IID
	for _, sg := range network.SecurityGroupIIDs {
		// "#<CSP ID>" names a security group CB-Spider does not manage (e.g., the NHN NKS cluster
		// security group); CB-Spider cannot resolve it, so it is skipped.
		if sg.NameId != "" && !strings.HasPrefix(sg.NameId, "#") {
			sgs = append(sgs, model.IID{NameId: sg.NameId})
		}
	}
	if len(sgs) == 0 {
		return nil
	}
	targets := make([]model.SpiderFileSystemMountTargetReq, 0, len(network.SubnetIIDs))
	for _, subnet := range network.SubnetIIDs {
		targets = append(targets, model.SpiderFileSystemMountTargetReq{SubnetIID: model.IID{NameId: subnet.NameId}, SecurityGroupIIDs: sgs})
	}
	return targets
}

// prepareK8sStorageFileSystem prepares a file system for a File driver with existing CB-Spider APIs only:
// GET cluster (VPC, subnets) → create or reuse the file system in the cluster VPC → attach every
// cluster subnet that is not attached yet.
func prepareK8sStorageFileSystem(api k8sStorageSpiderAPI, target k8sStorageTarget, fsReq *model.K8sStorageFileSystemReq, addAction func(model.K8sStorageAction)) (*model.K8sStorageFileSystemState, error) {
	cluster, err := api.GetCluster(target.CspResourceName, target.ConnectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster network from CB-Spider: %w", err)
	}
	vpc := cluster.Network.VpcIID
	if vpc.NameId == "" && vpc.SystemId == "" {
		return nil, fmt.Errorf("CB-Spider reported no VPC for the cluster")
	}

	name := fsReq.Name
	created := false
	var fs model.SpiderFileSystemInfo
	if name != "" {
		fs, err = api.GetFileSystem(name, target.ConnectionName)
		if err != nil {
			return nil, fmt.Errorf("file system(%s) not found: %w", name, err)
		}
		if !sameIID(fs.VpcIID, vpc) {
			return nil, fmt.Errorf("file system(%s) is in VPC(%s), not in the cluster VPC(%s)", name, fs.VpcIID.NameId+fs.VpcIID.SystemId, vpc.NameId)
		}
	} else {
		name = k8sStorageDefaultFileSystemName(target.K8sClusterId)
		// Reuse the file system of a previous run (idempotent re-preparation).
		if existing, getErr := api.GetFileSystem(name, target.ConnectionName); getErr == nil && existing.IId.NameId != "" {
			if !sameIID(existing.VpcIID, vpc) {
				return nil, fmt.Errorf("file system(%s) already exists in another VPC", name)
			}
			fs = existing
			created = true
		} else {
			reqInfo := model.SpiderFileSystemCreateReqInfo{
				Name:   name,
				Zone:   target.Zone,
				VpcIID: model.IID{NameId: vpc.NameId},
				// Empty: the CB-Spider driver uses the CSP default (AWS 4.1, Alibaba 4.0); a fixed 4.1 is
				// rejected by Alibaba NAS.
				NFSVersion:      "",
				MountTargetList: k8sStorageMountTargets(cluster.Network),
				CapacityGB:      fsCreateCapacityGB(target.FileSystemCreate),
				TagList:         []model.KeyValue{{Key: model.LabelManager, Value: model.StrManager}, {Key: "k8sClusterId", Value: target.K8sClusterId}},
			}
			if reqInfo.MountTargetList == nil {
				// Without security groups, attach the cluster subnets at creation: some CSPs (NHN NAS)
				// require them in the create request.
				for _, subnet := range cluster.Network.SubnetIIDs {
					reqInfo.AccessSubnetList = append(reqInfo.AccessSubnetList, model.IID{NameId: subnet.NameId})
				}
			}
			for _, perf := range fsCreatePerformanceInfos(target.FileSystemCreate) {
				reqInfo.PerformanceInfo = perf
				fs, err = api.CreateFileSystem(model.SpiderFileSystemCreateReq{ConnectionName: target.ConnectionName, ReqInfo: reqInfo})
				if err == nil || !isK8sStorageZoneNotSupportedError(err) {
					break
				}
			}
			if err != nil {
				return nil, fmt.Errorf("failed to create file system(%s) through CB-Spider: %w", name, err)
			}
			created = true
			addAction(model.K8sStorageAction{Step: "createFileSystem", Kind: model.K8sStorageDriverKindFile, Target: name})
		}
	}

	// Mount targets need an Available file system.
	start := time.Now()
	for fs.Status == model.SpiderFileSystemCreating && time.Since(start) < k8sStorageDriverPollTimeout {
		time.Sleep(k8sStorageDriverPollInterval)
		if cur, err := api.GetFileSystem(name, target.ConnectionName); err == nil {
			fs = cur
		}
	}
	if fs.Status == model.SpiderFileSystemError {
		return nil, fmt.Errorf("file system(%s) is in Error status", name)
	}
	if fs.Status == model.SpiderFileSystemCreating {
		return nil, fmt.Errorf("file system(%s) did not become Available within %s", name, k8sStorageDriverPollTimeout)
	}

	attached := 0
	for _, subnet := range cluster.Network.SubnetIIDs {
		already := false
		for _, a := range fs.AccessSubnetList {
			if sameIID(a, subnet) {
				already = true
				break
			}
		}
		if already {
			attached++
			continue
		}
		if _, err := api.AddFileSystemAccessSubnet(name, target.ConnectionName, subnet); err != nil {
			return nil, fmt.Errorf("failed to attach subnet(%s) to file system(%s): %w", subnet.NameId, name, err)
		}
		attached++
		addAction(model.K8sStorageAction{Step: "attachFileSystem", Kind: model.K8sStorageDriverKindFile, Target: name + ":" + subnet.NameId})
	}

	if cur, err := api.GetFileSystem(name, target.ConnectionName); err == nil {
		fs = cur
	}
	endpoint := ""
	for _, mt := range fs.MountTargetList {
		if mt.Endpoint != "" {
			endpoint = mt.Endpoint
			break
		}
	}
	return &model.K8sStorageFileSystemState{
		Name:               name,
		CspId:              fs.IId.SystemId,
		Endpoint:           endpoint,
		AccessSubnets:      attached,
		CreatedByTumblebug: created,
	}, nil
}

// refreshK8sStorageState recomputes the per-type status from the current CB-Spider driver list
// (no CSP-side change). Used by GET when no preparation is running.
func refreshK8sStorageState(catalog *model.K8sStorageAssetInfo, recorded model.K8sClusterStorageInfo, drivers []model.SpiderStorageDriverInfo) model.K8sClusterStorageInfo {
	out := recorded
	out.StorageTypes = make([]model.K8sStorageTypeState, len(recorded.StorageTypes))
	copy(out.StorageTypes, recorded.StorageTypes)
	for i := range out.StorageTypes {
		ts := &out.StorageTypes[i]
		kind := ""
		if ts.Driver != nil {
			kind = ts.Driver.Kind
		}
		if kind == "" {
			k, err := common.ResolveK8sStorageDriverKind(catalog, ts.StorageType)
			if err != nil {
				ts.Status, ts.Reason = model.K8sStorageNotReady, err.Error()
				continue
			}
			kind = k
		}
		d := findSpiderStorageDriver(drivers, kind)
		switch {
		case d == nil:
			ts.Status, ts.Reason = model.K8sStorageNotReady, fmt.Sprintf("CB-Spider reported no %s storage driver for this cluster", kind)
		case d.ManagementMode == model.K8sStorageDriverModeNotSupported:
			ts.Driver = toK8sStorageDriverRef(*d)
			ts.Status, ts.Reason = model.K8sStorageNotReady, fmt.Sprintf("the CSP offers no %s storage driver (NotSupported)", kind)
		case kind == model.K8sStorageDriverKindFile && d.RequiresFileSystem && ts.FileSystem == nil:
			ts.Driver = toK8sStorageDriverRef(*d)
			ts.Status = model.K8sStorageNotReady
			if ts.Reason == "" || recorded.Status == model.K8sStoragePreparing {
				ts.Reason = "file system required: set fileSystem.create=true or fileSystem.name and PUT .../storage again"
			}
		case d.Status == model.K8sStorageDriverEnabled:
			ts.Driver = toK8sStorageDriverRef(*d)
			ts.Status, ts.Reason = model.K8sStorageReady, ""
		default:
			ts.Driver = toK8sStorageDriverRef(*d)
			ts.Status, ts.Reason = model.K8sStorageNotReady, fmt.Sprintf("%s storage driver status is %s", kind, d.Status)
		}
	}
	out.Status = aggregateK8sStorageStatus(out.StorageTypes)
	return out
}

func k8sStorageStatusResponse(k8sClusterId, provider string, s *model.K8sClusterStorageInfo) *model.K8sClusterStorageStatusResponse {
	res := &model.K8sClusterStorageStatusResponse{K8sClusterId: k8sClusterId, Provider: provider, StorageTypes: []model.K8sStorageTypeState{}}
	if s == nil {
		res.Status = model.K8sStorageNotReady
		res.Reason = "storage has not been prepared; call PUT /ns/{nsId}/k8sCluster/{k8sClusterId}/storage"
		return res
	}
	res.Status = s.Status
	if s.StorageTypes != nil {
		res.StorageTypes = s.StorageTypes
	}
	res.Actions = s.Actions
	t := s.UpdatedAt
	res.UpdatedAt = &t
	return res
}

// GetK8sClusterStorage returns the storage state of a K8sCluster, refreshed from the CB-Spider driver status.
func GetK8sClusterStorage(nsId, k8sClusterId string) (*model.K8sClusterStorageStatusResponse, error) {
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return nil, err
	}
	provider := strings.ToLower(info.ConnectionConfig.ProviderName)
	if info.Storage == nil || isK8sStorageJobRunning(nsId, k8sClusterId) || info.CspResourceName == "" {
		return k8sStorageStatusResponse(k8sClusterId, provider, info.Storage), nil
	}

	drivers, err := k8sStorageSpider.ListStorageDrivers(info.CspResourceName, info.ConnectionName)
	if err != nil {
		return nil, spiderK8sStorageError(err, "failed to refresh storage drivers of K8sCluster(%s) from CB-Spider", k8sClusterId)
	}
	refreshed := refreshK8sStorageState(&common.RuntimeK8sStorageInfo, *info.Storage, drivers)
	refreshed.UpdatedAt = time.Now().UTC()

	// Persist so the manifest API sees the current readiness, unless a preparation started meanwhile.
	err = updateK8sClusterStorage(nsId, k8sClusterId, func(c *model.K8sClusterInfo) bool {
		if isK8sStorageJobRunning(nsId, k8sClusterId) {
			return false
		}
		s := refreshed
		c.Storage = &s
		return true
	})
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to persist refreshed storage state of K8sCluster(%s)", k8sClusterId)
	}
	return k8sStorageStatusResponse(k8sClusterId, provider, &refreshed), nil
}

/*
 * Storage preparation on cluster creation
 */

// ValidateK8sClusterStorageReq validates the optional storage option of a cluster creation request.
func ValidateK8sClusterStorageReq(req *model.K8sClusterStorageReq) error {
	if req == nil {
		return nil
	}
	if _, err := common.NormalizeK8sStorageTypes(&common.RuntimeK8sStorageInfo, req.StorageTypes); err != nil {
		return fmt.Errorf("invalid storage option: %w", err)
	}
	return validateK8sStorageFileSystemReq(req.FileSystem)
}

// startK8sStoragePreparationAfterCreate waits for the new cluster (and its node groups) to become
// Active, then prepares storage. Failures are recorded in the storage state only; they never fail
// the cluster creation.
func startK8sStoragePreparationAfterCreate(nsId, k8sClusterId string, req *model.K8sClusterStorageReq) {
	types, err := common.NormalizeK8sStorageTypes(&common.RuntimeK8sStorageInfo, req.StorageTypes)
	if err != nil {
		log.Warn().Err(err).Msgf("Skipping storage preparation of K8sCluster(%s)", k8sClusterId)
		return
	}
	jobKey := k8sStorageJobKey(nsId, k8sClusterId)
	if _, loaded := k8sStorageJobs.LoadOrStore(jobKey, time.Now()); loaded {
		return
	}
	recordK8sClusterStorage(nsId, k8sClusterId, newPreparingK8sStorageState(types, "waiting for the K8sCluster and its node groups to become Active"))
	fsReq := req.FileSystem

	go func() {
		defer k8sStorageJobs.Delete(jobKey)
		notReady := func(reason string) {
			state := newPreparingK8sStorageState(types, reason)
			for i := range state.StorageTypes {
				state.StorageTypes[i].Status = model.K8sStorageNotReady
			}
			state.Status = model.K8sStorageNotReady
			recordK8sClusterStorage(nsId, k8sClusterId, state)
		}

		start := time.Now()
		for {
			info, err := GetK8sCluster(nsId, k8sClusterId)
			if err != nil {
				exists, _ := CheckK8sCluster(nsId, k8sClusterId)
				if !exists {
					log.Info().Msgf("K8sCluster(%s) was removed; storage preparation cancelled", k8sClusterId)
					return
				}
			} else if isK8sClusterAndNodeGroupsActive(info) {
				runAndRecordK8sStoragePreparation(newK8sStorageTarget(nsId, info), types, fsReq)
				return
			}
			if time.Since(start) >= k8sStorageClusterPollTimeout {
				notReady(fmt.Sprintf("K8sCluster did not become Active within %s; prepare storage later with PUT .../storage", k8sStorageClusterPollTimeout))
				return
			}
			time.Sleep(k8sStorageClusterPollInterval)
		}
	}()
}

func isK8sClusterAndNodeGroupsActive(info *model.K8sClusterInfo) bool {
	if info.Status != model.K8sClusterActive || info.CspResourceName == "" {
		return false
	}
	// CSI drivers run on nodes, so wait for at least one node group. Some CSPs (e.g., EKS) create
	// the cluster without node groups.
	if len(info.K8sNodeGroupList) == 0 {
		return false
	}
	for _, ng := range info.K8sNodeGroupList {
		if ng.Status != model.K8sNodeGroupActive {
			return false
		}
	}
	return true
}

/*
 * Multi-cluster storage
 */

func listK8sStorageTargetIds(nsId string, ids []string) ([]string, error) {
	if len(ids) > 0 {
		out := []string{}
		seen := map[string]bool{}
		for _, id := range ids {
			for _, part := range strings.Split(id, ",") {
				part = strings.TrimSpace(part)
				if part != "" && !seen[part] {
					seen[part] = true
					out = append(out, part)
				}
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	all, err := ListK8sClusterId(nsId)
	if err != nil {
		return nil, newK8sStorageError(http.StatusInternalServerError, "failed to list K8sClusters of namespace(%s): %v", nsId, err)
	}
	out := []string{}
	for _, id := range all {
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, newK8sStorageError(http.StatusNotFound, "no K8sCluster in namespace(%s)", nsId)
	}
	return out, nil
}

// StartK8sMultiClusterStoragePreparation starts storage preparation on many clusters in parallel.
// Clusters that cannot start (not found, busy, not Active) are reported in results with a reason.
func StartK8sMultiClusterStoragePreparation(nsId string, req *model.K8sMultiClusterStorageReq) (*model.K8sMultiClusterStoragePrepareResponse, error) {
	if req == nil {
		req = &model.K8sMultiClusterStorageReq{}
	}
	if err := common.CheckString(nsId); err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	types, err := common.NormalizeK8sStorageTypes(&common.RuntimeK8sStorageInfo, req.StorageTypes)
	if err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if err := validateK8sStorageFileSystemReq(req.FileSystem); err != nil {
		return nil, err
	}
	ids, err := listK8sStorageTargetIds(nsId, req.K8sClusterIds)
	if err != nil {
		return nil, err
	}
	res := &model.K8sMultiClusterStoragePrepareResponse{NsId: nsId, Results: make([]model.K8sClusterStoragePrepareResponse, len(ids))}
	for i, id := range ids {
		var fsReq *model.K8sStorageFileSystemReq
		if req.FileSystem != nil {
			// A named file system belongs to one VPC; with many clusters only create=true is meaningful.
			f := *req.FileSystem
			if len(ids) > 1 {
				f.Name = ""
			}
			fsReq = &f
		}
		r, err := StartK8sClusterStoragePreparation(nsId, id, &model.K8sClusterStorageReq{StorageTypes: types, FileSystem: fsReq})
		if err != nil {
			res.Results[i] = model.K8sClusterStoragePrepareResponse{K8sClusterId: id, Status: model.K8sStorageNotReady, StorageTypes: types, Reason: err.Error()}
			continue
		}
		res.Results[i] = *r
	}
	return res, nil
}

// GetK8sMultiClusterStorage aggregates the storage state of all K8sClusters of a namespace (in parallel).
func GetK8sMultiClusterStorage(nsId string, k8sClusterIds []string) (*model.K8sMultiClusterStorageStatusResponse, error) {
	if err := common.CheckString(nsId); err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	ids, err := listK8sStorageTargetIds(nsId, k8sClusterIds)
	if err != nil {
		if K8sStorageErrorCode(err) == http.StatusNotFound {
			return &model.K8sMultiClusterStorageStatusResponse{NsId: nsId, Results: []model.K8sClusterStorageStatusResponse{}}, nil
		}
		return nil, err
	}
	results := make([]model.K8sClusterStorageStatusResponse, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			r, err := GetK8sClusterStorage(nsId, id)
			if err != nil {
				results[i] = model.K8sClusterStorageStatusResponse{K8sClusterId: id, Status: model.K8sStorageNotReady, Reason: err.Error(), StorageTypes: []model.K8sStorageTypeState{}}
				return
			}
			results[i] = *r
		}(i, id)
	}
	wg.Wait()
	return &model.K8sMultiClusterStorageStatusResponse{NsId: nsId, Summary: summarizeK8sStorage(results), Results: results}, nil
}

/*
 * StorageClass manifests
 */

// GetK8sClusterStorageClassManifests generates the cb-<storageType> StorageClasses of a K8sCluster
// from the catalog and the recorded storage state (no cluster access).
func GetK8sClusterStorageClassManifests(nsId, k8sClusterId string, storageTypes []string, setDefault bool) ([]model.K8sStorageClassManifest, error) {
	info, err := loadK8sClusterForStorage(nsId, k8sClusterId)
	if err != nil {
		return nil, err
	}
	return GenerateK8sStorageClassManifests(&common.RuntimeK8sStorageInfo, info.ConnectionConfig.ProviderName, storageTypes, setDefault, info.Storage)
}

// GetK8sMultiClusterStorageClassManifests generates StorageClass manifests for many clusters.
// Clusters whose storage types are not Ready are reported with a reason instead of failing the call.
func GetK8sMultiClusterStorageClassManifests(nsId string, k8sClusterIds, storageTypes []string, setDefault bool) (*model.K8sMultiClusterStorageClassManifestResponse, error) {
	if err := common.CheckString(nsId); err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	if _, err := common.NormalizeK8sStorageTypes(&common.RuntimeK8sStorageInfo, storageTypes); err != nil {
		return nil, newK8sStorageError(http.StatusBadRequest, "%v", err)
	}
	ids, err := listK8sStorageTargetIds(nsId, k8sClusterIds)
	if err != nil {
		if K8sStorageErrorCode(err) == http.StatusNotFound && len(k8sClusterIds) == 0 {
			return &model.K8sMultiClusterStorageClassManifestResponse{NsId: nsId, Results: []model.K8sClusterStorageClassManifestResult{}}, nil
		}
		return nil, err
	}
	res := &model.K8sMultiClusterStorageClassManifestResponse{NsId: nsId, Results: make([]model.K8sClusterStorageClassManifestResult, 0, len(ids))}
	for _, id := range ids {
		r := model.K8sClusterStorageClassManifestResult{K8sClusterId: id, Status: model.K8sStorageNotReady}
		info, err := loadK8sClusterForStorage(nsId, id)
		if err != nil {
			r.Reason = err.Error()
			res.Results = append(res.Results, r)
			continue
		}
		r.Provider = strings.ToLower(info.ConnectionConfig.ProviderName)
		manifests, err := GenerateK8sStorageClassManifests(&common.RuntimeK8sStorageInfo, r.Provider, storageTypes, setDefault, info.Storage)
		if err != nil {
			r.Reason = err.Error()
			res.Results = append(res.Results, r)
			continue
		}
		y, err := MarshalK8sStorageClassManifestsYAML(manifests)
		if err != nil {
			r.Reason = err.Error()
			res.Results = append(res.Results, r)
			continue
		}
		r.Status, r.Manifest = model.K8sStorageReady, y
		res.Results = append(res.Results, r)
	}
	return res, nil
}
