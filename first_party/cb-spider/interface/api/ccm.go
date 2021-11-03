// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package api

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	gc "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/common"
	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/config"
	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
	pb "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/stub/cbspider"
	"github.com/cloud-barista/cb-spider/interface/api/request"

	"google.golang.org/grpc"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// CCMApi - CCM API 구조 정의
type CCMApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientCCM    pb.CCMClient
	clientSSH    pb.SSHClient
	requestCCM   *request.CCMRequest
	requestSSH   *request.SSHRequest
	inType       string
	outType      string
}

// ImageReq - Image 정보 생성 요청 구조 정의
type ImageReq struct {
	ConnectionName string    `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        ImageInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// ImageInfo - Image 정보 구조 정의
type ImageInfo struct {
	Name string `yaml:"Name" json:"Name"`
}

// VPCReq - VPC 정보 생성 요청 구조 정의
type VPCReq struct {
	ConnectionName string  `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        VPCInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// VPCInfo - VPC 정보 구조 정의
type VPCInfo struct {
	Name           string        `yaml:"Name" json:"Name"`
	IPv4_CIDR      string        `yaml:"IPv4_CIDR" json:"IPv4_CIDR"`
	SubnetInfoList *[]SubnetInfo `yaml:"SubnetInfoList" json:"SubnetInfoList"`
}

// VPCRegisterReq - VPC Register 정보 생성 요청 구조 정의
type VPCRegisterReq struct {
	ConnectionName string          `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        VPCRegisterInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// VPCRegisterInfo - VPC Register 정보 구조 정의
type VPCRegisterInfo struct {
	Name  string `yaml:"Name" json:"Name"`
	CSPId string `yaml:"CSPId" json:"CSPId"`
}

// SubnetInfo - Subnet 정보 구조 정의
type SubnetInfo struct {
	Name      string `yaml:"Name" json:"Name"`
	IPv4_CIDR string `yaml:"IPv4_CIDR" json:"IPv4_CIDR"`
}

type SubnetReq struct {
	ConnectionName string     `yaml:"ConnectionName" json:"ConnectionName"`
	VPCName        string     `yaml:"VPCName" json:"VPCName"`
	ReqInfo        SubnetInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// SecurityReq - Security 정보 생성 요청 구조 정의
type SecurityReq struct {
	ConnectionName string       `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        SecurityInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// SecurityInfo - Security 정보 구조 정의
type SecurityInfo struct {
	Name          string              `yaml:"Name" json:"Name"`
	VPCName       string              `yaml:"VPCName" json:"VPCName"`
	Direction     string              `yaml:"Direction" json:"Direction"`
	SecurityRules *[]SecurityRuleInfo `yaml:"SecurityRules" json:"SecurityRules"`
}

// SecurityRuleInfo - Security Rule 정보 구조 정의
type SecurityRuleInfo struct {
	FromPort   string `yaml:"FromPort" json:"FromPort"`
	ToPort     string `yaml:"ToPort" json:"ToPort"`
	IPProtocol string `yaml:"IPProtocol" json:"IPProtocol"`
	Direction  string `yaml:"Direction" json:"Direction"`
	CIDR       string `yaml:"CIDR" json:"CIDR"`
}

// SecurityRegisterReq - Security Register 정보 생성 요청 구조 정의
type SecurityRegisterReq struct {
	ConnectionName string               `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        SecurityRegisterInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// SecurityRegisterInfo - Security Register 정보 구조 정의
type SecurityRegisterInfo struct {
	VPCName string `yaml:"VPCName" json:"VPCName"`
	Name    string `yaml:"Name" json:"Name"`
	CSPId   string `yaml:"CSPId" json:"CSPId"`
}

// KeyReq - Key Pair 정보 생성 요청 구조 정의
type KeyReq struct {
	ConnectionName string  `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        KeyInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// KeyInfo - Key 정보 구조 정의
type KeyInfo struct {
	Name string `yaml:"Name" json:"Name"`
}

// KeyRegisterReq - Key Register 정보 생성 요청 구조 정의
type KeyRegisterReq struct {
	ConnectionName string          `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        KeyRegisterInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// KeyRegisterInfo - Key Register 정보 구조 정의
type KeyRegisterInfo struct {
	Name  string `yaml:"Name" json:"Name"`
	CSPId string `yaml:"CSPId" json:"CSPId"`
}

// VMReq - VM 정보 생성 요청 구조 정의
type VMReq struct {
	ConnectionName string `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        VMInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

//VMInfo - VM 정보 구조 정의
type VMInfo struct {
	Name               string   `yaml:"Name" json:"Name"`
	ImageName          string   `yaml:"ImageName" json:"ImageName"`
	VPCName            string   `yaml:"VPCName" json:"VPCName"`
	SubnetName         string   `yaml:"SubnetName" json:"SubnetName"`
	SecurityGroupNames []string `yaml:"SecurityGroupNames" json:"SecurityGroupNames"`
	VMSpecName         string   `yaml:"VMSpecName" json:"VMSpecName"`
	KeyPairName        string   `yaml:"KeyPairName" json:"KeyPairName"`

	VMUserId     string `yaml:"VMUserId" json:"VMUserId"`
	VMUserPasswd string `yaml:"VMUserPasswd" json:"VMUserPasswd"`
}

// VMRegisterReq - VM Register 정보 생성 요청 구조 정의
type VMRegisterReq struct {
	ConnectionName string         `yaml:"ConnectionName" json:"ConnectionName"`
	ReqInfo        VMRegisterInfo `yaml:"ReqInfo" json:"ReqInfo"`
}

// VMRegisterInfo - VM Register 정보 구조 정의
type VMRegisterInfo struct {
	Name  string `yaml:"Name" json:"Name"`
	CSPId string `yaml:"CSPId" json:"CSPId"`
}

// SSHRUNReq - SSH 실행 요청 구조 정의
type SSHRUNReq struct {
	UserName   string   `yaml:"UserName" json:"UserName"`
	PrivateKey []string `yaml:"PrivateKey" json:"PrivateKey"`
	ServerPort string   `yaml:"ServerPort" json:"ServerPort"`
	Command    string   `yaml:"Command" json:"Command"`
}

// ===== [ Implementations ] =====

// SetServerAddr - Spider 서버 주소 설정
func (ccm *CCMApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	ccm.gConf.GSL.SpiderCli.ServerAddr = addr
	return nil
}

// GetServerAddr - Spider 서버 주소 값 조회
func (ccm *CCMApi) GetServerAddr() (string, error) {
	return ccm.gConf.GSL.SpiderCli.ServerAddr, nil
}

// SetTLSCA - TLS CA 설정
func (ccm *CCMApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if ccm.gConf.GSL.SpiderCli.TLS == nil {
		ccm.gConf.GSL.SpiderCli.TLS = &config.TLSConfig{}
	}

	ccm.gConf.GSL.SpiderCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA - TLS CA 값 조회
func (ccm *CCMApi) GetTLSCA() (string, error) {
	if ccm.gConf.GSL.SpiderCli.TLS == nil {
		return "", nil
	}

	return ccm.gConf.GSL.SpiderCli.TLS.TLSCA, nil
}

// SetTimeout - Timeout 설정
func (ccm *CCMApi) SetTimeout(timeout time.Duration) error {
	ccm.gConf.GSL.SpiderCli.Timeout = timeout
	return nil
}

// GetTimeout - Timeout 값 조회
func (ccm *CCMApi) GetTimeout() (time.Duration, error) {
	return ccm.gConf.GSL.SpiderCli.Timeout, nil
}

// SetJWTToken - JWT 인증 토큰 설정
func (ccm *CCMApi) SetJWTToken(token string) error {
	if token == "" {
		return errors.New("parameter is empty")
	}

	if ccm.gConf.GSL.SpiderCli.Interceptors == nil {
		ccm.gConf.GSL.SpiderCli.Interceptors = &config.InterceptorsConfig{}
		ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}
	if ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT == nil {
		ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}

	ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT.JWTToken = token
	return nil
}

// GetJWTToken - JWT 인증 토큰 값 조회
func (ccm *CCMApi) GetJWTToken() (string, error) {
	if ccm.gConf.GSL.SpiderCli.Interceptors == nil {
		return "", nil
	}
	if ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return ccm.gConf.GSL.SpiderCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath - 환경설정 파일 설정
func (ccm *CCMApi) SetConfigPath(configFile string) error {
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

	// SPIDER CLIENT 필수 입력 항목 체크
	spidercli := gConf.GSL.SpiderCli

	if spidercli == nil {
		return errors.New("spidercli field are not specified")
	}

	if spidercli.ServerAddr == "" {
		return errors.New("spidercli.server_addr field are not specified")
	}

	if spidercli.Timeout == 0 {
		spidercli.Timeout = 90 * time.Second
	}

	if spidercli.TLS != nil {
		if spidercli.TLS.TLSCA == "" {
			return errors.New("spidercli.tls.tls_ca field are not specified")
		}
	}

	if spidercli.Interceptors != nil {
		if spidercli.Interceptors.AuthJWT != nil {
			if spidercli.Interceptors.AuthJWT.JWTToken == "" {
				return errors.New("spidercli.interceptors.auth_jwt.jwt_token field are not specified")
			}
		}
		if spidercli.Interceptors.Opentracing != nil {
			if spidercli.Interceptors.Opentracing.Jaeger != nil {
				if spidercli.Interceptors.Opentracing.Jaeger.Endpoint == "" {
					return errors.New("spidercli.interceptors.opentracing.jaeger.endpoint field are not specified")
				}
			}
		}
	}

	ccm.gConf = &gConf
	return nil
}

// Open - 연결 설정
func (ccm *CCMApi) Open() error {

	spidercli := ccm.gConf.GSL.SpiderCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(spidercli)
	if err != nil {
		return err
	}

	if closer != nil {
		ccm.jaegerCloser = closer
	}

	ccm.conn = cbconn.Conn

	// grpc 클라이언트 생성
	ccm.clientCCM = pb.NewCCMClient(ccm.conn)
	ccm.clientSSH = pb.NewSSHClient(ccm.conn)

	// grpc 호출 Wrapper
	ccm.requestCCM = &request.CCMRequest{Client: ccm.clientCCM, Timeout: spidercli.Timeout, InType: ccm.inType, OutType: ccm.outType}
	ccm.requestSSH = &request.SSHRequest{Client: ccm.clientSSH, Timeout: spidercli.Timeout, InType: ccm.inType, OutType: ccm.outType}

	return nil
}

// Close - 연결 종료
func (ccm *CCMApi) Close() {
	if ccm.conn != nil {
		ccm.conn.Close()
	}
	if ccm.jaegerCloser != nil {
		ccm.jaegerCloser.Close()
	}

	ccm.jaegerCloser = nil
	ccm.conn = nil
	ccm.clientCCM = nil
	ccm.clientSSH = nil
	ccm.requestCCM = nil
	ccm.requestSSH = nil
}

// SetInType - 입력 문서 타입 설정 (json/yaml)
func (ccm *CCMApi) SetInType(in string) error {
	if in == "json" {
		ccm.inType = in
	} else if in == "yaml" {
		ccm.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if ccm.requestCCM != nil {
		ccm.requestCCM.InType = ccm.inType
	}

	return nil
}

// GetInType - 입력 문서 타입 값 조회
func (ccm *CCMApi) GetInType() (string, error) {
	return ccm.inType, nil
}

// SetOutType - 출력 문서 타입 설정 (json/yaml)
func (ccm *CCMApi) SetOutType(out string) error {
	if out == "json" {
		ccm.outType = out
	} else if out == "yaml" {
		ccm.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if ccm.requestCCM != nil {
		ccm.requestCCM.OutType = ccm.outType
	}

	return nil
}

// GetOutType - 출력 문서 타입 값 조회
func (ccm *CCMApi) GetOutType() (string, error) {
	return ccm.outType, nil
}

// CreateImage - Image 생성
func (ccm *CCMApi) CreateImage(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.CreateImage()
}

// CreateImageByParam - Image 생성
func (ccm *CCMApi) CreateImageByParam(req *ImageReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.CreateImage()
	ccm.SetInType(holdType)

	return result, err
}

// ListImage - Image 목록
func (ccm *CCMApi) ListImage(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListImage()
}

// ListImageByParam - Image 목록
func (ccm *CCMApi) ListImageByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListImage()
	ccm.SetInType(holdType)

	return result, err
}

// GetImage - Image 조회
func (ccm *CCMApi) GetImage(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetImage()
}

// GetImageByParam - Image 조회
func (ccm *CCMApi) GetImageByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetImage()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteImage - Image 삭제
func (ccm *CCMApi) DeleteImage(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteImage()
}

// DeleteImageByParam - Image 삭제
func (ccm *CCMApi) DeleteImageByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.DeleteImage()
	ccm.SetInType(holdType)

	return result, err
}

// ListVMSpec - VM Spec 목록
func (ccm *CCMApi) ListVMSpec(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListVMSpec()
}

// ListVMSpecByParam - VM Spec 목록
func (ccm *CCMApi) ListVMSpecByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListVMSpec()
	ccm.SetInType(holdType)

	return result, err
}

// GetVMSpec - VM Spec 조회
func (ccm *CCMApi) GetVMSpec(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetVMSpec()
}

// GetVMSpecByParam - VM Spec 조회
func (ccm *CCMApi) GetVMSpecByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetVMSpec()
	ccm.SetInType(holdType)

	return result, err
}

// ListOrgVMSpec - 클라우드의 원래 VM Spec 목록
func (ccm *CCMApi) ListOrgVMSpec(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListOrgVMSpec()
}

// ListOrgVMSpecByParam - 클라우드의 원래 VM Spec 목록
func (ccm *CCMApi) ListOrgVMSpecByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListOrgVMSpec()
	ccm.SetInType(holdType)

	return result, err
}

// GetOrgVMSpec - 클라우드의 원래 VM Spec 조회
func (ccm *CCMApi) GetOrgVMSpec(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetOrgVMSpec()
}

// GetOrgVMSpecByParam - 클라우드의 원래 VM Spec 조회
func (ccm *CCMApi) GetOrgVMSpecByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetOrgVMSpec()
	ccm.SetInType(holdType)

	return result, err
}

// CreateVPC - VPC 생성
func (ccm *CCMApi) CreateVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.CreateVPC()
}

// CreateVPCByParam - VPC 생성
func (ccm *CCMApi) CreateVPCByParam(req *VPCReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.CreateVPC()
	ccm.SetInType(holdType)

	return result, err
}

// ListVPC - VPC 목록
func (ccm *CCMApi) ListVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListVPC()
}

