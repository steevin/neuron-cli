package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/tui/styles"
)

// Editor is the right-hand preview pane. It renders the selected note's
// content as formatted markdown inside a scrollable viewport.
type Editor struct {
	viewport viewport.Model
	note     *notes.Note
	theme    *styles.Theme
	width    int
	height   int
	focused  bool
	renderer *glamour.TermRenderer
}

// NewEditor constructs an Editor with an initial glamour renderer. Call
// SetSize before the first render to give it accurate dimensions.
func NewEditor(theme *styles.Theme) Editor {
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Foreground(theme.Text).
		Background(theme.Background)

	return Editor{
		viewport: vp,
		theme:    theme,
		renderer: renderer,
	}
}

func (e Editor) Init() tea.Cmd { return nil }

func (e Editor) Update(msg tea.Msg) (Editor, tea.Cmd) {
	if !e.focused {
		return e, nil
	}
	var cmd tea.Cmd
	e.viewport, cmd = e.viewport.Update(msg)
	return e, cmd
}

func (e Editor) View() string {
	if e.note == nil {
		return e.emptyState()
	}

	header := e.renderHeader()
	content := e.viewport.View()

	pct := fmt.Sprintf("  %d%%", int(e.viewport.ScrollPercent()*100))
	scrollHint := lipgloss.NewStyle().
		Foreground(e.theme.Muted).
		Background(e.theme.Surface).
		Width(e.width).
		Align(lipgloss.Right).
		Render(pct)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, scrollHint)
}

func (e *Editor) SetNote(note *notes.Note) {
	e.note = note
	if note == nil {
		e.viewport.SetContent("")
		return
	}
	e.refreshContent()
}

func (e *Editor) SetSize(width, height int) {
	e.width = width
	e.height = height

	bodyHeight := height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	e.viewport.Width = width
	e.viewport.Height = bodyHeight

	wrapWidth := width - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	if r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
	); err == nil {
		e.renderer = r
	}

	if e.note != nil {
		e.refreshContent()
	}
}

func (e *Editor) SetFocused(focused bool) {
	e.focused = focused
}

func (e *Editor) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(e.theme.TextBright).
		Background(e.theme.Surface).
		Padding(0, 1)

	metaStyle := lipgloss.NewStyle().
		Foreground(e.theme.Muted).
		Background(e.theme.Surface).
		Padding(0, 1)

	tagStyle := lipgloss.NewStyle().
		Foreground(e.theme.AccentAlt).
		Background(e.theme.Surface).
		Padding(0, 0)

	title := titleStyle.Render(e.note.Title)

	updated := "updated " + formatRelativeTime(e.note.Updated)
	tagParts := make([]string, 0, len(e.note.Tags))
	for _, t := range e.note.Tags {
		tagParts = append(tagParts, tagStyle.Render("#"+t))
	}
	tags := strings.Join(tagParts, " ")

	meta := metaStyle.Render(updated)
	if tags != "" {
		meta = metaStyle.Render(updated + "   " + tags)
	}

	bar := lipgloss.NewStyle().
		Background(e.theme.Surface).
		Width(e.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(e.theme.Border)

	return bar.Render(lipgloss.JoinHorizontal(lipgloss.Center, title, meta))
}

// emptyState returns a full-pane message when no note is selected.
func (e *Editor) emptyState() string {
	msg := lipgloss.NewStyle().
		Foreground(e.theme.Muted).
		Bold(false).
		Render("Select a note to preview  ↑/↓ to navigate")

	return lipgloss.NewStyle().
		Width(e.width).
		Height(e.height).
		Align(lipgloss.Center, lipgloss.Center).
		Background(e.theme.Background).
		Render(msg)
}

// refreshContent (re-)renders the note markdown through glamour and loads the
// result into the viewport.
func (e *Editor) refreshContent() {
	if e.renderer == nil || e.note == nil {
		return
	}
	src := e.note.Content
	if src == "" {
		src = e.note.RawContent
	}
	rendered, err := e.renderer.Render(src)
	if err != nil {
		rendered = src // fall back to plain text on render error
	}
	e.viewport.SetContent(rendered)
	e.viewport.GotoTop()
}
