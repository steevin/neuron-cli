package main

import "github.com/spf13/cobra"

// Keep workflow help together so parent and nested commands explain the same
// source paths, persistence rules and search behavior.
type commandHelp struct {
	short   string
	long    string
	example string
}

var productivityHelp = map[string]commandHelp{
	"capture": {
		"Capture text into Inbox without prompts or an editor",
		"Save text as a Markdown note in Inbox/. Pass text as arguments or pipe it on stdin.\nThe first line supplies the title; the complete text is preserved in the body.\nPrints the relative note path. Empty input is rejected; repeated titles create separate notes.",
		"  neuron capture \"Investigate API timeout\"\n  printf 'API follow-up\\n- [ ] Reproduce timeout\\n' | neuron capture",
	},
	"inbox": {
		"Review captured notes",
		"List notes in Inbox/ and its subfolders, with paths relative to the vault.\nUse 'inbox file' to classify a capture, or 'neuron edit <title>' to edit it.",
		"  neuron inbox\n  neuron inbox file \"Inbox/api-follow-up.md\" \"1. Projects\"",
	},
	"inbox file": {
		"Move an Inbox note into a destination folder",
		"Move a captured note out of Inbox. The destination is relative to the vault.\nUse the exact relative path printed by 'neuron inbox'; note IDs and titles also work.\nNotes outside Inbox are rejected. The destination folder is created if needed.",
		"  neuron inbox file \"Inbox/api-follow-up.md\" \"1. Projects\"",
	},
	"tasks": {
		"List Markdown tasks with source paths and line numbers",
		"Collect unchecked Markdown tasks across the vault. Use --all to include completed tasks.\nEach result includes its vault-relative path and actual source line number.\nEmpty tasks, YAML frontmatter and fenced code blocks are excluded.\nTasks stay in their original notes; no separate task database is created.",
		"  neuron tasks\n  neuron tasks --all --folder \"1. Projects\"\n  neuron tasks done \"Inbox/api-follow-up.md\" 10",
	},
	"tasks done": {
		"Complete a task in its source note",
		"Mark one Markdown checkbox as completed, preserving the rest of the source file.\nUse the note path and line printed by 'neuron tasks'. The line below is illustrative;\nlist tasks again after editing a note because line numbers may change.",
		"  neuron tasks\n  neuron tasks done \"Inbox/api-follow-up.md\" 10",
	},
	"tasks reopen": {
		"Reopen a completed task in its source note",
		"Uncheck one Markdown task, preserving the rest of the source file.\nUse --all to find completed tasks, then copy their note path and actual line number.",
		"  neuron tasks --all\n  neuron tasks reopen \"Inbox/api-follow-up.md\" 10",
	},
	"tasks open": {
		"Open the source note of a task in your editor",
		"Validate the task at the specified source line and open its entire note in the\nconfigured editor. This command does not position the editor cursor at that line.\nUse the path and actual line printed by 'neuron tasks'.",
		"  neuron tasks\n  neuron tasks open \"Inbox/api-follow-up.md\" 10",
	},
	"dashboard": {
		"Show pending tasks, Inbox, linked projects and recent notes",
		"Print a daily overview with up to --limit items per section. Pending tasks span\nthe entire vault; they are not filtered by due date. Linked repositories count as\nactive projects. Run 'neuron today' to open or create today's daily note.\nThis is a read-only terminal overview, not an interactive screen.",
		"  neuron dashboard\n  neuron dashboard --limit 10\n  neuron today",
	},
	"project": {
		"Open the context note linked to the current Git repository",
		"Run inside a Git repository or any of its subdirectories to open its linked note\nin your configured editor. Use 'project init' to create a context note or\n'project link' to associate an existing note. Links are local to this vault and\nstored in .neuron/workspace.json; repository paths are specific to this machine.",
		"  neuron project init --name \"API redesign\"\n  neuron project\n  neuron project list",
	},
	"project init": {
		"Create and link a project context note",
		"Create a Markdown note with Objective, Next steps, Decisions and Related notes\nsections. Uses a detected Projects folder, or Projects/ when none is found.\nRun inside a Git repository. An already linked repository is left unchanged.",
		"  neuron project init\n  neuron project init --name \"API redesign\"",
	},
	"project link": {
		"Link an existing note to the current Git repository",
		"Associate a vault-relative note path, ID or title with the current Git repository.\nReplaces this repository's previous link without deleting either note.",
		"  neuron project link \"Projects/api.md\"",
	},
	"project list": {
		"List repository links in the current vault",
		"List linked repository paths and their context notes. Works outside a Git repository.",
		"  neuron project list",
	},
	"project unlink": {
		"Remove the current repository link without deleting its note",
		"Run inside the linked Git repository. Removes its association from the current\nvault and its active-project entry in the dashboard; the Markdown note remains.",
		"  neuron project unlink",
	},
	"search": {
		"Search notes with text, tag: and folder: filters",
		"Search locally with BM25. Combine tag: and folder: filters with AND; folder filters\ninclude subfolders. Quote the full query and values containing spaces as shown below.\nThis command does not use AI. Unfiltered 'neuron list -q' uses semantic search when\nAI is enabled. Use 'search save' to keep a query in .neuron/workspace.json.\nTo search a word reserved as a subcommand, use 'neuron list -q <word>'.",
		"  neuron search 'timeout tag:work folder:\"1. Projects\"'\n  neuron search 'tag:work' --limit 0\n  neuron search save work 'tag:work folder:\"1. Projects\"'",
	},
	"search save": {
		"Save or replace a named query",
		"Store a query in the current vault. Reusing a name replaces its previous query.\nNames are case-sensitive. Quote the query to preserve filters containing spaces.",
		"  neuron search save work 'tag:work folder:\"1. Projects\"'\n  neuron search run work",
	},
	"search list": {
		"List saved search names and queries",
		"Show the saved searches in the current vault, sorted by name.",
		"  neuron search list",
	},
	"search run": {
		"Run a saved search",
		"Execute a named query with local BM25 and show up to 50 results.\nUse 'neuron list --saved <name> --limit 0' to display all matches.",
		"  neuron search run work\n  neuron list --saved work --limit 0",
	},
	"search remove": {
		"Remove a saved query without deleting notes",
		"Delete a named search from this vault. Matching notes remain unchanged.",
		"  neuron search remove work",
	},
}

func applyProductivityHelp(cmd *cobra.Command, path string) {
	if h, ok := productivityHelp[path]; ok {
		cmd.Short, cmd.Long, cmd.Example = h.short, h.long, h.example
	}
	for _, child := range cmd.Commands() {
		applyProductivityHelp(child, path+" "+child.Name())
	}
}
