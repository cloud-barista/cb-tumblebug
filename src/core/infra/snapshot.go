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
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	clientManager "github.com/cloud-barista/cb-tumblebug/src/core/common/client"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/resource"
	"github.com/rs/zerolog/log"
)

// mapSpiderToTumblebugImageStatus delegates to the shared implementation in the resource package
// to avoid duplicated logic. See resource.MapSpiderToTumblebugImageStatus for detailed mapping.
func mapSpiderToTumblebugImageStatus(spiderStatus string) model.ImageStatus {
	return resource.MapSpiderToTumblebugImageStatus(spiderStatus)
}

// CreateNodeSnapshot is func to create Node snapshot
func CreateNodeSnapshot(nsId string, infraId string, nodeId string, snapshotReq model.SnapshotReq) (model.ImageInfo, error) {
	node, err := GetNodeObject(nsId, infraId, nodeId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	snapshotName := snapshotReq.Name

	if snapshotName == "" {
		snapshotName = common.GenUid()
	}

	requestBody := model.SpiderMyImageReq{
		ConnectionName: node.ConnectionName,
		ReqInfo: struct {
			Name     string
			SourceNode string
		}{
			Name:     snapshotName,
			SourceNode: node.CspResourceName,
		},
	}

	// Inspect DataDisks before creating Node snapshot
	// Disabled because: there is no difference in dataDisks before and after creating Node snapshot
	// inspect_result_before_snapshot, err := InspectResources(node.ConnectionName, model.StrDataDisk)
	// dataDisks_before_snapshot := inspect_result_before_snapshot.Resources.OnTumblebug.Info
	// if err != nil {
	// 	err := fmt.Errorf("Failed to get current datadisks' info. \n")
	// 	log.Error().Err(err).Msg("")
	// 	return model.ImageInfo{}, err
	// }

	// Create Node snapshot using ExecuteHttpRequest
	var tempSpiderMyImageInfo model.SpiderMyImageInfo
	client := clientManager.NewHttpClient()
	client.SetTimeout(5 * time.Minute)
	url := fmt.Sprintf("%s/myimage", model.SpiderRestUrl)
	method := "POST"

	_, err = clientManager.ExecuteHttpRequest(
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

	// Get the source Node's image information to inherit properties
	// Try to get image using CspImageName with provider/region information
	var sourceImageInfo model.ImageInfo
	var imgErr error

	// First, try to get by composite primary key (ProviderName + CspImageName)
	if node.CspImageName != "" && node.ConnectionConfig.ProviderName != "" {
		sourceImageInfo, imgErr = resource.GetImageByPrimaryKey(model.SystemCommonNs, node.ConnectionConfig.ProviderName, node.CspImageName)
		if imgErr != nil {
			log.Debug().Err(imgErr).Msgf("Failed to get image by primary key (provider: %s, cspImageName: %s)", node.ConnectionConfig.ProviderName, node.CspImageName)
		}
	}

	// If not found, try using ImageId (TB image ID)
	if imgErr != nil && node.ImageId != "" {
		sourceImageInfo, imgErr = resource.GetImage(nsId, node.ImageId)
		if imgErr != nil {
			log.Debug().Err(imgErr).Msgf("Failed to get image by ImageId: %s", node.ImageId)
		}
	}

	// If still not found, use minimal information
	if imgErr != nil {
		log.Warn().Msgf("Failed to get source image info for Node %s (ImageId: %s, CspImageName: %s), using minimal information",
			nodeId, node.ImageId, node.CspImageName)
		sourceImageInfo = model.ImageInfo{}
	} else {
		log.Debug().Msgf("Successfully retrieved source image info for VM %s", nodeId)
	}

	commandHistory := []model.ImageSourceCommandHistory{}
	for _, cmd := range node.CommandStatus {
		commandHistory = append(commandHistory, model.ImageSourceCommandHistory{
			Index:           cmd.Index,
			CommandExecuted: cmd.CommandExecuted,
		})
	}

	// Create ImageInfo inheriting from source image
	// Use ConnectionConfig from Node (already contains all necessary information)
	tempImageInfo := model.ImageInfo{
		// CustomImage-specific fields
		ResourceType: model.StrCustomImage,
		CspImageId:   tempSpiderMyImageInfo.IId.SystemId,
		SourceNodeUid:  node.Uid,

		// Composite primary key fields (inherited from source image)
		Namespace:    nsId,
		ProviderName: node.ConnectionConfig.ProviderName,
		CspImageName: tempSpiderMyImageInfo.IId.NameId, // Custom image's CSP name

		// Array field
		RegionList: []string{node.Region.Region},

		// Identifiers
		Id:             snapshotName,
		Uid:            common.GenUid(),
		Name:           snapshotName,
		ConnectionName: node.ConnectionName,
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

		// Status - Use CB-Tumblebug's own status management
		// Map Spider's status to CB-Tumblebug's enhanced status
		ImageStatus: mapSpiderToTumblebugImageStatus(string(tempSpiderMyImageInfo.Status)),

		// Additional information
		Details:     tempSpiderMyImageInfo.KeyValueList,
		SystemLabel: "Created from Node snapshot",
		Description: fmt.Sprintf("Custom image from Infra/Node: %s/%s (Uid: %s): %s", infraId, node.Name, node.Uid, snapshotReq.Description),

		CommandHistory: commandHistory,
	}

	result, err := resource.RegisterCustomImageWithInfo(nsId, tempImageInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ImageInfo{}, err
	}

	// Inspect DataDisks after creating Node snapshot
	// Disabled because: there is no difference in dataDisks before and after creating Node snapshot
	// inspect_result_after_snapshot, err := InspectResources(node.ConnectionName, model.StrDataDisk)
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
	// 		Name:           fmt.Sprintf("%s-%s", node.Name, common.GenerateNewRandomString(5)),
	// 		ConnectionName: node.ConnectionName,
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

// CreateInfraSnapshot creates snapshots for the first running Node in each nodegroup of an Infra in parallel
// Snapshots are created with provider-specific semaphores to safely limit concurrent requests per CSP
func CreateInfraSnapshot(nsId string, infraId string, snapshotReq model.SnapshotReq) (model.InfraSnapshotResult, error) {
	// Get Infra information
	infra, _, err := GetInfraObject(nsId, infraId)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Infra object")
		return model.InfraSnapshotResult{}, err
	}

	// Result structure
	result := model.InfraSnapshotResult{
		InfraId:   infraId,
		Namespace: nsId,
		Results:   []model.NodeSnapshotResult{},
	}

	// Snapshot task structure with provider information
	type snapshotTask struct {
		nodegroupId    string
		nodeId           string
		nodeName         string
		providerName   string
		connectionName string
	}

	// Find first running Node in each nodegroup
	var tasks []snapshotTask
	nodegroupMap := make(map[string]bool)

	for _, node := range infra.Node {
		// Skip if we already have a Node from this nodegroup
		if nodegroupMap[node.NodeGroupId] {
			continue
		}

		// Check if Node is running
		if node.Status == model.StatusRunning {
			tasks = append(tasks, snapshotTask{
				nodegroupId:    node.NodeGroupId,
				nodeId:           node.Id,
				nodeName:         node.Name,
				providerName:   node.ConnectionConfig.ProviderName,
				connectionName: node.ConnectionName,
			})
			nodegroupMap[node.NodeGroupId] = true
		}
	}

	if len(tasks) == 0 {
		err := fmt.Errorf("no running VMs found in any nodegroup")
		log.Error().Err(err).Msg("")
		return result, err
	}

	log.Info().Msgf("Creating snapshots for %d VMs (one per nodegroup) with provider-specific semaphores", len(tasks))

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
	resultChan := make(chan model.NodeSnapshotResult, len(tasks))

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

			log.Debug().Msgf("Creating snapshot for VM %s (Provider: %s, NodeGroup: %s)",
				t.nodeId, providerName, t.nodegroupId)

			nodeResult := model.NodeSnapshotResult{
				NodeGroupId: t.nodegroupId,
				NodeId:        t.nodeId,
				NodeName:      t.nodeName,
			}

			// Generate unique snapshot name per VM
			nodeSnapshotName := snapshotReq.Name
			if nodeSnapshotName == "" {
				nodeSnapshotName = fmt.Sprintf("%s-%s-%s", infraId, t.nodegroupId, common.GenUid())
			} else {
				nodeSnapshotName = fmt.Sprintf("%s-%s", nodeSnapshotName, t.nodegroupId)
			}

			nodeSnapshotReq := model.SnapshotReq{
				Name:        nodeSnapshotName,
				Description: fmt.Sprintf("%s (NodeGroup: %s, Provider: %s)", snapshotReq.Description, t.nodegroupId, providerName),
			}

			// Create snapshot
			imageInfo, err := CreateNodeSnapshot(nsId, infraId, t.nodeId, nodeSnapshotReq)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to create snapshot for VM %s (Provider: %s)", t.nodeId, providerName)
				nodeResult.Status = "Failed"
				nodeResult.Error = err.Error()
			} else {
				log.Info().Msgf("Successfully created snapshot for VM %s (Provider: %s): %s", t.nodeId, providerName, imageInfo.Id)
				nodeResult.Status = "Success"
				nodeResult.ImageId = imageInfo.Id
				nodeResult.ImageInfo = imageInfo
			}

			resultChan <- nodeResult
		}(task)
	}

	// Collect results
	for i := 0; i < len(tasks); i++ {
		nodeResult := <-resultChan
		result.Results = append(result.Results, nodeResult)
		if nodeResult.Status == "Success" {
			result.SuccessCount++
		} else {
			result.FailCount++
		}
	}
	close(resultChan)

	log.Info().Msgf("Infra snapshot completed: %d success, %d failed out of %d total",
		result.SuccessCount, result.FailCount, len(tasks))

	return result, nil
}

