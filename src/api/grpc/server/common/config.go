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

// CreateConfig
func (s *UtilityService) CreateConfig(ctx context.Context, req *pb.ConfigCreateRequest) (*pb.ConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.CreateConfig()")

	// Copy gRPC message to 'common' object
	var commObj common.ConfigReq
	err := gc.CopySrcToDest(&req.Item, &commObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.CreateConfig()")
	}

	content, err := common.UpdateConfig(&commObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.CreateConfig()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.ConfigInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.CreateConfig()")
	}

	resp := &pb.ConfigInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListConfig
func (s *UtilityService) ListConfig(ctx context.Context, req *pb.Empty) (*pb.ListConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.ListConfig()")

	configList, err := common.ListConfig()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListConfig()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj []*pb.ConfigInfo
	err = gc.CopySrcToDest(&configList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListConfig()")
	}

	resp := &pb.ListConfigInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetConfig
func (s *UtilityService) GetConfig(ctx context.Context, req *pb.ConfigQryRequest) (*pb.ConfigInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.GetConfig()")

	res, err := common.GetConfig(req.ConfigId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetConfig()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.ConfigInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetConfig()")
	}

	resp := &pb.ConfigInfoResponse{Item: &grpcObj}
	return resp, nil
}

// InitConfig
func (s *UtilityService) InitConfig(ctx context.Context, req *pb.ConfigQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.InitConfig()")

	err := common.InitConfig(req.ConfigId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InitConfig()")
	}

	resp := &pb.MessageResponse{Message: "The config " + req.ConfigId + " has been initialized."}
	return resp, nil
}

// InitAllConfig
func (s *UtilityService) InitAllConfig(ctx context.Context, req *pb.Empty) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.InitAllConfig()")

	err := common.InitAllConfig()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InitAllConfig()")
	}

	resp := &pb.MessageResponse{Message: "All configs have been initialized."}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
