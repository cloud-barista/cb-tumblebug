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
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/request/mcir"

	"google.golang.org/grpc"
)

// ===== [ Comtants and Variables ] =====

// ===== [ Types ] =====

// MCIRApi - MCIR API 구조 정의
type MCIRApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientMCIR   pb.MCIRClient
	requestMCIR  *mcir.MCIRRequest
	inType       string
	outType      string
}

// KeyValue - Key / Value 구조 정의
type KeyValue struct {
	Key   string `yaml:"Key" json:"Key"`
	Value string `yaml:"Value" json:"Value"`
}

// IID - IID 구조 정의
type IID struct {
	NameId   string `yaml:"NameId" json:"NameId"`
	SystemId string `yaml:"SystemId" json:"SystemId"`
}

// TbImageCreateRequest - Image ID를 이용한 생성 요청 구조 Wrapper 정의
type TbImageCreateRequest struct {
	NsId string     `yaml:"nsId" json:"nsId"`
	Item TbImageReq `yaml:"image" json:"image"`
}

// TbImageReq - Image ID를 이용한 생성 요청 구조 정의
type TbImageReq struct {
	Name           string `yaml:"name" json:"name"`
	ConnectionName string `yaml:"connectionName" json:"connectionName"`
	CspImageId     string `yaml:"cspImageId" json:"cspImageId"`
	Description    string `yaml:"description" json:"description"`
}

// TbImageInfoRequest - Image 정보를 이용한 생성 요청 구조 Wrapper 정의
type TbImageInfoRequest struct {
	NsId string      `yaml:"nsId" json:"nsId"`
	Item TbImageInfo `yaml:"image" json:"image"`
}

// TbImageInfo - Image 정보를 이용한 생성 요청 구조 정의
type TbImageInfo struct {
	Id             string     `yaml:"id" json:"id"`
	Name           string     `yaml:"name" json:"name"`
	ConnectionName string     `yaml:"connectionName" json:"connectionName"`
	CspImageId     string     `yaml:"cspImageId" json:"cspImageId"`
	CspImageName   string     `yaml:"cspImageName" json:"cspImageName"`
	Description    string     `yaml:"description" json:"description"`
	CreationDate   string     `yaml:"creationDate" json:"creationDate"`
	GuestOS        string     `yaml:"guestOS" json:"guestOS"`
	Status         string     `yaml:"status" json:"status"`
	KeyValueList   []KeyValue `yaml:"keyValueList" json:"keyValueList"`
}

// TbSecurityGroupCreateRequest - Security Group 생성 요청 구조 Wrapper 정의
type TbSecurityGroupCreateRequest struct {
	NsId string             `yaml:"nsId" json:"nsId"`
	Item TbSecurityGroupReq `yaml:"securityGroup" json:"securityGroup"`
}

// TbSecurityGroupReq - Security Group 생성 요청 구조 정의
type TbSecurityGroupReq struct { // Tumblebug
	Name           string                    `yaml:"name" json:"name"`
	ConnectionName string                    `yaml:"connectionName" json:"connectionName"`
	VNetId         string                    `yaml:"vNetId" json:"vNetId"`
	Description    string                    `yaml:"description" json:"description"`
	FirewallRules  *[]SpiderSecurityRuleInfo `yaml:"firewallRules" json:"firewallRules"`
}

// SpiderSecurityRuleInfo - Security Rule 구조 정의
type SpiderSecurityRuleInfo struct { // Spider
	FromPort   string `yaml:"fromPort" json:"fromPort"`
	ToPort     string `yaml:"toPort" json:"toPort"`
	IPProtocol string `yaml:"ipProtocol" json:"ipProtocol"`
	Direction  string `yaml:"direction" json:"direction"`
}

// TbSpecCreateRequest - Spec 이름을 이용한 생성 요청 구조 Wrapper 정의
type TbSpecCreateRequest struct {
	NsId string    `yaml:"nsId" json:"nsId"`
	Item TbSpecReq `yaml:"spec" json:"spec"`
}

// TbSpecReq - Spec 이름을 이용한 생성 요청 구조 정의
type TbSpecReq struct { // Tumblebug
	Name           string `yaml:"name" json:"name"`
	ConnectionName string `yaml:"connectionName" json:"connectionName"`
	CspSpecName    string `yaml:"cspSpecName" json:"cspSpecName"`
	Description    string `yaml:"description" json:"description"`
}

