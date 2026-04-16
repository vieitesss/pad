package daily

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vieitesss/pad/internal/issueform"
)

func TestEntryFromIssueBodyParsesIssueFormMarkdown(t *testing.T) {
	template := mustTemplate(t)
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

	got := EntryFromIssueBody("2026-04-17", template, body)

	if got.Date != "2026-04-17" {
		t.Fatalf("expected date 2026-04-17, got %q", got.Date)
	}

	if got.Text("yesterday") != "- https://github.com/prefapp/features/issues/918" {
		t.Fatalf("unexpected yesterday %q", got.Text("yesterday"))
	}

	if got.Text("today") != "- https://github.com/prefapp/gitops-k8s/pull/1755" {
		t.Fatalf("unexpected today %q", got.Text("today"))
	}

	if got.Text("blockers") != "" {
		t.Fatalf("expected empty blockers, got %q", got.Text("blockers"))
	}

	if !got.Checked("parking_lot") {
		t.Fatalf("expected parking lot to be true")
	}

	if got.Text("parking_details") != "- Need help with deployment scope" {
		t.Fatalf("unexpected parking lot details %q", got.Text("parking_details"))
	}

	if got.Text("comments") != "- Offline after 17:00" {
		t.Fatalf("unexpected additional comments %q", got.Text("comments"))
	}
}

func TestEntryFromIssueBodyParsesLegacyPadMarkdown(t *testing.T) {
	template := mustTemplate(t)
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

	got := EntryFromIssueBody("2026-04-17", template, body)

	if got.Text("yesterday") != "- Reviewed PR #42" {
		t.Fatalf("unexpected yesterday %q", got.Text("yesterday"))
	}

	if got.Text("today") != "- Continue feature work" {
		t.Fatalf("unexpected today %q", got.Text("today"))
	}

	if got.Text("blockers") != "" {
		t.Fatalf("expected empty blockers, got %q", got.Text("blockers"))
	}

	if !got.Checked("parking_lot") {
		t.Fatalf("expected parking lot to be true")
	}

	if got.Text("parking_details") != "- Need product input" {
		t.Fatalf("unexpected parking lot details %q", got.Text("parking_details"))
	}

	if got.Text("comments") != "- Waiting for design confirmation" {
		t.Fatalf("unexpected additional comments %q", got.Text("comments"))
	}
}

func TestEntryFromIssueBodyUsesHiddenIDsWhenLabelsChange(t *testing.T) {
	template := mustTemplateWithRenamedYesterday(t)
	body := `## Yesterday Work <!-- pad:id:yesterday -->
- Reviewed PR #42

## Current Focus <!-- pad:id:today -->
- Continue feature work`

	got := EntryFromIssueBody("2026-04-17", template, body)

	if got.Text("yesterday") != "- Reviewed PR #42" {
		t.Fatalf("unexpected yesterday %q", got.Text("yesterday"))
	}

	if got.Text("today") != "- Continue feature work" {
		t.Fatalf("unexpected today %q", got.Text("today"))
	}
}

func TestEntryFromIssueBodyAddsCarryoverForRemovedFields(t *testing.T) {
	template := mustTemplateWithRenamedYesterday(t)
	body := `## Yesterday Work <!-- pad:id:yesterday -->
- Reviewed PR #42

## Current Focus <!-- pad:id:today -->
- Continue feature work

## 💬 Additional Comments <!-- pad:id:comments -->
- Offline after 17:00`

	got := EntryFromIssueBody("2026-04-17", template, body)

	if got.Text(carryoverFieldID) == "" {
		t.Fatalf("expected carryover field to be populated")
	}

	if !strings.Contains(got.Text(carryoverFieldID), "Additional Comments") {
		t.Fatalf("expected carryover to mention removed section, got %q", got.Text(carryoverFieldID))
	}
	if !strings.Contains(got.Body(), "Carryover From Previous Template") {
		t.Fatalf("expected rendered body to include carryover section")
	}
}

func TestEntryFromIssueBodyKeepsMarkdownHeadingsInsideResponseBody(t *testing.T) {
	template := mustTemplate(t)
	body := `## ✅ What did you do yesterday? <!-- pad:id:yesterday -->
- Reviewed PR #42

## 🎯 What will you do today? <!-- pad:id:today -->
- Continue feature work

### Notes
- Include nested heading in response

## 🚧 Any blockers? <!-- pad:id:blockers -->
_None._`

	got := EntryFromIssueBody("2026-04-17", template, body)

	if !strings.Contains(got.Text("today"), "### Notes") {
		t.Fatalf("expected nested heading to stay inside today response, got %q", got.Text("today"))
	}
}

