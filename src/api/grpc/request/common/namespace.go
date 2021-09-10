package common

import (
	"context"
	"errors"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateNS is to Namespace 생성
func (r *NSRequest) CreateNS() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.NsReq
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateNS(ctx, &pb.NSCreateRequest{Item: &item})
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListNS is to Namespace 목록
func (r *NSRequest) ListNS() (string, error) {
	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListNS(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ListNSId
func (r *NSRequest) ListNSId() (string, error) {
	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListNSId(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetNS is to Namespace 조회
func (r *NSRequest) GetNS() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.NSQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetNS(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteNS is to Namespace 삭제
func (r *NSRequest) DeleteNS() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.NSQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteNS(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteAllNS is to Namespace 전체 삭제
func (r *NSRequest) DeleteAllNS() (string, error) {
	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteAllNS(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// CheckNS is to Namespace 체크
func (r *NSRequest) CheckNS() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.NSQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CheckNS(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
