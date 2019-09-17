// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// to server a local cpu usage.
//
// by powerkim@powerkim.co.kr, 2019.02.
 package cpuusage


 import (
         "runtime"
         "fmt"
         "strconv"
         "github.com/shirou/gopsutil/cpu"
 )

 func dealwithErr(err error) {
         if err != nil {
                 fmt.Println(err)
                 //os.Exit(-1)
         }
 }

 // the number of total cores(logical CPUs)
 func GetLogicalCPUNumber() int {
	return runtime.NumCPU()
 }

 func GetCPUModelName() string {
	cpuStat, err := cpu.Info()
	dealwithErr(err)

	return cpuStat[0].ModelName;
 }

 // percentages of each logical CPUs
 func GetAllUtilPercentages() []string {
	percentage, err := cpu.Percent(0, true)
	dealwithErr(err)

	strPercentageArr := make([]string,0)
	cpuNum := runtime.NumCPU()
	for idx := 0; idx < cpuNum; idx++ {
		//strPercentageArr[idx] = strconv.FormatFloat(cpupercent, 'f', 2, 64)
		strPercentageArr = append(strPercentageArr, strconv.FormatFloat(percentage[idx], 'f', 2, 64))
	}

	return strPercentageArr
 }