// TbSpecCreateRequest - Spec 정보를 이용한 생성 요청 구조 Wrapper 정의
type TbSpecInfoRequest struct {
	NsId string     `yaml:"nsId" json:"nsId"`
	Item TbSpecInfo `yaml:"spec" json:"spec"`
}

// TbSpecInfo - Spec 정보를 이용한 생성 요청 구조 정의
type TbSpecInfo struct { // Tumblebug
	Id                    string  `yaml:"id" json:"id"`
	Name                  string  `yaml:"name" json:"name"`
	ConnectionName        string  `yaml:"connectionName" json:"connectionName"`
	CspSpecName           string  `yaml:"cspSpecName" json:"cspSpecName"`
	Os_type               string  `yaml:"os_type" json:"os_type"`
	Num_vCPU              uint16  `yaml:"num_vCPU" json:"num_vCPU"`
	Num_core              uint16  `yaml:"num_core" json:"num_core"`
	Mem_GiB               uint16  `yaml:"mem_GiB" json:"mem_GiB"`
	Storage_GiB           uint32  `yaml:"storage_GiB" json:"storage_GiB"`
	Description           string  `yaml:"description" json:"description"`
	Cost_per_hour         float32 `yaml:"cost_per_hour" json:"cost_per_hour"`
	Num_storage           uint8   `yaml:"num_storage" json:"num_storage"`
	Max_num_storage       uint8   `yaml:"max_num_storage" json:"max_num_storage"`
	Max_total_storage_TiB uint16  `yaml:"max_total_storage_TiB" json:"max_total_storage_TiB"`
	Net_bw_Gbps           uint16  `yaml:"net_bw_Gbps" json:"net_bw_Gbps"`
	Ebs_bw_Mbps           uint32  `yaml:"ebs_bw_Mbps" json:"ebs_bw_Mbps"`
	Gpu_model             string  `yaml:"gpu_model" json:"gpu_model"`
	Num_gpu               uint8   `yaml:"num_gpu" json:"num_gpu"`
	Gpumem_GiB            uint16  `yaml:"gpumem_GiB" json:"gpumem_GiB"`
	Gpu_p2p               string  `yaml:"gpu_p2p" json:"gpu_p2p"`
	OrderInFilteredResult uint16  `yaml:"orderInFilteredResult" json:"orderInFilteredResult"`
	EvaluationStatus      string  `yaml:"evaluationStatus" json:"evaluationStatus"`
	EvaluationScore_01    float32 `yaml:"evaluationScore_01" json:"evaluationScore_01"`
	EvaluationScore_02    float32 `yaml:"evaluationScore_02" json:"evaluationScore_02"`
	EvaluationScore_03    float32 `yaml:"evaluationScore_03" json:"evaluationScore_03"`
	EvaluationScore_04    float32 `yaml:"evaluationScore_04" json:"evaluationScore_04"`
	EvaluationScore_05    float32 `yaml:"evaluationScore_05" json:"evaluationScore_05"`
	EvaluationScore_06    float32 `yaml:"evaluationScore_06" json:"evaluationScore_06"`
	EvaluationScore_07    float32 `yaml:"evaluationScore_07" json:"evaluationScore_07"`
	EvaluationScore_08    float32 `yaml:"evaluationScore_08" json:"evaluationScore_08"`
	EvaluationScore_09    float32 `yaml:"evaluationScore_09" json:"evaluationScore_09"`
	EvaluationScore_10    float32 `yaml:"evaluationScore_10" json:"evaluationScore_10"`
}

// TbSshKeyCreateRequest - Keypair 생성 요청 구조 Wrapper 정의
type TbSshKeyCreateRequest struct {
	NsId string      `yaml:"nsId" json:"nsId"`
	Item TbSshKeyReq `yaml:"sshKey" json:"sshKey"`
}

// TbSshKeyReq - Keypair 생성 요청 구조 정의
type TbSshKeyReq struct {
	Name           string `yaml:"name" json:"name"`
	ConnectionName string `yaml:"connectionName" json:"connectionName"`
	Description    string `yaml:"description" json:"description"`
}

// TbVNetCreateRequest - VNet 생성 요청 구조 Wrapper 정의
type TbVNetCreateRequest struct {
	NsId string    `yaml:"nsId" json:"nsId"`
	Item TbVNetReq `yaml:"vNet" json:"vNet"`
}

