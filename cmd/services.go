package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/XBS-Nathan/apex-flow-dev-cli/internal/config"
	"github.com/XBS-Nathan/apex-flow-dev-cli/internal/docker"
)

func init() {
	rootCmd.AddCommand(servicesCmd)
	servicesCmd.AddCommand(servicesUpCmd)
	servicesCmd.AddCommand(servicesDownCmd)
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage shared Docker services",
}

var servicesUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start shared services (MySQL, Redis, Typesense, etc.)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting shared services...")
		projectsDir := projectsDirOrHome()
		if err := docker.Up(projectsDir, []string{config.DefaultPHP}); err != nil {
			return err
		}
		fmt.Println("✓ Services running")
		return nil
	},
}

var servicesDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop shared services",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stopping shared services...")
		if err := docker.Down(); err != nil {
			return err
		}
		fmt.Println("✓ Services stopped")
		return nil
	},
}

// projectsDirOrHome returns the current working directory or falls back to the user's home directory.
func projectsDirOrHome() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}
