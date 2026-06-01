// Package styles defines the visual theme for the NeuronCLI TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Theme holds colors and pre-built lipgloss styles for every UI component.
type Theme struct {
	Background lipgloss.Color
	Surface    lipgloss.Color
	Surface2   lipgloss.Color // slightly elevated surface for overlays
	Border     lipgloss.Color
	Muted      lipgloss.Color
	Text       lipgloss.Color
	TextBright lipgloss.Color

	Accent    lipgloss.Color // primary accent (blue)
	AccentAlt lipgloss.Color // secondary accent (green)
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Success   lipgloss.Color
	Info      lipgloss.Color

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
	CommandPalette  lipgloss.Style
	NewNoteInput    lipgloss.Style
	Badge           lipgloss.Style
	ModeIndicator   lipgloss.Style
	SuccessMsg      lipgloss.Style
	ErrorMsg        lipgloss.Style
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

	t.CommandPalette = lipgloss.NewStyle().
		Foreground(t.Info).
		Background(t.Surface2).
		Padding(0, 1)

	t.NewNoteInput = lipgloss.NewStyle().
		Foreground(t.TextBright).
		Background(t.Surface).
		Bold(true)

	t.Badge = lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.AccentAlt).
		Bold(true).
		Padding(0, 1)

	t.ModeIndicator = lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.Accent).
		Bold(true).
		Padding(0, 1)

	t.SuccessMsg = lipgloss.NewStyle().
		Foreground(t.Success).
		Background(t.Surface).
		Bold(true).
		Padding(0, 1)

	t.ErrorMsg = lipgloss.NewStyle().
		Foreground(t.Error).
		Background(t.Surface).
		Bold(true).
		Padding(0, 1)

	return &t
}

// DarkTheme returns a premium dark theme based on Tokyo Night.
func DarkTheme() *Theme {
	t := Theme{
		Background: lipgloss.Color("#1a1b26"),
		Surface:    lipgloss.Color("#24283b"),
		Surface2:   lipgloss.Color("#2a2f45"),
		Border:     lipgloss.Color("#414868"),
		Muted:      lipgloss.Color("#565f89"),
		Text:       lipgloss.Color("#a9b1d6"),
		TextBright: lipgloss.Color("#c0caf5"),
		Accent:     lipgloss.Color("#7aa2f7"),
		AccentAlt:  lipgloss.Color("#9ece6a"),
		Warning:    lipgloss.Color("#e0af68"),
		Error:      lipgloss.Color("#f7768e"),
		Success:    lipgloss.Color("#9ece6a"),
		Info:       lipgloss.Color("#73daca"),
	}
	return buildStyles(t)
}

// LightTheme returns a light theme based on GitHub's light palette.
func LightTheme() *Theme {
	t := Theme{
		Background: lipgloss.Color("#ffffff"),
		Surface:    lipgloss.Color("#f6f8fa"),
		Surface2:   lipgloss.Color("#eaeef2"),
		Border:     lipgloss.Color("#d0d7de"),
		Muted:      lipgloss.Color("#656d76"),
		Text:       lipgloss.Color("#24292f"),
		TextBright: lipgloss.Color("#1f2328"),
		Accent:     lipgloss.Color("#0969da"),
		AccentAlt:  lipgloss.Color("#1a7f37"),
		Warning:    lipgloss.Color("#9a6700"),
		Error:      lipgloss.Color("#cf222e"),
		Success:    lipgloss.Color("#1a7f37"),
		Info:       lipgloss.Color("#0550ae"),
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
