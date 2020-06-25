package server

import (
	"log"
	"net"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	Port = ":50051"
)

// server는 protobuf에서 정의된 함수의 인자로서 사용된다.
type server struct{}

// ProtoBuf의 IDL에 정의되어 있는 함수
// 함수의 인자와 리턴 값인 HelloRequest, HelloReply, 그리고 아래의 함수들은 모두
// protoc에서 생성된 skeleton 코드를 그대로 사용한다.

func RunServer() {
	lis, err := net.Listen("tcp", Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterNsServer(s, &server{})
	pb.RegisterImageServer(s, &server{})

	// Register reflection service on gRPC server.
	reflection.Register(s)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	//fmt.Println("gRPC server started on " + Port)
}
