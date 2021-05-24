package common

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateConfig - Config 생성
func (s *UTILITYService) CreateConfig(ctx context.Context, req *pb.ConfigCreateRequest) (*pb.ConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.CreateConfig()")

	// GRPC 메시지에서 COMMON 객체로 복사
	var commObj common.ConfigReq
	err := gc.CopySrcToDest(&req.Item, &commObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.CreateConfig()")
	}

	content, err := common.UpdateConfig(&commObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.CreateConfig()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ConfigInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.CreateConfig()")
	}

	resp := &pb.ConfigInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListConfig - Config 목록
func (s *UTILITYService) ListConfig(ctx context.Context, req *pb.Empty) (*pb.ListConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.ListConfig()")

	configList, err := common.ListConfig()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListConfig()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.ConfigInfo
	err = gc.CopySrcToDest(&configList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListConfig()")
	}

	resp := &pb.ListConfigInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetConfig - Config 조회
func (s *UTILITYService) GetConfig(ctx context.Context, req *pb.ConfigQryRequest) (*pb.ConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.GetConfig()")

	res, err := common.GetConfig(req.ConfigId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetConfig()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ConfigInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetConfig()")
	}

	resp := &pb.ConfigInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteAllConfig - Config 전체 삭제
func (s *UTILITYService) DeleteAllConfig(ctx context.Context, req *pb.Empty) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.DeleteAllConfig()")

	err := common.DelAllConfig()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.DeleteAllConfig()")
	}

	resp := &pb.MessageResponse{Message: "All configs has been deleted"}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
