package cmd

import "github.com/spf13/cobra"

func init() { rootCmd.AddCommand(yarnCmd) }

var yarnCmd = &cobra.Command{
	Use:                "yarn [args...]",
	Short:              "Run yarn in the project's Node container",
	DisableFlagParsing: true,
	Hidden:             true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNodeCommand(append([]string{"yarn"}, args...)...)
	},
}