// TbVNetReq - VNet 생성 요청 구조 정의
type TbVNetReq struct { // Tumblebug
	Name           string                `yaml:"name" json:"name"`
	ConnectionName string                `yaml:"connectionName" json:"connectionName"`
	CidrBlock      string                `yaml:"cidrBlock" json:"cidrBlock"`
	SubnetInfoList []SpiderSubnetReqInfo `yaml:"subnetInfoList" json:"subnetInfoList"`
	Description    string                `yaml:"description" json:"description"`
}

// SpiderSubnetReqInfo - Subnet 요청 구조 정의
type SpiderSubnetReqInfo struct { // Spider
	Name         string     `yaml:"Name" json:"Name"`
	IPv4_CIDR    string     `yaml:"IPv4_CIDR" json:"IPv4_CIDR"`
	KeyValueList []KeyValue `yaml:"KeyValueList" json:"KeyValueList"`
}

// ===== [ Implementatiom ] =====

// SetServerAddr - Tumblebug 서버 주소 설정
func (m *MCIRApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	m.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr - Tumblebug 서버 주소 값 조회
func (m *MCIRApi) GetServerAddr() (string, error) {
	return m.gConf.GSL.TumblebugCli.ServerAddr, nil
}

// SetTLSCA - TLS CA 설정
func (m *MCIRApi) SetTLSCA(tlsCAFile string) error {
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
func (m *MCIRApi) GetTLSCA() (string, error) {
	if m.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout - Timeout 설정
func (m *MCIRApi) SetTimeout(timeout time.Duration) error {
	m.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout - Timeout 값 조회
func (m *MCIRApi) GetTimeout() (time.Duration, error) {
	return m.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken - JWT 인증 토큰 설정
func (m *MCIRApi) SetJWTToken(token string) error {
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
func (m *MCIRApi) GetJWTToken() (string, error) {
	if m.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return m.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath - 환경설정 파일 설정
func (m *MCIRApi) SetConfigPath(configFile string) error {
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
func (m *MCIRApi) Open() error {

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
	m.clientMCIR = pb.NewMCIRClient(m.conn)

	// grpc 호출 Wrapper
	m.requestMCIR = &mcir.MCIRRequest{Client: m.clientMCIR, Timeout: tumblebugcli.Timeout, InType: m.inType, OutType: m.outType}

	return nil
}

// Close - 연결 종료
func (m *MCIRApi) Close() {
	if m.conn != nil {
		m.conn.Close()
	}
	if m.jaegerCloser != nil {
		m.jaegerCloser.Close()
	}

	m.jaegerCloser = nil
	m.conn = nil
	m.clientMCIR = nil
	m.requestMCIR = nil
}

// SetInType - 입력 문서 타입 설정 (json/yaml)
func (m *MCIRApi) SetInType(in string) error {
	if in == "json" {
		m.inType = in
	} else if in == "yaml" {
		m.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if m.requestMCIR != nil {
		m.requestMCIR.InType = m.inType
	}

	return nil
}

// GetInType - 입력 문서 타입 값 조회
func (m *MCIRApi) GetInType() (string, error) {
	return m.inType, nil
}

// SetOutType - 출력 문서 타입 설정 (json/yaml)
func (m *MCIRApi) SetOutType(out string) error {
	if out == "json" {
		m.outType = out
	} else if out == "yaml" {
		m.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if m.requestMCIR != nil {
		m.requestMCIR.OutType = m.outType
	}

	return nil
}

// GetOutType - 출력 문서 타입 값 조회
func (m *MCIRApi) GetOutType() (string, error) {
	return m.outType, nil
}

// CreateImageWithInfo - Image 생성
func (m *MCIRApi) CreateImageWithInfo(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateImageWithInfo()
}

// CreateImageWithInfoByParam - Image 생성
func (m *MCIRApi) CreateImageWithInfoByParam(req *TbImageInfoRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateImageWithInfo()
	m.SetInType(holdType)

	return result, err
}

// CreateImageWithID - Image 생성
func (m *MCIRApi) CreateImageWithID(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateImageWithID()
}

// CreateImageWithIDByParam - Image 생성
func (m *MCIRApi) CreateImageWithIDByParam(req *TbImageCreateRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateImageWithID()
	m.SetInType(holdType)

	return result, err
}

// ListImage - Image 목록
func (m *MCIRApi) ListImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListImage()
}

// ListImageByParam - Image 목록
func (m *MCIRApi) ListImageByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"image"}`
	result, err := m.requestMCIR.ListImage()
	m.SetInType(holdType)

	return result, err
}

// GetImage - Image 조회
func (m *MCIRApi) GetImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetImage()
}

// GetImageByParam - Image 조회
func (m *MCIRApi) GetImageByParam(nameSpaceID string, resourceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"image", "resourceId":"` + resourceID + `"}`
	result, err := m.requestMCIR.GetImage()
	m.SetInType(holdType)

	return result, err
}

// DeleteImage - Image 삭제
func (m *MCIRApi) DeleteImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteImage()
}

// DeleteImageByParam - Image 삭제
func (m *MCIRApi) DeleteImageByParam(nameSpaceID string, resourceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"image", "resourceId":"` + resourceID + `", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteImage()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllImage - Image 전체 삭제
func (m *MCIRApi) DeleteAllImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteAllImage()
}

// DeleteAllImageByParam - Image 전체 삭제
func (m *MCIRApi) DeleteAllImageByParam(nameSpaceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"image", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteAllImage()
	m.SetInType(holdType)

	return result, err
}

// FetchImage - Image 가져오기
func (m *MCIRApi) FetchImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.FetchImage()
}

// FetchImageByParam - Image 가져오기
func (m *MCIRApi) FetchImageByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIR.FetchImage()
	m.SetInType(holdType)

	return result, err
}

// ListLookupImage - Image 목록
func (m *MCIRApi) ListLookupImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListLookupImage()
}

// ListLookupImageByParam - Image 목록
func (m *MCIRApi) ListLookupImageByParam(connConfigName string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"ConnectionName":"` + connConfigName + `"}`
	result, err := m.requestMCIR.ListLookupImage()
	m.SetInType(holdType)

	return result, err
}

// GetLookupImage - Image 조회
func (m *MCIRApi) GetLookupImage(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetLookupImage()
}

// GetLookupImageByParam - Image 조회
func (m *MCIRApi) GetLookupImageByParam(connConfigName string, imageId string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"ConnectionName":"` + connConfigName + `", "imageId": "` + imageId + `"}`
	result, err := m.requestMCIR.GetLookupImage()
	m.SetInType(holdType)

	return result, err
}

// CreateSecurityGroup - Security Group 생성
func (m *MCIRApi) CreateSecurityGroup(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateSecurityGroup()
}

// CreateSecurityGroupByParam - Security Group 생성
func (m *MCIRApi) CreateSecurityGroupByParam(req *TbSecurityGroupCreateRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateSecurityGroup()
	m.SetInType(holdType)

	return result, err
}

// ListSecurityGroup - Security Group 목록
func (m *MCIRApi) ListSecurityGroup(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListSecurityGroup()
}

// ListSecurityGroupByParam - Security Group 목록
func (m *MCIRApi) ListSecurityGroupByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"securityGroup"}`
	result, err := m.requestMCIR.ListSecurityGroup()
	m.SetInType(holdType)

	return result, err
}

// GetSecurityGroup - Security Group 조회
func (m *MCIRApi) GetSecurityGroup(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetSecurityGroup()
}

// GetSecurityGroupByParam - Security Group 조회
func (m *MCIRApi) GetSecurityGroupByParam(nameSpaceID string, resourceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"securityGroup", "resourceId":"` + resourceID + `"}`
	result, err := m.requestMCIR.GetSecurityGroup()
	m.SetInType(holdType)

	return result, err
}

