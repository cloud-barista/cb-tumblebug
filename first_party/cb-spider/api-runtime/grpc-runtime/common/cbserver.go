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
	"fmt"
	"io"
	"log"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/jaegertracer"

	grpc_accesslog "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/accesslog"
	grpc_authjwt "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/interceptors/authjwt"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/config"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// CBServer - CB-GRPC에서 사용하는 grpc 서버를 위한 Wrapper 구조
type CBServer struct {
	Server *grpc.Server
}

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCBServer - 초기화된 grpc 서버의 인스턴스 생성
func NewCBServer(gConf *config.GrpcServerConfig) (*CBServer, io.Closer, error) {

	var (
		tracer      opentracing.Tracer             = nil
		closer      io.Closer                      = nil
		reg         *prometheus.Registry           = nil
		grpcMetrics *grpc_prometheus.ServerMetrics = nil
	)

	if gConf == nil {
		return nil, nil, errors.New("grpc server config is null")
	}

	opts := []grpc.ServerOption{}

	// TLS 설정
	if gConf.TLS != nil {
		creds, err := credentials.NewServerTLSFromFile(gConf.TLS.TLSCert, gConf.TLS.TLSKey)
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// 인터셉터 설정
	unaryIntercepters := []grpc.UnaryServerInterceptor{}
	streamIntercepters := []grpc.StreamServerInterceptor{}

	// access 로그 인터셉터 기본 설정
	unaryIntercepters = append(unaryIntercepters, grpc_accesslog.UnaryServerInterceptor())
	streamIntercepters = append(streamIntercepters, grpc_accesslog.StreamServerInterceptor())

	if gConf.Interceptors != nil {

		// AuthJWT 인터셉터 설정
		if gConf.Interceptors.AuthJWT != nil {
			unaryIntercepters = append(unaryIntercepters, grpc_authjwt.UnaryServerInterceptor(gConf.Interceptors.AuthJWT.JWTKey))
			streamIntercepters = append(streamIntercepters, grpc_authjwt.StreamServerInterceptor(gConf.Interceptors.AuthJWT.JWTKey))
		}

		// Opentracing 인터셉터 설정
		if gConf.Interceptors.Opentracing != nil {
			if gConf.Interceptors.Opentracing.Jaeger != nil {
				tracer, closer = jaegertracer.InitJaeger(gConf.Interceptors.Opentracing.Jaeger)

				tracingOpts := []grpc_opentracing.Option{}
				tracingOpts = append(tracingOpts, grpc_opentracing.WithTracer(tracer))

				unaryIntercepters = append(unaryIntercepters, grpc_opentracing.UnaryServerInterceptor(tracingOpts...))
				streamIntercepters = append(streamIntercepters, grpc_opentracing.StreamServerInterceptor(tracingOpts...))
			}
		}

		// Prometheus Metrics 인터셉터 설정
		if gConf.Interceptors.PrometheusMetrics != nil {
			grpcMetrics = grpc_prometheus.NewServerMetrics()
			grpcMetrics.EnableHandlingTimeHistogram()

			reg = prometheus.NewRegistry()
			reg.MustRegister(grpcMetrics)
			reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
			reg.MustRegister(prometheus.NewGoCollector())

			unaryIntercepters = append(unaryIntercepters, grpcMetrics.UnaryServerInterceptor())
			streamIntercepters = append(streamIntercepters, grpcMetrics.StreamServerInterceptor())
		}

	}

	// recovery 인터셉터 기본 설정
	unaryIntercepters = append(unaryIntercepters, grpc_recovery.UnaryServerInterceptor())
	streamIntercepters = append(streamIntercepters, grpc_recovery.StreamServerInterceptor())

	opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryIntercepters...)))
	opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamIntercepters...)))
	gs := grpc.NewServer(opts...)

	if gConf.Interceptors != nil {
		if gConf.Interceptors.PrometheusMetrics != nil {

			// Create a HTTP server for prometheus.
			httpServer := &http.Server{
				Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
				Addr:    fmt.Sprintf("0.0.0.0:%d", gConf.Interceptors.PrometheusMetrics.ListenPort),
			}
			// Initialize all metrics.
			grpcMetrics.InitializeMetrics(gs)
			// Start your http server for prometheus.
			go func() {
				if err := httpServer.ListenAndServe(); err != nil {
					log.Fatal("Unable to start a http server for prometheus.")
				}
			}()

		}
	}

	return &CBServer{Server: gs}, closer, nil
}
