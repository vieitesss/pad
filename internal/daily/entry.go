package daily

import (
	"fmt"
	"strings"
	"time"
)

const DateLayout = "2006-01-02"

const issueTitlePrefix = "[Daily Update] ["
const reportTitlePrefix = "[Daily Report] "

type IssueRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type Entry struct {
	Date               string    `json:"date"`
	Yesterday          string    `json:"yesterday"`
	Today              string    `json:"today"`
	Blockers           string    `json:"blockers,omitempty"`
	ParkingLot         bool      `json:"parking_lot,omitempty"`
	ParkingLotDetails  string    `json:"parking_lot_details,omitempty"`
	AdditionalComments string    `json:"additional_comments,omitempty"`
	Source             string    `json:"source,omitempty"`
	Issue              *IssueRef `json:"issue,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func New(date string) Entry {
	return Entry{
		Date:   date,
		Source: "manual",
	}
}

func EntryFromIssueBody(date, body string) Entry {
	sections := parseSections(body)
	parkingLot, parkingLotDetails := parseParkingLotSections(sections)

	return Entry{
		Date:               date,
		Yesterday:          normalizeParsedSection(sections["yesterday"]),
		Today:              normalizeParsedSection(sections["today"]),
		Blockers:           normalizeParsedSection(sections["blockers"]),
		ParkingLot:         parkingLot,
		ParkingLotDetails:  parkingLotDetails,
		AdditionalComments: normalizeParsedSection(sections["comments"]),
	}
}

func (e Entry) Title() (string, error) {
	return TitleForDate(e.Date)
}

func TitleForDate(date string) (string, error) {
	return titleForDate(date, issueTitlePrefix, "2006/01/02", "]")
}

func ReportTitleForDate(date string) (string, error) {
	return titleForDate(date, reportTitlePrefix, "2006/01/02", "")
}

func DateFromIssueTitle(title string) (string, bool) {
	return dateFromTitle(title, issueTitlePrefix, "]")
}

func DateFromReportTitle(title string) (string, bool) {
	return dateFromTitle(title, reportTitlePrefix, "")
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

func (e Entry) Body() string {
	sections := []section{
		{Title: "✅ What did you do yesterday?", Body: e.Yesterday},
		{Title: "🎯 What will you do today?", Body: e.Today},
		{Title: "🚧 Any blockers?", Body: e.Blockers},
	}

	parkingLotBody := buildParkingLotBody(e)
	if parkingLotBody != "" {
		sections = append(sections, section{Title: "🚨 Parking Lot / Escalation", Body: parkingLotBody})
	}

	if strings.TrimSpace(e.AdditionalComments) != "" {
		sections = append(sections, section{Title: "💬 Additional Comments", Body: e.AdditionalComments})
	}

	var body strings.Builder
	for index, section := range sections {
		if index > 0 {
			body.WriteString("\n\n")
		}

		body.WriteString("## ")
		body.WriteString(section.Title)
		body.WriteString("\n")
		body.WriteString(normalizeSectionBody(section.Body))
	}

	return body.String()
}

func (e Entry) ValidateForCreate() error {
	if strings.TrimSpace(e.Yesterday) == "" {
		return fmt.Errorf("yesterday section is required; fill it in with `pad create` or `pad repeat`")
	}

	if strings.TrimSpace(e.Today) == "" {
		return fmt.Errorf("today section is required; fill it in with `pad create` or `pad repeat`")
	}

	return nil
}

func (e Entry) Normalize() Entry {
	e.Yesterday = strings.TrimSpace(e.Yesterday)
	e.Today = strings.TrimSpace(e.Today)
	e.Blockers = strings.TrimSpace(e.Blockers)
	e.ParkingLotDetails = strings.TrimSpace(e.ParkingLotDetails)
	e.AdditionalComments = strings.TrimSpace(e.AdditionalComments)
	e.Source = strings.TrimSpace(e.Source)
	return e
}

type section struct {
	Title string
	Body  string
}

func normalizeSectionBody(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "_None._"
	}

	return trimmed
}

func buildParkingLotBody(e Entry) string {
	var parts []string

	if e.ParkingLot {
		parts = append(parts, "- ✅ Yes, I need a Parking Lot or escalation")
	}

	if details := strings.TrimSpace(e.ParkingLotDetails); details != "" {
		parts = append(parts, details)
	}

	return strings.Join(parts, "\n\n")
}

func parseSections(body string) map[string]string {
	sections := make(map[string][]string)
	current := ""

	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if key, ok := sectionKey(strings.TrimSpace(line)); ok {
			current = key
			continue
		}

		if current == "" {
			continue
		}

		sections[current] = append(sections[current], line)
	}

	parsed := make(map[string]string, len(sections))
	for key, lines := range sections {
		parsed[key] = strings.TrimSpace(strings.Join(lines, "\n"))
	}

	return parsed
}

func sectionKey(line string) (string, bool) {
	if !strings.HasPrefix(line, "##") {
		return "", false
	}

	heading := strings.TrimSpace(strings.TrimLeft(line, "# "))
	switch heading {
	case "✅ What did you do yesterday?":
		return "yesterday", true
	case "🎯 What will you do today?":
		return "today", true
	case "🚧 Any blockers?":
		return "blockers", true
	case "🚨 Do you request a Parking Lot or escalation?":
		return "parking_checkbox", true
	case "🚨 Parking Lot / Escalation":
		return "parking_combined", true
	case "📝 Parking Lot Details":
		return "parking_details", true
	case "💬 Additional Comments":
		return "comments", true
	default:
		return "", false
	}
}

func parseParkingLotSections(sections map[string]string) (bool, string) {
	parkingLot := false
	details := normalizeParsedSection(sections["parking_details"])

	checkbox := normalizeParsedSection(sections["parking_checkbox"])
	if strings.Contains(strings.ToLower(checkbox), "[x]") {
		parkingLot = true
	}

	combined := normalizeParsedSection(sections["parking_combined"])
	if combined != "" {
		combinedParkingLot, combinedDetails := parseCombinedParkingLot(combined)
		if combinedParkingLot {
			parkingLot = true
		}
		if details == "" {
			details = combinedDetails
		}
	}

	if details != "" {
		parkingLot = true
	}

	return parkingLot, details
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

		if strings.Contains(trimmed, "Yes, I need a Parking Lot or escalation") {
			parkingLot = true
			continue
		}

		detailLines = append(detailLines, line)
	}

	details := normalizeParsedSection(strings.TrimSpace(strings.Join(detailLines, "\n")))
	if details != "" {
		parkingLot = true
	}

	return parkingLot, details
}

func normalizeParsedSection(body string) string {
	trimmed := strings.TrimSpace(body)
	switch trimmed {
	case "", "_No response_", "_None._":
		return ""
	default:
		return trimmed
	}
}
