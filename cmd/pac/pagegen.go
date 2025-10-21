package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/eagraf/habitat-new/cmd/pac/adapters"
	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	pagegenProjectRoot string
	pagegenAgent       string
	pagegenForce       bool
	pagegenResume      string
)

// PageGenConfig holds the configuration for generating a page
type PageGenConfig struct {
	Description       string   `json:"description"`
	AvailableLexicons []string `json:"available_lexicons"`

	// Map Lexicon NSIDs to configs containing permitted operations and descriptions of how they will be used
	LexiconConfigs map[string]LexiconConfig `json:"lexicon_configs"`

	// Route relative to the root of the website
	Route string `json:"route"`

	// Map query params to their descriptions
	QueryParams map[string]string `json:"query_params"`
}

type LexOp string

const (
	LexOpCreateRecord LexOp = "createRecord"
	LexOpListRecords  LexOp = "listRecords"
	LexOpGetRecord    LexOp = "getRecord"
	LexOpPutRecord    LexOp = "putRecord"
	LexOpDeleteRecord LexOp = "deleteRecord"

	// Private operations
	LexOpCreatePrivateRecord LexOp = "createPrivateRecord"
	LexOpGetPrivateRecord    LexOp = "getPrivateRecord"
	LexOpListPrivateRecords  LexOp = "listPrivateRecords"
	LexOpPutPrivateRecord    LexOp = "putPrivateRecord"
	LexOpDeletePrivateRecord LexOp = "deletePrivateRecord"
)

// LexiconConfig defines how a particular page will interact with a lexicon
type LexiconConfig struct {
	LexiconID string `json:"lexicon_id"`

	// Maps permitted operations to a description of how they will be used
	Operations map[LexOp]string `json:"operations"`
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

		var config *PageGenConfig

		// Check if we're resuming from a saved config
		if pagegenResume != "" {
			logging.Infof("Loading configuration from: %s", pagegenResume)
			var err error
			config, err = loadConfigFromFile(pagegenResume)
			if err != nil {
				logging.CheckErr(fmt.Errorf("Failed to load configuration from file: %w", err))
				os.Exit(1)
			}
			logging.Success("Configuration loaded successfully!")
		} else {
			// Run the interactive page generation flow
			var err error
			config, err = runPageGenInteractive(absRoot, pagegenForce)
			if err != nil {
				logging.CheckErr(fmt.Errorf("Failed to gather page generation requirements: %w", err))
				os.Exit(1)
			}

			// Save the config to a backup file
			configPath, err := saveConfigToFile(config, absRoot)
			if err != nil {
				logging.Warnf("Failed to save configuration backup: %v", err)
			} else {
				logging.Infof("Configuration saved to: %s", configPath)
			}
		}

		// Generate the prompt for the coding agent
		prompt := generateAgentPrompt(config)

		logging.Success("Page generation configuration completed!")

		// Initialize the agent
		agent, err := getAgent(pagegenAgent)
		if err != nil {
			logging.CheckErr(fmt.Errorf("Failed to initialize agent: %w", err))
			os.Exit(1)
		}

		// Check if the agent is installed
		version, err := agent.GetVersion()
		if err == adapters.ErrNotInstalled {
			logging.Errorf("Agent is not installed:\n%s", agent.GetInstallInstructions())
			os.Exit(1)
		} else if err != nil {
			logging.CheckErr(fmt.Errorf("Failed to check agent version: %w", err))
			os.Exit(1)
		}

		logging.Infof("Using agent: %s (version: %s)", pagegenAgent, version)
		logging.Infof("\n=== Generated Prompt ===\n%s\n", prompt)

		// Call the agent to generate the page
		logging.Info("Calling agent to generate page...")
		if err := agent.Prompt(absRoot, prompt); err != nil {
			logging.CheckErr(fmt.Errorf("Failed to run agent: %w", err))
			os.Exit(1)
		}

		logging.Success("Page generation completed!")
	},
}

