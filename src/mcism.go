package main

import (
	"github.com/cloud-barista/cb-tumblebug/src/apiserver"
	"os"
)

func main() {

	//fmt.Println("\n[cb-tumblebug (Multi-Cloud Infra Service Management Framework)]")
	//fmt.Println("\nInitiating REST API Server ...")
	//fmt.Println("\n[REST API call examples]")

	apiserver.SPIDER_URL = os.Getenv("SPIDER_URL")

	/*
		fmt.Println("[List MCISs]:\t\t curl <ServerIP>:1323/mcis")
		fmt.Println("[Create MCIS]:\t\t curl -X POST <ServerIP>:1323/mcis  -H 'Content-Type: application/json' -d '{<MCIS_REQ_JSON>}'")
		fmt.Println("[Get MCIS Info]:\t curl <ServerIP>:1323/mcis/<McisID>")
		fmt.Println("[Get MCIS status]:\t curl <ServerIP>:1323/mcis/<McisID>?action=monitor")
		fmt.Println("[Terminate MCIS]:\t curl <ServerIP>:1323/mcis/<McisID>?action=terminate")
		fmt.Println("[Del MCIS Info]:\t curl -X DELETE <ServerIP>:1323/mcis/<McisID>")
		fmt.Println("[Del MCISs Info]:\t curl -X DELETE <ServerIP>:1323/mcis")

		fmt.Println("\n")
		fmt.Println("[List Images]:\t\t curl <ServerIP>:1323/image")
		fmt.Println("[Create Image]:\t\t curl -X POST <ServerIP>:1323/image?action=create -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
		fmt.Println("[Register Image]:\t\t curl -X POST <ServerIP>:1323/image?action=register -H 'Content-Type: application/json' -d '{<IMAGE_REQ_JSON>}'")
		fmt.Println("[Get Image Info]:\t curl <ServerIP>:1323/image/<imageID>")
		fmt.Println("[Del Image Info]:\t curl -X DELETE <ServerIP>:1323/image/<imageID>")
		fmt.Println("[Del Images Info]:\t curl -X DELETE <ServerIP>:1323/image")
	*/

	// load config
	//masterConfigInfos = confighandler.GetMasterConfigInfos()

	// Run API Server
	apiserver.ApiServer()

}