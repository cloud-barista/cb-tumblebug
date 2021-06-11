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

// CreateSecurityGroup - Security Group 생성
func (r *MCIRRequest) CreateSecurityGroup() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.TbSecurityGroupCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateSecurityGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListSecurityGroup - Security Group 목록
func (r *MCIRRequest) ListSecurityGroup() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListSecurityGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ListSecurityGroupId
func (r *MCIRRequest) ListSecurityGroupId() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListSecurityGroupId(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetSecurityGroup - Security Group 조회
func (r *MCIRRequest) GetSecurityGroup() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.ResourceQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetSecurityGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteSecurityGroup - Security Group 삭제
func (r *MCIRRequest) DeleteSecurityGroup() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.ResourceQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteSecurityGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteAllSecurityGroup - Security Group 전체 삭제
func (r *MCIRRequest) DeleteAllSecurityGroup() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.ResourceAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteAllSecurityGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
