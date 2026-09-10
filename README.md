<div align="center">

<img src="docs/assets/logo.png" alt="Neuron CLI" width="140" />

# Neuron CLI

**Capture ideas. Find your next task. Pick up where you left off.**

A local Markdown workspace for your terminal — compatible with Obsidian.

**Local first** · **Keyboard driven** · **Markdown files** · **Optional AI & MCP**

[Quick start](#quick-start) · [Web workspace](#local-web-workspace) · [Workflows](#daily-workflows) · [Commands](#command-reference) · [Español](README.es.md)

</div>

---

| Capture | Act | Reconnect |
| :--- | :--- | :--- |
| Save an idea to **Inbox** without opening an editor. | See Markdown **tasks** and your daily overview. | Open the **project context** linked to your repository. |
| `neuron capture "An idea"` | `neuron dashboard` | `neuron project` |

Your vault stays a directory of Markdown files. Edit the same notes with Neuron,
Obsidian or your favorite editor. Core note workflows run locally; AI, remote
attachments and Git remotes are optional integrations.

> **Development checkout:** the productivity commands below are implemented in this
> repository. A published package may lag behind. Build this checkout to try them:
> `go build -o bin/neuron ./cmd/neuron`, then use `./bin/neuron` instead of `neuron`.

## Quick start

```bash
brew install steevin/tap/neuron
neuron init
neuron
```

Choose an existing vault or create one during setup. In the terminal interface
(TUI), press **`?`** for shortcuts, **`/`** for the command palette and **`e`** to edit.

<details>
<summary><strong>Other installation options and updates</strong></summary>

**From source** — requires Go 1.26.3 or newer, as declared in `go.mod`:

```bash
git clone https://github.com/steevin/neuron-cli.git
cd neuron-cli
go build -o bin/neuron ./cmd/neuron
./bin/neuron init
```

**With Go** — installs the published command into your Go binary directory:

```bash
go install github.com/steevin/neuron-cli/cmd/neuron@latest
```

**Prebuilt binaries:** select your OS and architecture on the
[releases page](https://github.com/steevin/neuron-cli/releases).
Release archives follow `neuron_<version>_<os>_<arch>.tar.gz`
(`.zip` on Windows); checksums are included with releases.

**Update:** use `brew upgrade steevin/tap/neuron`, repeat the Go install command,
or download a newer release. Local source builds must be rebuilt after updating.

</details>

## Daily workflows

```text
Capture → Review Inbox → Work through tasks → Revisit project context
                          neuron dashboard
```

### 1 · Capture now, organize later

```bash
neuron capture "Investigate API timeout"
printf 'API follow-up\n- [ ] Reproduce timeout\n' | neuron capture
neuron inbox
neuron inbox file "Inbox/api-follow-up.md" "1. Projects"
```

Capture writes to `Inbox/` without prompts or an editor. The first line supplies
the title and the complete text stays in the body. The command prints the saved
path; duplicate titles create separate notes. Use the path from `neuron inbox`
when filing a note into a folder.

### 2 · Work with tasks where they live

```bash
neuron tasks
neuron tasks --all --folder "1. Projects"
```

Tasks come from Markdown checkboxes such as `- [ ] Reproduce timeout`.
Results show a vault-relative path and the actual file line number.

```bash
# Replace 10 with the line printed by neuron tasks.
neuron tasks done "1. Projects/api-follow-up.md" 10
neuron tasks reopen "1. Projects/api-follow-up.md" 10
neuron tasks open "1. Projects/api-follow-up.md" 10
```

Completion changes only the checkbox. Empty tasks, frontmatter and fenced code
examples are excluded. `tasks open` opens the whole note in your configured
editor. After editing a note, list tasks again to get current line numbers.

### 3 · Start with a daily overview

```bash
neuron dashboard --limit 5
neuron today
```

The dashboard prints **pending tasks · Inbox · linked projects · recent notes**,
with a limit per section. It is a read-only overview of the whole vault; pending
tasks are not filtered by due date. `today` opens or creates `Daily YYYY-MM-DD`
in your editor, using the daily template when available.

### 4 · Keep context with your repository

Run inside a Git repository:

```bash
neuron project init --name "API redesign"
neuron project
neuron project list
```

The context note includes **Objective**, **Next steps**, **Decisions** and
**Related notes**. `neuron project` also works from repository subdirectories.

```bash
neuron project link "1. Projects/api-follow-up.md"
neuron project unlink
```

Linking replaces the repository's previous association. Unlinking removes it
from the dashboard without deleting the note. Linked projects count as active;
repository paths are machine-specific.

### 5 · Save the searches you repeat

```bash
neuron search 'timeout tag:work folder:"1. Projects"'
neuron search save work 'tag:work folder:"1. Projects"'
neuron search list
neuron search run work
neuron list --saved work --limit 0
neuron list -q timeout --tag work --folder "1. Projects"
```

Filters combine with **AND**; `folder:` includes subfolders. Quote values with
spaces as shown above. Saving an existing name replaces its query.
`neuron search remove work` deletes the query, leaving all notes intact.

| Search mode | Behavior |
| :--- | :--- |
| `neuron search '<query>'` | Local BM25 with optional `tag:` and `folder:` filters |
| `neuron list --saved work` | Saved query; extra filters can narrow it |
| `neuron list -q '<text>'` without filters | BM25 by default; semantic search when AI is enabled |
| `neuron search run work` | Up to 50 results; use `list --saved work --limit 0` for all |

These five workflows run in the **shell**, not as `/commands` in the TUI.

## Local web workspace

```bash
neuron web
```

Opens your configured vault in the browser automatically. The web workspace is
embedded in the binary: no Node.js, extra installation or hosting service required.

| View | What you can do |
| :--- | :--- |
| Library | Browse folders and tags; switch between lists and cards |
| Search | Find words in titles and the full content of notes |
| Editor | Create notes with folders and tags; edit their Markdown bodies |
| Read / Write / Both | Read, edit or work with a live preview |
| Connections | Follow `[[wikilinks]]`, Markdown note links and backlinks |
| Idea graph | Open notes from the map, zoom and pan |
| Today | Open an existing daily note or create one in the browser |
| Theme | Switch between light and dark |

**Editing modes:** **Escribir (Write)** edits Markdown source; **Lectura (Read)**
shows the formatted result without editing it; **Ambos (Both)** combines the
Markdown editor and preview. A Word/Notion-style visual editor (WYSIWYG) is not
yet implemented. Notes always remain `.md` files.

**Saving:** edits autosave after one second of inactivity. You can also press Save
or `⌘/Ctrl S`. Existing frontmatter is preserved. Every four seconds, the workspace
checks for external edits: clean notes refresh automatically; conflicting drafts
can be compared with the disk version, saved as a separate note, or discarded by
reloading from disk. Unsaved drafts live in the tab; keep it open until saved or
a conflict is resolved. The current web interface uses Spanish labels.

```bash
neuron web --port 7878
neuron web --vault /path/to/vault --no-open
```

An available port is chosen by default. `--no-open` prints the session link for
manual opening. The server listens only on `127.0.0.1`; keep its terminal open
and press `Ctrl+C` to stop. The session link grants access to this vault while the
server is running; use the new link after restarting the server.

**Shortcuts:** `⌘/Ctrl K` searches, `⌘/Ctrl N` creates a note and `⌘/Ctrl S` saves.

Preview supports tables, code blocks, checkboxes and local PNG, JPEG, GIF, WebP
and AVIF images. It does not execute note HTML or fetch remote images. The library
skips hidden files and symbolic links; notes are limited to 4 MiB and images to
50 MiB. The graph shows the 150 most recent notes; hover or focus a node in dense
maps to see its title. Ambiguous wikilinks require locating the note with search.
Daily notes created on the web use a basic body; `neuron today` retains the CLI
workflow with custom templates.

## Terminal interface

Browse notes with a preview, folder breadcrumbs, dark/light themes and a command
palette. Edit in your configured editor; paste clipboard text into the selected
note, or extract its code blocks without opening an editor.

| Key | Action |
| :--- | :--- |
| `j / k` or `↑ / ↓` | Navigate notes |
| `Tab / Shift+Tab` | Switch pane focus |
| `n` | Create a note; select a destination when PARA folders are detected |
| `e` | Edit the selected note |
| `ctrl+v` | Append clipboard content to the selected note |
| `c / y` | Select and copy a code block |
| `/` | Search notes or enter a palette command |
| `s` | Sync with Git |
| `ctrl+g` | Show graph counts; this is not an interactive graph |
| `?` | Show or hide shortcut help |
| `q` | Quit |

<details>
<summary><strong>Command palette and selection controls</strong></summary>

| Command | Action |
| :--- | :--- |
| `/add <title>` | Create a note; `/add folder/title` specifies its folder |
| `/today`, `/t` | Select or create the daily note |
| `/edit`, `/e` | Open the selected note in your editor |
| `/copy`, `/c` | Copy the whole note |
| `/attach <path_or_url>`, `/a` | Copy or download an attachment |
| `/links`, `/l` | Open a link; choose one when several are found |
| `/move <folder>`, `/m` | Move the selected note |
| `/rm` | Delete the selected note |
| `/sync`, `/s` | Sync with Git |
| `/stats` | Show note and tag counts |
| `/doctor`, `/health` | Show a quick health summary |
| `/backlinks` | Show a summary of incoming links |
| `/orphan` | Show isolated-note counts and examples |
| `/open`, `/o` | Open the vault in the file manager |
| `/theme dark\|light` | Change theme; `/theme` toggles it |
| `/help`, `/?` | Show shortcut help |
| `/quit`, `/q` | Quit |

In folder, link and code selection modes, use the arrow keys or `h/j/k/l`,
then `Enter` to confirm or `Esc` to cancel.

</details>

## Command reference

Use `neuron <command> --help` for flags and examples, including nested commands
such as `neuron tasks done --help`.

| Purpose | Commands |
| :--- | :--- |
| Setup and interface | `init`, `tui`, `web`, `config get`, `config set`, `completion`, `version` |
| Capture and daily work | `capture`, `inbox`, `inbox file`, `dashboard`, `today` |
| Markdown tasks | `tasks`, `tasks done`, `tasks reopen`, `tasks open` |
| Repository context | `project`, `project init`, `project link`, `project list`, `project unlink` |
| Search and favorites | `list`, `search`, `search save`, `search list`, `search run`, `search remove` |
| Notes and attachments | `add`, `edit`, `move`, `rm`, `attach`, `links`, `open` |
| Connections and maintenance | `backlinks`, `orphan`, `timeline`, `stats`, `doctor`, `restore`, `sync` |
| Agent access | `mcp` |

<details>
<summary><strong>Everyday note commands</strong></summary>

```bash
neuron add "Standup" --folder "1. Projects" --tag work
neuron add "Standup" --template standup
neuron add "Config" --file nginx.conf --code --no-edit
cat script.py | neuron add "Script" --code python
neuron edit "Standup"
neuron move "Standup" "1. Projects"
neuron attach "Standup" ./image.png
neuron links "Standup"
neuron backlinks "Standup"
neuron timeline --created
neuron restore --list
neuron restore "Old note" --folder "4. Archive"
neuron doctor
neuron sync --pull
neuron sync --remote backup
```

`neuron add` without a title prompts for a title and folder. With a title, pass
`--folder` explicitly; the CLI does not interpret `folder/title` as a destination.
Older note commands accept IDs or titles; productivity commands also accept
vault-relative paths. Attachments use Obsidian's configured folder or `assets/`,
with a 50 MiB limit on remote downloads.

</details>

## Configuration and vault

Settings live at `~/.config/neuron/config.toml`:

```bash
neuron config get vault_path
neuron config set editor "code -w"
neuron config set theme dark
neuron config set git_remote origin
```

Supported `config get/set` keys: `vault_path`, `editor`, `theme`, `git_remote`.
The editor supports arguments. PARA folders are optional:

```text
vault/
├── Inbox/                 Captured notes
├── 1. Projects/           Project notes
├── 2. Areas/
├── 3. Resources/
├── 4. Archive/
├── templates/             Reusable Markdown templates
└── .neuron/               Local metadata and indexes
    └── workspace.json     Saved searches and repository links
```

Notes are Markdown with optional YAML frontmatter, `[[wikilinks]]` and `#tags`.
Created and moved notes stay inside the vault. Deleted notes go to `.trash`.
Project and search metadata are separate from note content.

<details>
<summary><strong>Templates and optional semantic search</strong></summary>

Put `standup.md` or `daily.md` in `.obsidian/templates/` or `templates/` within
the vault (checked in that order). Templates support Go template variables:

```markdown
# {{.Title}}
Date: {{.Date}}

## Next steps
- [ ] Define the next action
```

AI is optional. Edit the existing `[ai]` section in `config.toml` to enable
Ollama; the model must already be available on the configured server:

```toml
[ai]
enabled = true
provider = "ollama"
model = "nomic-embed-text"
ollama_url = "http://localhost:11434"
```

Run `neuron list -q "ideas related to my budget"`. Filtered searches and
`neuron search` remain local BM25. The code also supports an OpenAI embedding
provider; selecting a remote provider sends embedding input to that service.
AI settings are edited in TOML, not through `neuron config set`.

</details>

## AI agent access · MCP

Configure a compatible MCP client to launch Neuron over stdio:

```json
{
  "mcpServers": {
    "neuron": {
      "command": "neuron",
      "args": ["mcp", "--read-only"]
    }
  }
}
```

The server exposes `search_notes`, `get_note`, `create_note`, `update_note`,
`list_notes` and `get_daily`. Remove `--read-only` to allow writes. Read-only
mode lets `get_daily` read an existing daily note but blocks creating one. MCP does not currently
expose project, task, move or saved-search tools.

```bash
neuron mcp --vault /absolute/path/to/vault --read-only
neuron mcp --audit-log ~/.local/state/neuron/mcp-audit.jsonl
```

## Help and current scope

| If… | Try… |
| :--- | :--- |
| A new command is missing | Build this checkout and run `./bin/neuron --help` |
| Search finds nothing | Check `neuron config get vault_path`, then broaden the filters |
| A task cannot be updated | Run `neuron tasks --all` again and use its current path and line |
| A project has no context | Inside its repository, run `neuron project init` or `project link <note>` |
| An editor command fails | Check `neuron config get editor`; configure an installed editor |
| The vault needs checking | Run `neuron doctor` and `neuron restore --list` |

**Current scope:** the web workspace browses, creates and edits note bodies.
Existing frontmatter is preserved; rename, move and delete operations remain in
the CLI or your editor. The CLI dashboard is a terminal report and tasks have no
due-date scheduling. The interactive graph is available on the web; the TUI graph
still displays counts.

## Contributing and support

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and
[SECURITY.md](SECURITY.md) for the security policy.

```bash
go test -race ./...
go vet ./...
go build -o bin/neuron ./cmd/neuron
```

[Report an issue](https://github.com/steevin/neuron-cli/issues) ·
[Support the project](https://paypal.me/steevin) ·
[neuron@steevin.com](mailto:neuron@steevin.com)

---

<div align="center">

Created by **Daniel Steevin** · Licensed under [GNU GPL v3](LICENSE)

If Neuron helps you, a GitHub star helps others find it.

</div>
