package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/prefapp/pad/internal/daily"
	"github.com/prefapp/pad/internal/tui"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var date string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Open the async daily editor and create the GitHub issue",
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
			if !dryRun {
				if err := ensureCanCreateForDate(ctx, env, resolvedDate); err != nil {
					return err
				}
			}

			entry := daily.New(resolvedDate)

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

			if err := entry.ValidateForCreate(); err != nil {
				return err
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Open the editor and print the title/body without creating a GitHub issue")

	return cmd
}
