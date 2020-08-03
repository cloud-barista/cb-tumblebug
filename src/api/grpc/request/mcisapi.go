package request

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/config"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/request/mcis"
	"github.com/cloud-barista/cb-tumblebug/src/core/common"

	"google.golang.org/grpc"
)

// ===== [ Comtants and Variables ] =====

// ===== [ Types ] =====

// MCISApi - MCIS API 구조 정의
type MCISApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientMCIS   pb.MCISClient
	requestMCIS  *mcis.MCISRequest
	inType       string
	outType      string
}

// TbMcisCreateRequest - MCIS 생성 요청 구조 Wrapper 정의
type TbMcisCreateRequest struct {
	NsId string    `yaml:"nsId" json:"nsId"`
	Item TbMcisReq `yaml:"mcis" json:"mcis"`
}

// TbMcisReq - MCIS 생성 요청 구조 정의
type TbMcisReq struct {
	Name           string    `yaml:"name" json:"name"`
	Vm             []TbVmReq `yaml:"vm" json:"vm"`
	Placement_algo string    `yaml:"placement_algo" json:"placement_algo"`
	Description    string    `yaml:"description" json:"description"`
}

// TbVmReq - MCIS VM 생성 요청 구조 정의
type TbVmReq struct {
	Name             string   `yaml:"name" json:"name"`
	ConnectionName   string   `yaml:"connectionName" json:"connectionName"`
	SpecId           string   `yaml:"specId" json:"specId"`
	ImageId          string   `yaml:"imageId" json:"imageId"`
	VNetId           string   `yaml:"vNetId" json:"vNetId"`
	SubnetId         string   `yaml:"subnetId" json:"subnetId"`
	SecurityGroupIds []string `yaml:"securityGroupIds" json:"securityGroupIds"`
	SshKeyId         string   `yaml:"sshKeyId" json:"sshKeyId"`
	VmUserAccount    string   `yaml:"vmUserAccount" json:"vmUserAccount"`
	VmUserPassword   string   `yaml:"vmUserPassword" json:"vmUserPassword"`
	Description      string   `yaml:"description" json:"description"`
}

// TbVmCreateRequest - MCIS VM 생성 요청 구조 Wrapper 정의
type TbVmCreateRequest struct {
	NsId   string   `yaml:"nsId" json:"nsId"`
	McisId string   `yaml:"mcisId" json:"mcisId"`
	Item   TbVmInfo `yaml:"mcisvm" json:"mcisvm"`
}

// TbVmInfo - MCIS VM 구조 정의
type TbVmInfo struct {
	Id               string   `yaml:"id" json:"id"`
	Name             string   `yaml:"name" json:"name"`
	ConnectionName   string   `yaml:"connectionName" json:"connectionName"`
	SpecId           string   `yaml:"specId" json:"specId"`
	ImageId          string   `yaml:"imageId" json:"imageId"`
	VNetId           string   `yaml:"vNetId" json:"vNetId"`
	SubnetId         string   `yaml:"subnetId" json:"subnetId"`
	SecurityGroupIds []string `yaml:"securityGroupIds" json:"securityGroupIds"`
	SshKeyId         string   `yaml:"sshKeyId" json:"sshKeyId"`
	VmUserAccount    string   `yaml:"vmUserAccount" json:"vmUserAccount"`
	VmUserPassword   string   `yaml:"vmUserPassword" json:"vmUserPassword"`
	Description      string   `yaml:"description" json:"description"`

	Location GeoLocation `yaml:"location" json:"location"`

	// 2. Provided by CB-Spider
	Region      RegionInfo `yaml:"region" json:"region"`
	PublicIP    string     `yaml:"publicIP" json:"publicIP"`
	PublicDNS   string     `yaml:"publicDNS" json:"publicDNS"`
	PrivateIP   string     `yaml:"privateIP" json:"privateIP"`
	PrivateDNS  string     `yaml:"privateDNS" json:"privateDNS"`
	VMBootDisk  string     `yaml:"vmBootDisk" json:"vmBootDisk"`
	VMBlockDisk string     `yaml:"vmBlockDisk" json:"vmBlockDisk"`

	// 3. Required by CB-Tumblebug
	Status       string `yaml:"status" json:"status"`
	TargetStatus string `yaml:"targetStatus" json:"targetStatus"`
	TargetAction string `yaml:"targetAction" json:"targetAction"`

	CspViewVmDetail SpiderVMInfo `yaml:"cspViewVmDetail" json:"cspViewVmDetail"`
}

