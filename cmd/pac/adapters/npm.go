package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// NpmAdapter implements the Adapter interface for npm
type NpmAdapter struct{}

// NewNpmAdapter creates a new NpmAdapter instance
func NewNpmAdapter() *NpmAdapter {
	return &NpmAdapter{}
}

// GetVersion returns the npm version string
// Returns ErrNotInstalled if npm is not available
func (n *NpmAdapter) GetVersion() (string, error) {
	cmd := exec.Command("npm", "--version")
	output, err := cmd.Output()
	if err != nil {
		// Check if error is due to npm not being found
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return "", ErrNotInstalled
		}
		// Some other error occurred
		return "", fmt.Errorf("error running npm: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// GetInstallInstructions returns instructions for installing npm
func (n *NpmAdapter) GetInstallInstructions() string {
	return `npm is not installed. Please install it using one of the following methods:

macOS (using Homebrew):
  brew install node

Ubuntu/Debian:
  curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
  sudo apt-get install -y nodejs

CentOS/RHEL:
  curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash -
  sudo yum install -y nodejs

Windows:
  Download from https://nodejs.org/

Or visit https://nodejs.org/ for more installation options.`
}

func (n *NpmAdapter) install(targetDir string) (*exec.Cmd, error) {
	cmd := exec.Command("npm", "install")
	cmd.Dir = targetDir
	return cmd, nil
}

// Install runs npm install in the specified directory
// Returns ErrNotInstalled if npm is not available
func (n *NpmAdapter) Install(targetDir string) error {
	cmd, err := n.install(targetDir)
	if err != nil {
		return err
	}
	cmd.Dir = targetDir

	// Set output to go to stdout and stderr so user can see progress
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run npm install: %s", err)
	}

	return nil
}
