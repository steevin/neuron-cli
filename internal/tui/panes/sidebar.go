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

// Package panes provides the individual UI pane components for NeuronCLI:
// the sidebar note list, the markdown editor/preview, and the search bar.
package panes

import (
	"fmt"
	"strings"

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
func (n NoteItem) Title() string {
	// Prefix note title with a type icon
	for _, t := range n.Note.Tags {
		if t == "daily" {
			return "📅 " + n.Note.Title
		}
	}
	return "📝 " + n.Note.Title
}

// Description returns a compact secondary line containing up to three tags and
// a human-readable relative timestamp, e.g. "#go #tui  •  3h ago".
func (n NoteItem) Description() string {
	max := 3
	if len(n.Note.Tags) < max {
		max = len(n.Note.Tags)
	}
	tags := ""
	if len(n.Note.Tags) > 0 {
		tags = "#" + strings.Join(n.Note.Tags[:max], " #") + "  •  "
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
	version string
}

// NewSidebar constructs a Sidebar. Call SetSize before the first render.
func NewSidebar(theme *styles.Theme, version string) Sidebar {
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
	l.DisableQuitKeybindings()

	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(theme.Accent)
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)

	return Sidebar{
		list:    l,
		theme:   theme,
		version: version,
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

	// Logo block
	logoWidth := s.width - 4
	if logoWidth < 10 {
		logoWidth = 10
	}

	cCyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#00d2ff")).Bold(true).Render
	cVersion := lipgloss.NewStyle().Foreground(s.theme.Muted).Render

	logoContent := cCyan("NEURON") + "\n" + cVersion("v"+s.version)

	logoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.theme.Border).
		Padding(0, 1).
		MarginBottom(1).
		Width(logoWidth).
		Align(lipgloss.Center).
		Render(logoContent)

	// Header: show focused indicator
	headerTitle := " 📝 NOTES"
	header := s.theme.TitleBar.
		Width(s.width - 2). // account for border
		Render(headerTitle)

	body := s.list.View()

	count := len(s.list.Items())
	footerText := fmt.Sprintf(" %d notes", count)
	footer := lipgloss.NewStyle().
		Foreground(s.theme.Muted).
		Background(s.theme.Surface).
		Width(s.width - 2).
		Padding(0, 1).
		Render(footerText)

	inner := lipgloss.JoinVertical(lipgloss.Left, logoBox, header, body, footer)
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
	listHeight := height - 4 - 10
	if listHeight < 5 {
		listHeight = 5
	}
	s.list.SetSize(width-2, listHeight)
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

func (s *Sidebar) SelectNoteByID(id string) {
	for i, item := range s.list.Items() {
		if ni, ok := item.(NoteItem); ok && ni.Note.ID == id {
			s.list.Select(i)
			break
		}
	}
}



