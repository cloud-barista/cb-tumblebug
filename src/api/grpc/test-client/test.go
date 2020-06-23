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
	c := pb.NewNsClient(conn)

	// Contact the server and print out its response.
	name := defaultName
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// CheckNs
	log.Printf("")
	log.Printf("CheckNs()")
	check, err := c.CheckNs(ctx, &pb.NsId{Id: name})
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
	r, err := c.CreateNs(ctx, &pb.NsReq{Name: name})
	if err != nil {
		log.Printf("CreateNS failed: %v", err)
	} else {
		log.Printf("CreateNS success: %s", r.GetName())
	}

	// CheckNs
	log.Printf("")
	log.Printf("CheckNs()")
	check, err = c.CheckNs(ctx, &pb.NsId{Id: name})
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
	r, err = c.GetNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("GetNs failed: %v", err)
	} else {
		log.Printf("GetNS success: %s", r.GetName())
	}

	// ListNs
	log.Printf("")
	log.Printf("ListNs()")
	grpcNsList, err := c.ListNs(ctx, &types.Empty{})
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
	grpcNsIdList, err := c.ListNsId(ctx, &types.Empty{})
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
	_, err = c.DelNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Fatalf("DelNs failed: %v", err)
	} else {
		log.Printf("DelNS success: %s", name)
	}

	// DelNs
	log.Printf("")
	log.Printf("DelNs()")
	_, err = c.DelNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Printf("DelNs failed: %v", err)
	} else {
		log.Printf("DelNS success: %s", name)
	}

	// GetNs
	log.Printf("")
	log.Printf("GetNs()")
	r, err = c.GetNs(ctx, &pb.NsId{Id: name})
	if err != nil {
		log.Printf("GetNs failed: %v", err)
	} else {
		log.Printf("GetNS success: %s", r.GetName())
	}

	// ListNs
	log.Printf("")
	log.Printf("ListNs()")
	grpcNsList, err = c.ListNs(ctx, &types.Empty{})
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
	grpcNsIdList, err = c.ListNsId(ctx, &types.Empty{})
	if err != nil {
		log.Fatalf("ListNsId failed: %v", err)
	} else {
		log.Printf("ListNSId success")
		for _, v := range grpcNsIdList.Items {
			log.Printf("%s", v)
		}
	}

}
