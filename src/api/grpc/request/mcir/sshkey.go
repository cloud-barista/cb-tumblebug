package mcir

import (
	"context"
	"errors"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateSshKey
func (r *MCIRRequest) CreateSshKey() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbSshKeyCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateSshKey(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListSshKey
func (r *MCIRRequest) ListSshKey() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListSshKey(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ListSshKeyId
func (r *MCIRRequest) ListSshKeyId() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListSshKeyId(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetSshKey
func (r *MCIRRequest) GetSshKey() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ResourceQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetSshKey(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteSshKey
func (r *MCIRRequest) DeleteSshKey() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ResourceQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteSshKey(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteAllSshKey
func (r *MCIRRequest) DeleteAllSshKey() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteAllSshKey(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
