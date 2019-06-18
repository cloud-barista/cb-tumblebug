// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for ec2handler.
//
// by powerkim@powerkim.co.kr, 2019.03.
package main


 import (
         "github.com/cloud-barista/poc-farmoni/farmoni_master/ec2handler"
	 "fmt"
 )

 func main() {
    region := "ap-northeast-2" // seoul region.

    svc := ec2handler.Connect(region)

    instanceIds := ec2handler.CreateInstances(svc, "ami-047f7b46bd6dd5d84", "t2.micro", 1, 1,
        "aws.powerkim.keypair", "sg-2334584f", "subnet-8c4a53e4", "powerkimInstance_")

    // waiting for completion of new instance running.
    for _, v := range instanceIds {
	    // wait until running status
	    ec2handler.WaitForRun(svc, *v)
	    // get public IP
	    publicIP, err := ec2handler.GetPublicIP(svc, *v)
	    if err != nil {
		fmt.Println("Error", err)
		return
	    } 
	    fmt.Println(publicIP);
    }
 }

