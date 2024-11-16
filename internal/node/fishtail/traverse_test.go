package fishtail

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

func TestTraverseCBOR(t *testing.T) {
	// Test case 1: Simple map with string that should be visited
	original := map[string]interface{}{
		"uri":   "at://did:plc:123/app.bsky.feed.post/123",
		"other": "not-a-uri",
	}

	var buf bytes.Buffer
	encoder := cbor.NewEncoder(&buf)
	err := encoder.Encode(original)
	assert.NoError(t, err)

	visited := make([]interface{}, 0)
	visitor := func(node interface{}) error {
		visited = append(visited, node)
		return nil
	}

	err = traverseCBOR(buf.Bytes(), visitor)
	assert.NoError(t, err)

	// Should visit the map and both string values
	assert.Contains(t, visited, "at://did:plc:123/app.bsky.feed.post/123")
	assert.Contains(t, visited, "not-a-uri")

	// Test case 2: Nested structure
	original2 := map[string]interface{}{
		"nested": map[string]interface{}{
			"uri": "at://did:plc:456/app.bsky.feed.post/456",
		},
		"array": []interface{}{
			"at://did:plc:789/app.bsky.feed.post/789",
			map[string]interface{}{
				"uri": "at://did:plc:abc/app.bsky.feed.post/abc",
			},
		},
	}

	buf.Reset()
	err = encoder.Encode(original2)
	assert.NoError(t, err)

	visited = make([]interface{}, 0)
	err = traverseCBOR(buf.Bytes(), visitor)
	assert.NoError(t, err)

	// Should visit all URIs in the nested structure
	assert.Contains(t, visited, "at://did:plc:456/app.bsky.feed.post/456")
	assert.Contains(t, visited, "at://did:plc:789/app.bsky.feed.post/789")
	assert.Contains(t, visited, "at://did:plc:abc/app.bsky.feed.post/abc")

	// Test case 3: Invalid CBOR
	err = traverseCBOR([]byte{0xFF}, visitor)
	assert.Error(t, err)
}

func TestTraverseJSON(t *testing.T) {
	// Test case 1: Simple structure
	original := map[string]interface{}{
		"uri":     "at://did:plc:123/app.bsky.feed.post/123",
		"notAUri": "not-a-uri",
	}

	visited := make([]interface{}, 0)
	visitor := func(node interface{}) error {
		visited = append(visited, node)
		return nil
	}

	err := traverseJSON(original, visitor)
	assert.NoError(t, err)

	// Should visit the map and both string values
	assert.Contains(t, visited, "at://did:plc:123/app.bsky.feed.post/123")
	assert.Contains(t, visited, "not-a-uri")

	// Test case 2: Nested structure
	original2 := map[string]interface{}{
		"nested": map[string]interface{}{
			"uri": "at://did:plc:456/app.bsky.feed.post/456",
		},
		"array": []interface{}{
			"at://did:plc:789/app.bsky.feed.post/789",
			map[string]interface{}{
				"uri": "at://did:plc:abc/app.bsky.feed.post/abc",
			},
		},
	}

	visited = make([]interface{}, 0)
	err = traverseJSON(original2, visitor)
	assert.NoError(t, err)

	// Should visit all URIs in the nested structure
	assert.Contains(t, visited, "at://did:plc:456/app.bsky.feed.post/456")
	assert.Contains(t, visited, "at://did:plc:789/app.bsky.feed.post/789")
	assert.Contains(t, visited, "at://did:plc:abc/app.bsky.feed.post/abc")

	// Test case 3: Error from visitor
	errorVisitor := func(node interface{}) error {
		return fmt.Errorf("test error")
	}

	err = traverseJSON(original, errorVisitor)
	assert.Error(t, err)
}
