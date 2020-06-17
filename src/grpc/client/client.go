package main

import (
	"context"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	pb "github.com/cloud-barista/cb-tumblebug/src/grpc/protobuf"
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
	r, err := c.CreateNS(ctx, &pb.NsReq{Name: name})
	if err != nil {
		log.Fatalf("NsReq failed: %v", err)
	}
	log.Printf("NsReq success: %s", r.GetName())
}
