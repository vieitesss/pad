package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/prefapp/pad/internal/daily"
)

var ErrCanceled = errors.New("edit canceled")

const (
	yesterdayField = iota
	todayField
	blockersField
	parkingLotField
	parkingLotDetailsField
	additionalCommentsField
)

type fieldKind int

const (
	textField fieldKind = iota
	boolField
)

type editorMode int

const (
	modeSave editorMode = iota
	modeCreate
)

type fieldDef struct {
	Title       string
	Description string
	Kind        fieldKind
	Placeholder string
}

var fieldDefs = []fieldDef{
	{
		Title:       "✅ What did you do yesterday?",
		Description: "Describe what you accomplished yesterday.",
		Kind:        textField,
		Placeholder: "- Reviewed PR #123\n- Finished API changes",
	},
	{
		Title:       "🎯 What will you do today?",
		Description: "Outline your plans for today.",
		Kind:        textField,
		Placeholder: "- Continue feature work\n- Write documentation",
	},
	{
		Title:       "🚧 Any blockers?",
		Description: "Optional. Mention obstacles you are facing.",
		Kind:        textField,
		Placeholder: "- Waiting for code review\n- Need clarification on scope",
	},
	{
		Title:       "🚨 Request a Parking Lot or escalation?",
		Description: "Toggle this on when you need escalation or want to add a topic to the Parking Lot.",
		Kind:        boolField,
	},
	{
		Title:       "📝 Parking Lot Details",
		Description: "Only used when escalation is enabled.",
		Kind:        textField,
		Placeholder: "- Need clarification on API contract changes",
	},
	{
		Title:       "💬 Additional Comments",
		Description: "Optional extra notes or context for the team.",
		Kind:        textField,
		Placeholder: "- Offline after 17:00\n- Waiting for design confirmation",
	},
}

var (
	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	paneStyle           = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(1)
	paneTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	currentFieldStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	mutedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	confirmStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	helpTextStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	navKeyStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	actionKeyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	previewKeyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	cancelKeyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	secondaryKeyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	helpDividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusFilledStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusEmptyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusDisabledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type model struct {
	mode           editorMode
	entry          daily.Entry
	index          int
	editor         textarea.Model
	preview        viewport.Model
	width          int
	height         int
	message        string
	messageIsError bool
	confirm        bool
	submitted      bool
	canceled       bool
	result         daily.Entry
}

func Edit(entry daily.Entry) (daily.Entry, error) {
	return run(entry, modeSave)
}

func EditForCreate(entry daily.Entry) (daily.Entry, error) {
	return run(entry, modeCreate)
}

func run(entry daily.Entry, mode editorMode) (daily.Entry, error) {
	program := tea.NewProgram(newModel(entry, mode), tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		return daily.Entry{}, err
	}

	final, ok := finalModel.(model)
	if !ok {
		return daily.Entry{}, fmt.Errorf("unexpected tea model type %T", finalModel)
	}

	if final.canceled {
		return daily.Entry{}, ErrCanceled
	}

	if !final.submitted {
		return daily.Entry{}, fmt.Errorf("editor exited without completing the action")
	}

	return final.result, nil
}

func newModel(entry daily.Entry, mode editorMode) model {
	editor := textarea.New()
	editor.ShowLineNumbers = false
	editor.Prompt = ""
	editor.CharLimit = 0
	editor.SetWidth(40)
	editor.SetHeight(8)

	preview := viewport.New(40, 8)

	m := model{
		mode:    mode,
		entry:   entry.Normalize(),
		index:   yesterdayField,
		editor:  editor,
		preview: preview,
		width:   120,
		height:  32,
	}
	m.syncEditor()
	m.resizePanes()
	m.refreshPreview()
	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePanes()
		m.refreshPreview()
		return m, nil

	case tea.KeyMsg:
		if m.confirm {
			return m.updateConfirmation(msg)
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			return m, tea.Quit
		case "ctrl+s":
			return m.handlePrimaryAction()
		case "tab":
			m.move(1)
			return m, textarea.Blink
		case "shift+tab":
			m.move(-1)
			return m, textarea.Blink
		case "pgup", "pgdown":
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}

		if m.currentField().Kind == boolField {
			switch msg.String() {
			case " ", "enter":
				m.entry.ParkingLot = !m.entry.ParkingLot
				m.refreshPreview()
				return m, nil
			}

			return m, nil
		}
	}

	if m.currentField().Kind != textField {
		return m, nil
	}

	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	m.persistCurrentField()
	m.refreshPreview()
	return m, cmd
}

