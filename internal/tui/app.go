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

// Package tui wires the sidebar, editor, and search pane into a Bubble Tea
// MVU application.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	focusNewNote      // inline note creation mode
	focusFolderSelect // PARA folder selection after entering a title
	focusLinkSelect   // Link selection when multiple links are present
)

// notesLoadedMsg is dispatched when the initial note scan completes.
type notesLoadedMsg struct {
	notes          []*notes.Note
	selectedNoteID string
}

// noteLoadedMsg is dispatched when a single note is reloaded from disk.
type noteLoadedMsg struct {
	note *notes.Note
}

// editorFinishedMsg is dispatched when the external editor process exits.
type editorFinishedMsg struct {
	noteID string
}

// errMsg carries an error back to the main update loop.
type errMsg struct{ err error }

// statusMsg carries a transient human-readable status update.
type statusMsg struct{ msg string }

// successMsg carries a success message (shown with green styling).
type successMsg struct{ msg string }

// pasteNoteMsg is dispatched when a paste-to-note operation completes.
// It carries the freshly-saved note so the editor can display the new content
// immediately without a full vault reload.
type pasteNoteMsg struct {
	note  *notes.Note
	bytes int
}

// Model is the root Bubble Tea model. It owns all child pane models and
// orchestrates focus, layout, and data flow.
type Model struct {
	cfg             *config.Config
	store           *notes.Store
	theme           *styles.Theme
	sidebar         panes.Sidebar
	editor          panes.Editor
	search          panes.SearchPane
	newNote         textinput.Model // inline new-note input
	spinner         spinner.Model
	help            help.Model
	showHelp        bool
	syncing         bool
	focused         focus
	allNotes        []*notes.Note // unfiltered master list
	width           int
	height          int
	ready           bool // true once the terminal size is known
	err             error
	statusMsg       string
	isSuccess       bool     // whether statusMsg should render green
	showSplash      bool     // controls splash screen
	clipboardBody   string   // clipboard content staged for next note creation
	pendingTitle    string   // title staged while selecting PARA folder
	pendingBody     string   // body staged while selecting PARA folder
	paraFolders     []string // detected PARA folders for folder-select mode
	folderSelectIdx int      // current selection (0 = root, 1..n = PARA folder)

	pendingLinks      []string // URLs/paths for focusLinkSelect
	pendingLinkTitles []string // Display titles for focusLinkSelect
	linkSelectIdx     int      // current selection for focusLinkSelect
	appVersion      string
}

type keyMap struct {
	Tab    key.Binding
	Search key.Binding
	New    key.Binding
	Paste  key.Binding
	Edit   key.Binding
	Sync   key.Binding
	Graph  key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Search, k.New, k.Paste, k.Edit, k.Sync, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Search, k.New, k.Paste},
		{k.Edit, k.Sync, k.Graph, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Tab:    key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab", "focus")),
	Search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "cmd palette")),
	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new note")),
	Paste:  key.NewBinding(key.WithKeys("ctrl+v"), key.WithHelp("ctrl+v", "paste clipboard")),
	Edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Sync:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	Graph:  key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "graph")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// allPaletteCommands is the full list for fuzzy suggestions.
var allPaletteCommands = []string{
	"/add", "/today", "/sync", "/stats", "/open", "/edit", "/rm", "/move", "/theme", "/help", "/quit", "/copy", "/attach", "/links",
}

// New constructs a Model from the provided configuration.
func New(cfg *config.Config, appVersion string) (*Model, error) {
	store, err := notes.NewStore(cfg.VaultPath)
	if err != nil {
		return nil, fmt.Errorf("tui: create note store: %w", err)
	}

	theme := styles.GetTheme(cfg.Theme)

	sidebar := panes.NewSidebar(theme, appVersion)
	sidebar.SetFocused(true)

	editor := panes.NewEditor(theme)
	search := panes.NewSearchPane(theme)

	// New-note inline input — CharLimit = 0 means unlimited.
	// The title itself shouldn't be truncated by the widget.
	nn := textinput.New()
	nn.Placeholder = "Note title..."
	nn.CharLimit = 0
	nn.Width = 50
	nn.PromptStyle = lipgloss.NewStyle().Foreground(theme.AccentAlt).Bold(true)
	nn.TextStyle = lipgloss.NewStyle().Foreground(theme.TextBright)
	nn.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.Muted)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Accent)

	h := help.New()

	return &Model{
		cfg:        cfg,
		store:      store,
		theme:      theme,
		sidebar:    sidebar,
		editor:     editor,
		search:     search,
		newNote:    nn,
		spinner:    s,
		help:       h,
		focused:    focusSidebar,
		showSplash: true,
		appVersion: appVersion,
	}, nil
}

