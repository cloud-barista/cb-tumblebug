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

// ===== [ Coutants and Variables ] =====

// ===== [ Types ] =====

// UtilityApi is for Utility API 구조 정의
type UtilityApi struct {
	gConf          *config.GrpcConfig
	conn           *grpc.ClientConn
	jaegerCloser   io.Closer
	clientUtility  pb.UtilityClient
	requestUtility *common.UtilityRequest
	inType         string
	outType        string
}

// ConfigReq is for Config 정보 생성 요청 구조 정의
// type ConfigReq struct {
// 	Name  string `yaml:"name" json:"name"`
// 	Value string `yaml:"value" json:"value"`
// }

// ===== [ Implementatiou ] =====

// SetServerAddr is to Tumblebug 서버 주소 설정
func (u *UtilityApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	u.gConf.GSL.TumblebugCli.ServerAddr = addr
	return nil
}

// GetServerAddr is to Tumblebug 서버 주소 값 조회
func (u *UtilityApi) GetServerAddr() (string, error) {
	return u.gConf.GSL.TumblebugCli.ServerAddr, nil
}

// SetTLSCA is to TLS CA 설정
func (u *UtilityApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if u.gConf.GSL.TumblebugCli.TLS == nil {
		u.gConf.GSL.TumblebugCli.TLS = &config.TLSConfig{}
	}

	u.gConf.GSL.TumblebugCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA is to TLS CA 값 조회
func (u *UtilityApi) GetTLSCA() (string, error) {
	if u.gConf.GSL.TumblebugCli.TLS == nil {
		return "", nil
	}

	return u.gConf.GSL.TumblebugCli.TLS.TLSCA, nil
}

// SetTimeout is to Timeout 설정
func (u *UtilityApi) SetTimeout(timeout time.Duration) error {
	u.gConf.GSL.TumblebugCli.Timeout = timeout
	return nil
}

// GetTimeout is to Timeout 값 조회
func (u *UtilityApi) GetTimeout() (time.Duration, error) {
	return u.gConf.GSL.TumblebugCli.Timeout, nil
}

// SetJWTToken is to JWT 인증 토큰 설정
func (u *UtilityApi) SetJWTToken(token string) error {
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

// GetJWTToken is to JWT 인증 토큰 값 조회
func (u *UtilityApi) GetJWTToken() (string, error) {
	if u.gConf.GSL.TumblebugCli.Interceptors == nil {
		return "", nil
	}
	if u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return u.gConf.GSL.TumblebugCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath is to 환경설정 파일 설정
func (u *UtilityApi) SetConfigPath(configFile string) error {
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

	u.gConf = &gConf
	return nil
}

// Open is to 연결 설정
func (u *UtilityApi) Open() error {

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
	u.clientUtility = pb.NewUtilityClient(u.conn)

	// grpc 호출 Wrapper
	u.requestUtility = &common.UtilityRequest{Client: u.clientUtility, Timeout: tumblebugcli.Timeout, InType: u.inType, OutType: u.outType}

	return nil
}

// Close is to 연결 종료
func (u *UtilityApi) Close() {
	if u.conn != nil {
		u.conn.Close()
	}
	if u.jaegerCloser != nil {
		u.jaegerCloser.Close()
	}

	u.jaegerCloser = nil
	u.conn = nil
	u.clientUtility = nil
	u.requestUtility = nil
}

// SetInType is to 입력 문서 타입 설정 (json/yaml)
func (u *UtilityApi) SetInType(in string) error {
	if in == "json" {
		u.inType = in
	} else if in == "yaml" {
		u.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if u.requestUtility != nil {
		u.requestUtility.InType = u.inType
	}

	return nil
}

// GetInType is to 입력 문서 타입 값 조회
func (u *UtilityApi) GetInType() (string, error) {
	return u.inType, nil
}

// SetOutType is to 출력 문서 타입 설정 (json/yaml)
func (u *UtilityApi) SetOutType(out string) error {
	if out == "json" {
		u.outType = out
	} else if out == "yaml" {
		u.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if u.requestUtility != nil {
		u.requestUtility.OutType = u.outType
	}

	return nil
}

// GetOutType is to 출력 문서 타입 값 조회
func (u *UtilityApi) GetOutType() (string, error) {
	return u.outType, nil
}

// ListConnConfig is to Connection Config 목록
func (u *UtilityApi) ListConnConfig() (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUtility.ListConnConfig()
}

// GetConnConfig is to Connection Config 조회
func (u *UtilityApi) GetConnConfig(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.GetConnConfig()
}

// GetConnConfigByParam is to Connection Config 조회
func (u *UtilityApi) GetConnConfigByParam(connConfigName string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"connConfigName":"` + connConfigName + `"}`
	result, err := u.requestUtility.GetConnConfig()
	u.SetInType(holdType)

	return result, err
}

// ListRegion is to Region 목록
func (u *UtilityApi) ListRegion() (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUtility.ListRegion()
}

// GetRegion is to Region 조회
func (u *UtilityApi) GetRegion(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.GetRegion()
}

// GetRegionByParam is to Region 조회
func (u *UtilityApi) GetRegionByParam(regionName string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"regionName":"` + regionName + `"}`
	result, err := u.requestUtility.GetRegion()
	u.SetInType(holdType)

	return result, err
}

// CreateConfig is to Config 생성
func (u *UtilityApi) CreateConfig(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.CreateConfig()
}

// CreateConfigByParam is to Config 생성
func (u *UtilityApi) CreateConfigByParam(req *core_common.ConfigReq) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	j, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	u.requestUtility.InData = string(j)
	result, err := u.requestUtility.CreateConfig()
	u.SetInType(holdType)

	return result, err
}

// ListConfig is to Config 목록
func (u *UtilityApi) ListConfig() (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUtility.ListConfig()
}

// GetConfig is to Config 조회
func (u *UtilityApi) GetConfig(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.GetConfig()
}

// GetConfigByParam is to Config 조회
func (u *UtilityApi) GetConfigByParam(configId string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"configId":"` + configId + `"}`
	result, err := u.requestUtility.GetConfig()
	u.SetInType(holdType)

	return result, err
}

// InitConfig is to Config 조회
func (u *UtilityApi) InitConfig(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.InitConfig()
}

// InitConfigByParam is to Config 조회
func (u *UtilityApi) InitConfigByParam(configId string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"configId":"` + configId + `"}`
	result, err := u.requestUtility.InitConfig()
	u.SetInType(holdType)

	return result, err
}

// InitAllConfig is to Config 전체 삭제
func (u *UtilityApi) InitAllConfig() (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	return u.requestUtility.InitAllConfig()
}

// InspectMcirResources is to MCIR 리소스 점검
func (u *UtilityApi) InspectMcirResources(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.InspectMcirResources()
}

// InspectMcirResourcesByParam is to MCIR 리소스 점검
func (u *UtilityApi) InspectMcirResourcesByParam(connectionName string, mcirType string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"connectionName":"` + connectionName + `", "type":"` + mcirType + `"}`
	result, err := u.requestUtility.InspectMcirResources()
	u.SetInType(holdType)

	return result, err
}

// InspectVmResources is to VM 리소스 점검
func (u *UtilityApi) InspectVmResources(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.InspectVmResources()
}

// InspectVmResourcesByParam is to VM 리소스 점검
func (u *UtilityApi) InspectVmResourcesByParam(connectionName string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"connectionName":"` + connectionName + `"}`
	result, err := u.requestUtility.InspectVmResources()
	u.SetInType(holdType)

	return result, err
}

// ListObject is to 객체 목록
func (u *UtilityApi) ListObject(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.ListObject()
}

// ListObjectByParam is to 객체 목록
func (u *UtilityApi) ListObjectByParam(key string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"key":"` + key + `"}`
	result, err := u.requestUtility.ListObject()
	u.SetInType(holdType)

	return result, err
}

// GetObject is to 객체 조회
func (u *UtilityApi) GetObject(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.GetObject()
}

// GetObjectByParam is to 객체 조회
func (u *UtilityApi) GetObjectByParam(key string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"key":"` + key + `"}`
	result, err := u.requestUtility.GetObject()
	u.SetInType(holdType)

	return result, err
}

