package proc

import (
	"encoding/json"
	"fmt"

	gc "github.com/cloud-barista/cb-tumblebug/src/api/grpc/common"
	tb_api "github.com/cloud-barista/cb-tumblebug/src/api/grpc/request"
)

// ===== [ Constants and Variables ] =====

// VMListInfo
type VMListInfo struct {
	Id   string   `yaml:"id" json:"id"`
	Name string   `yaml:"name" json:"name"`
	Vm   []string `yaml:"vm" json:"vm"`
}

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// ListMcisVM
func ListMcisVM(mcis *tb_api.MCISApi, nameSpaceID string, mcisID string) (string, error) {

	holdType, _ := mcis.GetOutType()
	mcis.SetOutType("json")
	defer mcis.SetOutType(holdType)

	result, err := mcis.GetMcisInfoByParam(nameSpaceID, mcisID)
	if err != nil {
		return "", err
	}

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(result), &jsonMap)
	if err != nil {
		return "", err
	}

	vmList := []string{}
	for _, m := range jsonMap["vm"].([]interface{}) {
		item := m.(map[string]interface{})
		vmList = append(vmList, fmt.Sprintf("%v", item["id"]))
	}

	vmListInfo := VMListInfo{}
	vmListInfo.Id = fmt.Sprintf("%v", jsonMap["id"])
	vmListInfo.Name = fmt.Sprintf("%v", jsonMap["name"])
	vmListInfo.Vm = vmList

	outType, _ := mcis.GetOutType()
	return gc.ConvertToOutput(outType, &vmListInfo)
}
