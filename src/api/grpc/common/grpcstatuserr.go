package common

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// ConvGrpcStatusErr is to GRPC 상태 코드 에러로 변환
func ConvGrpcStatusErr(err error, tag string, method string) error {
	logger := logger.NewLogger()

	if err != nil {
		if errStatus, ok := status.FromError(err); ok {
			logger.Error(tag, " error while calling ", method, " method: ", errStatus.Message())
			return status.Errorf(errStatus.Code(), "%s error while calling %s method: %v ", tag, method, errStatus.Message())
		}
		logger.Error(tag, " error while calling ", method, " method: ", err)
		return status.Errorf(codes.Internal, "%s error while calling %s method: %v ", tag, method, err)
	}

	return nil
}

// NewGrpcStatusErr is to GRPC 상태 코드 에러 생성
func NewGrpcStatusErr(msg string, tag string, method string) error {
	logger := logger.NewLogger()

	logger.Error(tag, " error while calling ", method, " method: ", msg)
	return status.Errorf(codes.Internal, "%s error while calling %s method: %s ", tag, method, msg)
}
