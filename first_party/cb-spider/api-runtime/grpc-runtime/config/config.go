// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

// Package config - Configuration for Cloud-Barista's GRPC and provides the required process
package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// ===== [ Constants and Variables ] =====

const (
	// ConfigVersion - 설정 구조에 대한 버전
	ConfigVersion = 1
)

// ===== [ Types ] =====

// GrpcConfig - CB-GRPC 서비스 설정 구조
type GrpcConfig struct {
	Version int             `mapstructure:"version"`
	GSL     GrpcServiceList `mapstructure:"grpc"`
}

// GrpcServiceList - CB-GRPC 서비스 목록
type GrpcServiceList struct {
	SpiderSrv *GrpcServerConfig `mapstructure:"spidersrv"`
	SpiderCli *GrpcClientConfig `mapstructure:"spidercli"`
}

// GrpcServerConfig - CB-GRPC 서버 설정 구조
type GrpcServerConfig struct {
	Addr         string              `mapstructure:"addr"`
	Reflection   string              `mapstructure:"reflection"`
	TLS          *TLSConfig          `mapstructure:"tls"`
	Interceptors *InterceptorsConfig `mapstructure:"interceptors"`
}

// GrpcClientConfig - CB-GRPC 클라이언트 설정 구조
type GrpcClientConfig struct {
	ServerAddr   string              `mapstructure:"server_addr"`
	Timeout      time.Duration       `mapstructure:"timeout"`
	TLS          *TLSConfig          `mapstructure:"tls"`
	Interceptors *InterceptorsConfig `mapstructure:"interceptors"`
}

// TLSConfig - TLS 설정 구조
type TLSConfig struct {
	TLSCert string `mapstructure:"tls_cert"`
	TLSKey  string `mapstructure:"tls_key"`
	TLSCA   string `mapstructure:"tls_ca"`
}

// InterceptorsConfig - GRPC 인터셉터 설정 구조
type InterceptorsConfig struct {
	AuthJWT           *AuthJWTConfig           `mapstructure:"auth_jwt"`
	PrometheusMetrics *PrometheusMetricsConfig `mapstructure:"prometheus_metrics"`
	Opentracing       *OpentracingConfig       `mapstructure:"opentracing"`
}

// AuthJWTConfig - AuthJWT 설정 구조
type AuthJWTConfig struct {
	JWTKey   string `mapstructure:"jwt_key"`
	JWTToken string `mapstructure:"jwt_token"`
}

// PrometheusMetricsConfig - Prometheus Metrics 설정 구조
type PrometheusMetricsConfig struct {
	ListenPort int `mapstructure:"listen_port"`
}

// OpentracingConfig - Opentracing 설정 구조
type OpentracingConfig struct {
	Jaeger *JaegerClientConfig `mapstructure:"jaeger"`
}

// JaegerClientConfig - Jaeger Client 설정 구조
type JaegerClientConfig struct {
	Endpoint    string  `mapstructure:"endpoint"`
	ServiceName string  `mapstructure:"service_name"`
	SampleRate  float64 `mapstructure:"sample_rate"`
}

// UnsupportedVersionError - 설정 초기화 과정에서 버전 검증을 통해 반환할 오류 구조
type UnsupportedVersionError struct {
	Have int
	Want int
}

// ===== [ Implementations ] =====

// Init - 설정에 대한 검사 및 초기화
func (gConf *GrpcConfig) Init() error {
	// 설정 파일 버전 검증
	if gConf.Version != ConfigVersion {
		return &UnsupportedVersionError{
			Have: gConf.Version,
			Want: ConfigVersion,
		}
	}
	// 전역변수 초기화
	gConf.initGlobalParams()

	return nil
}

// initGlobalParams - 전역 설정 초기화
func (gConf *GrpcConfig) initGlobalParams() {

	if gConf.GSL.SpiderSrv != nil {

		if gConf.GSL.SpiderSrv.TLS != nil {
			if gConf.GSL.SpiderSrv.TLS.TLSCert != "" {
				gConf.GSL.SpiderSrv.TLS.TLSCert = ReplaceEnvPath(gConf.GSL.SpiderSrv.TLS.TLSCert)
			}
			if gConf.GSL.SpiderSrv.TLS.TLSKey != "" {
				gConf.GSL.SpiderSrv.TLS.TLSKey = ReplaceEnvPath(gConf.GSL.SpiderSrv.TLS.TLSKey)
			}
		}

		if gConf.GSL.SpiderSrv.Interceptors != nil {
			if gConf.GSL.SpiderSrv.Interceptors.Opentracing != nil {
				if gConf.GSL.SpiderSrv.Interceptors.Opentracing.Jaeger != nil {

					if gConf.GSL.SpiderSrv.Interceptors.Opentracing.Jaeger.ServiceName == "" {
						gConf.GSL.SpiderSrv.Interceptors.Opentracing.Jaeger.ServiceName = "grpc spider server"
					}

					if gConf.GSL.SpiderSrv.Interceptors.Opentracing.Jaeger.SampleRate == 0 {
						gConf.GSL.SpiderSrv.Interceptors.Opentracing.Jaeger.SampleRate = 1
					}

				}
			}
		}
	}

	if gConf.GSL.SpiderCli != nil {

		if gConf.GSL.SpiderCli.Timeout == 0 {
			gConf.GSL.SpiderCli.Timeout = 90 * time.Second
		}

		if gConf.GSL.SpiderCli.TLS != nil {
			if gConf.GSL.SpiderCli.TLS.TLSCA != "" {
				gConf.GSL.SpiderCli.TLS.TLSCA = ReplaceEnvPath(gConf.GSL.SpiderCli.TLS.TLSCA)
			}
		}

		if gConf.GSL.SpiderCli.Interceptors != nil {
			if gConf.GSL.SpiderCli.Interceptors.Opentracing != nil {
				if gConf.GSL.SpiderCli.Interceptors.Opentracing.Jaeger != nil {

					if gConf.GSL.SpiderCli.Interceptors.Opentracing.Jaeger.ServiceName == "" {
						gConf.GSL.SpiderCli.Interceptors.Opentracing.Jaeger.ServiceName = "grpc spider client"
					}

					if gConf.GSL.SpiderCli.Interceptors.Opentracing.Jaeger.SampleRate == 0 {
						gConf.GSL.SpiderCli.Interceptors.Opentracing.Jaeger.SampleRate = 1
					}

				}
			}
		}
	}

}

// Error - 비 호환 버전에 대한 오류 문자열 반환
func (u *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("Unsupported version: %d (wanted: %d)", u.Have, u.Want)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// ReplaceEnvPath - $ABC/def ==> /abc/def
func ReplaceEnvPath(str string) string {
	if strings.Index(str, "$") == -1 {
		return str
	}

	// ex) input "$CBSTORE_ROOT/meta_db/dat"
	strList := strings.Split(str, "/")
	for n, one := range strList {
		if strings.Index(one, "$") != -1 {
			cbstoreRootPath := os.Getenv(strings.Trim(one, "$"))
			if cbstoreRootPath == "" {
				log.Fatal(one + " is not set!")
			}
			strList[n] = cbstoreRootPath
		}
	}

	var resultStr string
	for _, one := range strList {
		resultStr = resultStr + one + "/"
	}
	// ex) "/root/go/src/github.com/cloud-barista/cb-spider/meta_db/dat/"
	resultStr = strings.TrimRight(resultStr, "/")
	resultStr = strings.ReplaceAll(resultStr, "//", "/")
	return resultStr
}
