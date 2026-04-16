package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/prefapp/pad/internal/version"
	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "pad",
		Short:         "Async daily standup helper",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			showUpdateNotice()
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newRepeatCmd())
	rootCmd.AddCommand(newShowCmd())
	rootCmd.AddCommand(newCreateCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newUpgradeCmd())

	return rootCmd
}

func showUpdateNotice() {
	release, hasUpdate := version.CheckUpdate()
	if !hasUpdate {
		return
	}

	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE"))

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(
		os.Stderr,
		"%s %s is available. Run %s to update.\n",
		yellow.Render("→"),
		cyan.Render(release.TagName),
		cyan.Render("pad upgrade"),
	)
}
