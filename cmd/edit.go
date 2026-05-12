package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/daily"
	"github.com/vieitesss/pad/internal/ghcli"
	"github.com/vieitesss/pad/internal/tui"
)

func newEditCmd() *cobra.Command {
	var date string
	var force bool

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing daily update issue on GitHub",
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

			template, err := loadIssueTemplate(ctx, env)
			if err != nil {
				return err
			}

			issue, err := tui.RunWithSpinner(ctx, fmt.Sprintf("Fetching daily update for %s", resolvedDate), func(ctx context.Context) (ghcli.DailyUpdateIssue, error) {
				return env.gh.FindDailyUpdateIssueByDate(ctx, env.cfg.GitHubRepo, env.cfg.Labels, resolvedDate)
			})
			if err != nil {
				if errors.Is(err, ghcli.ErrIssueNotFound) {
					return fmt.Errorf("no daily update issue found for %s", resolvedDate)
				}

				return err
			}

			if strings.EqualFold(issue.State, "closed") && !force {
				return fmt.Errorf(
					"issue #%d is already closed — it may have been merged into a report\nUse --force to edit it anyway (the published report will not be updated)",
					issue.Number,
				)
			}

			entry := daily.EntryFromIssueBody(resolvedDate, template, issue.Body)
			entry.Source = fmt.Sprintf("edit:#%d", issue.Number)

			entry, err = tui.Edit(entry)
			if err != nil {
				if errors.Is(err, tui.ErrCanceled) {
					fmt.Fprintln(cmd.OutOrStdout(), "edit canceled")
					return nil
				}

				return err
			}

			if err := updateIssueFromEntry(ctx, env, issue.Number, entry); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), issue.URL)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Entry date in YYYY-MM-DD format (default: today)")
	cmd.Flags().BoolVar(&force, "force", false, "Edit the issue even if it is already closed")

	return cmd
}
