// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package accesslog

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// UnaryServerInterceptor - rpc unary receive 정보를 기록하는 서버 인터셉터
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Now().Sub(startTime)

		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		logger := logger.NewLogger()
		logger.Info("[", clientIP, "] grpc server unary received : ", info.FullMethod, " service [", elapsed.Nanoseconds()/1000000, " ms]")

		return resp, err
	}
}

// StreamServerInterceptor - rpc stream receive 정보를 기록하는 서버 인터셉터
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		err := handler(srv, stream)
		elapsed := time.Now().Sub(startTime)

		clientIP := "unknown"
		if p, ok := peer.FromContext(stream.Context()); ok {
			clientIP = p.Addr.String()
		}

		logger := logger.NewLogger()
		logger.Info("[", clientIP, "] grpc server stream received : ", info.FullMethod, " service [", elapsed.Nanoseconds()/1000000, " ms]")
		return err
	}
}