// GeoLocation - 위치 정보 구조 정의
type GeoLocation struct {
	Latitude     string `yaml:"latitude" json:"latitude"`
	Longitude    string `yaml:"longitude" json:"longitude"`
	BriefAddr    string `yaml:"briefAddr" json:"briefAddr"`
	CloudType    string `yaml:"cloudType" json:"cloudType"`
	NativeRegion string `yaml:"nativeRegion" json:"nativeRegion"`
}

// RegionInfo - Region 정보 구조 정의
type RegionInfo struct { // Spider
	Region string `yaml:"Region" json:"Region"`
	Zone   string `yaml:"Zone" json:"Zone"`
}

// SpiderVMInfo - VM 정보 구조 정의
type SpiderVMInfo struct { // Spider
	// Fields for request
	Name               string   `yaml:"Name" json:"Name"`
	ImageName          string   `yaml:"ImageName" json:"ImageName"`
	VPCName            string   `yaml:"VPCName" json:"VPCName"`
	SubnetName         string   `yaml:"SubnetName" json:"SubnetName"`
	SecurityGroupNames []string `yaml:"SecurityGroupNames" json:"SecurityGroupNames"`
	KeyPairName        string   `yaml:"KeyPairName" json:"KeyPairName"`

	// Fields for both request and response
	VMSpecName   string `yaml:"VMSpecName" json:"VMSpecName"`
	VMUserId     string `yaml:"VMUserId" json:"VMUserId"`
	VMUserPasswd string `yaml:"VMUserPasswd" json:"VMUserPasswd"`

	// Fields for response
	IId               IID        `yaml:"IId" json:"IId"`
	ImageIId          IID        `yaml:"ImageIId" json:"ImageIId"`
	VpcIID            IID        `yaml:"VpcIID" json:"VpcIID"`
	SubnetIID         IID        `yaml:"SubnetIID" json:"SubnetIID"`
	SecurityGroupIIds []IID      `yaml:"SecurityGroupIIds" json:"SecurityGroupIIds"`
	KeyPairIId        IID        `yaml:"KeyPairIId" json:"KeyPairIId"`
	StartTime         string     `yaml:"StartTime" json:"StartTime"`
	Region            RegionInfo `yaml:"Region" json:"Region"`
	NetworkInterface  string     `yaml:"NetworkInterface" json:"NetworkInterface"`
	PublicIP          string     `yaml:"PublicIP" json:"PublicIP"`
	PublicDNS         string     `yaml:"PublicDNS" json:"PublicDNS"`
	PrivateIP         string     `yaml:"PrivateIP" json:"PrivateIP"`
	PrivateDNS        string     `yaml:"PrivateDNS" json:"PrivateDNS"`
	VMBootDisk        string     `yaml:"VMBootDisk" json:"VMBootDisk"`
	VMBlockDisk       string     `yaml:"VMBlockDisk" json:"VMBlockDisk"`
	KeyValueList      []KeyValue `yaml:"KeyValueList" json:"KeyValueList"`
}

// McisRecommendCreateRequest - MCIS 추천 요청 구조 Wrapper 정의
type McisRecommendCreateRequest struct {
	NsId string           `yaml:"nsId" json:"nsId"`
	Item McisRecommendReq `yaml:"recommend" json:"recommend"`
}

// McisRecommendReq - MCIS 추천 요청 구조 정의
type McisRecommendReq struct {
	Vm_req          []TbVmRecommendReq `yaml:"vm_req" json:"vm_req"`
	Placement_algo  string             `yaml:"placement_algo" json:"placement_algo"`
	Placement_param []common.KeyValue  `yaml:"placement_param" json:"placement_param"`
	Max_result_num  string             `yaml:"max_result_num" json:"max_result_num"`
}

