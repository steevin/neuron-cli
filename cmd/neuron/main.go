// NeuronCLI — a terminal-based personal knowledge manager with Obsidian-compatible
// Markdown vaults, local AI embeddings, and an MCP server for AI agent integration.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/mcp"
	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/search"
	gitsync "github.com/steevin/neuron-cli/internal/sync"
	"github.com/steevin/neuron-cli/internal/tui"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

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

Commands:
  add <title>              Create a new note (--tag, --template)
  edit <title|id>          Open a note in your configured editor
  today                    Open or create today's daily note
  list                     List notes (-q query, --tag, --limit)
  rm <title|id>            Delete a note (--force to skip confirmation)
  stats                    Show vault statistics (note count, tags, words)
  open                     Open the vault folder in Finder
  sync                     Sync vault with Git remote (--pull to fetch first)
  tui                      Open the interactive full-screen TUI
  mcp                      Start the MCP server for AI agent integration
  config get <key>         Print a setting (vault_path, editor, theme, git_remote)
  config set <key> <val>   Update a setting and save it to config.toml
  version                  Print the build version

Update:
  Homebrew   brew upgrade steevin/tap/neuron
  curl       curl -sSfL https://github.com/steevin/neuron-cli/releases/latest/download/neuron_$(uname -s)_$(uname -m).tar.gz | tar -xz -C /usr/local/bin neuron

Run 'neuron help <command>' for detailed usage of any subcommand.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip banner for the bare version command — it has its own output.
		// Also skip for mcp command, as it expects clean JSON-RPC over stdio.
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

var addCmd = &cobra.Command{
	Use:   "add [title]",
	Short: "Create a new note",
	Long:  "Create a new Markdown note in your vault, optionally from clipboard content or a template.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("title is required")
		}
		title := strings.Join(args, " ")
		cfg, _ := config.Load()
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		tags, _ := cmd.Flags().GetStringSlice("tag")

		templateName, _ := cmd.Flags().GetString("template")
		content := ""
		if templateName != "" {
			var renderErr error
			content, renderErr = store.RenderTemplate(templateName, title)
			if renderErr != nil {
				return fmt.Errorf("template error: %v", renderErr)
			}
		}

		note, err := store.Create(title, tags, content)
		if err != nil {
			return err
		}
		fmt.Printf("Created %s\n", note.Path)
		return nil
	},
}

// listCmd lists notes in the vault with optional filtering.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List notes in your vault",
	Long:  "List notes in your vault. Supports filtering by tag or full-text/semantic query.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		query, _ := cmd.Flags().GetString("query")
		tags, _ := cmd.Flags().GetStringSlice("tag")

		if query != "" {
			if cfg.AI.Enabled {
				// Use semantic search
				idx, err := search.NewSemanticIndex(cfg)
				if err != nil {
					return fmt.Errorf("semantic search setup failed: %v", err)
				}
				noteList, _ := store.List(notes.ListOptions{})
				// Ideally Rebuild should be cached, but for CLI we build on-the-fly or load from db if chromem persists.
				// chromem-go NewDB() is in-memory by default, but we can just rebuild it fast enough for a small vault.
				fmt.Println("Generating embeddings...")
				_ = idx.Rebuild(cmd.Context(), noteList)
				res, err := idx.Search(cmd.Context(), query, limit)
				if err != nil {
					return err
				}
				for _, r := range res {
					fmt.Printf("%.2f - %s\n", r.Score, r.Note.Title)
				}
				return nil
			} else {
				// Use BM25 search
				idx := search.NewIndex()
				noteList, _ := store.List(notes.ListOptions{})
				idx.Rebuild(noteList)
				res := idx.Search(query, limit)
				for _, r := range res {
					fmt.Printf("%s\n", r.Note.Title)
				}
				return nil
			}
		}

		noteList, err := store.List(notes.ListOptions{Limit: limit, Tags: tags})
		if err != nil {
			return err
		}
		for _, n := range noteList {
			fmt.Printf("%s\n", n.Title)
		}
		return nil
	},
}

// resolveEditor returns the editor to use, consulting (in order):
//  1. cfg.Editor (set via `neuron config set editor <cmd>` or the config file)
//  2. The $EDITOR environment variable
//  3. The $VISUAL environment variable
//  4. "vi" as a last-resort fallback
func resolveEditor(cfg *config.Config) string {
	editor := "vi"
	if cfg.Editor != "" {
		editor = cfg.Editor
	} else if e := os.Getenv("EDITOR"); e != "" {
		editor = e
	} else if e := os.Getenv("VISUAL"); e != "" {
		editor = e
	}

	// Sanitize the input to prevent arbitrary shell command injection
	// if the user provided arguments like `vi; rm -rf /`. This strictly uses
	// the first field as the command. If users need flags, they should use a
	// wrapper script.
	editorParts := strings.Fields(editor)
	if len(editorParts) > 0 {
		return editorParts[0]
	}
	return "vi"
}

// editCmd opens an existing note in the configured editor.
var editCmd = &cobra.Command{
	Use:   "edit [id-or-title]",
	Short: "Open a note in your editor",
	Long:  "Locate a note by ID or fuzzy title match and open it in the configured editor.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("id or title required")
		}
		cfg, _ := config.Load()
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		note, err := store.Get(strings.Join(args, " "))
		if err != nil {
			return err
		}
		c := exec.Command(resolveEditor(cfg), note.Path)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

