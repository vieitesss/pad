package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/prefapp/pad/internal/daily"
	"github.com/prefapp/pad/internal/ghcli"
	"github.com/prefapp/pad/internal/tui"
	"github.com/spf13/cobra"
)

func newRepeatCmd() *cobra.Command {
	var date string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "repeat",
		Short: "Prefill from your latest GitHub async daily issue and create a new one",
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

			if !dryRun {
				if err := ensureCanCreateForDate(ctx, env, resolvedDate); err != nil {
					return err
				}
			}

			latestIssue, err := env.gh.LatestAsyncDailyIssue(ctx, env.cfg.GitHubRepo, env.cfg.Labels)
			if err != nil {
				if errors.Is(err, ghcli.ErrIssueNotFound) {
					return fmt.Errorf("no previous async daily issues found for the authenticated user")
				}

				return err
			}

			entry := daily.EntryFromIssueBody(resolvedDate, latestIssue.Body)
			entry.Source = fmt.Sprintf("repeat:%s", latestIssue.Date)

			if dryRun {
				entry, err = tui.Edit(entry)
			} else {
				entry, err = tui.EditForCreate(entry)
			}
			if err != nil {
				if errors.Is(err, tui.ErrCanceled) {
					fmt.Fprintln(cmd.OutOrStdout(), "edit canceled")
					return nil
				}

				return err
			}

			if dryRun {
				title, err := entry.Title()
				if err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n%s\n", title, entry.Body())
				return nil
			}

			issue, err := createIssueFromEntry(ctx, env, entry)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), issue.URL)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Entry date in YYYY-MM-DD format")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Open the editor with the repeated content and print the title/body without creating a GitHub issue")

	return cmd
}
