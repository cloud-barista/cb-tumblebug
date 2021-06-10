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
	Name            string    `yaml:"name" json:"name"`
	InstallMonAgent string    `yaml:"installMonAgent" json:"installMonAgent"`
	Label           string    `yaml:"label" json:"label"`
	PlacementAlgo   string    `yaml:"placementAlgo" json:"placementAlgo"`
	Description     string    `yaml:"description" json:"description"`
	Vm              []TbVmReq `yaml:"vm" json:"vm"`
}

// TbVmReq - MCIS VM 생성 요청 구조 정의
type TbVmReq struct {
	Name             string   `yaml:"name" json:"name"`
	VmGroupSize      string   `yaml:"vmGroupSize" json:"vmGroupSize"`
	Label            string   `yaml:"label" json:"label"`
	Description      string   `yaml:"description" json:"description"`
	ConnectionName   string   `yaml:"connectionName" json:"connectionName"`
	SpecId           string   `yaml:"specId" json:"specId"`
	ImageId          string   `yaml:"imageId" json:"imageId"`
	VNetId           string   `yaml:"vNetId" json:"vNetId"`
	SubnetId         string   `yaml:"subnetId" json:"subnetId"`
	SecurityGroupIds []string `yaml:"securityGroupIds" json:"securityGroupIds"`
	SshKeyId         string   `yaml:"sshKeyId" json:"sshKeyId"`
	VmUserAccount    string   `yaml:"vmUserAccount" json:"vmUserAccount"`
	VmUserPassword   string   `yaml:"vmUserPassword" json:"vmUserPassword"`
}

// TbVmCreateRequest - MCIS VM 생성 요청 구조 Wrapper 정의
type TbVmCreateRequest struct {
	NsId   string   `yaml:"nsId" json:"nsId"`
	McisId string   `yaml:"mcisId" json:"mcisId"`
	Item   TbVmInfo `yaml:"mcisvm" json:"mcisvm"`
}

// TbVmGroupCreateRequest - MCIS VM 그룹 생성 요청 구조 Wrapper 정의
type TbVmGroupCreateRequest struct {
	NsId   string  `yaml:"nsId" json:"nsId"`
	McisId string  `yaml:"mcisId" json:"mcisId"`
	Item   TbVmReq `yaml:"groupvm" json:"groupvm"`
}

// TbVmInfo - MCIS VM 구조 정의
type TbVmInfo struct {
	Id               string      `yaml:"id" json:"id"`
	Name             string      `yaml:"name" json:"name"`
	VmGroupId        string      `yaml:"vmGroupId" json:"vmGroupId"`
	Location         GeoLocation `yaml:"location" json:"location"`
	Status           string      `yaml:"status" json:"status"`
	TargetStatus     string      `yaml:"targetStatus" json:"targetStatus"`
	TargetAction     string      `yaml:"targetAction" json:"targetAction"`
	MonAgentStatus   string      `yaml:"monAgentStatus" json:"monAgentStatus"`
	SystemMessage    string      `yaml:"systemMessage" json:"systemMessage"`
	CreatedTime      string      `yaml:"createdTime" json:"createdTime"`
	Label            string      `yaml:"label" json:"label"`
	Description      string      `yaml:"description" json:"description"`
	Region           RegionInfo  `yaml:"region" json:"region"`
	PublicIP         string      `yaml:"publicIP" json:"publicIP"`
	SSHPort          string      `yaml:"sshPort" json:"sshPort"`
	PublicDNS        string      `yaml:"publicDNS" json:"publicDNS"`
	PrivateIP        string      `yaml:"privateIP" json:"privateIP"`
	PrivateDNS       string      `yaml:"privateDNS" json:"privateDNS"`
	VMBootDisk       string      `yaml:"vmBootDisk" json:"vmBootDisk"`
	VMBlockDisk      string      `yaml:"vmBlockDisk" json:"vmBlockDisk"`
	ConnectionName   string      `yaml:"connectionName" json:"connectionName"`
	SpecId           string      `yaml:"specId" json:"specId"`
	ImageId          string      `yaml:"imageId" json:"imageId"`
	VNetId           string      `yaml:"vNetId" json:"vNetId"`
	SubnetId         string      `yaml:"subnetId" json:"subnetId"`
	SecurityGroupIds []string    `yaml:"securityGroupIds" json:"securityGroupIds"`
	SshKeyId         string      `yaml:"sshKeyId" json:"sshKeyId"`
	VmUserAccount    string      `yaml:"vmUserAccount" json:"vmUserAccount"`
	VmUserPassword   string      `yaml:"vmUserPassword" json:"vmUserPassword"`

	// StartTime 필드가 공백일 경우 json 객체 복사할 때 time format parsing 에러 방지
	// CspViewVmDetail  SpiderVMInfo `yaml:"cspViewVmDetail" json:"cspViewVmDetail"`
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
	SSHAccessPoint    string     `yaml:"SSHAccessPoint" json:"SSHAccessPoint"`
	KeyValueList      []KeyValue `yaml:"KeyValueList" json:"KeyValueList"`
}