// DeleteSecurityGroup - Security Group 삭제
func (m *MCIRApi) DeleteSecurityGroup(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteSecurityGroup()
}

// DeleteSecurityGroupByParam - Security Group 삭제
func (m *MCIRApi) DeleteSecurityGroupByParam(nameSpaceID string, resourceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"securityGroup", "resourceId":"` + resourceID + `", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteSecurityGroup()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllSecurityGroup - Security Group 전체 삭제
func (m *MCIRApi) DeleteAllSecurityGroup(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteAllSecurityGroup()
}

// DeleteAllSecurityGroupByParam - Security Group 전체 삭제
func (m *MCIRApi) DeleteAllSecurityGroupByParam(nameSpaceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"securityGroup", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteAllImage()
	m.SetInType(holdType)

	return result, err
}

// CreateSpecWithInfo - Spec 생성
func (m *MCIRApi) CreateSpecWithInfo(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateSpecWithInfo()
}

// CreateSpecWithInfoByParam - Spec 생성
func (m *MCIRApi) CreateSpecWithInfoByParam(req *TbSpecInfoRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateSpecWithInfo()
	m.SetInType(holdType)

	return result, err
}

// CreateSpecWithSpecName - Spec 생성
func (m *MCIRApi) CreateSpecWithSpecName(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateSpecWithSpecName()
}

// CreateSpecWithSpecNameByParam - Spec 생성
func (m *MCIRApi) CreateSpecWithSpecNameByParam(req *TbSpecCreateRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateSpecWithSpecName()
	m.SetInType(holdType)

	return result, err
}

// ListSpec - Spec 목록
func (m *MCIRApi) ListSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListSpec()
}

