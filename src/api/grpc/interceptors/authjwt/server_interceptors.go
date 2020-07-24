package authjwt

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ===== [ Constants and Variables ] =====
var (
	jwtKey = ""
)

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// UnaryServerInterceptor - authentication 을 처리하는 Unary 서버 인터셉터
func UnaryServerInterceptor(key string) grpc.UnaryServerInterceptor {
	jwtKey = key
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		if jwtKey == "" {
			return nil, status.Errorf(codes.Unauthenticated, "jwt key is not supplied")
		}

		_, err := validateToken(ctx)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamServerInterceptor - authentication 을 처리하는 Stream 서버 인터셉터
func StreamServerInterceptor(key string) grpc.StreamServerInterceptor {
	jwtKey = key
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

		if jwtKey == "" {
			return status.Errorf(codes.Unauthenticated, "jwt key is not supplied")
		}

		_, err := validateToken(stream.Context())
		if err != nil {
			return err
		}

		return handler(srv, stream)
	}
}
