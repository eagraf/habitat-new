package main

import (
	"fmt"
	"os"

	"github.com/eagraf/habitat-new/cmd/pac/logging"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

const version = "v0.0.1"

var (
	debug bool
)

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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(typegenCmd)

	// Add flags
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
}

func main() {
	// Initialize the logger first
	logging.Init()

	// Set up pre-run hook to handle debug flag
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if debug {
			logging.InitWithLevel(zerolog.DebugLevel)
		}
	}

	if err := rootCmd.Execute(); err != nil {
		logging.Error(err.Error())
		os.Exit(1)
	}
}
