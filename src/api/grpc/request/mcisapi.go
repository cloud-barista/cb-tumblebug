package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/config"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/request/mcis"

	common "github.com/cloud-barista/cb-tumblebug/src/core/common"
	core_mcis "github.com/cloud-barista/cb-tumblebug/src/core/mcis"

	"google.golang.org/grpc"
)

// ===== [ Comtants and Variables ] =====

// ===== [ Types ] =====

// MCISApi is for MCIS API 구조 정의
type MCISApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientMCIS   pb.MCISClient
	requestMCIS  *mcis.MCISRequest
	inType       string
	outType      string
}

// TbMcisCreateRequest is for MCIS 생성 요청 구조 Wrapper 정의
type TbMcisCreateRequest struct {
	NsId string              `yaml:"nsId" json:"nsId"`
	Item core_mcis.TbMcisReq `yaml:"mcis" json:"mcis"`
}

// TbMcisReq is for MCIS 생성 요청 구조 정의
// type TbMcisReq struct {
// 	Name            string    `yaml:"name" json:"name"`
// 	InstallMonAgent string    `yaml:"installMonAgent" json:"installMonAgent"`
// 	Label           string    `yaml:"label" json:"label"`
// 	PlacementAlgo   string    `yaml:"placementAlgo" json:"placementAlgo"`
// 	Description     string    `yaml:"description" json:"description"`
// 	Vm              []TbVmReq `yaml:"vm" json:"vm"`
// }

// TbVmReq is for MCIS VM 생성 요청 구조 정의
// type TbVmReq struct {
// 	Name             string   `yaml:"name" json:"name"`
// 	SubGroupSize      string   `yaml:"subGroupSize" json:"subGroupSize"`
// 	Label            string   `yaml:"label" json:"label"`
// 	Description      string   `yaml:"description" json:"description"`
// 	ConnectionName   string   `yaml:"connectionName" json:"connectionName"`
// 	SpecId           string   `yaml:"specId" json:"specId"`
// 	ImageId          string   `yaml:"imageId" json:"imageId"`
// 	VNetId           string   `yaml:"vNetId" json:"vNetId"`
// 	SubnetId         string   `yaml:"subnetId" json:"subnetId"`
// 	SecurityGroupIds []string `yaml:"securityGroupIds" json:"securityGroupIds"`
// 	SshKeyId         string   `yaml:"sshKeyId" json:"sshKeyId"`
// 	VmUserAccount    string   `yaml:"vmUserAccount" json:"vmUserAccount"`
// 	VmUserPassword   string   `yaml:"vmUserPassword" json:"vmUserPassword"`
// }

// TbVmCreateRequest is for MCIS VM 생성 요청 구조 Wrapper 정의
type TbVmCreateRequest struct {
	NsId   string   `yaml:"nsId" json:"nsId"`
	McisId string   `yaml:"mcisId" json:"mcisId"`
	Item   TbVmInfo `yaml:"mcisvm" json:"mcisvm"`
}

// TbSubGroupCreateRequest is for MCIS VM 그룹 생성 요청 구조 Wrapper 정의
type TbSubGroupCreateRequest struct {
	NsId   string            `yaml:"nsId" json:"nsId"`
	McisId string            `yaml:"mcisId" json:"mcisId"`
	Item   core_mcis.TbVmReq `yaml:"groupvm" json:"groupvm"`
}

