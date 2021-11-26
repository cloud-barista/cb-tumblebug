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

// CreateSecurityGroup is to Security Group 생성
func (s *MCIRService) CreateSecurityGroup(ctx context.Context, req *pb.TbSecurityGroupCreateRequest) (*pb.TbSecurityGroupInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateSecurityGroup()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSecurityGroupReq
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSecurityGroup()")
	}

	content, err := mcir.CreateSecurityGroup(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSecurityGroup()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSecurityGroupInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSecurityGroup()")
	}

	resp := &pb.TbSecurityGroupInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListSecurityGroup is to Security Group 목록
func (s *MCIRService) ListSecurityGroup(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbSecurityGroupInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListSecurityGroup()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSecurityGroup()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSecurityGroupInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSecurityGroup()")
	}

	resp := &pb.ListTbSecurityGroupInfoResponse{Items: grpcObj}
	return resp, nil
}

// ListSecurityGroupId is to list security group IDs
func (s *MCIRService) ListSecurityGroupId(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListSecurityGroupId()")

	resourceList, err := mcir.ListResourceId(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSecurityGroupId()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSecurityGroupId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// GetSecurityGroup is to Security Group 조회
func (s *MCIRService) GetSecurityGroup(ctx context.Context, req *pb.ResourceQryRequest) (*pb.TbSecurityGroupInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.GetSecurityGroup()")

	res, err := mcir.GetResource(req.NsId, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSecurityGroup()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSecurityGroupInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSecurityGroup()")
	}

	resp := &pb.TbSecurityGroupInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteSecurityGroup is to Security Group 삭제
func (s *MCIRService) DeleteSecurityGroup(ctx context.Context, req *pb.ResourceQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteSecurityGroup()")

	err := mcir.DelResource(req.NsId, req.ResourceType, req.ResourceId, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteSecurityGroup()")
	}

	resp := &pb.MessageResponse{Message: "The " + req.ResourceType + " " + req.ResourceId + " has been deleted"}
	return resp, nil
}

// DeleteAllSecurityGroup is to Security Group 전체 삭제
func (s *MCIRService) DeleteAllSecurityGroup(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.IdListResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllSecurityGroup()")

	content, err := mcir.DelAllResources(req.NsId, req.ResourceType, "", req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllSecurityGroup()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.IdListResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllSecurityGroup()")
	}

	// resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	resp := &grpcObj
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
