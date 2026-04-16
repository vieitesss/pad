package cmd

import "github.com/spf13/cobra"

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "pad",
		Short:         "Async daily standup helper",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRepeatCmd())
	rootCmd.AddCommand(newShowCmd())
	rootCmd.AddCommand(newCreateCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newReportCmd())

	return rootCmd
}
