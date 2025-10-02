package gen

import (
	"testing"
)

func TestParseDefinition(t *testing.T) {
	tests := []struct {
		name     string
		defType  string
		rawDef   map[string]interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:    "record definition",
			defType: "record",
			rawDef: map[string]interface{}{
				"type":        "record",
				"description": "A test record",
				"key":         "com.test.record",
				"record": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"text": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			expected: RecordDefinition{
				Type:        "record",
				Description: "A test record",
				Key:         "com.test.record",
				Record: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"text": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "query definition",
			defType: "query",
			rawDef: map[string]interface{}{
				"type":        "query",
				"description": "A test query",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			expected: QueryDefinition{
				Type:        "query",
				Description: "A test query",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "schema field definition",
			defType: "object",
			rawDef: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: SchemaField{
				Type: "object",
				Properties: map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDefinition(tt.defType, tt.rawDef)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Compare the result with expected based on type
				switch expected := tt.expected.(type) {
				case RecordDefinition:
					if resultDef, ok := result.(RecordDefinition); ok {
						if resultDef.Type != expected.Type || resultDef.Key != expected.Key {
							t.Errorf("ParseDefinition() = %v, want %v", result, expected)
						}
					} else {
						t.Errorf("ParseDefinition() returned wrong type: %T", result)
					}
				case QueryDefinition:
					if resultDef, ok := result.(QueryDefinition); ok {
						if resultDef.Type != expected.Type {
							t.Errorf("ParseDefinition() = %v, want %v", result, expected)
						}
					} else {
						t.Errorf("ParseDefinition() returned wrong type: %T", result)
					}
				case SchemaField:
					if resultDef, ok := result.(SchemaField); ok {
						if resultDef.Type != expected.Type {
							t.Errorf("ParseDefinition() = %v, want %v", result, expected)
						}
					} else {
						t.Errorf("ParseDefinition() returned wrong type: %T", result)
					}
				}
			}
		})
	}
}
