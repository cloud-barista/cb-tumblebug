/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	crand "crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func GetAssetsFilePath(filename string) string {
	var attemptedPaths []string

	// Try TB_ROOT_PATH environment variable first
	if rootPath := os.Getenv("TB_ROOT_PATH"); rootPath != "" {
		path := filepath.Join(rootPath, "assets", filename)
		attemptedPaths = append(attemptedPaths, path)
		if _, err := os.Stat(path); err == nil {
			log.Debug().
				Str("filename", filename).
				Str("resolved_path", path).
				Msg("Assets file resolved via TB_ROOT_PATH")
			return path
		}
	}

	// Try multiple standard paths
	possiblePaths := []string{
		filepath.Join(".", "assets", filename),
		filepath.Join("../assets/", filename),
	}

	for _, path := range possiblePaths {
		attemptedPaths = append(attemptedPaths, path)
		if _, err := os.Stat(path); err == nil {
			log.Debug().
				Str("filename", filename).
				Str("resolved_path", path).
				Msg("Assets file resolved")
			return path
		}
	}

	// Default fallback to ../assets/ (most common case)
	defaultPath := filepath.Join("../assets/", filename)
	log.Warn().
		Str("filename", filename).
		Str("default_path", defaultPath).
		Strs("attempted_paths", attemptedPaths).
		Msg("Assets file not found, using default path")

	return defaultPath
}

// SetupViperPaths configures standard asset paths for a Viper instance.
// This centralizes the path configuration logic used across multiple config files.
// It adds paths in priority order:
// 1. TB_ROOT_PATH/assets (if TB_ROOT_PATH is set)
// 2. TB_ROOT_PATH (if TB_ROOT_PATH is set)
// 3. . (current directory)
// 4. ./assets/
// 5. ../assets/ (default for src/ execution)
func SetupViperPaths(v *viper.Viper) {
	// Check TB_ROOT_PATH environment variable
	if rootPath := os.Getenv("TB_ROOT_PATH"); rootPath != "" {
		v.AddConfigPath(filepath.Join(rootPath, "assets"))
		v.AddConfigPath(rootPath)
		log.Debug().
			Str("TB_ROOT_PATH", rootPath).
			Msg("Added TB_ROOT_PATH to Viper config paths")
	}

	// Standard paths (in priority order)
	v.AddConfigPath(".")
	v.AddConfigPath("./assets/")
	v.AddConfigPath("../assets/")
}

// Infra utilities

const maxUidLength = 20

var b32Encoding = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

// GenUid returns a uid string of exactly maxUidLength characters.
// The prefix length is derived from model.StrUidPrefix; the remaining characters
// are filled with crypto/rand bytes encoded in lowercase base32.
// Changing either maxUidLength or StrUidPrefix automatically adjusts the random part.
func GenUid() string {
	return GenUidWithLength(maxUidLength)
}

// GenUidWithLength returns a uid string of exactly length characters, for names whose
// length a CSP caps below maxUidLength (e.g., Azure AKS node pools allow 12).
// A shorter uid has less entropy, so use it only where the target enforces the limit;
// a collision is then rejected by the CSP rather than silently shared.
// length is clamped to [len(model.StrUidPrefix)+1, maxUidLength] so the result always
// keeps the prefix and at least one random character.
func GenUidWithLength(length int) string {
	prefix := model.StrUidPrefix
	if length > maxUidLength {
		length = maxUidLength
	}
	if length <= len(prefix) {
		length = len(prefix) + 1
	}
	randomLen := length - len(prefix)
	byteCount := (randomLen*5 + 7) / 8
	b := make([]byte, byteCount)
	crand.Read(b) // Go 1.20+: always succeeds; OS random source is guaranteed available
	return prefix + b32Encoding.EncodeToString(b)[:randomLen]
}

// GenRandomPassword is func to return a RandomPassword
func GenRandomPassword(length int) string {
	rand.Seed(time.Now().Unix())

	charset := "A1!$"
	shuff := []rune(charset)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})
	randomString := GenUid()
	if len(randomString) < length {
		randomString = randomString + GenUid()
	}
	reducedString := randomString[0 : length-len(charset)]
	reducedString = reducedString + string(shuff)

	shuff = []rune(reducedString)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})

	pw := string(shuff)

	return pw
}

// RandomSleep is func to make a caller waits for during random Milliseconds (from ms, to ms)
func RandomSleep(from, to int) {
	const minSleepTime = 1

	if from == 0 && to == 0 {
		from = minSleepTime
		to = minSleepTime
	} else if from > to {
		from, to = to, from
	}

	t := to - from
	if t == 0 {
		t = minSleepTime
	}

	n := rand.Intn(t) + from
	time.Sleep(time.Duration(n) * time.Millisecond)
}

// GetFuncName is func to get the name of the running function
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

// CheckString is func to check string by the given rule `[a-z]([-a-z0-9]*[a-z0-9])?`
func CheckString(name string) error {

	if name == "" {
		err := fmt.Errorf("CheckString - empty string")
		return err
	}

	r, _ := regexp.Compile("(?i)[a-z]([-a-z0-9+]*[a-z0-9])?")
	filtered := r.FindString(name)

	if filtered != name {
		err := fmt.Errorf("%s: The name must follow these rules: 1. The first character must be a letter (case-insensitive). 2. All following characters can be a dash, letter (case-insensitive), digit, or +. 3. The last character cannot be a dash.", name)
		return err
	}

	return nil
}

