// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// scp & ssh test for serverhandler
//
// by powerkim@powerkim.co.kr, 2019.03.
package main

import (
	"fmt"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/scp"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/sshrun"
)

func main() {

	// server connection info
	userName := "ec2-user"
	keyName := "/root/.aws/awspowerkimkeypair.pem"
	server := "52.78.36.226"
	port := ":22"
	serverPort := server + port

	// file info to copy
	sourceFile := "/root/go/src/farmoni/farmoni_agent/farmoni_agent"
	targetFile := "/tmp/farmoni_agent"

	// command for ssh run
        cmd := "/tmp/farmoni_agent &"

	// Connect to the server for scp
	scpCli, err := scp.Connect(userName, keyName, serverPort) 
	if err != nil {
                fmt.Println("Couldn't establisch a connection to the remote server ", err)
                return 
        }

	// copy agent into the server.
	if err := scp.Copy(scpCli, sourceFile, targetFile); err !=nil {
                fmt.Println("Error while copying file ", err)
	}

	// close the session
	scp.Close(scpCli)


        // Connect to the server for ssh
	sshCli, err := sshrun.Connect(userName, keyName, serverPort) 
        if err != nil {
                fmt.Println("Couldn't establisch a connection to the remote server ", err)
                return
        }

        if err := sshrun.RunCommand(sshCli, cmd); err != nil {
                fmt.Println("Error while running cmd: " + cmd, err)
        }

        sshrun.Close(sshCli)
	
}
