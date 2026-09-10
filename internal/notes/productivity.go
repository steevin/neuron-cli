package notes

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Task points to an actual source line, so completion preserves all other Markdown.
type Task struct {
	Note *Note
	Line int
	Text string
	Done bool
}

var taskPattern = regexp.MustCompile(`^\s*(?:[-+*]|[0-9]+[.)])\s+\[([ xX])\]\s+(.*)$`)

func Tasks(n *Note) []Task {
	var tasks []Task
	fence := byte(0)
	fenceLen := 0
	frontmatter := false
	for i, line := range strings.Split(n.RawContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			frontmatter = true
			continue
		}
		if frontmatter {
			if trimmed == "---" || trimmed == "..." {
				frontmatter = false
			}
			continue
		}
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			count := 0
			for count < len(trimmed) && trimmed[count] == trimmed[0] {
				count++
			}
			if fence == 0 && count >= 3 {
				fence = trimmed[0]
				fenceLen = count
				continue
			}
			if fence == trimmed[0] && count >= fenceLen && strings.TrimSpace(trimmed[count:]) == "" {
				fence = 0
				continue
			}
		}
		if fence != 0 {
			continue
		}
		match := taskPattern.FindStringSubmatch(line)
		if match != nil && strings.TrimSpace(match[2]) != "" {
			tasks = append(tasks, Task{n, i + 1, strings.TrimSpace(match[2]), match[1] != " "})
		}
	}
	return tasks
}

// SetTaskDone rejects stale snapshots instead of overwriting an externally edited note.
func SetTaskDone(task Task, done bool) error {
	raw, err := os.ReadFile(task.Note.Path)
	if err != nil {
		return err
	}
	if string(raw) != task.Note.RawContent {
		return fmt.Errorf("note changed; list tasks again before updating")
	}
	lines := strings.Split(string(raw), "\n")
	if task.Line < 1 || task.Line > len(lines) {
		return fmt.Errorf("invalid task line")
	}
	line := lines[task.Line-1]
	match := taskPattern.FindStringSubmatchIndex(line)
	if match == nil {
		return fmt.Errorf("line is not a task")
	}
	mark := " "
	if done {
		mark = "x"
	}
	lines[task.Line-1] = line[:match[2]] + mark + line[match[3]:]
	return os.WriteFile(task.Note.Path, []byte(strings.Join(lines, "\n")), 0o600)
}