// ListSpecByParam - Spec 목록
func (m *MCIRApi) ListSpecByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"spec"}`
	result, err := m.requestMCIR.ListSpec()
	m.SetInType(holdType)

	return result, err
}

// GetSpec - Spec 조회
func (m *MCIRApi) GetSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetSpec()
}

// GetSpecByParam - Spec 조회
func (m *MCIRApi) GetSpecByParam(nameSpaceID string, resourceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"spec", "resourceId":"` + resourceID + `"}`
	result, err := m.requestMCIR.GetSpec()
	m.SetInType(holdType)

	return result, err
}

// DeleteSpec - Spec 삭제
func (m *MCIRApi) DeleteSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteSpec()
}

// DeleteSpecByParam - Spec 삭제
func (m *MCIRApi) DeleteSpecByParam(nameSpaceID string, resourceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"spec", "resourceId":"` + resourceID + `", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteSpec()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllSpec - Spec 전체 삭제
func (m *MCIRApi) DeleteAllSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteAllSpec()
}

// DeleteAllSpecByParam - Spec 전체 삭제
func (m *MCIRApi) DeleteAllSpecByParam(nameSpaceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"spec", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteAllSpec()
	m.SetInType(holdType)

	return result, err
}

// FetchSpec - Spec 가져오기
func (m *MCIRApi) FetchSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.FetchSpec()
}

// FetchSpecByParam - Spec 가져오기
func (m *MCIRApi) FetchSpecByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := m.requestMCIR.FetchSpec()
	m.SetInType(holdType)

	return result, err
}

// FilterSpec
func (m *MCIRApi) FilterSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.FilterSpec()
}

// ListLookupSpec - Spec 목록
func (m *MCIRApi) ListLookupSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListLookupSpec()
}

// ListLookupSpecByParam - Spec 목록
func (m *MCIRApi) ListLookupSpecByParam(connConfigName string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"ConnectionName":"` + connConfigName + `"}`
	result, err := m.requestMCIR.ListLookupSpec()
	m.SetInType(holdType)

	return result, err
}

// GetLookupSpec - Spec 조회
func (m *MCIRApi) GetLookupSpec(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetLookupSpec()
}

// GetLookupSpecByParam - Spec 조회
func (m *MCIRApi) GetLookupSpecByParam(connConfigName string, specName string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"ConnectionName":"` + connConfigName + `", "specName": "` + specName + `"}`
	result, err := m.requestMCIR.GetLookupSpec()
	m.SetInType(holdType)

	return result, err
}

// CreateSshKey - KeyPair 생성
func (m *MCIRApi) CreateSshKey(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateSshKey()
}

// CreateSshKeyByParam - KeyPair 생성
func (m *MCIRApi) CreateSshKeyByParam(req *TbSshKeyCreateRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateSshKey()
	m.SetInType(holdType)

	return result, err
}

// ListSshKey - KeyPair 목록
func (m *MCIRApi) ListSshKey(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListSshKey()
}

