// Proof of Concepts for the Cloud-Barista Multi-Cloud Project.
//      * Cloud-Barista: https://github.com/cloud-barista
//
// ETCD Hander (ETCD Version 3 API, Thanks ETCD.)
//
// by powerkim@powerkim.co.kr, 2019.03.
package etcdhandler

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"time"
	"log"
	"strings"
)


func Connect(moniServerPort *string) (*clientv3.Client, error) {
	etcdcli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://" + *moniServerPort},
                //DialTimeout: 5 * time.Second,
                DialTimeout: 3 * time.Second,
        })

	return etcdcli, err
}

func Close(cli *clientv3.Client) {
	cli.Close()
}

// manage service resources

func AddService(ctx context.Context, cli *clientv3.Client, svcId *string, svcName *string, fetchtype *string) {
        // /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
        cli.Put(ctx, "/service/"+ *svcId + "/" + *svcName, *fetchtype)
        fmt.Println("added a " + *svcId + "-"+ *svcName + " into the Service List...\n")
}

func AddServerToService(ctx context.Context, cli *clientv3.Client, svcId *string, svcName *string, provider *string, instanceId *string, addserver *string, fetchtype *string) {
        // /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
        cli.Put(ctx, "/service/"+ *svcId + "/" + *svcName + "/server/"+ *provider + "/" + *instanceId + "/" + *addserver, *fetchtype)
        fmt.Println("added a server" + *svcId + "<-"+ *instanceId + " into Service...\n")
}

func GetServersInService(ctx context.Context, cli *clientv3.Client, serviceId *string) []*string {
        // get with prefix, all list of /service's key
        resp, err := cli.Get(ctx, "/service/" + *serviceId, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        serverList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/"), ev.Value)
                // /server/aws/i-1234567890abcdef0/129.254.175:2019

                //tmpStr := strings.Trim(string(ev.Key), "/server/")
                tmpStrs := strings.Split(string(ev.Key), "/server/")
                serverList[k] = &tmpStrs[1]
                //serverList[k] = &tmpStr
        }

        return serverList
}


/*
func AddServerToSvc(ctx context.Context, cli *clientv3.Client, provider *string, instanceId *string, addserver *string, fetchtype *string) {
        // /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
        cli.Put(ctx, "/server/"+ *provider + "/" + *instanceId + "/" + *addserver, *fetchtype)
        fmt.Println("added a " + *addserver + " into the Server List...\n")
}


func DelSvc(ctx context.Context, cli *clientv3.Client, delserver *string) {
        // get with prefix, all list of /server's key
        resp, err := cli.Get(ctx, "/server", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        for _, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/"), ev.Value)
                // /server/aws/i-1234567890abcdef0/129.254.175:2019
                if strings.Contains(string(ev.Key), *delserver) {
                        cli.Delete(ctx, string(ev.Key))
                        fmt.Println("deleted a " + *delserver + " from the Server List...\n")
                }
        }
}
*/
func DelAllSvcs(ctx context.Context, cli *clientv3.Client) {
        // delete all list of /server's key with prefix
        _, err:=cli.Delete(ctx, "/service", clientv3.WithPrefix())
        fmt.Println("deleted all service list...\n")
        if err != nil {
                log.Fatal(err)
        }

}

func ServiceList(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /service's key
        resp, err := cli.Get(ctx, "/service", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        serviceList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/"), ev.Value)
                // /server/aws/i-1234567890abcdef0/129.254.175:2019

                tmpStr := strings.Trim(string(ev.Key), "/service/")
                //tmpStrs := strings.Split(string(tmpStr), "/")
                //serverList[k] = &tmpStrs[2]
                serviceList[k] = &tmpStr
        }

        return serviceList
}



// manage server resources

func AddServer(ctx context.Context, cli *clientv3.Client, provider *string, instanceId *string, addserver *string, fetchtype *string) {
	// /server/aws/i-1234567890abcdef0/129.254.175:2019  PULL
	cli.Put(ctx, "/server/"+ *provider + "/" + *instanceId + "/" + *addserver, *fetchtype)
	fmt.Println("added a " + *addserver + " into the Server List...\n")
}

func DelServer(ctx context.Context, cli *clientv3.Client, delserver *string) {
        // get with prefix, all list of /server's key
        resp, err := cli.Get(ctx, "/server", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        for _, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/"), ev.Value)
                // /server/aws/i-1234567890abcdef0/129.254.175:2019
                if strings.Contains(string(ev.Key), *delserver) {
                        cli.Delete(ctx, string(ev.Key))
                        fmt.Println("deleted a " + *delserver + " from the Server List...\n")
                }
        }
}

