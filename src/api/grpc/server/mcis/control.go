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

// CreateMcis - MCIS 생성
func (s *MCISService) CreateMcis(ctx context.Context, req *pb.TbMcisCreateRequest) (*pb.TbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.TbMcisReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	result, err := mcis.CorePostMcis(req.NsId, &mcisObj)
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

// ListMcis - MCIS 목록
func (s *MCISService) ListMcis(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.ListTbMcisInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ListMcis()")

	result, err := mcis.CoreGetAllMcis(req.NsId, req.Option)
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

// ControlMcis - MCIS 제어
func (s *MCISService) ControlMcis(ctx context.Context, req *pb.TbMcisActionRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.ControlMcis()")

	result, err := mcis.CoreGetMcisAction(req.NsId, req.McisId, req.Action)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.ControlMcis()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// GetMcisStatus - MCIS 상태 조회
func (s *MCISService) GetMcisStatus(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.TbMcisStatusInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetMcisStatus()")

	result, err := mcis.CoreGetMcisStatus(req.NsId, req.McisId)
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

// GetMcisInfo - MCIS 정보 조회
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

// DeleteMcis - MCIS 삭제
func (s *MCISService) DeleteMcis(ctx context.Context, req *pb.TbMcisQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteMcis()")

	err := mcis.DelMcis(req.NsId, req.McisId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteMcis()")
	}

	resp := &pb.MessageResponse{Message: "Deleting the MCIS info"}
	return resp, nil
}

// DeleteAllMcis - MCIS 전체 삭제
func (s *MCISService) DeleteAllMcis(ctx context.Context, req *pb.TbMcisAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteAllMcis()")

	result, err := mcis.CoreDelAllMcis(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteAllMcis()")
	}

	resp := &pb.MessageResponse{Message: result}
	return resp, nil
}

// CreateMcisVM - MCIS VM 생성
func (s *MCISService) CreateMcisVM(ctx context.Context, req *pb.TbVmCreateRequest) (*pb.TbVmInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CreateMcisVM()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.TbVmInfo
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcisVM()")
	}

	result, err := mcis.CorePostMcisVm(req.NsId, req.McisId, &mcisObj)
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

// ControlMcisVM - MCIS VM 제어
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

// GetMcisVMStatus - MCIS VM 상태 조회
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

// GetMcisVMInfo - MCIS VM 정보 조회
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

// DeleteMcisVM - MCIS VM 삭제
func (s *MCISService) DeleteMcisVM(ctx context.Context, req *pb.TbVmQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.DeleteMcisVM()")

	err := mcis.DelMcisVm(req.NsId, req.McisId, req.VmId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.DeleteMcisVM()")
	}

	resp := &pb.MessageResponse{Message: "Deleting the VM info"}
	return resp, nil
}

// RecommandMcis - MCIS 추천
func (s *MCISService) RecommandMcis(ctx context.Context, req *pb.McisRecommendCreateRequest) (*pb.McisRecommendInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.RecommandMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisRecommendReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommandMcis()")
	}

	result, err := mcis.CorePostMcisRecommand(req.NsId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CreateMcis()")
	}

	content := rest_mcis.RestPostMcisRecommandResponse{}
	content.Vm_recommend = result
	content.Placement_algo = mcisObj.Placement_algo
	content.Placement_param = mcisObj.Placement_param

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.McisRecommendInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.RecommandMcis()")
	}

	resp := &pb.McisRecommendInfoResponse{Item: &grpcObj}
	return resp, nil
}

// CmdMcis - MCIS 명령 실행
func (s *MCISService) CmdMcis(ctx context.Context, req *pb.McisCmdCreateRequest) (*pb.ListCmdMcisResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CmdMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	result, err := mcis.CorePostCmdMcis(req.NsId, req.McisId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.CmdMcisResult
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcis()")
	}

	resp := &pb.ListCmdMcisResponse{Items: grpcObj}
	return resp, nil
}

// CmdMcisVm - MCIS VM 명령 실행
func (s *MCISService) CmdMcisVm(ctx context.Context, req *pb.McisCmdVmCreateRequest) (*pb.StringResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.CmdMcisVm()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcisVm()")
	}

	result, err := mcis.CorePostCmdMcisVm(req.NsId, req.McisId, req.VmId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.CmdMcisVm()")
	}

	resp := &pb.StringResponse{Result: result}
	return resp, nil
}

// InstallAgentToMcis - MCIS Agent 설치
func (s *MCISService) InstallAgentToMcis(ctx context.Context, req *pb.McisCmdCreateRequest) (*pb.ListAgentInstallResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.InstallAgentToMcis()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj mcis.McisCmdReq
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallAgentToMcis()")
	}

	content, err := mcis.InstallAgentToMcis(req.NsId, req.McisId, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallAgentToMcis()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.ListAgentInstallResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.InstallAgentToMcis()")
	}

	return &grpcObj, nil
}

// GetBenchmark - Benchmark 조회
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
	var grpcObj []*pb.BenchmarkInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetBenchmark()")
	}

	resp := &pb.ListBenchmarkInfoResponse{Items: grpcObj}
	return resp, nil
}

// GetAllBenchmark - Benchmark 목록
func (s *MCISService) GetAllBenchmark(ctx context.Context, req *pb.BmQryAllRequest) (*pb.ListBenchmarkInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCISService.GetAllBenchmark()")

	// GRPC 메시지에서 MCIS 객체로 복사
	var mcisObj rest_mcis.RestGetAllBenchmarkRequest
	err := gc.CopySrcToDest(&req.Item, &mcisObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	result, err := mcis.CoreGetAllBenchmark(req.NsId, req.McisId, mcisObj.Host)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	// MCIS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.BenchmarkInfo
	err = gc.CopySrcToDest(&result, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCISService.GetAllBenchmark()")
	}

	resp := &pb.ListBenchmarkInfoResponse{Items: grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
