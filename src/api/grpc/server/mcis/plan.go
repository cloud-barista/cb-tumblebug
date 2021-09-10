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

// RecommendVM is to MCIS VM 추천
func (s *MCISService) RecommendVM(ctx context.Context, req *pb.McisRecommendVmCreateRequest) (*pb.ListTbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.RecommendVM()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.DeploymentPlan
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommendVM()")
	}

	content, err := mcis.RecommendVm(req.NsId, mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommendVM()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSpecInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommendVM()")
	}

	resp := &pb.ListTbSpecInfoResponse{Items: grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
