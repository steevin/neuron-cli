package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/notes"
)

func productivityFixture(t *testing.T) *notes.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := config.DefaultConfig()
	cfg.VaultPath = filepath.Join(home, "vault")
	if err := config.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	s, err := notes.NewStore(cfg.VaultPath)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func runProductivity(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
func TestCaptureInboxAndFile(t *testing.T) {
	s := productivityFixture(t)
	for i := 0; i < 2; i++ {
		runProductivity(t, newCaptureCommand(), "Idea / API\n- [ ] Implement")
	}
	all, err := s.List(notes.ListOptions{})
	if err != nil || len(all) != 2 {
		t.Fatalf("%v %d", err, len(all))
	}
	for _, n := range all {
		if !inFolder(n, "Inbox") || !strings.Contains(n.Content, "Idea / API") {
			t.Fatalf("bad capture: %+v", n)
		}
	}
	runProductivity(t, newInboxCommand(), "file", all[0].RelPath, "Projects")
	out := runProductivity(t, newInboxCommand())
	if strings.Count(out, "Inbox/") != 1 {
		t.Fatal(out)
	}
	out = runProductivity(t, newTasksCommand())
	if strings.Count(out, "Implement") != 2 {
		t.Fatal(out)
	}
	out = runProductivity(t, newDashboardCommand())
	if !strings.Contains(out, "2 pending") || !strings.Contains(out, "1 to process") {
		t.Fatal(out)
	}
}
func TestSavedSearchFiltersAndPersistence(t *testing.T) {
	s := productivityFixture(t)
	if _, err := s.Create("1. Projects/API", "API", []string{"work"}, "timeout bug"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("1. Projects-old", "Wrong", []string{"work"}, "timeout"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("Inbox", "Other", []string{"personal"}, "timeout"); err != nil {
		t.Fatal(err)
	}
	runProductivity(t, newSearchCommand(), "save", "work", `timeout tag:work folder:"1. Projects"`)
	out := runProductivity(t, newSearchCommand(), "run", "work")
	if !strings.Contains(out, "api.md") || strings.Contains(out, "Wrong") || strings.Contains(out, "Other") {
		t.Fatal(out)
	}
	out = runProductivity(t, newSearchCommand(), "list")
	if !strings.Contains(out, "work") {
		t.Fatal(out)
	}
	runProductivity(t, newSearchCommand(), "remove", "work")
	state, err := readWorkspace(s)
	if err != nil || len(state.Searches) != 0 {
		t.Fatalf("%+v %v", state, err)
	}
	if _, _, err := filterQuery(nil, `folder:"unclosed`); err == nil {
		t.Fatal("accepted invalid query")
	}
}
func TestProjectLinksPlainMarkdownAcrossReload(t *testing.T) {
	s := productivityFixture(t)
	path := filepath.Join(s.VaultPath, "external.md")
	if err := os.WriteFile(path, []byte("# External\n"), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := resolveNote(s, "external.md")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := readWorkspace(s)
	state.Projects["/repo"] = projectLink{"/repo", n.ID, n.RelPath}
	if err := saveWorkspace(s, state); err != nil {
		t.Fatal(err)
	}
	fresh, err := notes.NewStore(s.VaultPath)
	if err != nil {
		t.Fatal(err)
	}
	state, err = readWorkspace(fresh)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveProjectNote(fresh, state.Projects["/repo"])
	if err != nil || got.Title != "External" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestProjectLifecycle(t *testing.T) {
	s := productivityFixture(t)
	repo := t.TempDir()
	t.Chdir(repo)
	if output, err := exec.Command("git", "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", output, err)
	}
	runProductivity(t, newProjectCommand(), "init", "--name", "Test project")
	state, err := readWorkspace(s)
	if err != nil || len(state.Projects) != 1 {
		t.Fatalf("%+v %v", state, err)
	}
	if !strings.Contains(runProductivity(t, newDashboardCommand()), "test-project.md") {
		t.Fatal("project missing from dashboard")
	}
	sub := filepath.Join(repo, "src")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Projects[root]; !ok {
		t.Fatal("subdirectory not resolved")
	}
	runProductivity(t, newProjectCommand(), "unlink")
	all, err := s.List(notes.ListOptions{})
	if err != nil || len(all) != 1 {
		t.Fatal("unlink removed source note")
	}
	state, err = readWorkspace(s)
	if err != nil || len(state.Projects) != 0 {
		t.Fatal("unlink did not persist")
	}
}
