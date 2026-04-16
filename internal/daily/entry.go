package daily

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vieitesss/pad/internal/issueform"
)

const DateLayout = "2006-01-02"

// Legacy title formats (used as fallback when template has no title)
const issueTitlePrefix = "[Daily Update] ["
const reportTitlePrefix = "[Daily Report] "
const carryoverFieldID = "pad_carryover"

type IssueRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type Response struct {
	Text    string `json:"text,omitempty"`
	Checked bool   `json:"checked,omitempty"`
}

type Entry struct {
	Date      string              `json:"date"`
	Template  issueform.Template  `json:"-"`
	Responses map[string]Response `json:"responses,omitempty"`
	Source    string              `json:"source,omitempty"`
	Issue     *IssueRef           `json:"issue,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type parsedSection struct {
	Heading string
	ID      string
	Body    string
}

var headingPattern = regexp.MustCompile(`^#{2,6}\s*(.*?)\s*(?:<!--\s*pad:id:([A-Za-z0-9._-]+)\s*-->)?\s*$`)

var legacyFieldAliases = map[string][]string{
	"yesterday":       {"✅ What did you do yesterday?"},
	"today":           {"🎯 What will you do today?"},
	"blockers":        {"🚧 Any blockers?"},
	"parking_lot":     {"🚨 Do you request a Parking Lot or escalation?", "🚨 Parking Lot / Escalation"},
	"parking_details": {"📝 Parking Lot Details", "🚨 Parking Lot / Escalation"},
	"comments":        {"💬 Additional Comments"},
}

func New(date string, template issueform.Template) Entry {
	return Entry{
		Date:      date,
		Template:  template,
		Responses: make(map[string]Response),
		Source:    "manual",
	}
}

func EntryFromIssueBody(date string, template issueform.Template, body string) Entry {
	entry := New(date, template)
	sections := parseSections(body, template)
	used := make(map[int]bool, len(sections))

	for _, field := range template.EditableFields() {
		index, ok := matchSection(field, sections)
		if !ok {
			continue
		}

		used[index] = true
		applySection(&entry, field, sections[index])
	}

	if carryover := buildCarryoverBody(sections, used); carryover != "" {
		entry.Template = entry.Template.WithAppendedField(issueform.Field{
			Type:        issueform.FieldTextarea,
			ID:          carryoverFieldID,
			Label:       "🗂 Carryover From Previous Template",
			Description: "Responses from fields that no longer exist in the current template. Keep, move, or clear before creating.",
		})
		entry.SetText(carryoverFieldID, carryover)
	}

	return entry.Normalize()
}

// Title generates the issue title from the template title, replacing date placeholders.
// Falls back to the legacy format if no template title is available.
func (e Entry) Title() (string, error) {
	templateTitle := strings.TrimSpace(e.Template.Title)
	if templateTitle == "" {
		// Fallback to legacy format
		return TitleForDate(e.Date)
	}

	// Parse the date
	t, err := time.Parse(DateLayout, e.Date)
	if err != nil {
		return "", fmt.Errorf("parse entry date: %w", err)
	}

	// Replace common date placeholders
	title := templateTitle
	replacements := map[string]string{
		"YYYY/MM/DD": t.Format("2006/01/02"),
		"YYYY-MM-DD": t.Format("2006-01-02"),
		"DD/MM/YYYY": t.Format("02/01/2006"),
		"MM/DD/YYYY": t.Format("01/02/2006"),
	}

	for placeholder, value := range replacements {
		title = strings.ReplaceAll(title, placeholder, value)
	}

	return title, nil
}

// TitleForDate generates the legacy title format (used as fallback).
func TitleForDate(date string) (string, error) {
	return titleForDate(date, issueTitlePrefix, "2006/01/02", "]")
}

func ReportTitleForDate(date string) (string, error) {
	return titleForDate(date, reportTitlePrefix, "2006/01/02", "")
}

// DateFromIssueTitle extracts the date from an issue title.
// It tries the template title format first, then falls back to legacy formats.
func DateFromIssueTitle(title string) (string, bool) {
	// Try to extract date using common patterns
	date, ok := extractDateFromTemplateTitle(title)
	if ok {
		return date, true
	}

	// Fall back to legacy format
	return dateFromTitle(title, issueTitlePrefix, "]")
}

// extractDateFromTemplateTitle tries to extract a date from a template-style title.
func extractDateFromTemplateTitle(title string) (string, bool) {
	// Common date patterns in titles
	patterns := []string{
		"2006/01/02", // YYYY/MM/DD
		"2006-01-02", // YYYY-MM-DD
		"01/02/2006", // MM/DD/YYYY
		"02/01/2006", // DD/MM/YYYY
	}

	for _, pattern := range patterns {
		// Try to find a substring matching this pattern
		if idx := findDatePattern(title, pattern); idx != -1 {
			dateStr := title[idx : idx+len(pattern)]
			t, err := time.Parse(pattern, dateStr)
			if err == nil {
				return t.Format(DateLayout), true
			}
		}
	}

	return "", false
}

// findDatePattern finds the start index of a date pattern in a string.
func findDatePattern(s, pattern string) int {
	// Simple heuristic: look for a substring that matches the pattern structure
	// Count the number of digits and separators
	digitCount := 0
	sepCount := 0
	for _, r := range pattern {
		if r >= '0' && r <= '9' {
			digitCount++
		} else {
			sepCount++
		}
	}

	// Look for potential matches
	for i := 0; i <= len(s)-len(pattern); i++ {
		substr := s[i : i+len(pattern)]
		if matchesPattern(substr, pattern) {
			return i
		}
	}

	return -1
}

// matchesPattern checks if a string matches a date pattern structure.
func matchesPattern(s, pattern string) bool {
	if len(s) != len(pattern) {
		return false
	}

	for i := 0; i < len(pattern); i++ {
		patternChar := pattern[i]
		strChar := s[i]

		if patternChar >= '0' && patternChar <= '9' {
			// Pattern expects a digit
			if strChar < '0' || strChar > '9' {
				return false
			}
		} else {
			// Pattern expects a specific separator
			if strChar != patternChar {
				return false
			}
		}
	}

	return true
}

func DateFromReportTitle(title string) (string, bool) {
	return dateFromTitle(title, reportTitlePrefix, "")
}

func (e Entry) Body() string {
	blocks := make([]string, 0, len(e.Template.Fields))
	for _, field := range e.Template.Fields {
		switch field.Type {
		case issueform.FieldMarkdown:
			if strings.TrimSpace(field.Markdown) == "" {
				continue
			}
			blocks = append(blocks, field.Markdown)
		case issueform.FieldTextarea, issueform.FieldInput:
			blocks = append(blocks, renderSection(field, normalizeSectionBody(e.Text(field.ID))))
		case issueform.FieldCheckboxes:
			blocks = append(blocks, renderSection(field, renderCheckboxBody(field, e.Checked(field.ID))))
		}
	}

	return strings.Join(blocks, "\n\n")
}

func (e Entry) ValidateForCreate() error {
	for _, field := range e.Template.EditableFields() {
		if !field.Required {
			continue
		}

		switch field.Type {
		case issueform.FieldCheckboxes:
			if e.Checked(field.ID) {
				continue
			}
		default:
			if strings.TrimSpace(e.Text(field.ID)) != "" {
				continue
			}
		}

		return fmt.Errorf("section %q is required; fill it in with `pad create` or `pad repeat`", field.Label)
	}

	return nil
}

func (e Entry) Normalize() Entry {
	if e.Responses == nil {
		e.Responses = make(map[string]Response)
	}

	for key, response := range e.Responses {
		response.Text = strings.TrimSpace(response.Text)
		e.Responses[key] = response
	}

	e.Source = strings.TrimSpace(e.Source)
	return e
}

func (e Entry) Text(fieldID string) string {
	return e.Responses[fieldID].Text
}

func (e *Entry) SetText(fieldID, value string) {
	if e.Responses == nil {
		e.Responses = make(map[string]Response)
	}

	response := e.Responses[fieldID]
	response.Text = value
	e.Responses[fieldID] = response
}

func (e Entry) Checked(fieldID string) bool {
	return e.Responses[fieldID].Checked
}

func (e *Entry) SetChecked(fieldID string, checked bool) {
	if e.Responses == nil {
		e.Responses = make(map[string]Response)
	}

	response := e.Responses[fieldID]
	response.Checked = checked
	e.Responses[fieldID] = response
}

func titleForDate(date, prefix, format, suffix string) (string, error) {
	t, err := time.Parse(DateLayout, date)
	if err != nil {
		return "", fmt.Errorf("parse entry date: %w", err)
	}

	return fmt.Sprintf("%s%s%s", prefix, t.Format(format), suffix), nil
}

func dateFromTitle(title, prefix, suffix string) (string, bool) {
	if !strings.HasPrefix(title, prefix) || !strings.HasSuffix(title, suffix) {
		return "", false
	}

	raw := strings.TrimSuffix(strings.TrimPrefix(title, prefix), suffix)
	t, err := time.Parse("2006/01/02", raw)
	if err != nil {
		return "", false
	}

	return t.Format(DateLayout), true
}

func renderSection(field issueform.Field, body string) string {
	heading := "## " + field.Label
	if field.ID != "" {
		heading += " <!-- pad:id:" + field.ID + " -->"
	}
	return heading + "\n" + body
}

func renderCheckboxBody(field issueform.Field, checked bool) string {
	lines := make([]string, 0, len(field.Options))
	for index, option := range field.Options {
		marker := " "
		if checked && index == 0 {
			marker = "x"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", marker, option.Label))
	}
	return strings.Join(lines, "\n")
}

func parseSections(body string, template issueform.Template) []parsedSection {
	sections := make([]parsedSection, 0)
	current := parsedSection{}
	active := false
	allowedHeadings := collectAllowedHeadings(template)

	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		heading, id, ok := parseHeading(line)
		if ok && shouldStartSection(heading, id, allowedHeadings) {
			if active {
				current.Body = strings.TrimSpace(current.Body)
				sections = append(sections, current)
			}
			current = parsedSection{Heading: heading, ID: id}
			active = true
			continue
		}

		if !active {
			continue
		}

		if current.Body == "" {
			current.Body = line
			continue
		}
		current.Body += "\n" + line
	}

	if active {
		current.Body = strings.TrimSpace(current.Body)
		sections = append(sections, current)
	}

	return sections
}

func collectAllowedHeadings(template issueform.Template) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, field := range template.EditableFields() {
		allowed[normalizeHeading(field.Label)] = struct{}{}
	}
	for _, aliases := range legacyFieldAliases {
		for _, alias := range aliases {
			allowed[normalizeHeading(alias)] = struct{}{}
		}
	}
	return allowed
}

