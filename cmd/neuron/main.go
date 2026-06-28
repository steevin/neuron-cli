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

// NeuronCLI — un gestor de conocimiento personal en la terminal, compatible con bóvedas de Obsidian,
// embeddings de IA locales y un servidor MCP para agentes.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/mcp"
	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/search"
	gitsync "github.com/steevin/neuron-cli/internal/sync"
	"github.com/steevin/neuron-cli/internal/tui"
)

// la versión se inyecta al compilar con -ldflags "-X main.version=<tag>".
var version = "1.4.0"

var rootCmd = &cobra.Command{
	Use:   "neuron",
	Short: "🧠 Your second brain, from the terminal",
	Long: `neuron — a terminal-based personal knowledge manager.

Features:
  • Obsidian-compatible Markdown vault (works alongside the Obsidian app)
  • Full-text and semantic search powered by local AI embeddings (Ollama)
  • Daily notes, wikilinks, tags, and frontmatter out of the box
  • Git-based sync to any remote (GitHub, Gitea, …)
  • MCP server for seamless AI agent integration (Claude, GPT-4, …)
  • A buttery-smooth Bubble Tea TUI for keyboard-driven browsing

TUI Commands (Press '/' inside the TUI):
  /copy, /c                Copy the current note to clipboard
  /attach <path_or_url>    Download/copy an asset and attach it to the note
  /links, /l               Open the first URL in the note in your browser
  /add <title>             Create a new note (skip picker with folder/title)
  /edit, /e                Edit current note in external editor
  /rm                      Delete current note
  /sync, /s                Sync vault with Git remote
  /today, /t               Open or create today's daily note
  /theme                   Toggle UI theme (dark/light)
  /open, /o                Open vault folder in the system file browser
  /stats                   Show vault statistics

CLI Graph & Maintenance:
  backlinks <note>          Show notes linking to a note
  orphan                    List notes with no links or backlinks
  timeline                  Show recent notes by updated/created time
  restore --list            List notes in .trash
  restore <note>            Restore a note from .trash
  doctor                    Check vault health, Git, AI, and broken links

Update:
  Homebrew   brew upgrade steevin/tap/neuron
  curl       curl -sSfL https://github.com/steevin/neuron-cli/releases/latest/download/neuron_$(uname -s)_$(uname -m).tar.gz | tar -xz -C /usr/local/bin neuron

Run 'neuron help <command>' for detailed usage of any subcommand.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// saltamos el banner en comandos como version o mcp para no ensuciar la salida.
		if cmd.Name() == "version" || cmd.Name() == "mcp" || cmd.Name() == "anlly" {
			return
		}
		nameStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#58a6ff"))
		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e7681"))
		fmt.Printf("%s %s\n\n",
			nameStyle.Render("neuron"),
			dimStyle.Render("v"+version),
		)
	},
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("config: empty configuration")
	}
	return cfg, nil
}

func openExternal(target string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", target)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		c = exec.Command("xdg-open", target)
	}
	return c.Start()
}

var attachCmd = &cobra.Command{
	Use:   "attach [note ID or title] [path or URL]",
	Short: "Attach an image or file to a note",
	Long: `Attach a local file or HTTP(S) URL to a note.

Assets are stored in the vault's Obsidian attachment folder when configured,
otherwise in assets/. Remote downloads are capped at 50 MiB.`,
	Example: `  neuron attach "My Note" /path/to/image.png
  neuron attach "My Note" https://example.com/image.jpg`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}

		noteID := args[0]
		pathOrURL := args[1]

		note, err := store.Get(noteID)
		if err != nil {
			return fmt.Errorf("finding note %q: %w", noteID, err)
		}

		if err := store.AttachAsset(note.ID, pathOrURL); err != nil {
			return fmt.Errorf("attaching asset: %w", err)
		}

		fmt.Printf("✅ Attached asset to note %q\n", note.Title)
		return nil
	},
}

