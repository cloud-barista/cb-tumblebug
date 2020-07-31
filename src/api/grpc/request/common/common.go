package common

import (
	"time"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// UTILITYRequest - Utility 서비스 요청 구현
type UTILITYRequest struct {
	Client  pb.UTILITYClient
	Timeout time.Duration

	InType  string
	InData  string
	OutType string
}

// NSRequest - Namespace 서비스 요청 구현
type NSRequest struct {
	Client  pb.NSClient
	Timeout time.Duration

	InType  string
	InData  string
	OutType string
}

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
