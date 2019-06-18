// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// to connect a remote server and copy a file to that remote server
//
// by powerkim@powerkim.co.kr, 2019.03.
package scp

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

func Copy(client scp.Client, sourcePath string, remotePath string) error {
        // Open a file
        file, _ := os.Open(sourcePath)

        // Close the file after it has been copied
        defer file.Close()

        err := client.CopyFile(file, remotePath, "0755")
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

	if err := Copy(client, "/root/go/src/farmoni/farmoni_agent/farmoni_agent", "/tmp/farmoni_agent"); err !=nil {
                fmt.Println("Error while copying file ", err)
	}


	Close(client)
}
*/