var linksCmd = &cobra.Command{
	Use:     "links [note ID or title]",
	Short:   "Extract and open links or images from a note",
	Example: `  neuron links "My Note"`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}

		note, err := store.Get(args[0])
		if err != nil {
			return fmt.Errorf("finding note: %w", err)
		}

		re := regexp.MustCompile(`(https?://[^\s)\]]+|file://[^\s)\]]+)|\[(.*?)\]\(([^)]+)\)`)
		matches := re.FindAllStringSubmatch(note.RawContent, -1)

		if len(matches) == 0 {
			fmt.Println("No links or images found in note.")
			return nil
		}

		var options []huh.Option[string]
		for _, m := range matches {
			if m[1] != "" {
				display := m[1]
				if len(display) > 50 {
					display = display[:47] + "..."
				}
				options = append(options, huh.NewOption(display, m[1]))
			} else if m[3] != "" {
				title := m[2]
				if title == "" {
					title = filepath.Base(m[3])
				}
				options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", title, m[3]), m[3]))
			}
		}

		var selectedLink string
		err = huh.NewSelect[string]().
			Title(fmt.Sprintf("Links in %q", note.Title)).
			Options(options...).
			Value(&selectedLink).
			Run()
		if err != nil {
			return err
		}

		target := selectedLink
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "file://") {
			if strings.HasPrefix(target, "/") {
				target = filepath.Join(store.VaultPath, strings.TrimPrefix(target, "/"))
			} else {
				noteDir := filepath.Dir(note.Path)
				candidate1 := filepath.Join(noteDir, target)
				if _, err := os.Stat(candidate1); err == nil {
					target = candidate1
				} else {
					candidate2 := filepath.Join(store.VaultPath, target)
					if _, err := os.Stat(candidate2); err == nil {
						target = candidate2
					} else {
						// Fallback to what it was
						target = candidate1
					}
				}
			}
		}

		fmt.Printf("Opening %s...\n", target)
		return openExternal(target)
	},
}

var addCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Create a new note",
	Long:  "Create a new Markdown note in your vault, optionally from clipboard content or a template.",
	Example: `  neuron add "Meeting Notes"
  neuron add "Project Plan" --folder "1. Projects"
  neuron add "Idea" --tag idea --tag work
  
  # Capturar código desde un archivo o consola:
  neuron add "Config" --file nginx.conf --code
  cat script.py | neuron add "Script" --code python
  neuron add "Snippet" --from-clipboard --code go`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var title string
		var folder string
		folderFlag, _ := cmd.Flags().GetString("folder")

		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}

		if len(args) == 0 {
			paraFolders := store.DetectPARAFolders()
			options := []huh.Option[string]{
				huh.NewOption("Root Vault (No folder)", ""),
			}
			for _, pf := range paraFolders {
				options = append(options, huh.NewOption("📁 "+pf, pf))
			}

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Note Title").
						Description("Enter the title for the new note").
						Value(&title),
					huh.NewSelect[string]().
						Title("Select Folder (PARA)").
						Options(options...).
						Value(&folder),
				),
			).Run()
			if err != nil {
				return err
			}
			if title == "" {
				return fmt.Errorf("title is required")
			}
		} else {
			title = strings.Join(args, " ")
			folder = folderFlag
		}

		tags, _ := cmd.Flags().GetStringSlice("tag")
		fromClipboard, _ := cmd.Flags().GetBool("from-clipboard")
		fileFlag, _ := cmd.Flags().GetString("file")
		codeFlag, _ := cmd.Flags().GetString("code")
		codeFlagSet := cmd.Flags().Changed("code")
		noEdit, _ := cmd.Flags().GetBool("no-edit")

		templateName, _ := cmd.Flags().GetString("template")
		content := ""

		stat, _ := os.Stdin.Stat()
		isPiped := (stat.Mode() & os.ModeCharDevice) == 0

		if isPiped {
			bytes, err := io.ReadAll(os.Stdin)
			if err == nil {
				content = string(bytes)
				noEdit = true // imply no-edit if reading from stdin
			}
		} else if fileFlag != "" {
			bytes, err := os.ReadFile(fileFlag)
			if err == nil {
				content = string(bytes)
				if !codeFlagSet {
					ext := filepath.Ext(fileFlag)
					if ext != "" {
						codeFlag = strings.TrimPrefix(ext, ".")
						codeFlagSet = true
					}
				}
			} else {
				return fmt.Errorf("reading file: %v", err)
			}
		} else if fromClipboard {
			text, err := clipboard.ReadAll()
			if err == nil {
				content = text
			}
		} else if templateName != "" {
			var renderErr error
			content, renderErr = store.RenderTemplate(templateName, title)
			if renderErr != nil {
				return fmt.Errorf("template error: %v", renderErr)
			}
		}

		if codeFlagSet && content != "" {
			lang := codeFlag
			if lang == " " || lang == "txt" {
				lang = ""
			}
			content = fmt.Sprintf("```%s\n%s\n```\n", lang, strings.TrimRight(content, "\n"))
		}

		note, err := store.Create(folder, title, tags, content)
		if err != nil {
			return err
		}
		fmt.Printf("Created %s\n", note.Path)

		if !noEdit {
			c, err := editorCommand(cfg, note.Path)
			if err != nil {
				return err
			}
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		}
		return nil
	},
}

