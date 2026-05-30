package panes

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/steevin/neuron-cli/internal/tui/styles"
)

// ── Messages ──────────────────────────────────────────────────────────────────

// SearchQueryMsg is emitted when the user presses Enter in the search bar.
// Listeners should filter the note list to entries matching Query.
type SearchQueryMsg struct{ Query string }

// SearchClearMsg is emitted when the user presses Escape in the search bar.
// Listeners should restore the full, unfiltered note list.
type SearchClearMsg struct{}

// ── SearchPane ────────────────────────────────────────────────────────────────

// SearchPane is the bottom-of-screen search input. When inactive it displays a
// "/ to search" hint; when active it shows a text input that emits
// SearchQueryMsg on Enter and SearchClearMsg on Escape.
type SearchPane struct {
	input  textinput.Model
	theme  *styles.Theme
	active bool
	width  int
}

// NewSearchPane creates a SearchPane with a styled textinput ready to use.
func NewSearchPane(theme *styles.Theme) SearchPane {
	ti := textinput.New()
	ti.Placeholder = "Search notes..."
	ti.CharLimit = 200
	ti.Width = 40

	ti.PromptStyle = lipgloss.NewStyle().Foreground(theme.Accent)
	ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Muted)

	return SearchPane{
		input: ti,
		theme: theme,
	}
}

// Init satisfies tea.Model. The search pane has no startup commands.
func (s SearchPane) Init() tea.Cmd { return nil }

// Update handles key events when the pane is active:
//   - Enter → emit SearchQueryMsg with the current query value
//   - Escape → clear input, deactivate, emit SearchClearMsg
//
// All other key events are forwarded to the underlying textinput.
func (s SearchPane) Update(msg tea.Msg) (SearchPane, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			query := s.input.Value()
			return s, func() tea.Msg {
				return SearchQueryMsg{Query: query}
			}

		case tea.KeyEsc:
			s.input.SetValue("")
			s.active = false
			s.input.Blur()
			return s, func() tea.Msg {
				return SearchClearMsg{}
			}
		}
	}

	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View renders either the active text input or the inactive "/ to search" hint,
// both styled consistently with the theme's status-bar appearance.
func (s SearchPane) View() string {
	containerStyle := s.theme.StatusBar.
		Width(s.width)

	if s.active {
		inputStyle := lipgloss.NewStyle().
			Foreground(s.theme.Text).
			Background(s.theme.Surface)
		return containerStyle.Render(inputStyle.Render(s.input.View()))
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(s.theme.Muted).
		Background(s.theme.Surface)
	return containerStyle.Render(hintStyle.Render("/ to search"))
}

// SetActive activates or deactivates the search input. Activating focuses the
// textinput so keystrokes are captured; deactivating blurs it.
func (s *SearchPane) SetActive(active bool) {
	s.active = active
	if active {
		s.input.Focus()
	} else {
		s.input.Blur()
	}
}

// SetWidth updates the pane's rendering width in terminal columns.
func (s *SearchPane) SetWidth(w int) {
	s.width = w
	// Leave a few columns of margin for the prompt glyph.
	if w > 6 {
		s.input.Width = w - 6
	}
}

// Query returns the current text in the search input.
func (s SearchPane) Query() string {
	return s.input.Value()
}

// SetQuery sets the current text in the search input and puts the cursor at the end.
func (s *SearchPane) SetQuery(q string) {
	s.input.SetValue(q)
	s.input.CursorEnd()
}
