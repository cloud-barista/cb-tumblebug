package main

import (
	"fmt"
	"time"

	sp_api "github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	tb_api "github.com/cloud-barista/cb-tumblebug/src/api/grpc/request"

	core_common "github.com/cloud-barista/cb-tumblebug/src/core/common"
	core_mcir "github.com/cloud-barista/cb-tumblebug/src/core/mcir"
	core_mcis "github.com/cloud-barista/cb-tumblebug/src/core/mcis"
)

func main() {
	SimpleNSApiTest()
	ConfigNSApiTest()
	DocTypeNSApiTest()

	ConfigMCIRApiTest()
	ConfigMCISApiTest()

	CreateCIMApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)

	CreateNSApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)

	CreateMCIRApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)

	CreateMCISApiTest()
	fmt.Print("\n\n============= 60 seconds waiting.. =============\n")
	time.Sleep(60 * time.Second)

	DeleteMCISApiTest()
	fmt.Print("\n\n============= 60 seconds waiting.. =============\n")
	time.Sleep(60 * time.Second)

	DeleteMCIRApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)

	DeleteNSApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)

	DeleteCIMApiTest()
	fmt.Print("\n\n============= 3 seconds waiting.. =============\n")
	time.Sleep(3 * time.Second)
}

// SimpleNSApiTest
func SimpleNSApiTest() {

	fmt.Print("\n\n============= SimpleNSApiTest() =============\n")

	logger := logger.NewLogger()

	ns := tb_api.NewNSManager()

	err := ns.SetServerAddr("localhost:50252")
	if err != nil {
		logger.Fatal(err)
	}

	err = ns.SetTimeout(90 * time.Second)
	if err != nil {
		logger.Fatal(err)
	}

	/* TLS enabled
	err = ns.SetTLSCA(os.Getenv("CBTUMBLEBUG_ROOT") + "/certs/ca.crt")
	if err != nil {
		logger.Fatal(err)
	}
	*/

	/* JWT enabled
	err = ns.SetJWTToken("xxxxxxxxxxxxxxxxxxx")
	if err != nil {
		logger.Fatal(err)
	}
	*/

	err = ns.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := ns.ListNS()
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	ns.Close()
}

// ConfigNSApiTest is to Call NS API using env config file
func ConfigNSApiTest() {

	fmt.Print("\n\n============= ConfigNSApiTest() =============\n")

	logger := logger.NewLogger()

	ns := tb_api.NewNSManager()

	err := ns.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = ns.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := ns.ListNS()
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	ns.Close()
}

// DocTypeNSApiTest is to Call NS API using input/output
func DocTypeNSApiTest() {

	fmt.Print("\n\n============= DocTypeNSApiTest() =============\n")

	logger := logger.NewLogger()

	ns := tb_api.NewNSManager()

	err := ns.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = ns.Open()
	if err != nil {
		logger.Fatal(err)
	}

	// for JSON input JSON output
	err = ns.SetInType("json")
	if err != nil {
		logger.Fatal(err)
	}
	err = ns.SetOutType("json")
	if err != nil {
		logger.Fatal(err)
	}

	doc := `{
		"name":"ns-test",
		"description": "NameSpace for General Testing"
	}`
	result, err := ns.CreateNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	doc = `{
		"nsId":"ns-test"
	}`
	result, err = ns.GetNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\njson result :\n%s\n", result)

	// Change output into yaml
	err = ns.SetOutType("yaml")
	if err != nil {
		logger.Fatal(err)
	}

	result, err = ns.GetNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nyaml result :\n%s\n", result)

	// Change input into yaml
	err = ns.SetInType("yaml")
	if err != nil {
		logger.Fatal(err)
	}

	doc = `
nsId: ns-test
`
	result, err = ns.GetNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nyaml result :\n%s\n", result)

	// Change output into JSON and provide parameter info
	err = ns.SetOutType("json")
	if err != nil {
		logger.Fatal(err)
	}

	result, err = ns.GetNSByParam("ns-test")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\njson result :\n%s\n", result)

	doc = `
nsId: ns-test
`
	result, err = ns.DeleteNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\njson result :\n%s\n", result)

	ns.Close()
}

