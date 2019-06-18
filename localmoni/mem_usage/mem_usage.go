// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// to serve a local memory usage.
//
// by powerkim@powerkim.co.kr, 2019.02.
 package memusage


 import (
         "fmt"
         "github.com/shirou/gopsutil/mem"
 )

 func dealwithErr(err error) {
         if err != nil {
                 fmt.Println(err)
                 //os.Exit(-1)
         }
 }

 // return value: the number of byte
 func GetTotalMem() uint64 {
        vmStat, err := mem.VirtualMemory()
        dealwithErr(err)
	return vmStat.Total
 }

 // return value: the number of byte
 func GetUsedMem() uint64 {
        vmStat, err := mem.VirtualMemory()
        dealwithErr(err)
	return vmStat.Used
 }

 // return value: the number of byte
 func GetFreeMem() uint64 {
        vmStat, err := mem.VirtualMemory()
        dealwithErr(err)
	return vmStat.Free
 }

