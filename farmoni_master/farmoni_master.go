// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// Farmoni Master to control server and fetch monitoring info.
//
// by powerkim@powerkim.co.kr, 2019.03.
package main

import (
	"flag"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/ec2handler"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/gcehandler"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/azurehandler"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/scp"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/serverhandler/sshrun"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/etcdhandler"
	"github.com/cloud-barista/poc-farmoni/farmoni_master/confighandler"

	"fmt"
	"os"
	"context"
	"time"
	"log"
	"strings"
	"strconv"
        "google.golang.org/grpc"
        pb "github.com/cloud-barista/poc-farmoni/grpc_def"

	// REST API (echo)
	"net/http"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// REST API
/*
type (
	user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
)
var (
	users = map[int]*user{}
	seq   = 1
)
*/


type (
        svc struct {
                ID   int    `json:"id"`
		NAME string `json:"name"`
	        CSP  string `json:"csp"`
		NUM  int    `json:"num"`
        }
)
var (
        svcs = map[int]*svc{}
        seqSvc   = 1
)

/*
type (
        svc struct {
                ID   int    `json:"id"`
                NAME string `json:"name"`
                SERVER []struct {
                        CSP  string `json:"csp"`
                        NUM  int    `json:"num"`
                        VMID string `json:"vmid"`
                        IP   string `json:"ip"`
                }`json:"server"`
        }
)
var (
        svcs = map[int]*svc{}
        seqSvc   = 1
)
*/




const (
	defaultServerName = "129.254.184.79"
	port     = "2019"
)

var masterConfigInfos confighandler.MASTERCONFIGTYPE

var etcdServerPort *string
var fetchType *string

var addServer *string
var delServer *string

var addVMNumAWS *int
var delServerNumAWS *int

var addVMNumGCP *int
var delServerNumGCP *int

var addVMNumAZURE *int

var listvm *bool
var monitoring *bool
var delVMAWS *bool
var delVMGCP *bool
var delVMAZURE *bool

func apiServer() {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World! This is cloud-barista Poc-farmoni")
	})

	// Routeso
	/*
	e.POST("/users", createUser)
	e.GET("/users/:id", getUser)
	e.PUT("/users/:id", updateUser)
	e.DELETE("/users/:id", deleteUser)
	*/

        e.POST("/svcs", createSvc)
        e.GET("/svcs/:id", getSvc)
	e.GET("/svcs", getSvcs)
	e.PUT("/svcs/:id", updateSvc)
        e.DELETE("/svcs/:id", deleteSvc)
	e.DELETE("/svcs", deleteAllSvc)


	e.Logger.Fatal(e.Start(":1323"))


}

// API Handlers

/*
func createUser(c echo.Context) error {
	u := &user{
		ID: seq,
	}
	if err := c.Bind(u); err != nil {
		return err
	}
	users[u.ID] = u
	seq++
	return c.JSON(http.StatusCreated, u)
}

func getUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	return c.JSON(http.StatusOK, users[id])
}

func updateUser(c echo.Context) error {
	u := new(user)
	if err := c.Bind(u); err != nil {
		return err
	}
	id, _ := strconv.Atoi(c.Param("id"))
	users[id].Name = u.Name
	return c.JSON(http.StatusOK, users[id])
}

func deleteUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	delete(users, id)
	return c.NoContent(http.StatusNoContent)
}
*/

func createSvc(c echo.Context) error {
        u := &svc{
                ID: int(time.Now().UnixNano() / 1e6),
        }
        if err := c.Bind(u); err != nil {
                return err
        }
        svcs[u.ID] = u
	fmt.Print("CSP"+u.CSP)


	//addServiceToEtcd(strconv.Itoa(u.ID), u.NAME)

	if u.CSP == "aws" {
		fmt.Println("######### addVMaws....")
		instanceIds, publicIPs := addVMaws(u.NUM)

		for i := 0; i < len(instanceIds) && i < len(publicIPs); i++{
			fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" +  string(*publicIPs[i]) )
			addServerToService(strconv.Itoa(u.ID), u.NAME,"aws", instanceIds, publicIPs)
		}

        }
	if u.CSP == "gcp" {
                fmt.Println("######### addVMgcp....")
		instanceIds, publicIPs := addVMgcp(u.NUM)

                for i := 0; i < len(instanceIds) && i < len(publicIPs); i++{
                        fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" +  string(*publicIPs[i]) )
                        addServerToService(strconv.Itoa(u.ID), u.NAME,"gcp", instanceIds, publicIPs)
                }

        }
	if u.CSP == "azure" {
                fmt.Println("######### addVMazure....")
		instanceIds, publicIPs := addVMazure(u.NUM)

                for i := 0; i < len(instanceIds) && i < len(publicIPs); i++{
                        fmt.Println("[instanceIds=] " + string(*instanceIds[i]) + "[publicIPs]" +  string(*publicIPs[i]) )
                        addServerToService(strconv.Itoa(u.ID), u.NAME,"azure", instanceIds, publicIPs)
                }
	}
/*
	for _, v := range instanceIds {
                //vs := strings.Split(string(*v), "/")

                fmt.Println("[instanceIds=] " + v )
        }
*/

        seqSvc++
        return c.JSON(http.StatusCreated, u)
}