// TbVmInfo is for MCIS VM 구조 정의
type TbVmInfo struct {
	Id               string               `yaml:"id" json:"id"`
	Name             string               `yaml:"name" json:"name"`
	SubGroupId       string               `yaml:"subGroupId" json:"subGroupId"`
	Location         common.GeoLocation   `yaml:"location" json:"location"`
	Status           string               `yaml:"status" json:"status"`
	TargetStatus     string               `yaml:"targetStatus" json:"targetStatus"`
	TargetAction     string               `yaml:"targetAction" json:"targetAction"`
	MonAgentStatus   string               `yaml:"monAgentStatus" json:"monAgentStatus"`
	SystemMessage    string               `yaml:"systemMessage" json:"systemMessage"`
	CreatedTime      string               `yaml:"createdTime" json:"createdTime"`
	Label            string               `yaml:"label" json:"label"`
	Description      string               `yaml:"description" json:"description"`
	Region           core_mcis.RegionInfo `yaml:"region" json:"region"`
	PublicIP         string               `yaml:"publicIP" json:"publicIP"`
	SSHPort          string               `yaml:"sshPort" json:"sshPort"`
	PublicDNS        string               `yaml:"publicDNS" json:"publicDNS"`
	PrivateIP        string               `yaml:"privateIP" json:"privateIP"`
	PrivateDNS       string               `yaml:"privateDNS" json:"privateDNS"`
	ConnectionName   string               `yaml:"connectionName" json:"connectionName"`
	SpecId           string               `yaml:"specId" json:"specId"`
	ImageId          string               `yaml:"imageId" json:"imageId"`
	VNetId           string               `yaml:"vNetId" json:"vNetId"`
	SubnetId         string               `yaml:"subnetId" json:"subnetId"`
	SecurityGroupIds []string             `yaml:"securityGroupIds" json:"securityGroupIds"`
	SshKeyId         string               `yaml:"sshKeyId" json:"sshKeyId"`
	VmUserAccount    string               `yaml:"vmUserAccount" json:"vmUserAccount"`
	VmUserPassword   string               `yaml:"vmUserPassword" json:"vmUserPassword"`

	// StartTime 필드가 공백일 경우 json 객체 복사할 때 time format parsing 에러 방지
	// CspViewVmDetail  SpiderVMInfo `yaml:"cspViewVmDetail" json:"cspViewVmDetail"`
}

// GeoLocation is for 위치 정보 구조 정의
// type GeoLocation struct {
// 	Latitude     string `yaml:"latitude" json:"latitude"`
// 	Longitude    string `yaml:"longitude" json:"longitude"`
// 	BriefAddr    string `yaml:"briefAddr" json:"briefAddr"`
// 	CloudType    string `yaml:"cloudType" json:"cloudType"`
// 	NativeRegion string `yaml:"nativeRegion" json:"nativeRegion"`
// }

// RegionInfo is for Region 정보 구조 정의
// type RegionInfo struct {
// 	Region string `yaml:"Region" json:"Region"`
// 	Zone   string `yaml:"Zone" json:"Zone"`
// }

// SpiderVMInfo is for VM 정보 구조 정의
// type SpiderVMInfo struct {
// 	// Fields for request
// 	Name               string   `yaml:"Name" json:"Name"`
// 	ImageName          string   `yaml:"ImageName" json:"ImageName"`
// 	VPCName            string   `yaml:"VPCName" json:"VPCName"`
// 	SubnetName         string   `yaml:"SubnetName" json:"SubnetName"`
// 	SecurityGroupNames []string `yaml:"SecurityGroupNames" json:"SecurityGroupNames"`
// 	KeyPairName        string   `yaml:"KeyPairName" json:"KeyPairName"`

// 	// Fields for both request and response
// 	VMSpecName   string `yaml:"VMSpecName" json:"VMSpecName"`
// 	VMUserId     string `yaml:"VMUserId" json:"VMUserId"`
// 	VMUserPasswd string `yaml:"VMUserPasswd" json:"VMUserPasswd"`

// 	// Fields for response
// 	IId               IID        `yaml:"IId" json:"IId"`
// 	ImageIId          IID        `yaml:"ImageIId" json:"ImageIId"`
// 	VpcIID            IID        `yaml:"VpcIID" json:"VpcIID"`
// 	SubnetIID         IID        `yaml:"SubnetIID" json:"SubnetIID"`
// 	SecurityGroupIIds []IID      `yaml:"SecurityGroupIIds" json:"SecurityGroupIIds"`
// 	KeyPairIId        IID        `yaml:"KeyPairIId" json:"KeyPairIId"`
// 	StartTime         string     `yaml:"StartTime" json:"StartTime"`
// 	Region            RegionInfo `yaml:"Region" json:"Region"`
// 	NetworkInterface  string     `yaml:"NetworkInterface" json:"NetworkInterface"`
// 	PublicIP          string     `yaml:"PublicIP" json:"PublicIP"`
// 	PublicDNS         string     `yaml:"PublicDNS" json:"PublicDNS"`
// 	PrivateIP         string     `yaml:"PrivateIP" json:"PrivateIP"`
// 	PrivateDNS        string     `yaml:"PrivateDNS" json:"PrivateDNS"`
// 	SSHAccessPoint    string     `yaml:"SSHAccessPoint" json:"SSHAccessPoint"`
// 	KeyValueList      []KeyValue `yaml:"KeyValueList" json:"KeyValueList"`
// }

