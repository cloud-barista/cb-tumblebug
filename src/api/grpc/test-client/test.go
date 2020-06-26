package main

import (
	"context"
	"log"
	"os"
	"time"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf"
	"github.com/gogo/protobuf/types"
	"google.golang.org/grpc"
)

const (
	address     = "localhost:50051"
	defaultName = "gRPC-test"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	nsClient := pb.NewNsClient(conn)
	imageClient := pb.NewImageClient(conn)
	specClient := pb.NewSpecClient(conn)

	// Contact the server and print out its response.
	name := defaultName
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// CheckNs
	log.Printf("")
	log.Printf("CheckNs()")
	check, err := nsClient.CheckNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("CheckNs failed: %v", err)
	}
	if check.Exists == true {
		log.Printf("CheckNs success; The namespace " + name + " exists.")
	} else {
		log.Printf("CheckNs success; The namespace " + name + " does not exist.")
	}

	// CreateNs
	log.Printf("")
	log.Printf("CreateNs()")
	nsGrpcResult, err := nsClient.CreateNs(ctx, &pb.NsReq{Name: name})
	if err != nil {
		log.Printf("CreateNS failed: %v", err)
	} else {
		log.Printf("CreateNS success: %s", nsGrpcResult.GetName())
	}

	// CheckNs
	log.Printf("")
	log.Printf("CheckNs()")
	check, err = nsClient.CheckNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("CheckNs failed: %v", err)
	}
	if check.Exists == true {
		log.Printf("CheckNs success; The namespace " + name + " exists.")
	} else {
		log.Printf("CheckNs success; The namespace " + name + " does not exist.")
	}

	// GetNs
	log.Printf("")
	log.Printf("GetNs()")
	nsGrpcResult, err = nsClient.GetNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("GetNs failed: %v", err)
	} else {
		log.Printf("GetNS success: %s", nsGrpcResult.GetName())
	}

	// ListNs
	log.Printf("")
	log.Printf("ListNs()")
	grpcNsList, err := nsClient.ListNs(ctx, &types.Empty{})
	if err != nil {
		log.Fatalf("ListNs failed: %v", err)
	} else {
		log.Printf("ListNS success")
		for _, v := range grpcNsList.Items {
			log.Printf("%s", v.GetName())
		}
	}

	// ListNsId
	log.Printf("")
	log.Printf("ListNsId()")
	grpcNsIdList, err := nsClient.ListNsId(ctx, &types.Empty{})
	if err != nil {
		log.Fatalf("ListNsId failed: %v", err)
	} else {
		log.Printf("ListNSId success")
		for _, v := range grpcNsIdList.Items {
			log.Printf("%s", v)
		}
	}

	// DelNs
	log.Printf("")
	log.Printf("DelNs()")
	_, err = nsClient.DelNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("DelNs failed: %v", err)
	} else {
		log.Printf("DelNS success: %s", name)
	}

	// DelNs
	log.Printf("")
	log.Printf("DelNs()")
	_, err = nsClient.DelNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Printf("DelNs failed: %v", err)
	} else {
		log.Printf("DelNS success: %s", name)
	}

	// GetNs
	log.Printf("")
	log.Printf("GetNs()")
	nsGrpcResult, err = nsClient.GetNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Printf("GetNs failed: %v", err)
	} else {
		log.Printf("GetNS success: %s", nsGrpcResult.GetName())
	}

	// ListNs
	log.Printf("")
	log.Printf("ListNs()")
	grpcNsList, err = nsClient.ListNs(ctx, &types.Empty{})
	if err != nil {
		log.Fatalf("ListNs failed: %v", err)
	} else {
		log.Printf("ListNS success")
		for _, v := range grpcNsList.Items {
			log.Printf("%s", v.GetName())
		}
	}

	// ListNsId
	log.Printf("")
	log.Printf("ListNsId()")
	grpcNsIdList, err = nsClient.ListNsId(ctx, &types.Empty{})
	if err != nil {
		log.Fatalf("ListNsId failed: %v", err)
	} else {
		log.Printf("ListNSId success")
		for _, v := range grpcNsIdList.Items {
			log.Printf("%s", v)
		}
	}

	// CreateNs
	log.Printf("")
	log.Printf("CreateNs()")
	nsGrpcResult, err = nsClient.CreateNs(ctx, &pb.NsReq{Name: name})
	if err != nil {
		log.Printf("CreateNS failed: %v", err)
	} else {
		log.Printf("CreateNS success: %s", nsGrpcResult.GetName())
	}

	// ListImage
	log.Printf("")
	log.Printf("ListImage()")
	grpcImageList, err := imageClient.ListImage(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListImage failed: %v", err)
	} else {
		log.Printf("ListImage success")
		for _, v := range grpcImageList.TbImageInfos {
			log.Printf("%s", v.GetName())
		}
	}

	// RegisterImageWithInfo
	log.Printf("")
	log.Printf("RegisterImageWithInfo()")
	imageGrpcResult, err := imageClient.RegisterImageWithInfo(ctx, &pb.RegisterImageWithInfoWrapper{
		NsId: name,
		TbImageInfo: &pb.TbImageInfo{
			Name:           name,
			ConnectionName: "aws-us-east-1",
			CspImageId:     "ami-07ebfd5b3428b6f4d",
			CspImageName:   "ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-20190814",
		},
	})
	if err != nil {
		log.Printf("RegisterImageWithInfo failed: %v", err)
	} else {
		log.Printf("RegisterImageWithInfo success: %s", imageGrpcResult.GetId())
	}

	// GetImage
	log.Printf("")
	log.Printf("GetImage()")
	imageGrpcResult, err = imageClient.GetImage(ctx, &pb.GetResourceWrapper{
		NsId:       name,
		ResourceId: name,
	})
	if err != nil {
		log.Printf("GetImage failed: %v", err)
	} else {
		log.Printf("GetImage success: %s", imageGrpcResult.GetId())
	}

	// ListImage
	log.Printf("")
	log.Printf("ListImage()")
	grpcImageList, err = imageClient.ListImage(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListImage failed: %v", err)
	} else {
		log.Printf("ListImage success")
		for _, v := range grpcImageList.TbImageInfos {
			log.Printf("%s", v.GetName())
		}
	}

	// ListImageId
	log.Printf("")
	log.Printf("ListImageId()")
	grpcImageIdList, err := imageClient.ListImageId(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListImageId failed: %v", err)
	} else {
		log.Printf("ListImageId success")
		for _, v := range grpcImageIdList.Items {
			log.Printf("%s", v)
		}
	}

	// DelImage
	log.Printf("")
	log.Printf("DelImage()")
	_, err = imageClient.DelImage(ctx, &pb.DelResourceWrapper{
		NsId:       name,
		ResourceId: name,
		ForceFlag:  "false",
	})
	if err != nil {
		log.Fatalf("DelImage failed: %v", err)
	} else {
		log.Printf("DelImage success: %s", name)
	}

	/*
		// RegisterImageWithId: Not yet implemented in CB-Tumblebug
		log.Printf("")
		log.Printf("RegisterImageWithId()")
		imageGrpcResult, err = imageClient.RegisterImageWithId(ctx, &pb.RegisterImageWithIdWrapper{
			NsId: name,
			TbImageReq: &pb.TbImageReq{
				Name:           name,
				ConnectionName: "aws-us-east-1",
				CspImageId:     "ami-07ebfd5b3428b6f4d",
			},
		})
		if err != nil {
			log.Printf("RegisterImageWithId failed: %v", err)
		} else {
			log.Printf("RegisterImageWithId success: %s", imageGrpcResult.GetId())
		}
	*/

	// DelImage
	log.Printf("")
	log.Printf("DelImage()")
	_, err = imageClient.DelImage(ctx, &pb.DelResourceWrapper{
		NsId:       name,
		ResourceId: name,
		ForceFlag:  "false",
	})
	if err != nil {
		log.Printf("DelImage failed: %v", err)
	} else {
		log.Printf("DelImage success: %s", name)
	}

	// ListSpec
	log.Printf("")
	log.Printf("ListSpec()")
	grpcSpecList, err := specClient.ListSpec(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListSpec failed: %v", err)
	} else {
		log.Printf("ListSpec success")
		for _, v := range grpcSpecList.TbSpecInfos {
			log.Printf("%s", v.GetName())
		}
	}

	// RegisterSpecWithInfo
	log.Printf("")
	log.Printf("RegisterSpecWithInfo()")
	specGrpcResult, err := specClient.RegisterSpecWithInfo(ctx, &pb.RegisterSpecWithInfoWrapper{
		NsId: name,
		TbSpecInfo: &pb.TbSpecInfo{
			Name:           name,
			ConnectionName: "aws-us-east-1",
			CspSpecName:    "t2.micro",
		},
	})
	if err != nil {
		log.Printf("RegisterSpecWithInfo failed: %v", err)
	} else {
		log.Printf("RegisterSpecWithInfo success: %s", specGrpcResult.GetId())
	}

	// GetSpec
	log.Printf("")
	log.Printf("GetSpec()")
	specGrpcResult, err = specClient.GetSpec(ctx, &pb.GetResourceWrapper{
		NsId:       name,
		ResourceId: name,
	})
	if err != nil {
		log.Printf("GetSpec failed: %v", err)
	} else {
		log.Printf("GetSpec success: %s", specGrpcResult.GetId())
	}

	// ListSpec
	log.Printf("")
	log.Printf("ListSpec()")
	grpcSpecList, err = specClient.ListSpec(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListSpec failed: %v", err)
	} else {
		log.Printf("ListSpec success")
		for _, v := range grpcSpecList.TbSpecInfos {
			log.Printf("%s", v.GetName())
		}
	}

	// ListSpecId
	log.Printf("")
	log.Printf("ListSpecId()")
	grpcSpecIdList, err := specClient.ListSpecId(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("ListSpecId failed: %v", err)
	} else {
		log.Printf("ListSpecId success")
		for _, v := range grpcSpecIdList.Items {
			log.Printf("%s", v)
		}
	}

	// DelSpec
	log.Printf("")
	log.Printf("DelSpec()")
	_, err = specClient.DelSpec(ctx, &pb.DelResourceWrapper{
		NsId:       name,
		ResourceId: name,
		ForceFlag:  "false",
	})
	if err != nil {
		log.Fatalf("DelSpec failed: %v", err)
	} else {
		log.Printf("DelSpec success: %s", name)
	}

	log.Printf("")
	log.Printf("RegisterSpecWithCspSpecName()")
	specGrpcResult, err = specClient.RegisterSpecWithCspSpecName(ctx, &pb.RegisterSpecWithCspSpecNameWrapper{
		NsId: name,
		TbSpecReq: &pb.TbSpecReq{
			Name:           name,
			ConnectionName: "aws-us-east-1",
			CspSpecName:    "t2.micro",
		},
	})
	if err != nil {
		log.Printf("RegisterSpecWithCspSpecName failed: %v", err)
	} else {
		log.Printf("RegisterSpecWithCspSpecName success: %s", specGrpcResult.GetId())
	}

	// DelSpec
	log.Printf("")
	log.Printf("DelSpec()")
	_, err = specClient.DelSpec(ctx, &pb.DelResourceWrapper{
		NsId:       name,
		ResourceId: name,
		ForceFlag:  "false",
	})
	if err != nil {
		log.Printf("DelSpec failed: %v", err)
	} else {
		log.Printf("DelSpec success: %s", name)
	}

	// DelNs
	log.Printf("")
	log.Printf("DelNs()")
	_, err = nsClient.DelNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Printf("DelNs failed: %v", err)
	} else {
		log.Printf("DelNS success: %s", name)
	}

}
