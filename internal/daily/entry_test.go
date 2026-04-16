package daily

import (
	"strings"
	"testing"
)

func TestEntryFromIssueBodyParsesIssueFormMarkdown(t *testing.T) {
	body := `### ✅ What did you do yesterday?

- https://github.com/prefapp/features/issues/918

### 🎯 What will you do today?

- https://github.com/prefapp/gitops-k8s/pull/1755

### 🚧 Any blockers?

_No response_

### 🚨 Do you request a Parking Lot or escalation?

- [x] ✅ Yes, I need a Parking Lot or escalation

### 📝 Parking Lot Details

- Need help with deployment scope

### 💬 Additional Comments

- Offline after 17:00`

	got := EntryFromIssueBody("2026-04-17", body)

	if got.Date != "2026-04-17" {
		t.Fatalf("expected date 2026-04-17, got %q", got.Date)
	}

	if got.Yesterday != "- https://github.com/prefapp/features/issues/918" {
		t.Fatalf("unexpected yesterday %q", got.Yesterday)
	}

	if got.Today != "- https://github.com/prefapp/gitops-k8s/pull/1755" {
		t.Fatalf("unexpected today %q", got.Today)
	}

	if got.Blockers != "" {
		t.Fatalf("expected empty blockers, got %q", got.Blockers)
	}

	if !got.ParkingLot {
		t.Fatalf("expected parking lot to be true")
	}

	if got.ParkingLotDetails != "- Need help with deployment scope" {
		t.Fatalf("unexpected parking lot details %q", got.ParkingLotDetails)
	}

	if got.AdditionalComments != "- Offline after 17:00" {
		t.Fatalf("unexpected additional comments %q", got.AdditionalComments)
	}
}

func TestEntryFromIssueBodyParsesPadMarkdown(t *testing.T) {
	body := `## ✅ What did you do yesterday?
- Reviewed PR #42

## 🎯 What will you do today?
- Continue feature work

## 🚧 Any blockers?
_None._

## 🚨 Parking Lot / Escalation
- ✅ Yes, I need a Parking Lot or escalation

- Need product input

## 💬 Additional Comments
- Waiting for design confirmation`

	got := EntryFromIssueBody("2026-04-17", body)

	if got.Yesterday != "- Reviewed PR #42" {
		t.Fatalf("unexpected yesterday %q", got.Yesterday)
	}

	if got.Today != "- Continue feature work" {
		t.Fatalf("unexpected today %q", got.Today)
	}

	if got.Blockers != "" {
		t.Fatalf("expected empty blockers, got %q", got.Blockers)
	}

	if !got.ParkingLot {
		t.Fatalf("expected parking lot to be true")
	}

	if got.ParkingLotDetails != "- Need product input" {
		t.Fatalf("unexpected parking lot details %q", got.ParkingLotDetails)
	}

	if got.AdditionalComments != "- Waiting for design confirmation" {
		t.Fatalf("unexpected additional comments %q", got.AdditionalComments)
	}
}

func TestBodyRendersTemplateSections(t *testing.T) {
	entry := Entry{
		Date:      "2026-04-16",
		Yesterday: "- Reviewed PR #42",
		Today:     "- Continue feature work",
	}

	body := entry.Body()

	checks := []string{
		"## ✅ What did you do yesterday?",
		"## 🎯 What will you do today?",
		"## 🚧 Any blockers?",
		"_None._",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Fatalf("expected body to contain %q, got %q", check, body)
		}
	}

	if strings.Contains(body, "Parking Lot / Escalation") {
		t.Fatalf("expected empty parking lot section to be omitted")
	}
}

func TestDateFromIssueTitle(t *testing.T) {
	date, ok := DateFromIssueTitle("[Async Daily] [2026/04/16]")
	if !ok {
		t.Fatalf("expected title to parse")
	}

	if date != "2026-04-16" {
		t.Fatalf("expected parsed date 2026-04-16, got %q", date)
	}
}

func TestDateFromIssueTitleRejectsNonTemplateTitles(t *testing.T) {
	if _, ok := DateFromIssueTitle("other title"); ok {
		t.Fatalf("expected non-template title to be rejected")
	}
}

func TestDateFromReportTitle(t *testing.T) {
	date, ok := DateFromReportTitle("[Daily Report] 2026/04/16")
	if !ok {
		t.Fatalf("expected report title to parse")
	}

	if date != "2026-04-16" {
		t.Fatalf("expected parsed date 2026-04-16, got %q", date)
	}
}
