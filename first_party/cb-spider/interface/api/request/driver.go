// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package request

import (
	"context"
	"errors"

	gc "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/common"
	pb "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/stub/cbspider"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateCloudDriver - Cloud Driver 생성
func (r *CIMRequest) CreateCloudDriver() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CloudDriverInfo
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.CreateCloudDriver(ctx, &pb.CloudDriverInfoRequest{Item: &item})
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListCloudDriver -Cloud Driver 목록
func (r *CIMRequest) ListCloudDriver() (string, error) {
	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListCloudDriver(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetCloudDriver - Cloud Driver 조회
func (r *CIMRequest) GetCloudDriver() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CloudDriverQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.GetCloudDriver(ctx, &item)
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteCloudDriver - Cloud Driver 삭제
func (r *CIMRequest) DeleteCloudDriver() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CloudDriverQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.DeleteCloudDriver(ctx, &item)
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
