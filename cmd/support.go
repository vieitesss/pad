package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vieitesss/pad/internal/appfs"
	"github.com/vieitesss/pad/internal/config"
	"github.com/vieitesss/pad/internal/daily"
	"github.com/vieitesss/pad/internal/ghcli"
)

type commandEnv struct {
	cfg config.Config
	gh  *ghcli.Client
}

func loadEnv() (*commandEnv, error) {
	paths, err := appfs.Discover()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		return nil, err
	}

	if cfg.GitHubRepo == "" {
		return nil, fmt.Errorf("no repository configured; run `pad init --repo owner/repo`")
	}

	return &commandEnv{
		cfg: cfg,
		gh:  ghcli.New(),
	}, nil
}

func resolveDate(raw string) (string, error) {
	if raw == "" {
		return time.Now().Format(daily.DateLayout), nil
	}

	if _, err := time.Parse(daily.DateLayout, raw); err != nil {
		return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD", raw)
	}

	return raw, nil
}

func ensureCanCreateForDate(ctx context.Context, env *commandEnv, date string) error {
	if err := env.gh.EnsureReady(ctx); err != nil {
		return err
	}

	existingIssue, err := env.gh.FindDailyUpdateIssueByDate(ctx, env.cfg.GitHubRepo, env.cfg.Labels, date)
	if err == nil {
		return fmt.Errorf("daily update issue already exists for %s: %s", date, existingIssue.URL)
	}

	if errors.Is(err, ghcli.ErrIssueNotFound) {
		return nil
	}

	return fmt.Errorf("check existing GitHub issues: %w", err)
}

func createIssueFromEntry(ctx context.Context, env *commandEnv, entry daily.Entry) (daily.IssueRef, error) {
	title, err := entry.Title()
	if err != nil {
		return daily.IssueRef{}, err
	}

	return env.gh.CreateIssue(ctx, env.cfg.GitHubRepo, title, entry.Body(), env.cfg.Labels)
}
