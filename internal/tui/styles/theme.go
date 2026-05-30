// Package styles defines the visual theme for the NeuronCLI TUI, including
// color palettes and pre-built lipgloss styles for every UI component.
package styles

import "github.com/charmbracelet/lipgloss"

// Theme holds the complete set of colors and pre-built styles used throughout
// the NeuronCLI TUI. Construct one via DarkTheme() or LightTheme().
type Theme struct {
	// ── Base palette ────────────────────────────────────────────────────────
	Background lipgloss.Color // application background
	Surface    lipgloss.Color // panel / card surfaces
	Border     lipgloss.Color // border lines
	Muted      lipgloss.Color // de-emphasised / secondary text
	Text       lipgloss.Color // primary body text
	TextBright lipgloss.Color // headings and highlighted text

	// ── Accent palette ──────────────────────────────────────────────────────
	Accent    lipgloss.Color // primary accent (blue)
	AccentAlt lipgloss.Color // secondary accent (green)
	Warning   lipgloss.Color
	Error     lipgloss.Color

	// ── Pre-built component styles ──────────────────────────────────────────
	TitleBar        lipgloss.Style // full-width title bar
	SidebarItem     lipgloss.Style // unselected sidebar list row
	SidebarSelected lipgloss.Style // selected sidebar list row
	Preview         lipgloss.Style // right-hand preview pane container
	SearchBar       lipgloss.Style // search input bar
	StatusBar       lipgloss.Style // bottom status bar
	Tag             lipgloss.Style // inactive tag pill
	TagSelected     lipgloss.Style // active / highlighted tag pill
	Separator       lipgloss.Style // vertical / horizontal separator
	KeyHint         lipgloss.Style // keybinding hint text, e.g. [q] quit
	AppName         lipgloss.Style // application name in title bar
}

// buildStyles populates all computed lipgloss.Style fields from a Theme whose
// colour fields have already been set. It returns the fully-initialised Theme.
func buildStyles(t Theme) *Theme {
	t.TitleBar = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.TextBright).
		Background(t.Surface).
		Padding(0, 1)

	t.SidebarItem = lipgloss.NewStyle().
		Foreground(t.Text).
		Padding(0, 1)

	t.SidebarSelected = lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.Accent).
		Bold(true).
		Padding(0, 1)

	t.Preview = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Background).
		Padding(0, 1)

	t.SearchBar = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Surface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Padding(0, 1)

	t.StatusBar = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(t.Surface).
		Padding(0, 1)

	t.Tag = lipgloss.NewStyle().
		Foreground(t.AccentAlt).
		Background(t.Surface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.AccentAlt).
		Padding(0, 1)

	t.TagSelected = lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.AccentAlt).
		Bold(true).
		Padding(0, 1)

	t.Separator = lipgloss.NewStyle().
		Foreground(t.Border)

	t.KeyHint = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(t.Surface)

	t.AppName = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Accent).
		Background(t.Surface)

	return &t
}

// DarkTheme returns a Theme based on GitHub's dark palette.
func DarkTheme() *Theme {
	t := Theme{
		Background: lipgloss.Color("#0d1117"),
		Surface:    lipgloss.Color("#161b22"),
		Border:     lipgloss.Color("#30363d"),
		Muted:      lipgloss.Color("#8b949e"),
		Text:       lipgloss.Color("#c9d1d9"),
		TextBright: lipgloss.Color("#f0f6fc"),
		Accent:     lipgloss.Color("#58a6ff"),
		AccentAlt:  lipgloss.Color("#3fb950"),
		Warning:    lipgloss.Color("#d29922"),
		Error:      lipgloss.Color("#f85149"),
	}
	return buildStyles(t)
}

// LightTheme returns a Theme based on GitHub's light palette.
func LightTheme() *Theme {
	t := Theme{
		Background: lipgloss.Color("#ffffff"),
		Surface:    lipgloss.Color("#f6f8fa"),
		Border:     lipgloss.Color("#d0d7de"),
		Muted:      lipgloss.Color("#656d76"),
		Text:       lipgloss.Color("#24292f"),
		TextBright: lipgloss.Color("#1f2328"),
		Accent:     lipgloss.Color("#0969da"),
		AccentAlt:  lipgloss.Color("#1a7f37"),
		Warning:    lipgloss.Color("#9a6700"),
		Error:      lipgloss.Color("#cf222e"),
	}
	return buildStyles(t)
}

// GetTheme returns a *Theme by name. "dark" returns DarkTheme; anything else
// (including "light") returns LightTheme.
func GetTheme(name string) *Theme {
	if name == "dark" {
		return DarkTheme()
	}
	return LightTheme()
}