type server struct {
        CSP  string `json:"csp"`
        VMID string `json:"vmid"`
        IP   string `json:"ip"`
        AGENT_PORT   string `json:"agent_port"`
}
type stService struct {
        ID   int    `json:"id"`
        NAME string `json:"name"`
        SERVER []server `json:"server"`
}


func getSvc(c echo.Context) error {
        id, _ := strconv.Atoi(c.Param("id"))



        return c.JSON(http.StatusOK, svcs[id])
}




func getSvcs(c echo.Context) error {

	/*
	type server struct {
		CSP  string `json:"csp"`
	        VMID string `json:"vmid"`
                IP   string `json:"ip"`
        }


	var content struct {
		ID   int    `json:"id"`
                NAME string `json:"name"`
                SERVER []server `json:"server"`
		Response  string    `json:"response"`
		Timestamp time.Time `json:"timestamp"`
		Random    int       `json:"random"`
	}
	*/
	var content struct {
		SERVICE []stService `json:"service"`
		Response  string    `json:"response"`
        	Timestamp time.Time `json:"timestamp"`
	        Random    int       `json:"random"`
	}
	/*
	content.Response = "Sent via JSONP"
	content.Timestamp = time.Now().UTC()
	content.Random = 1000
*/

//	content.ID = 123123
//	content.NAME = "name"
/*
	sv := server{"a","b","c"}

	svcTmp := stService{}
	svcTmp.ID = 12345
	svcTmp.NAME = "NameST"
	svcTmp.SERVER = append(svcTmp.SERVER, sv)

	content.SERVICE = append(content.SERVICE, svcTmp)
*/
	/*
	type (
        svc struct {
                ID   int    `json:"id"`
                NAME string `json:"name"`
                SERVER []struct {
                        CSP  string `json:"csp"`
                        NUM  int    `json:"num"`
                        VMID string `json:"vmid"`
                        IP   string `json:"ip"`
                }`json:"server"`
        }
	)
*/


        list := getServerList()
        fmt.Print("######### all server list....(" + strconv.Itoa(len(list)) + ")\n")


        for _, v := range list {
                vs := strings.Split(string(*v), "/")
		
                fmt.Println("[CSP] " + vs[0] + "\t/ [VmID] "+ vs[1] +"\t/ [IP] " + vs[2])
        }

	serviceList()

	serviceList := getServiceList()
        fmt.Print("######### all service list....(" + strconv.Itoa(len(serviceList)) + ")\n")


	checkDuplication := "inital"
        for _, k := range serviceList {
                slice0 := strings.Split(string(*k), "/server/")
		slice00 := strings.Split(string(slice0[0]), "/")
		//slice01 := strings.Split(string(slice0[1]), "/")

		if checkDuplication != slice00[0] {

			svcTmp := stService{}
			svcTmp.ID, _ = strconv.Atoi(slice00[0])
			svcTmp.NAME = slice00[1]

			slice01 := getServersInServiceFromEtcd(slice00[0])

			for _, j := range slice01 {

				//fmt.Println("[VM-CSP] " + string(*j))
				serverInfo := strings.Split(string(*j), "/")
				//sv := server{string(j[0]),string(j[1]),string(j[2])}
				//fmt.Println("string(serverInfo[2]): ", string(serverInfo[2])); // jhseo debug
				ip_port := strings.Split(string(serverInfo[2]), ":"); // jhseo
				//fmt.Println("ip: ", ip_port[0], ", port: ", ip_port[1]); // jhseo debug
				sv := server{string(serverInfo[0]),string(serverInfo[1]), ip_port[0], ip_port[1]}
				svcTmp.SERVER = append(svcTmp.SERVER, sv)
			}
			content.SERVICE = append(content.SERVICE, svcTmp)
			checkDuplication = slice00[0] 
		}

//		fmt.Println("[Service]"+ string(vs[0]) +"\n")
        }
        content.Response = "Sent via Cloud-Barista"
        content.Timestamp = time.Now().UTC()
        content.Random = 1000


        return c.JSON(http.StatusOK, &content)
}

