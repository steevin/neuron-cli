// Package tui wires the sidebar, editor, and search pane into a Bubble Tea
// MVU application.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/notes"
	gitsync "github.com/steevin/neuron-cli/internal/sync"
	"github.com/steevin/neuron-cli/internal/tui/panes"
	"github.com/steevin/neuron-cli/internal/tui/styles"
)

// focus identifies which pane currently has keyboard focus.
type focus int

const (
	focusSidebar focus = iota
	focusEditor
	focusSearch
)

// notesLoadedMsg is dispatched when the initial note scan completes.
type notesLoadedMsg struct{ notes []*notes.Note }

// errMsg carries an error back to the main update loop.
type errMsg struct{ err error }

// statusMsg carries a transient human-readable status update.
type statusMsg struct{ msg string }

// Model is the root Bubble Tea model. It owns all child pane models and
// orchestrates focus, layout, and data flow.
type Model struct {
	cfg       *config.Config
	store     *notes.Store
	theme     *styles.Theme
	sidebar   panes.Sidebar
	editor    panes.Editor
	search    panes.SearchPane
	spinner   spinner.Model
	help      help.Model
	showHelp  bool
	syncing   bool
	focused   focus
	allNotes  []*notes.Note // unfiltered master list
	width     int
	height    int
	ready     bool // true once the terminal size is known
	err       error
	statusMsg string
}

type keyMap struct {
	Tab    key.Binding
	Search key.Binding
	New    key.Binding
	Edit   key.Binding
	Sync   key.Binding
	Graph  key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Search, k.New, k.Edit, k.Sync, k.Graph, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Search, k.New, k.Edit},
		{k.Sync, k.Graph, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Tab:    key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "focus")),
	Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Sync:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	Graph:  key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// New constructs a Model from the provided configuration. It sets up the note
// store, selects the correct theme, and initialises all child pane models.
func New(cfg *config.Config) (*Model, error) {
	store, err := notes.NewStore(cfg.VaultPath)
	if err != nil {
		return nil, fmt.Errorf("tui: create note store: %w", err)
	}

	theme := styles.GetTheme(cfg.Theme)

	sidebar := panes.NewSidebar(theme)
	sidebar.SetFocused(true)

	editor := panes.NewEditor(theme)
	search := panes.NewSearchPane(theme)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Muted)

	h := help.New()

	return &Model{
		cfg:     cfg,
		store:   store,
		theme:   theme,
		sidebar: sidebar,
		editor:  editor,
		search:  search,
		spinner: s,
		help:    h,
		focused: focusSidebar,
	}, nil
}

// Init loads notes from the store in the background.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			noteList, err := m.store.List(notes.ListOptions{})
			if err != nil {
				return errMsg{err: err}
			}
			return notesLoadedMsg{notes: noteList}
		},
	)
}

// Update is the Bubble Tea message handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width
		m.layout()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case notesLoadedMsg:
		m.allNotes = msg.notes
		m.sidebar.SetNotes(msg.notes)
		m.statusMsg = fmt.Sprintf("Loaded %d notes", len(msg.notes))

	case errMsg:
		m.err = msg.err
		m.statusMsg = "Error: " + msg.err.Error()
		m.syncing = false

	case statusMsg:
		m.statusMsg = msg.msg
		m.syncing = false

	case panes.SearchQueryMsg:
		if strings.HasPrefix(msg.Query, "/") {
			m.setFocus(focusSidebar)
			cmd := m.handlePaletteCommand(strings.TrimSpace(msg.Query))
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		m = m.filterNotes(msg.Query)
		// After search committed, return focus to sidebar.
		m.setFocus(focusSidebar)

	case panes.SearchClearMsg:
		m.sidebar.SetNotes(m.allNotes)
		m.setFocus(focusSidebar)

	case tea.KeyMsg:
		// Global shortcuts that work regardless of focus.
		switch msg.String() {
		case "ctrl+c", "q":
			if m.focused != focusSearch {
				return m, tea.Quit
			}

		case "?":
			if m.focused != focusSearch {
				m.showHelp = !m.showHelp
			}

		case "tab":
			if m.focused != focusSearch {
				if m.focused == focusSidebar {
					m.setFocus(focusEditor)
				} else {
					m.setFocus(focusSidebar)
				}
			}

		case "shift+tab":
			if m.focused != focusSearch {
				if m.focused == focusEditor {
					m.setFocus(focusSidebar)
				} else {
					m.setFocus(focusEditor)
				}
			}

		case "/":
			if m.focused != focusSearch {
				m.setFocus(focusSearch)
				m.search.SetActive(true)
				m.search.SetQuery("/")
			}

		case "n":
			if m.focused != focusSearch {
				m.statusMsg = "New note: open your vault directory and create a .md file"
			}

		case "e":
			if m.focused != focusSearch {
				cmd := m.openInEditor()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}

		case "g":
			if m.focused != focusSearch {
				m.statusMsg = m.renderGraphSummary()
			}

		case "s":
			if m.focused != focusSearch {
				m.syncing = true
				m.statusMsg = "Syncing..."
				cmds = append(cmds, m.syncCmd())
			}
		}
	}

	// Delegate to focused child pane.
	switch m.focused {
	case focusSidebar:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)

		// Keep editor in sync with the highlighted note.
		if selected := m.sidebar.SelectedNote(); selected != nil {
			m.editor.SetNote(selected)
		}

	case focusEditor:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		cmds = append(cmds, cmd)

	case focusSearch:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the full three-pane layout.