// McisRecommendCreateRequest is for MCIS 추천 요청 구조 Wrapper 정의
type McisRecommendCreateRequest struct {
	NsId string                     `yaml:"nsId" json:"nsId"`
	Item core_mcis.McisRecommendReq `yaml:"recommend" json:"recommend"`
}

// McisRecommendReq is for MCIS 추천 요청 구조 정의
// type McisRecommendReq struct {
// 	VmReq          []TbVmRecommendReq `yaml:"vmReq" json:"vmReq"`
// 	PlacementAlgo  string             `yaml:"placementAlgo" json:"placementAlgo"`
// 	PlacementParam []KeyValue         `yaml:"placementParam" json:"placementParam"`
// 	MaxResultNum   string             `yaml:"maxResultNum" json:"maxResultNum"`
// }

// McisRecommendReq is for MCIS VM 추천 요청 구조 정의
// type TbVmRecommendReq struct {
// 	RequestName  string `yaml:"requestName" json:"requestName"`
// 	MaxResultNum string `yaml:"maxResultNum" json:"maxResultNum"`

// 	VcpuSize   string `yaml:"vcpuSize" json:"vcpuSize"`
// 	MemorySize string `yaml:"memorySize" json:"memorySize"`
// 	DiskSize   string `yaml:"diskSize" json:"diskSize"`

// 	PlacementAlgo  string     `yaml:"placementAlgo" json:"placementAlgo"`
// 	PlacementParam []KeyValue `yaml:"placementParam" json:"placementParam"`
// }

// McisCmdCreateRequest is for MCIS 명령 실행 요청 구조 Wrapper 정의
type McisCmdCreateRequest struct {
	NsId   string               `yaml:"nsId" json:"nsId"`
	McisId string               `yaml:"mcisId" json:"mcisId"`
	Item   core_mcis.McisCmdReq `yaml:"cmd" json:"cmd"`
}

// McisCmdReq is for MCIS 명령 실행 요청 구조 정의
// type McisCmdReq struct {
// 	McisId   string `yaml:"mcisId" json:"mcisId"`
// 	VmId     string `yaml:"vmId" json:"vmId"`
// 	Ip       string `yaml:"ip" json:"ip"`
// 	UserName string `yaml:"userName" json:"userName"`
// 	SshKey   string `yaml:"sshKey" json:"sshKey"`
// 	Command  string `yaml:"command" json:"command"`
// }

// McisCmdVmCreateRequest is for MCIS VM 명령 실행 요청 구조 Wrapper 정의
type McisCmdVmCreateRequest struct {
	NsId   string               `yaml:"nsId" json:"nsId"`
	McisId string               `yaml:"mcisId" json:"mcisId"`
	VmId   string               `yaml:"vmId" json:"vmId"`
	Item   core_mcis.McisCmdReq `yaml:"cmd" json:"cmd"`
}

