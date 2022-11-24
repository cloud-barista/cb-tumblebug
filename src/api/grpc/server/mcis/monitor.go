package mcis

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	common "github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// InstallMonitorAgentToMcis is to MCIS Monitor Agent 설치
func (s *MCISService) InstallMonitorAgentToMcis(ctx context.Context, req *pb.McisCmdCreateRequest) (*pb.ListAgentInstallResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.InstallMonitorAgentToMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallMonitorAgentToMcis()")
	}

	// mcisTmpSystemLabel := mcis.DefaultSystemLabel
	content, err := mcis.InstallMonitorAgentToMcis(req.NsId, req.McisId, common.StrMCIS, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallMonitorAgentToMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListAgentInstallResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallMonitorAgentToMcis()")
	}

	return &grpcObj, nil
}

// GetMonitorData is to MCIS Monitor 정보 조회
func (s *MCISService) GetMonitorData(ctx context.Context, req *pb.MonitorQryRequest) (*pb.MonitorResultSimpleResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMonitorData()")

	content, err := mcis.GetMonitoringData(req.NsId, req.McisId, req.Metric)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMonitorData()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.MonResultSimpleInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMonitorData()")
	}

	resp := &pb.MonitorResultSimpleResponse{Item: &grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
