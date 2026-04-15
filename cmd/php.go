package cmd

import "github.com/spf13/cobra"

func init() { rootCmd.AddCommand(phpCmd) }

var phpCmd = &cobra.Command{
	Use:                "php [args...]",
	Short:              "Run php with the project's PHP version",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInContainer(append([]string{"php"}, args...)...)
	},
}
