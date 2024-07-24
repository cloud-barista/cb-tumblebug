package kvutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
)

// GetKeyList extracts keys from a slice of KeyValue pairs.
func GetKeyList(kvs []kvstore.KeyValue) []string {
	result := make([]string, 0)
	for _, kv := range kvs {
		result = append(result, kv.Key)
	}
	return result
}

// FilterKvMapBy filters a KeyValue map based on the given prefix key.
// It returns a new KeyValue map containing only the key-value pairs that match the prefix criteria.
func FilterKvMapBy(kvmap kvstore.KeyValueMap, prefixKey string, depthAfterPrefix int) kvstore.KeyValueMap {
	result := make(kvstore.KeyValueMap)
	prefix := strings.TrimSuffix(prefixKey, "/")

	for key, value := range kvmap {
		if strings.HasPrefix(key, prefix) && checkKeyDepth(key, prefix, depthAfterPrefix) {
			result[key] = value
		}
	}

	return result
}

// FilterKvListBy filters a slice of KeyValue pairs based on the given prefix key.
// It returns a new slice containing only the key-value pairs that match the prefix criteria.
func FilterKvListBy(kvs []kvstore.KeyValue, prefixKey string, depthAfterPrefix int) []kvstore.KeyValue {
	result := make([]kvstore.KeyValue, 0)
	prefix := strings.TrimSuffix(prefixKey, "/")

	for _, kv := range kvs {
		if strings.HasPrefix(kv.Key, prefix) && checkKeyDepth(kv.Key, prefix, depthAfterPrefix) {
			result = append(result, kv)
		}
	}

	return result
}

// checkKeyDepth checks if the key has only the specified number of segments after the prefix.
func checkKeyDepth(key, prefix string, depthAfterPrefix int) bool {
	// Trim the prefix from the key and split the remaining part
	trimmedKey := strings.TrimPrefix(key, prefix)
	trimmedKey = strings.TrimPrefix(trimmedKey, "/")
	segments := strings.Split(trimmedKey, "/")

	// Check if the key has only one additional key segment
	return len(segments) == depthAfterPrefix
}

// ExtractIDsFromKey extracts specific IDs from a given key based on the provided structure.
// It returns a slice of extracted ID values in the order of provided ID types.
// If any ID type is not found in the key, it returns an error.
func ExtractIDsFromKey(key string, idTypes ...string) ([]string, error) {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	if len(parts) < len(idTypes)*2 {
		return nil, fmt.Errorf("key does not contain all requested ID types")
	}

	ids := make([]string, len(idTypes))
	for i, idType := range idTypes {
		index := indexOf(parts, idType)
		if index == -1 || index+1 >= len(parts) {
			return nil, fmt.Errorf("could not find ID for type: %s", idType)
		}
		ids[i] = parts[index+1]
	}
	return ids, nil
}

// ExtractLastKeySegmentList function extracts the last segment of each key in the keyValue slice.
func ExtractLastKeySegmentList(keyValue []kvstore.KeyValue) ([]string, error) {
	var idList []string
	for _, kv := range keyValue {
		parts := strings.Split(kv.Key, "/")
		if len(parts) == 0 {
			return nil, errors.New("invalid key format, expected at least one segment")
		}
		lastSegment := parts[len(parts)-1]
		idList = append(idList, lastSegment)
	}
	return idList, nil
}

// ContainsIDs checks if a key contains specific ID values.
// It returns true if the key contains all ID types and values specified in the ids map.
func ContainsIDs(key string, ids map[string]string) bool {
	parts := strings.Split(strings.Trim(key, "/"), "/")
	for idType, idValue := range ids {
		index := indexOf(parts, idType)
		if index == -1 || index+1 >= len(parts) || parts[index+1] != idValue {
			return false
		}
	}
	return true
}

// [May not needed]
// // BuildKeyBy constructs a key from given ID types and values.
// // It returns a string representing the constructed key.
// func BuildKeyBy(ids map[string]string) string {
// 	var parts []string
// 	for idType, idValue := range ids {
// 		parts = append(parts, idType, idValue)
// 	}
// 	return "/" + strings.Join(parts, "/")
// }

// indexOf finds the index of a string in a slice.
// It returns the index if found, or -1 if not found.
func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
