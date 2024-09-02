package fishtail

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Some helpers for working with CBOR in Go.

func convertCBORToMapStringInterface(cborBytes []byte) (map[string]interface{}, error) {
	var res map[string]interface{}
	decoder := cbor.NewDecoder(bytes.NewReader(cborBytes))
	if err := decoder.Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode CBOR: %w", err)
	}

	// Convert all sub-maps to map[string]interface{}
	convertedData := convertToStringKeyMap(res)
	res = convertedData.(map[string]interface{})

	return res, nil
}

// convertToStringKeyMap ensures all keys in a map and it's sub-maps are strings.
// This resolves a problem with the fxamacker/cbor library where decoded keys are sometimes of type interface{}.
func convertToStringKeyMap(v interface{}) interface{} {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range x {
			m[fmt.Sprint(k)] = convertToStringKeyMap(v)
		}
		return m
	case map[string]interface{}:
		for k, v := range x {
			x[k] = convertToStringKeyMap(v)
		}
	case []interface{}:
		for i, v := range x {
			x[i] = convertToStringKeyMap(v)
		}
	}
	return v
}
