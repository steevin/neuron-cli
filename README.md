<div align="center">

<img src="docs/assets/logo.png" alt="Neuron CLI Logo" width="200" />
<br>
<img src="https://readme-typing-svg.herokuapp.com?font=Fira+Code&weight=800&size=50&pause=1000&color=00D2FF&center=true&vCenter=true&width=600&lines=NEURON+CLI;TERMINAL+KNOWLEDGE;LOCAL+FIRST;AI+READY" alt="Typing SVG" />

**A terminal knowledge manager that doesn't get in your way.**

[![CI](https://img.shields.io/github/actions/workflow/status/steevin/neuron-cli/ci.yml?style=for-the-badge&color=00d2ff&logo=github)](https://github.com/steevin/neuron-cli/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/steevin/neuron-cli?style=for-the-badge&color=8a2be2&logo=go)](go.mod)
[![License](https://img.shields.io/badge/license-BUSL--1.1-00d2ff?style=for-the-badge)](LICENSE)

</div>

<br>

> **neuron** is a local-first, Obsidian-compatible note manager for the terminal. It keeps your Markdown vault exactly where it is — no migration, no cloud, no subscription. Just fast, keyboard-driven access to your notes from anywhere in a shell.

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> ✨ FEATURES

#### 🗂️ PARA Methodology — Built In
neuron understands the **Projects · Areas · Resources · Archive** framework out of the box. It scans your vault for PARA folders and surfaces them throughout the UI:

- **Folder picker on note creation** — After typing a title (`n` in the TUI or `neuron add`), an interactive chip list lets you choose the destination folder before the note is saved. No more notes silently landing at the vault root.
- **Keyboard navigation**: `← →` / `h l` / `↑ ↓` / `j k` to move between folders, `Enter` to confirm, `Esc` to cancel.
- **`neuron move`** — Relocate any note to a different PARA folder at any time, from both the CLI and the `/move` TUI command.
- Vaults without PARA folders skip the picker entirely — zero friction for simple setups.

#### 📂 Live Folder Breadcrumb
The status bar at the bottom of the TUI permanently shows where the currently highlighted note lives — e.g. `📂 1. Projects` or `📂 2. Areas/Finance` — giving instant spatial context while browsing without opening the note.

#### 📋 Clipboard-to-Note (Paste Workflow)
- **`ctrl+v`** on a selected note appends your clipboard contents directly to that note on disk — great for capturing web snippets or code blocks without leaving the terminal.
- **Bracketed paste** support: paste any amount of text while typing a new note title to pre-fill both the title (first line, up to 40 chars) and the note body in a single gesture.

#### 🔍 Dual Search Engine
- **BM25 full-text search** — instant keyword search across all notes, available out of the box.
- **Semantic / AI search** — enable Ollama in your config to get embedding-based similarity search (`neuron list -q "your concept"`).

#### 📝 Template System
Create note templates and render them on demand:

```bash
neuron add "2025-06-01 Standup" --template standup
neuron today                                        # uses a "daily" template automatically
```

#### 🔗 Wikilinks & Knowledge Graph
- Full `[[wikilink]]` extraction and index — same format as Obsidian.
- Press `g` in the TUI to get an instant summary of your knowledge graph: nodes (notes) and edges (links).
- Inline `#tags` are extracted automatically from note bodies.

#### 💅 Interactive CLI Prompts
Missing arguments? `neuron` will prompt you interactively for anything it needs — title, folder, confirmation — using rich terminal forms (`huh`).

#### ⚡ Command Palette
Press `/` in the TUI to fuzzy-search all available commands:

| Command | Description |
|---------|-------------|
| `/add <title>` | Create a new note (triggers folder picker) |
| `/today` | Open or create today's daily note |
| `/edit` | Open the selected note in `$EDITOR` |
| `/move <folder>` | Move the selected note to a PARA folder |
| `/rm` | Delete the selected note |
| `/sync` | Git push (with optional pull) |
| `/stats` | Show vault statistics |
| `/open` | Reveal vault in Finder |
| `/theme dark\|light` | Switch the TUI colour scheme live |
| `/quit` | Exit neuron |

#### 🎨 Splash Screen & Theming
- A premium ASCII splash screen greets you on launch with your vault stats and a quick-start keybinding reference.
- Two built-in themes: **dark** (Tokyo Night) and **light** (GitHub). Switch live with `/theme dark` or persist with `neuron config set theme dark`.

---

### <img src="https://img.shields.io/badge/--00d2ff?style=flat-square" width="10" height="20"> 🚀 INSTALLATION

```bash
# Homebrew
brew install steevin/tap/neuron

# Go
go install github.com/steevin/neuron-cli@latest

# Source
git clone https://github.com/steevin/neuron-cli && cd neuron-cli && make build
```

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> 💻 USAGE

```bash
neuron                                   # open the TUI (default)
neuron init                              # interactive setup wizard (first run)
neuron add                               # prompt for title + PARA folder picker
neuron add "standup notes" --tag work    # create note with tag, then pick folder
neuron add "1. Projects/API redesign"    # skip picker — explicit path prefix
neuron edit "standup notes"             # open in $EDITOR
neuron today                             # daily note for today
neuron list -q "kubernetes"              # full-text / semantic search
neuron move "standup notes" projects    # move note to your Projects folder
neuron sync --pull                       # git pull + push
neuron stats                             # note count, tag count
neuron config set editor nvim            # change default editor
neuron config set theme dark             # set colour theme
neuron mcp                               # start the MCP server
```

---

### <img src="https://img.shields.io/badge/--00d2ff?style=flat-square" width="10" height="20"> 🧠 MCP (AI AGENT ACCESS)

neuron exposes your vault as an [MCP server](https://modelcontextprotocol.io). Add it to any compatible client (Claude Desktop, Cursor, Antigravity…):

```json
{
  "mcpServers": {
    "neuron": { "command": "neuron", "args": ["mcp"] }
  }
}
```

Then you can ask your AI to search, create, summarize, or move notes directly from your vault — without leaving the chat.

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> ⌨️ TUI KEYBINDINGS

| Key | Action |
|-----|--------|
| `j / k` or `↑ / ↓` | Navigate note list |
| `Enter` | Select / confirm |
| `Tab / Shift+Tab` | Switch pane focus (sidebar ↔ preview) |
| `n` | New note (triggers PARA folder picker) |
| `e` | Edit selected note in `$EDITOR` |
| `ctrl+v` | Paste clipboard into selected note |
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

---

### <img src="https://img.shields.io/badge/--00d2ff?style=flat-square" width="10" height="20"> 📂 VAULT FORMAT

Plain Markdown with YAML frontmatter — identical to Obsidian. Point neuron at an existing Obsidian vault and it just works.

```markdown
---
title: My Note
tags: [ideas, project]
created: 2025-05-30T09:00:00Z
---

Content with [[wikilinks]] and #inline-tags.
```

**Recommended PARA structure** (neuron auto-detects any variant):

```
vault/
├── 1. Projects/
├── 2. Areas/
├── 3. Resources/
└── 4. Archive/
```

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> 🔄 UPDATE

```bash
brew upgrade steevin/tap/neuron
```

---

### <img src="https://img.shields.io/badge/--00d2ff?style=flat-square" width="10" height="20"> ❤️ SUPPORT THIS PROJECT

If you find Neuron CLI useful, you can help support the development by donating via PayPal:
[**Donate via PayPal ➔**](https://paypal.me/steevin)

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> ✉️ CONTACT

For support, feedback, business inquiries, or any other questions, please contact:
[**neuron@steevin.com**](mailto:neuron@steevin.com)

---

<div align="center">
Made by Daniel Steevin
<br>
<a href="LICENSE">Business Source License 1.1</a> — free for personal and internal use.
</div>