// ListVPCByParam - VPC 목록
func (ccm *CCMApi) ListVPCByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListVPC()
	ccm.SetInType(holdType)

	return result, err
}

// GetVPC - VPC 조회
func (ccm *CCMApi) GetVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetVPC()
}

// GetVPCByParam - VPC 조회
func (ccm *CCMApi) GetVPCByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetVPC()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteVPC - VPC 삭제
func (ccm *CCMApi) DeleteVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteVPC()
}

// DeleteVPCByParam - VPC 삭제
func (ccm *CCMApi) DeleteVPCByParam(connectionName string, name string, force string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `", "force":"` + force + `"}`
	result, err := ccm.requestCCM.DeleteVPC()
	ccm.SetInType(holdType)

	return result, err
}

// ListAllVPC - 관리 VPC 목록
func (ccm *CCMApi) ListAllVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListAllVPC()
}

// ListAllVPCByParam - 관리 VPC 목록
func (ccm *CCMApi) ListAllVPCByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListAllVPC()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteCSPVPC - 관리 VPC 삭제
func (ccm *CCMApi) DeleteCSPVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteCSPVPC()
}

// DeleteCSPVPCByParam - 관리 VPC 삭제
func (ccm *CCMApi) DeleteCSPVPCByParam(connectionName string, id string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Id":"` + id + `"}`
	result, err := ccm.requestCCM.DeleteCSPVPC()
	ccm.SetInType(holdType)

	return result, err
}

