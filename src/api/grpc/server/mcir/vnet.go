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

// CreateVNet is to VNet 생성
func (s *MCIRService) CreateVNet(ctx context.Context, req *pb.TbVNetCreateRequest) (*pb.TbVNetInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateVNet()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbVNetReq
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateVNet()")
	}

	content, err := mcir.CreateVNet(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateVNet()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbVNetInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateVNet()")
	}

	resp := &pb.TbVNetInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListVNet is to VNet 목록
func (s *MCIRService) ListVNet(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbVNetInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListVNet()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListVNet()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbVNetInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListVNet()")
	}

	resp := &pb.ListTbVNetInfoResponse{Items: grpcObj}
	return resp, nil
}

// ListVNetId is to list vNet IDs
func (s *MCIRService) ListVNetId(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListVNetId()")

	resourceList, err := mcir.ListResourceId(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListVNetId()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListVNetId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// GetVNet is to VNet 조회
func (s *MCIRService) GetVNet(ctx context.Context, req *pb.ResourceQryRequest) (*pb.TbVNetInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.GetVNet()")

	res, err := mcir.GetResource(req.NsId, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetVNet()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbVNetInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetVNet()")
	}

	resp := &pb.TbVNetInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteVNet is to VNet 삭제
func (s *MCIRService) DeleteVNet(ctx context.Context, req *pb.ResourceQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteVNet()")

	err := mcir.DelResource(req.NsId, req.ResourceType, req.ResourceId, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteVNet()")
	}

	resp := &pb.MessageResponse{Message: "The " + req.ResourceType + " " + req.ResourceId + " has been deleted"}
	return resp, nil
}

// DeleteAllVNet is to VNet 전체 삭제
func (s *MCIRService) DeleteAllVNet(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllVNet()")

	err := mcir.DelAllResources(req.NsId, req.ResourceType, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllVNet()")
	}

	resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
