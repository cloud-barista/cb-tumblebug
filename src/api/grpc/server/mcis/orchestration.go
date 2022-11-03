package mcis

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateMcisPolicy is to Policy 생성
func (s *MCISService) CreateMcisPolicy(ctx context.Context, req *pb.McisPolicyCreateRequest) (*pb.McisPolicyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcisPolicy()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisPolicyReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisPolicy()")
	}

	content, err := mcis.CreateMcisPolicy(req.NsId, req.McisId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisPolicy()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.McisPolicyInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisPolicy()")
	}

	resp := &pb.McisPolicyInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListMcisPolicy is to Policy 목록
func (s *MCISService) ListMcisPolicy(ctx context.Context, req *pb.McisPolicyAllQryRequest) (*pb.ListMcisPolicyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcisPolicy()")

	result, err := mcis.GetAllMcisPolicyObject(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisPolicy()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.McisPolicyInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisPolicy()")
	}

	resp := &pb.ListMcisPolicyInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetMcisPolicy is to Policy 조회
func (s *MCISService) GetMcisPolicy(ctx context.Context, req *pb.McisPolicyQryRequest) (*pb.McisPolicyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisPolicy()")

	result, err := mcis.GetMcisPolicyObject(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisPolicy()")
	}

	if result.Id == "" {
		return nil, gc.NewGrpcStatusErr("Failed to find McisPolicyObject : "+req.McisId, "", "MCISService.GetMcisPolicy()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.McisPolicyInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisPolicy()")
	}

	resp := &pb.McisPolicyInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteMcisPolicy is to Policy 삭제
func (s *MCISService) DeleteMcisPolicy(ctx context.Context, req *pb.McisPolicyQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteMcisPolicy()")

	err := mcis.DelMcisPolicy(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteMcisPolicy()")
	}

	resp := &pb.MessageResponse{Message: "Deleted the MCIS Policy info"}
	return resp, nil
}

// DeleteAllMcisPolicy is to Policy 전체 삭제
func (s *MCISService) DeleteAllMcisPolicy(ctx context.Context, req *pb.McisPolicyAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteAllMcisPolicy()")

	result, err := mcis.DelAllMcisPolicy(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteAllMcisPolicy()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
