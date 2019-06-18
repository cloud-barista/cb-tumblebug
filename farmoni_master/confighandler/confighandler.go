// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// CONFIG Hander with YAML
// ref) https://github.com/go-yaml/yaml/tree/v3
//	https://godoc.org/gopkg.in/yaml.v3
//
// by powerkim@powerkim.co.kr, 2019.04.


package confighandler

import (
    "os"
    "io/ioutil"
    "log"

    "gopkg.in/yaml.v3"
)

type MASTERCONFIGTYPE struct {
        ETCDSERVERPORT string
        AWS struct {
                REGION string
                IMAGEID string
                INSTANCENAMEPREFIX string
                INSTANCETYPE string
                SECURITYGROUPID string
                SUBNETID string

                USERNAME string
                KEYNAME string
                KEYFILEPATH string
        }

        GCP struct {
                CREDENTIALFILE string
                REGION string
                ZONE string
                PROJECTID string
                PREFIX string

                IMAGEID string
                INSTANCENAMEPREFIX string
                INSTANCETYPE string
                SECURITYGROUPID string
                SUBNETID string
                NETWORKNAME string
                SERVICEACCOUTSMAIL string

                USERNAME string
                KEYNAME string
                KEYFILEPATH string
        }

        AZURE struct {
                CREDENTIALFILE string
                GROUPNAME string
                LOCATION string
                VIRTUALNETWORKNAME string
                SUBNET1NAME string
                SUBNET2NAME string
                NETWORKSECURITYGROUPNAME string
//                IPNAME string
//                NICNAME string
                BASENAME string

                USERNAME string
                PASSWORD string
                KEYFILEPATH string
                PUBLICKEYFILEPATH string

	}
}

func readConfigFile(filePath string) ([]byte, error) {
        data, err := ioutil.ReadFile(filePath)
        return data, err
}

func GetMasterConfigInfos() MASTERCONFIGTYPE {
        masterRootPath := os.Getenv("FARMONI_MASTER")
        data, err := readConfigFile(masterRootPath + "/conf/config.yaml")

        if err != nil {
                log.Fatalf("error: %v", err)
        }

        masterConfigInfos := MASTERCONFIGTYPE{}
        err = yaml.Unmarshal([]byte(data), &masterConfigInfos)
        if err != nil {
                log.Fatalf("error: %v", err)
        }

	return masterConfigInfos
}

func GetConfigString(configInfos *MASTERCONFIGTYPE) string {
        d, err := yaml.Marshal(configInfos)
        if err != nil {
                log.Fatalf("error: %v", err)
        }
	return string(d)
}
