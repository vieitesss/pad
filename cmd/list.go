package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List async daily issues created by you",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := loadEnv()
			if err != nil {
				return err
			}

			ctx := context.Background()
			if err := env.gh.EnsureReady(ctx); err != nil {
				return err
			}

			issues, err := env.gh.ListAsyncDailyIssues(ctx, env.cfg.GitHubRepo, env.cfg.Labels, limit)
			if err != nil {
				return err
			}

			if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no remote async daily issues found for the authenticated user")
				return nil
			}

			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "DATE\tNUMBER\tSTATE\tURL")
			for _, issue := range issues {
				fmt.Fprintf(writer, "%s\t#%d\t%s\t%s\n", issue.Date, issue.Number, issue.State, issue.URL)
			}

			return writer.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of entries to show")

	return cmd
}
