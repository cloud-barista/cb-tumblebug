package common

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
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

// InspectMcirResources - MCIR 리소스 점검
func (s *UTILITYService) InspectMcirResources(ctx context.Context, req *pb.InspectQryRequest) (*pb.InspectMcirInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.InspectMcirResources()")

	content, err := mcir.InspectResources(req.ConnectionName, req.Type)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.InspectMcirResources()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.InspectMcirInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.InspectMcirResources()")
	}

	resp := &pb.InspectMcirInfoResponse{Item: &grpcObj}
	return resp, nil
}

// InspectVmResources - VM 리소스 점검
func (s *UTILITYService) InspectVmResources(ctx context.Context, req *pb.InspectQryRequest) (*pb.InspectVmInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.InspectVmResources()")

	content, err := mcis.InspectVMs(req.ConnectionName)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.InspectVmResources()")
	}

	// COMMON 객체에서 GRPC 메시지로 복사
	var grpcObj pb.InspectVmInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.InspectVmResources()")
	}

	resp := &pb.InspectVmInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListObject - 객체 목록
func (s *UTILITYService) ListObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.ListObjectInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.ListObject()")

	content := common.GetObjectList(req.Key)

	resp := &pb.ListObjectInfoResponse{Items: content}
	return resp, nil
}

// GetObject - 객체 조회
func (s *UTILITYService) GetObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.ObjectInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.GetObject()")

	content, err := common.GetObjectValue(req.Key)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "UTILITYService.GetObject()")
	}

	resp := &pb.ObjectInfoResponse{Item: content}
	return resp, nil
}

// DeleteObject - 객체 삭제
func (s *UTILITYService) DeleteObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.DeleteObject()")

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

// DeleteAllObject - 객체 전체 삭제
func (s *UTILITYService) DeleteAllObject(ctx context.Context, req *pb.ObjectQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling UTILITYService.DeleteAllObject()")

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