// ToLower is func to change strings (_ to -, " " to -, to lower string ) (deprecated soon)
func ToLower(name string) string {
	out := strings.ReplaceAll(name, "_", "-")
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ToLower(out)
	return out
}

// ChangeIdString is func to change strings in id or name (special chars to -, to lower string )
func ChangeIdString(name string) string {
	// Regex for letters and numbers
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	changedString := strings.ToLower(reg.ReplaceAllString(name, "-"))
	if changedString[len(changedString)-1:] == "-" {
		changedString += "r"
	}
	return changedString
}

// GenInfraKey is func to generate a key used in keyValue store

func ConvertToMessage(inType string, inData string, obj any) error {
	//logger := logging.NewLogger()

	if inType == "yaml" {
		err := yaml.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("yaml Unmarshal: \n", obj)
	}

	if inType == "json" {
		err := json.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("json Unmarshal: \n", obj)
	}

	return nil
}

// ConvertToOutput is func to convert gRPC message to print format
func ConvertToOutput(outType string, obj any) (string, error) {
	//logger := logging.NewLogger()

	if outType == "yaml" {
		// marshal using JSON to remove fields with XXX prefix
		j, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}

		// use MapSlice to avoid sorting fields
		jsonObj := yaml.MapSlice{}
		err2 := yaml.Unmarshal(j, &jsonObj)
		if err2 != nil {
			return "", err2
		}

		// yaml marshal
		y, err3 := yaml.Marshal(jsonObj)
		if err3 != nil {
			return "", err3
		}
		//logger.Debug("yaml Marshal: \n", string(y))

		return string(y), nil
	}

	if outType == "json" {
		j, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		//logger.Debug("json Marshal: \n", string(j))

		return string(j), nil
	}

	return "", nil
}

// CopySrcToDest is func to copy data from source to target
func CopySrcToDest(src any, dest any) error {
	//logger := logging.NewLogger()

	j, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("source value : \n", string(j))

	err = json.Unmarshal(j, dest)
	if err != nil {
		return err
	}

	j, err = json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("target value : \n", string(j))

	return nil
}

// NVL is func for null value logic
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

// GetChildIdList is func to get child id list from given key
func GetChildIdList(key string) []string {

	keyValue, _ := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, strings.TrimPrefix(v.Key, key+"/"))

	}
	for _, v := range childIdList {
		fmt.Println("<" + v + "> \n")
	}

	return childIdList

}

// GetObjectList is func to return IDs of each child objects that has the same key
func GetObjectList(key string) []string {

	keyValue, _ := kvstore.GetKvList(key)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, v.Key)
	}

	return childIdList

}

// GetObjectValue is func to return the object value
func GetObjectValue(key string) (string, error) {

	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if !exists {
		return "", nil
	}
	return keyValue.Value, nil
}

// DeleteObject is func to delete the object
func DeleteObject(key string) error {

	err := kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return nil
}

// DeleteObjects is func to delete objects
func DeleteObjects(key string) error {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		err := fmt.Errorf("invalid empty key for prefix delete")
		log.Error().Err(err).Msg("")
		return err
	}
	return kvstore.DeleteWithPrefix(trimmedKey)
}

func CheckElement(a string, list []string) bool {
	return slices.Contains(list, a)
}

const (
	// Random string generation
	letterBytes   = "abcdefghijklmnopqrstuvwxyz1234567890"
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

/* generate a random string (from CB-MCKS source code) */
func GenerateNewRandomString(n int) string {
	randSrc := rand.NewSource(time.Now().UnixNano()) //Random source by nano time
	b := make([]byte, n)
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

// GetK8sClusterInfo is func to get all kubernetes cluster info from the asset

func ConvertToBaseCurrency(cost float32, currency string) float32 {
	// If currency is already USD or empty, return the original cost
	if currency == "" || strings.ToUpper(currency) == "USD" {
		return cost
	}

	// Normalize currency code to uppercase for consistency
	currencyCode := strings.ToUpper(currency)

	// Exchange rates table (base: USD)
	// These should ideally come from an external source or database for up-to-date rates
	exchangeRates := map[string]float32{
		"USD": 1.0,     // 1 USD = 1 USD
		"KRW": 0.00074, // 1 KRW = 0.00074 USD (approx. 1350:1)
		"EUR": 1.09,    // 1 EUR = 1.09 USD
		"JPY": 0.0067,  // 1 JPY = 0.0067 USD
		"CNY": 0.14,    // 1 CNY = 0.14 USD
		"GBP": 1.27,    // 1 GBP = 1.27 USD
		"CAD": 0.74,    // 1 CAD = 0.74 USD
		"AUD": 0.66,    // 1 AUD = 0.66 USD
	}

	// Check if the currency code exists in our exchange rates table
	rate, exists := exchangeRates[currencyCode]
	if !exists {
		log.Warn().Msgf("Unknown currency code: %s, using original value", currencyCode)
		return cost
	}

	// Convert to USD (base currency) by multiplying by the exchange rate
	// For currencies like KRW, JPY: multiply by a small number (e.g., 0.00074)
	// For currencies like EUR, GBP: multiply by a number > 1
	return cost * rate
}
