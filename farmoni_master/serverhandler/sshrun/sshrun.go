// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// to connect a remote server and execute a command on that remote server.
//
// by powerkim@powerkim.co.kr, 2019.03.
package sshrun

import (
	_ "fmt"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
	"os"
)

func Connect(user string, keyPath string, serverPort string) (scp.Client, error) {

        clientConfig, _ := auth.PrivateKey(user, keyPath, ssh.InsecureIgnoreHostKey())
        // Create a new SCP client
        client := scp.NewClient(serverPort, &clientConfig)
        // Connect to the remote server
        err := client.Connect()

	return client, err
}

func Close(client scp.Client){
	client.Close()	
}

func RunCommand(client scp.Client, cmd string) error {
	sess := client.Session
	// setup standard out and error
	// uses writer interface
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	// run single command
	//err := sess.Run(cmd)
	err := sess.Start(cmd)
	return err
}

/*
func main() {

	// Connect to the server
	client, err := Connect("ec2-user", "/root/.aws/awspowerkimkeypair.pem", "52.78.36.226:22")	
	if err != nil {
                fmt.Println("Couldn't establisch a connection to the remote server ", err)
                return 
        }

	cmd := "/tmp/farmoni_agent &"
	if err := RunCommand(client, cmd); err != nil {
                fmt.Println("Error while running cmd: " + cmd, err)
	}

	Close(client)
}
*/
