package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	pagegenProjectRoot string
)

// PageGenConfig holds the configuration for generating a page
type PageGenConfig struct {
	Description       string
	ReadLexicons      []string
	WriteLexicons     []string
	AvailableLexicons []string
}

var pagegenCmd = &cobra.Command{
	Use:   "pagegen",
	Short: "Generate a new page for the application",
	Long:  `Generate a new page using a coding agent. This command will gather requirements interactively and generate a prompt for the coding agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Determine project root
		root := pagegenProjectRoot
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

		// Run the interactive page generation flow
		config, err := runPageGenInteractive(absRoot)
		if err != nil {
			logging.CheckErr(fmt.Errorf("Failed to gather page generation requirements: %w", err))
			os.Exit(1)
		}

		// Generate the prompt for the coding agent
		prompt := generateAgentPrompt(config)

		// TODO: This is where we would call the coding agent with the generated prompt
		// For now, just display the prompt
		logging.Success("Page generation configuration completed!")
		logging.Infof("\n%s", prompt)
	},
}

// runPageGenInteractive runs the interactive CLI flow to gather page generation requirements
func runPageGenInteractive(projectRoot string) (*PageGenConfig, error) {
	config := &PageGenConfig{}

	// Discover available lexicons from the types directory
	lexicons, err := discoverLexicons(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to discover lexicons: %w", err)
	}

	if len(lexicons) == 0 {
		return nil, fmt.Errorf("no lexicons found in %s/src/types. Run 'pac clientgen' to generate types first", projectRoot)
	}

	config.AvailableLexicons = lexicons

	logging.Infof("Found %d available lexicons: %s", len(lexicons), strings.Join(lexicons, ", "))
	fmt.Println()

	// Step 1: Get page description
	description, err := promptForDescription()
	if err != nil {
		return nil, fmt.Errorf("failed to get page description: %w", err)
	}
	config.Description = description

	// Step 2: Select lexicons for read access
	readLexicons, err := promptForLexicons("Select lexicons this page will have READ access to", lexicons)
	if err != nil {
		return nil, fmt.Errorf("failed to select read lexicons: %w", err)
	}
	config.ReadLexicons = readLexicons

	// Step 3: Select lexicons for write access
	writeLexicons, err := promptForLexicons("Select lexicons this page will have WRITE access to", lexicons)
	if err != nil {
		return nil, fmt.Errorf("failed to select write lexicons: %w", err)
	}
	config.WriteLexicons = writeLexicons

	return config, nil
}

// promptForDescription prompts the user for a description of the page using their default editor
func promptForDescription() (string, error) {
	// Ask user if they want to use an editor
	prompt := promptui.Select{
		Label: "How would you like to provide the page description?",
		Items: []string{
			"Open text editor for multi-line description",
			"Enter single-line description here",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return "", err
	}

	if idx == 0 {
		// Use text editor
		return promptWithEditor()
	}

	// Use simple prompt
	textPrompt := promptui.Prompt{
		Label: "Describe the page you want to generate",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("description cannot be empty")
			}
			return nil
		},
	}

	result, err := textPrompt.Run()
	if err != nil {
		return "", err
	}

	return result, nil
}

// promptWithEditor opens the user's default text editor for multi-line input
func promptWithEditor() (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "pagegen-description-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write instructions to the file
	instructions := `# Enter your page description below
# Lines starting with # will be ignored
# Save and close the editor when done

`
	if _, err := tmpFile.WriteString(instructions); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Determine which editor to use
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "vi", "nano", "emacs"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return "", fmt.Errorf("no text editor found. Please set the EDITOR environment variable")
	}

	logging.Infof("Opening %s for description input...", editor)

	// Open the editor
	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}

	// Read the file contents
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	// Process the content - remove comment lines and trim
	lines := strings.Split(string(content), "\n")
	var resultLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			resultLines = append(resultLines, line)
		}
	}

	result := strings.TrimSpace(strings.Join(resultLines, "\n"))
	if result == "" {
		return "", fmt.Errorf("description cannot be empty")
	}

	return result, nil
}

// promptForLexicons prompts the user to select lexicons from a list using a multi-select interface
func promptForLexicons(label string, lexicons []string) ([]string, error) {
	if len(lexicons) == 0 {
		logging.Info("No lexicons available")
		return []string{}, nil
	}

	// Create a map to track selected items
	selected := make(map[string]bool)

	// Use a loop to allow multiple selections
	for {
		// Create display items with checkboxes
		displayItems := make([]string, 0, len(lexicons)+1)
		for _, lex := range lexicons {
			checkbox := "[ ]"
			if selected[lex] {
				checkbox = "[✓]"
			}
			displayItems = append(displayItems, fmt.Sprintf("%s %s", checkbox, lex))
		}
		displayItems = append(displayItems, "Done - Continue to next step")

		prompt := promptui.Select{
			Label: label,
			Items: displayItems,
			Size:  10,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, err
		}

		// Check if user selected "Done"
		if idx == len(lexicons) {
			break
		}

		// Toggle selection
		lexicon := lexicons[idx]
		selected[lexicon] = !selected[lexicon]
	}

	// Convert selected map to slice
	result := make([]string, 0, len(selected))
	for lex, isSelected := range selected {
		if isSelected {
			result = append(result, lex)
		}
	}

	return result, nil
}

// discoverLexicons discovers available lexicons by examining the types directory
func discoverLexicons(projectRoot string) ([]string, error) {
	typesDir := filepath.Join(projectRoot, "src", "types")

	// Check if types directory exists
	if _, err := os.Stat(typesDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Find all *_types.ts files
	pattern := filepath.Join(typesDir, "*_types.ts")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob types directory: %w", err)
	}

	lexicons := make([]string, 0, len(matches))
	for _, match := range matches {
		// Extract the lexicon name from the filename
		base := filepath.Base(match)
		name := strings.TrimSuffix(base, "_types.ts")
		lexicons = append(lexicons, name)
	}

	return lexicons, nil
}

// generateAgentPrompt generates the prompt that will be passed to the coding agent
func generateAgentPrompt(config *PageGenConfig) string {
	var sb strings.Builder

	sb.WriteString("=== Page Generation Prompt ===\n\n")
	sb.WriteString("## Page Description\n")
	sb.WriteString(config.Description)
	sb.WriteString("\n\n")

	sb.WriteString("## Data Access Requirements\n\n")

	sb.WriteString("### Read Access\n")
	if len(config.ReadLexicons) == 0 {
		sb.WriteString("- None\n")
	} else {
		for _, lex := range config.ReadLexicons {
			sb.WriteString(fmt.Sprintf("- %s\n", lex))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("### Write Access\n")
	if len(config.WriteLexicons) == 0 {
		sb.WriteString("- None\n")
	} else {
		for _, lex := range config.WriteLexicons {
			sb.WriteString(fmt.Sprintf("- %s\n", lex))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Next Steps\n")
	sb.WriteString("TODO: This prompt will be passed to a coding agent to generate the page.\n")
	sb.WriteString("The agent will:\n")
	sb.WriteString("1. Create a new route file in src/routes/\n")
	sb.WriteString("2. Import the necessary type definitions and API clients\n")
	sb.WriteString("3. Implement the page component with the specified functionality\n")
	sb.WriteString("4. Use TanStack Query for data fetching where appropriate\n")

	return sb.String()
}

func init() {
	// Add project root flag
	pagegenCmd.Flags().StringVarP(&pagegenProjectRoot, "project-root", "r", "", "Project root directory (default: current directory)")
}