// McisRecommendReq - MCIS VM 추천 요청 구조 정의
type TbVmRecommendReq struct {
	Request_name   string `yaml:"request_name" json:"request_name"`
	Max_result_num string `yaml:"max_result_num" json:"max_result_num"`

	Vcpu_size   string `yaml:"vcpu_size" json:"vcpu_size"`
	Memory_size string `yaml:"memory_size" json:"memory_size"`
	Disk_size   string `yaml:"disk_size" json:"disk_size"`

	Placement_algo  string     `yaml:"placement_algo" json:"placement_algo"`
	Placement_param []KeyValue `yaml:"placement_param" json:"placement_param"`
}

// McisCmdCreateRequest - MCIS 명령 실행 요청 구조 Wrapper 정의
type McisCmdCreateRequest struct {
	NsId   string     `yaml:"nsId" json:"nsId"`
	McisId string     `yaml:"mcisId" json:"mcisId"`
	Item   McisCmdReq `yaml:"cmd" json:"cmd"`
}

// McisCmdReq - MCIS 명령 실행 요청 구조 정의
type McisCmdReq struct {
	Mcis_id   string `yaml:"mcis_id" json:"mcis_id"`
	Vm_id     string `yaml:"vm_id" json:"vm_id"`
	Ip        string `yaml:"ip" json:"ip"`
	User_name string `yaml:"user_name" json:"user_name"`
	Ssh_key   string `yaml:"ssh_key" json:"ssh_key"`
	Command   string `yaml:"command" json:"command"`
}

// McisCmdVmCreateRequest - MCIS VM 명령 실행 요청 구조 Wrapper 정의
type McisCmdVmCreateRequest struct {
	NsId   string     `yaml:"nsId" json:"nsId"`
	McisId string     `yaml:"mcisId" json:"mcisId"`
	VmId   string     `yaml:"vmId" json:"vmId"`
	Item   McisCmdReq `yaml:"cmd" json:"cmd"`
}

// ===== [ Implementatiom ] =====

// SetServerAddr - Tumblebug 서버 주소 설정
func (m *MCISApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	m.gConf.GSL.TumblebugSrv.Addr = addr
	return nil
}

// GetServerAddr - Tumblebug 서버 주소 값 조회
func (m *MCISApi) GetServerAddr() (string, error) {
	return m.gConf.GSL.TumblebugSrv.Addr, nil
}

