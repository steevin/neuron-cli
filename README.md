<div align="center">
<img src="docs/assets/logo.png" alt="Neuron CLI Logo" width="200" />
<h1>Neuron CLI</h1>

**Your notes are plain text. Why does managing them have to feel so heavy?**

*If you like Neuron CLI, please consider giving it a ⭐ on GitHub!*

[English] | [Español](README.es.md)
</div>

<br>

> Neuron is a local-first, Obsidian-compatible note manager built for the terminal. It keeps your Markdown vault exactly where it is — no migrations, no proprietary databases, and no cloud subscriptions. Just blistering fast, keyboard-driven access to your thoughts from anywhere in your shell.

---

### Why another note manager?

If you're like me, you spend your day in the terminal and keep your notes in plain Markdown. But most note-taking tools feel too heavy—they are click-heavy, run on resource-hungry Electron wrappers, or try to lock your notes behind a subscription.

Neuron is built differently:
* **Zero Lock-in:** It works directly with your local directory of Markdown files. You can open them in Obsidian, VS Code, or Vim at any time.
* **Frictionless Speed:** Launch, search, create, and organize notes in milliseconds with optimized keyboard shortcuts.
* **AI-Ready:** Query your vault using local AI (via Ollama) or expose it to LLM agents using the built-in Model Context Protocol (MCP) server.

---

### Why Neuron? (Philosophy)

* **Keyboard First:** Your hands should never have to leave the home row. Every action—from searching notes to moving folders and copying code blocks—is a keystroke away.
* **Privacy by Default:** Your thoughts are yours. Neuron is offline-first. It doesn't track you, upload your notes, or require an account.
* **Format Freedom:** We believe plain Markdown with standard YAML frontmatter is the best way to own your notes. If you decide to stop using Neuron tomorrow, they are still just text files—readable by any editor, with zero lock-in.

---

### Quick Start

Get up and running in three simple commands:

1. **Install Neuron:**
   ```bash
   brew install steevin/tap/neuron
   ```
2. **Initialize Your Vault:**
   ```bash
   neuron init
   ```
   *Point it to an existing Obsidian directory, or create a brand new vault.*
3. **Launch the TUI:**
   ```bash
   neuron
   ```
   *Press `?` inside the interface to see all available shortcuts.*

---

### Installation

Detailed options for installing Neuron:

```bash
# Homebrew (macOS & Linux)
brew install steevin/tap/neuron

# Binary via curl
curl -sSfL https://github.com/steevin/neuron-cli/releases/latest/download/neuron_$(uname -s)_$(uname -m).tar.gz | tar -xz -C /usr/local/bin neuron

# Go (requires Go installed)
go install github.com/steevin/neuron-cli@latest

# From Source
git clone https://github.com/steevin/neuron-cli && cd neuron-cli && make build
```

---

### Features

#### Organize without thinking about it (PARA)
Neuron understands the **Projects · Areas · Resources · Archive** (PARA) framework out of the box. It scans your vault folders and helps you stay organized without breaking your flow:
* **Intelligent Folder Picker:** When you create a note (`n` or `neuron add`), an interactive menu prompts you to choose the destination folder. No more notes piling up in your vault's root.
* **Fluid Navigation:** Move around folders using `← →` / `h l` or `↑ ↓` / `j k`. Press `Enter` to save, `Esc` to cancel.
* **Quick Moves:** Use `neuron move` or the `/move` TUI command to relocate any note instantly.
* *Flat vault support:* If you don't use PARA, the folder picker steps aside automatically.

#### Never get lost (Live Folder Breadcrumbs)
The breadcrumb bar at the bottom of the TUI shows you the file's path (e.g., `📂 1. Projects` or `📂 2. Areas/Finance`) as you scroll through your list.

#### Capture thoughts instantly (Clipboard & Paste)
* **Instant Appending (`ctrl+v`):** Press `ctrl+v` on any note in the list to append your clipboard content directly to the file on disk. Perfect for clipping web highlights or stack traces.
* **Smart Asset Management:** If your clipboard contains an image URL or local path, Neuron downloads/copies it into your vault's `assets/` directory and creates a clean Markdown link automatically.
* **Bracketed Paste:** Paste a block of text when creating a note to automatically set the first line as the title and the rest as the body.

#### Find anything as fast as you think it (Dual Search)
* **BM25 Search:** Standard keyword search that responds as fast as you type.
* **Semantic / AI Search:** Connect Ollama in your configuration to query your notes by concepts and ideas rather than exact keyword matches (e.g., `neuron list -q "show me things related to my budget"`).

#### Templates that save you keystrokes
Stop typing frontmatter by hand. Define reusable templates and instantiate them on the fly:
```bash
neuron add "2025-06-01 Standup" --template standup
neuron today                                        # Auto-generates your daily note using your custom template
```

#### Link your notes like your brain does
* **Obsidian-style Wikilinks:** Full support for `[[wikilink]]` extraction and indexing.
* **Tags:** Inline `#tags` are automatically indexed and searchable.
* **Knowledge Summary:** Press `g` in the TUI to see an instant count of notes (nodes) and connections (edges) in your personal knowledge graph.

#### Friendly interactive prompts
Forgot a flag? Neuron prompts you with terminal forms powered by `huh` to guide you through note creation, folder picking, and confirmation dialogs.

#### Easy on the eyes (Themes)
Toggle between dark (Tokyo Night) and light (GitHub) color schemes live using `/theme` or lock it in via your configuration.