func shouldStartSection(heading, id string, allowed map[string]struct{}) bool {
	if id != "" {
		return true
	}
	_, ok := allowed[normalizeHeading(heading)]
	return ok
}

func parseHeading(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", "", false
	}

	match := headingPattern.FindStringSubmatch(trimmed)
	if len(match) == 0 {
		return "", "", false
	}

	return strings.TrimSpace(match[1]), strings.TrimSpace(match[2]), true
}

func matchSection(field issueform.Field, sections []parsedSection) (int, bool) {
	if field.ID != "" {
		for index, section := range sections {
			if section.ID == field.ID {
				return index, true
			}
		}
	}

	wanted := append([]string{normalizeHeading(field.Label)}, normalizedAliases(field.ID)...)
	for index, section := range sections {
		heading := normalizeHeading(section.Heading)
		for _, alias := range wanted {
			if alias == heading {
				return index, true
			}
		}
	}

	return 0, false
}

func normalizedAliases(fieldID string) []string {
	aliases := legacyFieldAliases[fieldID]
	normalized := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		normalized = append(normalized, normalizeHeading(alias))
	}
	return normalized
}

func normalizeHeading(heading string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(heading)), " "))
}

func applySection(entry *Entry, field issueform.Field, section parsedSection) {
	switch field.Type {
	case issueform.FieldCheckboxes:
		entry.SetChecked(field.ID, parseCheckboxValue(field, section))
	case issueform.FieldTextarea, issueform.FieldInput:
		if normalizeHeading(section.Heading) == normalizeHeading("🚨 Parking Lot / Escalation") && field.ID == "parking_details" {
			_, details := parseCombinedParkingLot(section.Body)
			entry.SetText(field.ID, details)
			return
		}
		entry.SetText(field.ID, normalizeParsedSection(section.Body))
	}
}

