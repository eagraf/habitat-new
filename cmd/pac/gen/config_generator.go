package gen

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ConfigGenerator generates data route configuration files
type ConfigGenerator struct{}

// NewConfigGenerator creates a new config generator
func NewConfigGenerator() *ConfigGenerator {
	return &ConfigGenerator{}
}

// GenerateConfig generates a data route config file
func (g *ConfigGenerator) GenerateConfig(entityFiles []string, projectRoot string) ([]byte, error) {
	// Extract lexicon IDs from entity files
	lexiconIDs, err := g.extractLexiconIDs(entityFiles, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to extract lexicon IDs: %w", err)
	}

	var output strings.Builder

	// Add header comment
	output.WriteString("// This file is intended to be overwritten by code generation CLI tools\n")
	output.WriteString("// Contains route configuration and lexicon definitions for the data debugger\n\n")

	// Generate the lexicon IDs array
	output.WriteString("export const DATA_ROUTE_LEXICONS = [")
	if len(lexiconIDs) > 0 {
		output.WriteString("\n")
		for i, lexiconID := range lexiconIDs {
			output.WriteString(fmt.Sprintf("  '%s'", lexiconID))
			if i < len(lexiconIDs)-1 {
				output.WriteString(",")
			}
			output.WriteString("\n")
		}
	}
	output.WriteString("] as const;\n\n")

	// Generate the config object
	output.WriteString("// Additional route configuration can be added here as needed\n")
	output.WriteString("export const DATA_ROUTE_CONFIG = {\n")
	output.WriteString("  lexicons: DATA_ROUTE_LEXICONS,\n")
	output.WriteString("  // Future configuration options can be added here\n")
	output.WriteString("} as const;\n")

	return []byte(output.String()), nil
}

// extractLexiconIDs extracts lexicon IDs from entity client files
func (g *ConfigGenerator) extractLexiconIDs(entityFiles []string, _ string) ([]string, error) {
	var lexiconIDs []string
	seen := make(map[string]bool) // To avoid duplicates

	for _, entityFile := range entityFiles {
		// Read the client file to extract the lexicon ID from the collection parameter
		lexiconID, err := g.extractLexiconIDFromClientFile(entityFile)
		if err != nil {
			// If we can't extract from the client file, try to infer from the file path
			entityName := g.getEntityNameFromFile(entityFile)
			lexiconID = g.inferLexiconID(entityName)
		}

		if lexiconID != "" && !seen[lexiconID] {
			lexiconIDs = append(lexiconIDs, lexiconID)
			seen[lexiconID] = true
		}
	}

	return lexiconIDs, nil
}

// extractLexiconIDFromClientFile reads a client file and extracts the lexicon ID from the collection parameter
func (g *ConfigGenerator) extractLexiconIDFromClientFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	collectionRegex := regexp.MustCompile(`collection:\s*['"]([^'"]+)['"]`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := collectionRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no collection parameter found")
}

// inferLexiconID infers a lexicon ID from an entity name
func (g *ConfigGenerator) inferLexiconID(entityName string) string {
	// This is a simple mapping - in practice, you might want to store this information
	// when generating the client files, or parse it from the original lexicon files

	// Common patterns
	switch entityName {
	case "event":
		return "community.lexicon.calendar.event"
	case "note":
		return "dev.eagraf.note"
	default:
		// For unknown entities, we could try to infer from common patterns
		// or return an empty string to indicate we don't know
		return ""
	}
}

// getEntityNameFromFile extracts entity name from file path
func (g *ConfigGenerator) getEntityNameFromFile(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	// Remove "_client" suffix if present
	if strings.HasSuffix(name, "_client") {
		name = name[:len(name)-7] // Remove "_client"
	}

	return strings.ToLower(name)
}
