package ui

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
	err      error
}

func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(string(Primary)))
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case errMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit
	case doneMsg:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m SpinnerModel) View() tea.View {
	if m.quitting {
		if m.err != nil {
			return tea.NewView(ErrorStyle.Render("✗ " + m.message + " failed: " + m.err.Error() + "\n"))
		}
		return tea.NewView(SuccessStyle.Render("✓ " + m.message + "\n"))
	}
	return tea.NewView(m.spinner.View() + " " + m.message + "\n")
}

type errMsg struct{ err error }
type doneMsg struct{}

func RunWithSpinner(message string, fn func() error) error {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		fmt.Printf("⏳ %s...\n", message)
		start := time.Now()
		err := fn()
		elapsed := time.Since(start)
		if err != nil {
			fmt.Printf("✗ %s failed (%s): %v\n", message, elapsed.Round(time.Millisecond), err)
		} else {
			fmt.Printf("✓ %s (%s)\n", message, elapsed.Round(time.Millisecond))
		}
		return err
	}

	m := NewSpinner(message)
	p := tea.NewProgram(m)

	errChan := make(chan error, 1)
	go func() {
		err := fn()
		errChan <- err
		if err != nil {
			p.Send(errMsg{err})
		} else {
			p.Send(doneMsg{})
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("spinner error: %w", err)
	}

	return <-errChan
}
