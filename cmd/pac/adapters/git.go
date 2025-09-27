package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitAdapter implements the Adapter interface for Git
type GitAdapter struct{}

// NewGitAdapter creates a new GitAdapter instance
func NewGitAdapter() *GitAdapter {
	return &GitAdapter{}
}

// GetVersion returns the git version string
// Returns ErrNotInstalled if git is not available
func (g *GitAdapter) GetVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		// Check if error is due to git not being found
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return "", ErrNotInstalled
		}
		// Some other error occurred
		return "", fmt.Errorf("error running git: %w", err)
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

// GetInstallInstructions returns instructions for installing git
func (g *GitAdapter) GetInstallInstructions() string {
	return `Git is not installed. Please install it using one of the following methods:

macOS (using Homebrew):
  brew install git

Ubuntu/Debian:
  sudo apt update && sudo apt install git

CentOS/RHEL:
  sudo yum install git

Windows:
  Download from https://git-scm.com/download/win

Or visit https://git-scm.com/downloads for more installation options.`
}

// Clone creates a git clone command with --depth 1
// Returns ErrNotInstalled if git is not available
func (g *GitAdapter) clone(repoURL, targetDir string) (*exec.Cmd, error) {
	// Check if git is installed by trying to get version
	_, err := g.GetVersion()
	if err != nil {
		return nil, fmt.Errorf("git is not installed: %s", g.GetInstallInstructions())
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, targetDir)
	return cmd, nil
}

func (g *GitAdapter) Clone(repoURL, targetDir string) error {
	cmd, err := g.clone(repoURL, targetDir)
	if err != nil {
		return err
	}

	// Set output to go to stdout and stderr so user can see progress
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s", err)
	}

	return nil
}