func (m model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "ctrl+c":
		m.canceled = true
		return m, tea.Quit
	case "esc", "n":
		m.confirm = false
		return m, nil
	case "y", "enter":
		entry := m.finalEntry()
		if err := entry.ValidateForCreate(); err != nil {
			m.message = err.Error()
			m.messageIsError = true
			m.confirm = false
			m.refreshPreview()
			return m, nil
		}

		m.result = entry
		m.submitted = true
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m model) handlePrimaryAction() (tea.Model, tea.Cmd) {
	entry := m.finalEntry()
	if m.mode == modeSave {
		m.result = entry
		m.submitted = true
		return m, tea.Quit
	}

	if err := entry.ValidateForCreate(); err != nil {
		m.message = err.Error()
		m.messageIsError = true
		m.refreshPreview()
		return m, nil
	}

	m.message = ""
	m.messageIsError = false
	m.confirm = true
	m.refreshPreview()
	return m, nil
}

func (m model) View() string {
	left := paneStyle.Width(m.leftPaneWidth()).Height(m.leftPaneHeight()).Render(m.leftPaneView())
	right := paneStyle.Width(m.rightPaneWidth()).Height(m.rightPaneHeight()).Render(m.rightPaneView())

	headerLines := []string{
		headerStyle.Render(fmt.Sprintf("pad  %s  %s", m.actionTitle(), m.entry.Date)),
	}
	if m.entry.Source != "" {
		headerLines = append(headerLines, mutedStyle.Render("Source: "+m.entry.Source))
	}

	content := []string{
		lipgloss.JoinVertical(lipgloss.Left, headerLines...),
		m.bodyView(left, right),
		m.footerView(),
	}

	return lipgloss.JoinVertical(lipgloss.Left, content...)
}

func (m model) bodyView(left, right string) string {
	if m.splitLayout() {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	return lipgloss.JoinVertical(lipgloss.Left, left, right)
}

func (m model) leftPaneView() string {
	fieldLines := make([]string, 0, len(fieldDefs))
	for index, def := range fieldDefs {
		prefix := "  "
		titleStyle := mutedStyle
		if index == m.index {
			prefix = "> "
			titleStyle = currentFieldStyle
		}

		fieldLines = append(fieldLines, fmt.Sprintf("%s%s %s", prefix, titleStyle.Render(def.Title), m.fieldStatus(index)))
	}

	current := m.currentField()
	editingBlock := []string{
		paneTitleStyle.Render("Template"),
		mutedStyle.Render("Fill the async-daily template on the left. The right pane updates live."),
		"",
		strings.Join(fieldLines, "\n"),
		"",
		paneTitleStyle.Render("Editing"),
		currentFieldStyle.Render(current.Title),
		mutedStyle.Render(current.Description),
		"",
	}

	if current.Kind == boolField {
		value := "No"
		if m.entry.ParkingLot {
			value = "Yes"
		}
		editingBlock = append(editingBlock,
			fmt.Sprintf("Current value: %s", currentFieldStyle.Render(value)),
			mutedStyle.Render("Press space or enter to toggle this field."),
		)
	} else {
		editingBlock = append(editingBlock, m.editor.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, editingBlock...)
}

func (m model) rightPaneView() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		paneTitleStyle.Render("Live Preview"),
		mutedStyle.Render("Use pgup/pgdown when the preview is longer than the screen."),
		"",
		m.preview.View(),
	)
}

