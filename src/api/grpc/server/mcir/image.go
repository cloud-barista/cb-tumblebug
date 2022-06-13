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

// CreateImageWithInfo is to Image 생성
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

// CreateImageWithID is to Image 생성
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

// ListImage is to Image 목록
func (s *MCIRService) ListImage(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListTbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListImage()")

	resourceList, err := mcir.ListResource(req.NsId, req.ResourceType, "", "")
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

// ListImageId is to list image IDs
func (s *MCIRService) ListImageId(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.ListIdResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.ListImageId()")

	resourceList, err := mcir.ListResourceId(req.NsId, req.ResourceType)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListImageId()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []string
	err = gc.CopySrcToDest(&resourceList, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.ListImageId()")
	}

	resp := &pb.ListIdResponse{IdList: grpcObj}
	return resp, nil
}

// GetImage is to Image 조회
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

// DeleteImage is to Image 삭제
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

// DeleteAllImage is to Image 전체 삭제
func (s *MCIRService) DeleteAllImage(ctx context.Context, req *pb.ResourceAllQryRequest) (*pb.IdListResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.DeleteAllImage()")

	content, err := mcir.DelAllResources(req.NsId, req.ResourceType, "", req.Force)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllImage()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.IdListResponse
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.DeleteAllImage()")
	}

	// resp := &pb.MessageResponse{Message: "All " + req.ResourceType + "s has been deleted"}
	resp := &grpcObj
	return resp, nil
}

// FetchImage is to Image 가져오기
func (s *MCIRService) FetchImage(ctx context.Context, req *pb.FetchImageQryRequest) (*pb.MessageResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.FetchImage()")

	var connConfigCount, imageCount uint
	var err error

	if req.ConnectionName == "!all" {
		connConfigCount, imageCount, err = mcir.FetchImagesForAllConnConfigs(req.NsId)
		if err != nil {
			return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FetchImage()")
		}
	} else {
		connConfigCount = 1
		imageCount, err = mcir.FetchImagesForConnConfig(req.ConnectionName, req.NsId)
		if err != nil {
			return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.FetchImage()")
		}
	}

	resp := &pb.MessageResponse{Message: "Fetched " + fmt.Sprint(imageCount) + " images (from " + fmt.Sprint(connConfigCount) + " connConfigs)"}
	return resp, nil
}

// SearchImage is to Image 검색
func (s *MCIRService) SearchImage(ctx context.Context, req *pb.SearchImageQryRequest) (*pb.ListTbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.SearchImage()")

	content, err := mcir.SearchImage(req.NsId, req.Keywords...)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.SearchImage()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj []*pb.TbImageInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.SearchImage()")
	}

	resp := &pb.ListTbImageInfoResponse{Items: grpcObj}
	return resp, nil
}

// UpdateImage is to update images
func (s *MCIRService) UpdateImage(ctx context.Context, req *pb.TbUpdateImageRequest) (*pb.TbImageInfoResponse, error) {
	logger := logger.NewLogger()

	logger.Debug("calling MCIRService.UpdateImage()")

	// GRPC 메시지에서 MCIR 객체로 복사
	var mcirObj mcir.TbImageInfo
	err := gc.CopySrcToDest(&req.Item, &mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateImage()")
	}

	content, err := mcir.UpdateImage(req.NsId, req.ImageId, mcirObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateImage()")
	}

	// MCIR 객체에서 GRPC 메시지로 복사
	var grpcObj pb.TbImageInfo
	err = gc.CopySrcToDest(&content, &grpcObj)
	if err != nil {
		return nil, gc.ConvGrpcStatusErr(err, "", "MCIRService.UpdateImage()")
	}

	resp := &pb.TbImageInfoResponse{Item: &grpcObj}
	return resp, nil
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
