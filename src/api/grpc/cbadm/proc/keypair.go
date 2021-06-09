package proc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	tb_api "github.com/cloud-barista/cb-tumblebug/src/api/grpc/request"
)

// ===== [ Constants and Variables ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// SaveSshKey : Write keypair to file
func SaveSshKey(mcir *tb_api.MCIRApi, nameSpaceID string, resourceID string, sshSaveFileName string) (string, error) {

	holdType, _ := mcir.GetOutType()
	mcir.SetOutType("json")

	result, err := mcir.GetSshKeyByParam(nameSpaceID, resourceID)
	mcir.SetOutType(holdType)
	if err != nil {
		return "", err
	}

	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(result), &jsonMap)
	if err != nil {
		return "", err
	}

	privateKey := fmt.Sprintf("%v", jsonMap["privateKey"])
	err = ioutil.WriteFile(sshSaveFileName, []byte(privateKey), 0644)
	if err != nil {
		return "", err
	}

	return "ssh key file saved", nil
}
