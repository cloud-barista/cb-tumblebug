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

// Package mci is to manage multi-cloud infra
package infra

import (
	"fmt"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// CreateVmSnapshot is func to create VM snapshot
func CreateVmSnapshot(nsId string, mciId string, vmId string, snapshotReq model.SnapshotReq) (model.ImageInfo, error) {
	vm, err := GetVmObject(nsId, mciId, vmId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	snapshotName := snapshotReq.Name

	if snapshotName == "" {
		snapshotName = common.GenUid()
	}

	requestBody := model.SpiderMyImageReq{
		ConnectionName: vm.ConnectionName,
		ReqInfo: struct {
			Name     string
			SourceVM string
		}{
			Name:     snapshotName,
			SourceVM: vm.CspResourceName,
		},
	}

	// Inspect DataDisks before creating VM snapshot
	// Disabled because: there is no difference in dataDisks before and after creating VM snapshot
	// inspect_result_before_snapshot, err := InspectResources(vm.ConnectionName, model.StrDataDisk)
	// dataDisks_before_snapshot := inspect_result_before_snapshot.Resources.OnTumblebug.Info
	// if err != nil {
	// 	err := fmt.Errorf("Failed to get current datadisks' info. \n")
	// 	log.Error().Err(err).Msg("")
	// 	return model.ImageInfo{}, err
	// }

	// Create VM snapshot using ExecuteHttpRequest
	var tempSpiderMyImageInfo model.SpiderMyImageInfo
	client := resty.New()
	client.SetTimeout(5 * time.Minute)
	url := fmt.Sprintf("%s/myimage", model.SpiderRestUrl)
	method := "POST"

	err = clientManager.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		clientManager.SetUseBody(requestBody),
		&requestBody,
		&tempSpiderMyImageInfo,
		clientManager.MediumDuration,
	)

	if err != nil {
		log.Trace().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// Get the source VM's image information to inherit properties
	// Try to get image using CspImageName with provider/region information
	var sourceImageInfo model.ImageInfo
	var imgErr error

	// First, try to get by composite primary key (ProviderName + CspImageName)
	if vm.CspImageName != "" && vm.ConnectionConfig.ProviderName != "" {
		sourceImageInfo, imgErr = resource.GetImageByPrimaryKey(model.SystemCommonNs, vm.ConnectionConfig.ProviderName, vm.CspImageName)
		if imgErr != nil {
			log.Debug().Err(imgErr).Msgf("Failed to get image by primary key (provider: %s, cspImageName: %s)", vm.ConnectionConfig.ProviderName, vm.CspImageName)
		}
	}

	// If not found, try using ImageId (TB image ID)
	if imgErr != nil && vm.ImageId != "" {
		sourceImageInfo, imgErr = resource.GetImage(nsId, vm.ImageId)
		if imgErr != nil {
			log.Debug().Err(imgErr).Msgf("Failed to get image by ImageId: %s", vm.ImageId)
		}
	}

	// If still not found, use minimal information
	if imgErr != nil {
		log.Warn().Msgf("Failed to get source image info for VM %s (ImageId: %s, CspImageName: %s), using minimal information",
			vmId, vm.ImageId, vm.CspImageName)
		sourceImageInfo = model.ImageInfo{}
	} else {
		log.Debug().Msgf("Successfully retrieved source image info for VM %s", vmId)
	}

	// Create ImageInfo inheriting from source image
	// Use ConnectionConfig from VM (already contains all necessary information)
	tempImageInfo := model.ImageInfo{
		// CustomImage-specific fields
		ResourceType: model.StrCustomImage,
		CspImageId:   tempSpiderMyImageInfo.IId.SystemId,
		SourceVmUid:  vm.Uid,

		// Composite primary key fields (inherited from source image)
		Namespace:    nsId,
		ProviderName: vm.ConnectionConfig.ProviderName,
		CspImageName: tempSpiderMyImageInfo.IId.NameId, // Custom image's CSP name

		// Array field
		RegionList: []string{vm.Region.Region},

		// Identifiers
		Id:             snapshotName,
		Uid:            common.GenUid(),
		Name:           snapshotName,
		ConnectionName: vm.ConnectionName,
		InfraType:      sourceImageInfo.InfraType,

		// Time fields - Update creation date to custom image's creation date
		FetchedTime:  time.Now().Format(time.RFC3339),
		CreationDate: tempSpiderMyImageInfo.CreatedTime.Format(time.RFC3339),

		// Image type flags (inherited from source image)
		IsGPUImage:        sourceImageInfo.IsGPUImage,
		IsKubernetesImage: sourceImageInfo.IsKubernetesImage,
		IsBasicImage:      false, // Custom images are not basic images

		// OS information (inherited from source image)
		OSType:         sourceImageInfo.OSType,
		OSArchitecture: sourceImageInfo.OSArchitecture,
		OSPlatform:     sourceImageInfo.OSPlatform,
		OSDistribution: sourceImageInfo.OSDistribution,
		OSDiskType:     sourceImageInfo.OSDiskType,
		OSDiskSizeGB:   sourceImageInfo.OSDiskSizeGB,

		// Status
		ImageStatus: model.ImageStatus(tempSpiderMyImageInfo.Status),

		// Additional information
		Details:     tempSpiderMyImageInfo.KeyValueList,
		SystemLabel: "Created from VM snapshot",
		Description: fmt.Sprintf("Custom image from MCI/VM: %s/%s (Uid: %s): %s", mciId, vm.Name, vm.Uid, snapshotReq.Description),
	}

	result, err := resource.RegisterCustomImageWithInfo(nsId, tempImageInfo)
	if err != nil {
		err := fmt.Errorf("failed to find 'ns/mci/vm': %s/%s/%s", nsId, mciId, vmId)
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// Inspect DataDisks after creating VM snapshot
	// Disabled because: there is no difference in dataDisks before and after creating VM snapshot
	// inspect_result_after_snapshot, err := InspectResources(vm.ConnectionName, model.StrDataDisk)
	// dataDisks_after_snapshot := inspect_result_after_snapshot.Resources.OnTumblebug.Info
	// if err != nil {
	// 	err := fmt.Errorf("Failed to get current datadisks' info. \n")
	// 	log.Error().Err(err).Msg("")
	// 	return model.ImageInfo{}, err
	// }

	// difference_dataDisks := Difference_dataDisks(dataDisks_before_snapshot, dataDisks_after_snapshot)

	// // create 'n' dataDisks
	// for _, v := range difference_dataDisks {
	// 	tempDataDiskReq := model.DataDiskReq{
	// 		Name:           fmt.Sprintf("%s-%s", vm.Name, common.GenerateNewRandomString(5)),
	// 		ConnectionName: vm.ConnectionName,
	// 		CspResourceId:  v.CspResourceId,
	// 	}

	// 	_, err = resource.CreateDataDisk(nsId, &tempDataDiskReq, "register")
	// 	if err != nil {
	// 		err := fmt.Errorf("Failed to register the created dataDisk %s to TB. \n", v.CspResourceId)
	// 		log.Error().Err(err).Msg("")
	// 		continue
	// 	}
	// }

	return result, nil
}

// CreateMciSnapshot creates snapshots for the first running VM in each subgroup of an MCI in parallel
// Snapshots are created with provider-specific semaphores to safely limit concurrent requests per CSP
func CreateMciSnapshot(nsId string, mciId string, snapshotReq model.SnapshotReq) (model.MciSnapshotResult, error) {
	// Get MCI information
	mci, _, err := GetMciObject(nsId, mciId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get MCI object")
		return model.MciSnapshotResult{}, err
	}

	// Result structure
	result := model.MciSnapshotResult{
		MciId:     mciId,
		Namespace: nsId,
		Results:   []model.VmSnapshotResult{},
	}

	// Snapshot task structure with provider information
	type snapshotTask struct {
		subgroupId     string
		vmId           string
		vmName         string
		providerName   string
		connectionName string
	}

	// Find first running VM in each subgroup
	var tasks []snapshotTask
	subgroupMap := make(map[string]bool)

	for _, vm := range mci.Vm {
		// Skip if we already have a VM from this subgroup
		if subgroupMap[vm.SubGroupId] {
			continue
		}

		// Check if VM is running
		if vm.Status == model.StatusRunning {
			tasks = append(tasks, snapshotTask{
				subgroupId:     vm.SubGroupId,
				vmId:           vm.Id,
				vmName:         vm.Name,
				providerName:   vm.ConnectionConfig.ProviderName,
				connectionName: vm.ConnectionName,
			})
			subgroupMap[vm.SubGroupId] = true
		}
	}

	if len(tasks) == 0 {
		err := fmt.Errorf("no running VMs found in any subgroup")
		log.Error().Err(err).Msg("")
		return result, err
	}

	log.Info().Msgf("Creating snapshots for %d VMs (one per subgroup) with provider-specific semaphores", len(tasks))

	// Group tasks by provider
	providerGroups := make(map[string][]snapshotTask)
	for _, task := range tasks {
		providerName := task.providerName
		if providerName == "" {
			providerName = "unknown"
		}
		providerGroups[providerName] = append(providerGroups[providerName], task)
	}

	// Create provider-specific semaphores (limit concurrent snapshots per provider)
	const maxConcurrentPerProvider = 3 // Conservative limit for snapshot operations
	providerSemaphores := make(map[string]chan struct{})
	for providerName := range providerGroups {
		providerSemaphores[providerName] = make(chan struct{}, maxConcurrentPerProvider)
		log.Info().Msgf("Provider %s: %d VMs (max concurrent: %d)",
			providerName, len(providerGroups[providerName]), maxConcurrentPerProvider)
	}

	// Channel for collecting results
	resultChan := make(chan model.VmSnapshotResult, len(tasks))

	// Execute snapshots in parallel with provider-specific semaphores
	for _, task := range tasks {
		go func(t snapshotTask) {
			// Get provider name for semaphore
			providerName := t.providerName
			if providerName == "" {
				providerName = "unknown"
			}

			// Acquire semaphore for this provider
			semaphore := providerSemaphores[providerName]
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			log.Debug().Msgf("Creating snapshot for VM %s (Provider: %s, SubGroup: %s)",
				t.vmId, providerName, t.subgroupId)

			vmResult := model.VmSnapshotResult{
				SubGroupId: t.subgroupId,
				VmId:       t.vmId,
				VmName:     t.vmName,
			}

			// Generate unique snapshot name per VM
			vmSnapshotName := snapshotReq.Name
			if vmSnapshotName == "" {
				vmSnapshotName = fmt.Sprintf("%s-%s-%s", mciId, t.subgroupId, common.GenUid())
			} else {
				vmSnapshotName = fmt.Sprintf("%s-%s", vmSnapshotName, t.subgroupId)
			}

			vmSnapshotReq := model.SnapshotReq{
				Name:        vmSnapshotName,
				Description: fmt.Sprintf("%s (SubGroup: %s, Provider: %s)", snapshotReq.Description, t.subgroupId, providerName),
			}

			// Create snapshot
			imageInfo, err := CreateVmSnapshot(nsId, mciId, t.vmId, vmSnapshotReq)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to create snapshot for VM %s (Provider: %s)", t.vmId, providerName)
				vmResult.Status = "Failed"
				vmResult.Error = err.Error()
			} else {
				log.Info().Msgf("Successfully created snapshot for VM %s (Provider: %s): %s", t.vmId, providerName, imageInfo.Id)
				vmResult.Status = "Success"
				vmResult.ImageId = imageInfo.Id
				vmResult.ImageInfo = imageInfo
			}

			resultChan <- vmResult
		}(task)
	}

	// Collect results
	for i := 0; i < len(tasks); i++ {
		vmResult := <-resultChan
		result.Results = append(result.Results, vmResult)
		if vmResult.Status == "Success" {
			result.SuccessCount++
		} else {
			result.FailCount++
		}
	}
	close(resultChan)

	log.Info().Msgf("MCI snapshot completed: %d success, %d failed out of %d total",
		result.SuccessCount, result.FailCount, len(tasks))

	return result, nil
}

func Difference_dataDisks(a, b []model.ResourceOnTumblebugInfo) []model.ResourceOnTumblebugInfo {
	mb := make(map[interface{}]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []model.ResourceOnTumblebugInfo
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
