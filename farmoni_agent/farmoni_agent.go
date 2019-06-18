// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// This Server is agent to serve a local resource status.
//
// by powerkim@powerkim.co.kr, 2019.03.15
package main

import (
	"context"
	"log"
	"net"

	"os"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	pb "github.com/cloud-barista/poc-farmoni/grpc_def"
        "github.com/cloud-barista/poc-farmoni/localmoni/cpu_usage"
        "github.com/cloud-barista/poc-farmoni/localmoni/mem_usage"
        "github.com/cloud-barista/poc-farmoni/localmoni/disk_stat"

	_ "os"
	"strconv"
	"github.com/dustin/go-humanize"	
)

 func cpu() string {
	cpu_stat := "  [CPU USG]"

        // utilization for each logical CPU
        strCPUUtilizationArr := cpuusage.GetAllUtilPercentages()
	
        for i, cpupercent := range strCPUUtilizationArr {
                if(i!=0) { cpu_stat = cpu_stat + ", " }
		cpu_stat = cpu_stat + " C" + strconv.Itoa(i) +":" + cpupercent + "%"	
        }
	return cpu_stat
 }

 func mem() string {
        // total memory in this machine
        totalMem := memusage.GetTotalMem()
        // mega byte
        //strTotalMemM := strconv.FormatUint(totalMem/1024/1024, 10)
        strTotalMemM := humanize.Comma(int64(totalMem/1024/1024))

        // used memory in this machine
        usedMem := memusage.GetUsedMem()
        // mega byte
        //strUsedMemM := strconv.FormatUint(usedMem/1024/1024, 10)
        strUsedMemM := humanize.Comma(int64(usedMem/1024/1024))

        // free memory in this machine
        freeMem := memusage.GetFreeMem()
        // mega byte
        //strFreeMemM := strconv.FormatUint(freeMem/1024/1024, 10)
        strFreeMemM := humanize.Comma(int64(freeMem/1024/1024))

        return "  [MEM USG] TOTAL: " + strTotalMemM + "MB, USED: " + strUsedMemM + "MB, FREE: " + strFreeMemM + "MB"
 }


 // for global variables of disk statistics
 var partitionList [] string
 var readBytes [] uint64
 var writeBytes [] uint64
 var beforeReadBytes [] uint64
 var beforeWriteBytes [] uint64

 // get effective partion list
 func init() {
	partitionList = diskstat.GetPartitionList()
	readBytes = make([]uint64, len(partitionList))
	writeBytes = make([]uint64, len(partitionList))
	beforeReadBytes = make([]uint64, len(partitionList))
	beforeWriteBytes = make([]uint64, len(partitionList))
 }


 func dsk() string {

	dsk_stat := "  [DSK RAT]"


	for i, partition := range partitionList {
		dsk_stat = dsk_stat + partition + ": "
		readBytes[i], writeBytes[i] = diskstat.GetRWBytes(partition)
		dsk_stat = dsk_stat + "R/s:   " + strconv.FormatUint(readBytes[i]-beforeReadBytes[i], 10)
		dsk_stat = dsk_stat + ", W/s:   " + strconv.FormatUint(writeBytes[i]-beforeWriteBytes[i], 10)

		beforeReadBytes[i] = readBytes[i]
		beforeWriteBytes[i] = writeBytes[i]

		if(i<(len(partitionList))) {
			dsk_stat = dsk_stat + "\t"
		}
		
	}
	return dsk_stat

 }


const (
	port = ":2019"
)

// server is used to implement memstat.MemstatServer.
type server struct{}

// GetMemStat implements memstat.MemstatServer
func (s *server) GetResourceStat(ctx context.Context, in *pb.ResourceStatRequest) (*pb.ResourceStatReply, error) {
	serverName, _ := os.Hostname()
	cpu := cpu()
	mem := mem()
	dsk := dsk()
	return &pb.ResourceStatReply{Servername: serverName, Cpu: cpu, Mem: mem, Dsk: dsk}, nil
}


func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterResourceStatServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
