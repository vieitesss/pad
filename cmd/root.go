package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/version"
)

func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "pad",
		Short:         "Daily update standup helper",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Don't show upgrade notice after running upgrade command
			if cmd.Name() == "upgrade" {
				return
			}
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
	release, hasUpdate := version.CheckUpdate(false)
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