// AddSubnet - Subnet 추가
func (ccm *CCMApi) AddSubnet(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.AddSubnet()
}

// AddSubnetByParam - Subnet 추가
func (ccm *CCMApi) AddSubnetByParam(req *SubnetReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.AddSubnet()
	ccm.SetInType(holdType)

	return result, err
}

// RemoveSubnet - Subnet 삭제
func (ccm *CCMApi) RemoveSubnet(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RemoveSubnet()
}

// RemoveSubnetByParam - Subnet 삭제
func (ccm *CCMApi) RemoveSubnetByParam(connectionName string, vpcName string, subnetName string, force string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "VPCName":"` + vpcName + `", "SubnetName":"` + subnetName + `", "force":"` + force + `"}`
	result, err := ccm.requestCCM.RemoveSubnet()
	ccm.SetInType(holdType)

	return result, err
}

// RemoveCSPSubnet - CSP Subnet 삭제
func (ccm *CCMApi) RemoveCSPSubnet(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RemoveCSPSubnet()
}

// RemoveCSPSubnetByParam - CSP Subnet 삭제
func (ccm *CCMApi) RemoveCSPSubnetByParam(connectionName string, vpcName string, id string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "VPCName":"` + vpcName + `", "Id":"` + id + `"}`
	result, err := ccm.requestCCM.RemoveCSPSubnet()
	ccm.SetInType(holdType)

	return result, err
}