// listCmd lista las notas del vault (soporta filtros opcionales).
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List notes in your vault",
	Long:  "List notes in your vault. Supports filtering by tag or full-text/semantic query.",
	Example: `  neuron list
  neuron list -q "machine learning"
  neuron list --tag meeting --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		query, _ := cmd.Flags().GetString("query")
		tags, _ := cmd.Flags().GetStringSlice("tag")

		if query != "" {
			if cfg.AI.Enabled {
				idx, err := search.NewSemanticIndex(cfg, store.EmbedDir())
				if err != nil {
					return fmt.Errorf("semantic search setup failed: %v", err)
				}
				noteList, err := store.List(notes.ListOptions{})
				if err != nil {
					return err
				}
				fmt.Println("Generating embeddings...")
				if err := idx.Rebuild(cmd.Context(), noteList); err != nil {
					return err
				}
				res, err := idx.Search(cmd.Context(), query, limit)
				if err != nil {
					return err
				}
				scoreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
				titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
				for _, r := range res {
					fmt.Printf("%s %s\n", scoreStyle.Render(fmt.Sprintf("%.2f", r.Score)), titleStyle.Render(r.Note.Title))
				}
				return nil
			} else {
				// búsqueda por palabras (BM25)
				noteList, err := store.List(notes.ListOptions{})
				if err != nil {
					return err
				}
				idx, err := search.RebuildWithCache(store.CacheDir(), noteList)
				if err != nil {
					return fmt.Errorf("search setup failed: %v", err)
				}
				res := idx.Search(query, limit)
				titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
				for _, r := range res {
					fmt.Printf("%s\n", titleStyle.Render(r.Note.Title))
				}
				return nil
			}
		}

		noteList, err := store.List(notes.ListOptions{Limit: limit, Tags: tags})
		if err != nil {
			return err
		}
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e6edf3"))
		tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681"))
		for _, n := range noteList {
			tagsStr := ""
			if len(n.Tags) > 0 {
				tagsStr = tagStyle.Render(" #" + strings.Join(n.Tags, " #"))
			}
			fmt.Printf("%s%s\n", titleStyle.Render(n.Title), tagsStr)
		}
		return nil
	},
}

// editorSpec decide qué editor usar en este orden:
//  1. cfg.Editor (guardado en config.toml)
//  2. $EDITOR
//  3. $VISUAL
//  4. "vi" como último recurso
func editorSpec(cfg *config.Config) string {
	editor := "vi"
	if cfg.Editor != "" {
		editor = cfg.Editor
	} else if e := os.Getenv("EDITOR"); e != "" {
		editor = e
	} else if e := os.Getenv("VISUAL"); e != "" {
		editor = e
	}

	return editor
}

func editorCommand(cfg *config.Config, path string) (*exec.Cmd, error) {
	parts, err := splitCommand(editorSpec(cfg))
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...), nil
}

func splitCommand(input string) ([]string, error) {
	var parts []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, r := range strings.TrimSpace(input) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ';' || r == '&' || r == '|' || r == '<' || r == '>' || r == '`' {
			return nil, fmt.Errorf("editor command contains unsupported shell metacharacter %q", r)
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("editor command has unterminated quote")
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts, nil
}

