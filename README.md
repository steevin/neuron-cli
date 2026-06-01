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

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> ✨ PRO FEATURES INCLUDED

- <kbd>Interactive CLI Prompts</kbd> Missing arguments? `neuron` will interactively prompt you using beautiful terminal UI (`huh`).
- <kbd>Fuzzy Command Palette</kbd> Press `/` in the TUI to fuzzy-search available commands.
- <kbd>Rich TUI Help</kbd> Press `?` in the TUI to open a modal overlay with all keybindings.
- <kbd>Colorized Outputs & Spinners</kbd> Clean, colorful formatting using `lipgloss` and `bubbles/spinner`.

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
user@neuron-cli:~$ neuron                              # open the TUI (default)
user@neuron-cli:~$ neuron init                         # interactive setup wizard (first run)
user@neuron-cli:~$ neuron add                          # interactively prompts for a note title
user@neuron-cli:~$ neuron add "standup notes" --tag work
user@neuron-cli:~$ neuron edit "standup notes"         # opens in $EDITOR
user@neuron-cli:~$ neuron today                        # daily note for today
user@neuron-cli:~$ neuron list -q "kubernetes"         # search your vault (now with colors!)
user@neuron-cli:~$ neuron sync --pull                  # git pull + push
user@neuron-cli:~$ neuron config set editor nvim       # change editor
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

Then you can just ask your AI to search, create, or summarize notes directly from your vault.

---

### <img src="https://img.shields.io/badge/--8a2be2?style=flat-square" width="10" height="20"> ⌨️ TUI KEYBINDINGS

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

<div align="center">
Made by Daniel Steevin
<br>
[Business Source License 1.1](LICENSE) — free for personal and internal use.
</div>