// RegisterVPC - VPC 등록
func (ccm *CCMApi) RegisterVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RegisterVPC()
}

// RegisterVPCByParam - VPC 등록
func (ccm *CCMApi) RegisterVPCByParam(req *VPCRegisterReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.RegisterVPC()
	ccm.SetInType(holdType)

	return result, err
}

// UnregisterVPC - VPC 제거
func (ccm *CCMApi) UnregisterVPC(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.UnregisterVPC()
}

// UnregisterVPCByParam - VPC 제거
func (ccm *CCMApi) UnregisterVPCByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.UnregisterVPC()
	ccm.SetInType(holdType)

	return result, err
}

// CreateSecurity - Security 생성
func (ccm *CCMApi) CreateSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.CreateSecurity()
}

// CreateSecurityByParam - Security 생성
func (ccm *CCMApi) CreateSecurityByParam(req *SecurityReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.CreateSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// ListSecurity - Security 목록
func (ccm *CCMApi) ListSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListSecurity()
}

// ListSecurityByParam - Security 목록
func (ccm *CCMApi) ListSecurityByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// GetSecurity - Security 조회
func (ccm *CCMApi) GetSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetSecurity()
}

// GetSecurityByParam - Security 조회
func (ccm *CCMApi) GetSecurityByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteSecurity - Security 삭제
func (ccm *CCMApi) DeleteSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteSecurity()
}