---

### TUI Keybindings

| Key | Action |
|-----|--------|
| `j / k` or `↑ / ↓` | Navigate note list |
| `Enter` | Select / confirm |
| `Tab / Shift+Tab` | Switch pane focus (sidebar ↔ preview) |
| `n` | New note (triggers PARA folder picker) |
| `e` | Edit selected note in `$EDITOR` |
| `ctrl+v` | Paste clipboard into selected note |
| `c` / `y` | Copy/yank code blocks from the selected note |
| `/` | Command palette (fuzzy search) |
| `s` | Git sync |
| `ctrl+g` | Knowledge graph summary |
| `?` | Help overlay (all keybindings) |
| `q` | Quit |

**During folder selection (`📁 SAVE TO` mode)**

| Key | Action |
|-----|--------|
| `← → / h l / ↑ ↓ / j k` | Navigate folder chips |
| `Enter` | Confirm folder |
| `Esc` | Cancel |

**During code block extraction (`💻 COPY CODE` mode)**

| Key | Action |
|-----|--------|
| `← → / h l / ↑ ↓ / j k` | Navigate code blocks |
| `Enter` | Copy selected code block to clipboard |
| `Esc` | Cancel |

---

### Command Palette

Fuzzy-search any command in the TUI at any time by pressing `/`:

| Command | Description |
|---------|-------------|
| `/add <title>` | Create a new note (triggers folder picker) |
| `/today` | Open or create today's daily note |
| `/edit`, `/e` | Open the selected note in `$EDITOR` |
| `/copy`, `/c` | Copy the current note to clipboard |
| `/attach <path_or_url>`| Download or copy image to assets/ and attach to note |
| `/links`, `/l` | Open the first URL in the note in your browser |
| `/move <folder>` | Move the selected note to a PARA folder |
| `/rm` | Delete the selected note |
| `/sync`, `/s` | Git push (with optional pull) |
| `/stats` | Show vault statistics |
| `/open`, `/o` | Reveal vault in Finder |
| `/theme dark\|light` | Switch the TUI colour scheme live |
| `/quit` | Exit neuron |

---

### CLI Usage

```bash
neuron                                   # open the TUI (default)
neuron init                              # interactive setup wizard (first run)
neuron add                               # prompt for title + PARA folder picker
neuron add "standup notes" --tag work    # create note with tag, then pick folder
neuron add "1. Projects/API redesign"    # skip picker — explicit path prefix
neuron add "Config" --file nginx.conf --code # create note directly from file
cat script.py | neuron add "Script" --code python # create note from piped code
neuron edit "standup notes"             # open in $EDITOR
neuron today                             # daily note for today
neuron list -q "kubernetes"              # full-text / semantic search
neuron move "standup notes" projects    # move note to your Projects folder
neuron attach "standup notes" ./img.png # attach an image or file to a note
neuron links "standup notes"             # extract and open links or images
neuron sync --pull                       # git pull + push
neuron stats                             # note count, tag count
neuron config set editor nvim            # change default editor
neuron config set theme dark             # set colour theme
neuron mcp                               # start the MCP server
```

---

### MCP (AI Agent Access)

Neuron exposes your vault as a [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server. You can add it to any compatible client (Claude Desktop, Cursor, Antigravity…) to give your AI assistants direct access to your knowledge base:

```json
{
  "mcpServers": {
    "neuron": { "command": "neuron", "args": ["mcp"] }
  }
}
```

Once configured, you can ask your AI to search, create, summarize, or move notes directly from your vault — without leaving the chat.

---

### Vault Format

Neuron uses plain Markdown with YAML frontmatter — identical to Obsidian. Point Neuron at an existing Obsidian vault, and it just works.

```markdown
---
title: My Note
tags: [ideas, project]
created: 2025-05-30T09:00:00Z
---

Content with [[wikilinks]] and #inline-tags.
```

**Recommended PARA structure** (Neuron auto-detects any variant):

```
vault/
├── 1. Projects/
├── 2. Areas/
├── 3. Resources/
└── 4. Archive/
```

---

### Updating Neuron

Keep your installation up to date:

```bash
# Homebrew
brew upgrade steevin/tap/neuron

# Binary (curl)
curl -sSfL https://github.com/steevin/neuron-cli/releases/latest/download/neuron_$(uname -s)_$(uname -m).tar.gz | tar -xz -C /usr/local/bin neuron
```

---

### Support

Building and maintaining open-source tools takes time. If Neuron makes your day-to-day work in the terminal a little better, consider buying me a coffee or supporting the project:
[**Support the project ➔**](https://paypal.me/steevin)

---

### License & Attribution

Neuron CLI is open-source and licensed under the **GNU GPL v3**.

#### In plain English:
* **Keep it open:** If you modify and share this code, your version must also be open-source under the GPL v3.
* **Give credit:** Please keep the original copyright notice and Daniel Steevin's author info.
* **Show some love:** If you use parts of this project in a public fork, please add a small note in your README pointing back to Neuron CLI. It really helps the project grow!

---

### Contact

For support, feedback, business inquiries, or any other questions, please contact:
[**neuron@steevin.com**](mailto:neuron@steevin.com)

---

<div align="center">
Made by Daniel Steevin
<br>
Licensed under the <a href="LICENSE">GNU GPL v3 License</a> — Open Source with Copyleft.
</div>
