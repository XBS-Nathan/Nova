package cmd

import "github.com/spf13/cobra"

func init() { rootCmd.AddCommand(composerCmd) }

var composerCmd = &cobra.Command{
	Use:                "composer [args...]",
	Short:              "Run composer with the project's PHP version",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInContainer(append([]string{"composer"}, args...)...)
	},
}
