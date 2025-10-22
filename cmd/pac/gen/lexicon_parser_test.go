package gen

import (
	"strings"
	"testing"
)

func TestParser_ParseLexicon(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid lexicon",
			input: `{
				"lexicon": 1,
				"id": "com.test.example",
				"description": "A test lexicon",
				"defs": {
					"main": {
						"type": "record",
						"key": "com.test.record",
						"record": {
							"type": "object",
							"properties": {
								"text": {
									"type": "string"
								}
							}
						}
					}
				}
			}`,
			wantErr: false,
		},
		{
			name: "missing lexicon version",
			input: `{
				"id": "com.test.example",
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
			wantErr: true, // Missing lexicon version should error
		},
		{
			name: "unsupported lexicon version",
			input: `{
				"lexicon": 2,
				"id": "com.test.example",
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
			name: "missing id",
			input: `{
				"lexicon": 1,
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
			name: "missing defs",
			input: `{
				"lexicon": 1,
				"id": "com.test.example"
			}`,
			wantErr: true,
		},
		{
			name: "invalid JSON",
			input: `{
				"lexicon": 1,
				"id": "com.test.example",
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

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			doc, err := parser.ParseLexicon(reader)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLexicon() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && doc == nil {
				t.Errorf("ParseLexicon() returned nil document but no error")
				return
			}

			if !tt.wantErr {
				// Verify basic structure
				if doc.ID == "" {
					t.Errorf("ParseLexicon() returned document with empty ID")
				}
				if doc.Defs == nil {
					t.Errorf("ParseLexicon() returned document with nil defs")
				}
			}
		})
	}
}

func TestParser_GetPrimaryDefinition(t *testing.T) {
	tests := []struct {
		name       string
		doc        *LexiconDocument
		parsedDefs map[string]interface{}
		wantName   string
		wantErr    bool
	}{
		{
			name: "main definition exists",
			doc: &LexiconDocument{
				ID: "com.test.example",
				Defs: map[string]interface{}{
					"main": map[string]interface{}{
						"type": "record",
					},
				},
			},
			parsedDefs: map[string]interface{}{
				"main": RecordDefinition{
					Type: "record",
				},
			},
			wantName: "main",
			wantErr:  false,
		},
		{
			name: "no main definition, has primary type",
			doc: &LexiconDocument{
				ID: "com.test.example",
				Defs: map[string]interface{}{
					"getPost": map[string]interface{}{
						"type": "query",
					},
				},
			},
			parsedDefs: map[string]interface{}{
				"getPost": QueryDefinition{
					Type: "query",
				},
			},
			wantName: "getPost",
			wantErr:  false,
		},
		{
			name: "no primary definition",
			doc: &LexiconDocument{
				ID: "com.test.example",
				Defs: map[string]interface{}{
					"someType": map[string]interface{}{
						"type": "object",
					},
				},
			},
			parsedDefs: map[string]interface{}{
				"someType": SchemaField{
					Type: "object",
				},
			},
			wantName: "",
			wantErr:  true,
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, def, err := parser.GetPrimaryDefinition(tt.doc, tt.parsedDefs)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPrimaryDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if name != tt.wantName {
					t.Errorf("GetPrimaryDefinition() name = %v, want %v", name, tt.wantName)
				}
				if def == nil {
					t.Errorf("GetPrimaryDefinition() returned nil definition")
				}
			}
		})
	}
}
