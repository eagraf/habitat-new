package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eagraf/habitat-new/cmd/pac/gen"
	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/spf13/cobra"
)

var (
	projectRoot string
)

// ClientGenerator defines the interface for generating TypeScript client code from atproto schemas
type ClientGenerator interface {
	GenerateClient(inputReader io.Reader) ([]byte, error)
}

var clientgenCmd = &cobra.Command{
	Use:   "clientgen <atprotofile>",
	Short: "Generate TypeScript client code from atproto lexicon schemas",
	Long:  `Generate TypeScript client helper functions for CRUD operations against an ATProto Personal Data Server (PDS).`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		atprotoFile := args[0]

		// Validate that the input file exists
		if _, err := os.Stat(atprotoFile); os.IsNotExist(err) {
			logging.CheckErr(fmt.Errorf("Input file does not exist: %s", atprotoFile))
			os.Exit(1)
		}

		// Determine project root
		root := projectRoot
		if root == "" {
			// Default to current directory
			root = "."
		}

		// Convert to absolute path
		absRoot, err := filepath.Abs(root)
		if err != nil {
			logging.CheckErr(fmt.Errorf("Failed to resolve project root path: %w", err))
			os.Exit(1)
		}

		// Validate that the project has been initialized
		if err := validateProjectInit(absRoot); err != nil {
			logging.CheckErr(fmt.Errorf("Project not properly initialized: %w\nRun 'pac init' first", err))
			os.Exit(1)
		}

		// Output directory is always <project-root>/src/api
		outputPath := filepath.Join(absRoot, "src", "api")

		// Ensure output directory exists
		if err := os.MkdirAll(outputPath, 0755); err != nil {
			logging.Error(fmt.Sprintf("Failed to create output directory: %v", err))
			os.Exit(1)
		}

		// Determine entity name from the lexicon file
		entityName := getEntityNameFromFile(atprotoFile)
		outputFile := filepath.Join(outputPath, fmt.Sprintf("%s_client.ts", entityName))

		logging.Infof("Generating TypeScript client code from: %s", atprotoFile)
		logging.Infof("Output file: %s", outputFile)

		// Use the client generator from the gen package
		generator := gen.NewClientGenerator()
		if err := generateClientWithGenerator(atprotoFile, outputFile, generator); err != nil {
			logging.CheckErr(err)
		}

		logging.Success("TypeScript client code generated successfully!")
	},
}

// generateClientWithGenerator handles the file I/O and delegates client generation to the injected generator
func generateClientWithGenerator(inputFile, outputFile string, generator ClientGenerator) error {
	logging.Debug("Reading atproto schema file")
	logging.Debugf("Input file: %s", inputFile)

	// Open the input file
	inputReader, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %v", err)
	}
	defer inputReader.Close()

	// Generate client code using the injected generator
	logging.Debug("Generating TypeScript client code")
	generatedContent, err := generator.GenerateClient(inputReader)
	if err != nil {
		return fmt.Errorf("failed to generate client: %v", err)
	}

	// Write the generated content to the output file
	if err := os.WriteFile(outputFile, generatedContent, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	logging.Debug("Client generation completed")
	return nil
}

// validateProjectInit checks if the project has been properly initialized
func validateProjectInit(projectRoot string) error {
	// Check for src/sdk/atproto.ts file relative to project root
	sdkPath := filepath.Join(projectRoot, "src", "sdk", "atproto.ts")
	if _, err := os.Stat(sdkPath); os.IsNotExist(err) {
		return fmt.Errorf("SDK file not found: %s", sdkPath)
	}
	return nil
}

// getEntityNameFromFile extracts the entity name from the lexicon file
func getEntityNameFromFile(filePath string) string {
	// Try to parse the lexicon file to get the ID
	inputReader, err := os.Open(filePath)
	if err != nil {
		// Fallback to using filename
		return getEntityNameFromFilename(filePath)
	}
	defer inputReader.Close()

	parser := gen.NewParser()
	doc, err := parser.ParseLexicon(inputReader)
	if err != nil {
		// Fallback to using filename
		return getEntityNameFromFilename(filePath)
	}

	// Extract entity name from lexicon ID
	parts := strings.Split(doc.ID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return getEntityNameFromFilename(filePath)
}

// getEntityNameFromFilename extracts entity name from the filename
func getEntityNameFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return strings.ToLower(name)
}

func init() {
	// Add project root flag
	clientgenCmd.Flags().StringVarP(&projectRoot, "project-root", "r", "", "Project root directory (default: current directory)")
}
