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

// CreateCredential - Credential 생성
func (r *CIMRequest) CreateCredential() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CredentialInfo
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.CreateCredential(ctx, &pb.CredentialInfoRequest{Item: &item})
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListCredential -Credential 목록
func (r *CIMRequest) ListCredential() (string, error) {
	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListCredential(ctx, &pb.Empty{})

	if err != nil {
		return "", err
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetCredential - Credential 조회
func (r *CIMRequest) GetCredential() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CredentialQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.GetCredential(ctx, &item)
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteCredential - Credential 삭제
func (r *CIMRequest) DeleteCredential() (string, error) {
	// 입력데이터 검사
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// 입력데이터 언마샬링
	var item pb.CredentialQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// 서버에 요청
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err2 := r.Client.DeleteCredential(ctx, &item)
	if err2 != nil {
		return "", err2
	}

	// 결과값 마샬링
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