func updateSvc(c echo.Context) error {
        u := new(svc)
        if err := c.Bind(u); err != nil {
                return err
        }
        id, _ := strconv.Atoi(c.Param("id"))
        svcs[id].NAME = u.NAME
        return c.JSON(http.StatusOK, svcs[id])
}

func deleteSvc(c echo.Context) error {
        id, _ := strconv.Atoi(c.Param("id"))
        delete(svcs, id)
	//delServicesFromEtcd()

        return c.NoContent(http.StatusNoContent)
}

func deleteAllSvc(c echo.Context) error {
        //id, _ := strconv.Atoi(c.Param("id"))
        //delete(svcs, id)
        delServicesFromEtcd()

        fmt.Println("######### delete all servers in AWS....")
        delAllVMaws()
        fmt.Println("######### delete all servers in GCP....")
        delAllVMgcp()
        fmt.Println("######### delete all servers in AZURE....")
        delAllVMazure()

        return c.NoContent(http.StatusNoContent)
}


func addServiceToEtcd(svcId string, svcName string) {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	etcdhandler.AddService(ctx, etcdcli, &svcId, &svcName, fetchType)
}

func getServersInServiceFromEtcd(svcId string) []*string {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	return etcdhandler.GetServersInService(ctx, etcdcli, &svcId)
}


func delServicesFromEtcd() {
        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

        fmt.Println("######### delete  all Services....")
        etcdhandler.DelAllSvcs(ctx, etcdcli)
}



func addServerToService(svcId string, svcName string, provider string, instanceIds []*string, serverIPs []*string) {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

        for i, v := range serverIPs {
                serverPort := *v + ":2019" // 2019 Port is dedicated value for PoC.
                fmt.Println("######### addServer...." + serverPort)
                // /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
                etcdhandler.AddServerToService(ctx, etcdcli, &svcId, &svcName, &provider, instanceIds[i], &serverPort, fetchType)
        }

}

func serviceList() {

        list := getServiceList()
        fmt.Print("######### all serivce list....(" + strconv.Itoa(len(list)) + ")\n")


        for _, v := range list {
//                vs := strings.Split(string(*v), "/")
                fmt.Println("[Service]"+ string(*v) +"\n")
        }
}

func getServiceList() []*string {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

        return etcdhandler.ServiceList(ctx, etcdcli)
}






func parseRequest() {

        etcdServerPort = &masterConfigInfos.ETCDSERVERPORT

        //etcdServerPort = flag.String("etcdserver", "129.254.175.43:2379", "etcdserver=129.254.175.43:2379")
        fetchType = flag.String("fetchtype", "PULL", "fetch type: -fetchtype=PUSH")
/*
        addServer = flag.String("addserver", "none", "add a server: -addserver=192.168.0.10:5000")
        delServer = flag.String("delserver", "none", "delete a server: -delserver=192.168.0.10")
*/
        addVMNumAWS = flag.Int("addvm-aws", 0, "add servers in AWS: -addvm-aws=10")
        delVMAWS = flag.Bool("delvm-aws", false, "delete all servers in AWS: -delvm-aws")

        addVMNumGCP = flag.Int("addvm-gcp", 0, "add servers in GCP: -addvm-gcp=10")
        delVMGCP = flag.Bool("delvm-gcp", false, "delete all servers in GCP: -delvm-gcp")

        addVMNumAZURE = flag.Int("addvm-azure", 0, "add servers in AZURE: -addvm-azure=10")
        delVMAZURE = flag.Bool("delvm-azure", false, "delete all servers in AZURE: -delvm-azure")

        listvm = flag.Bool("listvm", false, "report server list: -listvm")
        monitoring = flag.Bool("monitor", false, "report all server' resources status: -monitor")

        flag.Parse()
}