// McisPolicyCreateRequest is for MCIS Policy 생성 요청 구조 Wrapper 정의
type McisPolicyCreateRequest struct {
	NsId   string                   `yaml:"nsId" json:"nsId"`
	McisId string                   `yaml:"mcisId" json:"mcisId"`
	Item   core_mcis.McisPolicyInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// AutoCondition is for MCIS AutoCondition 요청 구조 정의
// type AutoCondition struct {
// 	Metric           string   `yaml:"metric" json:"metric"`
// 	Operator         string   `yaml:"operator" json:"operator"`
// 	Operand          string   `yaml:"operand" json:"operand"`
// 	EvaluationPeriod string   `yaml:"evaluationPeriod" json:"evaluationPeriod"`
// 	EvaluationValue  []string `yaml:"evaluationValue" json:"evaluationValue"`
// }

// AutoAction is for MCIS AutoAction 요청 구조 정의
// type AutoAction struct {
// 	ActionType    string     `yaml:"actionType" json:"actionType"`
// 	Vm            TbVmInfo   `yaml:"vm" json:"vm"`
// 	PostCommand   McisCmdReq `yaml:"postCommand" json:"postCommand"`
// 	PlacementAlgo string     `yaml:"placementAlgo" json:"placementAlgo"`
// }

// Policy is for MCIS Policy 요청 구조 정의
// type Policy struct {
// 	AutoCondition AutoCondition `yaml:"autoCondition" json:"autoCondition"`
// 	AutoAction    AutoAction    `yaml:"autoAction" json:"autoAction"`
// 	Status        string        `yaml:"status" json:"status"`
// }

// McisPolicyInfo is for MCIS Policy 정보 구조 정의
// type McisPolicyInfo struct {
// 	Name   string   `yaml:"Name" json:"Name"`
// 	Id     string   `yaml:"Id" json:"Id"`
// 	Policy []Policy `yaml:"policy" json:"policy"`

// 	ActionLog   string `yaml:"actionLog" json:"actionLog"`
// 	Description string `yaml:"description" json:"description"`
// }

// McisRecommendVmCreateRequest is for MCIS VM 추천 요청 구조 Wrapper 정의
type McisRecommendVmCreateRequest struct {
	NsId string                   `yaml:"nsId" json:"nsId"`
	Item core_mcis.DeploymentPlan `yaml:"plan" json:"plan"`
}

// DeploymentPlan is for DeploymentPlan 요청 구조 정의
// type DeploymentPlan struct {
// 	Filter   FilterInfo   `yaml:"filter" json:"filter"`
// 	Priority PriorityInfo `yaml:"priority" json:"priority"`
// 	Limit    string       `yaml:"limit" json:"limit"`
// }

// FilterInfo is for FilterInfo 요청 구조 정의
// type FilterInfo struct {
// 	Policy []FilterCondition `yaml:"policy" json:"policy"`
// }

// FilterCondition is for FilterCondition 요청 구조 정의
// type FilterCondition struct {
// 	Metric    string      `yaml:"metric" json:"metric"`
// 	Condition []Operation `yaml:"condition" json:"condition"`
// }

// Operation is for Operation 요청 구조 정의
// type Operation struct {
// 	Operator string `yaml:"operator" json:"operator"`
// 	Operand  string `yaml:"operand" json:"operand"`
// }

// PriorityInfo is for PriorityInfo 요청 구조 정의
// type PriorityInfo struct {
// 	Policy []PriorityCondition `yaml:"policy" json:"policy"`
// }

// PriorityCondition is for PriorityCondition 요청 구조 정의
// type PriorityCondition struct {
// 	Metric    string            `yaml:"metric" json:"metric"`
// 	Weight    string            `yaml:"weight" json:"weight"`
// 	Parameter []ParameterKeyVal `yaml:"parameter" json:"parameter"`
// }

// ParameterKeyVal is for ParameterKeyVal 요청 구조 정의
// type ParameterKeyVal struct {
// 	Key string   `yaml:"key" json:"key"`
// 	Val []string `yaml:"val" json:"val"`
// }

// ===== [ Implementatiom ] =====

// SetServerAddr is to Tumblebug 서버 주소 설정
func (m *MCISApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	m.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr is to Tumblebug 서버 주소 값 조회
func (m *MCISApi) GetServerAddr() (string, error) {
	return m.gConf.GSL.TumblebugCli.ServerAddr, nil
}

// SetTLSCA is to TLS CA 설정
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

// GetTLSCA is to TLS CA 값 조회
func (m *MCISApi) GetTLSCA() (string, error) {
	if m.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout is to Timeout 설정
func (m *MCISApi) SetTimeout(timeout time.Duration) error {
	m.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout is to Timeout 값 조회
func (m *MCISApi) GetTimeout() (time.Duration, error) {
	return m.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken is to JWT 인증 토큰 설정
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

// GetJWTToken is to JWT 인증 토큰 값 조회
func (m *MCISApi) GetJWTToken() (string, error) {
	if m.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath is to 환경설정 파일 설정
func (m *MCISApi) SetConfigPath(configFile string) error {
	logger := logger.NewLogger()

	// Make new config parser that uses Viper library
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

// Open is to 연결 설정
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

// Close is to 연결 종료
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

// SetInType is to 입력 문서 타입 설정 (json/yaml)
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

// GetInType is to 입력 문서 타입 값 조회
func (m *MCISApi) GetInType() (string, error) {
	return m.inType, nil
}

// SetOutType is to 출력 문서 타입 설정 (json/yaml)
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

// GetOutType is to 출력 문서 타입 값 조회
func (m *MCISApi) GetOutType() (string, error) {
	return m.outType, nil
}

// CreateMcis is to MCIS 생성
func (m *MCISApi) CreateMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcis()
}

// CreateMcisByParam is to MCIS 생성
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

// ListMcis is to MCIS 목록
func (m *MCISApi) ListMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcis()
}

// ListMcisByParam is to MCIS 목록
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

// ListMcisId is to MCIS 목록
func (m *MCISApi) ListMcisId(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisId()
}

// ListMcisIdByParam is to MCIS 목록
func (m *MCISApi) ListMcisIdByParam(nameSpaceID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIS.ListMcisId()
	m.SetInType(holdType)

	return result, err
}

// ControlMcis is to MCIS 제어
func (m *MCISApi) ControlMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ControlMcis()
}

// ControlMcisByParam is to MCIS 제어
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

// ListMcisStatus is to MCIS 상태 목록
func (m *MCISApi) ListMcisStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisStatus()
}

// ListMcisStatusByParam is to MCIS 상태 목록
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

// GetMcisStatus is to MCIS 상태 조회
func (m *MCISApi) GetMcisStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisStatus()
}

// GetMcisStatusByParam is to MCIS 상태 조회
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

// GetMcisInfo is to MCIS 정보 조회
func (m *MCISApi) GetMcisInfo(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisInfo()
}

// GetMcisInfoByParam is to MCIS 정보 조회
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

// ListMcisVmId is to MCIS 정보 조회
func (m *MCISApi) ListMcisVmId(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisVmId()
}

// ListMcisVmIdByParam is to MCIS 정보 조회
func (m *MCISApi) ListMcisVmIdByParam(nameSpaceID string, mcisID string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `"}`
	result, err := m.requestMCIS.ListMcisVmId()
	m.SetInType(holdType)

	return result, err
}

// DeleteMcis is to MCIS 삭제
func (m *MCISApi) DeleteMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcis()
}

// DeleteMcisByParam is to MCIS 삭제
func (m *MCISApi) DeleteMcisByParam(nameSpaceID string, mcisID string, option string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	// m.requestMCIS.InData = `{"nsId":"` + nameSpaceID + `", "mcisId":"` + mcisID + `", "option":"` + option + `"}` // Style 1
	m.requestMCIS.InData = fmt.Sprintf("{\"nsId\":\"%s\", \"mcisId\":\"%s\", \"option\":\"%s\"}", nameSpaceID, mcisID, option) // Style 2
	result, err := m.requestMCIS.DeleteMcis()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllMcis is to MCIS 전체 삭제
func (m *MCISApi) DeleteAllMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteAllMcis()
}

// DeleteAllMcisByParam is to MCIS 전체 삭제
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

// CreateMcisVM is to MCIS VM 생성
func (m *MCISApi) CreateMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisVM()
}

