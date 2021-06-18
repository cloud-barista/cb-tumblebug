package common

import (
	"context"
	"encoding/json"
	"errors"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ListConnConfig
func (r *UtilityRequest) ListConnConfig() (string, error) {
	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListConnConfig(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetConnConfig
func (r *UtilityRequest) GetConnConfig() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ConnConfigQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetConnConfig(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListRegion
func (r *UtilityRequest) ListRegion() (string, error) {
	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListRegion(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetRegion
func (r *UtilityRequest) GetRegion() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.RegionQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetRegion(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// InspectMcirResources
func (r *UtilityRequest) InspectMcirResources() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.InspectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.InspectMcirResources(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// InspectVmResources
func (r *UtilityRequest) InspectVmResources() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.InspectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.InspectVmResources(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListObject
func (r *UtilityRequest) ListObject() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ObjectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListObject(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetObject
func (r *UtilityRequest) GetObject() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ObjectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetObject(ctx, &item)
	if err != nil {
		return "", err
	}

	var contentJSON map[string]interface{}
	json.Unmarshal([]byte(resp.Item), &contentJSON)

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &contentJSON)
}

// DeleteObject
func (r *UtilityRequest) DeleteObject() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ObjectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteObject(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteAllObject
func (r *UtilityRequest) DeleteAllObject() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ObjectQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteAllObject(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
