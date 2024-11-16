package fishtail

import (
	"bytes"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

func TestConvertCBORToMapStringInterface(t *testing.T) {
	// Test case 1: Simple map
	original := map[string]interface{}{
		"a": uint64(1),
		"b": int64(-2),
		"c": "17",
	}

	var buf bytes.Buffer
	encoder := cbor.NewEncoder(&buf)
	err := encoder.Encode(original)
	assert.NoError(t, err)

	result, err := convertCBORToMapStringInterface(buf.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, original, result)

	// Test case 2: Nested map
	original2 := map[string]interface{}{
		"a": uint64(1),
		"b": map[string]interface{}{
			"c": uint64(2),
			"d": "17",
		},
	}

	buf.Reset()
	err = encoder.Encode(original2)
	assert.NoError(t, err)

	result2, err := convertCBORToMapStringInterface(buf.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, original2, result2)

	// Test case 3: Array in map
	original3 := map[string]interface{}{
		"a": []interface{}{
			uint64(1),
			"two",
			map[string]interface{}{
				"nested": "value",
			},
		},
	}

	buf.Reset()
	err = encoder.Encode(original3)
	assert.NoError(t, err)

	result3, err := convertCBORToMapStringInterface(buf.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, original3, result3)

	// Test case 4: Invalid CBOR
	_, err = convertCBORToMapStringInterface([]byte{0xFF})
	assert.Error(t, err)
}
