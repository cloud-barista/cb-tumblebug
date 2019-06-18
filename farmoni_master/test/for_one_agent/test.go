// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// master to control server and fetch monitoring info.
//
// by powerkim@powerkim.co.kr, 2019.03.
package main

import (
	_ "fmt"
	"context"
	"log"
	"os"
	"time"
	"google.golang.org/grpc"
	pb "github.com/cloud-barista/poc-farmoni/grpc_def"
)

const (
	defaultServerName = "129.254.184.79"
	port     = "2019"
)

func main() {
	// Contact the server and print out its response.
	serverName := defaultServerName
	if len(os.Args) > 1 {
		serverName = os.Args[1]
	}

	address := serverName + ":" + port
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewResourceStatClient(conn)

	//ctx, cancel := context.WithTimeout(context.Background(), 100*time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Hour)
	defer cancel()


	for{
		r, err := c.GetResourceStat(ctx, &pb.ResourceStatRequest{})
		if err != nil {
			log.Fatalf("could not Fetch Resource Status Information: %v", err)
		}

		println("[" + r.Servername + "]")
		log.Printf("%s", r.Cpu)
		log.Printf("%s", r.Mem)
		log.Printf("%s", r.Dsk)

		println("-----------")
                time.Sleep(time.Second)

	}
}