// McisRecommendCreateRequest - MCIS 추천 요청 구조 Wrapper 정의
type McisRecommendCreateRequest struct {
	NsId string           `yaml:"nsId" json:"nsId"`
	Item McisRecommendReq `yaml:"recommend" json:"recommend"`
}

// McisRecommendReq - MCIS 추천 요청 구조 정의
type McisRecommendReq struct {
	VmReq          []TbVmRecommendReq `yaml:"vmReq" json:"vmReq"`
	PlacementAlgo  string             `yaml:"placementAlgo" json:"placementAlgo"`
	PlacementParam []KeyValue         `yaml:"placementParam" json:"placementParam"`
	MaxResultNum   string             `yaml:"maxResultNum" json:"maxResultNum"`
}

// McisRecommendReq - MCIS VM 추천 요청 구조 정의
type TbVmRecommendReq struct {
	RequestName  string `yaml:"requestName" json:"requestName"`
	MaxResultNum string `yaml:"maxResultNum" json:"maxResultNum"`

	VcpuSize   string `yaml:"vcpuSize" json:"vcpuSize"`
	MemorySize string `yaml:"memorySize" json:"memorySize"`
	DiskSize   string `yaml:"diskSize" json:"diskSize"`

	PlacementAlgo  string     `yaml:"placementAlgo" json:"placementAlgo"`
	PlacementParam []KeyValue `yaml:"placementParam" json:"placementParam"`
}

// McisCmdCreateRequest - MCIS 명령 실행 요청 구조 Wrapper 정의
type McisCmdCreateRequest struct {
	NsId   string     `yaml:"nsId" json:"nsId"`
	McisId string     `yaml:"mcisId" json:"mcisId"`
	Item   McisCmdReq `yaml:"cmd" json:"cmd"`
}

// McisCmdReq - MCIS 명령 실행 요청 구조 정의
type McisCmdReq struct {
	McisId   string `yaml:"mcisId" json:"mcisId"`
	VmId     string `yaml:"vmId" json:"vmId"`
	Ip       string `yaml:"ip" json:"ip"`
	UserName string `yaml:"userName" json:"userName"`
	SshKey   string `yaml:"sshKey" json:"sshKey"`
	Command  string `yaml:"command" json:"command"`
}

// McisCmdVmCreateRequest - MCIS VM 명령 실행 요청 구조 Wrapper 정의
type McisCmdVmCreateRequest struct {
	NsId   string     `yaml:"nsId" json:"nsId"`
	McisId string     `yaml:"mcisId" json:"mcisId"`
	VmId   string     `yaml:"vmId" json:"vmId"`
	Item   McisCmdReq `yaml:"cmd" json:"cmd"`
}

