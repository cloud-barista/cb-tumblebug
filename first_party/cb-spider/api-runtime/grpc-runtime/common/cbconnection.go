// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package common

import (
	"errors"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/jaegertracer"
	"github.com/opentracing/opentracing-go"

	grpc_accesslog "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/accesslog"
	grpc_authjwt "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/authjwt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/config"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// CBConnection - CB-GRPC에서 사용하는 grpc 클라이언트를 위한 Wrapper 구조
type CBConnection struct {
	Conn *grpc.ClientConn
}

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCBConnection - 초기화된 grpc 클라이언트의 인스턴스 생성
func NewCBConnection(gConf *config.GrpcClientConfig) (*CBConnection, io.Closer, error) {

	var (
		tracer opentracing.Tracer = nil
		closer io.Closer          = nil
	)

	if gConf == nil {
		return nil, nil, errors.New("grpc connection config is null")
	}

	if gConf.ServerAddr == "" {
		return nil, nil, errors.New("server addr is empty")
	}

	opts := []grpc.DialOption{}

	// TLS 설정
	if gConf.TLS != nil {
		creds, err := credentials.NewClientTLSFromFile(gConf.TLS.TLSCA, "")
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	// 인터셉터 설정
	unaryIntercepters := []grpc.UnaryClientInterceptor{}
	streamIntercepters := []grpc.StreamClientInterceptor{}

	// access 로그 인터셉터 기본 설정
	unaryIntercepters = append(unaryIntercepters, grpc_accesslog.UnaryClientInterceptor())
	streamIntercepters = append(streamIntercepters, grpc_accesslog.StreamClientInterceptor())

	if gConf.Interceptors != nil {

		// AuthJWT 인터셉터 설정
		if gConf.Interceptors.AuthJWT != nil {
			unaryIntercepters = append(unaryIntercepters, grpc_authjwt.UnaryClientInterceptor(gConf.Interceptors.AuthJWT.JWTToken))
			streamIntercepters = append(streamIntercepters, grpc_authjwt.StreamClientInterceptor(gConf.Interceptors.AuthJWT.JWTToken))
		}

		// Opentracing 인터셉터 설정
		if gConf.Interceptors.Opentracing != nil {
			tracer, closer = jaegertracer.InitJaeger(gConf.Interceptors.Opentracing.Jaeger)

			tracingOpts := []grpc_opentracing.Option{}
			tracingOpts = append(tracingOpts, grpc_opentracing.WithTracer(tracer))

			unaryIntercepters = append(unaryIntercepters, grpc_opentracing.UnaryClientInterceptor(tracingOpts...))
			streamIntercepters = append(streamIntercepters, grpc_opentracing.StreamClientInterceptor(tracingOpts...))
		}

	}

	opts = append(opts, grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(unaryIntercepters...)))
	opts = append(opts, grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(streamIntercepters...)))
	conn, err := grpc.Dial(gConf.ServerAddr, opts...)

	return &CBConnection{Conn: conn}, closer, err
}
