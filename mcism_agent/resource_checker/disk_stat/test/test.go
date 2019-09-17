// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test fo disk statistics. 
//
// by powerkim@powerkim.co.kr, 2019.02.
 package main


 import (
         "os"
         "github.com/cloud-barista/poc-farmoni/localmoni/disk_stat"
         "strconv"
 )

 func main() {
	// get Host Name
	hostname, _ := os.Hostname()

	// get effective partion list
	partitionList := diskstat.GetPartitionList()

for{
	for _, partition := range partitionList {
	//	println(partition)
		readBytes, writeBytes := diskstat.GetRWBytesPerSecond(partition)
		println("[" + hostname + "] PARTITION[" + partition + "] ReadBytes:   " + strconv.FormatUint(readBytes, 10))
		println("[" + hostname + "] PARTITION[" + partition + "] WriteBytes:   " + strconv.FormatUint(writeBytes, 10))
	}
} // end of for




/*
var before uint64 = 0 

for{
	// IOCountersStat
	ret, err := disk.IOCounters(partitionList[0])
	dealwithErr(err)

	empty := disk.IOCountersStat{}
	for part, io := range ret {
		println("[" + hostname + "] PARTITION[" + part + "] ReadBytes:   " + strconv.FormatUint(io.ReadBytes, 10))
//		println("[" + hostname + "] PARTITION[" + part + "] WriteBytes:   " + strconv.FormatUint(io.WriteBytes, 10))

		if(before==0) { 
			println("[" + hostname + "] PARTITION[" + part + "] ReadBytes/sec:   " + strconv.FormatUint(0, 10))
		} else {
			readBytespersec := io.ReadBytes - before
			println("[" + hostname + "] PARTITION[" + part + "] ReadBytes/sec:   " + strconv.FormatUint(readBytespersec, 10))
		}
		before = io.ReadBytes
	
		if io == empty {
			println("io_counter error")
		}
	}
	println("-----------")
	time.Sleep(time.Second)	
} // end of for
*/

//	fmt.Scanln()

 }