func DelAllServers(ctx context.Context, cli *clientv3.Client) {
	// delete all list of /server's key with prefix
	_, err:=cli.Delete(ctx, "/server", clientv3.WithPrefix())
	fmt.Println("deleted all server list...\n")
        if err != nil {
                log.Fatal(err)
        }

}

func DelProviderServer(ctx context.Context, cli *clientv3.Client, provider *string, delserver *string) {
        // get with prefix, all list of /server's key
        resp, err := cli.Get(ctx, "/server/" + *provider, clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        for _, ev := range resp.Kvs {
                // /server/aws/i-1234567890abcdef0/129.254.175:2019
                if strings.Contains(string(ev.Key), *delserver) {
                        cli.Delete(ctx, string(ev.Key))
                        fmt.Println("deleted a " + *delserver + " from the Server List...\n")
			break;
                }
        }
}

func DelProviderAllServers(ctx context.Context, cli *clientv3.Client, provider *string) {
	// delete all list of /server/aws's key with prefix
        _, err:=cli.Delete(ctx, "/server/" + *provider, clientv3.WithPrefix())
        fmt.Printf("deleted all %s server list...\n", *provider)
        if err != nil {
                log.Fatal(err)
        }
}


func ServerList(ctx context.Context, cli *clientv3.Client) []*string {
	// get with prefix, all list of /server's key
	resp, err := cli.Get(ctx, "/server", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	if err != nil {
		log.Fatal(err)
	}

	serverList := make([]*string, len(resp.Kvs))
	for k, ev := range resp.Kvs {
		//fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/"), ev.Value)
		// /server/aws/i-1234567890abcdef0/129.254.175:2019

		tmpStr := strings.Trim(string(ev.Key), "/server/")
                //tmpStrs := strings.Split(string(tmpStr), "/")
                //serverList[k] = &tmpStrs[2]
		serverList[k] = &tmpStr
	}

	return serverList
}

func InstanceIDListAWS(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /server/aws's key
        resp, err := cli.Get(ctx, "/server/aws", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        idList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/aws"), ev.Value)

                // /server/aws/i-1234567890abcdef0/129.254.175:2019
                tmpStr := strings.Split(string(ev.Key), "/server/aws")
                tmpStrs := strings.Split(string(tmpStr[1]), "/")
                idList[k] = &tmpStrs[1]
        }

        return idList
}

func InstanceIDListGCP(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /server/gcp's key
        resp, err := cli.Get(ctx, "/server/gcp", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        idList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/gcp"), ev.Value)

                // /server/gcp/gcepowerkim0/129.254.175:2019
                tmpStr := strings.Split(string(ev.Key), "/server/gcp")
                tmpStrs := strings.Split(string(tmpStr[1]), "/")
                idList[k] = &tmpStrs[1]
        }

        return idList
}

func InstanceIDListAZURE(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /server/azure's key
        resp, err := cli.Get(ctx, "/server/azure", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        idList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/azure"), ev.Value)

                // /server/azure/gcepowerkim0/129.254.175:2019
                tmpStr := strings.Split(string(ev.Key), "/server/azure")
                tmpStrs := strings.Split(string(tmpStr[1]), "/")
                idList[k] = &tmpStrs[1]
        }

        return idList
}


func ServerListAWS(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /server/aws's key
        resp, err := cli.Get(ctx, "/server/aws", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        serverList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/aws"), ev.Value)

		// /server/aws/i-1234567890abcdef0/129.254.175:2019
                tmpStr := strings.Trim(string(ev.Key), "/server/aws")
                tmpStrs := strings.Split(string(tmpStr), "/")
                serverList[k] = &tmpStrs[1]
        }

        return serverList
}

func ServerListGCP(ctx context.Context, cli *clientv3.Client) []*string {
        // get with prefix, all list of /server/gcp's key
        resp, err := cli.Get(ctx, "/server/gcp", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
        if err != nil {
                log.Fatal(err)
        }

        serverList := make([]*string, len(resp.Kvs))
        for k, ev := range resp.Kvs {
                //fmt.Printf("%s : %s\n", strings.Trim(string(ev.Key), "/server/gcp"), ev.Value)

                // /server/gcp/i-1234567890abcdef0/129.254.175:2019
                tmpStr := strings.Trim(string(ev.Key), "/server/gcp")
                tmpStrs := strings.Split(string(tmpStr), "/")
                serverList[k] = &tmpStrs[1]
        }

        return serverList
}

