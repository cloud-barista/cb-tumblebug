// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for local cpu usage.
//
// by powerkim@powerkim.co.kr, 2019.02.
 package main


 import (
         "os"
         "github.com/cloud-barista/poc-farmoni/localmoni/cpu_usage"
         "strconv"
         "time"
 )

 func main() {
	// get Host Name
	hostname, _ := os.Hostname()

	// CPU Model name
	strCPUModelName := cpuusage.GetCPUModelName()

	// logical cpu number(total cores)
	strCPUNumber := strconv.Itoa(cpuusage.GetLogicalCPUNumber())
for{
	// CPU Model name
	println("[" + hostname + "] CPU Model: " + strCPUModelName)
	println("[" + hostname + "] logical CPU#: " + strCPUNumber + " EA")

	// utilization for each logical CPU
	strCPUUtilizationArr := cpuusage.GetAllUtilPercentages()
	for i, cpupercent := range strCPUUtilizationArr {
		println("[" + hostname + "] CPU" + strconv.Itoa(i) +":   " + cpupercent + " %")
	}
	println("-----------")
	time.Sleep(time.Second)	
}

//	fmt.Scanln()

 }
