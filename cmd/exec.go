package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() { rootCmd.AddCommand(execCmd) }

var execCmd = &cobra.Command{
	Use:                "exec [command...]",
	Short:              "Run a command in the project's PHP container",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: dev exec <command> [args...]")
		}
		return runInContainer(args...)
	},
}
