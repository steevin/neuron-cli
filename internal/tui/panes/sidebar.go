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

	"github.com/danielsteevin/neuron-cli/internal/notes"
	"github.com/danielsteevin/neuron-cli/internal/tui/styles"
)

// ── List item adapter ─────────────────────────────────────────────────────────

// NoteItem wraps a *notes.Note so it satisfies the list.DefaultItem interface
// expected by the bubbles/list component.
type NoteItem struct {
	Note *notes.Note
}

// FilterValue returns the string used for fuzzy-filtering in the list bubble.
func (n NoteItem) FilterValue() string { return n.Note.Title }

// Title returns the note's title, displayed as the primary line in the list.
func (n NoteItem) Title() string { return n.Note.Title }

// Description returns a compact secondary line containing up to three tags and
// a human-readable relative timestamp, e.g. "#go #tui  •  3h ago".
func (n NoteItem) Description() string {
	tags := ""
	if len(n.Note.Tags) > 0 {
		tags = "#" + strings.Join(n.Note.Tags[:minInt(3, len(n.Note.Tags))], " #") + "  •  "
	}
	return tags + formatRelativeTime(n.Note.Updated)
}

// ── Sidebar ───────────────────────────────────────────────────────────────────

// Sidebar is the left-hand note list pane. It renders a scrollable, filterable
// list of NoteItems and exposes the currently selected *notes.Note.
type Sidebar struct {
	list    list.Model
	theme   *styles.Theme
	width   int
	height  int
	focused bool
}

// NewSidebar constructs a Sidebar with sensible defaults. Call SetSize before
// the first render to give it accurate dimensions.
func NewSidebar(theme *styles.Theme) Sidebar {
	delegate := list.NewDefaultDelegate()

	// Style the delegate using the theme colours.
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
	l.SetShowTitle(false)        // we render our own header
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	// Style the list's built-in filter input.
	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(theme.Accent)
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)

	return Sidebar{
		list:  l,
		theme: theme,
	}
}

// Init satisfies tea.Model. The sidebar has no startup commands.
func (s Sidebar) Init() tea.Cmd { return nil }

// Update processes incoming messages and delegates most key events to the
// embedded list bubble.
func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	if !s.focused {
		return s, nil
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

// View renders the sidebar: a branded header, the scrollable list body, and a
// small footer showing the total note count.
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

// SetNotes replaces the sidebar's note list with the provided slice. Existing
// filter state is cleared.
func (s *Sidebar) SetNotes(noteSlice []*notes.Note) {
	items := make([]list.Item, len(noteSlice))
	for i, n := range noteSlice {
		items[i] = NoteItem{Note: n}
	}
	s.list.SetItems(items)
}

// SetSize informs the sidebar of the available terminal area in columns and
// rows so it can lay out its contents correctly.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
	// Reserve 4 rows: 1 header + 1 footer + 2 borders.
	s.list.SetSize(width-2, height-4)
}

// SetFocused controls whether the sidebar receives key events and whether its
// border is highlighted to indicate focus.
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// SelectedNote returns the *notes.Note for the currently highlighted list row,
// or nil if the list is empty.
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

// ── Helpers ───────────────────────────────────────────────────────────────────

// formatRelativeTime converts a time.Time into a concise human-readable string
// relative to now, e.g. "just now", "5m ago", "2h ago", "3d ago", "Jan 02".
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