// DeleteSecurityByParam - Security 삭제
func (ccm *CCMApi) DeleteSecurityByParam(connectionName string, name string, force string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `", "force":"` + force + `"}`
	result, err := ccm.requestCCM.DeleteSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// ListAllSecurity - 관리 Security 목록
func (ccm *CCMApi) ListAllSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListAllSecurity()
}

// ListAllSecurityByParam - 관리 Security 목록
func (ccm *CCMApi) ListAllSecurityByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListAllSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteCSPSecurity - 관리 Security 삭제
func (ccm *CCMApi) DeleteCSPSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteCSPSecurity()
}

// DeleteCSPSecurityByParam - 관리 Security 삭제
func (ccm *CCMApi) DeleteCSPSecurityByParam(connectionName string, id string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Id":"` + id + `"}`
	result, err := ccm.requestCCM.DeleteCSPSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// RegisterSecurity - Security 등록
func (ccm *CCMApi) RegisterSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RegisterSecurity()
}

// RegisterSecurityByParam - Security 등록
func (ccm *CCMApi) RegisterSecurityByParam(req *SecurityRegisterReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.RegisterSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// UnregisterSecurity - Security 제거
func (ccm *CCMApi) UnregisterSecurity(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.UnregisterSecurity()
}

// UnregisterSecurityByParam - Security 제거
func (ccm *CCMApi) UnregisterSecurityByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.UnregisterSecurity()
	ccm.SetInType(holdType)

	return result, err
}