func getInteractiveRequest() {

	command := -1
	fmt.Println("[Select opt (0:API-server, 1:create-vm, 2:delete-vm, 3:list-vm, 4:monitor-vm]")
	fmt.Print("Your section : ")
	fmt.Scanln(&command)
	fmt.Println(command)

	/*
        if command != 0 {
                fmt.Println("######### addVMaws....")
                addVMaws(*addVMNumAWS)
		if command
        }
	*/
	switch {
        case command == 0:
                apiServer()
	case command == 1:
	        selCsp := 0
	        fmt.Println("[Select cloud service provider (1:aws, 2:gcp, 3:azure, 4:TBD]")
	        fmt.Print("Your section : ")
	        fmt.Scanln(&selCsp)

                selVmNum := 1
                fmt.Println("[Provide the number of VM to create (e.g., 5)")
                fmt.Print("Your section : ")
                fmt.Scanln(&selVmNum)

	        switch {
	        case selCsp == 0:
		        fmt.Println("nothing was selected")
	        case selCsp == 1:
		        fmt.Println("Create VM(s) in aws")
			*addVMNumAWS = selVmNum
                case selCsp == 2:
                        fmt.Println("Create VM(s) in gcp")
                        *addVMNumGCP = selVmNum
                case selCsp == 3:
                        fmt.Println("Create VM(s) in azure")
                        *addVMNumAZURE = selVmNum
		case selCsp == 4:
                        fmt.Println("not implemented yet. will be provided soon")
                default:
                        fmt.Println("select within 1-4")
		}
	case command == 2:
                selCsp := -1
		fmt.Println("[Select cloud service provider (0: all, 1:aws, 2:gcp, 3:azure, 4:TBD]")
                fmt.Print("Your section : ")
                fmt.Scanln(&selCsp)

                switch {
                case selCsp == -1:
                        fmt.Println("nothing was selected")
                case selCsp == 0:
                        fmt.Println("Delete all VMs for all CPSs")
                        *delVMAWS = true
			*delVMGCP = true
			*delVMAZURE = true
                case selCsp == 1:
                        fmt.Println("Delete all VMs in aws")
                        *delVMAWS = true
                case selCsp == 2:
                        fmt.Println("Delete all VMs in gcp")
                        *delVMGCP = true
                case selCsp == 3:
                        fmt.Println("Delete all VMs in azure")
                        *delVMAZURE = true
                case selCsp == 4:
                        fmt.Println("not implemented yet. will be provided soon")
                default:
                        fmt.Println("select within 1-4")
                }
	case command == 3:
                *listvm = true
	case command == 4:
                *monitoring = true
	default:
		fmt.Println("select within 1-4")
	}


	/*
        fetchType = flag.String("fetchtype", "PULL", "fetch type: -fetchtype=PUSH")

        addVMNumAWS = flag.Int("addvm-aws", 0, "add servers in AWS: -addvm-aws=10")
        delVMAWS = flag.Bool("delvm-aws", false, "delete all servers in AWS: -delvm-aws")

        addVMNumGCP = flag.Int("addvm-gcp", 0, "add servers in GCP: -addvm-gcp=10")
        delVMGCP = flag.Bool("delvm-gcp", false, "delete all servers in GCP: -delvm-gcp")

        addVMNumAZURE = flag.Int("addvm-azure", 0, "add servers in AZURE: -addvm-azure=10")
        delVMAZURE = flag.Bool("delvm-azure", false, "delete all servers in AZURE: -delvm-azure")

        listvm = flag.Bool("listvm", false, "report server list: -listvm")
        monitoring = flag.Bool("monitor", false, "report all server' resources status: -monitor")

        flag.Parse()
	*/
}


// 1. setup a credential info of AWS.
// 2. setup a keypair for VM ssh login.

// 1. parsing user's request.

//<add Servers in AWS/GCP>
// 1.1. create Servers(VM).
// 1.2. get servers' public IP.
// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
// 1.5. add server list into etcd.

//<get all server list>
// 2.1. get server list from etcd.
// 2.2. fetch all agent's monitoring info.

//<delete all servers inAWS/GCP>
func main() {

        fmt.Println("## examples ##")
        fmt.Println("go run farmoni_master.go -addvm-aws=10")
        fmt.Println("go run farmoni_master.go -addvm-gcp=5")
        fmt.Println("go run farmoni_master.go -addvm-azure=5")
        fmt.Println("")
        fmt.Println("go run farmoni_master.go -listvm")
        fmt.Println("go run farmoni_master.go -monitor")

	// to delete all servers in aws
        fmt.Println("")
        fmt.Println("go run farmoni_master.go -delvm-aws")
        fmt.Println("go run farmoni_master.go -delvm-gcp")
        fmt.Println("go run farmoni_master.go -delvm-azure")
        fmt.Println("")

// load config
	// you can see the details of masterConfigInfos
	// at confighander/confighandler.go:MASTERCONFIGTYPE.
	masterConfigInfos = confighandler.GetMasterConfigInfos()

 // dedicated option for PoC
	// 1. parsing user's request.
	parseRequest()

	// Get interactive command request
	getInteractiveRequest()

//<add servers in AWS/GCP/AZURE>
	// 1.1. create Servers(VM).
	if *addVMNumAWS != 0 {
                fmt.Println("######### addVMaws....")
                addVMaws(*addVMNumAWS)
        }
	if *addVMNumGCP != 0 {
                fmt.Println("######### addVMgcp....")
                addVMgcp(*addVMNumGCP)
        }
	if *addVMNumAZURE != 0 {
                fmt.Println("######### addVMazure....")
                addVMazure(*addVMNumAZURE)
        }

//<get all server list>
	if *listvm != false {
                //fmt.Println("######### list of all servers....")
                serverList()
        }
// 2.2. fetch all agent's monitoring info.
	if *monitoring != false {
                fmt.Println("######### monitoring all servers....")
                monitoringAll()
        }

//<delete all servers inAWS/GCP/AZURE>
	if *delVMAWS != false {
                fmt.Println("######### delete all servers in AWS....")
                delAllVMaws()
        }
	if *delVMGCP != false {
                fmt.Println("######### delete all servers in GCP....")
                delAllVMgcp()
        }
	if *delVMAZURE != false {
                fmt.Println("######### delete all servers in AZURE....")
                delAllVMazure()
        }


}



