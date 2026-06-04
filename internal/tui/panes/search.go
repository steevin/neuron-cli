// Copyright (C) 2025 Daniel Steevin
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package panes

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/steevin/neuron-cli/internal/tui/styles"
)

// SearchQueryMsg is emitted when the user presses Enter in the search bar.
type SearchQueryMsg struct{ Query string }

// SearchClearMsg is emitted when the user presses Escape; restores the full list.
type SearchClearMsg struct{}

// SearchPane is the bottom search bar. Inactive it shows "/ to search";
// active it captures input and emits SearchQueryMsg or SearchClearMsg.
type SearchPane struct {
	input  textinput.Model
	theme  *styles.Theme
	active bool
	width  int
}

// NewSearchPane creates a SearchPane with a styled textinput.
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

func (s SearchPane) Init() tea.Cmd { return nil }

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

func (s *SearchPane) SetActive(active bool) {
	s.active = active
	if active {
		s.input.Focus()
	} else {
		s.input.Blur()
	}
}

func (s *SearchPane) SetWidth(w int) {
	s.width = w
	if w > 6 {
		s.input.Width = w - 6
	}
}

func (s SearchPane) Query() string {
	return s.input.Value()
}

func (s *SearchPane) SetQuery(q string) {
	s.input.SetValue(q)
	s.input.CursorEnd()
}