// CreateMcisVMByParam is to MCIS VM 생성
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

// CreateMcisSubGroup is to MCIS VM 그룹 생성
func (m *MCISApi) CreateMcisSubGroup(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisSubGroup()
}

// CreateMcisSubGroupByParam is to MCIS VM 생성
func (m *MCISApi) CreateMcisSubGroupByParam(req *TbSubGroupCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.CreateMcisSubGroup()
	m.SetInType(holdType)

	return result, err
}

// ControlMcisVM is to MCIS VM 제어
func (m *MCISApi) ControlMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ControlMcisVM()
}

// ControlMcisVMByParam is to MCIS VM 제어
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

// GetMcisVMStatus is to MCIS VM 상태 조회
func (m *MCISApi) GetMcisVMStatus(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisVMStatus()
}

// GetMcisVMStatusByParam is to MCIS VM 상태 조회
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

// GetMcisVMInfo is to MCIS VM 정보 조회
func (m *MCISApi) GetMcisVMInfo(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisVMInfo()
}

// GetMcisVMInfoByParam is to MCIS VM 정보 조회
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

// DeleteMcisVM is to MCIS VM 삭제
func (m *MCISApi) DeleteMcisVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcisVM()
}

// DeleteMcisVMByParam is to MCIS VM 삭제
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