// McisPolicyCreateRequest - MCIS Policy 생성 요청 구조 Wrapper 정의
type McisPolicyCreateRequest struct {
	NsId   string         `yaml:"nsId" json:"nsId"`
	McisId string         `yaml:"mcisId" json:"mcisId"`
	Item   McisPolicyInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// AutoCondition - MCIS AutoCondition 요청 구조 정의
type AutoCondition struct {
	Metric           string   `yaml:"metric" json:"metric"`
	Operator         string   `yaml:"operator" json:"operator"`
	Operand          string   `yaml:"operand" json:"operand"`
	EvaluationPeriod string   `yaml:"evaluationPeriod" json:"evaluationPeriod"`
	EvaluationValue  []string `yaml:"evaluationValue" json:"evaluationValue"`
}

// AutoAction - MCIS AutoAction 요청 구조 정의
type AutoAction struct {
	ActionType    string     `yaml:"actionType" json:"actionType"`
	Vm            TbVmInfo   `yaml:"vm" json:"vm"`
	PostCommand   McisCmdReq `yaml:"postCommand" json:"postCommand"`
	PlacementAlgo string     `yaml:"placementAlgo" json:"placementAlgo"`
}

// Policy - MCIS Policy 요청 구조 정의
type Policy struct {
	AutoCondition AutoCondition `yaml:"autoCondition" json:"autoCondition"`
	AutoAction    AutoAction    `yaml:"autoAction" json:"autoAction"`
	Status        string        `yaml:"status" json:"status"`
}

// McisPolicyInfo - MCIS Policy 정보 구조 정의
type McisPolicyInfo struct {
	Name   string   `yaml:"Name" json:"Name"`
	Id     string   `yaml:"Id" json:"Id"`
	Policy []Policy `yaml:"policy" json:"policy"`

	ActionLog   string `yaml:"actionLog" json:"actionLog"`
	Description string `yaml:"description" json:"description"`
}

// McisRecommendVmCreateRequest - MCIS VM 추천 요청 구조 Wrapper 정의
type McisRecommendVmCreateRequest struct {
	NsId string         `yaml:"nsId" json:"nsId"`
	Item DeploymentPlan `yaml:"plan" json:"plan"`
}

// DeploymentPlan - DeploymentPlan 요청 구조 정의
type DeploymentPlan struct {
	Filter   FilterInfo   `yaml:"filter" json:"filter"`
	Priority PriorityInfo `yaml:"priority" json:"priority"`
	Limit    string       `yaml:"limit" json:"limit"`
}

// FilterInfo - FilterInfo 요청 구조 정의
type FilterInfo struct {
	Policy []FilterCondition `yaml:"policy" json:"policy"`
}

// FilterCondition - FilterCondition 요청 구조 정의
type FilterCondition struct {
	Metric    string      `yaml:"metric" json:"metric"`
	Condition []Operation `yaml:"condition" json:"condition"`
}

// Operation - Operation 요청 구조 정의
type Operation struct {
	Operator string `yaml:"operator" json:"operator"`
	Operand  string `yaml:"operand" json:"operand"`
}

// PriorityInfo - PriorityInfo 요청 구조 정의
type PriorityInfo struct {
	Policy []PriorityCondition `yaml:"policy" json:"policy"`
}

// PriorityCondition - PriorityCondition 요청 구조 정의
type PriorityCondition struct {
	Metric    string            `yaml:"metric" json:"metric"`
	Weight    string            `yaml:"weight" json:"weight"`
	Parameter []ParameterKeyVal `yaml:"parameter" json:"parameter"`
}

// ParameterKeyVal - ParameterKeyVal 요청 구조 정의
type ParameterKeyVal struct {
	Key string   `yaml:"key" json:"key"`
	Val []string `yaml:"val" json:"val"`
}

// ===== [ Implementatiom ] =====

// SetServerAddr - Tumblebug 서버 주소 설정
func (m *MCISApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	m.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr - Tumblebug 서버 주소 값 조회
func (m *MCISApi) GetServerAddr() (string, error) {
	return m.gConf.GSL.TumblebugCli.ServerAddr, nil
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

	// TUMBLEBUG CLIENT 필수 입력 항목 체크
	tumblebugcli := gConf.GSL.TumblebugCli

	if tumblebugcli == nil {
		return errors.New("tumblebugcli field are not specified")
	}

	if tumblebugcli.ServerAddr == "" {
		return errors.New("tumblebugcli.server_addr field are not specified")
	}

	if tumblebugcli.Timeout == 0 {
		tumblebugcli.Timeout = 90 * time.Second
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

	tumblebugcli := m.gConf.GSL.TumblebugCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(tumblebugcli)
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
func (m *MCISApi) ListMcisByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
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

// ListMcisStatus - MCIS 상태 목록
func (m *MCISApi) ListMcisStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisStatus()
}

// ListMcisStatusByParam - MCIS 상태 목록
func (m *MCISApi) ListMcisStatusByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIS.ListMcisStatus()
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

// CreateMcisVMGroup - MCIS VM 그룹 생성
func (m *MCISApi) CreateMcisVMGroup(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisVMGroup()
}

// CreateMcisVMGroupByParam - MCIS VM 생성
func (m *MCISApi) CreateMcisVMGroupByParam(req *TbVmGroupCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.CreateMcisVMGroup()
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

// RecommendMcis - MCIS 추천
func (m *MCISApi) RecommendMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.RecommendMcis()
}

// RecommendMcisByParam - MCIS 추천
func (m *MCISApi) RecommendMcisByParam(req *McisRecommendCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.RecommendMcis()
	m.SetInType(holdType)

	return result, err
}

// RecommendVM - MCIS VM 추천
func (m *MCISApi) RecommendVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.RecommendVM()
}

// RecommendVMByParam - MCIS VM 추천
func (m *MCISApi) RecommendVMByParam(req *McisRecommendVmCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.RecommendVM()
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

// CreateMcisPolicy - Policy 생성
func (m *MCISApi) CreateMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisPolicy()
}

// CreateMcisPolicyByParam - Policy 생성
func (m *MCISApi) CreateMcisPolicyByParam(req *McisPolicyCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.CreateMcisPolicy()
	m.SetInType(holdType)

	return result, err
}

// ListMcisPolicy - Policy 목록
func (m *MCISApi) ListMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisPolicy()
}

// ListMcisPolicyByParam - Policy 목록
func (m *MCISApi) ListMcisPolicyByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIS.ListMcisPolicy()
	m.SetInType(holdType)

	return result, err
}

