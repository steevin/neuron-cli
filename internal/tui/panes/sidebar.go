// Package panes provides the individual UI pane components for NeuronCLI:
// the sidebar note list, the markdown editor/preview, and the search bar.
package panes

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/tui/styles"
)

// NoteItem wraps *notes.Note to satisfy the list.DefaultItem interface.
type NoteItem struct {
	Note *notes.Note
}

func (n NoteItem) FilterValue() string { return n.Note.Title }
func (n NoteItem) Title() string       { return n.Note.Title }

// Description returns a compact secondary line containing up to three tags and
// a human-readable relative timestamp, e.g. "#go #tui  •  3h ago".
func (n NoteItem) Description() string {
	tags := ""
	if len(n.Note.Tags) > 0 {
		tags = "#" + strings.Join(n.Note.Tags[:minInt(3, len(n.Note.Tags))], " #") + "  •  "
	}
	return tags + formatRelativeTime(n.Note.Updated)
}

// Sidebar is the left-hand note list pane.
type Sidebar struct {
	list    list.Model
	theme   *styles.Theme
	width   int
	height  int
	focused bool
}

// NewSidebar constructs a Sidebar. Call SetSize before the first render.
func NewSidebar(theme *styles.Theme) Sidebar {
	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.Background).
		Background(theme.Accent).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.Surface).
		Background(theme.Accent)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(theme.Text)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(theme.Muted)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(theme.Accent)
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)

	return Sidebar{
		list:  l,
		theme: theme,
	}
}

func (s Sidebar) Init() tea.Cmd { return nil }

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	if !s.focused {
		return s, nil
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s Sidebar) View() string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(s.theme.Border)

	if s.focused {
		borderStyle = borderStyle.BorderForeground(s.theme.Accent)
	}

	header := s.theme.TitleBar.
		Width(s.width - 2). // account for border
		Render(" 🧠 NOTES")

	body := s.list.View()

	count := len(s.list.Items())
	footerText := fmt.Sprintf(" %d notes", count)
	footer := s.theme.StatusBar.
		Width(s.width - 2).
		Render(footerText)

	inner := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return borderStyle.Render(inner)
}

func (s *Sidebar) SetNotes(noteSlice []*notes.Note) {
	items := make([]list.Item, len(noteSlice))
	for i, n := range noteSlice {
		items[i] = NoteItem{Note: n}
	}
	s.list.SetItems(items)
}

func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width-2, height-4)
}

func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

func (s Sidebar) SelectedNote() *notes.Note {
	item := s.list.SelectedItem()
	if item == nil {
		return nil
	}
	ni, ok := item.(NoteItem)
	if !ok {
		return nil
	}
	return ni.Note
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 02")
	}
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
