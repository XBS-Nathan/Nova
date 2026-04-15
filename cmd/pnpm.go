package cmd

import "github.com/spf13/cobra"

func init() { rootCmd.AddCommand(pnpmCmd) }

var pnpmCmd = &cobra.Command{
	Use:                "pnpm [args...]",
	Short:              "Run pnpm in the project's Node container",
	DisableFlagParsing: true,
	Hidden:             true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNodeCommand(append([]string{"pnpm"}, args...)...)
	},
}