// editCmd abre una nota en el editor que hayas configurado.
var editCmd = &cobra.Command{
	Use:   "edit [id-or-title]",
	Short: "Open a note in your editor",
	Long:  "Locate a note by ID or fuzzy title match and open it in the configured editor.",
	Example: `  neuron edit "Meeting Notes"
  neuron edit d8c1b3f`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var idOrTitle string
		if len(args) == 0 {
			err := huh.NewInput().
				Title("Note ID or Title").
				Description("Enter the ID or title of the note to edit").
				Value(&idOrTitle).
				Run()
			if err != nil {
				return err
			}
			if idOrTitle == "" {
				return fmt.Errorf("id or title required")
			}
		} else {
			idOrTitle = strings.Join(args, " ")
		}
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		note, err := store.Get(idOrTitle)
		if err != nil {
			return err
		}
		c, err := editorCommand(cfg, note.Path)
		if err != nil {
			return err
		}
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

// rmCmd borra una nota del vault.
var rmCmd = &cobra.Command{
	Use:   "rm [id-or-title]",
	Short: "Delete a note",
	Long:  "Permanently delete a note from the vault. Requires --force/-f to skip confirmation.",
	Example: `  neuron rm "Old Note"
  neuron rm "Old Note" --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var idOrTitle string
		if len(args) == 0 {
			err := huh.NewInput().
				Title("Note ID or Title").
				Description("Enter the ID or title of the note to delete").
				Value(&idOrTitle).
				Run()
			if err != nil {
				return err
			}
			if idOrTitle == "" {
				return fmt.Errorf("id or title required")
			}
		} else {
			idOrTitle = strings.Join(args, " ")
		}
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		note, err := store.Get(idOrTitle)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Print("Are you sure? (y/N) ")
			var answer string
			_, _ = fmt.Scanln(&answer)
			if strings.ToLower(answer) != "y" {
				return fmt.Errorf("aborted")
			}
		}
		if err := store.Delete(note.ID); err != nil {
			return err
		}
		fmt.Println("Note deleted")
		return nil
	},
}

// openCmd abre la carpeta del vault en el explorador de archivos del sistema.
var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open vault folder",
	Long:  "Open the vault directory in the system file browser.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return openExternal(cfg.VaultPath)
	},
}

// statsCmd muestra estadísticas básicas del vault.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show vault statistics",
	Long:  "Display note count, tag count, word count, and other vault-level metrics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		count, err := store.Count()
		if err != nil {
			return err
		}
		tags, err := store.Tags()
		if err != nil {
			return err
		}
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58a6ff"))
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3fb950"))
		fmt.Printf("%s %s\n", titleStyle.Render("Notes:"), valueStyle.Render(fmt.Sprintf("%d", count)))
		fmt.Printf("%s %s\n", titleStyle.Render("Tags: "), valueStyle.Render(fmt.Sprintf("%d", len(tags))))
		return nil
	},
}

// todayCmd abre (o crea) la nota diaria de hoy.
var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Open or create today's daily note",
	Long:  "Open the daily note for today (YYYY-MM-DD.md). Creates it if it doesn't exist.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		title := "Daily " + time.Now().Format("2006-01-02")
		note, err := store.Get(title)
		if err != nil {
			content, renderErr := store.RenderTemplate("daily", title)
			if renderErr != nil || content == "" {
				content = "## 🎯 Today's goals\n- [ ] \n\n## 📝 Notes\n\n## 🔗 Links\n"
			}
			note, err = store.Create("", title, []string{"daily"}, content)
			if err != nil {
				return err
			}
		}
		c, err := editorCommand(cfg, note.Path)
		if err != nil {
			return err
		}
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

// moveCmd mueve una nota a otra carpeta del vault.
var moveCmd = &cobra.Command{
	Use:   "move [id-or-title] [folder]",
	Short: "Move a note to a folder",
	Long:  "Move a note to another folder (e.g. projects, areas, resources, archive, or root).",
	RunE: func(cmd *cobra.Command, args []string) error {
		var idOrTitle string
		var targetFolder string

		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}

		if len(args) == 0 {
			err := huh.NewInput().
				Title("Note ID or Title").
				Description("Enter the ID or title of the note to move").
				Value(&idOrTitle).
				Run()
			if err != nil {
				return err
			}
			if idOrTitle == "" {
				return fmt.Errorf("id or title required")
			}
		} else {
			idOrTitle = args[0]
		}

		if len(args) < 2 {
			paraFolders := store.DetectPARAFolders()
			options := []huh.Option[string]{
				huh.NewOption("Root Vault (No folder)", ""),
			}
			for _, pf := range paraFolders {
				options = append(options, huh.NewOption("📁 "+pf, pf))
			}

			err := huh.NewSelect[string]().
				Title("Select Destination Folder").
				Options(options...).
				Value(&targetFolder).
				Run()
			if err != nil {
				return err
			}
		} else {
			argFolder := strings.ToLower(args[1])
			paraFolders := store.DetectPARAFolders()
			matched := false
			for _, pf := range paraFolders {
				if strings.Contains(strings.ToLower(pf), argFolder) {
					targetFolder = pf
					matched = true
					break
				}
			}
			if !matched {
				if argFolder == "root" || argFolder == "/" || argFolder == "" {
					targetFolder = ""
				} else {
					targetFolder = args[1]
				}
			}
		}

		if err := store.Move(idOrTitle, targetFolder); err != nil {
			return err
		}
		if targetFolder == "" {
			fmt.Printf("Moved note to root vault\n")
		} else {
			fmt.Printf("Moved note to %s\n", targetFolder)
		}
		return nil
	},
}

// syncCmd sincroniza el vault con un remoto de Git.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync vault with Git remote",
	Long:  "Commit any local changes and push to the configured Git remote. Use --pull to fetch first, or --remote to override the configured remote for this run.",
	Example: `  neuron sync
  neuron sync --pull
  neuron sync --remote backup
  neuron sync --remote https://github.com/me/notes.git`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		remote, _ := cmd.Flags().GetString("remote")
		if remote == "" {
			remote = cfg.GitRemote
		}
		syncer := gitsync.NewSyncer(cfg.VaultPath, remote)
		pull, _ := cmd.Flags().GetBool("pull")
		if pull {
			if err := syncer.Pull(); err != nil {
				fmt.Printf("Pull failed: %v\n", err)
			}
		}
		res, err := syncer.Sync()
		if err != nil {
			return err
		}
		fmt.Println(res.Message)
		if res.Pushed {
			fmt.Println("Changes pushed to remote.")
		}
		return nil
	},
}

