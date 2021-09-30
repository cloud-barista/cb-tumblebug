package common

import (
	"context"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateNS is to Namespace 생성
func (s *NSService) CreateNS(ctx context.Context, req *pb.NSCreateRequest) (*pb.NSInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.CreateNS()")

	// GRPC 메시지에서 NS 객체로 복사
	var nsObj common.NsReq
	err := gc.CopySrcToDest(&req.Item, &nsObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.CreateNS()")
	}

	content, err := common.CreateNs(&nsObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.CreateNS()")
	}

	// NS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.NSInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.CreateNS()")
	}

	resp := &pb.NSInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListNS is to Namespace 목록
func (s *NSService) ListNS(ctx context.Context, req *pb.Empty) (*pb.ListNSInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.ListNS()")

	nsList, err := common.ListNs()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.ListNS()")
	}

	// NS 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.NSInfo
	err = gc.CopySrcToDest(&nsList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.ListNS()")
	}

	resp := &pb.ListNSInfoResponse{Items: grpcObj}
	return resp, nil
}

// ListNSId
func (s *NSService) ListNSId(ctx context.Context, req *pb.Empty) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.ListNSId()")

	nsList, err := common.ListNsId()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.ListNSId()")
	}

	// NS 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&nsList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.ListNSId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// GetNS is to Namespace 조회
func (s *NSService) GetNS(ctx context.Context, req *pb.NSQryRequest) (*pb.NSInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.GetNS()")

	res, err := common.GetNs(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.GetNS()")
	}

	// NS 객체에서 GRPC 메시지로 복사
	var grpcObj pb.NSInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.GetNS()")
	}

	resp := &pb.NSInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteNS is to Namespace 삭제
func (s *NSService) DeleteNS(ctx context.Context, req *pb.NSQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.DeleteNS()")

	err := common.DelNs(req.NsId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.DeleteNS()")
	}

	resp := &pb.MessageResponse{Message: "The ns has been deleted"}
	return resp, nil
}

// DeleteAllNS is to Namespace 전체 삭제
func (s *NSService) DeleteAllNS(ctx context.Context, req *pb.Empty) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.DeleteAllNS()")

	err := common.DelAllNs()
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "NSService.DeleteAllNS()")
	}

	resp := &pb.MessageResponse{Message: "All namespaces has been deleted"}
	return resp, nil
}

// CheckNS is to Namespace 체크
func (s *NSService) CheckNS(ctx context.Context, req *pb.NSQryRequest) (*pb.ExistsResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling NSService.CheckNS()")

	err := common.CheckString(req.NsId)
	if err != nil {
		resp := &pb.ExistsResponse{Exists: false}
		common.CBLog.Error(err)
		return resp, err
	}
	exists, err := common.CheckNs(req.NsId)
	if err != nil {
		logger.Debug(err)
	}

	resp := &pb.ExistsResponse{Exists: exists}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
