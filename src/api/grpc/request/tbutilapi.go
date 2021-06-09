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

	"google.golang.org/grpc"
)

// ===== [ Coutants and Variables ] =====

// ===== [ Types ] =====

// UTILITYApi - UTILITY API 구조 정의
type UTILITYApi struct {
	gConf          *config.GrpcConfig
	conn           *grpc.ClientConn
	jaegerCloser   io.Closer
	clientUTILITY  pb.UTILITYClient
	requestUTILITY *common.UTILITYRequest
	inType         string
	outType        string
}

// ConfigReq - Config 정보 생성 요청 구조 정의
type ConfigReq struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// ===== [ Implementatiou ] =====

// SetServerAddr - Tumblebug 서버 주소 설정
func (u *UTILITYApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	u.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr - Tumblebug 서버 주소 값 조회
func (u *UTILITYApi) GetServerAddr() (string, error) {
	return u.gConf.GSL.TumblebugCli.ServerAddr, nil
}

// SetTLSCA - TLS CA 설정
func (u *UTILITYApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if u.gConf.GSL.TumblebugCli.TLS == nil {
		u.gConf.GSL.TumblebugCli.TLS = &config.TLSConfig{}
	}

	u.gConf.GSL.TumblebugCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA - TLS CA 값 조회
func (u *UTILITYApi) GetTLSCA() (string, error) {
	if u.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return u.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout - Timeout 설정
func (u *UTILITYApi) SetTimeout(timeout time.Duration) error {
	u.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout - Timeout 값 조회
func (u *UTILITYApi) GetTimeout() (time.Duration, error) {
	return u.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken - JWT 인증 토큰 설정
func (u *UTILITYApi) SetJWTToken(token string) error {
	if token == "" {
		return errors.New("parameter is empty")
	}

	if u.gConf.GSL.TumblebugCli.Interceptors == nil {
		u.gConf.GSL.TumblebugCli.Interceptors = &config.InterceptorsConfig{}
		u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}
	if u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}

	u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken = token
	return nil
}

// GetJWTToken - JWT 인증 토큰 값 조회
func (u *UTILITYApi) GetJWTToken() (string, error) {
	if u.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath - 환경설정 파일 설정
func (u *UTILITYApi) SetConfigPath(configFile string) error {
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

	u.gConf = &gConf
	return nil
}

// Open - 연결 설정
func (u *UTILITYApi) Open() error {

	tumblebugcli := u.gConf.GSL.TumblebugCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(tumblebugcli)
	if err != nil {
		return err
	}

	if closer != nil {
		u.jaegerCloser = closer
	}

	u.conn = cbconn.Conn

	// grpc 클라이언트 생성
	u.clientUTILITY = pb.NewUTILITYClient(u.conn)

	// grpc 호출 Wrapper
	u.requestUTILITY = &common.UTILITYRequest{Client: u.clientUTILITY, Timeout: tumblebugcli.Timeout, InType: u.inType, OutType: u.outType}

	return nil
}

// Close - 연결 종료
func (u *UTILITYApi) Close() {
	if u.conn != nil {
		u.conn.Close()
	}
	if u.jaegerCloser != nil {
		u.jaegerCloser.Close()
	}

	u.jaegerCloser = nil
	u.conn = nil
	u.clientUTILITY = nil
	u.requestUTILITY = nil
}

// SetInType - 입력 문서 타입 설정 (json/yaml)
func (u *UTILITYApi) SetInType(in string) error {
	if in == "json" {
		u.inType = in
	} else if in == "yaml" {
		u.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if u.requestUTILITY != nil {
		u.requestUTILITY.InType = u.inType
	}

	return nil
}

// GetInType - 입력 문서 타입 값 조회
func (u *UTILITYApi) GetInType() (string, error) {
	return u.inType, nil
}

// SetOutType - 출력 문서 타입 설정 (json/yaml)
func (u *UTILITYApi) SetOutType(out string) error {
	if out == "json" {
		u.outType = out
	} else if out == "yaml" {
		u.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if u.requestUTILITY != nil {
		u.requestUTILITY.OutType = u.outType
	}

	return nil
}

// GetOutType - 출력 문서 타입 값 조회
func (u *UTILITYApi) GetOutType() (string, error) {
	return u.outType, nil
}

// ListConnConfig - Connection Config 목록
func (u *UTILITYApi) ListConnConfig() (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUTILITY.ListConnConfig()
}

// GetConnConfig - Connection Config 조회
func (u *UTILITYApi) GetConnConfig(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.GetConnConfig()
}

// GetConnConfigByParam - Connection Config 조회
func (u *UTILITYApi) GetConnConfigByParam(connConfigName string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"connConfigName":"` + connConfigName + `"}`
	result, err := u.requestUTILITY.GetConnConfig()
	u.SetInType(holdType)

	return result, err
}

// ListRegion - Region 목록
func (u *UTILITYApi) ListRegion() (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUTILITY.ListRegion()
}

// GetRegion - Region 조회
func (u *UTILITYApi) GetRegion(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.GetRegion()
}

// GetRegionByParam - Region 조회
func (u *UTILITYApi) GetRegionByParam(regionName string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"regionName":"` + regionName + `"}`
	result, err := u.requestUTILITY.GetRegion()
	u.SetInType(holdType)

	return result, err
}

// CreateConfig - Config 생성
func (u *UTILITYApi) CreateConfig(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.CreateConfig()
}

// CreateConfigByParam - Config 생성
func (u *UTILITYApi) CreateConfigByParam(req *ConfigReq) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	u.requestUTILITY.InData = string(j)
	result, err := u.requestUTILITY.CreateConfig()
	u.SetInType(holdType)

	return result, err
}

// ListConfig - Config 목록
func (u *UTILITYApi) ListConfig() (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUTILITY.ListConfig()
}

// GetConfig - Config 조회
func (u *UTILITYApi) GetConfig(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.GetConfig()
}

// GetConfigByParam - Config 조회
func (u *UTILITYApi) GetConfigByParam(configId string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"configId":"` + configId + `"}`
	result, err := u.requestUTILITY.GetConfig()
	u.SetInType(holdType)

	return result, err
}

// InitConfig - Config 조회
func (u *UTILITYApi) InitConfig(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.InitConfig()
}

// InitConfigByParam - Config 조회
func (u *UTILITYApi) InitConfigByParam(configId string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"configId":"` + configId + `"}`
	result, err := u.requestUTILITY.InitConfig()
	u.SetInType(holdType)

	return result, err
}

// InitAllConfig - Config 전체 삭제
func (u *UTILITYApi) InitAllConfig() (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUTILITY.InitAllConfig()
}

// InspectMcirResources - MCIR 리소스 점검
func (u *UTILITYApi) InspectMcirResources(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.InspectMcirResources()
}

// InspectMcirResourcesByParam - MCIR 리소스 점검
func (u *UTILITYApi) InspectMcirResourcesByParam(connectionName string, mcirType string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"connectionName":"` + connectionName + `", "type":"` + mcirType + `"}`
	result, err := u.requestUTILITY.InspectMcirResources()
	u.SetInType(holdType)

	return result, err
}

// InspectVmResources - VM 리소스 점검
func (u *UTILITYApi) InspectVmResources(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.InspectVmResources()
}

// InspectVmResourcesByParam - VM 리소스 점검
func (u *UTILITYApi) InspectVmResourcesByParam(connectionName string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"connectionName":"` + connectionName + `"}`
	result, err := u.requestUTILITY.InspectVmResources()
	u.SetInType(holdType)

	return result, err
}

// ListObject - 객체 목록
func (u *UTILITYApi) ListObject(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.ListObject()
}

// ListObjectByParam - 객체 목록
func (u *UTILITYApi) ListObjectByParam(key string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"key":"` + key + `"}`
	result, err := u.requestUTILITY.ListObject()
	u.SetInType(holdType)

	return result, err
}

// GetObject - 객체 조회
func (u *UTILITYApi) GetObject(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.GetObject()
}

// GetObjectByParam - 객체 조회
func (u *UTILITYApi) GetObjectByParam(key string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"key":"` + key + `"}`
	result, err := u.requestUTILITY.GetObject()
	u.SetInType(holdType)

	return result, err
}

// DeleteObject - 객체 삭제
func (u *UTILITYApi) DeleteObject(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.DeleteObject()
}

// DeleteObjectByParam - 객체 삭제
func (u *UTILITYApi) DeleteObjectByParam(key string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"key":"` + key + `"}`
	result, err := u.requestUTILITY.DeleteObject()
	u.SetInType(holdType)

	return result, err
}

// DeleteAllObject - 객체 전체 삭제
func (u *UTILITYApi) DeleteAllObject(doc string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUTILITY.InData = doc
	return u.requestUTILITY.DeleteAllObject()
}

// DeleteAllObjectByParam - 객체 전체 삭제
func (u *UTILITYApi) DeleteAllObjectByParam(key string) (string, error) {
	if u.requestUTILITY == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUTILITY.InData = `{"key":"` + key + `"}`
	result, err := u.requestUTILITY.DeleteAllObject()
	u.SetInType(holdType)

	return result, err
}

// ===== [ Private Functiou ] =====

// ===== [ Public Functiou ] =====

// NewUTILITYManager - UTILITY API 객체 생성
func NewUTILITYManager() (u *UTILITYApi) {

	u = &UTILITYApi{}
	u.gConf = &config.GrpcConfig{}
	u.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	u.jaegerCloser = nil
	u.conn = nil
	u.clientUTILITY = nil
	u.requestUTILITY = nil

	u.inType = "json"
	u.outType = "json"

	return
}
