package authjwt

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ===== [ Constants and Variables ] =====
var (
	jwtToken = ""
)

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// UnaryClientInterceptor - JWT 토큰을 전달하는 Unary 클라이언트 인터셉터
func UnaryClientInterceptor(token string) grpc.UnaryClientInterceptor {
	jwtToken = token
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		if jwtToken == "" {
			return status.Errorf(codes.Unauthenticated, "jwt token is not supplied")
		}

		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", jwtToken)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor - JWT 토큰을 전달하는 Stream 클라이언트 인터셉터
func StreamClientInterceptor(token string) grpc.StreamClientInterceptor {
	jwtToken = token
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		if jwtToken == "" {
			return nil, status.Errorf(codes.Unauthenticated, "jwt token is not supplied")
		}

		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", jwtToken)

		return streamer(ctx, desc, cc, method, opts...)
	}
}
