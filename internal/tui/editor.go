package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vieitesss/pad/internal/daily"
	"github.com/vieitesss/pad/internal/issueform"
)

var ErrCanceled = errors.New("edit canceled")

var writeClipboard = clipboard.WriteAll

type editorMode int

const (
	modeSave editorMode = iota
	modeCreate
)

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
	mode             editorMode
	entry            daily.Entry
	fields           []issueform.Field
	index            int
	editor           textarea.Model
	preview          viewport.Model
	width            int
	height           int
	message          string
	messageIsError   bool
	clipboard        string
	clipboardSynced  bool
	clipboardMessage string
	clipboardCount   int
	confirm          bool
	submitted        bool
	canceled         bool
	result           daily.Entry
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
	fields := entry.Template.EditableFields()

	m := model{
		mode:    mode,
		entry:   entry.Normalize(),
		fields:  fields,
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
		m.clearClipboardMessageOnKey(msg)

		if m.confirm {
			return m.updateConfirmation(msg)
		}

		if m.currentFieldIsText() {
			switch msg.String() {
			case "ctrl+c":
				m.copyCurrentField(false)
				return m, nil
			case "ctrl+x":
				m.copyCurrentField(true)
				return m, nil
			case "ctrl+v":
				if !m.clipboardSynced && m.clipboard != "" {
					m.pasteClipboard()
					return m, nil
				}
			}
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

		if m.currentField().Type == issueform.FieldCheckboxes {
			switch msg.String() {
			case " ", "enter":
				m.entry.SetChecked(m.currentField().ID, !m.entry.Checked(m.currentField().ID))
				m.refreshPreview()
				return m, nil
			}

			return m, nil
		}
	}

	if !m.currentFieldIsText() {
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
			m.setMessage(err.Error(), true)
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
		m.setMessage(err.Error(), true)
		m.refreshPreview()
		return m, nil
	}

	m.setMessage("", false)
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
	if strings.TrimSpace(m.entry.Template.Path) != "" {
		headerLines = append(headerLines, mutedStyle.Render("Template: "+m.entry.Template.Path))
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
	fieldLines := make([]string, 0, len(m.fields))
	for index, field := range m.fields {
		prefix := "  "
		titleStyle := mutedStyle
		if index == m.index {
			prefix = "> "
			titleStyle = currentFieldStyle
		}

		fieldLines = append(fieldLines, fmt.Sprintf("%s%s %s", prefix, titleStyle.Render(field.Label), m.fieldStatus(index)))
	}

	current := m.currentField()
	editingBlock := []string{
		paneTitleStyle.Render("Template"),
		mutedStyle.Render("Fill the current remote issue template on the left. The right pane updates live."),
		"",
		strings.Join(fieldLines, "\n"),
		"",
		paneTitleStyle.Render("Editing"),
		currentFieldStyle.Render(current.Label),
	}
	if current.Description != "" {
		editingBlock = append(editingBlock, mutedStyle.Render(current.Description))
	}
	editingBlock = append(editingBlock, "")

	if current.Type == issueform.FieldCheckboxes {
		value := "No"
		if m.entry.Checked(current.ID) {
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
		helpItem(actionKeyStyle, "ctrl+s", primaryDescription),
		helpDividerStyle.Render("  •  "),
		helpItem(previewKeyStyle, "pgup/pgdn", "scroll preview"),
		helpDividerStyle.Render("  •  "),
		helpItem(cancelKeyStyle, "esc", "cancel"),
	))
	if m.currentFieldIsText() {
		lines = append(lines, lipgloss.JoinHorizontal(
			lipgloss.Top,
			helpItem(secondaryKeyStyle, "ctrl+c", "copy field"),
			helpDividerStyle.Render("  •  "),
			helpItem(secondaryKeyStyle, "ctrl+x", "cut field"),
			helpDividerStyle.Render("  •  "),
			helpItem(secondaryKeyStyle, "ctrl+v", "paste"),
		))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func helpItem(keyStyle lipgloss.Style, key, description string) string {
	return keyStyle.Render(key) + helpTextStyle.Render(" "+description)
}

func (m model) actionTitle() string {
	if m.mode == modeCreate {
		return "Create Daily Update"
	}

	return "Edit Draft"
}

func (m *model) move(step int) {
	m.persistCurrentField()
	m.index = nextVisibleIndex(m.index, step, len(m.fields))
	m.syncEditor()
	m.refreshPreview()
}

func (m *model) syncEditor() {
	if !m.currentFieldIsText() {
		m.editor.Blur()
		return
	}

	m.editor.Focus()
	m.editor.Placeholder = m.currentField().Placeholder
	m.editor.SetValue(m.entry.Text(m.currentField().ID))
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

func (m *model) copyCurrentField(clear bool) {
	value := m.editor.Value()
	if strings.TrimSpace(value) == "" {
		m.setMessage("Current field is empty.", false)
		return
	}

	m.clipboard = value

	action := "Copied"
	if clear {
		action = "Cut"
		m.editor.SetValue("")
		m.persistCurrentField()
		m.refreshPreview()
	}

	m.messageIsError = false
	if err := writeClipboard(value); err != nil {
		m.clipboardSynced = false
		m.setClipboardActionMessage(fmt.Sprintf("%s current field to pad clipboard only.", action))
		return
	}

	m.clipboardSynced = true
	m.setClipboardActionMessage(fmt.Sprintf("%s current field to clipboard.", action))
}

func (m *model) pasteClipboard() {
	if m.clipboard == "" {
		m.setMessage("Pad clipboard is empty.", false)
		return
	}

	m.editor.InsertString(m.clipboard)
	m.persistCurrentField()
	m.refreshPreview()
	m.setMessage("Pasted clipboard into current field.", false)
}

func (m *model) setMessage(message string, isError bool) {
	m.message = message
	m.messageIsError = isError
	m.clipboardMessage = ""
	m.clipboardCount = 0
}

func (m *model) clearClipboardMessageOnKey(msg tea.KeyMsg) {
	if m.clipboardMessage == "" {
		return
	}
	if msg.String() == "ctrl+c" || msg.String() == "ctrl+x" {
		return
	}

	m.setMessage("", false)
}

func (m *model) setClipboardActionMessage(base string) {
	m.messageIsError = false
	if m.clipboardMessage == base {
		m.clipboardCount++
	} else {
		m.clipboardMessage = base
		m.clipboardCount = 1
	}

	m.message = base
	if m.clipboardCount > 1 {
		m.message = fmt.Sprintf("%s (%d)", base, m.clipboardCount)
	}
}

func previewContent(entry daily.Entry) string {
	title, err := entry.Title()
	if err != nil {
		title = entry.Date
	}

	return fmt.Sprintf("%s\n\n%s", title, entry.Body())
}

func (m *model) persistCurrentField() {
	if !m.currentFieldIsText() {
		return
	}

	m.entry.SetText(m.currentField().ID, m.editor.Value())
}

func (m model) finalEntry() daily.Entry {
	entry := m.entry
	if m.currentFieldIsText() {
		entry.SetText(m.currentField().ID, m.editor.Value())
	}
	return entry.Normalize()
}

func (m model) currentField() issueform.Field {
	return m.fields[m.index]
}

func (m model) currentFieldIsText() bool {
	switch m.currentField().Type {
	case issueform.FieldTextarea, issueform.FieldInput:
		return true
	default:
		return false
	}
}

func (m model) fieldStatus(index int) string {
	field := m.fields[index]
	if field.Type == issueform.FieldCheckboxes {
		if m.entry.Checked(field.ID) {
			return statusFilledStyle.Render("[yes]")
		}
		return statusEmptyStyle.Render("[no]")
	}

	if strings.TrimSpace(m.textValue(index)) == "" {
		return statusEmptyStyle.Render("[empty]")
	}

	return statusFilledStyle.Render("[filled]")
}

func (m model) textValue(index int) string {
	field := m.fields[index]
	if index == m.index && m.currentFieldIsText() {
		return m.editor.Value()
	}

	return m.entry.Text(field.ID)
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

func nextVisibleIndex(current, step, total int) int {
	if total <= 0 {
		return 0
	}

	candidate := current + step
	if candidate < 0 {
		return total - 1
	}
	if candidate >= total {
		return 0
	}
	return candidate
}
