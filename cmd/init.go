package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/appfs"
	"github.com/vieitesss/pad/internal/config"
)

func newInitCmd() *cobra.Command {
	var repo string
	var labels []string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create or update local pad config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := appfs.Discover()
			if err != nil {
				return err
			}

			cfg, err := config.Load(paths.ConfigFile)
			if err != nil {
				return err
			}

			cfg.GitHubRepo = repo
			cfg.Labels = labels

			if err := config.Save(paths.ConfigFile, cfg); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "saved config: %s\n", paths.ConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "repo: %s\n", cfg.GitHubRepo)
			fmt.Fprintf(cmd.OutOrStdout(), "labels: %v\n", cfg.Labels)
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repository for daily update issues (required)")
	cmd.Flags().StringSliceVar(&labels, "labels", nil, "Labels to apply when creating issues (can be specified multiple times)")

	return cmd
}
