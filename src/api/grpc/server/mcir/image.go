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

// CreateImageWithInfo - Image 생성
func (s *MCIRService) CreateImageWithInfo(ctx context.Context, req *pb.TbImageInfoRequest) (*pb.TbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateImageWithInfo()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbImageInfo
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithInfo()")
	}

	content, err := mcir.RegisterImageWithInfo(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithInfo()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbImageInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithInfo()")
	}

	resp := &pb.TbImageInfoResponse{Item: &grpcObj}
	return resp, nil
}

// CreateImageWithID - Image 생성
func (s *MCIRService) CreateImageWithID(ctx context.Context, req *pb.TbImageCreateRequest) (*pb.TbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateImageWithID()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbImageReq
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithID()")
	}

	content, err := mcir.RegisterImageWithId(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithID()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbImageInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateImageWithID()")
	}

	resp := &pb.TbImageInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListImage - Image 목록
func (s *MCIRService) ListImage(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListImage()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListImage()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbImageInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListImage()")
	}

	resp := &pb.ListTbImageInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetImage - Image 조회
func (s *MCIRService) GetImage(ctx context.Context, req *pb.ResourceQryRequest) (*pb.TbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.GetImage()")

	res, err := mcir.GetResource(req.NsId, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetImage()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbImageInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetImage()")
	}

	resp := &pb.TbImageInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteImage - Image 삭제
func (s *MCIRService) DeleteImage(ctx context.Context, req *pb.ResourceQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteImage()")

	err := mcir.DelResource(req.NsId, req.ResourceType, req.ResourceId, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteImage()")
	}

	resp := &pb.MessageResponse{Message: "The " + req.ResourceType + " " + req.ResourceId + " has been deleted"}
	return resp, nil
}

// DeleteAllImage - Image 전체 삭제
func (s *MCIRService) DeleteAllImage(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllImage()")

	err := mcir.DelAllResources(req.NsId, req.ResourceType, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllImage()")
	}

	resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
