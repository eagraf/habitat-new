package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "v0.0.1"

var rootCmd = &cobra.Command{
	Use:   "pac",
	Short: "pac - A CLI tool for prototyping Habitat apps",
	Long:  `pac is a CLI tool for prototyping Habitat apps.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if version flag was set
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			fmt.Println(version)
			return
		}

		// Default behavior when no subcommand is provided
		fmt.Println("pac - Habitat apps generation CLI")
		fmt.Println("Use 'pac --help' for available commands")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of pac",
	Long:  `Print the version number of pac`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Add version flag to root command using Cobra's native flag system
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