var backlinksCmd = &cobra.Command{
	Use:   "backlinks [id-or-title]",
	Short: "Show notes that link to a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		note, err := store.Get(strings.Join(args, " "))
		if err != nil {
			return err
		}
		noteList, err := store.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		graph := notes.BuildGraph(noteList)
		node := graph.Nodes[note.Title]
		if node == nil || len(node.Backlinks) == 0 {
			fmt.Printf("No backlinks found for %q\n", note.Title)
			return nil
		}

		byTitle := make(map[string]*notes.Note, len(noteList))
		for _, n := range noteList {
			byTitle[n.Title] = n
		}
		fmt.Printf("Backlinks for %q:\n", note.Title)
		for _, title := range node.Backlinks {
			if linked := byTitle[title]; linked != nil {
				fmt.Printf("- %s (%s)\n", title, linked.RelPath)
			} else {
				fmt.Printf("- %s\n", title)
			}
		}
		return nil
	},
}

var orphanCmd = &cobra.Command{
	Use:   "orphan",
	Short: "List notes with no links or backlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		noteList, err := store.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		graph := notes.BuildGraph(noteList)
		orphans := graph.Orphans()
		if limit > 0 && len(orphans) > limit {
			orphans = orphans[:limit]
		}
		if len(orphans) == 0 {
			fmt.Println("No orphan notes found.")
			return nil
		}
		for _, node := range orphans {
			fmt.Printf("- %s (%s)\n", node.Title, node.Path)
		}
		return nil
	},
}

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Show notes ordered by updated or created time",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		created, _ := cmd.Flags().GetBool("created")
		sortBy := "updated"
		if created {
			sortBy = "created"
		}
		noteList, err := store.List(notes.ListOptions{Limit: limit, SortBy: sortBy})
		if err != nil {
			return err
		}
		for _, n := range noteList {
			ts := n.Updated
			label := "updated"
			if created {
				ts = n.Created
				label = "created"
			}
			fmt.Printf("%s %-7s %s (%s)\n", ts.Format("2006-01-02 15:04"), label, n.Title, n.RelPath)
		}
		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore [id-or-title]",
	Short: "List or restore notes from .trash",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		listOnly, _ := cmd.Flags().GetBool("list")
		folder, _ := cmd.Flags().GetString("folder")
		if listOnly {
			trashed, err := store.ListTrash()
			if err != nil {
				return err
			}
			if len(trashed) == 0 {
				fmt.Println("Trash is empty.")
				return nil
			}
			for _, n := range trashed {
				fmt.Printf("- %s (%s)\n", n.Title, n.RelPath)
			}
			return nil
		}
		if len(args) == 0 {
			return fmt.Errorf("id or title required, or use --list")
		}
		restored, err := store.Restore(strings.Join(args, " "), folder)
		if err != nil {
			return err
		}
		fmt.Printf("Restored %s\n", restored.RelPath)
		return nil
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check vault health and local integrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		fmt.Printf("Config: ok\n")
		fmt.Printf("Vault: %s\n", cfg.VaultPath)

		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		if store.Vault != nil && store.Vault.IsObsidian {
			fmt.Println("Obsidian: detected")
		} else {
			fmt.Println("Obsidian: not detected")
		}

		noteList, err := store.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		tags, err := store.Tags()
		if err != nil {
			return err
		}
		graph := notes.BuildGraph(noteList)
		broken := brokenLinks(noteList, graph)
		orphans := graph.Orphans()
		trashed, err := store.ListTrash()
		if err != nil {
			return err
		}

		fmt.Printf("Notes: %d\n", len(noteList))
		fmt.Printf("Tags: %d\n", len(tags))
		fmt.Printf("Orphans: %d\n", len(orphans))
		fmt.Printf("Broken links: %d\n", len(broken))
		fmt.Printf("Trash: %d\n", len(trashed))
		printGitDoctor(cfg)
		printAIDoctor(cfg)

		if len(broken) > 0 {
			fmt.Println("Broken link samples:")
			limit := 5
			if len(broken) < limit {
				limit = len(broken)
			}
			for _, item := range broken[:limit] {
				fmt.Printf("- %s -> [[%s]]\n", item.Source, item.Target)
			}
		}
		return nil
	},
}

