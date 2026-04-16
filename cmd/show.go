package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/ghcli"
)

func newShowCmd() *cobra.Command {
	var date string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print your daily update issue body from GitHub for a date",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := loadEnv()
			if err != nil {
				return err
			}

			resolvedDate, err := resolveDate(date)
			if err != nil {
				return err
			}

			ctx := context.Background()
			if err := env.gh.EnsureReady(ctx); err != nil {
				return err
			}

			issue, err := env.gh.FindDailyUpdateIssueByDate(ctx, env.cfg.GitHubRepo, env.cfg.Labels, resolvedDate)
			if err != nil {
				if errors.Is(err, ghcli.ErrIssueNotFound) {
					return fmt.Errorf("no remote daily update issue found for %s", resolvedDate)
				}

				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n\n%s\n", issue.Title, issue.URL, issue.Body)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Entry date in YYYY-MM-DD format")

	return cmd
}
