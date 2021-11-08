package mcir

import (
	"context"
	"fmt"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"

	"github.com/cloud-barista/cb-tumblebug/src/core/mcir"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// CreateSpecWithInfo is to Spec 생성
func (s *MCIRService) CreateSpecWithInfo(ctx context.Context, req *pb.TbSpecInfoRequest) (*pb.TbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateSpecWithInfo()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSpecInfo
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithInfo()")
	}

	content, err := mcir.RegisterSpecWithInfo(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithInfo()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSpecInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithInfo()")
	}

	resp := &pb.TbSpecInfoResponse{Item: &grpcObj}
	return resp, nil
}

// CreateSpecWithSpecName is to Spec 생성
func (s *MCIRService) CreateSpecWithSpecName(ctx context.Context, req *pb.TbSpecCreateRequest) (*pb.TbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.CreateSpecWithSpecName()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSpecReq
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithSpecName()")
	}

	content, err := mcir.RegisterSpecWithCspSpecName(req.NsId, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithSpecName()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSpecInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.CreateSpecWithSpecName()")
	}

	resp := &pb.TbSpecInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ListSpec is to Spec 목록
func (s *MCIRService) ListSpec(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListSpec()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSpec()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSpecInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSpec()")
	}

	resp := &pb.ListTbSpecInfoResponse{Items: grpcObj}
	return resp, nil
}

// ListSpecId is to list spec IDs
func (s *MCIRService) ListSpecId(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListSpecId()")

	resourceIdList, err := mcir.ListResourceId(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSpecId()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&resourceIdList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListSpecId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// GetSpec is to Spec 조회
func (s *MCIRService) GetSpec(ctx context.Context, req *pb.ResourceQryRequest) (*pb.TbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.GetSpec()")

	res, err := mcir.GetResource(req.NsId, req.ResourceType, req.ResourceId)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSpec()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSpecInfo
	err = gc.CopySrcToDest(&res, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.GetSpec()")
	}

	resp := &pb.TbSpecInfoResponse{Item: &grpcObj}
	return resp, nil
}

// DeleteSpec is to Spec 삭제
func (s *MCIRService) DeleteSpec(ctx context.Context, req *pb.ResourceQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteSpec()")

	err := mcir.DelResource(req.NsId, req.ResourceType, req.ResourceId, req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteSpec()")
	}

	resp := &pb.MessageResponse{Message: "The " + req.ResourceType + " " + req.ResourceId + " has been deleted"}
	return resp, nil
}

// DeleteAllSpec is to Spec 전체 삭제
func (s *MCIRService) DeleteAllSpec(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllSpec()")

	_, err := mcir.DelAllResources(req.NsId, req.ResourceType, "", req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllSpec()")
	}

	resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	return resp, nil
}

// FetchSpec is to Spec 가져오기
func (s *MCIRService) FetchSpec(ctx context.Context, req *pb.FetchSpecQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.FetchSpec()")

	// connConfigCount, specCount, err := mcir.FetchSpecsForAllConnConfigs(req.NsId)
	// if err != nil {
	// 	return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FetchSpec()")
	// }

	var connConfigCount, specCount uint
	var err error

	if req.ConnectionName == "!all" {
		connConfigCount, specCount, err = mcir.FetchSpecsForAllConnConfigs(req.NsId)
		if err != nil {
			return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FetchSpec()")
		}
	} else {
		connConfigCount = 1
		specCount, err = mcir.FetchSpecsForConnConfig(req.ConnectionName, req.NsId)
		if err != nil {
			return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FetchSpec()")
		}
	}

	resp := &pb.MessageResponse{Message: "Fetched " + fmt.Sprint(specCount) + " specs (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return resp, nil
}

// FilterSpec is to filter specs
func (s *MCIRService) FilterSpec(ctx context.Context, req *pb.TbSpecInfoRequest) (*pb.ListTbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.FilterSpec()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSpecInfo
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpec()")
	}

	resourceList, err := mcir.FilterSpecs(req.NsId, mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpec()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSpecInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpec()")
	}

	resp := &pb.ListTbSpecInfoResponse{Items: grpcObj}
	return resp, nil
}

// FilterSpecsByRange is filter specs by range
func (s *MCIRService) FilterSpecsByRange(ctx context.Context, req *pb.FilterSpecsByRangeRequest) (*pb.ListTbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.FilterSpecsByRange()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var filter mcir.FilterSpecsByRangeRequest
	err := gc.CopySrcToDest(&req.Filter, &filter)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpecsByRange()")
	}

	resourceList, err := mcir.FilterSpecsByRange(req.NsId, filter)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpecsByRange()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSpecInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FilterSpecsByRange()")
	}

	resp := &pb.ListTbSpecInfoResponse{Items: grpcObj}
	return resp, nil
}

// SortSpecs is to sort specs
func (s *MCIRService) SortSpecs(ctx context.Context, req *pb.SortSpecsRequest) (*pb.ListTbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.SortSpecs()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var specList []mcir.TbSpecInfo
	err := gc.CopySrcToDest(&req.Items, &specList)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.SortSpecs()")
	}

	resourceList, err := mcir.SortSpecs(specList, req.OrderBy, req.Direction)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.SortSpecs()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbSpecInfo
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.SortSpecs()")
	}

	resp := &pb.ListTbSpecInfoResponse{Items: grpcObj}
	return resp, nil
}

// UpdateSpec is to update specs
func (s *MCIRService) UpdateSpec(ctx context.Context, req *pb.TbUpdateSpecRequest) (*pb.TbSpecInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.UpdateSpec()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbSpecInfo
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateSpec()")
	}

	content, err := mcir.UpdateSpec(req.NsId, req.SpecId, mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateSpec()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbSpecInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateSpec()")
	}

	resp := &pb.TbSpecInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
