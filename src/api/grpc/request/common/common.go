package common

import (
	"time"

	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// UtilityRequest
type UtilityRequest struct {
	Client  pb.UtilityClient
	Timeout time.Duration

	InType  string
	InData  string
	OutType string
}

// NSRequest
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
