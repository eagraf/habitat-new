package gen

import "encoding/json"

// LexiconDocument represents the top-level lexicon document structure
type LexiconDocument struct {
	Lexicon     int                    `json:"lexicon"`
	ID          string                 `json:"id"`
	Description string                 `json:"description,omitempty"`
	Defs        map[string]interface{} `json:"defs"`
}

// Definition represents a single definition within a lexicon
type Definition struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// Additional fields depend on the type
	AdditionalFields map[string]interface{} `json:"-"`
}

// RecordDefinition represents a record type definition
type RecordDefinition struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Key         string      `json:"key"`
	Record      interface{} `json:"record"`
}

// QueryDefinition represents a query type definition
type QueryDefinition struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Output      *OutputDef  `json:"output,omitempty"`
	Errors      []ErrorDef  `json:"errors,omitempty"`
}

// ProcedureDefinition represents a procedure type definition
type ProcedureDefinition struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Input       *OutputDef  `json:"input,omitempty"`
	Output      *OutputDef  `json:"output,omitempty"`
	Errors      []ErrorDef  `json:"errors,omitempty"`
}

// SubscriptionDefinition represents a subscription type definition
type SubscriptionDefinition struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Message     *MessageDef `json:"message,omitempty"`
	Errors      []ErrorDef  `json:"errors,omitempty"`
}

// OutputDef represents output/input definition for queries and procedures
type OutputDef struct {
	Description string      `json:"description,omitempty"`
	Encoding    string      `json:"encoding"`
	Schema      interface{} `json:"schema,omitempty"`
}

// MessageDef represents message definition for subscriptions
type MessageDef struct {
	Description string      `json:"description,omitempty"`
	Schema      interface{} `json:"schema"`
}

// ErrorDef represents an error definition
type ErrorDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SchemaField represents a field in an object schema
type SchemaField struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Items       interface{}            `json:"items,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	KnownValues []string               `json:"knownValues,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	Const       interface{}            `json:"const,omitempty"`
	Format      string                 `json:"format,omitempty"`
	MaxLength   int                    `json:"maxLength,omitempty"`
	MinLength   int                    `json:"minLength,omitempty"`
	Minimum     int                    `json:"minimum,omitempty"`
	Maximum     int                    `json:"maximum,omitempty"`
}

// ParseDefinition parses a raw definition into the appropriate typed structure
func ParseDefinition(defType string, rawDef map[string]interface{}) (interface{}, error) {
	switch defType {
	case "record":
		var recordDef RecordDefinition
		if err := parseIntoStruct(rawDef, &recordDef); err != nil {
			return nil, err
		}
		return recordDef, nil
	case "query":
		var queryDef QueryDefinition
		if err := parseIntoStruct(rawDef, &queryDef); err != nil {
			return nil, err
		}
		return queryDef, nil
	case "procedure":
		var procDef ProcedureDefinition
		if err := parseIntoStruct(rawDef, &procDef); err != nil {
			return nil, err
		}
		return procDef, nil
	case "subscription":
		var subDef SubscriptionDefinition
		if err := parseIntoStruct(rawDef, &subDef); err != nil {
			return nil, err
		}
		return subDef, nil
	default:
		// For non-primary types, return as generic schema field
		var schemaDef SchemaField
		if err := parseIntoStruct(rawDef, &schemaDef); err != nil {
			return nil, err
		}
		return schemaDef, nil
	}
}

// parseIntoStruct is a helper to convert map[string]interface{} to struct
func parseIntoStruct(data map[string]interface{}, target interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}
