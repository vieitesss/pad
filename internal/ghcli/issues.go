package ghcli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/vieitesss/pad/internal/daily"
)

var ErrIssueNotFound = errors.New("issue not found")

type runner func(ctx context.Context, args ...string) ([]byte, error)

type Client struct {
	run runner
}

type DailyUpdateIssue struct {
	Number    int
	Title     string
	Body      string
	URL       string
	State     string
	Date      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReportIssue = DailyUpdateIssue

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

type repoContentItem struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
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

func (c *Client) ReadRepositoryFile(ctx context.Context, repo, filePath string) ([]byte, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	apiPath := fmt.Sprintf("repos/%s/%s/contents/%s", url.PathEscape(owner), url.PathEscape(name), escapeRepoPath(filePath))
	output, err := c.run(ctx, "api", apiPath)
	if err != nil {
		return nil, fmt.Errorf("read repository file: %s", strings.TrimSpace(string(output)))
	}

	var item repoContentItem
	if err := json.Unmarshal(output, &item); err != nil {
		return nil, fmt.Errorf("decode repository file: %w", err)
	}

	if item.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported repository file encoding %q", item.Encoding)
	}

	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(item.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode repository file content: %w", err)
	}

	return content, nil
}

func (c *Client) ListDailyUpdateIssues(ctx context.Context, repo string, labels []string, limit int) ([]DailyUpdateIssue, error) {
	return c.searchIssues(ctx, repo, "@me", labels, limit, "")
}

func (c *Client) FindDailyUpdateIssueByDate(ctx context.Context, repo string, labels []string, date string) (DailyUpdateIssue, error) {
	title, err := daily.TitleForDate(date)
	if err != nil {
		return DailyUpdateIssue{}, err
	}

	issues, err := c.searchIssues(ctx, repo, "@me", labels, 5, fmt.Sprintf("%q in:title", title))
	if err != nil {
		return DailyUpdateIssue{}, err
	}

	if len(issues) == 0 {
		issues, err = c.searchIssues(ctx, repo, "@me", labels, 10, fmt.Sprintf("created:%s", date))
		if err != nil {
			return DailyUpdateIssue{}, err
		}
	}

	for _, issue := range issues {
		if issue.Date != date {
			continue
		}

		return c.ViewIssue(ctx, repo, issue.Number)
	}

	return DailyUpdateIssue{}, ErrIssueNotFound
}

func (c *Client) LatestDailyUpdateIssue(ctx context.Context, repo string, labels []string) (DailyUpdateIssue, error) {
	issues, err := c.ListDailyUpdateIssues(ctx, repo, labels, 1)
	if err != nil {
		return DailyUpdateIssue{}, err
	}

	if len(issues) == 0 {
		return DailyUpdateIssue{}, ErrIssueNotFound
	}

	return c.ViewIssue(ctx, repo, issues[0].Number)
}

func (c *Client) ListReportIssues(ctx context.Context, repo string, limit int) ([]ReportIssue, error) {
	return c.searchIssues(ctx, repo, "", []string{"daily-update/report"}, limit, "")
}

func (c *Client) FindReportIssueByDate(ctx context.Context, repo string, date string) (ReportIssue, error) {
	title, err := daily.ReportTitleForDate(date)
	if err != nil {
		return ReportIssue{}, err
	}

	issues, err := c.searchIssues(ctx, repo, "", []string{"daily-update/report"}, 5, fmt.Sprintf("%q in:title", title))
	if err != nil {
		return ReportIssue{}, err
	}

	if len(issues) == 0 {
		issues, err = c.searchIssues(ctx, repo, "", []string{"daily-update/report"}, 10, fmt.Sprintf("created:%s", date))
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

func (c *Client) searchIssues(ctx context.Context, repo, author string, labels []string, limit int, search string) ([]DailyUpdateIssue, error) {
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

	issues := make([]DailyUpdateIssue, 0, len(raw))
	for _, item := range raw {
		createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse issue createdAt %q: %w", item.CreatedAt, err)
		}

		updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse issue updatedAt %q: %w", item.UpdatedAt, err)
		}

		issues = append(issues, DailyUpdateIssue{
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

func (c *Client) ViewIssue(ctx context.Context, repo string, number int) (DailyUpdateIssue, error) {
	output, err := c.run(ctx, "issue", "view", strconv.Itoa(number), "--repo", repo, "--json", "number,title,body,url,state,createdAt,updatedAt")
	if err != nil {
		return DailyUpdateIssue{}, fmt.Errorf("view GitHub issue: %s", strings.TrimSpace(string(output)))
	}

	var item issueViewItem
	if err := json.Unmarshal(output, &item); err != nil {
		return DailyUpdateIssue{}, fmt.Errorf("decode GitHub issue: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return DailyUpdateIssue{}, fmt.Errorf("parse issue createdAt %q: %w", item.CreatedAt, err)
	}

	updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
	if err != nil {
		return DailyUpdateIssue{}, fmt.Errorf("parse issue updatedAt %q: %w", item.UpdatedAt, err)
	}

	return DailyUpdateIssue{
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

func splitRepo(repo string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(repo), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q", repo)
	}
	return parts[0], parts[1], nil
}

func escapeRepoPath(filePath string) string {
	parts := strings.Split(strings.Trim(filePath, "/"), "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}

func runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	return cmd.CombinedOutput()
}