// DeleteObject is to 객체 삭제
func (u *UtilityApi) DeleteObject(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.DeleteObject()
}

// DeleteObjectByParam is to 객체 삭제
func (u *UtilityApi) DeleteObjectByParam(key string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"key":"` + key + `"}`
	result, err := u.requestUtility.DeleteObject()
	u.SetInType(holdType)

	return result, err
}

// DeleteAllObject is to 객체 전체 삭제
func (u *UtilityApi) DeleteAllObject(doc string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	u.requestUtility.InData = doc
	return u.requestUtility.DeleteAllObject()
}

// DeleteAllObjectByParam is to 객체 전체 삭제
func (u *UtilityApi) DeleteAllObjectByParam(key string) (string, error) {
	if u.requestUtility == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := u.GetInType()
	u.SetInType("json")
	u.requestUtility.InData = `{"key":"` + key + `"}`
	result, err := u.requestUtility.DeleteAllObject()
	u.SetInType(holdType)

	return result, err
}

// ===== [ Private Functiou ] =====

// ===== [ Public Functiou ] =====

// NewUtilityManager is to Utility API 객체 생성
func NewUtilityManager() (u *UtilityApi) {

	u = &UtilityApi{}
	u.gConf = &config.GrpcConfig{}
	u.gConf.GSL.TumblebugCli = &config.GrpcClientConfig{}

	u.jaegerCloser = nil
	u.conn = nil
	u.clientUtility = nil
	u.requestUtility = nil

	u.inType = "json"
	u.outType = "json"

	return
}
