package mcis

import (
	"context"
	"errors"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CheckMcis - MCIS 체크
func (r *MCISRequest) CheckMcis() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.TbMcisQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CheckMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// CheckVm - MCIS VM 체크
func (r *MCISRequest) CheckVm() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.TbVmQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CheckVm(ctx, &item)
	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
