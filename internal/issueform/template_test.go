package issueform

import "testing"

func TestParseTemplateKeepsEditableFieldsInOrder(t *testing.T) {
	tmpl, err := Parse(".github/ISSUE_TEMPLATE/daily-update.yml", []byte(`
name: Daily Update
title: "[Daily Update] [YYYY/MM/DD]"
body:
  - type: markdown
    attributes:
      value: |
        ## Daily Standup Update

  - type: textarea
    id: yesterday
    attributes:
      label: "✅ What did you do yesterday?"
      description: Yesterday work
      placeholder: |
        - Reviewed PR #123
    validations:
      required: true

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

	if tmpl.Path != ".github/ISSUE_TEMPLATE/daily-update.yml" {
		t.Fatalf("unexpected template path %q", tmpl.Path)
	}

	editable := tmpl.EditableFields()
	if len(editable) != 2 {
		t.Fatalf("expected 2 editable fields, got %d", len(editable))
	}

	if editable[0].ID != "yesterday" || editable[0].Type != FieldTextarea {
		t.Fatalf("unexpected first field %#v", editable[0])
	}

	if editable[1].ID != "parking_lot" || editable[1].Type != FieldCheckboxes {
		t.Fatalf("unexpected second field %#v", editable[1])
	}

	if editable[1].Options[0].Label != "✅ Yes, I need a Parking Lot or escalation" {
		t.Fatalf("unexpected checkbox option %#v", editable[1].Options)
	}
}

func TestParseTemplateRejectsCheckboxesWithMultipleOptions(t *testing.T) {
	_, err := Parse("daily.yml", []byte(`
body:
  - type: checkboxes
    id: multi
    attributes:
      label: Pick many
      options:
        - label: One
        - label: Two
`))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
