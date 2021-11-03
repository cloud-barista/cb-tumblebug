// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package jaegertracer

import (
	"fmt"
	"io"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"

	grpcconfig "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/config"
)

// InitJaeger - Jaeger Tracer 초기화
func InitJaeger(jcConf *grpcconfig.JaegerClientConfig) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "probabilistic",
			Param: jcConf.SampleRate,
		},
		Reporter: &config.ReporterConfig{
			LocalAgentHostPort: jcConf.Endpoint,
		},
	}
	tracer, closer, err := cfg.New(jcConf.ServiceName, config.Logger(jaeger.NullLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}
