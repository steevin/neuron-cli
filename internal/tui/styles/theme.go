// Package styles defines the visual theme for the NeuronCLI TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Theme holds colors and pre-built lipgloss styles for every UI component.
type Theme struct {
	Background lipgloss.Color
	Surface    lipgloss.Color
	Border     lipgloss.Color
	Muted      lipgloss.Color
	Text       lipgloss.Color
	TextBright lipgloss.Color

	Accent    lipgloss.Color // primary accent (blue)
	AccentAlt lipgloss.Color // secondary accent (green)
	Warning   lipgloss.Color
	Error     lipgloss.Color

	TitleBar        lipgloss.Style
	SidebarItem     lipgloss.Style
	SidebarSelected lipgloss.Style
	Preview         lipgloss.Style
	SearchBar       lipgloss.Style
	StatusBar       lipgloss.Style
	Tag             lipgloss.Style
	TagSelected     lipgloss.Style
	Separator       lipgloss.Style
	KeyHint         lipgloss.Style
	AppName         lipgloss.Style
}

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

// DarkTheme returns a dark theme based on GitHub's dark palette.
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

// LightTheme returns a light theme based on GitHub's light palette.
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

// GetTheme returns a *Theme by name; anything other than "dark" gets the light theme.
func GetTheme(name string) *Theme {
	if name == "dark" {
		return DarkTheme()
	}
	return LightTheme()
}