// CreateKey - Key Pair 생성
func (ccm *CCMApi) CreateKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.CreateKey()
}

// CreateKeyByParam - Key Pair 생성
func (ccm *CCMApi) CreateKeyByParam(req *KeyReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.CreateKey()
	ccm.SetInType(holdType)

	return result, err
}

// ListKey - Key Pair 목록
func (ccm *CCMApi) ListKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListKey()
}

// ListKeyByParam - Key Pair 목록
func (ccm *CCMApi) ListKeyByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListKey()
	ccm.SetInType(holdType)

	return result, err
}

// GetKey - Key Pair 조회
func (ccm *CCMApi) GetKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetKey()
}

// GetKeyByParam - Key Pair 조회
func (ccm *CCMApi) GetKeyByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetKey()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteKey - Key Pair 삭제
func (ccm *CCMApi) DeleteKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteKey()
}

// DeleteKeyByParam - Key Pair 삭제
func (ccm *CCMApi) DeleteKeyByParam(connectionName string, name string, force string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `", "force":"` + force + `"}`
	result, err := ccm.requestCCM.DeleteKey()
	ccm.SetInType(holdType)

	return result, err
}

// ListAllKey - 관리 Key Pair 목록
func (ccm *CCMApi) ListAllKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListAllKey()
}

// ListAllKeyByParam - 관리 Key Pair 목록
func (ccm *CCMApi) ListAllKeyByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListAllKey()
	ccm.SetInType(holdType)

	return result, err
}

// DeleteCSPKey - 관리 Key Pair 삭제
func (ccm *CCMApi) DeleteCSPKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.DeleteCSPKey()
}

// DeleteCSPKeyByParam - 관리 Key Pair 삭제
func (ccm *CCMApi) DeleteCSPKeyByParam(connectionName string, id string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Id":"` + id + `"}`
	result, err := ccm.requestCCM.DeleteCSPKey()
	ccm.SetInType(holdType)

	return result, err
}

// RegisterKey - KeyPair 등록
func (ccm *CCMApi) RegisterKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RegisterKey()
}

// RegisterKeyByParam - KeyPair 등록
func (ccm *CCMApi) RegisterKeyByParam(req *KeyRegisterReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.RegisterKey()
	ccm.SetInType(holdType)

	return result, err
}

// UnregisterKey - KeyPair 제거
func (ccm *CCMApi) UnregisterKey(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.UnregisterKey()
}

// UnregisterKeyByParam - KeyPair 제거
func (ccm *CCMApi) UnregisterKeyByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.UnregisterKey()
	ccm.SetInType(holdType)

	return result, err
}

// StartVM - VM 시작
func (ccm *CCMApi) StartVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.StartVM()
}

// StartVMByParam - VM 시작
func (ccm *CCMApi) StartVMByParam(req *VMReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.StartVM()
	ccm.SetInType(holdType)

	return result, err
}

// ControlVM - VM 제어
func (ccm *CCMApi) ControlVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ControlVM()
}

// ControlVMByParam - VM 제어
func (ccm *CCMApi) ControlVMByParam(connectionName string, name string, action string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `", "action":"` + action + `"}`
	result, err := ccm.requestCCM.ControlVM()
	ccm.SetInType(holdType)

	return result, err
}

// ListVMStatus - VM 상태 목록
func (ccm *CCMApi) ListVMStatus(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListVMStatus()
}

// ListVMStatusByParam - VM 상태 목록
func (ccm *CCMApi) ListVMStatusByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListVMStatus()
	ccm.SetInType(holdType)

	return result, err
}

// GetVMStatus - VM 상태 조회
func (ccm *CCMApi) GetVMStatus(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetVMStatus()
}

