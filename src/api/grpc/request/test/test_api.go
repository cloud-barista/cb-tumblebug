package main

import (
	"fmt"
	"time"

	sp_api "github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	tb_api "github.com/cloud-barista/cb-tumblebug/src/api/grpc/request"
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

// SimpleNSApiTest - 간단한 NS API 호출
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

	/* 서버가 TLS 가 설정된 경우
	err = ns.SetTLSCA(os.Getenv("CBTUMBLEBUG_ROOT") + "/certs/ca.crt")
	if err != nil {
		logger.Fatal(err)
	}
	*/

	/* 서버가 JWT 인증이 설정된 경우
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

// ConfigNSApiTest - 환경설정파일을 이용한 NS API 호출
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

// DocTypeNSApiTest - 입력/출력 타입을 이용한 NS API 호출
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

	// 입력타입이 json 이고 출력타입이 Json 경우
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

	// 출력타입을 yaml 로 변경
	err = ns.SetOutType("yaml")
	if err != nil {
		logger.Fatal(err)
	}

	result, err = ns.GetNS(doc)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nyaml result :\n%s\n", result)

	// 입력타입을 yaml 로 변경
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

	// 출력타입을 json 로 변경하고 파라미터로 정보 입력
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

// ConfigMCIRApiTest - 환경설정파일을 이용한 MCIR API 호출
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

// ConfigMCISApiTest - 환경설정파일을 이용한 MCIS API 호출
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

	result, err := mcis.GetMcisStatusByParam("ns-test", "mcis-01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcis.Close()
}

// CreateCIMApiTest - 파라미터를 이용한 Create CIM API 호출
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
			sp_api.KeyValue{Key: "IdentityEndpoint", Value: "http://192.168.201.208:5000/v3"},
			sp_api.KeyValue{Key: "Username", Value: "demo"},
			sp_api.KeyValue{Key: "Password", Value: "openstack"},
			sp_api.KeyValue{Key: "DomainName", Value: "Default"},
			sp_api.KeyValue{Key: "ProjectID", Value: "b31474c562184bcbaf3496e08f5a6a4c"},
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
			sp_api.KeyValue{Key: "Region", Value: "RegionOne"},
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

// CreateNSApiTest - 파라미터를 이용한 Create NS API 호출
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

	reqNs := &tb_api.NsReq{
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

// CreateMCIRApiTest - 파라미터를 이용한 Create MCIR API 호출
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
		Item: tb_api.TbVNetReq{
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			CidrBlock:      "192.168.0.0/16",
			SubnetInfoList: []tb_api.SpiderSubnetReqInfo{
				tb_api.SpiderSubnetReqInfo{
					Name:         "openstack-config01-test",
					IPv4_CIDR:    "192.168.1.0/24",
					KeyValueList: []tb_api.KeyValue{},
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
		Item: tb_api.TbImageInfo{
			Id:             "",
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			CspImageId:     "cirros-0.5.1",
			CspImageName:   "",
			Description:    "cirros image",
			CreationDate:   "",
			GuestOS:        "cirros",
			Status:         "",
			KeyValueList:   []tb_api.KeyValue{},
		},
	}
	result, err = mcir.CreateImageWithInfoByParam(reqTbImage)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbSecurityGroup := &tb_api.TbSecurityGroupCreateRequest{
		NsId: "ns-test",
		Item: tb_api.TbSecurityGroupReq{
			Name:           "openstack-config01-test",
			ConnectionName: "openstack-config01",
			VNetId:         "openstack-config01-test",
			Description:    "test description",
			FirewallRules: &[]tb_api.SpiderSecurityRuleInfo{
				tb_api.SpiderSecurityRuleInfo{
					FromPort:   "1",
					ToPort:     "65535",
					IPProtocol: "tcp",
					Direction:  "inbound",
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
		Item: tb_api.TbSpecInfo{
			Id:                    "",
			Name:                  "openstack-config01-test",
			ConnectionName:        "openstack-config01",
			CspSpecName:           "m1.tiny",
			Os_type:               "",
			Num_vCPU:              0,
			Num_core:              0,
			Mem_GiB:               0,
			Storage_GiB:           0,
			Description:           "",
			Cost_per_hour:         0,
			Num_storage:           0,
			Max_num_storage:       0,
			Max_total_storage_TiB: 0,
			Net_bw_Gbps:           0,
			Ebs_bw_Mbps:           0,
			Gpu_model:             "",
			Num_gpu:               0,
			Gpumem_GiB:            0,
			Gpu_p2p:               "",
		},
	}
	result, err = mcir.CreateSpecWithInfoByParam(reqTbSpecInfo)
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	reqTbSshKey := &tb_api.TbSshKeyCreateRequest{
		NsId: "ns-test",
		Item: tb_api.TbSshKeyReq{
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

// CreateMCISApiTest - 파라미터를 이용한 Create MCIS API 호출
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
		Item: tb_api.TbMcisReq{
			Name:           "mcis-01",
			Placement_algo: "",
			Description:    "",
			Vm: []tb_api.TbVmReq{
				tb_api.TbVmReq{
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
				},
				tb_api.TbVmReq{
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

// DeleteMCISApiTest - 파라미터를 이용한 Delete MCIS API 호출
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

	result, err := mcis.DeleteMcisByParam("ns-test", "mcis-01")
	if err != nil {
		logger.Fatal(err)
	}

	fmt.Printf("\nresult :\n%s\n", result)

	mcis.Close()
}

// DeleteMCIRApiTest - 파라미터를 이용한 Delete MCIR API 호출
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

// DeleteNSApiTest - 파라미터를 이용한 Delete NS API 호출
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

// DeleteCIMApiTest - 파라미터를 이용한 Delete CIM API 호출
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