// runPageGenInteractive runs the interactive CLI flow to gather page generation requirements
func runPageGenInteractive(projectRoot string, force bool) (*PageGenConfig, error) {
	config := &PageGenConfig{
		LexiconConfigs: make(map[string]LexiconConfig),
		QueryParams:    make(map[string]string),
	}

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

	// Step 2: Get page route
	route, err := promptForRoute()
	if err != nil {
		return nil, fmt.Errorf("failed to get page route: %w", err)
	}
	config.Route = route

	// Check if the target file already exists (immediately after getting route)
	filename := routeToFilename(route)
	targetFile := filepath.Join(projectRoot, "src", "routes", fmt.Sprintf("%s.lazy.tsx", filename))

	if _, err := os.Stat(targetFile); err == nil {
		// File exists
		if !force {
			return nil, fmt.Errorf("page file already exists: %s\nUse -f or --force flag to overwrite the existing file", targetFile)
		}
		logging.Warnf("Page file already exists: %s (will be overwritten)", targetFile)
		fmt.Println()
	}

	// Step 3: Select lexicons and configure operations
	selectedLexicons, err := promptForLexicons("Select lexicons this page will interact with", lexicons)
	if err != nil {
		return nil, fmt.Errorf("failed to select lexicons: %w", err)
	}

	// For each selected lexicon, configure operations
	for _, lexicon := range selectedLexicons {
		lexConfig, err := promptForLexiconConfig(lexicon)
		if err != nil {
			return nil, fmt.Errorf("failed to configure lexicon %s: %w", lexicon, err)
		}
		config.LexiconConfigs[lexicon] = lexConfig
	}

	// Step 4: Configure query parameters
	queryParams, err := promptForQueryParams()
	if err != nil {
		return nil, fmt.Errorf("failed to configure query params: %w", err)
	}
	config.QueryParams = queryParams

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

// promptForRoute prompts the user for the page route
func promptForRoute() (string, error) {
	prompt := promptui.Prompt{
		Label: "Enter the page route (e.g., /dashboard, /users/profile)",
		Validate: func(input string) error {
			input = strings.TrimSpace(input)
			if input == "" {
				return fmt.Errorf("route cannot be empty")
			}
			if !strings.HasPrefix(input, "/") {
				return fmt.Errorf("route must start with /")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
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

// promptForLexiconConfig prompts the user to configure operations for a lexicon
func promptForLexiconConfig(lexiconID string) (LexiconConfig, error) {
	config := LexiconConfig{
		LexiconID:  lexiconID,
		Operations: make(map[LexOp]string),
	}

	logging.Infof("\nConfiguring operations for lexicon: %s", lexiconID)

	// List of available operations
	availableOps := []LexOp{
		LexOpCreateRecord,
		LexOpListRecords,
		LexOpGetRecord,
		LexOpPutRecord,
		LexOpDeleteRecord,
		LexOpCreatePrivateRecord,
		LexOpGetPrivateRecord,
		LexOpListPrivateRecords,
		LexOpPutPrivateRecord,
		LexOpDeletePrivateRecord,
	}

	// Track which operations are already added
	selectedOps := make(map[LexOp]bool)

	for {
		// Ask if user wants to add an operation
		confirmPrompt := promptui.Select{
			Label: "Add an operation for this lexicon?",
			Items: []string{"Yes", "No - Continue to next lexicon"},
		}

		idx, _, err := confirmPrompt.Run()
		if err != nil {
			return config, err
		}

		// If user selected "No", break
		if idx == 1 {
			break
		}

		// Filter out already selected operations
		availableToSelect := make([]LexOp, 0)
		for _, op := range availableOps {
			if !selectedOps[op] {
				availableToSelect = append(availableToSelect, op)
			}
		}

		if len(availableToSelect) == 0 {
			logging.Info("All operations have been added")
			break
		}

		// Select an operation
		opItems := make([]string, len(availableToSelect))
		for i, op := range availableToSelect {
			opItems[i] = string(op)
		}

		opPrompt := promptui.Select{
			Label: "Select operation to add",
			Items: opItems,
			Size:  12,
		}

		opIdx, _, err := opPrompt.Run()
		if err != nil {
			return config, err
		}

		selectedOp := availableToSelect[opIdx]

		// Get description immediately
		description, err := promptForOperationDescription(selectedOp)
		if err != nil {
			return config, fmt.Errorf("failed to get description for %s: %w", selectedOp, err)
		}

		config.Operations[selectedOp] = description
		selectedOps[selectedOp] = true
		logging.Infof("Added operation: %s", selectedOp)
	}

	return config, nil
}

// promptForOperationDescription prompts for a description of how an operation will be used
func promptForOperationDescription(op LexOp) (string, error) {
	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Describe how '%s' will be used in this page", op),
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("description cannot be empty")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

// promptForQueryParams prompts the user to add query parameters with descriptions
func promptForQueryParams() (map[string]string, error) {
	params := make(map[string]string)

	logging.Info("\nConfiguring query parameters (optional)")

	for {
		// Ask if user wants to add a query param
		confirmPrompt := promptui.Select{
			Label: "Add a query parameter?",
			Items: []string{"Yes", "No - Skip to next step"},
		}

		idx, _, err := confirmPrompt.Run()
		if err != nil {
			return nil, err
		}

		// If user selected "No", break
		if idx == 1 {
			break
		}

		// Prompt for param name
		namePrompt := promptui.Prompt{
			Label: "Query parameter name",
			Validate: func(input string) error {
				input = strings.TrimSpace(input)
				if input == "" {
					return fmt.Errorf("parameter name cannot be empty")
				}
				if _, exists := params[input]; exists {
					return fmt.Errorf("parameter '%s' already exists", input)
				}
				return nil
			},
		}

		paramName, err := namePrompt.Run()
		if err != nil {
			return nil, err
		}
		paramName = strings.TrimSpace(paramName)

		// Prompt for param description
		descPrompt := promptui.Prompt{
			Label: fmt.Sprintf("Description for '%s'", paramName),
			Validate: func(input string) error {
				if strings.TrimSpace(input) == "" {
					return fmt.Errorf("description cannot be empty")
				}
				return nil
			},
		}

		description, err := descPrompt.Run()
		if err != nil {
			return nil, err
		}

		params[paramName] = strings.TrimSpace(description)
		logging.Infof("Added query parameter: %s", paramName)
	}

	return params, nil
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

// routeToFilename converts a route path to a valid filename
// e.g., "/dashboard" -> "dashboard", "/users/profile" -> "users.profile"
func routeToFilename(route string) string {
	// Remove leading slash
	filename := strings.TrimPrefix(route, "/")

	// Replace slashes with dots for nested routes
	filename = strings.ReplaceAll(filename, "/", ".")

	// Handle root path
	if filename == "" {
		filename = "index"
	}

	return filename
}

// generateAgentPrompt generates the prompt that will be passed to the coding agent
func generateAgentPrompt(config *PageGenConfig) string {
	var sb strings.Builder

	sb.WriteString("Generate a new React page with the following specifications:\n\n")

	sb.WriteString("## Page Description\n")
	sb.WriteString(config.Description)
	sb.WriteString("\n\n")

	// Generate filename from route
	filename := routeToFilename(config.Route)
	filepath := fmt.Sprintf("src/routes/%s.lazy.tsx", filename)

	sb.WriteString("## Page Route\n")
	sb.WriteString(fmt.Sprintf("The page should be accessible at: %s\n", config.Route))
	sb.WriteString(fmt.Sprintf("Create a new lazy-loaded route file at: **%s**\n\n", filepath))

	// Query Parameters
	if len(config.QueryParams) > 0 {
		sb.WriteString("## Query Parameters\n")
		sb.WriteString("This page accepts the following query parameters:\n")
		for param, desc := range config.QueryParams {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", param, desc))
		}
		sb.WriteString("\n")
	}

	// Lexicon Configurations
	if len(config.LexiconConfigs) > 0 {
		sb.WriteString("## Data Access Requirements\n\n")
		sb.WriteString("This page will interact with the following lexicons/resources:\n\n")

		for lexID, lexConfig := range config.LexiconConfigs {
			sb.WriteString(fmt.Sprintf("### %s\n", lexID))
			sb.WriteString(fmt.Sprintf("- Types: src/types/%s_types.ts\n", lexID))
			sb.WriteString(fmt.Sprintf("- API Client: src/api/%s_client.ts\n\n", lexID))

			if len(lexConfig.Operations) > 0 {
				sb.WriteString("**Operations to use:**\n")
				for op, desc := range lexConfig.Operations {
					sb.WriteString(fmt.Sprintf("- **%s**: %s\n", op, desc))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("## Implementation Guidelines\n")
	sb.WriteString("1. Import the necessary type definitions from src/types/\n")
	sb.WriteString("2. Import the necessary API clients from src/api/\n")
	sb.WriteString("3. Use the `useHabitatClient` hook from src/sdk/HabitatClientProvider to access the HabitatClient instance\n")
	sb.WriteString("4. Pass the client instance to all API client functions (they accept client as their first parameter)\n")
	sb.WriteString("5. Use TanStack Query (useQuery, useMutation) for data fetching and mutations\n")
	sb.WriteString("6. Use TanStack Router's useSearch() hook to access query parameters if applicable\n")
	sb.WriteString("7. Follow the existing patterns in the codebase for styling and component structure\n")
	sb.WriteString("8. Ensure the page is responsive and follows modern UX best practices\n")
	sb.WriteString("9. Add proper error handling and loading states\n")
	sb.WriteString("10. Use TypeScript for type safety\n")
	sb.WriteString("11. Implement the specific operations described for each lexicon\n")
	sb.WriteString("12. Respect the system light/dark mode\n")
	sb.WriteString("13. Do not create example data. Only use live data queried using the HabitatClient\n")

	return sb.String()
}

// saveConfigToFile saves the PageGenConfig to a JSON file
func saveConfigToFile(config *PageGenConfig, projectRoot string) (string, error) {
	// Create a .pac directory if it doesn't exist
	pacDir := filepath.Join(projectRoot, ".pac")
	if err := os.MkdirAll(pacDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .pac directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("pagegen-%s.json", timestamp)
	configPath := filepath.Join(pacDir, filename)

	// Marshal config to JSON with pretty printing
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

// loadConfigFromFile loads a PageGenConfig from a JSON file
func loadConfigFromFile(filePath string) (*PageGenConfig, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal JSON
	var config PageGenConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config JSON: %w", err)
	}

	return &config, nil
}

// getAgent returns an Agent implementation based on the agent name
func getAgent(agentName string) (adapters.Agent, error) {
	switch strings.ToLower(agentName) {
	case "cursor":
		return adapters.NewCursorAdapter(), nil
	default:
		return nil, fmt.Errorf("unknown agent: %s (supported: cursor)", agentName)
	}
}

func init() {
	// Add project root flag
	pagegenCmd.Flags().StringVarP(&pagegenProjectRoot, "project-root", "r", "", "Project root directory (default: current directory)")

	// Add agent flag
	pagegenCmd.Flags().StringVarP(&pagegenAgent, "agent", "a", "cursor", "AI coding agent to use (supported: cursor)")

	// Add force flag
	pagegenCmd.Flags().BoolVarP(&pagegenForce, "force", "f", false, "Force overwrite if page file already exists")

	// Add resume flag
	pagegenCmd.Flags().StringVar(&pagegenResume, "resume", "", "Resume from a saved configuration file (path to JSON file)")
}
