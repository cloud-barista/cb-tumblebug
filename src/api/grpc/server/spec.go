package server

import (
	"context"
	"log"

	"github.com/gogo/protobuf/types"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf"
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcir"
)

func (s *server) RegisterSpecWithCspSpecName(ctx context.Context, req *pb.RegisterSpecWithCspSpecNameWrapper) (*pb.TbSpecInfo, error) {
	log.Printf("RegisterSpecWithCspSpecName Received: %v", req.TbSpecReq.Name)

	var tbSpecReq mcir.TbSpecReq
	err := common.CopySrcToDest(&req.TbSpecReq, &tbSpecReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithCspSpecName()")
	}

	tbSpecInfo, err := mcir.RegisterSpecWithCspSpecName(req.NsId, &tbSpecReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithCspSpecName()")
	}

	var grpcSpecInfo pb.TbSpecInfo
	err = common.CopySrcToDest(&tbSpecInfo, &grpcSpecInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithCspSpecName()")
	}

	return &grpcSpecInfo, nil
}

func (s *server) RegisterSpecWithInfo(ctx context.Context, req *pb.RegisterSpecWithInfoWrapper) (*pb.TbSpecInfo, error) {
	log.Printf("RegisterSpecWithInfo Received: %v", req.TbSpecInfo.Name)

	var tbSpecInfoA mcir.TbSpecInfo
	err := common.CopySrcToDest(&req.TbSpecInfo, &tbSpecInfoA)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithInfo()")
	}

	tbSpecInfoB, err := mcir.RegisterSpecWithInfo(req.NsId, &tbSpecInfoA)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithInfo()")
	}

	var grpcSpecInfo pb.TbSpecInfo
	err = common.CopySrcToDest(&tbSpecInfoB, &grpcSpecInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "RegisterSpecWithInfo()")
	}

	return &grpcSpecInfo, nil
}

func (s *server) DelAllSpecs(ctx context.Context, req *pb.DelAllResourcesWrapper) (*types.Empty, error) {
	log.Printf("DelAllSpecs Received")

	err := mcir.DelAllResources(req.NsId, "spec", req.ForceFlag)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelAllSpecs()")
	}
	return &types.Empty{}, nil
}

func (s *server) DelSpec(ctx context.Context, req *pb.DelResourceWrapper) (*types.Empty, error) {
	log.Printf("grpc.DelSpec Received: %v", req.ResourceId)

	err := mcir.DelResource(req.NsId, "spec", req.ResourceId, req.ForceFlag)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelSpec()")
	}
	return &types.Empty{}, nil
}

func (s *server) GetSpec(ctx context.Context, req *pb.GetResourceWrapper) (*pb.TbSpecInfo, error) {
	log.Printf("GetSpec Received: %v", req.ResourceId)

	tbSpecInfo, err := mcir.GetResource(req.NsId, "spec", req.ResourceId)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetSpec()")
	}

	var grpcSpecInfo pb.TbSpecInfo
	err = common.CopySrcToDest(&tbSpecInfo, &grpcSpecInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetSpec()")
	}

	return &grpcSpecInfo, nil
}

func (s *server) ListSpec(ctx context.Context, req *pb.NsId) (*pb.TbSpecInfoList, error) {
	log.Printf("ListSpec Received")

	tbSpecInfoList, err := mcir.ListResource(req.Id, "spec")
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListSpec()")
	}

	var grpcObj []*pb.TbSpecInfo
	err = common.CopySrcToDest(&tbSpecInfoList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListSpec()")
	}

	resp := &pb.TbSpecInfoList{TbSpecInfos: grpcObj}
	return resp, nil
}

func (s *server) ListSpecId(ctx context.Context, req *pb.NsId) (*pb.ResourceIdList, error) {
	log.Printf("ListSpecId Received")

	tbSpecIdList := mcir.ListResourceId(req.Id, "spec")
	var grpcObj []string
	err := common.CopySrcToDest(&tbSpecIdList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListSpecId()")
	}

	resp := &pb.ResourceIdList{Items: grpcObj}
	return resp, nil
}
