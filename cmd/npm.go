package cmd

import "github.com/spf13/cobra"

func init() { rootCmd.AddCommand(npmCmd) }

var npmCmd = &cobra.Command{
	Use:                "npm [args...]",
	Short:              "Run npm in the project's Node container",
	DisableFlagParsing: true,
	Hidden:             true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNodeCommand(append([]string{"npm"}, args...)...)
	},
}
