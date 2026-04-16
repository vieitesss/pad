package tui

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RunWithSpinner runs a function with a loading spinner and returns the result.
// It shows the spinner in the terminal while the operation is in progress.
// If the terminal doesn't support interactive output, it runs without the spinner.
func RunWithSpinner[T any](ctx context.Context, message string, fn func(context.Context) (T, error)) (T, error) {
	// Check if we're in a terminal
	if !isTerminal() {
		// Non-interactive mode: just run the function
		return fn(ctx)
	}

	m := newSpinnerModel(message, fn)
	p := tea.NewProgram(m, tea.WithContext(ctx))

	result, err := p.Run()
	if err != nil {
		// If we can't run the program (e.g., no TTY), just run the function directly
		if isNoTTYError(err) {
			return fn(ctx)
		}
		var zero T
		return zero, err
	}

	finalModel := result.(spinnerModel[T])
	if finalModel.err != nil {
		var zero T
		return zero, finalModel.err
	}

	return finalModel.result, nil
}

// RunWithSpinnerNoResult runs a function with a loading spinner for operations without a return value.
func RunWithSpinnerNoResult(ctx context.Context, message string, fn func(context.Context) error) error {
	wrapper := func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	}

	_, err := RunWithSpinner(ctx, message, wrapper)
	return err
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeCharDevice != 0
}

// isNoTTYError checks if an error is related to missing TTY
func isNoTTYError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "tty") || contains(errStr, "terminal") || contains(errStr, "device")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// PrintSimpleProgress prints a simple progress message for non-interactive use
func PrintSimpleProgress(w io.Writer, message string) {
	fmt.Fprintf(w, "%s...\n", message)
}

// spinnerModel is a tea.Model that runs a spinner while executing work
type spinnerModel[T any] struct {
	spinner spinner.Model
	message string
	done    bool
	result  T
	err     error
	workFn  func(context.Context) (T, error)
}

type workDoneMsg[T any] struct {
	result T
	err    error
}

func newSpinnerModel[T any](message string, workFn func(context.Context) (T, error)) spinnerModel[T] {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	return spinnerModel[T]{
		spinner: s,
		message: message,
		workFn:  workFn,
	}
}

func (m spinnerModel[T]) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			result, err := m.workFn(ctx)
			return workDoneMsg[T]{result: result, err: err}
		},
	)
}

func (m spinnerModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case workDoneMsg[T]:
		m.done = true
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m spinnerModel[T]) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(m.message))
}
