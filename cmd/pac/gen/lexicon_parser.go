package gen

import (
	"encoding/json"
	"fmt"
	"io"
)

// Parser handles parsing of atproto lexicon documents
type Parser struct{}

// NewParser creates a new parser instance
func NewParser() *Parser {
	return &Parser{}
}

// ParseLexicon parses a lexicon document from a reader
func (p *Parser) ParseLexicon(reader io.Reader) (*LexiconDocument, error) {
	var doc LexiconDocument

	// Read the entire content
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate lexicon version
	if doc.Lexicon != 1 {
		return nil, fmt.Errorf("unsupported lexicon version: %d (expected 1)", doc.Lexicon)
	}

	// Validate required fields
	if doc.ID == "" {
		return nil, fmt.Errorf("missing required field: id")
	}

	if doc.Defs == nil || len(doc.Defs) == 0 {
		return nil, fmt.Errorf("missing or empty required field: defs")
	}

	return &doc, nil
}

// ParseDefinitions parses all definitions in a lexicon document
func (p *Parser) ParseDefinitions(doc *LexiconDocument) (map[string]interface{}, error) {
	parsedDefs := make(map[string]interface{})

	for name, rawDef := range doc.Defs {
		// Convert to map[string]interface{} if it's not already
		defMap, ok := rawDef.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("definition %s is not a valid object", name)
		}

		// Get the type field
		defType, ok := defMap["type"].(string)
		if !ok {
			return nil, fmt.Errorf("definition %s missing or invalid type field", name)
		}

		// Parse the definition based on its type
		parsedDef, err := ParseDefinition(defType, defMap)
		if err != nil {
			return nil, fmt.Errorf("failed to parse definition %s: %w", name, err)
		}

		parsedDefs[name] = parsedDef
	}

	return parsedDefs, nil
}

// GetPrimaryDefinition returns the main definition if it exists
func (p *Parser) GetPrimaryDefinition(doc *LexiconDocument, parsedDefs map[string]interface{}) (string, interface{}, error) {
	// Check if main definition exists
	if mainDef, exists := parsedDefs["main"]; exists {
		return "main", mainDef, nil
	}

	// If no main definition, look for any primary type definition
	primaryTypes := []string{"record", "query", "procedure", "subscription"}

	for _, primaryType := range primaryTypes {
		for name, def := range parsedDefs {
			// Check if this is a primary type
			if schemaDef, ok := def.(SchemaField); ok && schemaDef.Type == primaryType {
				return name, def, nil
			}
			// Check other definition types
			switch d := def.(type) {
			case RecordDefinition:
				if d.Type == primaryType {
					return name, def, nil
				}
			case QueryDefinition:
				if d.Type == primaryType {
					return name, def, nil
				}
			case ProcedureDefinition:
				if d.Type == primaryType {
					return name, def, nil
				}
			case SubscriptionDefinition:
				if d.Type == primaryType {
					return name, def, nil
				}
			}
		}
	}

	return "", nil, fmt.Errorf("no primary definition found")
}