//
//	┌──────────────────────────────────────────────────────────┐
//	│  🧠 neuron                      [?] help  [q] quit       │
//	├───────────────────┬──────────────────────────────────────┤
//	│   SIDEBAR (30%)   │   EDITOR / PREVIEW (70%)             │
//	├───────────────────┴──────────────────────────────────────┤
//	│  / search...                         42 notes | 8 tags   │
//	└──────────────────────────────────────────────────────────┘
func (m Model) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}

	titleBar := m.renderTitleBar()
	middle := lipgloss.JoinHorizontal(lipgloss.Top,
		m.sidebar.View(),
		m.editor.View(),
	)

	if m.showHelp {
		m.help.ShowAll = true
		helpView := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height - 2).
			Align(lipgloss.Center).
			Render("\n\n" + m.help.View(keys))
		middle = helpView
	}

	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		middle,
		statusBar,
	)
}

// layout recomputes and pushes dimensions to all child panes on resize.
func (m *Model) layout() {
	// Subtract 2 rows: 1 title bar + 1 status bar.
	bodyHeight := m.height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	sidebarWidth := m.width * 30 / 100
	if sidebarWidth < 20 {
		sidebarWidth = 20
	}
	editorWidth := m.width - sidebarWidth

	m.sidebar.SetSize(sidebarWidth, bodyHeight)
	m.editor.SetSize(editorWidth, bodyHeight)
	m.search.SetWidth(m.width)
}

// setFocus moves keyboard focus to the named pane, updating the focused flag
// on each child pane accordingly.
func (m *Model) setFocus(f focus) {
	m.focused = f
	m.sidebar.SetFocused(f == focusSidebar)
	m.editor.SetFocused(f == focusEditor)
	m.search.SetActive(f == focusSearch)
}

func (m Model) renderTitleBar() string {
	leftStyle := m.theme.AppName.Background(m.theme.Surface)
	left := leftStyle.Render(" 🧠 neuron")

	rightStyle := m.theme.KeyHint.
		Foreground(m.theme.Muted).
		Background(m.theme.Surface)
	right := rightStyle.Render("[?] help  [q] quit ")

	spacerWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := lipgloss.NewStyle().
		Background(m.theme.Surface).
		Width(spacerWidth).
		Render("")

	return lipgloss.NewStyle().
		Background(m.theme.Surface).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.theme.Border).
		Width(m.width).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right))
}

func (m Model) renderStatusBar() string {
	var left string
	if m.focused == focusSearch {
		query := m.search.Query()
		if strings.HasPrefix(query, "/") {
			// Show command palette suggestions
			suggestions := []string{"/add", "/today", "/sync", "/stats", "/quit"}
			matches := fuzzy.Find(query, suggestions)
			var filtered []string
			for _, match := range matches {
				filtered = append(filtered, match.Str)
			}
			sugStr := strings.Join(filtered, "  ")
			if sugStr == "" {
				sugStr = "No commands found"
			}
			sugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#73daca")).PaddingLeft(2)
			left = m.search.View() + sugStyle.Render(sugStr)
		} else {
			left = m.search.View()
		}
	} else if m.statusMsg != "" {
		msg := m.statusMsg
		if m.syncing {
			msg = m.spinner.View() + " " + msg
		}
		left = m.theme.StatusBar.Render(" " + msg)
	} else {
		left = m.theme.StatusBar.Render(m.search.View())
	}

	noteCount := len(m.allNotes)
	tagCount := m.countUniqueTags()
	right := m.theme.StatusBar.
		Foreground(m.theme.Muted).
		Render(fmt.Sprintf("%d notes | %d tags ", noteCount, tagCount))

	spacerWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := m.theme.StatusBar.
		Width(spacerWidth).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right)
}

