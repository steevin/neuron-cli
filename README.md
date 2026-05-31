<div align="center">

# neuron-cli

**A terminal knowledge manager that doesn't get in your way.**

[![CI](https://github.com/steevin/neuron-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/steevin/neuron-cli/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/steevin/neuron-cli)](go.mod)
[![License](https://img.shields.io/badge/license-BUSL--1.1-blue)](LICENSE)

</div>

neuron is a local-first, Obsidian-compatible note manager for the terminal. It keeps your Markdown vault exactly where it is — no migration, no cloud, no subscription. Just fast, keyboard-driven access to your notes from anywhere in a shell.

**✨ Pro Features Included:**
- **Interactive CLI Prompts:** Missing arguments? `neuron` will interactively prompt you using beautiful terminal UI (`huh`).
- **Fuzzy Command Palette:** Press `/` in the TUI to fuzzy-search available commands.
- **Rich TUI Help:** Press `?` in the TUI to open a modal overlay with all keybindings.
- **Colorized Outputs & Spinners:** Clean, colorful formatting using `lipgloss` and `bubbles/spinner`.

---

## Install

```bash
# Homebrew
brew install steevin/tap/neuron

# Go
go install github.com/steevin/neuron-cli@latest

# Source
git clone https://github.com/steevin/neuron-cli && cd neuron-cli && make build
```

## Usage

```bash
neuron                              # open the TUI (default)
neuron init                         # interactive setup wizard (first run)
neuron add                          # interactively prompts for a note title
neuron add "standup notes" --tag work
neuron edit "standup notes"         # opens in $EDITOR
neuron today                        # daily note for today
neuron list -q "kubernetes"         # search your vault (now with colors!)
neuron sync --pull                  # git pull + push
neuron config set editor nvim       # change editor
```

## MCP (AI agent access)

neuron exposes your vault as an [MCP server](https://modelcontextprotocol.io). Add it to any compatible client (Claude Desktop, Cursor, Antigravity…):

```json
{
  "mcpServers": {
    "neuron": { "command": "neuron", "args": ["mcp"] }
  }
}
```

Then you can just ask your AI to search, create, or summarize notes directly from your vault.

## TUI keybindings

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate |
| `/` | Search / command palette |
| `e` | Edit selected note |
| `n` | New note |
| `s` | Git sync |
| `g` | Knowledge graph summary |
| `tab` | Switch pane focus |
| `?` | Help |
| `q` | Quit |

## Vault format

Plain Markdown with YAML frontmatter — identical to Obsidian. Point neuron at an existing Obsidian vault and it just works.

```markdown
---
title: My Note
tags: [ideas, project]
created: 2025-05-30T09:00:00Z
---

Content with [[wikilinks]] and #inline-tags.
```

## Update

```bash
brew upgrade steevin/tap/neuron
```

## License

[Business Source License 1.1](LICENSE) — free for personal and internal use.

---

## ❤️ Support this project

If you find Neuron CLI useful, you can help support the development by donating via PayPal:
[Donate via PayPal](https://paypal.me/steevin)

---

<div align="center">
Made by Daniel Steevin
</div>