type brokenLink struct {
	Source string
	Target string
}

func brokenLinks(noteList []*notes.Note, graph *notes.Graph) []brokenLink {
	var broken []brokenLink
	for _, n := range noteList {
		for _, target := range n.Links {
			if _, ok := graph.Nodes[target]; !ok {
				broken = append(broken, brokenLink{Source: n.Title, Target: target})
			}
		}
	}
	sort.Slice(broken, func(i, j int) bool {
		if strings.ToLower(broken[i].Source) == strings.ToLower(broken[j].Source) {
			return strings.ToLower(broken[i].Target) < strings.ToLower(broken[j].Target)
		}
		return strings.ToLower(broken[i].Source) < strings.ToLower(broken[j].Source)
	})
	return broken
}

func printGitDoctor(cfg *config.Config) {
	if _, err := os.Stat(filepath.Join(cfg.VaultPath, ".git")); err != nil {
		fmt.Println("Git: not initialized")
		return
	}
	syncer := gitsync.NewSyncer(cfg.VaultPath, cfg.GitRemote)
	changed, err := syncer.Status()
	if err != nil {
		fmt.Printf("Git: error: %v\n", err)
		return
	}
	if cfg.GitRemote == "" {
		fmt.Printf("Git: initialized, %d changed file(s), no remote configured\n", len(changed))
		return
	}
	fmt.Printf("Git: initialized, %d changed file(s), remote %q\n", len(changed), cfg.GitRemote)
}