// SetTLSCA - TLS CA 설정
func (m *MCISApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if m.gConf.GSL.TumblebugCli.TLS == nil {
		m.gConf.GSL.TumblebugCli.TLS = &config.TLSConfig{}
	}

	m.gConf.GSL.TumblebugCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA - TLS CA 값 조회
func (m *MCISApi) GetTLSCA() (string, error) {
	if m.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout - Timeout 설정
func (m *MCISApi) SetTimeout(timeout time.Duration) error {
	m.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout - Timeout 값 조회
func (m *MCISApi) GetTimeout() (time.Duration, error) {
	return m.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken - JWT 인증 토큰 설정
func (m *MCISApi) SetJWTToken(token string) error {
	if token == "" {
		return errors.New("parameter is empty")
	}

	if m.gConf.GSL.TumblebugCli.Interceptors == nil {
		m.gConf.GSL.TumblebugCli.Interceptors = &config.InterceptorsConfig{}
		m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}
	if m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}

	m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken = token
	return nil
}

// GetJWTToken - JWT 인증 토큰 값 조회
func (m *MCISApi) GetJWTToken() (string, error) {
	if m.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath - 환경설정 파일 설정
func (m *MCISApi) SetConfigPath(configFile string) error {
	logger := logger.NewLogger()

	// Viper 를 사용하는 설정 파서 생성
	parser := config.MakeParser()

	var (
		gConf config.GrpcConfig
		err   error
	)

	if configFile == "" {
		logger.Error("Please, provide the path to your configuration file")
		return errors.New("configuration file are not specified")
	}

	logger.Debug("Parsing configuration file: ", configFile)
	if gConf, err = parser.GrpcParse(configFile); err != nil {
		logger.Error("ERROR - Parsing the configuration file.\n", err.Error())
		return err
	}

	// TUMBLEBUG SERVER 필수 입력 항목 체크
	tumblebugsrv := gConf.GSL.TumblebugSrv

	if tumblebugsrv == nil {
		return errors.New("tumblebugsrv field are not specified")
	}

	if tumblebugsrv.Addr == "" {
		return errors.New("tumblebugsrv.addr field are not specified")
	}

	// TUMBLEBUG CLIENT 필수 입력 항목 체크
	tumblebugcli := gConf.GSL.TumblebugCli

	if tumblebugcli == nil {
		gConf.GSL.TumblebugCli = &config.GrpcClientConfig{Timeout: 90 * time.Second}
		tumblebugcli = gConf.GSL.TumblebugCli
	}

	if tumblebugcli.TLS != nil {
		if tumblebugcli.TLS.TLSCA == "" {
			return errors.New("tumblebugcli.tls.tls_ca field are not specified")
		}
	}

	if tumblebugcli.Interceptors != nil {
		if tumblebugcli.Interceptors.AuthJWT != nil {
			if tumblebugcli.Interceptors.AuthJWT.JWTToken == "" {
				return errors.New("tumblebugcli.interceptors.auth_jwt.jwt_token field are not specified")
			}
		}
		if tumblebugcli.Interceptors.Opentracing != nil {
			if tumblebugcli.Interceptors.Opentracing.Jaeger != nil {
				if tumblebugcli.Interceptors.Opentracing.Jaeger.Endpoint == "" {
					return errors.New("tumblebugcli.interceptors.opentracing.jaeger.endpoint field are not specified")
				}
			}
		}
	}

	m.gConf = &gConf
	return nil
}

// Open - 연결 설정
func (m *MCISApi) Open() error {

	tumblebugsrv := m.gConf.GSL.TumblebugSrv
	tumblebugcli := m.gConf.GSL.TumblebugCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(tumblebugsrv.Addr, tumblebugcli)
	if err != nil {
		return err
	}

	if closer != nil {
		m.jaegerCloser = closer
	}

	m.conn = cbconn.Conn

	// grpc 클라이언트 생성
	m.clientMCIS = pb.NewMCISClient(m.conn)

	// grpc 호출 Wrapper
	m.requestMCIS = &mcis.MCISRequest{Client: m.clientMCIS, Timeout: tumblebugcli.Timeout, InType: m.inType, OutType: m.outType}

	return nil
}

// Close - 연결 종료
func (m *MCISApi) Close() {
	if m.conn != nil {
		m.conn.Close()
	}
	if m.jaegerCloser != nil {
		m.jaegerCloser.Close()
	}

	m.jaegerCloser = nil
	m.conn = nil
	m.clientMCIS = nil
	m.requestMCIS = nil
}

// SetInType - 입력 문서 타입 설정 (json/yaml)
func (m *MCISApi) SetInType(in string) error {
	if in == "json" {
		m.inType = in
	} else if in == "yaml" {
		m.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if m.requestMCIS != nil {
		m.requestMCIS.InType = m.inType
	}

	return nil
}

// GetInType - 입력 문서 타입 값 조회
func (m *MCISApi) GetInType() (string, error) {
	return m.inType, nil
}

// SetOutType - 출력 문서 타입 설정 (json/yaml)
func (m *MCISApi) SetOutType(out string) error {
	if out == "json" {
		m.outType = out
	} else if out == "yaml" {
		m.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if m.requestMCIS != nil {
		m.requestMCIS.OutType = m.outType
	}

	return nil
}

// GetOutType - 출력 문서 타입 값 조회
func (m *MCISApi) GetOutType() (string, error) {
	return m.outType, nil
}

// CreateMcis - MCIS 생성
func (m *MCISApi) CreateMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcis()
}

// CreateMcisByParam - MCIS 생성
func (m *MCISApi) CreateMcisByParam(req *TbMcisCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.CreateMcis()
	m.SetInType(holdType)

	return result, err
}

// ListMcis - MCIS 목록
func (m *MCISApi) ListMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcis()
}

// ListMcisByParam - MCIS 목록
func (m *MCISApi) ListMcisByParam(nameSpaceID string, option string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "option":"` + option + `"}`
	result, err := m.requestMCIS.ListMcis()
	m.SetInType(holdType)

	return result, err
}

// ControlMcis - MCIS 제어
func (m *MCISApi) ControlMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ControlMcis()
}

// ControlMcisByParam - MCIS 제어
func (m *MCISApi) ControlMcisByParam(nameSpaceID string, mcisID string, action string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "action":"` + action + `"}`
	result, err := m.requestMCIS.ControlMcis()
	m.SetInType(holdType)

	return result, err
}

// GetMcisStatus - MCIS 상태 조회
func (m *MCISApi) GetMcisStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisStatus()
}

// GetMcisStatusByParam - MCIS 상태 조회
func (m *MCISApi) GetMcisStatusByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.GetMcisStatus()
	m.SetInType(holdType)

	return result, err
}

// GetMcisInfo - MCIS 정보 조회
func (m *MCISApi) GetMcisInfo(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisInfo()
}

// GetMcisInfoByParam - MCIS 정보 조회
func (m *MCISApi) GetMcisInfoByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.GetMcisInfo()
	m.SetInType(holdType)

	return result, err
}

