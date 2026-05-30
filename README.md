<div align="center">

# 🧠 neuron-cli

**Your second brain, from the terminal.**

[![CI](https://github.com/danielsteevin/neuron-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/danielsteevin/neuron-cli/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danielsteevin/neuron-cli)](go.mod)
[![License](https://img.shields.io/badge/license-BUSL--1.1-blue)](LICENSE)

> A blazing-fast, local-first knowledge manager for people who live in the terminal.
> Obsidian-compatible. AI-powered. Privacy-first.

</div>

---

## ✨ Features
- 📁 **Obsidian-compatible** — point it at your existing vault, zero migration
- 🔍 **Instant search** — full-text search across all your notes
- 🤖 **MCP server** — let AI agents (Claude, Cursor, Antigravity) read & write your vault
- 🔗 **Knowledge graph** — visualize note connections in ASCII
- ⚡ **Lightning fast** — written in Go, starts in milliseconds
- 🔒 **Privacy first** — 100% local, your data never leaves your machine
- 🔄 **Git sync** — version control your knowledge

## 📦 Installation

### Homebrew
```bash
brew install danielsteevin/tap/neuron
```

### Go install
```bash
go install github.com/danielsteevin/neuron-cli@latest
```

### Build from source
```bash
git clone https://github.com/danielsteevin/neuron-cli
cd neuron-cli && make build
```

## 🚀 Quick Start

```bash
# Open the TUI
neuron

# Point to your Obsidian vault
neuron list --vault ~/Documents/ObsidianVault

# Add a quick note
neuron add "My brilliant idea" --tag ideas

# Search your vault
neuron list --query "distributed systems"

# Open today's daily note
neuron today

# Enable AI agent access (add to your AI config)
neuron mcp
```

## 🤖 AI Agent Integration (MCP)

neuron exposes your vault as an [MCP server](https://modelcontextprotocol.io), compatible with Claude Desktop, Cursor, Antigravity, and any MCP client.

Add to your AI agent config:
```json
{
  "mcpServers": {
    "neuron": {
      "command": "neuron",
      "args": ["mcp"]
    }
  }
}
```

Then ask your AI:
- *"Search my notes for anything about Kubernetes"*
- *"Create a meeting note for today's standup"*
- *"What did I write about Go concurrency?"*

## ⌨️ Keybindings

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate notes |
| `/` | Search |
| `n` | New note |
| `e` | Edit in $EDITOR |
| `g` | Knowledge graph |
| `s` | Git sync |
| `?` | Help |
| `q` | Quit |

## 📂 Vault Format

All notes are plain Markdown files with YAML frontmatter — 100% compatible with Obsidian:

```markdown
---
title: "My Note"
tags: [ideas, project]
created: 2025-05-30T09:00:00
---

# My Note

Content here with [[wikilinks]] and #inline-tags.
```

## 🏗️ Comparison

| Feature | neuron-cli | Obsidian | vim-wiki | zk |
|---------|-----------|----------|----------|---------|
| Terminal-native TUI | ✅ | ❌ | ⚠️ | ✅ |
| Obsidian compatible | ✅ | ✅ | ❌ | ❌ |
| MCP server (AI) | ✅ | ❌ | ❌ | ❌ |
| Local-first | ✅ | ✅ | ✅ | ✅ |
| Knowledge graph | ✅ | ✅ | ❌ | ⚠️ |
| No subscription | ✅ | ⚠️ | ✅ | ✅ |

## 📄 License

[Business Source License 1.1](LICENSE) — Free for personal and internal use.

---

<div align="center">
Made with ❤️ by Daniel Steevin
</div>