// 1.1. create Servers(VM).
// 1.2. get servers' public IP.
// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
// 1.5. add server list into etcd.
func addVMaws(count int) ([]*string, []*string) {
// ==> AWS-EC2
    //region := "ap-northeast-2" // seoul region.
    region := masterConfigInfos.AWS.REGION // seoul region.

    svc := ec2handler.Connect(region)

// 1.1. create Servers(VM).
    // some options are static for simple PoC.
    // These must be prepared before.

    imageId := masterConfigInfos.AWS.IMAGEID  // ami-047f7b46bd6dd5d84
    instanceType := masterConfigInfos.AWS.INSTANCETYPE  // t2.micro
    securityGroupId := masterConfigInfos.AWS.SECURITYGROUPID  // sg-2334584f
    subnetid := masterConfigInfos.AWS.SUBNETID  // subnet-8c4a53e4
    instanceNamePrefix := masterConfigInfos.AWS.INSTANCENAMEPREFIX  // powerkimInstance_

    userName := masterConfigInfos.AWS.USERNAME  // ec2-user
    keyName := masterConfigInfos.AWS.KEYNAME  // aws.powerkim.keypair
    keyPath := masterConfigInfos.AWS.KEYFILEPATH  // /root/.aws/awspowerkimkeypair.pem

    //instanceIds := ec2handler.CreateInstances(svc, "ami-047f7b46bd6dd5d84", "t2.micro", 1, count,
    //   "aws.powerkim.keypair", "sg-2334584f", "subnet-8c4a53e4", "powerkimInstance_")
    instanceIds := ec2handler.CreateInstances(svc, imageId, instanceType, 1, count, 
        keyName, securityGroupId, subnetid, instanceNamePrefix) 

    publicIPs := make([]*string, len(instanceIds))

// 1.2. get servers' public IP.
    // waiting for completion of new instance running.
    // after then, can get publicIP.
    for k, v := range instanceIds {
            // wait until running status
            ec2handler.WaitForRun(svc, *v)
            // get public IP
            publicIP, err := ec2handler.GetPublicIP(svc, *v)
            if err != nil {
                fmt.Println("Error", err)
                return nil, nil
            }
            fmt.Println("==============> " + publicIP);
	    publicIPs[k] = &publicIP
    }

    
// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
    for _, v := range publicIPs {
	    for i:=0; ; i++ {
		err:=copyAndPlayAgent(*v, userName, keyPath)
		if(i==30) { os.Exit(3) }
		    if err == nil {
			break;	
		    }
		    // need to load SSH Service on the VM
		    time.Sleep(time.Second*3)
	    } // end of for
    } // end of for

// 1.5. add server list into etcd.
    addServersToEtcd("aws", instanceIds, publicIPs)

    return instanceIds, publicIPs
}

// (1) get all AWS server id list from etcd
// (2) terminate all AWS servers
// (3) remove server list from etcd
func delAllVMaws() {

// (1) get all AWS server id list from etcd
    idList := getInstanceIdListAWS()

// (2) terminate all AWS servers
    //region := "ap-northeast-2" 
    region := masterConfigInfos.AWS.REGION

    svc := ec2handler.Connect(region)

//  destroy Servers(VMs).
    ec2handler.DestroyInstances(svc, idList)


// (3) remove all aws server list from etcd
    delProviderAllServersFromEtcd(string("aws"))
}

// (1) get all GCP server id list from etcd
// (2) terminate all GCP servers
// (3) remove server list from etcd
func delAllVMgcp() {

// (1) get all GCP server id list from etcd
    idList := getInstanceIdListGCP()

// (2) terminate all GCP servers
    credentialFile := masterConfigInfos.GCP.CREDENTIALFILE
    svc := gcehandler.Connect(credentialFile)

//  destroy all Servers(VMs).
    zone := masterConfigInfos.GCP.ZONE
    projectID := masterConfigInfos.GCP.PROJECTID
    gcehandler.DestroyInstances(svc, zone, projectID, idList)


// (3) remove all aws server list from etcd
    delProviderAllServersFromEtcd(string("gcp"))
}

