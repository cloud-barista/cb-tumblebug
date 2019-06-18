// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for configuration handling with YAML.
// ref) https://github.com/go-yaml/yaml/tree/v3
//
// by powerkim@powerkim.co.kr, 2019.04.

package main

import (
        "fmt"
        "os"
        "log"
	"io/ioutil"

        "gopkg.in/yaml.v3"
)

type CONFIGTYPE struct {
	ETCDSERVERPORT string
	AWS struct {
		KEYFILEPATH string
		REGION string
		IMAGEID string
		INSTANCENAMEPREFIX string
		INSTANCETYPE string
		KEYNAME string
		SECURITYGROUPID string
		SUBNETID string
	}
}

func ReadConfigFile(filePath string) ([]byte, error) {
	data, err := ioutil.ReadFile(filePath)
	return data, err
}


func main() {
        t := CONFIGTYPE{}

	masterPath := os.Getenv("FARMONI_MASTER")
	data,err := ReadConfigFile(masterPath + "/conf/config.yaml")    
//        fmt.Printf("===>:\n%s\n\n", data)

        if err != nil {
                log.Fatalf("error: %v", err)
        }

        err = yaml.Unmarshal([]byte(data), &t)
        if err != nil {
                log.Fatalf("error: %v", err)
        }

        fmt.Printf("\n<unmarshalled data>\n")
        fmt.Printf("%v\n\n", t)

/* 
	fmt.Printf("  %s\n", t.ETCDSERVERPORT)

	fmt.Printf("\t%s\n", t.AWS.KEYFILEPATH)
	fmt.Printf("\t%s\n", t.AWS.REGION)
	fmt.Printf("\t%s\n", t.AWS.IMAGEID)
	fmt.Printf("\t%s\n", t.AWS.INSTANCENAMEPREFIX)
	fmt.Printf("\t%s\n", t.AWS.INSTANCETYPE)
	fmt.Printf("\t%s\n", t.AWS.KEYNAME)
	fmt.Printf("\t%s\n", t.AWS.SECURITYGROUPID)
	fmt.Printf("\t%s\n\n", t.AWS.SUBNETID)
*/

	d, err := yaml.Marshal(&t)
        if err != nil {
                log.Fatalf("error: %v", err)
        }
        fmt.Printf("<marshaled data>\n")
        fmt.Printf("%s\n\n", string(d))
}