// BuildAgnosticImage creates an Infra, executes post commands, creates snapshots, and optionally cleans up
// This is a complete workflow for building agnostic (CSP-independent) custom images
func BuildAgnosticImage(nsId string, req model.BuildAgnosticImageReq) (model.BuildAgnosticImageResult, error) {
	startTime := time.Now()

	result := model.BuildAgnosticImageResult{
		Namespace: nsId,
	}

	// Step 1: Set PolicyOnPartialFailure to "refine" for better error handling
	req.SourceInfraReq.PolicyOnPartialFailure = "refine"

	log.Info().Msgf("Starting BuildAgnosticImage workflow for Infra: %s", req.SourceInfraReq.Name)

	// Step 2: Create Infra with dynamic provisioning
	log.Info().Msg("Step 1/4: Creating Infra infrastructure...")
	reqId := common.GenUid() // Generate unique request ID
	ctx := common.WithRequestID(common.NewDefaultContext(), reqId)
	infraInfo, err := CreateInfraDynamic(ctx, nsId, &req.SourceInfraReq, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Infra")
		return result, fmt.Errorf("failed to create Infra: %w", err)
	}

	result.InfraId = infraInfo.Id
	result.InfraStatus = string(infraInfo.Status)
	log.Info().Msgf("Infra created successfully: %s (Status: %s)", infraInfo.Id, infraInfo.Status)

	// Step 3: Wait for Infra to be fully running
	// Check if there are any failed VMs after "refine" policy
	log.Info().Msg("Step 2/4: Verifying Infra status...")

	// Get updated Infra status
	infraStatus, err := GetInfraStatus(nsId, infraInfo.Id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Infra status")
		// Don't fail here, try to continue with snapshot creation
	} else {
		log.Info().Msgf("Infra status - Total VMs: %d, Running: %d, Failed: %d",
			infraStatus.StatusCount.CountTotal,
			infraStatus.StatusCount.CountRunning,
			infraStatus.StatusCount.CountFailed)
	}

	// Check if we have any running VMs
	if infraStatus != nil && infraStatus.StatusCount.CountRunning == 0 {
		err := fmt.Errorf("no running VMs found in Infra after provisioning")
		log.Error().Err(err).Msg("")
		return result, err
	}

	// Step 4: Create snapshots from the Infra (one per nodegroup)
	log.Info().Msg("Step 3/4: Creating snapshots from running VMs...")
	snapshotResult, err := CreateInfraSnapshot(nsId, infraInfo.Id, req.SnapshotReq)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Infra snapshots")
		// Even if snapshot fails, we should still cleanup if requested
		if req.CleanupInfraAfterSnapshot {
			log.Info().Msg("Attempting to cleanup Infra despite snapshot failure...")
			_, cleanupErr := DelInfra(nsId, infraInfo.Id, model.ActionTerminate)
			if cleanupErr != nil {
				log.Error().Err(cleanupErr).Msg("Failed to cleanup Infra")
			} else {
				result.InfraCleanedUp = true
			}
		}
		return result, fmt.Errorf("failed to create snapshots: %w", err)
	}

	result.SnapshotResult = snapshotResult
	log.Info().Msgf("Snapshots created: %d success, %d failed",
		snapshotResult.SuccessCount, snapshotResult.FailCount)

	// Step 5: Wait for custom images to become Available before cleanup
	if req.CleanupInfraAfterSnapshot && snapshotResult.SuccessCount > 0 {
		log.Info().Msg("Step 4a/5: Waiting for custom images to become Available...")
		// Wait a short moment before starting checks
		initiatingInterval := 15 * time.Second
		time.Sleep(initiatingInterval)

		// Collect all successfully created image IDs
		imageIds := make([]string, 0, snapshotResult.SuccessCount)
		for _, nodeResult := range snapshotResult.Results {
			if nodeResult.Status == "Success" && nodeResult.ImageId != "" {
				imageIds = append(imageIds, nodeResult.ImageId)
			}
		}

		// Wait for all images to become Available
		maxWaitTime := resource.CustomImageCreationTimeout // Use shared timeout constant
		checkInterval := 10 * time.Second                  // Check every 10 seconds
		startWait := time.Now()

		allAvailable := false
		for time.Since(startWait) < maxWaitTime {
			allAvailable = true
			unavailableImages := []string{}

			for _, imageId := range imageIds {
				image, err := resource.GetImage(nsId, imageId)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to get image status for %s", imageId)
					unavailableImages = append(unavailableImages, imageId)
					allAvailable = false
					continue
				}

				if image.ImageStatus != model.ImageAvailable {
					log.Debug().Msgf("Image %s status: %s (waiting for Available)", imageId, image.ImageStatus)
					unavailableImages = append(unavailableImages, imageId)
					allAvailable = false
				}
			}

			if allAvailable {
				log.Info().Msgf("All %d custom images are now Available", len(imageIds))
				break
			}

			log.Debug().Msgf("Waiting for %d images to become Available... (elapsed: %s)",
				len(unavailableImages), time.Since(startWait).Round(time.Second))
			time.Sleep(checkInterval)
		}

		if !allAvailable {
			log.Warn().Msgf("Timeout waiting for images to become Available after %s", maxWaitTime)
			result.Message = fmt.Sprintf("Warning: Some images may not be Available yet after %s wait time", maxWaitTime)
		}
	}

	// Step 6: Cleanup Infra if requested
	if req.CleanupInfraAfterSnapshot {
		log.Info().Msg("Step 5/5: Cleaning up Infra after snapshot creation...")
		// Use DelInfra with "terminate" option (refine → terminate → delete)
		// This is the proper way instead of using "force" option
		_, err := DelInfra(nsId, infraInfo.Id, model.ActionTerminate)
		if err != nil {
			log.Error().Err(err).Msg("Failed to cleanup Infra")
			// Don't fail the entire operation if cleanup fails
			if result.Message == "" {
				result.Message = fmt.Sprintf("Successfully created %d custom images, but Infra cleanup failed: %v",
					snapshotResult.SuccessCount, err)
			} else {
				result.Message += fmt.Sprintf(" and Infra cleanup failed: %v", err)
			}
		} else {
			result.InfraCleanedUp = true
			result.InfraStatus = "Terminated"
			log.Info().Msg("Infra cleaned up successfully")
		}
	} else {
		log.Info().Msg("Step 5/5: Skipping Infra cleanup (CleanupInfraAfterSnapshot=false)")
	}

	// Calculate total duration
	duration := time.Since(startTime)
	result.TotalDuration = duration.Round(time.Second).String()

	// Set final message
	if result.Message == "" {
		if req.CleanupInfraAfterSnapshot {
			result.Message = fmt.Sprintf("Successfully created %d custom images from Infra %s and cleaned up infrastructure",
				snapshotResult.SuccessCount, infraInfo.Id)
		} else {
			result.Message = fmt.Sprintf("Successfully created %d custom images from Infra %s (infrastructure preserved)",
				snapshotResult.SuccessCount, infraInfo.Id)
		}
	}

	log.Info().Msgf("BuildAgnosticImage workflow completed in %s", result.TotalDuration)
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