// ConfigMCIRApiTest is to Call MCIR API using env config file
func ConfigMCIRApiTest() {

	fmt.Print("\n\n============= ConfigMCIRApiTest() =============\n")

	logger := logger.NewLogger()

	mcir := tb_api.NewMCIRManager()

	err := mcir.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcir.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := mcir.ListVNetByParam("ns-test")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcir.Close()
}

// ConfigMCISApiTest is to Call MCIS API using env config file
func ConfigMCISApiTest() {

	fmt.Print("\n\n============= ConfigMCISApiTest() =============\n")

	logger := logger.NewLogger()

	mcis := tb_api.NewMCISManager()

	err := mcis.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcis.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := mcis.ListMcisByParam("ns-test")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcis.Close()
}

// CreateCIMApiTest is to Call Create CIM API using parameter
func CreateCIMApiTest() {

	fmt.Print("\n\n============= CreateCIMApiTest() =============\n")

	logger := logger.NewLogger()

	cim := sp_api.NewCloudInfoManager()

	err := cim.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = cim.Open()
	if err != nil {
		logger.Fatal(err)
	}

	reqCloudDriver := &sp_api.CloudDriverReq{
		DriverName:        "openstack-driver01",
		ProviderName:      "OPENSTACK",
		DriverLibFileName: "openstack-driver-v1.0.so",
	}
	result, err := cim.CreateCloudDriverByParam(reqCloudDriver)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqCredential := &sp_api.CredentialReq{
		CredentialName: "openstack-credential01",
		ProviderName:   "OPENSTACK",
		KeyValueInfoList: []sp_api.KeyValue{
			{Key: "IdentityEndpoint", Value: "http://192.168.201.208:5000/v3"},
			{Key: "Username", Value: "demo"},
			{Key: "Password", Value: "openstack"},
			{Key: "DomainName", Value: "Default"},
			{Key: "ProjectID", Value: "b31474c562184bcbaf3496e08f5a6a4c"},
		},
	}
	result, err = cim.CreateCredentialByParam(reqCredential)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqRegion := &sp_api.RegionReq{
		RegionName:   "openstack-region01",
		ProviderName: "OPENSTACK",
		KeyValueInfoList: []sp_api.KeyValue{
			{Key: "Region", Value: "RegionOne"},
		},
	}
	result, err = cim.CreateRegionByParam(reqRegion)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqConnectionConfig := &sp_api.ConnectionConfigReq{
		ConfigName:     "openstack-config01",
		ProviderName:   "OPENSTACK",
		DriverName:     "openstack-driver01",
		CredentialName: "openstack-credential01",
		RegionName:     "openstack-region01",
	}
	result, err = cim.CreateConnectionConfigByParam(reqConnectionConfig)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	cim.Close()
}

// CreateNSApiTest is to Call Create NS API using parameter
func CreateNSApiTest() {

	fmt.Print("\n\n============= CreateNSApiTest() =============\n")

	logger := logger.NewLogger()

	ns := tb_api.NewNSManager()

	err := ns.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = ns.Open()
	if err != nil {
		logger.Fatal(err)
	}

	reqNs := &core_common.NsReq{
		Name:        "ns-test",
		Description: "NameSpace for General Testing",
	}
	result, err := ns.CreateNSByParam(reqNs)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	ns.Close()
}

