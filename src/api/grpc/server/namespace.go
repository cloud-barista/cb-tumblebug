package server

import (
	"context"
	"log"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/gogo/protobuf/types"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf"
)

func (s *server) CheckNs(ctx context.Context, in *pb.NsId) (*pb.BooleanResponse, error) {
	log.Printf("CheckNs Received: %v", in.Id)
	//	return &pb.NsInfo{Name: in.Name, Description: "CB-TB gRPC PB test"}, nil

	res, err := common.CheckNs(in.Id)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "CheckNs()")
	}

	var pbBool pb.BooleanResponse
	pbBool.Exists = res
	return &pbBool, nil
}

func (s *server) CreateNs(ctx context.Context, in *pb.NsInfo) (*pb.NsInfo, error) {
	log.Printf("CreateNs Received: %v", in.Name)
	//	return &pb.NsInfo{Name: in.Name, Description: "CB-TB gRPC PB test"}, nil

	var tbNsReq common.NsInfo
	err := common.CopySrcToDest(&in, &tbNsReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "CreateNs()")
	}

	tbNsInfo, err := common.CreateNs(&tbNsReq)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "CreateNs()")
	}

	var pbNsInfo pb.NsInfo
	err = common.CopySrcToDest(&tbNsInfo, &pbNsInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "CreateNs()")
	}

	return &pbNsInfo, nil
}

func (s *server) DelAllNs(ctx context.Context, req *types.Empty) (*types.Empty, error) {
	log.Printf("DelAllNs Received")

	err := common.DelAllNs()
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelAllNs()")
	}
	return &types.Empty{}, nil
}

func (s *server) DelNs(ctx context.Context, in *pb.NsId) (*types.Empty, error) {
	log.Printf("grpc.DelNs Received: %v", in.Id)

	err := common.DelNs(in.Id)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "DelNs()")
	}
	return &types.Empty{}, nil
}

func (s *server) GetNs(ctx context.Context, in *pb.NsId) (*pb.NsInfo, error) {
	log.Printf("GetNs Received: %v", in.Id)

	tbNsInfo, err := common.GetNs(in.Id)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetNs()")
	}

	var pbNsInfo pb.NsInfo
	err = common.CopySrcToDest(&tbNsInfo, &pbNsInfo)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "GetNs()")
	}

	return &pbNsInfo, nil
}

func (s *server) ListNs(ctx context.Context, req *types.Empty) (*pb.NsInfoList, error) {
	log.Printf("ListNs Received")

	tbNsInfoList, err := common.ListNs()
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListNs()")
	}

	//var pbNsInfoList pb.NsInfoList
	var grpcObj []*pb.NsInfo
	err = common.CopySrcToDest(&tbNsInfoList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListNs()")
	}

	resp := &pb.NsInfoList{Items: grpcObj}
	return resp, nil
}

func (s *server) ListNsId(ctx context.Context, req *types.Empty) (*pb.NsIdList, error) {
	log.Printf("ListNsId Received")

	tbNsIdList := common.ListNsId()
	/*
		if err != nil {
			//common.CBLog.Error(err)
			return nil, common.ConvGrpcStatusErr(err, "", "ListNsId()")
		}
	*/

	//var pbNsIdList pb.NsIdList
	//var grpcObj []*pb.NsId
	var grpcObj []string
	err := common.CopySrcToDest(&tbNsIdList, &grpcObj)
	if err != nil {
		//common.CBLog.Error(err)
		return nil, common.ConvGrpcStatusErr(err, "", "ListNsId()")
	}

	resp := &pb.NsIdList{Items: grpcObj}
	return resp, nil
}
