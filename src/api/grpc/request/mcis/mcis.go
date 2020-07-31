package mcis

import (
	"time"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// MCISRequest - MCIS 서비스 요청 구현
type MCISRequest struct {
	Client  pb.MCISClient
	Timeout time.Duration

	InType  string
	InData  string
	OutType string
}

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