// DeleteMcis - MCIS 삭제
func (m *MCISApi) DeleteMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcis()
}

// DeleteMcisByParam - MCIS 삭제
func (m *MCISApi) DeleteMcisByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.DeleteMcis()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllMcis - MCIS 전체 삭제
func (m *MCISApi) DeleteAllMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteAllMcis()
}

// DeleteAllMcisByParam - MCIS 전체 삭제
func (m *MCISApi) DeleteAllMcisByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIS.DeleteAllMcis()
	m.SetInType(holdType)

	return result, err
}

// CreateMcisVM - MCIS VM 생성
func (m *MCISApi) CreateMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisVM()
}

// CreateMcisVMByParam - MCIS VM 생성
func (m *MCISApi) CreateMcisVMByParam(req *TbVmCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.CreateMcisVM()
	m.SetInType(holdType)

	return result, err
}

// ControlMcisVM - MCIS VM 제어
func (m *MCISApi) ControlMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ControlMcisVM()
}

// ControlMcisVMByParam - MCIS VM 제어
func (m *MCISApi) ControlMcisVMByParam(nameSpaceID string, mcisID string, vmID string, action string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "vmId":"` + vmID + `", "action":"` + action + `"}`
	result, err := m.requestMCIS.ControlMcisVM()
	m.SetInType(holdType)

	return result, err
}

// GetMcisVMStatus - MCIS VM 상태 조회
func (m *MCISApi) GetMcisVMStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisVMStatus()
}

// GetMcisVMStatusByParam - MCIS VM 상태 조회
func (m *MCISApi) GetMcisVMStatusByParam(nameSpaceID string, mcisID string, vmID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "vmId":"` + vmID + `"}`
	result, err := m.requestMCIS.GetMcisVMStatus()
	m.SetInType(holdType)

	return result, err
}

// GetMcisVMInfo - MCIS VM 정보 조회
func (m *MCISApi) GetMcisVMInfo(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisVMInfo()
}

// GetMcisVMInfoByParam - MCIS VM 정보 조회
func (m *MCISApi) GetMcisVMInfoByParam(nameSpaceID string, mcisID string, vmID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "vmId":"` + vmID + `"}`
	result, err := m.requestMCIS.GetMcisVMInfo()
	m.SetInType(holdType)

	return result, err
}

// DeleteMcisVM - MCIS VM 삭제
func (m *MCISApi) DeleteMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcisVM()
}

// DeleteMcisVMByParam - MCIS VM 삭제
func (m *MCISApi) DeleteMcisVMByParam(nameSpaceID string, mcisID string, vmID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "vmId":"` + vmID + `"}`
	result, err := m.requestMCIS.DeleteMcisVM()
	m.SetInType(holdType)

	return result, err
}

// RecommandMcis - MCIS 추천
func (m *MCISApi) RecommandMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.RecommandMcis()
}

// RecommandMcisByParam - MCIS 추천
func (m *MCISApi) RecommandMcisByParam(req *McisRecommendCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.RecommandMcis()
	m.SetInType(holdType)

	return result, err
}