// CreateMCIRApiTest is to Call Create MCIR API using parameter
func CreateMCIRApiTest() {

	fmt.Print("\n\n============= CreateMCIRApiTest() =============\n")

	logger := logger.NewLogger()

	mcir := tb_api.NewMCIRManager()

	err := mcir.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcir.Open()
	if err != nil {
		logger.Fatal(err)
	}

	reqTbVNet := &tb_api.TbVNetCreateRequest{
		NsId: "ns-test",
		Item: core_mcir.TbVNetReq{
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			CidrBlock:      "192.168.0.0/16",
			SubnetInfoList: []core_mcir.TbSubnetReq{
				{
					Name:         "openstack-config01-test",
					IPv4_CIDR:    "192.168.1.0/24",
					KeyValueList: []core_common.KeyValue{},
				},
			},
			Description: "",
		},
	}
	result, err := mcir.CreateVNetByParam(reqTbVNet)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbImage := &tb_api.TbImageInfoRequest{
		NsId: "ns-test",
		Item: core_mcir.TbImageInfo{
			Id:             "",
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			CspImageId:     "cirros-0.5.1",
			CspImageName:   "",
			Description:    "cirros image",
			CreationDate:   "",
			GuestOS:        "cirros",
			Status:         "",
			KeyValueList:   []core_common.KeyValue{},
		},
	}
	result, err = mcir.CreateImageWithInfoByParam(reqTbImage)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbSecurityGroup := &tb_api.TbSecurityGroupCreateRequest{
		NsId: "ns-test",
		Item: core_mcir.TbSecurityGroupReq{
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			VNetId:         "openstack-config01-test",
			Description:    "test description",
			FirewallRules: &[]core_mcir.TbFirewallRuleInfo{
				{
					FromPort:   "1",
					ToPort:     "65535",
					IPProtocol: "tcp",
					Direction:  "inbound",
					CIDR:       "0.0.0.0/0",
				},
			},
		},
	}
	result, err = mcir.CreateSecurityGroupByParam(reqTbSecurityGroup)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbSpecInfo := &tb_api.TbSpecInfoRequest{
		NsId: "ns-test",
		Item: core_mcir.TbSpecInfo{
			Id:                    "",
			Name:                  "openstack-config01-test",
			ConnectionName:        "openstack-config01",
			CspSpecName:           "m1.tiny",
			OsType:                "",
			NumvCPU:               0,
			NumCore:               0,
			MemGiB:                0,
			StorageGiB:            0,
			Description:           "",
			CostPerHour:           0,
			NumStorage:            0,
			MaxNumStorage:         0,
			MaxTotalStorageTiB:    0,
			NetBwGbps:             0,
			EbsBwMbps:             0,
			GpuModel:              "",
			NumGpu:                0,
			GpuMemGiB:             0,
			GpuP2p:                "",
			OrderInFilteredResult: 0,
			EvaluationStatus:      "",
			EvaluationScore01:     0.0,
			EvaluationScore02:     0.0,
			EvaluationScore03:     0.0,
			EvaluationScore04:     0.0,
			EvaluationScore05:     0.0,
			EvaluationScore06:     0.0,
			EvaluationScore07:     0.0,
			EvaluationScore08:     0.0,
			EvaluationScore09:     0.0,
			EvaluationScore10:     0.0,
		},
	}
	result, err = mcir.CreateSpecWithInfoByParam(reqTbSpecInfo)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbSshKey := &tb_api.TbSshKeyCreateRequest{
		NsId: "ns-test",
		Item: core_mcir.TbSshKeyReq{
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			Description:    "",
		},
	}
	result, err = mcir.CreateSshKeyByParam(reqTbSshKey)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcir.Close()
}

