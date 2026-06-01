package panes

import (
	"fmt"
	"os/user"
	"strings"
	"time"

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
		glamour.WithPreservedNewLines(),
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

// RefreshNote force-re-renders the current note content into the viewport.
// Call this after the underlying file has been modified externally (e.g. paste).
func (e *Editor) RefreshNote() {
	e.refreshContent()
}

func (e *Editor) SetSize(width, height int) {
	e.width = width
	e.height = height

	// Reserve 2 rows: 1 header border + 1 scroll hint.
	bodyHeight := height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	e.viewport.Width = width
	e.viewport.Height = bodyHeight

	// Word wrap at viewport width minus a small margin so text never overflows.
	wrapWidth := width - 6
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	if r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
		glamour.WithPreservedNewLines(),
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

	// Note icon based on type
	icon := "📝 "
	for _, t := range e.note.Tags {
		if t == "daily" {
			icon = "📅 "
			break
		}
	}

	title := titleStyle.Render(icon + e.note.Title)

	updated := "updated " + formatRelativeTime(e.note.Updated)
	tagParts := make([]string, 0, len(e.note.Tags))
	for _, t := range e.note.Tags {
		tagParts = append(tagParts, tagStyle.Render("#"+t))
	}
	tags := strings.Join(tagParts, " ")

	meta := metaStyle.Render(updated)
	if tags != "" {
		meta = metaStyle.Render(updated+"   ") + tags
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
	lines := []string{
		lipgloss.NewStyle().Foreground(e.theme.Muted).Render("No note selected"),
		"",
		lipgloss.NewStyle().Foreground(e.theme.Muted).Italic(true).Render("↑ ↓  navigate   Enter  select"),
	}
	msg := strings.Join(lines, "\n")

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

// SplashView returns a premium splash screen shown on first launch.
func (e *Editor) SplashView(noteCount, tagCount int) string {
	// ASCII art logo
	logoLines := []string{
		"  ███╗   ██╗███████╗██╗   ██╗██████╗  ██████╗ ███╗   ██╗",
		"  ████╗  ██║██╔════╝██║   ██║██╔══██╗██╔═══██╗████╗  ██║",
		"  ██╔██╗ ██║█████╗  ██║   ██║██████╔╝██║   ██║██╔██╗ ██║",
		"  ██║╚██╗██║██╔══╝  ██║   ██║██╔══██╗██║   ██║██║╚██╗██║",
		"  ██║ ╚████║███████╗╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║",
		"  ╚═╝  ╚═══╝╚══════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝",
	}

	logo := lipgloss.NewStyle().
		Foreground(e.theme.Accent).
		Bold(true).
		Render(strings.Join(logoLines, "\n"))

	tagline := lipgloss.NewStyle().
		Foreground(e.theme.Muted).
		Italic(true).
		Render("  your second brain, from the terminal")

	// User greeting
	userName := "there"
	if u, err := user.Current(); err == nil {
		if u.Name != "" {
			userName = u.Name
		} else {
			userName = u.Username
		}
	}

	welcome := lipgloss.NewStyle().
		Foreground(e.theme.TextBright).
		Bold(true).
		Render(fmt.Sprintf("Hello, %s 👋", userName))

	// Stats badges
	notesBadge := lipgloss.NewStyle().
		Foreground(e.theme.Background).
		Background(e.theme.Accent).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf(" %d notes", noteCount))

	tagsBadge := lipgloss.NewStyle().
		Foreground(e.theme.Background).
		Background(e.theme.AccentAlt).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf(" %d tags", tagCount))

	stats := lipgloss.JoinHorizontal(lipgloss.Center, notesBadge, "  ", tagsBadge)

	// Keybinding chips
	type chip struct{ key, desc string }
	chips := []chip{
		{"n", "new note"},
		{"ctrl+v", "paste clipboard"},
		{"/", "commands"},
		{"e", "edit"},
		{"s", "sync"},
		{"?", "help"},
	}

	var chipParts []string
	for _, c := range chips {
		k := lipgloss.NewStyle().
			Foreground(e.theme.Background).
			Background(e.theme.Muted).
			Bold(true).
			Padding(0, 1).
			Render(c.key)
		d := lipgloss.NewStyle().
			Foreground(e.theme.Muted).
			Render(" " + c.desc)
		chipParts = append(chipParts, k+d)
	}

	keyRow1 := lipgloss.JoinHorizontal(lipgloss.Center,
		chipParts[0], "   ", chipParts[1], "   ", chipParts[2])
	keyRow2 := lipgloss.JoinHorizontal(lipgloss.Center,
		chipParts[3], "   ", chipParts[4], "   ", chipParts[5])

	instruction := lipgloss.NewStyle().
		Foreground(e.theme.Info).
		Render("Press any key to enter vault →")

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo,
		"",
		tagline,
		"",
		"",
		welcome,
		"",
		stats,
		"",
		"",
		keyRow1,
		"  "+keyRow2,
		"",
		instruction,
	)

	return lipgloss.NewStyle().
		Width(e.width).
		Height(e.height).
		Align(lipgloss.Center, lipgloss.Center).
		Background(e.theme.Background).
		Render(content)
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
