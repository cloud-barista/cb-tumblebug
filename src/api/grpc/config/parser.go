package config

import (
	"fmt"
	"os"
	"reflect"
	"unsafe"

	"github.com/spf13/viper"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// Parser - Viper lib를 활용하기 위한 Parser 정의 구조 형식
type Parser struct {
	viper *viper.Viper
}

// ===== [ Implementations ] =====

// GrpcParse - Viper lib를 이용해서 지정된 CB-GRPC configuration 정보 파싱
func (p Parser) GrpcParse(configFile string) (GrpcConfig, error) {
	p.viper.SetConfigFile(configFile)
	p.viper.AutomaticEnv()
	p.viper.SetConfigType("yaml")

	var cfg GrpcConfig

	// Reading
	if err := p.viper.ReadInConfig(); err != nil {
		return cfg, checkErr(err, configFile)
	}
	// Unmarshal to struct
	if err := p.viper.Unmarshal(&cfg); err != nil {
		return cfg, checkErr(err, configFile)
	}
	// Initialize
	if err := cfg.Init(); err != nil {
		return cfg, CheckErr(err, configFile)
	}
	return cfg, nil
}

// ===== [ Private Functions ] =====

// checkErr - Viper lib 처리에서 발생한 오류 반환 (Nested call)
func checkErr(err error, configFile string) error {
	switch e := err.(type) {
	case viper.ConfigParseError:
		var subErr error
		re := reflect.ValueOf(&e).Elem()
		rf := re.Field(0)
		rse := reflect.ValueOf(&subErr).Elem()
		rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
		rse.Set(rf)
		return checkErr(subErr, configFile)
	default:
		return CheckErr(err, configFile)
	}
}

// ===== [ Public Functions ] =====

// CheckErr - 검증된 오류 정보 반환
func CheckErr(err error, configFile string) error {
	switch e := err.(type) {
	case *os.PathError:
		return fmt.Errorf("'%s' (%s): %s", configFile, e.Op, e.Err.Error())
	default:
		return fmt.Errorf("'%s': %v", configFile, err)
	}
}

// MakeParser - Viber lib를 활용하는 설정 Parser 생성
func MakeParser() Parser {
	return Parser{viper.New()}
}
