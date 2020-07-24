package jaegertracer

import (
	"fmt"
	"io"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"

	grpcconfig "github.com/cloud-barista/cb-tumblebug/src/api/grpc/config"
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
