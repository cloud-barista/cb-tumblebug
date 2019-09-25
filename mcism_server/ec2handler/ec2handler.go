// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// EC2 Hander (AWS SDK GO Version 1.16.26, Thanks AWS.)
//
// by powerkim@powerkim.co.kr, 2019.03.
package ec2handler

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
    "strconv"
    "fmt"
    "log"
)

func Connect(region string) *ec2.EC2 {
    // setup Region
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String(region)}, 
    )

    if err != nil {
        fmt.Println("Could not create instance", err)
        return nil
    }

    // Create EC2 service client
    svc := ec2.New(sess)

    return svc
}
/*
func Close() {
}
*/

func CreateInstances(svc *ec2.EC2, imageID string, instanceType string, 
		minCount int, maxCount int, keyName string, securityGroupID string, 
		subnetID string, baseName string) []*string {

    runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
        ImageId:      aws.String(imageID),  // set imageID ex) ami-047f7b46bd6dd5d84
        InstanceType: aws.String(instanceType), // instance Type, ex) t2.micro
        MinCount:     aws.Int64(int64(minCount)),      //
        MaxCount:     aws.Int64(int64(maxCount)),
        KeyName:      aws.String(keyName),      // set a keypair Name, ex) aws.powerkim.keypair
        SecurityGroupIds:      []*string{
                        aws.String(securityGroupID), // set a security group.
				},
        //SubnetId: aws.String("subnet-8c4a53e4"),     // set a subnet.
        SubnetId: aws.String(subnetID),     // set a subnet.
    })

    if err != nil {
        fmt.Println("Could not create instance", err)
        return nil
    }

    // copy Instances's ID 
    instanceIds := make([]*string, len(runResult.Instances))
    for k, v := range runResult.Instances {
        instanceIds[k] = v.InstanceId
    }
/*
    for i:=0; i<maxCount; i++ {
	    fmt.Println("Created instance", *runResult.Instances[i].InstanceId)
	    instanceID = *runResult.Instances[i].InstanceId
    }
*/

    for i:=0; i<maxCount; i++ {
	    // Add tags to the created instance
	    _, errtag := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{runResult.Instances[i].InstanceId},
		Tags: []*ec2.Tag{
		    {
			Key:   aws.String("Name"),
			Value: aws.String(baseName + strconv.Itoa(i)),
		    },
		},
	    })
	    if errtag != nil {
		log.Println("Could not create tags for instance", runResult.Instances[i].InstanceId, errtag)
		return nil
	    }
	    fmt.Println("Successfully tagged instance:" + baseName + strconv.Itoa(i))
    } // end of for

    return instanceIds 
}

func GetPublicIP(svc *ec2.EC2, instanceID string) (string, error) {
    var publicIP string

    input := &ec2.DescribeInstancesInput{
        InstanceIds: []*string{
            aws.String(instanceID), 
        },
    }
    // Call to get detailed information on each instance
    result, err := svc.DescribeInstances(input)
    if err != nil {
        fmt.Println("Error", err)
	return publicIP, err
    } 

//    fmt.Println(result)

    for i, _ := range result.Reservations {
	for _, inst := range result.Reservations[i].Instances {
		publicIP = *inst.PublicIpAddress
	}
    }
    return publicIP, err
}

func WaitForRun(svc *ec2.EC2, instanceID string) {
    input := &ec2.DescribeInstancesInput{
        InstanceIds: []*string{
            aws.String(instanceID),
        },
    }
    err := svc.WaitUntilInstanceRunning(input)
    if err != nil {
        fmt.Println("failed to wait until instances exist: %v", err)
    }
}

func DestroyInstances(svc *ec2.EC2, instanceIds []*string) error {

    //input := &ec2.TerminateInstancesInput(instanceIds)
    input := &ec2.TerminateInstancesInput{
	InstanceIds: instanceIds ,
    }	

    _, err := svc.TerminateInstances(input)
	

    if err != nil {
        fmt.Println("Could not termiate instances", err)
    }

    return err
}