// GetVMStatusByParam - VM 상태 조회
func (ccm *CCMApi) GetVMStatusByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetVMStatus()
	ccm.SetInType(holdType)

	return result, err
}

// ListVM - VM 목록
func (ccm *CCMApi) ListVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListVM()
}

// ListVMByParam - VM 목록
func (ccm *CCMApi) ListVMByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListVM()
	ccm.SetInType(holdType)

	return result, err
}

// GetVM - VM 조회
func (ccm *CCMApi) GetVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.GetVM()
}

// GetVMByParam - VM 조회
func (ccm *CCMApi) GetVMByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.GetVM()
	ccm.SetInType(holdType)

	return result, err
}

// TerminateVM - VM 삭제
func (ccm *CCMApi) TerminateVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.TerminateVM()
}

// TerminateVMByParam - VM 삭제
func (ccm *CCMApi) TerminateVMByParam(connectionName string, name string, force string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `", "force":"` + force + `"}`
	result, err := ccm.requestCCM.TerminateVM()
	ccm.SetInType(holdType)

	return result, err
}

// ListAllVM - 관리 VM 목록
func (ccm *CCMApi) ListAllVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.ListAllVM()
}

// ListAllVMByParam - 관리 VM 목록
func (ccm *CCMApi) ListAllVMByParam(connectionName string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `"}`
	result, err := ccm.requestCCM.ListAllVM()
	ccm.SetInType(holdType)

	return result, err
}

// TerminateCSPVM - 관리 VM 삭제
func (ccm *CCMApi) TerminateCSPVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.TerminateCSPVM()
}

// TerminateCSPVMByParam - 관리 VM 삭제
func (ccm *CCMApi) TerminateCSPVMByParam(connectionName string, id string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Id":"` + id + `"}`
	result, err := ccm.requestCCM.TerminateCSPVM()
	ccm.SetInType(holdType)

	return result, err
}

// RegisterVM - VM 등록
func (ccm *CCMApi) RegisterVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.RegisterVM()
}

// RegisterVMByParam - VM 등록
func (ccm *CCMApi) RegisterVMByParam(req *VMRegisterReq) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestCCM.InData = string(j)
	result, err := ccm.requestCCM.RegisterVM()
	ccm.SetInType(holdType)

	return result, err
}

// UnregisterVM - VM 제거
func (ccm *CCMApi) UnregisterVM(doc string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestCCM.InData = doc
	return ccm.requestCCM.UnregisterVM()
}

// UnregisterVMByParam - VM 제거
func (ccm *CCMApi) UnregisterVMByParam(connectionName string, name string) (string, error) {
	if ccm.requestCCM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	ccm.requestCCM.InData = `{"ConnectionName":"` + connectionName + `", "Name":"` + name + `"}`
	result, err := ccm.requestCCM.UnregisterVM()
	ccm.SetInType(holdType)

	return result, err
}

// SSHRun - SSH 실행
func (ccm *CCMApi) SSHRun(doc string) (string, error) {
	if ccm.requestSSH == nil {
		return "", errors.New("The Open() function must be called")
	}

	ccm.requestSSH.InData = doc
	return ccm.requestSSH.SSHRun()
}

// SSHRunByParam - SSH 실행
func (ccm *CCMApi) SSHRunByParam(req *SSHRUNReq) (string, error) {
	if ccm.requestSSH == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ccm.GetInType()
	ccm.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ccm.requestSSH.InData = string(j)
	result, err := ccm.requestSSH.SSHRun()
	ccm.SetInType(holdType)

	return result, err
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCloudResourceHandler - CCM API 객체 생성
func NewCloudResourceHandler() (ccm *CCMApi) {

	ccm = &CCMApi{}
	ccm.gConf = &config.GrpcConfig{}
	ccm.gConf.GSL.SpiderCli = &config.GrpcClientConfig{}

	ccm.jaegerCloser = nil
	ccm.conn = nil
	ccm.clientCCM = nil
	ccm.clientSSH = nil
	ccm.requestCCM = nil
	ccm.requestSSH = nil

	ccm.inType = "json"
	ccm.outType = "json"

	return
}