// ListSshKeyByParam - KeyPair 목록
func (m *MCIRApi) ListSshKeyByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"sshKey"}`
	result, err := m.requestMCIR.ListSshKey()
	m.SetInType(holdType)

	return result, err
}

// GetSshKey - KeyPair 조회
func (m *MCIRApi) GetSshKey(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetSshKey()
}

// GetSshKeyByParam - KeyPair 조회
func (m *MCIRApi) GetSshKeyByParam(nameSpaceID string, resourceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"sshKey", "resourceId":"` + resourceID + `"}`
	result, err := m.requestMCIR.GetSshKey()
	m.SetInType(holdType)

	return result, err
}

// DeleteSshKey - KeyPair 삭제
func (m *MCIRApi) DeleteSshKey(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteSshKey()
}

// DeleteSshKeyByParam - KeyPair 삭제
func (m *MCIRApi) DeleteSshKeyByParam(nameSpaceID string, resourceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"sshKey", "resourceId":"` + resourceID + `", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteSshKey()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllSshKey - KeyPair 전체 삭제
func (m *MCIRApi) DeleteAllSshKey(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteAllSshKey()
}

// DeleteAllSshKeyByParam - KeyPair 전체 삭제
func (m *MCIRApi) DeleteAllSshKeyByParam(nameSpaceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"sshKey", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteAllSshKey()
	m.SetInType(holdType)

	return result, err
}

// CreateVNet - VNet 생성
func (m *MCIRApi) CreateVNet(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CreateVNet()
}

// CreateVNetByParam - VNet 생성
func (m *MCIRApi) CreateVNetByParam(req *TbVNetCreateRequest) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	m.requestMCIR.InData = string(j)
	result, err := m.requestMCIR.CreateVNet()
	m.SetInType(holdType)

	return result, err
}

// ListVNet - VNet 목록
func (m *MCIRApi) ListVNet(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.ListVNet()
}

// ListVNetByParam - VNet 목록
func (m *MCIRApi) ListVNetByParam(nameSpaceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"vNet"}`
	result, err := m.requestMCIR.ListVNet()
	m.SetInType(holdType)

	return result, err
}

// GetVNet - VNet 조회
func (m *MCIRApi) GetVNet(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.GetVNet()
}

// GetVNetByParam - VNet 조회
func (m *MCIRApi) GetVNetByParam(nameSpaceID string, resourceID string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"vNet", "resourceId":"` + resourceID + `"}`
	result, err := m.requestMCIR.GetVNet()
	m.SetInType(holdType)

	return result, err
}

// DeleteVNet - VNet 삭제
func (m *MCIRApi) DeleteVNet(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteVNet()
}

// DeleteVNetByParam - VNet 삭제
func (m *MCIRApi) DeleteVNetByParam(nameSpaceID string, resourceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"vNet", "resourceId":"` + resourceID + `", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteVNet()
	m.SetInType(holdType)

	return result, err
}

// DeleteAllVNet -  VNet 전체 삭제
func (m *MCIRApi) DeleteAllVNet(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.DeleteAllVNet()
}

// DeleteAllVNetByParam -  VNet 전체 삭제
func (m *MCIRApi) DeleteAllVNetByParam(nameSpaceID string, force string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"vNet", "force":"` + force + `"}`
	result, err := m.requestMCIR.DeleteAllVNet()
	m.SetInType(holdType)

	return result, err
}

// CheckResource - Resouce 체크
func (m *MCIRApi) CheckResource(doc string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	m.requestMCIR.InData = doc
	return m.requestMCIR.CheckResource()
}

// CheckResourceByParam - Resouce 체크
func (m *MCIRApi) CheckResourceByParam(nameSpaceID string, resourceID string, resourceType string) (string, error) {
	if m.requestMCIR == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := m.GetInType()
	m.SetInType("json")
	m.requestMCIR.InData = `{"nsId":"` + nameSpaceID + `", "resourceType":"vNet", "resourceId":"` + resourceID + `", "resourceType":"` + resourceType + `"}`
	result, err := m.requestMCIR.CheckResource()
	m.SetInType(holdType)

	return result, err
}

// ===== [ Private Functiom ] =====

// ===== [ Public Functiom ] =====

// NewMCIRManager - MCIR API 객체 생성
func NewMCIRManager() (m *MCIRApi) {

	m = &MCIRApi{}
	m.gConf = &config.GrpcConfig{}
	m.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	m.jaegerCloser = nil
	m.conn = nil
	m.clientMCIR = nil
	m.requestMCIR = nil

	m.inType = "json"
	m.outType = "json"

	return
}
