package tui

import (
	"strings"
	"testing"

	"github.com/vieitesss/pad/internal/daily"
)

func TestNextVisibleIndexWrapsForwardFromLastField(t *testing.T) {
	lastField := len(fieldDefs) - 1
	next := nextVisibleIndex(lastField, 1, false)
	if next != 0 {
		t.Fatalf("expected wrap to field 0 from last field, got %d", next)
	}
}

func TestNextVisibleIndexWrapsBackwardFromFirstField(t *testing.T) {
	next := nextVisibleIndex(0, -1, false)
	lastField := len(fieldDefs) - 1
	if next != lastField {
		t.Fatalf("expected wrap to field %d from first field, got %d", lastField, next)
	}
}

func TestNextVisibleIndexSkipsParkingLotDetailsWhenDisabledForward(t *testing.T) {
	next := nextVisibleIndex(parkingLotField, 1, false)
	if next != additionalCommentsField {
		t.Fatalf("expected skip to field %d, got %d", additionalCommentsField, next)
	}
}

func TestNextVisibleIndexSkipsParkingLotDetailsWhenDisabledBackward(t *testing.T) {
	next := nextVisibleIndex(additionalCommentsField, -1, false)
	if next != parkingLotField {
		t.Fatalf("expected skip back to field %d, got %d", parkingLotField, next)
	}
}

func TestNextVisibleIndexIncludesParkingLotDetailsWhenEnabled(t *testing.T) {
	next := nextVisibleIndex(parkingLotField, 1, true)
	if next != parkingLotDetailsField {
		t.Fatalf("expected next field %d, got %d", parkingLotDetailsField, next)
	}
}

func TestFinalEntryClearsParkingLotDetailsWhenDisabled(t *testing.T) {
	m := newModel(daily.Entry{
		Date:              "2026-04-16",
		ParkingLot:        false,
		ParkingLotDetails: "should be cleared",
	}, modeSave)

	entry := m.finalEntry()
	if entry.ParkingLotDetails != "" {
		t.Fatalf("expected parking lot details to be cleared, got %q", entry.ParkingLotDetails)
	}
}

func TestFinalEntryDoesNotMutateParkingLotDetailsInModel(t *testing.T) {
	m := newModel(daily.Entry{
		Date:              "2026-04-16",
		ParkingLot:        false,
		ParkingLotDetails: "keep me during editing",
	}, modeCreate)

	_ = m.finalEntry()
	if m.entry.ParkingLotDetails != "keep me during editing" {
		t.Fatalf("expected editor state to keep parking lot details, got %q", m.entry.ParkingLotDetails)
	}
}

func TestPreviewContentContainsRenderedTemplate(t *testing.T) {
	content := previewContent(daily.Entry{
		Date:      "2026-04-16",
		Yesterday: "- Reviewed PR #42",
		Today:     "- Continue feature work",
	})

	checks := []string{
		"[Daily Update] [2026/04/16]",
		"## ✅ What did you do yesterday?",
		"## 🎯 What will you do today?",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Fatalf("expected preview content to contain %q, got %q", check, content)
		}
	}
}

func TestMoveLoadsStoredTextForEachField(t *testing.T) {
	m := newModel(daily.Entry{Date: "2026-04-16"}, modeCreate)
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