// Init loads notes from the store in the background and enables bracketed
// paste mode so that pasted text (any length) arrives as a single message
// instead of being fed character by character to the active input.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnableBracketedPaste, // enables paste → KeyMsg{Paste:true}
		m.spinner.Tick,
		func() tea.Msg {
			noteList, err := m.store.List(notes.ListOptions{})
			if err != nil {
				return errMsg{err: err}
			}
			return notesLoadedMsg{notes: noteList, selectedNoteID: ""}
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
		m.newNote.Width = msg.Width - 20
		m.layout()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case notesLoadedMsg:
		m.allNotes = msg.notes
		query := m.search.Query()
		if query != "" {
			m = m.filterNotes(query)
		} else {
			m.sidebar.SetNotes(msg.notes)
		}
		if msg.selectedNoteID != "" {
			m.sidebar.SelectNoteByID(msg.selectedNoteID)
		}
		if selected := m.sidebar.SelectedNote(); selected != nil {
			m.editor.SetNote(selected)
		} else {
			m.editor.SetNote(nil)
		}
		m.statusMsg = fmt.Sprintf("Loaded %d notes", len(msg.notes))
		m.isSuccess = false

	case noteLoadedMsg:
		// Update the note in-place in allNotes so the sidebar reflects changes.
		for i, n := range m.allNotes {
			if n.ID == msg.note.ID {
				m.allNotes[i] = msg.note
				break
			}
		}
		notes.SortNotes(m.allNotes, "updated")

		query := m.search.Query()
		if query != "" {
			m = m.filterNotes(query)
		} else {
			m.sidebar.SetNotes(m.allNotes)
		}
		m.sidebar.SelectNoteByID(msg.note.ID)

		if selected := m.sidebar.SelectedNote(); selected != nil {
			m.editor.SetNote(selected)
		} else {
			m.editor.SetNote(nil)
		}
		m.statusMsg = "✓ Returned from editor"
		m.isSuccess = true

	case errMsg:
		m.err = msg.err
		m.statusMsg = "✗ " + msg.err.Error()
		m.isSuccess = false
		m.syncing = false

	case statusMsg:
		m.statusMsg = msg.msg
		m.isSuccess = false
		m.syncing = false

	case successMsg:
		m.statusMsg = "✓ " + msg.msg
		m.isSuccess = true
		m.syncing = false

	case editorFinishedMsg:
		m.statusMsg = "✓ Returned from editor"
		m.isSuccess = true
		var targetNote *notes.Note
		for _, n := range m.allNotes {
			if n.ID == msg.noteID {
				targetNote = n
				break
			}
		}
		if targetNote != nil {
			cmds = append(cmds, m.reloadNote(targetNote))
		} else {
			cmds = append(cmds, m.reloadNotes(msg.noteID))
		}

	case pasteNoteMsg:
		// Update the note in-place in allNotes so the sidebar reflects the change.
		for i, n := range m.allNotes {
			if n.ID == msg.note.ID {
				m.allNotes[i] = msg.note
				break
			}
		}
		// Push the fresh note content to the editor without a full reload.
		m.editor.SetNote(msg.note)
		m.statusMsg = fmt.Sprintf("✓ 📋 %s pasted in «%s»", formatBytes(msg.bytes), msg.note.Title)
		m.isSuccess = true

	case panes.SearchQueryMsg:
		if strings.HasPrefix(msg.Query, "/") {
			// Solo ejecutar el comando de paleta cuando Live=false (Enter).
			// Cuando Live=true el usuario sigue escribiendo: solo actualizamos
			// las sugerencias de la paleta sin ejecutar nada.
			if !msg.Live {
				m.search.SetQuery("")
				cmd := m.handlePaletteCommand(strings.TrimSpace(msg.Query))
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				// Only reset focus to sidebar if the palette command didn't change it
				// to a specific interactive pane (like focusLinkSelect).
				if m.focused == focusSearch {
					m.setFocus(focusSidebar)
				}
			}
			// En ambos casos no filtramos notas por query de paleta.
			return m, tea.Batch(cmds...)
		}
		// Query de texto normal.
		m = m.filterNotes(msg.Query)
		if !msg.Live {
			// El usuario confirmó con Enter → mover foco al sidebar para
			// que pueda navegar los resultados con ↑ ↓ / j k.
			m.setFocus(focusSidebar)
		}
		// Si Live=true el foco permanece en el buscador para seguir escribiendo.

	case panes.SearchClearMsg:
		m.sidebar.SetNotes(m.allNotes)
		m.search.SetQuery("")
		m.setFocus(focusSidebar)

	case tea.KeyMsg:
		// ── Bracketed paste (Cmd+V / ctrl+V / right-click) ──────────────
		// Bracketed paste mode is enabled in Init(). ALL pasted content
		// arrives as a single KeyMsg with Paste=true — no length limit.
		if msg.Paste {
			text := string(msg.Runes)
			// Normalize carriage returns to standard newlines to preserve formatting
			// while preventing terminal layout issues.
			text = strings.ReplaceAll(text, "\r\n", "\n")
			text = strings.ReplaceAll(text, "\r", "\n")

			if text == "" {
				break
			}
			switch m.focused {
			case focusNewNote:
				// Stage as note body
				m.clipboardBody = text
				// Auto-fill the title input with the first line if it's currently empty
				if strings.TrimSpace(m.newNote.Value()) == "" {
					lines := strings.Split(strings.TrimSpace(text), "\n")
					if len(lines) > 0 {
						title := lines[0]
						if len([]rune(title)) > 40 {
							title = string([]rune(title)[:40]) + "..."
						}
						m.newNote.SetValue(title)
					}
				}
			case focusSidebar, focusEditor:
				// Immediate feedback
				m.statusMsg = fmt.Sprintf("📋 Pasting %s...", formatBytes(len(text)))
				m.isSuccess = false
				cmds = append(cmds, m.pasteTextToSelectedNote(text))
			}
			return m, tea.Batch(cmds...)
		}

		// ── New-note mode ────────────────────────────────────────────────
		if m.focused == focusNewNote {
			switch {
			case msg.Type == tea.KeyEnter:
				title := strings.TrimSpace(m.newNote.Value())
				body := m.clipboardBody

				// Fallback auto-title if input is still empty somehow
				if title == "" && body != "" {
					lines := strings.Split(strings.TrimSpace(body), "\n")
					if len(lines) > 0 {
						title = lines[0]
						if len([]rune(title)) > 40 {
							title = string([]rune(title)[:40]) + "..."
						}
					}
				}

				m.newNote.SetValue("")
				m.clipboardBody = ""

				if title == "" {
					m.setFocus(focusSidebar)
					m.statusMsg = "Cancelled — note needs a title"
					m.isSuccess = false
					return m, tea.Batch(cmds...)
				}

				// If user typed "folder/title", honour it directly — no picker needed
				if idx := strings.LastIndex(title, "/"); idx != -1 {
					folder := title[:idx]
					title = title[idx+1:]
					m.setFocus(focusSidebar)
					note, err := m.store.Create(folder, title, nil, body)
					if err != nil {
						m.statusMsg = "✗ " + err.Error()
						m.isSuccess = false
						return m, tea.Batch(cmds...)
					}
					if body != "" {
						m.statusMsg = fmt.Sprintf("✓ Created: %s  ·  📋 %s pasted", title, formatBytes(len(body)))
					} else {
						m.statusMsg = "✓ Created: " + title
					}
					m.isSuccess = true
					cmds = append(cmds, m.reloadNotes(note.ID))
					return m, tea.Batch(cmds...)
				}

				// Detect PARA structure; skip picker when vault has no folders
				paraFolders := m.store.DetectPARAFolders()
				if len(paraFolders) == 0 {
					m.setFocus(focusSidebar)
					note, err := m.store.Create("", title, nil, body)
					if err != nil {
						m.statusMsg = "✗ " + err.Error()
						m.isSuccess = false
						return m, tea.Batch(cmds...)
					}
					if body != "" {
						m.statusMsg = fmt.Sprintf("✓ Created: %s  ·  📋 %s pasted", title, formatBytes(len(body)))
					} else {
						m.statusMsg = "✓ Created: " + title
					}
					m.isSuccess = true
					cmds = append(cmds, m.reloadNotes(note.ID))
					return m, tea.Batch(cmds...)
				}

				// Show PARA folder picker before creating the note
				m.pendingTitle = title
				m.pendingBody = body
				m.paraFolders = paraFolders
				m.folderSelectIdx = 0
				m.setFocus(focusFolderSelect)
				return m, tea.Batch(cmds...)

			case msg.Type == tea.KeyEsc:
				m.newNote.SetValue("")
				m.clipboardBody = ""
				m.setFocus(focusSidebar)
				m.statusMsg = "Cancelled"
				m.isSuccess = false
				return m, tea.Batch(cmds...)

			// ctrl+v fallback for terminals without bracketed paste support
			case msg.String() == "ctrl+v":
				text, err := clipboard.ReadAll()
				if err != nil || text == "" {
					m.statusMsg = "✗ Clipboard empty or unavailable"
					m.isSuccess = false
					return m, tea.Batch(cmds...)
				}
				m.clipboardBody = text
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.newNote, cmd = m.newNote.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// ── Folder-select mode ───────────────────────────────────────────
		if m.focused == focusFolderSelect {
			switch {
			case msg.Type == tea.KeyEnter:
				var folder string
				if m.folderSelectIdx > 0 {
					folder = m.paraFolders[m.folderSelectIdx-1]
				}
				title := m.pendingTitle
				body := m.pendingBody
				m.pendingTitle = ""
				m.pendingBody = ""
				m.paraFolders = nil
				m.folderSelectIdx = 0
				m.setFocus(focusSidebar)
				note, err := m.store.Create(folder, title, nil, body)
				if err != nil {
					m.statusMsg = "✗ " + err.Error()
					m.isSuccess = false
					return m, tea.Batch(cmds...)
				}
				if body != "" {
					m.statusMsg = fmt.Sprintf("✓ Created: %s  ·  📋 %s pasted", title, formatBytes(len(body)))
				} else {
					m.statusMsg = "✓ Created: " + title
				}
				m.isSuccess = true
				cmds = append(cmds, m.reloadNotes(note.ID))
				return m, tea.Batch(cmds...)

			case msg.Type == tea.KeyEsc:
				m.pendingTitle = ""
				m.pendingBody = ""
				m.paraFolders = nil
				m.folderSelectIdx = 0
				m.setFocus(focusSidebar)
				m.statusMsg = "Cancelled"
				m.isSuccess = false
				return m, tea.Batch(cmds...)

			case msg.String() == "left" || msg.String() == "h":
				if m.folderSelectIdx > 0 {
					m.folderSelectIdx--
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "right" || msg.String() == "l":
				if m.folderSelectIdx < len(m.paraFolders) {
					m.folderSelectIdx++
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "up" || msg.String() == "k":
				if m.folderSelectIdx > 0 {
					m.folderSelectIdx--
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "down" || msg.String() == "j":
				if m.folderSelectIdx < len(m.paraFolders) {
					m.folderSelectIdx++
				}
				return m, tea.Batch(cmds...)
			}
			return m, tea.Batch(cmds...)
		}

		// ── Link-select mode ───────────────────────────────────────────
		if m.focused == focusLinkSelect {
			switch {
			case msg.Type == tea.KeyEnter:
				if len(m.pendingLinks) > 0 {
					m.openLink(m.pendingLinks[m.linkSelectIdx])
				}
				m.pendingLinks = nil
				m.pendingLinkTitles = nil
				m.linkSelectIdx = 0
				m.setFocus(focusSidebar)
				return m, tea.Batch(cmds...)

			case msg.Type == tea.KeyEsc:
				m.pendingLinks = nil
				m.pendingLinkTitles = nil
				m.linkSelectIdx = 0
				m.setFocus(focusSidebar)
				m.statusMsg = "Cancelled"
				m.isSuccess = false
				return m, tea.Batch(cmds...)

			case msg.String() == "left" || msg.String() == "h":
				if m.linkSelectIdx > 0 {
					m.linkSelectIdx--
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "right" || msg.String() == "l":
				if m.linkSelectIdx < len(m.pendingLinks)-1 {
					m.linkSelectIdx++
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "up" || msg.String() == "k":
				if m.linkSelectIdx > 0 {
					m.linkSelectIdx--
				}
				return m, tea.Batch(cmds...)

			case msg.String() == "down" || msg.String() == "j":
				if m.linkSelectIdx < len(m.pendingLinks)-1 {
					m.linkSelectIdx++
				}
				return m, tea.Batch(cmds...)
			}
			return m, tea.Batch(cmds...)
		}

		// ── Global shortcuts ─────────────────────────────────────────────
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
				// Clear the query. The '/' keypress will be forwarded to the search pane
				// at the bottom of this Update function, effectively typing the '/' for us.
				m.search.SetQuery("")
			}

		case "n":
			if m.focused != focusSearch {
				m.clipboardBody = ""
				m.setFocus(focusNewNote)
				m.newNote.SetValue("")
				m.newNote.Focus()
				m.statusMsg = ""
				m.isSuccess = false
			}

		// ctrl+v global: paste clipboard into the selected note (append)
		case "ctrl+v":
			if m.focused == focusSidebar || m.focused == focusEditor {
				cmds = append(cmds, m.pasteClipboardToSelectedNote())
			}

		case "e":
			if m.focused != focusSearch {
				cmd := m.openInEditor()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}

		case "ctrl+g":
			if m.focused != focusSearch {
				m.statusMsg = m.renderGraphSummary()
				m.isSuccess = false
			}

		case "s":
			if m.focused != focusSearch {
				m.syncing = true
				m.statusMsg = "Syncing..."
				m.isSuccess = false
				cmds = append(cmds, m.syncCmd())
			}
		}

		// Hide splash screen on any key press
		if m.showSplash {
			m.showSplash = false
		}
	}

	// Delegate to focused child pane.
	switch m.focused {
	case focusSidebar:
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		cmds = append(cmds, cmd)

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
func (m Model) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}

	titleBar := m.renderTitleBar()

	bodyHeight := m.height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	rightWidth := m.width * 25 / 100
	if rightWidth < 22 {
		rightWidth = 22
	}

	editorView := m.editor.View()
	if m.showSplash {
		editorView = m.editor.SplashView(len(m.allNotes), m.countUniqueTags())
	}

	// Add right border to editor view to separate it from the stats column
	editorBorderView := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(m.theme.Border).
		Render(editorView)

	rightView := m.renderRightColumn(bodyHeight, rightWidth)

	middle := lipgloss.JoinHorizontal(lipgloss.Top,
		m.sidebar.View(),
		editorBorderView,
		rightView,
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

	sidebarWidth := m.width * 25 / 100
	if sidebarWidth < 22 {
		sidebarWidth = 22
	}

	rightWidth := m.width * 25 / 100
	if rightWidth < 22 {
		rightWidth = 22
	}

	editorWidth := m.width - sidebarWidth - rightWidth
	if editorWidth < 20 {
		editorWidth = 20
	}

	m.sidebar.SetSize(sidebarWidth, bodyHeight)
	m.editor.SetSize(editorWidth-1, bodyHeight)
	m.search.SetWidth(m.width)
}

// setFocus moves keyboard focus to the named pane.
func (m *Model) setFocus(f focus) {
	m.focused = f
	m.sidebar.SetFocused(f == focusSidebar)
	m.editor.SetFocused(f == focusEditor)
	m.search.SetActive(f == focusSearch)
	if f != focusNewNote {
		m.newNote.Blur()
	}
}

func (m Model) renderTitleBar() string {
	// Mode label
	var modeLabel string
	switch m.focused {
	case focusNewNote:
		modeLabel = m.theme.ModeIndicator.
			Background(m.theme.AccentAlt).
			Foreground(m.theme.Background).
			Render(" ✦ NEW NOTE ")
	case focusSearch:
		modeLabel = m.theme.ModeIndicator.
			Background(m.theme.Accent).
			Render(" ⌕ SEARCH ")
	case focusEditor:
		modeLabel = m.theme.ModeIndicator.
			Background(m.theme.Muted).
			Render(" ⊞ PREVIEW ")

	case focusFolderSelect:
		modeLabel = m.theme.ModeIndicator.
			Background(m.theme.AccentAlt).
			Foreground(m.theme.Background).
			Render(" 📁 SAVE TO ")
	}

	leftStyle := m.theme.AppName.Background(m.theme.Surface)
	left := leftStyle.Render(" 🧠 neuron")
	if modeLabel != "" {
		left = left + "  " + modeLabel
	}

	rightStyle := m.theme.KeyHint.
		Foreground(m.theme.Muted).
		Background(m.theme.Surface)
	right := rightStyle.Render("[n] new  [/] cmd  [?] help  [q] quit ")

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

	switch m.focused {
	case focusNewNote:
		// Show the inline new-note input
		prompt := m.theme.NewNoteInput.Render(" ✦ New note → ")
		inputView := m.theme.StatusBar.Render(m.newNote.View())

		// Clipboard indicator: show size badge if content is staged, hint otherwise
		var clipIndicator string
		if m.clipboardBody != "" {
			// Preview first ~30 chars of the clipboard
			preview := strings.ReplaceAll(m.clipboardBody, "\n", " ")
			if len([]rune(preview)) > 30 {
				preview = string([]rune(preview)[:30]) + "…"
			}
			clipIndicator = "  " + lipgloss.NewStyle().
				Foreground(m.theme.Background).
				Background(m.theme.AccentAlt).
				Bold(true).
				Padding(0, 1).
				Render(fmt.Sprintf("📋 %s ready", formatBytes(len(m.clipboardBody)))) +
				"  " + lipgloss.NewStyle().
				Foreground(m.theme.Muted).
				Render("\""+preview+"\"")
		} else {
			clipIndicator = "  " + lipgloss.NewStyle().
				Foreground(m.theme.Muted).
				Render("[ctrl+v] paste clipboard · [esc] cancel")
		}

		left = lipgloss.JoinHorizontal(lipgloss.Left,
			m.theme.StatusBar.Render(prompt),
			inputView,
			m.theme.StatusBar.Render(clipIndicator),
		)

	case focusFolderSelect:
		// Build selectable folder chips — index 0 is always Root
		labels := make([]string, 0, len(m.paraFolders)+1)
		labels = append(labels, "📂 Root Vault")
		for _, pf := range m.paraFolders {
			labels = append(labels, "📁 "+pf)
		}
		var folderChips []string
		for i, label := range labels {
			var chip string
			if i == m.folderSelectIdx {
				chip = lipgloss.NewStyle().
					Foreground(m.theme.Background).
					Background(m.theme.AccentAlt).
					Bold(true).
					Padding(0, 1).
					Render("▶ " + label)
			} else {
				chip = lipgloss.NewStyle().
					Foreground(m.theme.TextBright).
					Background(m.theme.Surface2).
					Padding(0, 1).
					Render(label)
			}
			folderChips = append(folderChips, chip)
		}
		promptLabel := lipgloss.NewStyle().
			Foreground(m.theme.AccentAlt).
			Background(m.theme.Surface).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("📁 «%s» →", m.pendingTitle))
		navHint := lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Background(m.theme.Surface).
			Render("  [← →] navigate · [Enter] confirm · [Esc] cancel")
		left = promptLabel + "  " + strings.Join(folderChips, " ") + navHint

	case focusLinkSelect:
		var linkChips []string
		for i, title := range m.pendingLinkTitles {
			var chip string
			if i == m.linkSelectIdx {
				chip = lipgloss.NewStyle().
					Foreground(m.theme.Background).
					Background(m.theme.AccentAlt).
					Bold(true).
					Padding(0, 1).
					Render("▶ " + title)
			} else {
				chip = lipgloss.NewStyle().
					Foreground(m.theme.TextBright).
					Background(m.theme.Surface2).
					Padding(0, 1).
					Render(title)
			}
			linkChips = append(linkChips, chip)
		}
		promptLabel := lipgloss.NewStyle().
			Foreground(m.theme.AccentAlt).
			Background(m.theme.Surface).
			Bold(true).
			Padding(0, 1).
			Render("🔗 Abrir enlace →")
		navHint := lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Background(m.theme.Surface).
			Render("  [← →] navigate · [Enter] confirm · [Esc] cancel")
		left = promptLabel + "  " + strings.Join(linkChips, " ") + navHint

	case focusSearch:
		query := m.search.Query()
		if strings.HasPrefix(query, "/") {
			// Show command palette with fuzzy suggestions as chips
			matches := fuzzy.Find(query, allPaletteCommands)
			var chips []string
			for _, match := range matches {
				chip := lipgloss.NewStyle().
					Foreground(m.theme.Info).
					Background(m.theme.Surface2).
					Padding(0, 1).
					Render(match.Str)
				chips = append(chips, chip)
			}
			chipRow := ""
			if len(chips) > 0 {
				chipRow = " " + strings.Join(chips, " ")
			} else {
				chipRow = lipgloss.NewStyle().
					Foreground(m.theme.Muted).
					Render("  No matching commands")
			}
			left = m.search.View() + chipRow
		} else {
			left = m.search.View()
		}

	default:
		// Build a folder breadcrumb for the currently selected note.
		folderCrumb := ""
		if note := m.sidebar.SelectedNote(); note != nil {
			folder := noteFolderLabel(note, m.cfg.VaultPath)
			if folder != "" {
				folderCrumb = lipgloss.NewStyle().
					Foreground(m.theme.Muted).
					Background(m.theme.Surface).
					Render("  📂 " + folder)
			}
		}

		if m.statusMsg != "" {
			msg := m.statusMsg
			if m.syncing {
				msg = m.spinner.View() + " " + msg
				left = m.theme.StatusBar.Render(" "+msg) + folderCrumb
			} else if m.isSuccess {
				left = m.theme.SuccessMsg.Render(msg) + folderCrumb
			} else {
				left = m.theme.StatusBar.Render(" "+msg) + folderCrumb
			}
		} else if folderCrumb != "" {
			hint := lipgloss.NewStyle().
				Foreground(m.theme.Muted).
				Background(m.theme.Surface).
				Render(" [n] new  [ctrl+v] paste to note  [/] commands  [e] edit  [s] sync  [ctrl+g] graph")
			left = hint + folderCrumb
		} else {
			hint := lipgloss.NewStyle().
				Foreground(m.theme.Muted).
				Background(m.theme.Surface).
				Render(" [n] new  [ctrl+v] paste to note  [/] commands  [e] edit  [s] sync  [ctrl+g] graph")
			left = hint
		}
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
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}
	base := parts[0]

	switch base {
	case "/quit", "/q":
		return tea.Quit

	case "/sync", "/s":
		m.syncing = true
		m.statusMsg = "Syncing..."
		m.isSuccess = false
		return m.syncCmd()

	case "/today", "/t":
		title := "Daily " + time.Now().Format("2006-01-02")
		var targetID string
		note, err := m.store.Get(title)
		if err != nil {
			content, _ := m.store.RenderTemplate("daily", title)
			if content == "" {
				content = "## 🎯 Today's goals\n- [ ] \n\n## 📝 Notes\n\n## 🔗 Links\n"
			}
			newNote, err := m.store.Create("", title, []string{"daily"}, content)
			if err != nil {
				m.statusMsg = "✗ Error creating daily note: " + err.Error()
				m.isSuccess = false
				return nil
			}
			targetID = newNote.ID
		} else {
			targetID = note.ID
		}
		m.statusMsg = "✓ Daily note ready"
		m.isSuccess = true
		return m.reloadNotes(targetID)

	case "/add":
		if len(parts) < 2 {
			m.statusMsg = "Usage: /add <title>"
			m.isSuccess = false
			return nil
		}
		title := strings.Join(parts[1:], " ")
		// If user typed "folder/title", honour it directly — no picker needed
		if idx := strings.LastIndex(title, "/"); idx != -1 {
			folder := title[:idx]
			title = title[idx+1:]
			note, err := m.store.Create(folder, title, nil, "")
			if err != nil {
				m.statusMsg = "✗ " + err.Error()
				m.isSuccess = false
				return nil
			}
			m.statusMsg = "✓ Created: " + title
			m.isSuccess = true
			return m.reloadNotes(note.ID)
		}
		// Detect PARA structure; skip picker when vault has no folders
		paraFolders := m.store.DetectPARAFolders()
		if len(paraFolders) == 0 {
			note, err := m.store.Create("", title, nil, "")
			if err != nil {
				m.statusMsg = "✗ " + err.Error()
				m.isSuccess = false
				return nil
			}
			m.statusMsg = "✓ Created: " + title
			m.isSuccess = true
			return m.reloadNotes(note.ID)
		}
		// Show PARA folder picker before creating the note
		m.pendingTitle = title
		m.pendingBody = ""
		m.paraFolders = paraFolders
		m.folderSelectIdx = 0
		m.setFocus(focusFolderSelect)
		return nil

	case "/stats":
		count := len(m.allNotes)
		tags := m.countUniqueTags()
		m.statusMsg = fmt.Sprintf("Vault: %d notes · %d tags", count, tags)
		m.isSuccess = false
		return nil

	case "/open", "/o":
		go func() {
			_ = exec.Command("open", "--", m.cfg.VaultPath).Run()
		}()
		m.statusMsg = "✓ Opened vault in Finder"
		m.isSuccess = true
		return nil

	case "/edit", "/e":
		return m.openInEditor()

	case "/rm":
		note := m.sidebar.SelectedNote()
		if note == nil {
			m.statusMsg = "✗ No note selected"
			m.isSuccess = false
			return nil
		}
		if err := m.store.Delete(note.ID); err != nil {
			m.statusMsg = "✗ " + err.Error()
			m.isSuccess = false
			return nil
		}
		m.statusMsg = "✓ Deleted: " + note.Title
		m.isSuccess = true
		return m.reloadNotes("")

	case "/copy", "/c":
		note := m.sidebar.SelectedNote()
		if note == nil {
			m.statusMsg = "✗ No note selected"
			m.isSuccess = false
			return nil
		}
		if err := clipboard.WriteAll(note.RawContent); err != nil {
			m.statusMsg = "✗ Error copying to clipboard: " + err.Error()
			m.isSuccess = false
		} else {
			m.statusMsg = "✓ Note copied to clipboard"
			m.isSuccess = true
		}
		return nil

	case "/attach", "/a":
		if len(parts) < 2 {
			m.statusMsg = "Usage: /attach <URL or drag&drop file here> then press Enter"
			m.isSuccess = false
			return nil
		}
		arg := strings.Join(parts[1:], " ")
		m.statusMsg = "Attaching asset..."
		m.isSuccess = false
		
		return func() tea.Msg {
			note := m.sidebar.SelectedNote()
			if note == nil {
				return errMsg{err: fmt.Errorf("no note selected")}
			}
			
			err := m.store.AttachAsset(note.ID, arg)
			if err != nil {
				return errMsg{err: err}
			}
			
			updatedNote, err := m.store.Get(note.ID)
			if err != nil {
				return errMsg{err: err}
			}
			// Use pasteNoteMsg to efficiently update the editor view
			return pasteNoteMsg{note: updatedNote, bytes: len(arg)}
		}

	case "/links", "/l":
		note := m.sidebar.SelectedNote()
		if note == nil {
			m.statusMsg = "✗ No note selected"
			m.isSuccess = false
			return nil
		}
		
		re := regexp.MustCompile(`(https?://[^\s)\]]+|file://[^\s)\]]+)|\[(.*?)\]\(([^)]+)\)`)
		matches := re.FindAllStringSubmatch(note.RawContent, -1)
		
		if len(matches) == 0 {
			m.statusMsg = "✗ No links found in note"
			m.isSuccess = false
			return nil
		}
		
		var links []string
		var titles []string
		for _, m := range matches {
			if m[1] != "" {
				links = append(links, m[1])
				// Shorten long URLs for display
				display := m[1]
				if len(display) > 30 {
					display = display[:27] + "..."
				}
				titles = append(titles, display)
			} else if m[3] != "" {
				links = append(links, m[3])
				title := m[2]
				if title == "" {
					title = filepath.Base(m[3])
				}
				titles = append(titles, title)
			}
		}

		if len(links) == 0 {
			m.statusMsg = "✗ No valid links found"
			m.isSuccess = false
			return nil
		}

		if len(links) == 1 {
			m.openLink(links[0])
		} else {
			m.pendingLinks = links
			m.pendingLinkTitles = titles
			m.linkSelectIdx = 0
			m.setFocus(focusLinkSelect)
			m.statusMsg = fmt.Sprintf("Found %d links. Use arrow keys to select.", len(links))
			m.isSuccess = true
		}
		return nil

	case "/move", "/m":
		note := m.sidebar.SelectedNote()
		if note == nil {
			m.statusMsg = "✗ No note selected"
			m.isSuccess = false
			return nil
		}
		if len(parts) < 2 {
			m.statusMsg = "Usage: /move projects|areas|resources|archive|root"
			m.isSuccess = false
			return nil
		}
		target := parts[1]
		paraFolders := m.store.DetectPARAFolders()
		var targetFolder string
		matched := false
		for _, pf := range paraFolders {
			if strings.Contains(strings.ToLower(pf), strings.ToLower(target)) {
				targetFolder = pf
				matched = true
				break
			}
		}
		if !matched {
			if strings.ToLower(target) == "root" || target == "/" {
				targetFolder = ""
			} else {
				targetFolder = target
			}
		}
		if err := m.store.Move(note.ID, targetFolder); err != nil {
			m.statusMsg = "✗ " + err.Error()
			m.isSuccess = false
			return nil
		}
		if targetFolder == "" {
			m.statusMsg = "✓ Moved note to root vault"
		} else {
			m.statusMsg = "✓ Moved note to " + targetFolder
		}
		m.isSuccess = true
		return m.reloadNotes(note.ID)

	case "/theme":
		if len(parts) < 2 {
			m.statusMsg = "Usage: /theme dark|light"
			m.isSuccess = false
			return nil
		}
		newTheme := parts[1]
		if newTheme != "dark" && newTheme != "light" {
			m.statusMsg = "✗ Theme must be dark or light"
			m.isSuccess = false
			return nil
		}
		m.cfg.Theme = newTheme
		m.theme = styles.GetTheme(newTheme)
		m.sidebar = panes.NewSidebar(m.theme, m.appVersion)
		m.sidebar.SetFocused(true)
		m.editor = panes.NewEditor(m.theme)
		m.search = panes.NewSearchPane(m.theme)
		m.layout()
		m.sidebar.SetNotes(m.allNotes)
		m.statusMsg = "✓ Theme changed to " + newTheme
		m.isSuccess = true
		return nil

	case "/help", "/?":
		m.showHelp = true
		return nil

	default:
		m.statusMsg = "Unknown command: " + base + "  (try /add /today /sync /stats /open /edit /rm /theme /quit)"
		m.isSuccess = false
		return nil
	}
}

// reloadNotes fetches the note list from disk and updates the sidebar.
func (m *Model) reloadNotes(selectedNoteID string) tea.Cmd {
	if selectedNoteID == "" {
		if sel := m.sidebar.SelectedNote(); sel != nil {
			selectedNoteID = sel.ID
		}
	}
	store := m.store
	return func() tea.Msg {
		noteList, err := store.List(notes.ListOptions{})
		if err != nil {
			return errMsg{err: err}
		}
		return notesLoadedMsg{
			notes:          noteList,
			selectedNoteID: selectedNoteID,
		}
	}
}

// reloadNote re-reads and parses a single note from disk and updates the TUI model.
func (m *Model) reloadNote(note *notes.Note) tea.Cmd {
	store := m.store
	return func() tea.Msg {
		fresh, err := store.Reload(note)
		if err != nil {
			return errMsg{err: err}
		}
		return noteLoadedMsg{note: fresh}
	}
}

// openInEditor opens the currently selected note in the configured editor.
func (m *Model) openInEditor() tea.Cmd {
	note := m.sidebar.SelectedNote()
	if note == nil {
		m.statusMsg = "✗ No note selected"
		m.isSuccess = false
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
		return editorFinishedMsg{noteID: note.ID}
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
		return successMsg{msg: result.Message}
	}
}

// filterNotes returns a new Model with the sidebar filtered to notes matching
// the query string (case-insensitive title/tag match).
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
	m.isSuccess = false
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
	return fmt.Sprintf("Knowledge graph: %d nodes · %d edges", len(m.allNotes), totalLinks)
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

// pasteClipboardToSelectedNote reads the system clipboard and appends its
// full contents to the currently selected note, then saves it to disk.
func (m *Model) pasteClipboardToSelectedNote() tea.Cmd {
	note := m.sidebar.SelectedNote()
	if note == nil {
		m.statusMsg = "✗ No note selected — select one first"
		m.isSuccess = false
		return nil
	}
	store := m.store

	return func() tea.Msg {
		text, err := clipboard.ReadAll()
		if err != nil || text == "" {
			return errMsg{err: fmt.Errorf("clipboard empty or unavailable")}
		}
		return appendTextToNote(store, note, text)
	}
}

// pasteTextToSelectedNote appends the given text to the selected note on disk.
// Used when the content arrives via bracketed paste (msg.Paste == true).
func (m *Model) pasteTextToSelectedNote(text string) tea.Cmd {
	note := m.sidebar.SelectedNote()
	if note == nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("no note selected — navigate to one first")}
		}
	}
	store := m.store

	return func() tea.Msg {
		return appendTextToNote(store, note, text)
	}
}

// appendTextToNote is the shared logic for both paste helpers.
// It re-reads the note from disk, appends the text, saves and returns a
// pasteNoteMsg so the editor can refresh without a full vault reload.
func appendTextToNote(store *notes.Store, note *notes.Note, text string) tea.Msg {
	// If the text looks like a file path or URL, try to attach it automatically.
	cleanPath := strings.TrimSpace(text)
	cleanPath = strings.Trim(cleanPath, "'\" ")
	cleanPath = strings.ReplaceAll(cleanPath, "\\ ", " ") // Fix macOS Terminal drag and drop

	// Only try if it's a single line and looks like an absolute path or http url.
	if !strings.Contains(cleanPath, "\n") {
		// If it's an existing file or a URL (AttachAsset will handle download for images)
		if _, err := os.Stat(cleanPath); err == nil || strings.HasPrefix(cleanPath, "http://") || strings.HasPrefix(cleanPath, "https://") {
			if err := store.AttachAsset(note.ID, cleanPath); err == nil {
				fresh, _ := store.Get(note.ID)
				return pasteNoteMsg{note: fresh, bytes: len(text)}
			}
			// If AttachAsset failed (e.g. not an image URL), fall through and paste as text
		}
	}

	// Re-read from disk to get the freshest content.
	fresh, err := store.Reload(note)
	if err != nil {
		fresh = note
	}

	// Append after a blank separator line.
	// We use fresh.Content (the body without frontmatter) so we don't duplicate YAML.
	sep := "\n\n"
	if fresh.Content == "" {
		sep = ""
	}
	fresh.Content = fresh.Content + sep + text

	if err := store.Update(fresh); err != nil {
		return errMsg{err: fmt.Errorf("saving note: %w", err)}
	}

	return pasteNoteMsg{note: fresh, bytes: len(text)}
}

// formatBytes returns a human-readable byte size string (B / KB / MB).
func formatBytes(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

// noteFolderLabel returns the relative folder of a note within the vault,
// e.g. "1. Projects" or "1. Projects/Web Apps". Returns "" for root notes.
func noteFolderLabel(note *notes.Note, vaultPath string) string {
	rel := note.RelPath
	if rel == "" {
		// Derive from absolute path when RelPath is not set.
		abs := note.Path
		if len(abs) > len(vaultPath)+1 {
			rel = abs[len(vaultPath)+1:]
		}
	}
	if rel == "" {
		return ""
	}
	// Strip the filename — keep only the directory portion.
	var dir string
	if idx := strings.LastIndexAny(rel, "/\\"); idx >= 0 {
		dir = rel[:idx]
	} else {
		return "" // file sits directly at vault root
	}
	return dir
}

// renderRightColumn builds a column of statistics and quick shortcuts.
func (m Model) renderRightColumn(height, width int) string {
	theme := m.theme

	// Card style: rounded border, surface background, padding
	cardStyle := lipgloss.NewStyle().
		Background(theme.Surface).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		Width(width - 2) // account for borders

	// ─── STATISTICS ───
	statsTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		MarginBottom(1)
	statsTitle := statsTitleStyle.Render("STATISTICS")

	totalNotes := len(m.allNotes)
	uniqueTags := m.countUniqueTags()

	labelStyle := lipgloss.NewStyle().Foreground(theme.Text)
	valStyle := lipgloss.NewStyle().Foreground(theme.AccentAlt).Bold(true)

	// Build stats rows
	statsRows := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("Notes:     "), valStyle.Render(fmt.Sprintf("%d", totalNotes))),
		fmt.Sprintf("%s %s", labelStyle.Render("Tags:      "), valStyle.Render(fmt.Sprintf("%d", uniqueTags))),
	}

	// Count PARA folders dynamically
	paraFolders := m.store.DetectPARAFolders()
	if len(paraFolders) > 0 {
		statsRows = append(statsRows, "")
		statsRows = append(statsRows, lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render("Vault (PARA):"))
		for _, pf := range paraFolders {
			count := 0
			for _, n := range m.allNotes {
				if strings.HasPrefix(n.RelPath, pf+"/") || n.RelPath == pf {
					count++
				}
			}
			displayName := pf
			runes := []rune(displayName)
			if len(runes) > 15 {
				displayName = string(runes[:12]) + "..."
			}
			statsRows = append(statsRows, fmt.Sprintf("%s %s", labelStyle.Render(fmt.Sprintf("  %-11s ", displayName+":")), valStyle.Render(fmt.Sprintf("%d", count))))
		}
	}

	// Extra (non-PARA) folders
	extraFolders := m.store.ExtraFolders()
	if len(extraFolders) > 0 {
		statsRows = append(statsRows, "")
		statsRows = append(statsRows, lipgloss.NewStyle().Foreground(theme.Muted).Bold(true).Render("Other folders:"))
		for _, ef := range extraFolders {
			count := 0
			for _, n := range m.allNotes {
				if strings.HasPrefix(n.RelPath, ef+"/") || n.RelPath == ef {
					count++
				}
			}
			displayName := ef
			runes := []rune(displayName)
			if len(runes) > 15 {
				displayName = string(runes[:12]) + "..."
			}
			statsRows = append(statsRows, fmt.Sprintf("%s %s", labelStyle.Render(fmt.Sprintf("  %-11s ", displayName+":")), valStyle.Render(fmt.Sprintf("%d", count))))
		}
	}

	statsContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{statsTitle}, statsRows...)...)
	statsCard := statsContent
	if width > 6 {
		statsCard = cardStyle.Render(statsContent)
	}

	// ─── SHORTCUTS & COMMANDS ───
	quickTitle := statsTitleStyle.Render("SHORTCUTS & COMMANDS")

	keyStyle := lipgloss.NewStyle().
		Foreground(theme.Background).
		Background(theme.Accent).
		Bold(true).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().Foreground(theme.Text)

	renderHint := func(key, desc string) string {
		k := keyStyle.Render(key)
		d := descStyle.Render(desc)
		// Right align the key or just show it cleanly with dots or space
		spacerWidth := width - 4 - lipgloss.Width(k) - lipgloss.Width(d)
		if spacerWidth < 1 {
			spacerWidth = 1
		}
		spacer := strings.Repeat(" ", spacerWidth)
		return d + spacer + k
	}

	quickRows := []string{
		renderHint("↑/↓", "Navigate"),
		renderHint("Enter", "Select/Confirm"),
		renderHint("n", "New note"),
		renderHint("ctrl+v", "Paste clip"),
		renderHint("ctrl+g", "View graph"),
		renderHint("?", "Help"),
		renderHint("/c", "Copy note"),
		renderHint("/a", "Attach img"),
		renderHint("/l", "Open links"),
		renderHint("/s", "Sync git"),
	}

	quickContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{quickTitle}, quickRows...)...)
	quickCard := quickContent
	if width > 6 {
		quickCard = cardStyle.Render(quickContent)
	}

	// ─── LICENSE ───
	licenseTitle := statsTitleStyle.Render("LICENSE")
	licenseRows := []string{
		fmt.Sprintf("%s", labelStyle.Render("GNU GPL v3 (Copyleft)")),
		fmt.Sprintf("%s", lipgloss.NewStyle().Foreground(theme.Muted).Render("🄲 2025-2026 Daniel Steevin")),
	}
	licenseContent := lipgloss.JoinVertical(lipgloss.Left, append([]string{licenseTitle}, licenseRows...)...)
	licenseCard := licenseContent
	if width > 6 {
		licenseCard = cardStyle.Render(licenseContent)
	}

	// Join all right column elements vertically with spacing
	return lipgloss.JoinVertical(lipgloss.Left, statsCard, quickCard, licenseCard)
}// openLink resolves the given link and opens it using the OS default application.
func (m *Model) openLink(link string) {
	note := m.sidebar.SelectedNote()
	if note == nil {
		m.statusMsg = "✗ No note selected"
		m.isSuccess = false
		return
	}

	// Resolve local paths to absolute paths so the OS 'open' command works.
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "file://") {
		// If it starts with "/", it's an absolute path from the vault root (Obsidian setting).
		if strings.HasPrefix(link, "/") {
			link = filepath.Join(m.cfg.VaultPath, strings.TrimPrefix(link, "/"))
		} else {
			// It's a relative path. Try resolving relative to the note's directory first.
			noteDir := filepath.Dir(note.Path)
			candidate1 := filepath.Join(noteDir, link)
			if _, err := os.Stat(candidate1); err == nil {
				link = candidate1
			} else {
				// Fallback: Try resolving relative to the vault root
				candidate2 := filepath.Join(m.cfg.VaultPath, link)
				if _, err := os.Stat(candidate2); err == nil {
					link = candidate2
				}
			}
		}
	}
	go func() {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", link)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
		default: // linux, freebsd, etc.
			cmd = exec.Command("xdg-open", link)
		}
		_ = cmd.Run()
	}()
	m.statusMsg = "✓ Opened link: " + link
	m.isSuccess = true
}

// Run constructs the root model and starts the Bubble Tea program.
func Run(cfg *config.Config, appVersion string) error {
	m, err := New(cfg, appVersion)
	if err != nil {
		return fmt.Errorf("tui: init: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
