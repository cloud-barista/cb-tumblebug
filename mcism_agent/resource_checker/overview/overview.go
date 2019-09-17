// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// Short reports of local resource monitoring info.
//
// by powerkim@powerkim.co.kr, 2019.02.
 package main


 import (
         "os"
         "github.com/cloud-barista/poc-farmoni/localmoni/cpu_usage"
         "github.com/cloud-barista/poc-farmoni/localmoni/mem_usage"
         "github.com/cloud-barista/poc-farmoni/localmoni/disk_stat"
         "strconv"
         "time"
	 "github.com/dustin/go-humanize"
 )

 func cpu() {
        // utilization for each logical CPU
        strCPUUtilizationArr := cpuusage.GetAllUtilPercentages()
	print("  [CPU USG]")
        for i, cpupercent := range strCPUUtilizationArr {
		if(i!=0) { print(", ") }
                print(" C" + strconv.Itoa(i) +":" + cpupercent + "%")
        }
 }

 func mem() {
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

	println("  [MEM USG] TOTAL: " + strTotalMemM + "MB, USED: " + strUsedMemM + "MB, FREE: " + strFreeMemM + "MB") 
 }


 func main() {
        // get Host Name
        hostname, _ := os.Hostname()

	// get effective partion list
	partitionList := diskstat.GetPartitionList()

        var readBytes [] uint64 = make([]uint64, len(partitionList))
        var writeBytes [] uint64 = make([]uint64, len(partitionList))
        var beforeReadBytes [] uint64 = make([]uint64, len(partitionList))
        var beforeWriteBytes [] uint64 = make([]uint64, len(partitionList))

        for{
                println("[" + hostname + "]")
                cpu()
                println("")

                mem()

                print("  [DSK RAT]")
                for i, partition := range partitionList {
                        print(partition + ": ")
			readBytes[i], writeBytes[i] = diskstat.GetRWBytes(partition)
                        print("R/s:   " + strconv.FormatUint(readBytes[i]-beforeReadBytes[i], 10))
                        print(", W/s:   " + strconv.FormatUint(writeBytes[i]-beforeWriteBytes[i], 10))

			beforeReadBytes[i] = readBytes[i]
			beforeWriteBytes[i] = writeBytes[i]
			if(i<(len(partitionList))) {
				print("\t")
			}
                }
                println("\n-----------")
                time.Sleep(time.Second)
        }

 }

