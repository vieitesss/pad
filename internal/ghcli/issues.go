package ghcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/prefapp/pad/internal/daily"
)

var ErrIssueNotFound = errors.New("issue not found")

type runner func(ctx context.Context, args ...string) ([]byte, error)

type Client struct {
	run runner
}

type AsyncDailyIssue struct {
	Number    int
	Title     string
	Body      string
	URL       string
	State     string
	Date      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReportIssue = AsyncDailyIssue

type issueListItem struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type issueViewItem struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	State     string `json:"state"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func New() *Client {
	return &Client{run: runGH}
}

func newForTests(run runner) *Client {
	return &Client{run: run}
}

func (c *Client) EnsureReady(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("`gh` not found in PATH")
	}

	output, err := c.run(ctx, "auth", "status")
	if err != nil {
		return fmt.Errorf("GitHub auth is not ready: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

func (c *Client) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (daily.IssueRef, error) {
	args := []string{"issue", "create", "--repo", repo, "--title", title, "--body", body}
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}

		args = append(args, "--label", label)
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return daily.IssueRef{}, fmt.Errorf("create GitHub issue: %s", strings.TrimSpace(string(output)))
	}

	issueURL := strings.TrimSpace(string(output))
	issueNumber, err := issueNumberFromURL(issueURL)
	if err != nil {
		return daily.IssueRef{}, err
	}

	return daily.IssueRef{Number: issueNumber, URL: issueURL}, nil
}

func (c *Client) ListAsyncDailyIssues(ctx context.Context, repo string, labels []string, limit int) ([]AsyncDailyIssue, error) {
	return c.searchIssues(ctx, repo, "@me", labels, limit, "")
}

func (c *Client) FindAsyncDailyIssueByDate(ctx context.Context, repo string, labels []string, date string) (AsyncDailyIssue, error) {
	title, err := daily.TitleForDate(date)
	if err != nil {
		return AsyncDailyIssue{}, err
	}

	issues, err := c.searchIssues(ctx, repo, "@me", labels, 5, fmt.Sprintf("%q in:title", title))
	if err != nil {
		return AsyncDailyIssue{}, err
	}

	if len(issues) == 0 {
		issues, err = c.searchIssues(ctx, repo, "@me", labels, 10, fmt.Sprintf("created:%s", date))
		if err != nil {
			return AsyncDailyIssue{}, err
		}
	}

	for _, issue := range issues {
		if issue.Date != date {
			continue
		}

		return c.ViewIssue(ctx, repo, issue.Number)
	}

	return AsyncDailyIssue{}, ErrIssueNotFound
}

func (c *Client) LatestAsyncDailyIssue(ctx context.Context, repo string, labels []string) (AsyncDailyIssue, error) {
	issues, err := c.ListAsyncDailyIssues(ctx, repo, labels, 1)
	if err != nil {
		return AsyncDailyIssue{}, err
	}

	if len(issues) == 0 {
		return AsyncDailyIssue{}, ErrIssueNotFound
	}

	return c.ViewIssue(ctx, repo, issues[0].Number)
}

func (c *Client) ListReportIssues(ctx context.Context, repo string, limit int) ([]ReportIssue, error) {
	return c.searchIssues(ctx, repo, "", []string{"async-daily/report"}, limit, "")
}

func (c *Client) FindReportIssueByDate(ctx context.Context, repo string, date string) (ReportIssue, error) {
	title, err := daily.ReportTitleForDate(date)
	if err != nil {
		return ReportIssue{}, err
	}

	issues, err := c.searchIssues(ctx, repo, "", []string{"async-daily/report"}, 5, fmt.Sprintf("%q in:title", title))
	if err != nil {
		return ReportIssue{}, err
	}

	if len(issues) == 0 {
		issues, err = c.searchIssues(ctx, repo, "", []string{"async-daily/report"}, 10, fmt.Sprintf("created:%s", date))
		if err != nil {
			return ReportIssue{}, err
		}
	}

	for _, issue := range issues {
		if issue.Date != date {
			continue
		}

		return c.ViewIssue(ctx, repo, issue.Number)
	}

	return ReportIssue{}, ErrIssueNotFound
}

func (c *Client) searchIssues(ctx context.Context, repo, author string, labels []string, limit int, search string) ([]AsyncDailyIssue, error) {
	if limit <= 0 {
		limit = 100
	}

	args := []string{"issue", "list", "--repo", repo, "--state", "all", "--limit", strconv.Itoa(limit)}
	if strings.TrimSpace(author) != "" {
		args = append(args, "--author", author)
	}

	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}

		args = append(args, "--label", label)
	}

	if strings.TrimSpace(search) != "" {
		args = append(args, "--search", search)
	}

	args = append(args, "--json", "number,title,url,createdAt,updatedAt,state")

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list GitHub issues: %s", strings.TrimSpace(string(output)))
	}

	var raw []issueListItem
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("decode GitHub issues: %w", err)
	}

	issues := make([]AsyncDailyIssue, 0, len(raw))
	for _, item := range raw {
		createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse issue createdAt %q: %w", item.CreatedAt, err)
		}

		updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse issue updatedAt %q: %w", item.UpdatedAt, err)
		}

		issues = append(issues, AsyncDailyIssue{
			Number:    item.Number,
			Title:     item.Title,
			URL:       item.URL,
			State:     item.State,
			Date:      issueDate(item.Title, createdAt),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return issues, nil
}

func (c *Client) ViewIssue(ctx context.Context, repo string, number int) (AsyncDailyIssue, error) {
	output, err := c.run(ctx, "issue", "view", strconv.Itoa(number), "--repo", repo, "--json", "number,title,body,url,state,createdAt,updatedAt")
	if err != nil {
		return AsyncDailyIssue{}, fmt.Errorf("view GitHub issue: %s", strings.TrimSpace(string(output)))
	}

	var item issueViewItem
	if err := json.Unmarshal(output, &item); err != nil {
		return AsyncDailyIssue{}, fmt.Errorf("decode GitHub issue: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return AsyncDailyIssue{}, fmt.Errorf("parse issue createdAt %q: %w", item.CreatedAt, err)
	}

	updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
	if err != nil {
		return AsyncDailyIssue{}, fmt.Errorf("parse issue updatedAt %q: %w", item.UpdatedAt, err)
	}

	return AsyncDailyIssue{
		Number:    item.Number,
		Title:     item.Title,
		Body:      item.Body,
		URL:       item.URL,
		State:     item.State,
		Date:      issueDate(item.Title, createdAt),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func issueDate(title string, createdAt time.Time) string {
	trimmedTitle := strings.TrimSpace(title)
	if date, ok := daily.DateFromIssueTitle(trimmedTitle); ok {
		return date
	}

	if date, ok := daily.DateFromReportTitle(trimmedTitle); ok {
		return date
	}

	return createdAt.Format(daily.DateLayout)
}

func issueNumberFromURL(rawURL string) (int, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return 0, fmt.Errorf("parse issue URL: %w", err)
	}

	number, err := strconv.Atoi(path.Base(parsed.Path))
	if err != nil {
		return 0, fmt.Errorf("parse issue number from %q: %w", rawURL, err)
	}

	return number, nil
}

func runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	return cmd.CombinedOutput()
}
