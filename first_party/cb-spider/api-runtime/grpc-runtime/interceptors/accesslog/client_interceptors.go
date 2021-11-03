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

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// UnaryClientInterceptor - rpc unary call 정보를 기록하는 클라이언트 인터셉터
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		elapsed := time.Now().Sub(startTime)

		logger := logger.NewLogger()
		logger.Info("grpc client unary call : ", method, " service [", elapsed.Nanoseconds()/1000000, " ms]")

		return err
	}
}

// StreamClientInterceptor - rpc stream call 정보를 기록하는 클라이언트 인터셉터
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		startTime := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		elapsed := time.Now().Sub(startTime)

		logger := logger.NewLogger()
		logger.Info("grpc client stream call : ", method, " service [", elapsed.Nanoseconds()/1000000, " ms]")

		return clientStream, err
	}
}
