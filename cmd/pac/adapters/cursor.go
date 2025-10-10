package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CursorAdapter implements the Agent interface for Cursor CLI
type CursorAdapter struct{}

// NewCursorAdapter creates a new CursorAdapter instance
func NewCursorAdapter() *CursorAdapter {
	return &CursorAdapter{}
}

// GetVersion returns the cursor version string
// Returns ErrNotInstalled if cursor is not available
func (c *CursorAdapter) GetVersion() (string, error) {
	cmd := exec.Command("cursor", "--version")
	output, err := cmd.Output()
	if err != nil {
		// Check if error is due to cursor not being found
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return "", ErrNotInstalled
		}
		// Some other error occurred
		return "", fmt.Errorf("error running cursor: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// GetInstallInstructions returns instructions for installing cursor CLI
func (c *CursorAdapter) GetInstallInstructions() string {
	return `Cursor CLI is not installed. Please install it using the following steps:

1. Install Cursor from https://cursor.sh/

2. Open Cursor and enable the CLI by going to:
   - macOS/Linux: Command Palette (Cmd/Ctrl+Shift+P) → "Shell Command: Install 'cursor' command in PATH"
   - Or add Cursor to your PATH manually

3. Verify installation by running:
   cursor --version

For more information, visit https://docs.cursor.sh/`
}

// Prompt executes a cursor agent prompt in the specified working directory
// Returns ErrNotInstalled if cursor is not available
func (c *CursorAdapter) Prompt(workingDir string, prompt string) error {
	// First check if cursor is installed
	if _, err := c.GetVersion(); err != nil {
		if err == ErrNotInstalled {
			return ErrNotInstalled
		}
		return fmt.Errorf("failed to verify cursor installation: %w", err)
	}

	// Run cursor agent prompt command
	cmd := exec.Command("cursor", "agent", "prompt", prompt)
	cmd.Dir = workingDir

	// Set output to go to stdout and stderr so user can see the agent's output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run cursor agent prompt: %w", err)
	}

	return nil
}