func TestBodyRendersTemplateSectionsAndHiddenIDs(t *testing.T) {
	entry := New("2026-04-16", mustTemplate(t))
	entry.SetText("yesterday", "- Reviewed PR #42")
	entry.SetText("today", "- Continue feature work")

	body := entry.Body()

	checks := []string{
		"## Daily Standup Update",
		"## ✅ What did you do yesterday? <!-- pad:id:yesterday -->",
		"## 🎯 What will you do today? <!-- pad:id:today -->",
		"## 🚧 Any blockers? <!-- pad:id:blockers -->",
		"_None._",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Fatalf("expected body to contain %q, got %q", check, body)
		}
	}
}

func TestValidateForCreateRequiresTemplateRequiredFields(t *testing.T) {
	entry := New("2026-04-16", mustTemplate(t))
	entry.SetText("today", "- Continue feature work")

	if err := entry.ValidateForCreate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDateFromIssueTitle(t *testing.T) {
	date, ok := DateFromIssueTitle("[Daily Update] [2026/04/16]")
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

func TestEntryTitleUsesTemplateTitle(t *testing.T) {
	template := mustTemplateWithTitle(t, "[Standup] YYYY/MM/DD")
	entry := New("2026-04-16", template)

	title, err := entry.Title()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if title != "[Standup] 2026/04/16" {
		t.Fatalf("expected title '[Standup] 2026/04/16', got %q", title)
	}
}

func TestEntryTitleSupportsMultipleDateFormats(t *testing.T) {
	tests := []struct {
		templateTitle string
		expected      string
	}{
		{"Daily Update - YYYY-MM-DD", "Daily Update - 2026-04-16"},
		{"Standup MM/DD/YYYY", "Standup 04/16/2026"},
		{"Update DD/MM/YYYY", "Update 16/04/2026"},
	}

	for _, test := range tests {
		template := mustTemplateWithTitle(t, test.templateTitle)
		entry := New("2026-04-16", template)

		title, err := entry.Title()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if title != test.expected {
			t.Fatalf("expected title %q, got %q", test.expected, title)
		}
	}
}

func TestEntryTitleFallsBackToLegacyFormat(t *testing.T) {
	template := mustTemplate(t) // No title set
	entry := New("2026-04-16", template)

	title, err := entry.Title()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fallback to legacy format since template has no title
	if title != "[Daily Update] [2026/04/16]" {
		t.Fatalf("expected legacy title format, got %q", title)
	}
}

func TestDateFromIssueTitleWithTemplateTitle(t *testing.T) {
	date, ok := DateFromIssueTitle("[Standup] 2026/04/16")
	if !ok {
		t.Fatalf("expected template-style title to parse")
	}

	if date != "2026-04-16" {
		t.Fatalf("expected parsed date 2026-04-16, got %q", date)
	}
}

func TestDateFromIssueTitleWithDifferentFormats(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Daily Update - 2026-04-16", "2026-04-16"},
		{"Standup 04/16/2026", "2026-04-16"},
		{"Update 16/04/2026", "2026-04-16"},
	}

	for _, test := range tests {
		date, ok := DateFromIssueTitle(test.title)
		if !ok {
			t.Fatalf("expected title %q to parse", test.title)
		}

		if date != test.expected {
			t.Fatalf("expected parsed date %q, got %q", test.expected, date)
		}
	}
}

func mustTemplate(t *testing.T) issueform.Template {
	t.Helper()
	tmpl, err := issueform.Parse(".github/ISSUE_TEMPLATE/daily-update.yml", []byte(`
body:
  - type: markdown
    attributes:
      value: |
        ## Daily Standup Update

  - type: textarea
    id: yesterday
    attributes:
      label: "✅ What did you do yesterday?"
    validations:
      required: true

  - type: textarea
    id: today
    attributes:
      label: "🎯 What will you do today?"
    validations:
      required: true

  - type: textarea
    id: blockers
    attributes:
      label: "🚧 Any blockers?"

  - type: checkboxes
    id: parking_lot
    attributes:
      label: "🚨 Do you request a Parking Lot or escalation?"
      options:
        - label: "✅ Yes, I need a Parking Lot or escalation"

  - type: textarea
    id: parking_details
    attributes:
      label: "📝 Parking Lot Details"

  - type: textarea
    id: comments
    attributes:
      label: "💬 Additional Comments"
`))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}

func mustTemplateWithRenamedYesterday(t *testing.T) issueform.Template {
	t.Helper()
	tmpl, err := issueform.Parse(".github/ISSUE_TEMPLATE/daily-update.yml", []byte(`
body:
  - type: textarea
    id: yesterday
    attributes:
      label: Yesterday Work
    validations:
      required: true

  - type: textarea
    id: today
    attributes:
      label: Current Focus
    validations:
      required: true
`))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}

func mustTemplateWithTitle(t *testing.T, title string) issueform.Template {
	t.Helper()
	yaml := fmt.Sprintf(`
title: "%s"
body:
  - type: textarea
    id: yesterday
    attributes:
      label: "What did you do yesterday?"
    validations:
      required: true
`, title)
	tmpl, err := issueform.Parse(".github/ISSUE_TEMPLATE/daily-update.yml", []byte(yaml))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}
