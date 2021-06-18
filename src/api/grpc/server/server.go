package server

import (
	"errors"
	"fmt"
	"net"
	"os"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/config"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
	grpc_common "github.com/cloud-barista/cb-tumblebug/src/api/grpc/server/common"
	grpc_mcir "github.com/cloud-barista/cb-tumblebug/src/api/grpc/server/mcir"
	grpc_mcis "github.com/cloud-barista/cb-tumblebug/src/api/grpc/server/mcis"

	"google.golang.org/grpc/reflection"
)

// RunServer - Run gRPC server
func RunServer() {
	logger := logger.NewLogger()

	configPath := os.Getenv("CBTUMBLEBUG_ROOT") + "/conf/grpc_conf.yaml"
	gConf, err := configLoad(configPath)
	if err != nil {
		logger.Error("failed to load config : ", err)
		return
	}

	tumblebugsrv := gConf.GSL.TumblebugSrv

	conn, err := net.Listen("tcp", tumblebugsrv.Addr)
	if err != nil {
		logger.Error("failed to listen: ", err)
		return
	}

	cbserver, closer, err := gc.NewCBServer(tumblebugsrv)
	if err != nil {
		logger.Error("failed to create grpc server: ", err)
		return
	}

	if closer != nil {
		defer closer.Close()
	}

	gs := cbserver.Server
	pb.RegisterUtilityServer(gs, &grpc_common.UtilityService{})
	pb.RegisterNSServer(gs, &grpc_common.NSService{})
	pb.RegisterMCIRServer(gs, &grpc_mcir.MCIRService{})
	pb.RegisterMCISServer(gs, &grpc_mcis.MCISService{})

	if tumblebugsrv.Reflection == "enable" {
		if tumblebugsrv.Interceptors != nil && tumblebugsrv.Interceptors.AuthJWT != nil {
			fmt.Printf("\n\n*** you can run reflection when jwt auth interceptor is not used ***\n\n")
		} else {
			reflection.Register(gs)
		}
	}

	//fmt.Printf("\n[CB-Tumblebug: Multi-Cloud Infra Service Management]")
	//fmt.Printf("\n   Initiating GRPC API Server....__^..^__....")
	fmt.Printf("⇨ grpc server started on [::]%s\n", tumblebugsrv.Addr)

	if err := gs.Serve(conn); err != nil {
		logger.Error("failed to serve: ", err)
	}
}

func configLoad(cf string) (config.GrpcConfig, error) {
	logger := logger.NewLogger()

	// Make new config parser that uses Viper library
	parser := config.MakeParser()

	var (
		gConf config.GrpcConfig
		err   error
	)

	if cf == "" {
		logger.Error("Please, provide the path to your configuration file")
		return gConf, errors.New("configuration file are not specified")
	}

	logger.Debug("Parsing configuration file: ", cf)
	if gConf, err = parser.GrpcParse(cf); err != nil {
		logger.Error("ERROR - Parsing the configuration file.\n", err.Error())
		return gConf, err
	}

	// Apply command line params (which have higher priority)

	// Check if mandatory CB-Tumblebug params are set
	tumblebugsrv := gConf.GSL.TumblebugSrv

	if tumblebugsrv == nil {
		return gConf, errors.New("tumblebugsrv field are not specified")
	}

	if tumblebugsrv.Addr == "" {
		return gConf, errors.New("tumblebugsrv.addr field are not specified")
	}

	if tumblebugsrv.TLS != nil {
		if tumblebugsrv.TLS.TLSCert == "" {
			return gConf, errors.New("tumblebugsrv.tls.tls_cert field are not specified")
		}
		if tumblebugsrv.TLS.TLSKey == "" {
			return gConf, errors.New("tumblebugsrv.tls.tls_key field are not specified")
		}
	}

	if tumblebugsrv.Interceptors != nil {
		if tumblebugsrv.Interceptors.AuthJWT != nil {
			if tumblebugsrv.Interceptors.AuthJWT.JWTKey == "" {
				return gConf, errors.New("tumblebugsrv.interceptors.auth_jwt.jwt_key field are not specified")
			}
		}
		if tumblebugsrv.Interceptors.PrometheusMetrics != nil {
			if tumblebugsrv.Interceptors.PrometheusMetrics.ListenPort == 0 {
				return gConf, errors.New("tumblebugsrv.interceptors.prometheus_metrics.listen_port field are not specified")
			}
		}
		if tumblebugsrv.Interceptors.Opentracing != nil {
			if tumblebugsrv.Interceptors.Opentracing.Jaeger != nil {
				if tumblebugsrv.Interceptors.Opentracing.Jaeger.Endpoint == "" {
					return gConf, errors.New("tumblebugsrv.interceptors.opentracing.jaeger.endpoint field are not specified")
				}
			}
		}
	}

	return gConf, nil
}
