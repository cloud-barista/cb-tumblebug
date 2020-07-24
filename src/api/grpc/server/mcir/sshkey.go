package mcir

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateSshKey - KeyPair 생성
func (s *MCIRService) CreateSshKey(ctx context.Context, req *pb.TbSshKeyCreateRequest) (*pb.TbSshKeyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateSshKey()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSshKeyReq
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSshKey()")
	}

	content, err := mcir.CreateSshKey(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSshKey()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSshKeyInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSshKey()")
	}

	resp := &pb.TbSshKeyInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListSshKey - KeyPair 목록
func (s *MCIRService) ListSshKey(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbSshKeyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListSshKey()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSshKey()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSshKeyInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSshKey()")
	}

	resp := &pb.ListTbSshKeyInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetSshKey - KeyPair 조회
func (s *MCIRService) GetSshKey(ctx context.Context, req *pb.ResourceQryRequest) (*pb.TbSshKeyInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.GetSshKey()")

	res, err := mcir.GetResource(req.NsId, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSshKey()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSshKeyInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSshKey()")
	}

	resp := &pb.TbSshKeyInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteSshKey - KeyPair 삭제
func (s *MCIRService) DeleteSshKey(ctx context.Context, req *pb.ResourceQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteSshKey()")

	err := mcir.DelResource(req.NsId, req.ResourceType, req.ResourceId, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteSshKey()")
	}

	resp := &pb.MessageResponse{Message: "The " + req.ResourceType + " " + req.ResourceId + " has been deleted"}
	return resp, nil
}

// DeleteAllSshKey - KeyPair 전체 삭제
func (s *MCIRService) DeleteAllSshKey(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllSshKey()")

	err := mcir.DelAllResources(req.NsId, req.ResourceType, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllSshKey()")
	}

	resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