// GetMcisPolicy - Policy 조회
func (m *MCISApi) GetMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisPolicy()
}

// GetMcisPolicyByParam - Policy 조회
func (m *MCISApi) GetMcisPolicyByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.GetMcisPolicy()
	m.SetInType(holdType)

	return result, err
}

// DeleteMcisPolicy - Policy 삭제
func (m *MCISApi) DeleteMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcisPolicy()
}

// DeleteMcisPolicyByParam - Policy 삭제
func (m *MCISApi) DeleteMcisPolicyByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.DeleteMcisPolicy()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllMcisPolicy - Policy 전체 삭제
func (m *MCISApi) DeleteAllMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteAllMcisPolicy()
}

// DeleteAllMcisPolicyByParam - Policy 전체 삭제
func (m *MCISApi) DeleteAllMcisPolicyByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIS.DeleteAllMcisPolicy()
	m.SetInType(holdType)

	return result, err
}

// ===== [ Private Functiom ] =====

// ===== [ Public Functiom ] =====

// NewMCISManager - MCIS API 객체 생성
func NewMCISManager() (m *MCISApi) {

	m = &MCISApi{}
	m.gConf = &config.GrpcConfig{}
	m.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	m.jaegerCloser = nil
	m.conn = nil
	m.clientMCIS = nil
	m.requestMCIS = nil

	m.inType = "json"
	m.outType = "json"

	return
}
