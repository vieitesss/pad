package ghcli

import (
	"context"
	"strings"
	"testing"
)

func TestListAsyncDailyIssuesKeepsLabeledIssuesAndFallsBackToCreatedDate(t *testing.T) {
	client := newForTests(func(_ context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "issue list") {
			t.Fatalf("expected issue list command, got %q", joined)
		}

		if !strings.Contains(joined, "--author @me") {
			t.Fatalf("expected author filter, got %q", joined)
		}

		return []byte(`[
			{"number":470,"title":"[Async Daily] [2026/04/16]","url":"https://example.com/470","state":"CLOSED","createdAt":"2026-04-16T08:54:39Z","updatedAt":"2026-04-16T11:08:57Z"},
			{"number":9,"title":"Unrelated issue","url":"https://example.com/9","state":"OPEN","createdAt":"2026-04-16T08:54:39Z","updatedAt":"2026-04-16T11:08:57Z"}
		]`), nil
	})

	issues, err := client.ListAsyncDailyIssues(context.Background(), "prefapp/doc-asyncdaily", []string{"async-daily/member"}, 10)
	if err != nil {
		t.Fatalf("list async daily issues: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("expected 2 labeled issues, got %d", len(issues))
	}

	if issues[0].Date != "2026-04-16" {
		t.Fatalf("expected parsed date 2026-04-16, got %q", issues[0].Date)
	}

	if issues[1].Date != "2026-04-16" {
		t.Fatalf("expected createdAt fallback date 2026-04-16, got %q", issues[1].Date)
	}
}

func TestFindAsyncDailyIssueByDateLoadsIssueBody(t *testing.T) {
	client := newForTests(func(_ context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "issue list"):
			return []byte(`[
				{"number":470,"title":"[Async Daily] [2026/04/16]","url":"https://example.com/470","state":"CLOSED","createdAt":"2026-04-16T08:54:39Z","updatedAt":"2026-04-16T11:08:57Z"}
			]`), nil
		case strings.Contains(joined, "issue view 470"):
			return []byte(`{"number":470,"title":"[Async Daily] [2026/04/16]","body":"remote body","url":"https://example.com/470","state":"CLOSED","createdAt":"2026-04-16T08:54:39Z","updatedAt":"2026-04-16T11:08:57Z"}`), nil
		default:
			t.Fatalf("unexpected gh command %q", joined)
			return nil, nil
		}
	})

	issue, err := client.FindAsyncDailyIssueByDate(context.Background(), "prefapp/doc-asyncdaily", []string{"async-daily/member"}, "2026-04-16")
	if err != nil {
		t.Fatalf("find async daily issue: %v", err)
	}

	if issue.Body != "remote body" {
		t.Fatalf("expected remote body, got %q", issue.Body)
	}
}

func TestListReportIssuesDoesNotUseAuthorFilter(t *testing.T) {
	client := newForTests(func(_ context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--author") {
			t.Fatalf("did not expect author filter, got %q", joined)
		}

		if !strings.Contains(joined, "--label async-daily/report") {
			t.Fatalf("expected report label filter, got %q", joined)
		}

		return []byte(`[
			{"number":473,"title":"[Daily Report] 2026/04/16","url":"https://example.com/473","state":"OPEN","createdAt":"2026-04-16T11:08:51Z","updatedAt":"2026-04-16T11:08:52Z"}
		]`), nil
	})

	issues, err := client.ListReportIssues(context.Background(), "prefapp/doc-asyncdaily", 10)
	if err != nil {
		t.Fatalf("list report issues: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 report issue, got %d", len(issues))
	}

	if issues[0].Date != "2026-04-16" {
		t.Fatalf("expected parsed report date 2026-04-16, got %q", issues[0].Date)
	}
}

func TestFindReportIssueByDateLoadsIssueBody(t *testing.T) {
	client := newForTests(func(_ context.Context, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "issue list"):
			if !strings.Contains(joined, "\"[Daily Report] 2026/04/16\" in:title") {
				t.Fatalf("expected exact report title search, got %q", joined)
			}
			return []byte(`[
				{"number":473,"title":"[Daily Report] 2026/04/16","url":"https://example.com/473","state":"OPEN","createdAt":"2026-04-16T11:08:51Z","updatedAt":"2026-04-16T11:08:52Z"}
			]`), nil
		case strings.Contains(joined, "issue view 473"):
			return []byte(`{"number":473,"title":"[Daily Report] 2026/04/16","body":"team report body","url":"https://example.com/473","state":"OPEN","createdAt":"2026-04-16T11:08:51Z","updatedAt":"2026-04-16T11:08:52Z"}`), nil
		default:
			t.Fatalf("unexpected gh command %q", joined)
			return nil, nil
		}
	})

	issue, err := client.FindReportIssueByDate(context.Background(), "prefapp/doc-asyncdaily", "2026-04-16")
	if err != nil {
		t.Fatalf("find report issue: %v", err)
	}

	if issue.Body != "team report body" {
		t.Fatalf("expected team report body, got %q", issue.Body)
	}
}
