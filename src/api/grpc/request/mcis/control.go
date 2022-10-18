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

// CreateMcis
func (r *MCISRequest) CreateMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListMcis
func (r *MCISRequest) ListMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ListMcisId
func (r *MCISRequest) ListMcisId() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListMcisId(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ControlMcis is to Control MCIS's lifecycle
func (r *MCISRequest) ControlMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisActionRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ControlMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ListMcisStatus
func (r *MCISRequest) ListMcisStatus() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListMcisStatus(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetMcisStatus
func (r *MCISRequest) GetMcisStatus() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetMcisStatus(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetMcisInfo
func (r *MCISRequest) GetMcisInfo() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetMcisInfo(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ListMcisVmId
func (r *MCISRequest) ListMcisVmId() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ListMcisVmId(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteMcis
func (r *MCISRequest) DeleteMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// DeleteAllMcis
func (r *MCISRequest) DeleteAllMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbMcisAllQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteAllMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// CreateMcisVM
func (r *MCISRequest) CreateMcisVM() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbVmCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateMcisVM(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// CreateMcisSubGroup
func (r *MCISRequest) CreateMcisSubGroup() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbSubGroupCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CreateMcisSubGroup(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// ControlMcisVM is to Control MCIS VM's lifecycle
func (r *MCISRequest) ControlMcisVM() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbVmActionRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.ControlMcisVM(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetMcisVMStatus
func (r *MCISRequest) GetMcisVMStatus() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbVmQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetMcisVMStatus(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// GetMcisVMInfo
func (r *MCISRequest) GetMcisVMInfo() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbVmQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetMcisVMInfo(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// DeleteMcisVM
func (r *MCISRequest) DeleteMcisVM() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.TbVmQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.DeleteMcisVM(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// RecommendMcis
func (r *MCISRequest) RecommendMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.McisRecommendCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.RecommendMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp.Item)
}

// CmdMcis is to Send a command to execute to MCIS
func (r *MCISRequest) CmdMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.McisCmdCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CmdMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// CmdMcisVm is to Send a command to execute to MCIS VM
func (r *MCISRequest) CmdMcisVm() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.McisCmdVmCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.CmdMcisVm(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// InstallBenchmarkAgentToMcis is to Install CB-Tumblebug benchmarking agent (cb-milkyway), which will be deprecated
func (r *MCISRequest) InstallBenchmarkAgentToMcis() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.McisCmdCreateRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.InstallBenchmarkAgentToMcis(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetBenchmark is to Get benchmark result
func (r *MCISRequest) GetBenchmark() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.BmQryRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetBenchmark(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// GetAllBenchmark is to List benchmark results
func (r *MCISRequest) GetAllBenchmark() (string, error) {
	// Check input data
	if r.InData == "" {
		return "", errors.New("input data required")
	}

	// Unmarshal (json/yaml -> Request Input)
	var item pb.BmQryAllRequest
	err := gc.ConvertToMessage(r.InType, r.InData, &item)
	if err != nil {
		return "", err
	}

	// Request to server
	ctx, cancel := context.WithTimeout(context.Background(), r.Timeout)
	defer cancel()

	resp, err := r.Client.GetAllBenchmark(ctx, &item)
	if err != nil {
		return "", err
	}

	// Marshal (Response -> json/yaml)
	return gc.ConvertToOutput(r.OutType, &resp)
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====
