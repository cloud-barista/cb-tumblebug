package mcis

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	rest_mcis "github.com/cloud-barista/cb-tumblebug/src/api/rest/server/mcis"
	"github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateMcis is to MCIS 생성
func (s *MCISService) CreateMcis(ctx context.Context, req *pb.TbMcisCreateRequest) (*pb.TbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.TbMcisReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	result, err := mcis.CreateMcis(req.NsId, &mcisObj, "create")
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbMcisInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	resp := &pb.TbMcisInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListMcis is to MCIS 목록
func (s *MCISService) ListMcis(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.ListTbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcis()")

	result, err := mcis.CoreGetAllMcis(req.NsId, "status")
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbMcisInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcis()")
	}

	resp := &pb.ListTbMcisInfoResponse{Items: grpcObj}
	return resp, nil
}

// ListMcisId
func (s *MCISService) ListMcisId(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcisId()")

	result, err := mcis.ListMcisId(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisId()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// ControlMcis is to MCIS 제어
func (s *MCISService) ControlMcis(ctx context.Context, req *pb.TbMcisActionRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ControlMcis()")

	result, err := mcis.HandleMcisAction(req.NsId, req.McisId, req.Action, false)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ControlMcis()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// ListMcisStatus is to MCIS 상태 목록
func (s *MCISService) ListMcisStatus(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.ListTbMcisStatusInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcisStatus()")

	result, err := mcis.GetMcisStatusAll(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisStatus()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.McisStatusInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisStatus()")
	}

	resp := &pb.ListTbMcisStatusInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetMcisStatus is to MCIS 상태 조회
func (s *MCISService) GetMcisStatus(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.TbMcisStatusInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisStatus()")

	result, err := mcis.GetMcisStatus(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisStatus()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.McisStatusInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisStatus()")
	}

	resp := &pb.TbMcisStatusInfoResponse{Item: &grpcObj}
	return resp, nil
}

// GetMcisInfo is to MCIS 정보 조회
func (s *MCISService) GetMcisInfo(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.TbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisInfo()")

	result, err := mcis.GetMcisInfo(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisInfo()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbMcisInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisInfo()")
	}

	resp := &pb.TbMcisInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListMcisVmId
func (s *MCISService) ListMcisVmId(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcisVmId()")

	result, err := mcis.ListVmId(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisVmId()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ListMcisVmId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// DeleteMcis is to MCIS 삭제
func (s *MCISService) DeleteMcis(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteMcis()")

	_, err := mcis.DelMcis(req.NsId, req.McisId, req.Option)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteMcis()")
	}

	resp := &pb.MessageResponse{Message: "Deleted the MCIS " + req.McisId}
	return resp, nil
}

// DeleteAllMcis is to MCIS 전체 삭제
func (s *MCISService) DeleteAllMcis(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteAllMcis()")

	result, err := mcis.DelAllMcis(req.NsId, "")
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteAllMcis()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// CreateMcisVM is to MCIS VM 생성
func (s *MCISService) CreateMcisVM(ctx context.Context, req *pb.TbVmCreateRequest) (*pb.TbVmInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcisVM()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.TbVmInfo
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisVM()")
	}

	result, err := mcis.CreateMcisVm(req.NsId, req.McisId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisVM()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbVmInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisVM()")
	}

	resp := &pb.TbVmInfoResponse{Item: &grpcObj}
	return resp, nil
}

// CreateMcisSubGroup is to MCIS VM 그룹 생성
func (s *MCISService) CreateMcisSubGroup(ctx context.Context, req *pb.TbSubGroupCreateRequest) (*pb.TbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcisSubGroup()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.TbVmReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisSubGroup()")
	}

	result, err := mcis.CreateMcisGroupVm(req.NsId, req.McisId, &mcisObj, true)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisSubGroup()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbMcisInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisSubGroup()")
	}

	resp := &pb.TbMcisInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ControlMcisVM is to MCIS VM 제어
func (s *MCISService) ControlMcisVM(ctx context.Context, req *pb.TbVmActionRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ControlMcisVM()")

	result, err := mcis.CoreGetMcisVmAction(req.NsId, req.McisId, req.VmId, req.Action)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ControlMcisVM()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// GetMcisVMStatus is to MCIS VM 상태 조회
func (s *MCISService) GetMcisVMStatus(ctx context.Context, req *pb.TbVmQryRequest) (*pb.TbVmStatusInfoesponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisVMStatus()")

	result, err := mcis.CoreGetMcisVmStatus(req.NsId, req.McisId, req.VmId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisVMStatus()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbVmStatusInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisVMStatus()")
	}

	resp := &pb.TbVmStatusInfoesponse{Item: &grpcObj}
	return resp, nil
}

// GetMcisVMInfo is to MCIS VM 정보 조회
func (s *MCISService) GetMcisVMInfo(ctx context.Context, req *pb.TbVmQryRequest) (*pb.TbVmInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisVMInfo()")

	result, err := mcis.CoreGetMcisVmInfo(req.NsId, req.McisId, req.VmId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisVMInfo()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbVmInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetMcisVMInfo()")
	}

	resp := &pb.TbVmInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteMcisVM is to MCIS VM 삭제
func (s *MCISService) DeleteMcisVM(ctx context.Context, req *pb.TbVmQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteMcisVM()")

	err := mcis.DelMcisVm(req.NsId, req.McisId, req.VmId, "")
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteMcisVM()")
	}

	resp := &pb.MessageResponse{Message: "Deleted the VM info"}
	return resp, nil
}

// RecommendMcis is to MCIS 추천
func (s *MCISService) RecommendMcis(ctx context.Context, req *pb.McisRecommendCreateRequest) (*pb.McisRecommendInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.RecommendMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisRecommendReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommendMcis()")
	}

	result, err := mcis.CorePostMcisRecommend(req.NsId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	content := rest_mcis.RestPostMcisRecommendResponse{}
	content.VmRecommend = result
	content.PlacementAlgo = mcisObj.PlacementAlgo
	content.PlacementParam = mcisObj.PlacementParam

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.McisRecommendInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommendMcis()")
	}

	resp := &pb.McisRecommendInfoResponse{Item: &grpcObj}
	return resp, nil
}

// CmdMcis is to MCIS 명령 실행
func (s *MCISService) CmdMcis(ctx context.Context, req *pb.McisCmdCreateRequest) (*pb.ListCmdMcisResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CmdMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	result, err := mcis.RemoteCommandToMcis(req.NsId, req.McisId, "", &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.CmdMcisResult
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	for _, v := range grpcObj {
		v.McisId = req.McisId
	}

	resp := &pb.ListCmdMcisResponse{Items: grpcObj}
	return resp, nil
}

// CmdMcisVm is to MCIS VM 명령 실행
func (s *MCISService) CmdMcisVm(ctx context.Context, req *pb.McisCmdVmCreateRequest) (*pb.StringResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CmdMcisVm()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcisVm()")
	}

	result, err := mcis.RemoteCommandToMcisVm(req.NsId, req.McisId, req.VmId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcisVm()")
	}

	resp := &pb.StringResponse{Result: result}
	return resp, nil
}

// InstallBenchmarkAgentToMcis is to MCIS Agent 설치
func (s *MCISService) InstallBenchmarkAgentToMcis(ctx context.Context, req *pb.McisCmdCreateRequest) (*pb.ListAgentInstallResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.InstallBenchmarkAgentToMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallBenchmarkAgentToMcis()")
	}

	content, err := mcis.InstallBenchmarkAgentToMcis(req.NsId, req.McisId, &mcisObj, "")
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallBenchmarkAgentToMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListAgentInstallResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallBenchmarkAgentToMcis()")
	}

	return &grpcObj, nil
}

// GetBenchmark is to Benchmark 조회
func (s *MCISService) GetBenchmark(ctx context.Context, req *pb.BmQryRequest) (*pb.ListBenchmarkInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetBenchmark()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj rest_mcis.RestGetBenchmarkRequest
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetBenchmark()")
	}

	result, err := mcis.CoreGetBenchmark(req.NsId, req.McisId, req.Action, mcisObj.Host)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetBenchmark()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListBenchmarkInfoResponse
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetBenchmark()")
	}

	return &grpcObj, nil
}

// GetAllBenchmark is to Benchmark 목록
func (s *MCISService) GetAllBenchmark(ctx context.Context, req *pb.BmQryAllRequest) (*pb.ListBenchmarkInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetAllBenchmark()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj rest_mcis.RestGetAllBenchmarkRequest
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	result, err := mcis.RunAllBenchmarks(req.NsId, req.McisId, mcisObj.Host)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListBenchmarkInfoResponse
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	return &grpcObj, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