// rmCmd deletes a note from the vault.
var rmCmd = &cobra.Command{
	Use:   "rm [id-or-title]",
	Short: "Delete a note",
	Long:  "Permanently delete a note from the vault. Requires --force/-f to skip confirmation.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("id or title required")
		}
		cfg, _ := config.Load()
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		note, err := store.Get(strings.Join(args, " "))
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

// openCmd reveals the vault folder in Finder (macOS).
var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open vault folder in Finder",
	Long:  "Open the vault directory in the macOS Finder (uses 'open' under the hood).",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		c := exec.Command("open", "--", cfg.VaultPath)
		return c.Run()
	},
}

// statsCmd prints aggregate statistics about the vault.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show vault statistics",
	Long:  "Display note count, tag count, word count, and other vault-level metrics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return err
		}
		count, _ := store.Count()
		tags, _ := store.Tags()
		fmt.Printf("Notes: %d\n", count)
		fmt.Printf("Tags:  %d\n", len(tags))
		return nil
	},
}

// todayCmd opens or creates the daily note for the current date.
var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Open or create today's daily note",
	Long:  "Open the daily note for today (YYYY-MM-DD.md). Creates it if it doesn't exist.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
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
			note, err = store.Create(title, []string{"daily"}, content)
			if err != nil {
				return err
			}
		}
		c := exec.Command(resolveEditor(cfg), note.Path)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}

// syncCmd synchronises the vault with a Git remote.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync vault with Git remote",
	Long:  "Commit any local changes and push to the configured Git remote. Use --pull to fetch first.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		syncer := gitsync.NewSyncer(cfg.VaultPath, cfg.GitRemote)
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

// mcpCmd starts the Model Context Protocol server.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server for AI agent integration",
	Long: `Start an MCP (Model Context Protocol) server that exposes vault tools
to AI agents such as Claude Desktop, Cursor, or any MCP-compatible client.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		vaultOverride, _ := cmd.Flags().GetString("vault")
		if vaultOverride != "" {
			cfg.VaultPath = vaultOverride
		}

		store, err := notes.NewStore(cfg.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to load vault: %v", err)
		}

		// Build index
		idx := search.NewIndex()
		noteList, _ := store.List(notes.ListOptions{})
		idx.Rebuild(noteList)

		srv, err := mcp.NewServer(cfg, store, idx)
		if err != nil {
			return err
		}

		return srv.Start()
	},
}

// configCmd allows the user to inspect and change NeuronCLI settings without
// editing the TOML file by hand.
//
// Usage:
//
//	neuron config get <key>          — print the current value of a setting
//	neuron config set <key> <value>  — update a setting and save it
//
// Supported keys: vault_path, editor, theme, git_remote
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read or update NeuronCLI settings",
	Long: `Read or update NeuronCLI settings stored in ~/.config/neuron/config.toml.

Supported keys:
  vault_path   Absolute path to your Markdown vault
  editor       Command used to open notes (e.g. code, nvim, nano)
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

// tuiCmd launches the interactive Bubble Tea TUI.
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive TUI",
	Long:  "Launch the full-screen keyboard-driven terminal UI for browsing and editing your vault.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return tui.Run(cfg)
	},
}

// versionCmd prints the build version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  "Print the neuron build version and exit.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("neuron version %s\n", version)
	},
}

// anllyCmd is a special hidden command.
var anllyCmd = &cobra.Command{
	Use:    "anlly",
	Hidden: true,
	Short:  "Anlly",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("She was always my inspiration while creating this app.")
	},
}

func init() {
	// addCmd flags
	addCmd.Flags().Bool("from-clipboard", false, "Populate note body from clipboard contents")
	addCmd.Flags().StringSlice("tag", nil, "Tags to apply to the new note (repeatable)")
	addCmd.Flags().String("template", "", "Name of the note template to use")

	// listCmd flags
	listCmd.Flags().StringSlice("tag", nil, "Filter by tag (repeatable)")
	listCmd.Flags().StringP("query", "q", "", "Full-text or semantic search query")
	listCmd.Flags().Int("limit", 50, "Maximum number of notes to display")

	// rmCmd flags
	rmCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// syncCmd flags
	syncCmd.Flags().String("remote", "", "Override the configured Git remote")
	syncCmd.Flags().Bool("pull", false, "Pull from remote before pushing")

	// mcpCmd flags
	mcpCmd.Flags().String("vault", "", "Override vault path for this session")
}

func main() {
	// Wire config sub-subcommands.
	configCmd.AddCommand(configGetCmd, configSetCmd)

	// Register all subcommands.
	rootCmd.AddCommand(
		addCmd,
		listCmd,
		editCmd,
		rmCmd,
		openCmd,
		statsCmd,
		todayCmd,
		syncCmd,
		mcpCmd,
		tuiCmd,
		versionCmd,
		configCmd,
		anllyCmd,
	)

	// When neuron is invoked with no subcommand, fall through to the TUI.
	rootCmd.RunE = tuiCmd.RunE

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
