package common

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ListConnConfig
func (s *UtilityService) ListConnConfig(ctx context.Context, req *pb.Empty) (*pb.ListConnConfigResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.ListConnConfig()")

	content, err := common.GetConnConfigList()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListConnConfig()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.ListConnConfigResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListConnConfig()")
	}

	return &grpcObj, nil
}

// GetConnConfig
func (s *UtilityService) GetConnConfig(ctx context.Context, req *pb.ConnConfigQryRequest) (*pb.ConnConfigResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.GetConnConfig()")

	content, err := common.GetConnConfig(req.ConnConfigName)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetConnConfig()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.ConnConfig
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetConnConfig()")
	}

	resp := &pb.ConnConfigResponse{Item: &grpcObj}
	return resp, nil
}

// ListRegion
func (s *UtilityService) ListRegion(ctx context.Context, req *pb.Empty) (*pb.ListRegionResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.ListRegion()")

	content, err := common.GetRegionList()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListRegion()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.ListRegionResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.ListRegion()")
	}

	return &grpcObj, nil
}

// GetRegion
func (s *UtilityService) GetRegion(ctx context.Context, req *pb.RegionQryRequest) (*pb.RegionResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.GetRegion()")

	content, err := common.GetRegion(req.RegionName)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetRegion()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.Region
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetRegion()")
	}

	resp := &pb.RegionResponse{Item: &grpcObj}
	return resp, nil
}

// InspectMcirResources
func (s *UtilityService) InspectMcirResources(ctx context.Context, req *pb.InspectQryRequest) (*pb.InspectMcirInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.InspectMcirResources()")

	content, err := mcis.InspectResources(req.ConnectionName, req.Type)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InspectMcirResources()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.InspectMcirInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InspectMcirResources()")
	}

	resp := &pb.InspectMcirInfoResponse{Item: &grpcObj}
	return resp, nil
}

// InspectVmResources
func (s *UtilityService) InspectVmResources(ctx context.Context, req *pb.InspectQryRequest) (*pb.InspectVmInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.InspectVmResources()")

	content, err := mcis.InspectResources(req.ConnectionName, common.StrVM)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InspectVmResources()")
	}

	// Copy 'common' object to gRPC message
	var grpcObj pb.InspectVmInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.InspectVmResources()")
	}

	resp := &pb.InspectVmInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListObject
func (s *UtilityService) ListObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.ListObjectInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.ListObject()")

	content := common.GetObjectList(req.Key)

	resp := &pb.ListObjectInfoResponse{Items: content}
	return resp, nil
}

// GetObject
func (s *UtilityService) GetObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.ObjectInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.GetObject()")

	content, err := common.GetObjectValue(req.Key)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UtilityService.GetObject()")
	}

	resp := &pb.ObjectInfoResponse{Item: content}
	return resp, nil
}

// DeleteObject
func (s *UtilityService) DeleteObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.DeleteObject()")

	content, err := common.GetObjectValue(req.Key)
	if err != nil || content == "" {
		resp := &pb.MessageResponse{Message: "Cannot find [" + req.Key + "] object"}
		return resp, nil
	}

	err = common.DeleteObject(req.Key)
	if err != nil {
		resp := &pb.MessageResponse{Message: "Cannot delete [" + req.Key + "] object"}
		return resp, nil
	}

	resp := &pb.MessageResponse{Message: "The object has been deleted"}
	return resp, nil
}

// DeleteAllObject
func (s *UtilityService) DeleteAllObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UtilityService.DeleteAllObject()")

	err := common.DeleteObjects(req.Key)
	if err != nil {
		resp := &pb.MessageResponse{Message: "Cannot delete  objects"}
		return resp, nil
	}

	resp := &pb.MessageResponse{Message: "Objects have been deleted"}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
