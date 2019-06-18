// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// test for etcdehandler.
//
//   ex) test -etcdserver=129.254.175.43:2379 -addserver=node01:5000
//   ex) test -etcdserver=129.254.175.43:2379 -delserver=node01:5000
//   ex) test -etcdserver=129.254.175.43:2379 -serverlist
// by powerkim@powerkim.co.kr, 2019.03.
 package main

 import (
         "flag"
         "github.com/cloud-barista/poc-farmoni/farmoni_master/etcdhandler"
	 "context"
         "fmt"
 )

 func main() {
        etcdServerPort := flag.String("etcdserver", "129.254.175.43:2379", "etcdserver=129.254.175.43:2379")
        fetchType := flag.String("fetchtype", "PULL", "fetch type: -fetchtype=PUSH")
        addServer := flag.String("addserver", "none", "add a server: -addserver=192.168.0.10:5000")
        delServer := flag.String("delserver", "none", "delete a server: -delserver=192.168.0.10:5000")
        serverlist := flag.Bool("serverlist", false, "report server list: -serverlist")
        flag.Parse()

        etcdcli, err := etcdhandler.Connect(etcdServerPort)
        if err != nil {
                panic(err)
        }
        fmt.Println("connected to etcd - " + *etcdServerPort)

        defer etcdhandler.Close(etcdcli)

        ctx := context.Background()

        if *addServer != "none" {
                fmt.Println("######### addServer....")
                etcdhandler.AddServer(ctx, etcdcli, addServer, fetchType)
        }
        if *delServer != "none" {
                fmt.Println("######### delServer....")
                etcdhandler.DelServer(ctx, etcdcli, delServer)
        }
        if *serverlist != false {
                fmt.Println("######### server list....")
                list := etcdhandler.ServerList(ctx, etcdcli)
		for _, v := range list {
			fmt.Println(*v)
		}
        }

 }
