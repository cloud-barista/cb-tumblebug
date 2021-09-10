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
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/request/common"

	core_common "github.com/cloud-barista/cb-tumblebug/src/core/common"

	"google.golang.org/grpc"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// NSApi is for NS API 구조 정의
type NSApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientNS     pb.NSClient
	requestNS    *common.NSRequest
	inType       string
	outType      string
}

// NsReq is for Namespace 정보 생성 요청 구조 정의
// type NsReq struct {
// 	Name        string `yaml:"name" json:"name"`
// 	Description string `yaml:"description" json:"description"`
// }

// ===== [ Implementations ] =====

// SetServerAddr is to Tumblebug 서버 주소 설정
func (ns *NSApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	ns.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr is to Tumblebug 서버 주소 값 조회
func (ns *NSApi) GetServerAddr() (string, error) {
	return ns.gConf.GSL.TumblebugCli.ServerAddr, nil
}

// SetTLSCA is to TLS CA 설정
func (ns *NSApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if ns.gConf.GSL.TumblebugCli.TLS == nil {
		ns.gConf.GSL.TumblebugCli.TLS = &config.TLSConfig{}
	}

	ns.gConf.GSL.TumblebugCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA is to TLS CA 값 조회
func (ns *NSApi) GetTLSCA() (string, error) {
	if ns.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return ns.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout is to Timeout 설정
func (ns *NSApi) SetTimeout(timeout time.Duration) error {
	ns.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout is to Timeout 값 조회
func (ns *NSApi) GetTimeout() (time.Duration, error) {
	return ns.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken is to JWT 인증 토큰 설정
func (ns *NSApi) SetJWTToken(token string) error {
	if token == "" {
		return errors.New("parameter is empty")
	}

	if ns.gConf.GSL.TumblebugCli.Interceptors == nil {
		ns.gConf.GSL.TumblebugCli.Interceptors = &config.InterceptorsConfig{}
		ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}
	if ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}

	ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken = token
	return nil
}

// GetJWTToken is to JWT 인증 토큰 값 조회
func (ns *NSApi) GetJWTToken() (string, error) {
	if ns.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return ns.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath is to 환경설정 파일 설정
func (ns *NSApi) SetConfigPath(configFile string) error {
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

	ns.gConf = &gConf
	return nil
}

// Open is to 연결 설정
func (ns *NSApi) Open() error {

	tumblebugcli := ns.gConf.GSL.TumblebugCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(tumblebugcli)
	if err != nil {
		return err
	}

	if closer != nil {
		ns.jaegerCloser = closer
	}

	ns.conn = cbconn.Conn

	// grpc 클라이언트 생성
	ns.clientNS = pb.NewNSClient(ns.conn)

	// grpc 호출 Wrapper
	ns.requestNS = &common.NSRequest{Client: ns.clientNS, Timeout: tumblebugcli.Timeout, InType: ns.inType, OutType: ns.outType}

	return nil
}

// Close is to 연결 종료
func (ns *NSApi) Close() {
	if ns.conn != nil {
		ns.conn.Close()
	}
	if ns.jaegerCloser != nil {
		ns.jaegerCloser.Close()
	}

	ns.jaegerCloser = nil
	ns.conn = nil
	ns.clientNS = nil
	ns.requestNS = nil
}

// SetInType is to 입력 문서 타입 설정 (json/yaml)
func (ns *NSApi) SetInType(in string) error {
	if in == "json" {
		ns.inType = in
	} else if in == "yaml" {
		ns.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if ns.requestNS != nil {
		ns.requestNS.InType = ns.inType
	}

	return nil
}

// GetInType is to 입력 문서 타입 값 조회
func (ns *NSApi) GetInType() (string, error) {
	return ns.inType, nil
}

// SetOutType is to 출력 문서 타입 설정 (json/yaml)
func (ns *NSApi) SetOutType(out string) error {
	if out == "json" {
		ns.outType = out
	} else if out == "yaml" {
		ns.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if ns.requestNS != nil {
		ns.requestNS.OutType = ns.outType
	}

	return nil
}

// GetOutType is to 출력 문서 타입 값 조회
func (ns *NSApi) GetOutType() (string, error) {
	return ns.outType, nil
}

// CreateNS is to Namespace 생성
func (ns *NSApi) CreateNS(doc string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	ns.requestNS.InData = doc
	return ns.requestNS.CreateNS()
}

// CreateNSByParam is to Namespace 생성
func (ns *NSApi) CreateNSByParam(req *core_common.NsReq) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ns.GetInType()
	ns.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ns.requestNS.InData = string(j)
	result, err := ns.requestNS.CreateNS()
	ns.SetInType(holdType)

	return result, err
}

// ListNS is to Namespace 목록
func (ns *NSApi) ListNS() (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	return ns.requestNS.ListNS()
}

// ListNSId is list Namespace IDs
func (ns *NSApi) ListNSId() (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	return ns.requestNS.ListNSId()
}

// GetNS is to Namespace 조회
func (ns *NSApi) GetNS(doc string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	ns.requestNS.InData = doc
	return ns.requestNS.GetNS()
}

// GetNSByParam is to Namespace 조회
func (ns *NSApi) GetNSByParam(nameSpaceID string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ns.GetInType()
	ns.SetInType("json")
	ns.requestNS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := ns.requestNS.GetNS()
	ns.SetInType(holdType)

	return result, err
}

// DeleteNS is to Namespace 삭제
func (ns *NSApi) DeleteNS(doc string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	ns.requestNS.InData = doc
	return ns.requestNS.DeleteNS()
}

// DeleteNSByParam is to Namespace 삭제
func (ns *NSApi) DeleteNSByParam(nameSpaceID string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ns.GetInType()
	ns.SetInType("json")
	ns.requestNS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := ns.requestNS.DeleteNS()
	ns.SetInType(holdType)

	return result, err
}

// DeleteAllNS is to Namespace 전체 삭제
func (ns *NSApi) DeleteAllNS() (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	return ns.requestNS.DeleteAllNS()
}

// CheckNS is to Namespace 체크
func (ns *NSApi) CheckNS(doc string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	ns.requestNS.InData = doc
	return ns.requestNS.CheckNS()
}

// CheckNSByParam is to Namespace 체크
func (ns *NSApi) CheckNSByParam(nameSpaceID string) (string, error) {
	if ns.requestNS == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := ns.GetInType()
	ns.SetInType("json")
	ns.requestNS.InData = `{"nsId":"` + nameSpaceID + `"}`
	result, err := ns.requestNS.CheckNS()
	ns.SetInType(holdType)

	return result, err
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewNSManager is to NS API 객체 생성
func NewNSManager() (ns *NSApi) {

	ns = &NSApi{}
	ns.gConf = &config.GrpcConfig{}
	ns.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	ns.jaegerCloser = nil
	ns.conn = nil
	ns.clientNS = nil
	ns.requestNS = nil

	ns.inType = "json"
	ns.outType = "json"

	return
}
