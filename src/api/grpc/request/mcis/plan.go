package mcis

import (
	"context"
	"errors"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	pb "github.com/cloud-barista/cb-tumblebug/src/api/grpc/protobuf/cbtumblebug"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// RecommendVM
func (r *MCISRequest) RecommendVM() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.McisRecommendVmCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.RecommendVM(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Items)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
