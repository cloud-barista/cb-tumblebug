package server

import (
	"context"
	"log"

	"github.com/gogo/protobuf/types"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf"
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

func (s *server) RegisterImageWithId(ctx context.Context, req *pb.RegisterImageWithIdWrapper) (*pb.TbImageInfo, error) {
	log.Printf("RegisterImageWithId Received: %v", req.TbImageReq.Name)

	var tbImageReq mcir.TbImageReq
	err := common.CopySrcToDest(&req.TbImageReq, &tbImageReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithId()")
	}

	tbImageInfo, err := mcir.RegisterImageWithId(req.NsId, &tbImageReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithId()")
	}

	var grpcImageInfo pb.TbImageInfo
	err = common.CopySrcToDest(&tbImageInfo, &grpcImageInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithId()")
	}

	return &grpcImageInfo, nil
}

func (s *server) RegisterImageWithInfo(ctx context.Context, req *pb.RegisterImageWithInfoWrapper) (*pb.TbImageInfo, error) {
	log.Printf("RegisterImageWithInfo Received: %v", req.TbImageInfo.Name)

	var tbImageInfoA mcir.TbImageInfo
	err := common.CopySrcToDest(&req.TbImageInfo, &tbImageInfoA)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithInfo()")
	}

	tbImageInfoB, err := mcir.RegisterImageWithInfo(req.NsId, &tbImageInfoA)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithInfo()")
	}

	var grpcImageInfo pb.TbImageInfo
	err = common.CopySrcToDest(&tbImageInfoB, &grpcImageInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterImageWithInfo()")
	}

	return &grpcImageInfo, nil
}

func (s *server) DelAllImages(ctx context.Context, req *pb.DelAllResourcesWrapper) (*types.Empty, error) {
	log.Printf("DelAllImages Received")

	err := mcir.DelAllResources(req.NsId, "image", req.ForceFlag)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelAllImages()")
	}
	return &types.Empty{}, nil
}

func (s *server) DelImage(ctx context.Context, req *pb.DelResourceWrapper) (*types.Empty, error) {
	log.Printf("grpc.DelImage Received: %v", req.ResourceId)

	err := mcir.DelResource(req.NsId, "image", req.ResourceId, req.ForceFlag)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelImage()")
	}
	return &types.Empty{}, nil
}

func (s *server) GetImage(ctx context.Context, req *pb.GetResourceWrapper) (*pb.TbImageInfo, error) {
	log.Printf("GetImage Received: %v", req.ResourceId)

	tbImageInfo, err := mcir.GetResource(req.NsId, "image", req.ResourceId)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetImage()")
	}

	var grpcImageInfo pb.TbImageInfo
	err = common.CopySrcToDest(&tbImageInfo, &grpcImageInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetImage()")
	}

	return &grpcImageInfo, nil
}

func (s *server) ListImage(ctx context.Context, req *pb.NsId) (*pb.TbImageInfoList, error) {
	log.Printf("ListImage Received")

	tbImageInfoList, err := mcir.ListResource(req.Id, "image")
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListImage()")
	}

	var grpcObj []*pb.TbImageInfo
	err = common.CopySrcToDest(&tbImageInfoList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListImage()")
	}

	resp := &pb.TbImageInfoList{TbImageInfos: grpcObj}
	return resp, nil
}

func (s *server) ListImageId(ctx context.Context, req *pb.NsId) (*pb.ResourceIdList, error) {
	log.Printf("ListImageId Received")

	tbImageIdList := mcir.ListResourceId(req.Id, "image")
	var grpcObj []string
	err := common.CopySrcToDest(&tbImageIdList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListImageId()")
	}

	resp := &pb.ResourceIdList{Items: grpcObj}
	return resp, nil
}