func printAIDoctor(cfg *config.Config) {
	if !cfg.AI.Enabled {
		fmt.Println("AI: disabled")
		return
	}
	if cfg.AI.Provider != "ollama" {
		fmt.Printf("AI: enabled provider %q\n", cfg.AI.Provider)
		return
	}
	client := &http.Client{Timeout: 2 * time.Second}
	url := strings.TrimRight(cfg.AI.OllamaURL, "/") + "/api/tags"
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("AI: ollama unreachable: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("AI: ollama reachable (%s)\n", cfg.AI.Model)
		return
	}
	fmt.Printf("AI: ollama returned %s\n", resp.Status)
}

// mcpCmd inicia el servidor MCP.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server for AI agent integration",
	Long: `Start an MCP (Model Context Protocol) server that exposes vault tools
to AI agents such as Claude Desktop, Cursor, or any MCP-compatible client.

Use --read-only to expose search/list/read tools while blocking note writes.
Use --audit-log to append JSONL records for MCP write attempts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		vaultOverride, _ := cmd.Flags().GetString("vault")
		if vaultOverride != "" {
			cfg.VaultPath = vaultOverride
		}
		readOnly, _ := cmd.Flags().GetBool("read-only")
		auditLog, _ := cmd.Flags().GetString("audit-log")

		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to load vault: %v", err)
		}

		// construimos el índice con persistencia
		noteList, err := store.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		idx, err := search.RebuildWithCache(store.CacheDir(), noteList)
		if err != nil {
			return fmt.Errorf("failed to build search index: %v", err)
		}

		srv, err := mcp.NewServerWithOptions(cfg, store, idx, mcp.ServerOptions{
			ReadOnly: readOnly,
			AuditLog: auditLog,
		})
		if err != nil {
			return err
		}

		return srv.Start()
	},
}

// configCmd te permite ver y cambiar la configuración de NeuronCLI sin
// tener que editar el archivo TOML a mano.
//
// Uso:
//
//	neuron config get <key>          — muestra el valor actual
//	neuron config set <key> <value>  — actualiza un valor
//
// Llaves soportadas: vault_path, editor, theme, git_remote
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read or update NeuronCLI settings",
	Long: `Read or update NeuronCLI settings stored in ~/.config/neuron/config.toml.

