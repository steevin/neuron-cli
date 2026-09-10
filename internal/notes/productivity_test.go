package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTasksPreserveSourceAndIgnoreExamples(t *testing.T) {
	raw := "---\r\ntitle: Sample\r\nexample: |\r\n  - [ ] yaml\r\n---\r\n- [ ] Real task\r\n```md\r\n- [ ] example\r\n```\r\n~~~\r\n- [ ] other example\r\n~~~\r\n  1. [X] Finished\r\n- [ ] \r\n"
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	n := &Note{Path: path, RawContent: raw}
	tasks := Tasks(n)
	if len(tasks) != 2 || tasks[0].Line != 6 || tasks[1].Done != true {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if err := SetTaskDone(tasks[0], true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := strings.Replace(raw, "- [ ] Real task", "- [x] Real task", 1)
	if string(got) != want {
		t.Fatalf("source was changed unexpectedly: %q", got)
	}
	if err := SetTaskDone(tasks[1], false); err == nil {
		t.Fatal("stale snapshot accepted")
	}
}
func TestTaskReopenAndLongFence(t *testing.T) {
	raw := "````markdown\n```\n- [ ] example\n````\n* [X] actual\n"
	path := filepath.Join(t.TempDir(), "note.md")
	os.WriteFile(path, []byte(raw), 0600)
	tasks := Tasks(&Note{Path: path, RawContent: raw})
	if len(tasks) != 1 || tasks[0].Line != 5 {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if err := SetTaskDone(tasks[0], false); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "* [ ] actual") {
		t.Fatal(string(got))
	}
}