func (m model) footerView() string {
	lines := make([]string, 0, 2)
	if m.message != "" {
		style := mutedStyle
		if m.messageIsError {
			style = errorStyle
		}
		lines = append(lines, style.Render(m.message))
	}

	if m.confirm {
		lines = append(lines, lipgloss.JoinHorizontal(
			lipgloss.Top,
			confirmStyle.Render(fmt.Sprintf("Create GitHub issue for %s? ", m.entry.Date)),
			helpItem(actionKeyStyle, "enter/y", "confirm"),
			helpDividerStyle.Render("  •  "),
			helpItem(secondaryKeyStyle, "n", "go back"),
			helpDividerStyle.Render("  •  "),
			helpItem(cancelKeyStyle, "esc", "go back"),
		))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	primaryKeyStyle := actionKeyStyle
	primaryDescription := "save"
	if m.mode == modeCreate {
		primaryDescription = "create"
	}

	lines = append(lines, lipgloss.JoinHorizontal(
		lipgloss.Top,
		helpItem(navKeyStyle, "tab", "next field"),
		helpDividerStyle.Render("  •  "),
		helpItem(navKeyStyle, "shift+tab", "previous field"),
		helpDividerStyle.Render("  •  "),
		helpItem(primaryKeyStyle, "ctrl+s", primaryDescription),
		helpDividerStyle.Render("  •  "),
		helpItem(previewKeyStyle, "pgup/pgdn", "scroll preview"),
		helpDividerStyle.Render("  •  "),
		helpItem(cancelKeyStyle, "esc", "cancel"),
	))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func helpItem(keyStyle lipgloss.Style, key, description string) string {
	return keyStyle.Render(key) + helpTextStyle.Render(" "+description)
}

func (m model) actionTitle() string {
	if m.mode == modeCreate {
		return "Create Async Daily"
	}

	return "Edit Draft"
}

func (m *model) move(step int) {
	m.persistCurrentField()
	m.index = nextVisibleIndex(m.index, step, m.entry.ParkingLot)
	m.syncEditor()
	m.refreshPreview()
}

func (m *model) syncEditor() {
	if m.currentField().Kind != textField {
		m.editor.Blur()
		return
	}

	m.editor.Focus()
	m.editor.Placeholder = m.currentField().Placeholder
	m.editor.SetValue(m.storedTextValue(m.index))
	m.resizeEditor()
}

func (m *model) resizePanes() {
	m.resizeEditor()
	m.preview.Width = m.previewWidth()
	m.preview.Height = m.previewHeight()
}

func (m *model) resizeEditor() {
	width := m.leftPaneWidth() - 6
	if width < 20 {
		width = 20
	}

	height := m.leftPaneHeight() - 16
	if height < 6 {
		height = 6
	}

	m.editor.SetWidth(width)
	m.editor.SetHeight(height)
}

func (m *model) refreshPreview() {
	content := previewContent(m.finalEntry())
	if m.preview.Width > 0 {
		content = lipgloss.NewStyle().Width(m.preview.Width).Render(content)
	}
	m.preview.SetContent(content)
}

func previewContent(entry daily.Entry) string {
	title, err := entry.Title()
	if err != nil {
		title = entry.Date
	}

	return fmt.Sprintf("%s\n\n%s", title, entry.Body())
}

func (m *model) persistCurrentField() {
	if m.currentField().Kind != textField {
		return
	}

	m.setTextValue(m.index, m.editor.Value())
}

func (m model) finalEntry() daily.Entry {
	entry := m.entry
	if m.currentField().Kind == textField {
		switch m.index {
		case yesterdayField:
			entry.Yesterday = m.editor.Value()
		case todayField:
			entry.Today = m.editor.Value()
		case blockersField:
			entry.Blockers = m.editor.Value()
		case parkingLotDetailsField:
			entry.ParkingLotDetails = m.editor.Value()
		case additionalCommentsField:
			entry.AdditionalComments = m.editor.Value()
		}
	}

	if !entry.ParkingLot {
		entry.ParkingLotDetails = ""
	}

	return entry.Normalize()
}

func (m model) currentField() fieldDef {
	return fieldDefs[m.index]
}

func (m model) fieldStatus(index int) string {
	var status string
	var style lipgloss.Style

	switch index {
	case parkingLotField:
		if m.entry.ParkingLot {
			status = "[yes]"
			style = statusFilledStyle
		} else {
			status = "[no]"
			style = statusEmptyStyle
		}
		return style.Render(status)
	case parkingLotDetailsField:
		if !m.entry.ParkingLot {
			return statusDisabledStyle.Render("[disabled]")
		}
	}

	if strings.TrimSpace(m.textValue(index)) == "" {
		return statusEmptyStyle.Render("[empty]")
	}

	return statusFilledStyle.Render("[filled]")
}

func (m model) textValue(index int) string {
	if index == m.index && m.currentField().Kind == textField {
		return m.editor.Value()
	}

	return m.storedTextValue(index)
}

func (m model) storedTextValue(index int) string {

	switch index {
	case yesterdayField:
		return m.entry.Yesterday
	case todayField:
		return m.entry.Today
	case blockersField:
		return m.entry.Blockers
	case parkingLotDetailsField:
		return m.entry.ParkingLotDetails
	case additionalCommentsField:
		return m.entry.AdditionalComments
	default:
		return ""
	}
}

func (m *model) setTextValue(index int, value string) {
	switch index {
	case yesterdayField:
		m.entry.Yesterday = value
	case todayField:
		m.entry.Today = value
	case blockersField:
		m.entry.Blockers = value
	case parkingLotDetailsField:
		m.entry.ParkingLotDetails = value
	case additionalCommentsField:
		m.entry.AdditionalComments = value
	}
}

func (m model) splitLayout() bool {
	return m.width >= 80
}

func (m model) leftPaneWidth() int {
	if !m.splitLayout() {
		width := m.width - 4
		if width < 30 {
			width = 30
		}
		return width
	}

	width := (m.width - 1) / 2
	if width < 30 {
		width = 30
	}
	return width
}

func (m model) rightPaneWidth() int {
	if !m.splitLayout() {
		width := m.width - 4
		if width < 30 {
			width = 30
		}
		return width
	}

	width := m.width - m.leftPaneWidth() - 1
	if width < 30 {
		width = 30
	}
	return width
}

func (m model) previewWidth() int {
	width := m.rightPaneWidth() - 4
	if width < 16 {
		width = 16
	}
	return width
}

func (m model) previewHeight() int {
	height := m.rightPaneHeight() - 5
	if height < 8 {
		height = 8
	}
	return height
}

func (m model) leftPaneHeight() int {
	if !m.splitLayout() {
		height := (m.height - 6) / 2
		if height < 14 {
			height = 14
		}
		return height
	}

	height := m.height - 5
	if height < 14 {
		height = 14
	}
	return height
}

func (m model) rightPaneHeight() int {
	if !m.splitLayout() {
		height := m.height - m.leftPaneHeight() - 6
		if height < 14 {
			height = 14
		}
		return height
	}

	height := m.height - 5
	if height < 14 {
		height = 14
	}
	return height
}

func nextVisibleIndex(current, step int, parkingLot bool) int {
	candidate := current + step

	// Wrap around boundaries
	if candidate < 0 {
		candidate = len(fieldDefs) - 1
	} else if candidate >= len(fieldDefs) {
		candidate = 0
	}

	// Skip parking lot details if disabled
	if candidate == parkingLotDetailsField && !parkingLot {
		// Continue in same direction, but prevent infinite loop
		if step == 0 {
			step = 1
		}
		return nextVisibleIndex(candidate, step, parkingLot)
	}

	return candidate
}
