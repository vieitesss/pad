package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestCtrlCCopiesCurrentFieldToClipboard(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	var copied string
	writeClipboard = func(value string) error {
		copied = value
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)

	if got.clipboard != "- first field text" {
		t.Fatalf("expected pad clipboard to store current field, got %q", got.clipboard)
	}
	if !got.clipboardSynced {
		t.Fatalf("expected clipboard sync to be marked successful")
	}
	if copied != "- first field text" {
		t.Fatalf("expected system clipboard write, got %q", copied)
	}
	if got.editor.Value() != "- first field text" {
		t.Fatalf("expected copy to keep field contents, got %q", got.editor.Value())
	}
	if got.message != "Copied current field to clipboard." {
		t.Fatalf("unexpected message %q", got.message)
	}
}

func TestCtrlXCutsCurrentFieldToClipboard(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(value string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- carry this forward")
	m.persistCurrentField()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got := updated.(model)

	if got.clipboard != "- carry this forward" {
		t.Fatalf("expected pad clipboard to store moved field, got %q", got.clipboard)
	}
	if got.editor.Value() != "" {
		t.Fatalf("expected move to clear field, got %q", got.editor.Value())
	}
	if got.entry.Text("yesterday") != "" {
		t.Fatalf("expected move to persist cleared field, got %q", got.entry.Text("yesterday"))
	}
}

func TestCtrlCDoesNotCopyEmptyField(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	called := false
	writeClipboard = func(string) error {
		called = true
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.clipboard = "- existing clipboard"
	m.editor.SetValue("   \n")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)

	if called {
		t.Fatalf("expected empty field copy to skip system clipboard")
	}
	if got.clipboard != "- existing clipboard" {
		t.Fatalf("expected pad clipboard to stay unchanged, got %q", got.clipboard)
	}
	if got.message != "Current field is empty." {
		t.Fatalf("unexpected empty copy message %q", got.message)
	}
}

func TestCtrlXDoesNotCutEmptyField(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	called := false
	writeClipboard = func(string) error {
		called = true
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.clipboard = "- existing clipboard"
	m.editor.SetValue("\n\t")
	before := m.editor.Value()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got := updated.(model)

	if called {
		t.Fatalf("expected empty field cut to skip system clipboard")
	}
	if got.clipboard != "- existing clipboard" {
		t.Fatalf("expected pad clipboard to stay unchanged, got %q", got.clipboard)
	}
	if got.editor.Value() != before {
		t.Fatalf("expected empty field cut to leave editor unchanged, got %q", got.editor.Value())
	}
	if got.message != "Current field is empty." {
		t.Fatalf("unexpected empty cut message %q", got.message)
	}
}

func TestCtrlVPastesPadClipboardIntoCurrentFieldWhenSystemClipboardUnavailable(t *testing.T) {
	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.clipboard = "- moved text"
	m.clipboardSynced = false
	m.editor.SetValue("Start\n")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	got := updated.(model)

	if got.editor.Value() != "Start\n- moved text" {
		t.Fatalf("expected paste to insert clipboard at cursor, got %q", got.editor.Value())
	}
	if got.entry.Text("yesterday") != "Start\n- moved text" {
		t.Fatalf("expected pasted value to persist, got %q", got.entry.Text("yesterday"))
	}
	if got.message != "Pasted clipboard into current field." {
		t.Fatalf("unexpected message %q", got.message)
	}
}

func TestCopyFallsBackToPadClipboardWhenSystemClipboardFails(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return errors.New("clipboard unavailable")
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)

	if got.clipboard != "- first field text" {
		t.Fatalf("expected pad clipboard to still store value, got %q", got.clipboard)
	}
	if got.clipboardSynced {
		t.Fatalf("expected clipboard sync to be marked failed")
	}
	if got.message != "Copied current field to pad clipboard only." {
		t.Fatalf("unexpected fallback message %q", got.message)
	}
}

func TestCtrlCCopyMessageCountsConsecutiveCopies(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)
	if got.message != "Copied current field to clipboard." {
		t.Fatalf("unexpected first copy message %q", got.message)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = updated.(model)
	if got.message != "Copied current field to clipboard. (2)" {
		t.Fatalf("unexpected second copy message %q", got.message)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = updated.(model)
	if got.message != "Copied current field to clipboard. (3)" {
		t.Fatalf("unexpected third copy message %q", got.message)
	}
}

func TestCopyMessageClearsOnNextNonCopyKeystroke(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)
	if got.message != "Copied current field to clipboard." {
		t.Fatalf("unexpected copy message %q", got.message)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(model)
	if got.message != "" {
		t.Fatalf("expected copy message to clear on next non-copy key, got %q", got.message)
	}
}

func TestCtrlXCutMessageCountsConsecutiveCuts(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first cut")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got := updated.(model)
	if got.message != "Cut current field to clipboard." {
		t.Fatalf("unexpected first cut message %q", got.message)
	}

	got.editor.SetValue("- second cut")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got = updated.(model)
	if got.message != "Cut current field to clipboard. (2)" {
		t.Fatalf("unexpected second cut message %q", got.message)
	}

	got.editor.SetValue("- third cut")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got = updated.(model)
	if got.message != "Cut current field to clipboard. (3)" {
		t.Fatalf("unexpected third cut message %q", got.message)
	}
}

func TestCutMessageClearsOnNextNonCutKeystroke(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first cut")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got := updated.(model)
	if got.message != "Cut current field to clipboard." {
		t.Fatalf("unexpected cut message %q", got.message)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(model)
	if got.message != "" {
		t.Fatalf("expected cut message to clear on next non-cut key, got %q", got.message)
	}
}

func TestCutResetsCopyMessageCount(t *testing.T) {
	originalWriteClipboard := writeClipboard
	t.Cleanup(func() {
		writeClipboard = originalWriteClipboard
	})

	writeClipboard = func(string) error {
		return nil
	}

	m := newModel(daily.New("2026-04-16", mustTemplate(t)), modeCreate)
	m.editor.SetValue("- first field text")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = updated.(model)
	if got.message != "Copied current field to clipboard. (2)" {
		t.Fatalf("unexpected copy message before cut %q", got.message)
	}

	got.editor.SetValue("- cut text")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	got = updated.(model)
	if got.message != "Cut current field to clipboard." {
		t.Fatalf("unexpected cut message %q", got.message)
	}

	got.editor.SetValue("- copied again")
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got = updated.(model)
	if got.message != "Copied current field to clipboard." {
		t.Fatalf("expected copy count reset after cut, got %q", got.message)
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
