// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// local disk statistics
//
// by powerkim@powerkim.co.kr, 2019.02.
 package diskstat


 import (
         "fmt"
         "github.com/shirou/gopsutil/disk"
         "time"
 )

 func dealwithErr(err error) {
         if err != nil {
                 fmt.Println(err)
                 //os.Exit(-1)
         }
 }

 // return partition list
 func GetPartitionList() []string {
	ret, err := disk.Partitions(false)
	dealwithErr(err)

	empty := disk.PartitionStat{}
	if len(ret) == 0 {
		fmt.Println("ret is empty")
	}

	strUniqPartiionList := make([]string,0)
	for _, disk := range ret {
		if disk == empty {
			fmt.Println("Could not get device info")
		}else {
			if(len(strUniqPartiionList)==0) {
				// if(disk.Mountpoint=="/boot") { continue } // except /boot, now no-ops
				strUniqPartiionList = append(strUniqPartiionList, disk.Device)
			}else {
				var exist bool=false
				for _, partition := range strUniqPartiionList {
					if(partition == disk.Device) { 
						exist=true 
						continue
					}
				} // end of for
				if(!exist) {
					if(disk.Mountpoint=="/boot") { continue }
						strUniqPartiionList = append(strUniqPartiionList, disk.Device)
				}
			} // end of if-else
			//fmt.Println("Device: " + disk.Device)
			//fmt.Println("Mountpoint: " + disk.Mountpoint)
			//fmt.Println("Fstype: " + disk.Fstype)
			//fmt.Println("Opts: " + disk.Opts)
		}
	}
	
	return strUniqPartiionList
 }

 // get Read Bytes and Write Bytes at now
 func GetRWBytes(partition string) (uint64, uint64) {
	// IOCountersStat
	ret, err := disk.IOCounters(partition)
	dealwithErr(err)

	//empty := disk.IOCountersStat{}
	var readBytes, writeBytes uint64
	for _, io := range ret {
		readBytes = io.ReadBytes
		writeBytes = io.WriteBytes
	} // end of for

	return readBytes, writeBytes
 }

 // get Read Bytes and Write Bytes per second
 func GetRWBytesPerSecond(partition string) (uint64, uint64) {
        // IOCountersStat
        first_ret, err := disk.IOCounters(partition)
        dealwithErr(err)

        var firstReadBytes, firstWriteBytes uint64
        for _, io := range first_ret {
                firstReadBytes = io.ReadBytes
                firstWriteBytes = io.WriteBytes
        } // end of for
	time.Sleep(time.Second)	

        // IOCountersStat
        second_ret, err := disk.IOCounters(partition)
        dealwithErr(err)

        var secondReadBytes, secondWriteBytes uint64
        for _, io := range second_ret {
                secondReadBytes = io.ReadBytes
                secondWriteBytes = io.WriteBytes
        } // end of for
        time.Sleep(time.Second)

        return secondReadBytes-firstReadBytes, secondWriteBytes-firstWriteBytes
 }

