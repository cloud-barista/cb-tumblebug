// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for local memory usage.
//
// by powerkim@powerkim.co.kr, 2019.02.
 package main


 import (
         "os"
         "github.com/cloud-barista/poc-farmoni/localmoni/mem_usage"
         "strconv"
         "time"
 )


 func main() {
	// get Host Name
	hostname, _ := os.Hostname()

for{
	totalMem := memusage.GetTotalMem()
	strTotalMemB := strconv.FormatUint(totalMem, 10)
	strTotalMemK := strconv.FormatUint(totalMem/1024, 10)
	strTotalMemM := strconv.FormatUint(totalMem/1024/1024, 10)
	strTotalMemG := strconv.FormatUint(totalMem/1024/1024/1024, 10)
	println("[" + hostname + "] Total Memory:  " + strTotalMemG + "GB ("  + strTotalMemB + "B, "  + strTotalMemK+ "KB, "  + strTotalMemM + "MB)" )

	freeMem := memusage.GetFreeMem()
	strFreeMemB := strconv.FormatUint(freeMem, 10)
	strFreeMemK := strconv.FormatUint(freeMem/1024, 10)
	strFreeMemM := strconv.FormatUint(freeMem/1024/1024, 10)
	strFreeMemG := strconv.FormatUint(freeMem/1024/1024/1024, 10)
	println("[" + hostname + "] Free Memory:   " + strFreeMemG + "GB ("  + strFreeMemB + "B, "  + strFreeMemK+ "KB, "  + strFreeMemM + "MB)" )
	println("-----------")
	time.Sleep(time.Second)
}

//	fmt.Scanln()

 }
