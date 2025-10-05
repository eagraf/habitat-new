package gen

import (
	"strings"
	"testing"
)

func TestTypeScriptGenerator_GenerateTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains []string
	}{
		{
			name: "simple record lexicon",
			input: `{
				"lexicon": 1,
				"id": "com.test.record",
				"description": "A test record",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.test.record",
						"record": {
							"type": "object",
							"properties": {
								"text": {
									"type": "string",
									"description": "The text content"
								},
								"count": {
									"type": "integer",
									"description": "The count"
								}
							},
							"required": ["text"]
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export interface Record",
				"$type: 'com.test.record'",
				"text: string",
				"count?: number",
			},
		},
		{
			name: "query with parameters and output",
			input: `{
				"lexicon": 1,
				"id": "com.test.query",
				"defs": {
					"getItem": {
						"type": "query",
						"description": "Get an item by ID",
						"parameters": {
							"type": "object",
							"properties": {
								"id": {
									"type": "string"
								}
							},
							"required": ["id"]
						},
						"output": {
							"encoding": "application/json",
							"schema": {
								"type": "object",
								"properties": {
									"item": {
										"type": "object",
										"properties": {
											"name": {
												"type": "string"
											}
										}
									}
								}
							}
						}
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export interface GetItemParams",
				"id: string",
				"export interface GetItemOutput",
				"item?: Record<string, any>",
			},
		},
		{
			name: "procedure with errors",
			input: `{
				"lexicon": 1,
				"id": "com.test.procedure",
				"defs": {
					"createItem": {
						"type": "procedure",
						"input": {
							"encoding": "application/json",
							"schema": {
								"type": "object",
								"properties": {
									"name": {
										"type": "string"
									}
								},
								"required": ["name"]
							}
						},
						"output": {
							"encoding": "application/json",
							"schema": {
								"type": "object",
								"properties": {
									"id": {
										"type": "string"
									}
								}
							}
						},
						"errors": [
							{
								"name": "InvalidName",
								"description": "The name is invalid"
							},
							{
								"name": "AlreadyExists",
								"description": "Item already exists"
							}
						]
					}
				}
			}`,
			wantErr: false,
			contains: []string{
				"export interface CreateItemInput",
				"name: string",
				"export interface CreateItemOutput",
				"export type CreateItemError",
				"'InvalidName'",
				"'AlreadyExists'",
			},
		},
		{
			name: "invalid lexicon version",
			input: `{
				"lexicon": 2,
				"id": "com.test.invalid",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.test.record",
						"record": {
							"type": "object"
						}
					}
				}
			}`,
			wantErr: true,
		},
		{
			name: "invalid JSON",
			input: `{
				"lexicon": 1,
				"id": "com.test.invalid",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.test.record",
						"record": {
							"type": "object"
						}
					}
				}
			`, // Missing closing brace
			wantErr: true,
		},
	}

	generator := NewTypeScriptGenerator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := generator.GenerateTypes(reader)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateTypes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Errorf("GenerateTypes() returned nil result")
					return
				}

				resultStr := string(result)

				// Check that the result contains expected strings
				for _, expected := range tt.contains {
					if !strings.Contains(resultStr, expected) {
						t.Errorf("GenerateTypes() result does not contain expected string: %s", expected)
						t.Logf("Generated content:\n%s", resultStr)
					}
				}

				// Basic sanity checks
				if !strings.Contains(resultStr, "Generated TypeScript types") {
					t.Errorf("GenerateTypes() result missing header comment")
				}
			}
		})
	}
}

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
