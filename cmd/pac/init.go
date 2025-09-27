package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/eagraf/habitat-new/cmd/pac/adapters"
	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/spf13/cobra"
)

var (
	projectDir string
	reset      bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new pac project",
	Long:  `Initialize a new pac project in the current directory or specified directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Determine the project directory
		var targetDir string

		if projectDir != "" {
			// Use the specified directory
			targetDir = projectDir
		} else {
			// Use current directory
			var err error
			targetDir, err = os.Getwd()
			logging.CheckErr(err)
		}

		// Resolve the absolute path
		absPath, err := filepath.Abs(targetDir)
		logging.CheckErr(err)

		logging.Infof("Preparing project directory: %s", targetDir)
		err = prepareDir(absPath, reset)
		logging.CheckErr(err)

		logging.Infof("Initializing project in: %s", absPath)
		err = initProject(absPath)
		logging.CheckErr(err)
	},
}

func isDirEmpty(targetDir string) (bool, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func prepareDir(targetDir string, reset bool) error {
	if !reset {
		isEmpty, err := isDirEmpty(targetDir)
		if err != nil {
			return err
		}
		if !isEmpty {
			return errors.New("Directory already exists and is not empty")
		}
	} else {
		err := os.RemoveAll(targetDir)
		if err != nil {
			return err
		}
	}

	// Create the project directory
	err := os.MkdirAll(targetDir, 0755)
	return err
}

func initProject(targetDir string) error {
	logging.Debug("Starting project initialization")

	// Check if git is installed
	gitAdapter := adapters.NewGitAdapter()

	version, err := gitAdapter.GetVersion()
	if err != nil {
		logging.Error("Git is not installed")
		logging.Info(gitAdapter.GetInstallInstructions())
		return err
	}
	logging.Debugf("Using git version: %s", version)

	frontendDir := filepath.Join(targetDir, "frontend")

	// Create the frontend directory
	logging.Debugf("Creating frontend directory: %s", frontendDir)
	err = os.MkdirAll(frontendDir, 0755)
	if err != nil {
		return err
	}

	// Clone the frontend template
	logging.Infof("Cloning frontend template to: %s", frontendDir)
	err = gitAdapter.Clone("git@github.com:eagraf/habitat-frontend-template.git", frontendDir)
	if err != nil {
		return err
	}

	// Remove the .git directory
	logging.Debug("Removing .git directory from cloned template")
	err = os.RemoveAll(filepath.Join(frontendDir, ".git"))
	if err != nil {
		return err
	}

	logging.Success("Project initialized successfully!")
	return nil
}

func init() {
	// Add directory flag
	initCmd.Flags().StringVarP(&projectDir, "directory", "d", "", "Directory where the project should be created")
	initCmd.Flags().BoolVarP(&reset, "reset", "r", false, "Reset the project directory")

	// This function will be called when the package is imported
	// The init command will be registered in main.go
}
