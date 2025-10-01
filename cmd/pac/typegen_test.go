package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FakeTypeGenerator is a test implementation that returns predictable output
type FakeTypeGenerator struct {
	Output string
}

func (m *FakeTypeGenerator) GenerateTypes(inputReader io.Reader) ([]byte, error) {
	// Read the input to verify it was passed correctly
	inputContent, err := io.ReadAll(inputReader)
	if err != nil {
		return nil, err
	}

	// Generate predictable output based on input
	output := `// Generated TypeScript types from atproto lexicon schema
// Input content length: ` + string(rune(len(inputContent))) + `

export interface MockType {
  // This is a test interface
  testField: string;
  inputLength: number;
}

export default MockType;
`
	return []byte(output), nil
}

func TestGenerateTypesWithGenerator(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "typegen_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test input file
	inputFile := filepath.Join(tempDir, "test-schema.json")
	inputContent := `{"lexicon": 1, "id": "com.test.example"}`
	if err := os.WriteFile(inputFile, []byte(inputContent), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	// Create output file path
	outputFile := filepath.Join(tempDir, "test_output.ts")

	// Create mock generator
	mockGenerator := &FakeTypeGenerator{}

	// Test the function
	err = generateTypesWithGenerator(inputFile, outputFile, mockGenerator)
	if err != nil {
		t.Fatalf("generateTypesWithGenerator failed: %v", err)
	}

	// Verify output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatalf("Output file was not created: %v", err)
	}

	// Verify output content
	outputContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(outputContent)
	if !strings.Contains(outputStr, "export interface MockType") {
		t.Errorf("Output file does not contain expected interface")
	}

	if !strings.Contains(outputStr, "inputLength: number") {
		t.Errorf("Output file does not contain expected field")
	}
}
