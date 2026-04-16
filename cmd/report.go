package cmd

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vieitesss/pad/internal/ghcli"
)

func newReportCmd() *cobra.Command {
	var date string
	var list bool
	var limit int

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show the merged daily update team report issue",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := loadEnv()
			if err != nil {
				return err
			}

			ctx := context.Background()
			if err := env.gh.EnsureReady(ctx); err != nil {
				return err
			}

			if list {
				issues, err := env.gh.ListReportIssues(ctx, env.cfg.GitHubRepo, limit)
				if err != nil {
					return err
				}

				if len(issues) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no daily update report issues found")
					return nil
				}

				writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				fmt.Fprintln(writer, "DATE\tNUMBER\tSTATE\tURL")
				for _, issue := range issues {
					fmt.Fprintf(writer, "%s\t#%d\t%s\t%s\n", issue.Date, issue.Number, issue.State, issue.URL)
				}

				return writer.Flush()
			}

			resolvedDate, err := resolveDate(date)
			if err != nil {
				return err
			}

			issue, err := env.gh.FindReportIssueByDate(ctx, env.cfg.GitHubRepo, resolvedDate)
			if err != nil {
				if errors.Is(err, ghcli.ErrIssueNotFound) {
					return fmt.Errorf("no daily update report issue found for %s", resolvedDate)
				}

				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n\n%s\n", issue.Title, issue.URL, issue.Body)
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Report date in YYYY-MM-DD format")
	cmd.Flags().BoolVar(&list, "list", false, "List recent report issues instead of showing one")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of report issues to show with --list")

	return cmd
}