func parseCheckboxValue(field issueform.Field, section parsedSection) bool {
	if normalizeHeading(section.Heading) == normalizeHeading("🚨 Parking Lot / Escalation") {
		checked, _ := parseCombinedParkingLot(section.Body)
		return checked
	}

	body := strings.ToLower(strings.TrimSpace(section.Body))
	if strings.Contains(body, "[x]") {
		return true
	}

	for _, option := range field.Options {
		if strings.Contains(body, strings.ToLower(option.Label)) && !strings.Contains(body, "[ ]") {
			return true
		}
	}

	return false
}

func buildCarryoverBody(sections []parsedSection, used map[int]bool) string {
	blocks := make([]string, 0)
	for index, section := range sections {
		if used[index] {
			continue
		}

		body := normalizeParsedSection(section.Body)
		if body == "" {
			continue
		}

		heading := section.Heading
		if heading == "" {
			heading = "Previous Response"
		}
		blocks = append(blocks, fmt.Sprintf("### %s\n%s", heading, body))
	}

	return strings.Join(blocks, "\n\n")
}

func normalizeSectionBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "_None._"
	}

	return trimmed
}

func parseCombinedParkingLot(body string) (bool, string) {
	parkingLot := false
	detailLines := make([]string, 0)

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(detailLines) > 0 {
				detailLines = append(detailLines, line)
			}
			continue
		}

		if strings.Contains(trimmed, "Yes, I need a Parking Lot or escalation") || strings.Contains(strings.ToLower(trimmed), "[x]") {
			parkingLot = true
			continue
		}

		detailLines = append(detailLines, line)
	}

	details := normalizeParsedSection(strings.Join(detailLines, "\n"))
	if details != "" {
		parkingLot = true
	}

	return parkingLot, details
}

func normalizeParsedSection(body string) string {
	trimmed := strings.TrimSpace(body)
	switch strings.ToLower(trimmed) {
	case "", "_no response_", "_none._", "none":
		return ""
	default:
		return trimmed
	}
}
