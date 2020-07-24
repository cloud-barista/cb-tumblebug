package mcis

import (
	"context"

	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CheckMcis - MCIS 체크
func (s *MCISService) CheckMcis(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.ExistsResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CheckMcis()")

	exists, err := mcis.CheckMcis(req.NsId, req.McisId)
	if err != nil {
		logger.Debug(err)
	}

	resp := &pb.ExistsResponse{Exists: exists}
	return resp, nil
}

// CheckVm - MCIS VM 체크
func (s *MCISService) CheckVm(ctx context.Context, req *pb.TbVmQryRequest) (*pb.ExistsResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CheckVm()")

	exists, err := mcis.CheckVm(req.NsId, req.McisId, req.VmId)
	if err != nil {
		logger.Debug(err)
	}

	resp := &pb.ExistsResponse{Exists: exists}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