// RecommendMcis is to MCIS 추천
func (m *MCISApi) RecommendMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.RecommendMcis()
}

// RecommendMcisByParam is to MCIS 추천
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

// RecommendVM is to MCIS VM 추천
func (m *MCISApi) RecommendVM(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.RecommendVM()
}

// RecommendVMByParam is to MCIS VM 추천
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

// CmdMcis is to MCIS 명령 실행
func (m *MCISApi) CmdMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CmdMcis()
}

// CmdMcisByParam is to MCIS 명령 실행
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

// CmdMcisVm is to MCIS VM 명령 실행
func (m *MCISApi) CmdMcisVm(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CmdMcisVm()
}

// CmdMcisVmByParam is to MCIS VM 명령 실행
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

// InstallBenchmarkAgentToMcis is to  MCIS Agent 설치
func (m *MCISApi) InstallBenchmarkAgentToMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.InstallBenchmarkAgentToMcis()
}

// InstallBenchmarkAgentToMcisByParam is to MCIS Agent 설치
func (m *MCISApi) InstallBenchmarkAgentToMcisByParam(req *McisCmdCreateRequest) (string, error) {
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
	result, err := m.requestMCIS.InstallBenchmarkAgentToMcis()
	m.SetInType(holdType)

	return result, err
}

// InstallMonitorAgentToMcis is to MCIS Monitor Agent 설치
func (m *MCISApi) InstallMonitorAgentToMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.InstallMonitorAgentToMcis()
}

// InstallMonitorAgentToMcisByParam is to MCIS Monitor Agent 설치
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

// GetMonitorData is to MCIS Monitor 정보 조회
func (m *MCISApi) GetMonitorData(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMonitorData()
}

// GetMonitorDataByParam is to MCIS Monitor 정보 조회
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

// GetBenchmark is to Benchmark 조회
func (m *MCISApi) GetBenchmark(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetBenchmark()
}

// GetBenchmarkByParam is to Benchmark 조회
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

// GetAllBenchmark is to Benchmark 목록
func (m *MCISApi) GetAllBenchmark(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetAllBenchmark()
}

// GetAllBenchmarkByParam is to Benchmark 목록
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

// CheckMcis is to MCIS 체크
func (m *MCISApi) CheckMcis(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CheckMcis()
}

// CheckMcisByParam is to MCIS 체크
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

// CheckVm is to MCIS VM 체크
func (m *MCISApi) CheckVm(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CheckVm()
}

// CheckVmByParam is to MCIS VM 체크
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

// CreateMcisPolicy is to Policy 생성
func (m *MCISApi) CreateMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.CreateMcisPolicy()
}

// CreateMcisPolicyByParam is to Policy 생성
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

// ListMcisPolicy is to Policy 목록
func (m *MCISApi) ListMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.ListMcisPolicy()
}

// ListMcisPolicyByParam is to Policy 목록
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

// GetMcisPolicy is to Policy 조회
func (m *MCISApi) GetMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.GetMcisPolicy()
}

// GetMcisPolicyByParam is to Policy 조회
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

// DeleteMcisPolicy is to Policy 삭제
func (m *MCISApi) DeleteMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteMcisPolicy()
}

// DeleteMcisPolicyByParam is to Policy 삭제
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

// DeleteAllMcisPolicy is to Policy 전체 삭제
func (m *MCISApi) DeleteAllMcisPolicy(doc string) (string, error) {
	if m.requestMCIS == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIS.InData = doc
	return m.requestMCIS.DeleteAllMcisPolicy()
}

// DeleteAllMcisPolicyByParam is to Policy 전체 삭제
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

// NewMCISManager is to MCIS API 객체 생성
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