// CreateMCISApiTest is to Call Create MCIS API using parameter
func CreateMCISApiTest() {

	fmt.Print("\n\n============= CreateMCISApiTest() =============\n")

	logger := logger.NewLogger()

	mcis := tb_api.NewMCISManager()

	err := mcis.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcis.Open()
	if err != nil {
		logger.Fatal(err)
	}

	reqTbMcis := &tb_api.TbMcisCreateRequest{
		NsId: "ns-test",
		Item: core_mcis.TbMcisReq{
			Name:            "mcis-01",
			PlacementAlgo:   "",
			InstallMonAgent: "no",
			Description:     "",
			Label:           "",
			Vm: []core_mcis.TbVmReq{
				{
					SubGroupSize:   "0",
					Name:           "openstack-config01-test-01",
					ConnectionName: "openstack-config01",
					SpecId:         "openstack-config01-test",
					ImageId:        "openstack-config01-test",
					VNetId:         "openstack-config01-test",
					SubnetId:       "openstack-config01-test",
					SecurityGroupIds: []string{
						"openstack-config01-test",
					},
					SshKeyId:       "openstack-config01-test",
					VmUserAccount:  "cb-user",
					VmUserPassword: "",
					Description:    "description",
					Label:          "label",
				},
				{
					SubGroupSize:   "0",
					Name:           "openstack-config01-test-02",
					ConnectionName: "openstack-config01",
					SpecId:         "openstack-config01-test",
					ImageId:        "openstack-config01-test",
					VNetId:         "openstack-config01-test",
					SubnetId:       "openstack-config01-test",
					SecurityGroupIds: []string{
						"openstack-config01-test",
					},
					SshKeyId:       "openstack-config01-test",
					VmUserAccount:  "cb-user",
					VmUserPassword: "",
					Description:    "description",
					Label:          "label",
				},
			},
		},
	}

	result, err := mcis.CreateMcisByParam(reqTbMcis)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcis.Close()
}

// DeleteMCISApiTest is to Call Delete MCIS API using parameter
func DeleteMCISApiTest() {

	fmt.Print("\n\n============= DeleteMCISApiTest() =============\n")

	logger := logger.NewLogger()

	mcis := tb_api.NewMCISManager()

	err := mcis.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcis.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := mcis.ControlMcisByParam("ns-test", "mcis-01", "terminate")
	if err != nil {
		logger.Fatal(err)
	}

	result, err = mcis.DeleteMcisByParam("ns-test", "mcis-01", "")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcis.Close()
}

// DeleteMCIRApiTest is to Call Delete MCIR API using parameter
func DeleteMCIRApiTest() {

	fmt.Print("\n\n============= DeleteMCIRApiTest() =============\n")

	logger := logger.NewLogger()

	mcir := tb_api.NewMCIRManager()

	err := mcir.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = mcir.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := mcir.DeleteSpecByParam("ns-test", "openstack-config01-test", "false")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = mcir.DeleteImageByParam("ns-test", "openstack-config01-test", "false")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = mcir.DeleteSshKeyByParam("ns-test", "openstack-config01-test", "false")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = mcir.DeleteSecurityGroupByParam("ns-test", "openstack-config01-test", "false")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = mcir.DeleteVNetByParam("ns-test", "openstack-config01-test", "false")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcir.Close()
}

// DeleteNSApiTest is to Call Delete NS API using parameter
func DeleteNSApiTest() {

	fmt.Print("\n\n============= DeleteNSApiTest() =============\n")

	logger := logger.NewLogger()

	ns := tb_api.NewNSManager()

	err := ns.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = ns.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := ns.DeleteNSByParam("ns-test")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	ns.Close()
}

// DeleteCIMApiTest is to Call Delete CIM API using parameter
func DeleteCIMApiTest() {

	fmt.Print("\n\n============= DeleteCIMApiTest() =============\n")

	logger := logger.NewLogger()

	cim := sp_api.NewCloudInfoManager()

	err := cim.SetConfigPath("../../cbadm/grpc_conf.yaml")
	if err != nil {
		logger.Fatal(err)
	}

	err = cim.Open()
	if err != nil {
		logger.Fatal(err)
	}

	result, err := cim.DeleteCloudDriverByParam("openstack-driver01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = cim.DeleteCredentialByParam("openstack-credential01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = cim.DeleteRegionByParam("openstack-region01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	result, err = cim.DeleteConnectionConfigByParam("openstack-config01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	cim.Close()
}