// CmdMcis - MCIS 명령 실행
func (m *MCISApi) CmdMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CmdMcis()
}

// CmdMcisByParam - MCIS 명령 실행
func (m *MCISApi) CmdMcisByParam(req *McisCmdCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.CmdMcis()
	m.SetInType(holdType)

	return result, err
}

// CmdMcisVm - MCIS VM 명령 실행
func (m *MCISApi) CmdMcisVm(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CmdMcisVm()
}

// CmdMcisVmByParam - MCIS VM 명령 실행
func (m *MCISApi) CmdMcisVmByParam(req *McisCmdVmCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.CmdMcisVm()
	m.SetInType(holdType)

	return result, err
}

// jmlee
// InstallAgentToMcis -  MCIS Agent 설치
func (m *MCISApi) InstallAgentToMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.InstallAgentToMcis()
}

// InstallAgentToMcisByParam - MCIS Agent 설치
func (m *MCISApi) InstallAgentToMcisByParam(req *McisCmdCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.InstallAgentToMcis()
	m.SetInType(holdType)

	return result, err
}

// InstallMonitorAgentToMcis - MCIS Monitor Agent 설치
func (m *MCISApi) InstallMonitorAgentToMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.InstallMonitorAgentToMcis()
}

// InstallMonitorAgentToMcisByParam - MCIS Monitor Agent 설치
func (m *MCISApi) InstallMonitorAgentToMcisByParam(req *McisCmdCreateRequest) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIS.InData = string(j)
	result, err := m.requestMCIS.InstallMonitorAgentToMcis()
	m.SetInType(holdType)

	return result, err
}

// GetMonitorData - MCIS Monitor 정보 조회
func (m *MCISApi) GetMonitorData(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMonitorData()
}

// GetMonitorDataByParam - MCIS Monitor 정보 조회
func (m *MCISApi) GetMonitorDataByParam(nameSpaceID string, mcisID string, metric string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "metric": "` + metric + `"}`
	result, err := m.requestMCIS.GetMonitorData()
	m.SetInType(holdType)

	return result, err
}

// GetBenchmark - Benchmark 조회
func (m *MCISApi) GetBenchmark(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetBenchmark()
}

// GetBenchmarkByParam - Benchmark 조회
func (m *MCISApi) GetBenchmarkByParam(nameSpaceID string, mcisID string, action string, host string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "action": "` + action + `", "bm": {"host":"` + host + `"} }`
	result, err := m.requestMCIS.GetBenchmark()
	m.SetInType(holdType)

	return result, err
}

// GetAllBenchmark - Benchmark 목록
func (m *MCISApi) GetAllBenchmark(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetAllBenchmark()
}

// GetAllBenchmarkByParam - Benchmark 목록
func (m *MCISApi) GetAllBenchmarkByParam(nameSpaceID string, mcisID string, host string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "bm": {"host":"` + host + `"} }`
	result, err := m.requestMCIS.GetAllBenchmark()
	m.SetInType(holdType)

	return result, err
}

// CheckMcis - MCIS 체크
func (m *MCISApi) CheckMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CheckMcis()
}

// CheckMcisByParam - MCIS 체크
func (m *MCISApi) CheckMcisByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.CheckMcis()
	m.SetInType(holdType)

	return result, err
}

// CheckVm - MCIS VM 체크
func (m *MCISApi) CheckVm(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CheckVm()
}

// CheckVmByParam - MCIS VM 체크
func (m *MCISApi) CheckVmByParam(nameSpaceID string, mcisID string, vmID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "vmId":"` + vmID + `"}`
	result, err := m.requestMCIS.CheckVm()
	m.SetInType(holdType)

	return result, err
}

// ===== [ Private Functiom ] =====

// ===== [ Public Functiom ] =====

// NewMCISManager - MCIS API 객체 생성
func NewMCISManager() (m *MCISApi) {

	m = &MCISApi{}
	m.gConf = &config.GrpcConfig{}
	m.gConf.GSL.TumblebugSrv = &config.GrpcServerConfig{}
	m.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	m.jaegerCloser = nil
	m.conn = nil
	m.clientMCIS = nil
	m.requestMCIS = nil

	m.inType = "json"
	m.outType = "json"

	return
}