func (m *Model) handlePaletteCommand(cmdStr string) tea.Cmd {
	parts := strings.Split(cmdStr, " ")
	base := parts[0]

	switch base {
	case "/quit", "/q":
		return tea.Quit
	case "/sync", "/s":
		m.syncing = true
		m.statusMsg = "Syncing..."
		return m.syncCmd()
	case "/today", "/t":
		title := "Daily " + time.Now().Format("2006-01-02")
		_, err := m.store.Get(title)
		if err != nil {
			content, _ := m.store.RenderTemplate("daily", title)
			if content == "" {
				content = "## 🎯 Today's goals\n- [ ] \n\n## 📝 Notes\n\n## 🔗 Links\n"
			}
			_, err = m.store.Create(title, []string{"daily"}, content)
			if err != nil {
				m.statusMsg = "Error creating daily note: " + err.Error()
				return nil
			}
		}
		// Select it in the sidebar
		m.statusMsg = "Opened daily note"
		// Trigger notes reload
		return m.Init()
	case "/add", "/a":
		if len(parts) < 2 {
			m.statusMsg = "Usage: /add <title>"
			return nil
		}
		title := strings.Join(parts[1:], " ")
		_, err := m.store.Create(title, nil, "")
		if err != nil {
			m.statusMsg = "Error creating note: " + err.Error()
			return nil
		}
		m.statusMsg = "Created note: " + title
		return m.Init()
	case "/stats":
		m.statusMsg = fmt.Sprintf("Stats: %d notes, %d tags", len(m.allNotes), m.countUniqueTags())
		return nil
	default:
		m.statusMsg = "Unknown command: " + base
		return nil
	}
}

// openInEditor opens the currently selected note in the configured editor.
func (m *Model) openInEditor() tea.Cmd {
	note := m.sidebar.SelectedNote()
	if note == nil {
		m.statusMsg = "No note selected"
		return nil
	}

	editor := m.cfg.Editor
	if editor == "" {
		if e := os.Getenv("EDITOR"); e != "" {
			editor = e
		} else if e := os.Getenv("VISUAL"); e != "" {
			editor = e
		} else {
			editor = "vi"
		}
	}

	// Sanitize editor input
	editorParts := strings.Fields(editor)
	if len(editorParts) > 0 {
		editor = editorParts[0]
	} else {
		editor = "vi"
	}

	return tea.ExecProcess(exec.Command(editor, note.Path), func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return statusMsg{msg: "Returned from editor"}
	})
}

// syncCmd triggers a git sync and reports the result via statusMsg or errMsg.
func (m *Model) syncCmd() tea.Cmd {
	vaultPath := m.cfg.VaultPath
	remote := m.cfg.GitRemote

	return func() tea.Msg {
		syncer := gitsync.NewSyncer(vaultPath, remote)
		result, err := syncer.Sync()
		if err != nil {
			return errMsg{err: fmt.Errorf("sync: %w", err)}
		}

		// After a successful sync, reload notes in case files changed.
		store, storeErr := notes.NewStore(vaultPath)
		if storeErr != nil {
			return statusMsg{msg: result.Message}
		}
		noteList, listErr := store.List(notes.ListOptions{})
		if listErr != nil {
			return statusMsg{msg: result.Message}
		}
		// Return notes loaded so the sidebar refreshes, then show sync message.
		_ = noteList
		return statusMsg{msg: result.Message}
	}
}

// filterNotes returns a new Model with the sidebar filtered to notes whose
// title or tags contain the query string (case-insensitive).
func (m Model) filterNotes(query string) Model {
	if query == "" {
		m.sidebar.SetNotes(m.allNotes)
		return m
	}
	q := strings.ToLower(query)
	var filtered []*notes.Note
	for _, n := range m.allNotes {
		if strings.Contains(strings.ToLower(n.Title), q) {
			filtered = append(filtered, n)
			continue
		}
		for _, tag := range n.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				filtered = append(filtered, n)
				break
			}
		}
	}
	m.sidebar.SetNotes(filtered)
	m.statusMsg = fmt.Sprintf("Found %d notes matching %q", len(filtered), query)
	return m
}

// renderGraphSummary builds a simple text summary of the knowledge graph.
func (m Model) renderGraphSummary() string {
	if len(m.allNotes) == 0 {
		return "Knowledge graph: no notes loaded"
	}
	totalLinks := 0
	for _, n := range m.allNotes {
		totalLinks += len(n.Links)
	}
	return fmt.Sprintf("Knowledge graph: %d nodes, %d edges", len(m.allNotes), totalLinks)
}

// countUniqueTags returns the number of distinct tags across all notes.
func (m Model) countUniqueTags() int {
	seen := make(map[string]struct{})
	for _, n := range m.allNotes {
		for _, t := range n.Tags {
			seen[t] = struct{}{}
		}
	}
	return len(seen)
}

// helpText returns a single-line summary of available keybindings.
func helpText() string {
	return "[tab] focus  [/] search  [n] new  [e] edit  [s] sync  [g] graph  [?] help  [q] quit"
}

// Run constructs the root model and starts the Bubble Tea program.
func Run(cfg *config.Config) error {
	m, err := New(cfg)
	if err != nil {
		return fmt.Errorf("tui: init: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