// (1) get all AZURE server id list from etcd
// (2) terminate all AZURE servers
// (3) remove server list from etcd
func delAllVMazure() {

// (1) get all AZURE server id list from etcd
    // idList := getInstanceIdListAZURE()

// (2) terminate all AZURE servers
    credentialFile := masterConfigInfos.AZURE.CREDENTIALFILE
    connInfo := azurehandler.Connect(credentialFile)

//  destroy all Servers(VMs).
    groupName := masterConfigInfos.AZURE.GROUPNAME
//    azurehandler.DestroyInstances(connInfo, groupName, idList)  @todo now, just delete target Group for convenience.
    azurehandler.DeleteGroup(connInfo, groupName)


// (3) remove all aws server list from etcd
    delProviderAllServersFromEtcd(string("azure"))
}


func addServersToEtcd(provider string, instanceIds []*string, serverIPs []*string) {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

        for i, v := range serverIPs {
                serverPort := *v + ":2019" // 2019 Port is dedicated value for PoC.
                fmt.Println("######### addServer...." + serverPort)
		// /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
                etcdhandler.AddServer(ctx, etcdcli, &provider, instanceIds[i], &serverPort, fetchType)
        }

}


func delProviderAllServersFromEtcd(provider string) {
        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        ctx := context.Background()

	fmt.Println("######### delete " + provider + " all Server....")
	etcdhandler.DelProviderAllServers(ctx, etcdcli, &provider)
}


func copyAndPlayAgent(serverIP string, userName string, keyPath string) error {

        // server connection info
	// some options are static for simple PoC.// some options are static for simple PoC.
        // These must be prepared before.
        //userName := "ec2-user"
        port := ":22"
        serverPort := serverIP + port

        //keyPath := "/root/.aws/awspowerkimkeypair.pem"
        //keyPath := masterConfigInfos.AWS.KEYFILEPATH

        // file info to copy
        //sourceFile := "/root/go/src/farmoni/farmoni_agent/farmoni_agent"
        //sourceFile := "/root/go/src/github.com/cloud-barista/poc-farmoni/farmoni_agent/farmoni_agent"
	homePath := os.Getenv("HOME")
        sourceFile := homePath + "/go/src/github.com/cloud-barista/poc-farmoni/farmoni_agent/farmoni_agent"
        targetFile := "/tmp/farmoni_agent"

        // command for ssh run
        cmd := "/tmp/farmoni_agent &"

        // Connect to the server for scp
        scpCli, err := scp.Connect(userName, keyPath, serverPort)
        if err != nil {
                fmt.Println("Couldn't establisch a connection to the remote server ", err)
                return err
        }

        // copy agent into the server.
        if err := scp.Copy(scpCli, sourceFile, targetFile); err !=nil {
                fmt.Println("Error while copying file ", err)
                return err
        }

        // close the session
        scp.Close(scpCli)


        // Connect to the server for ssh
        sshCli, err := sshrun.Connect(userName, keyPath, serverPort)
        if err != nil {
                fmt.Println("Couldn't establisch a connection to the remote server ", err)
                return err
        }

        if err := sshrun.RunCommand(sshCli, cmd); err != nil {
                fmt.Println("Error while running cmd: " + cmd, err)
                return err
        }

        sshrun.Close(sshCli)

	return err
}


