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

// ListConnConfig - Connection Config 목록
func (s *UTILITYService) ListConnConfig(ctx context.Context, req *pb.Empty) (*pb.ListConnConfigResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.ListConnConfig()")

	content, err := common.GetConnConfigList()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListConnConfig()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListConnConfigResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListConnConfig()")
	}

	return &grpcObj, nil
}

// GetConnConfig - Connection Config 조회
func (s *UTILITYService) GetConnConfig(ctx context.Context, req *pb.ConnConfigQryRequest) (*pb.ConnConfigResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.GetConnConfig()")

	content, err := common.GetConnConfig(req.ConnConfigName)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetConnConfig()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ConnConfig
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetConnConfig()")
	}

	resp := &pb.ConnConfigResponse{Item: &grpcObj}
	return resp, nil
}

// ListRegion - Region 목록
func (s *UTILITYService) ListRegion(ctx context.Context, req *pb.Empty) (*pb.ListRegionResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.ListRegion()")

	content, err := common.GetRegionList()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListRegion()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListRegionResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.ListRegion()")
	}

	return &grpcObj, nil
}

// GetRegion - Region 조회
func (s *UTILITYService) GetRegion(ctx context.Context, req *pb.RegionQryRequest) (*pb.RegionResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.GetRegion()")

	content, err := common.GetRegion(req.RegionName)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetRegion()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.Region
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetRegion()")
	}

	resp := &pb.RegionResponse{Item: &grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