Supported keys:
  vault_path   Absolute path to your Markdown vault
  editor       Command used to open notes (e.g. code -w, nvim, nano)
  theme        TUI colour scheme: dark or light
  git_remote   Git remote name or URL used by 'neuron sync'`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print the current value of a setting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		key := args[0]
		switch key {
		case "vault_path":
			fmt.Println(cfg.VaultPath)
		case "editor":
			v := cfg.Editor
			if v == "" {
				v = "(not set — using $EDITOR / $VISUAL / vi)"
			}
			fmt.Println(v)
		case "theme":
			fmt.Println(cfg.Theme)
		case "git_remote":
			fmt.Println(cfg.GitRemote)
		default:
			return fmt.Errorf("unknown key %q — valid keys: vault_path, editor, theme, git_remote", key)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a setting and save it to disk",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		key, value := args[0], args[1]
		switch key {
		case "vault_path":
			cfg.VaultPath = value
		case "editor":
			cfg.Editor = value
		case "theme":
			if value != "dark" && value != "light" {
				return fmt.Errorf("theme must be \"dark\" or \"light\", got %q", value)
			}
			cfg.Theme = value
		case "git_remote":
			cfg.GitRemote = value
		default:
			return fmt.Errorf("unknown key %q — valid keys: vault_path, editor, theme, git_remote", key)
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("✓ %s = %s\n", key, value)
		return nil
	},
}

// tuiCmd lanza la interfaz gráfica en la terminal (TUI).
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive TUI",
	Long:  "Launch the full-screen keyboard-driven terminal UI for browsing and editing your vault.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return tui.Run(cfg, version)
	},
}

// versionCmd imprime la versión.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  "Print the neuron build version and exit.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("neuron version %s\n", version)
		fmt.Println("Copyright (C) 2025-2026 Daniel Steevin")
		fmt.Println("License GPLv3+: GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>.")
		fmt.Println("This is free software: you are free to change and redistribute it.")
		fmt.Println("There is NO WARRANTY, to the extent permitted by law.")
	},
}

// anllyCmd es un comando especial y oculto.
var anllyCmd = &cobra.Command{
	Use:    "anlly",
	Hidden: true,
	Short:  "Anlly",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("She was always my inspiration while creating this app.")
	},
}

func init() {
	// flags de addCmd
	addCmd.Flags().Bool("from-clipboard", false, "Populate note body from clipboard contents")
	addCmd.Flags().String("file", "", "Create note from file contents")
	addCmd.Flags().String("code", "", "Wrap content in a markdown code block (optional: specify language)")
	addCmd.Flags().Lookup("code").NoOptDefVal = "txt"
	addCmd.Flags().StringSlice("tag", nil, "Tags to apply to the new note (repeatable)")
	addCmd.Flags().String("template", "", "Name of the note template to use")
	addCmd.Flags().String("folder", "", "Folder in the vault to save the note in (e.g. '1. Projects')")
	addCmd.Flags().Bool("no-edit", false, "Create the note without opening the editor")

	// flags de listCmd
	listCmd.Flags().StringSlice("tag", nil, "Filter by tag (repeatable)")
	listCmd.Flags().StringP("query", "q", "", "Full-text or semantic search query")
	listCmd.Flags().Int("limit", 50, "Maximum number of notes to display")

	// flags de rmCmd
	rmCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// flags de syncCmd
	syncCmd.Flags().String("remote", "", "Override the configured Git remote")
	syncCmd.Flags().Bool("pull", false, "Pull from remote before pushing")

	// flags de orphanCmd
	orphanCmd.Flags().Int("limit", 50, "Maximum number of orphan notes to display")

	// flags de timelineCmd
	timelineCmd.Flags().Int("limit", 50, "Maximum number of notes to display")
	timelineCmd.Flags().Bool("created", false, "Sort by creation time instead of updated time")

	// flags de restoreCmd
	restoreCmd.Flags().Bool("list", false, "List notes currently in .trash")
	restoreCmd.Flags().String("folder", "", "Folder to restore the note into")

	// flags de mcpCmd
	mcpCmd.Flags().String("vault", "", "Override vault path for this session")
	mcpCmd.Flags().Bool("read-only", false, "Disable MCP tools that write to the vault")
	mcpCmd.Flags().String("audit-log", "", "Append MCP write attempts to this JSONL file")
}

func main() {
	// enganchamos los subcomandos de config.
	configCmd.AddCommand(configGetCmd, configSetCmd)

	// registramos todos los comandos principales.
	rootCmd.AddCommand(
		addCmd,
		attachCmd,
		linksCmd,
		listCmd,
		editCmd,
		rmCmd,
		openCmd,
		statsCmd,
		todayCmd,
		moveCmd,
		syncCmd,
		backlinksCmd,
		orphanCmd,
		timelineCmd,
		restoreCmd,
		doctorCmd,
		mcpCmd,
		tuiCmd,
		versionCmd,
		configCmd,
		anllyCmd,
		initAppCmd,
	)

	// si corren neuron sin argumentos, abrimos la TUI por defecto.
	rootCmd.RunE = tuiCmd.RunE

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
