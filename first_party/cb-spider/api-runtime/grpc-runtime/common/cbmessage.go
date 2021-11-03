// gRPC Runtime of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// by CB-Spider Team, 2020.09.

package common

import (
	"encoding/json"
	"strings"

	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"

	"gopkg.in/yaml.v2"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====s

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// ConvertToMessage - 입력 데이터를 grpc 메시지로 변환
func ConvertToMessage(inType string, inData string, obj interface{}) error {
	logger := logger.NewLogger()

	if inType == "yaml" {
		err := yaml.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		logger.Debug("yaml Unmarshal: \n", obj)
	}

	if inType == "json" {
		err := json.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		logger.Debug("json Unmarshal: \n", obj)
	}

	return nil
}

// ConvertToOutput - grpc 메시지를 출력포맷으로 변환
func ConvertToOutput(outType string, obj interface{}) (string, error) {
	logger := logger.NewLogger()

	if outType == "yaml" {
		// 메시지 포맷에서 불필요한 필드(XXX_로 시작하는 필드)를 제거하기 위해 json 태그를 이용하여 마샬링
		j, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}

		// yaml 에서 지원하지 않는 control character 제거
		cleanStr := strings.Map(func(value rune) rune {
			switch {
			case value == 0x09:
				return value
			case value == 0x0A:
				return value
			case value == 0x0D:
				return value
			case value >= 0x20 && value <= 0x7E:
				return value
			case value == 0x85:
				return value
			case value >= 0xA0 && value <= 0xD7FF:
				return value
			case value >= 0xE000 && value <= 0xFFFD:
				return value
			case value >= 0x10000 && value <= 0x10FFFF:
				return value
			default:
				return -1 // control characters are not allowed
			}
		}, string(j))

		// 필드를 소팅하지 않고 지정된 순서대로 출력하기 위해 MapSlice 이용
		jsonObj := yaml.MapSlice{}
		err2 := yaml.Unmarshal([]byte(cleanStr), &jsonObj)
		if err2 != nil {
			return "", err2
		}

		// yaml 마샬링
		y, err3 := yaml.Marshal(jsonObj)
		if err3 != nil {
			return "", err3
		}
		logger.Debug("yaml Marshal: \n", string(y))

		return string(y), nil
	}

	if outType == "json" {
		j, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		outStr := string(j)

		// json.Marshal 함수는  <,>, & 문자를 escape 함.. 다시 원래대로 변환
		outStr = strings.Replace(outStr, "\\u003c", "<", -1)
		outStr = strings.Replace(outStr, "\\u003e", ">", -1)
		outStr = strings.Replace(outStr, "\\u0026", "&", -1)

		logger.Debug("json Marshal: \n", outStr)
		return outStr, nil
	}

	return "", nil
}

// CopySrcToDest - 소스에서 타켓으로 데이터 복사
func CopySrcToDest(src interface{}, dest interface{}) error {
	logger := logger.NewLogger()

	j, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	logger.Debug("source value : \n", string(j))

	err = json.Unmarshal(j, dest)
	if err != nil {
		return err
	}

	j, err = json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return err
	}
	logger.Debug("target value : \n", string(j))

	return nil
}