// 1.1. create Servers(VM).
// 1.2. get servers' public IP.
// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
// 1.5. add server list into etcd.
func addVMgcp(count int) ([]*string, []*string) {
// ==> GCP-GCE

/*
    credentialFile := "/root/.gcp/credentials"
    svc := gcehandler.Connect(credentialFile)

    region := "us-east1"
    zone := "us-east1-c"
    projectID := "ornate-course-236606"
    prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID
    imageURL := "projects/gce-uefi-images/global/images/centos-7-v20190326"
    machineType := prefix + "/zones/" + zone + "/machineTypes/f1-micro"
    subNetwork := prefix + "/regions/us-east1/subnetworks/default"
    networkName := prefix + "/global/networks/default"
    serviceAccoutsMail := "default"
    //baseName := "powerkimInstance"
    baseName := "gcepowerkim"

    userName := "byoungseob"
    keyPath := "/root/.gcp/gcppowerkimkeypair.pem"
*/


    credentialFile := masterConfigInfos.GCP.CREDENTIALFILE
    svc := gcehandler.Connect(credentialFile)

// 1.1. create Servers(VM).
    // some options are static for simple PoC.
    // These must be prepared before.
    region := masterConfigInfos.GCP.REGION
    zone := masterConfigInfos.GCP.ZONE
    projectID := masterConfigInfos.GCP.PROJECTID
    //prefix := masterConfigInfos.GCP.PREFIX
    imageURL := masterConfigInfos.GCP.IMAGEID
    machineType := masterConfigInfos.GCP.INSTANCETYPE
    subNetwork := masterConfigInfos.GCP.SUBNETID
    networkName := masterConfigInfos.GCP.NETWORKNAME
    serviceAccoutsMail := masterConfigInfos.GCP.SERVICEACCOUTSMAIL
    baseName := masterConfigInfos.GCP.INSTANCENAMEPREFIX

    userName := masterConfigInfos.GCP.USERNAME  // byoungseob
    keyPath := masterConfigInfos.GCP.KEYFILEPATH  // /root/.gcp/gcppowerkimkeypair.pem

    instanceIds := gcehandler.CreateInstances(svc, region, zone, projectID, imageURL, machineType, 1, count,
        subNetwork, networkName, serviceAccoutsMail, baseName)

    for _, v := range instanceIds {
        fmt.Println("\tInstanceName: ", *v)
    }


    publicIPs := make([]*string, len(instanceIds))
// 1.2. get servers' public IP.
    // waiting for completion of new instance running.
    // after then, can get publicIP.
    for k, v := range instanceIds {
            // wait until running status

        fmt.Println("===========> ", svc, zone, projectID, *v)
            gcehandler.WaitForRun(svc, zone, projectID, *v)

            // get public IP
            publicIP := gcehandler.GetPublicIP(svc, zone, projectID, *v)
            fmt.Println("==============> " + publicIP);
            publicIPs[k] = &publicIP
    }

// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
    for _, v := range publicIPs {
            for i:=0; ; i++ {
                err:=copyAndPlayAgent(*v, userName, keyPath)
                if(i==30) { os.Exit(3) }
                    if err == nil {
                        break;
                    }
                    // need to load SSH Service on the VM
                    time.Sleep(time.Second*3)
            } // end of for
    } // end of for

// 1.5. add server list into etcd.
    addServersToEtcd("gcp", instanceIds, publicIPs)

    return instanceIds, publicIPs
}


