package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/cmd/pac/gen"
	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/spf13/cobra"
)

var (
	outputFile string
)

// TypeGenerator defines the interface for generating TypeScript types from atproto schemas
type TypeGenerator interface {
	GenerateTypes(inputReader io.Reader) ([]byte, error)
}

var typegenCmd = &cobra.Command{
	Use:   "typegen <atprotofile>",
	Short: "Generate TypeScript types from atproto lexicon schemas",
	Long:  `Generate TypeScript type definitions from atproto lexicon schema files.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		atprotoFile := args[0]

		// Validate that the input file exists
		if _, err := os.Stat(atprotoFile); os.IsNotExist(err) {
			logging.CheckErr(fmt.Errorf("Input file does not exist: %s", atprotoFile))
			os.Exit(1)
		}

		// Determine output file path
		var outputPath string
		if outputFile != "" {
			outputPath = outputFile
		} else {
			// Use input filename with _types.ts appended
			inputBase := filepath.Base(atprotoFile)
			inputName := inputBase[:len(inputBase)-len(filepath.Ext(inputBase))]
			outputPath = filepath.Join(filepath.Dir(atprotoFile), inputName+"_types.ts")
		}

		// Ensure output directory exists
		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			logging.Error(fmt.Sprintf("Failed to create output directory: %v", err))
			os.Exit(1)
		}

		logging.Infof("Generating TypeScript types from: %s", atprotoFile)
		logging.Infof("Output file: %s", outputPath)

		// Use the real type generator from the gen package
		generator := gen.NewTypeScriptGenerator()
		err := generateTypesWithGenerator(atprotoFile, outputPath, generator)
		logging.CheckErr(err)

		logging.Success("TypeScript types generated successfully!")
	},
}

// generateTypesWithGenerator handles the file I/O and delegates type generation to the injected generator
func generateTypesWithGenerator(inputFile, outputFile string, generator TypeGenerator) error {
	logging.Debug("Reading atproto schema file")
	logging.Debugf("Input file: %s", inputFile)

	// Open the input file
	inputReader, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %v", err)
	}
	defer inputReader.Close()

	// Generate types using the injected generator
	logging.Debug("Generating TypeScript types")
	generatedContent, err := generator.GenerateTypes(inputReader)
	if err != nil {
		return fmt.Errorf("failed to generate types: %v", err)
	}

	// Write the generated content to the output file
	if err := os.WriteFile(outputFile, generatedContent, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	logging.Debug("Type generation completed")
	return nil
}

func init() {
	// Add output flag
	typegenCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path for generated TypeScript types")

	// This function will be called when the package is imported
	// The typegen command will be registered in main.go
}
