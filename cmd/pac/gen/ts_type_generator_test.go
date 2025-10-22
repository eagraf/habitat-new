package gen

import (
	"os"
	"strings"
	"testing"
)

func TestTypeScriptGenerator_toPascalCase(t *testing.T) {
	generator := NewTypeScriptGenerator()

	tests := []struct {
		input    string
		expected string
	}{
		{"main", "Main"},
		{"getPost", "GetPost"},
		{"get_post", "GetPost"},
		{"get-post", "GetPost"},
		{"get.post", "GetPost"},
		{"createUserPost", "CreateUserPost"},
		{"create_user_post", "CreateUserPost"},
		{"", ""},
		{"a", "A"},
		{"alreadyPascal", "AlreadyPascal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := generator.toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTypeScriptGenerator_getTypeScriptType(t *testing.T) {
	generator := NewTypeScriptGenerator()

	tests := []struct {
		name     string
		schema   interface{}
		expected string
	}{
		{
			name: "string type",
			schema: map[string]interface{}{
				"type": "string",
			},
			expected: "string",
		},
		{
			name: "integer type",
			schema: map[string]interface{}{
				"type": "integer",
			},
			expected: "number",
		},
		{
			name: "boolean type",
			schema: map[string]interface{}{
				"type": "boolean",
			},
			expected: "boolean",
		},
		{
			name: "array type",
			schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			expected: "string[]",
		},
		{
			name: "object type",
			schema: map[string]interface{}{
				"type": "object",
			},
			expected: "Record<string, any>",
		},
		{
			name: "enum string",
			schema: map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"option1", "option2"},
			},
			expected: "'option1' | 'option2'",
		},
		{
			name: "ref type",
			schema: map[string]interface{}{
				"$ref": "#someType",
			},
			expected: "SomeType",
		},
		{
			name:     "null schema",
			schema:   nil,
			expected: "any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.getTypeScriptType(tt.schema)
			if result != tt.expected {
				t.Errorf("getTypeScriptType() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestTypeScriptGenerator_GenerateTypes_EventTest(t *testing.T) {
	// Read the input JSON file
	inputBytes, err := os.ReadFile("test_files/event_test/event.json")
	if err != nil {
		t.Fatalf("Failed to read input file: %v", err)
	}

	// Read the expected output file
	expectedBytes, err := os.ReadFile("test_files/event_test/event_types.ts")
	if err != nil {
		t.Fatalf("Failed to read expected output file: %v", err)
	}

	// Generate the types
	generator := NewTypeScriptGenerator()
	reader := strings.NewReader(string(inputBytes))
	result, err := generator.GenerateTypes(reader)
	if err != nil {
		t.Fatalf("GenerateTypes() failed: %v", err)
	}

	// Compare the result with the expected output
	resultStr := string(result)
	expectedStr := string(expectedBytes)

	// Extract all non-comment, non-empty lines for comparison
	// This handles ordering differences from Go map iteration
	expectedLines := extractSignificantLines(expectedStr)
	resultLines := extractSignificantLines(resultStr)

	// Check that all expected type definitions are present
	for _, expectedLine := range expectedLines {
		found := false
		for _, resultLine := range resultLines {
			if expectedLine == resultLine {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected line not found in result:\n%s\n\nFull result:\n%s", expectedLine, resultStr)
		}
	}

	// Check that no extra types were generated
	if len(resultLines) != len(expectedLines) {
		t.Errorf("Expected %d significant lines, got %d", len(expectedLines), len(resultLines))
	}
}

// extractSignificantLines extracts type/interface definitions and property lines
func extractSignificantLines(content string) []string {
	lines := strings.Split(content, "\n")
	var significant []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments, empty lines, and opening/closing braces
		if trimmed == "" || strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "/**") || strings.HasPrefix(trimmed, "*") ||
			trimmed == "{" || trimmed == "}" {
			continue
		}
		significant = append(significant, trimmed)
	}

	return significant
}
