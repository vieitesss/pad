package tui

import (
	"strings"
	"testing"

	"github.com/vieitesss/pad/internal/daily"
	"github.com/vieitesss/pad/internal/issueform"
)

func TestNextVisibleIndexWrapsForwardFromLastField(t *testing.T) {
	next := nextVisibleIndex(2, 1, 3)
	if next != 0 {
		t.Fatalf("expected wrap to field 0 from last field, got %d", next)
	}
}

func TestNextVisibleIndexWrapsBackwardFromFirstField(t *testing.T) {
	next := nextVisibleIndex(0, -1, 3)
	if next != 2 {
		t.Fatalf("expected wrap to field 2 from first field, got %d", next)
	}
}

func TestPreviewContentContainsRenderedTemplate(t *testing.T) {
	entry := daily.New("2026-04-16", mustTemplate(t))
	entry.SetText("yesterday", "- Reviewed PR #42")
	entry.SetText("today", "- Continue feature work")

	content := previewContent(entry)

	checks := []string{
		"[Daily Update] [2026/04/16]",
		`<!-- pad:fields:{"fields":[{"id":"yesterday","label":"✅ What did you do yesterday?"},{"id":"today","label":"🎯 What will you do today?"},{"id":"parking_lot","label":"🚨 Do you request a Parking Lot or escalation?"}]} -->`,
		"### ✅ What did you do yesterday?",
		"### 🎯 What will you do today?",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Fatalf("expected preview content to contain %q, got %q", check, content)
		}
	}
}

func TestMoveLoadsStoredTextForEachField(t *testing.T) {
	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")
	m.persistCurrentField()

	m.move(1)
	if got := m.editor.Value(); got != "" {
		t.Fatalf("expected next field editor to start empty, got %q", got)
	}

	m.editor.SetValue("- second field text")
	m.persistCurrentField()

	m.move(-1)
	if got := m.editor.Value(); got != "- first field text" {
		t.Fatalf("expected previous field text to be restored, got %q", got)
	}
}

func TestCheckboxFieldTogglesInPreview(t *testing.T) {
	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.move(2)
	if m.currentField().ID != "parking_lot" {
		t.Fatalf("expected current field parking_lot, got %q", m.currentField().ID)
	}

	m.entry.SetChecked("parking_lot", true)
	content := previewContent(m.finalEntry())
	if !strings.Contains(content, "- [x] ✅ Yes, I need a Parking Lot or escalation") {
		t.Fatalf("expected preview to show checked checkbox, got %q", content)
	}
}

func mustTemplate(t *testing.T) issueform.Template {
	t.Helper()
	tmpl, err := issueform.Parse(".github/ISSUE_TEMPLATE/daily-update.yml", []byte(`
body:
  - type: textarea
    id: yesterday
    attributes:
      label: "✅ What did you do yesterday?"
      placeholder: "- Reviewed PR #123"

  - type: textarea
    id: today
    attributes:
      label: "🎯 What will you do today?"
      placeholder: "- Continue feature work"

  - type: checkboxes
    id: parking_lot
    attributes:
      label: "🚨 Do you request a Parking Lot or escalation?"
      options:
        - label: "✅ Yes, I need a Parking Lot or escalation"
`))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return tmpl
}