// 1.1. create Servers(VM).
// 1.2. get servers' public IP.
// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
// 1.5. add server list into etcd.
func addVMazure(count int) ([]*string, []*string) {
// ==> AZURE-Compute

/*
const (
        groupName = "VMGroupName"
        location = "westus2"
        virtualNetworkName = "virtualNetworkName"
        subnet1Name = "subnet1Name"
        subnet2Name = "subnet2Name"
        nsgName = "nsgName"
        ipName = "ipName"
        nicName = "nicName"

        baseName = "azurepowerkim"
        vmUserName = "powerkim"
        vmPassword = "powerkim"
	keyPath := "/root/.azure/azurepowerkimkeypair.pem"
        sshPublicKeyPath = "/root/.azure/azurepublickey.pem"
)
*/


    credentialFile := masterConfigInfos.AZURE.CREDENTIALFILE
    connInfo := azurehandler.Connect(credentialFile)

// 1.1. create Servers(VM).
    // some options are static for simple PoC.
    // These must be prepared before.
	groupName := masterConfigInfos.AZURE.GROUPNAME
        location := masterConfigInfos.AZURE.LOCATION
        virtualNetworkName := masterConfigInfos.AZURE.VIRTUALNETWORKNAME
        subnet1Name := masterConfigInfos.AZURE.SUBNET1NAME
        subnet2Name := masterConfigInfos.AZURE.SUBNET2NAME
        nsgName := masterConfigInfos.AZURE.NETWORKSECURITYGROUPNAME
//        ipName := masterConfigInfos.AZURE.IPNAME
//        nicName := masterConfigInfos.AZURE.NICNAME

        baseName := masterConfigInfos.AZURE.BASENAME
        vmUserName := masterConfigInfos.AZURE.USERNAME
        vmPassword := masterConfigInfos.AZURE.PASSWORD
        KeyPath := masterConfigInfos.AZURE.KEYFILEPATH
        sshPublicKeyPath := masterConfigInfos.AZURE.PUBLICKEYFILEPATH


        _, err := azurehandler.CreateGroup(connInfo, groupName, location)
        if err != nil {
                fmt.Println(err.Error())
        }
        _, err = azurehandler.CreateVirtualNetworkAndSubnets(connInfo, groupName, location, virtualNetworkName, subnet1Name, subnet2Name)

        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created vnet and 2 subnets")

        _, err = azurehandler.CreateNetworkSecurityGroup(connInfo, groupName, location, nsgName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created network security group")

/* PublicIP & NIC is made in CreateInstnaces()
        _, err = azurehandler.CreatePublicIP(connInfo, groupName, location, ipName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created public IP")
        _, err = azurehandler.CreateNIC(connInfo, groupName, location, virtualNetworkName, subnet1Name, nsgName, ipName, nicName)
        if err != nil {
                fmt.Println(err.Error())
        }
        fmt.Println("created nic")
*/


/*
type ImageInfo struct {
        Publisher string
        Offer     string
        Sku       string
        Version   string
}
*/
        imageInfo := azurehandler.ImageInfo{"Canonical", "UbuntuServer", "16.04.0-LTS", "latest"}

/*
type VMInfo struct {
        UserName string
        Password string
        SshPublicKeyPath string
}
*/
    vmInfo := azurehandler.VMInfo{vmUserName, vmPassword, sshPublicKeyPath}

/*
type NICInfo struct {
        VirtualNetworkName string
        SubnetName string
        NetworkSecurityGroup string
}
*/

    nicInfo := azurehandler.NICInfo{virtualNetworkName, subnet1Name, nsgName}

    instanceIds := azurehandler.CreateInstances(connInfo, groupName, location, baseName, nicInfo, imageInfo, vmInfo, count)


    for _, v := range instanceIds {
        fmt.Println("\tInstanceName: ", *v)
    }


    publicIPs := make([]*string, len(instanceIds))
// 1.2. get servers' public IP.
    // waiting for completion of new instance running.
    // after then, can get publicIP.
    for i, _ := range instanceIds {
            ipName := baseName + "IP" + strconv.Itoa(i)

            // get public IP
            publicIP, err := azurehandler.GetPublicIP(connInfo, groupName, ipName)
            if(err != nil) {
                fmt.Println(err.Error())
            }

            fmt.Println("==============> " + *publicIP.PublicIPAddressPropertiesFormat.IPAddress);
            publicIPs[i] = publicIP.PublicIPAddressPropertiesFormat.IPAddress

//          fmt.Printf("[PublicIP] %#v", publicIP);
//            fmt.Printf("[PublicIP] %s", *publicIP.PublicIPAddressPropertiesFormat.IPAddress);
    }


// 1.3. insert Farmoni Agent into Servers.
// 1.4. execute Servers' Agent.
    for _, v := range publicIPs {
            for i:=0; ; i++ {
                err:=copyAndPlayAgent(*v, vmUserName, KeyPath)
                if(i==30) { os.Exit(3) }
                    if err == nil {
                        break;
                    }
                    // need to load SSH Service on the VM
                    time.Sleep(time.Second*3)
            } // end of for
    } // end of for

// 1.5. add server list into etcd.
    addServersToEtcd("azure", instanceIds, publicIPs)

    return instanceIds, publicIPs
}


func serverList() {

	list := getServerList()
	fmt.Print("######### all server list....(" + strconv.Itoa(len(list)) + ")\n")


	for _, v := range list {
		vs := strings.Split(string(*v), "/")
		fmt.Println("[CSP] " + vs[0] + "\t/ [VmID] "+ vs[1] +"\t/ [IP] " + vs[2])
	}	
}

func getServerList() []*string {

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        //ctx := context.Background()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
        return etcdhandler.ServerList(ctx, etcdcli)
}

func getInstanceIdListAWS() []*string {
        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        ctx := context.Background()
        return etcdhandler.InstanceIDListAWS(ctx, etcdcli)
}

func getInstanceIdListGCP() []*string {
        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        ctx := context.Background()
        return etcdhandler.InstanceIDListGCP(ctx, etcdcli)
}

func getInstanceIdListAZURE() []*string {
        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }

        defer etcdhandler.Close(etcdcli)

        ctx := context.Background()
        return etcdhandler.InstanceIDListAZURE(ctx, etcdcli)
}

func monitoringAll() {

	for {
		list := getServerList()
		for _, v := range list {
			vs := strings.Split(string(*v), "/")
			println("-----monitoiring for------")
			fmt.Println("[CSP] " + vs[0] + "\t/ [VmID] "+ vs[1] +"\t/ [IP] " + vs[2])

			monitoringServer(vs[2])
			println("-----------")
		}
                println("==============================")
		time.Sleep(time.Second)
	} // end of for
}

func monitoringServer(serverPort string) {

        // Set up a connection to the server.
        conn, err := grpc.Dial(serverPort, grpc.WithInsecure())
        if err != nil {
                log.Fatalf("did not connect: %v", err)
        }
        defer conn.Close()
        c := pb.NewResourceStatClient(conn)

        ctx, cancel := context.WithTimeout(context.Background(), 100*time.Hour)
        defer cancel()


	r, err := c.GetResourceStat(ctx, &pb.ResourceStatRequest{})
	if err != nil {
		log.Fatalf("could not Fetch Resource Status Information: %v", err)
	}
	println("[" + r.Servername + "]")
	log.Printf("%s", r.Cpu)
	log.Printf("%s", r.Mem)
	log.Printf("%s", r.Dsk)

}
