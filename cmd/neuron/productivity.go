package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/search"
)

type projectLink struct {
	Repository string `json:"repository"`
	NoteID     string `json:"note_id"`
	NotePath   string `json:"note_path"`
}
type workspaceState struct {
	Searches map[string]string      `json:"searches"`
	Projects map[string]projectLink `json:"projects"`
}

func productivityStore() (*config.Config, *notes.Store, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	s, err := notes.NewStore(cfg.VaultPath)
	return cfg, s, err
}
func readWorkspace(s *notes.Store) (workspaceState, error) {
	state := workspaceState{Searches: map[string]string{}, Projects: map[string]projectLink{}}
	data, err := os.ReadFile(filepath.Join(s.NeuronDir(), "workspace.json"))
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	if state.Searches == nil {
		state.Searches = map[string]string{}
	}
	if state.Projects == nil {
		state.Projects = map[string]projectLink{}
	}
	return state, err
}
func saveWorkspace(s *notes.Store, state workspaceState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.NeuronDir(), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.NeuronDir(), "workspace-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.NeuronDir(), "workspace.json"))
}
func openProductivityNote(cfg *config.Config, n *notes.Note) error {
	c, err := editorCommand(cfg, n.Path)
	if err != nil {
		return err
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}
func resolveNote(s *notes.Store, ref string) (*notes.Note, error) {
	all, err := s.List(notes.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, n := range all {
		if filepath.ToSlash(n.RelPath) == filepath.ToSlash(ref) {
			return n, nil
		}
	}
	return s.Get(ref)
}
func resolveProjectNote(s *notes.Store, link projectLink) (*notes.Note, error) {
	n, err := resolveNote(s, link.NoteID)
	if err == nil {
		return n, nil
	}
	if link.NotePath != "" {
		return resolveNote(s, link.NotePath)
	}
	return nil, err
}
func printNotes(cmd *cobra.Command, all []*notes.Note) {
	for _, n := range all {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", n.RelPath, n.Title)
	}
	if len(all) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No notes found.")
	}
}
func inFolder(n *notes.Note, folder string) bool {
	path := strings.ToLower(filepath.ToSlash(n.RelPath))
	folder = strings.ToLower(strings.Trim(filepath.ToSlash(folder), "/"))
	return strings.HasPrefix(path, folder+"/")
}

func newCaptureCommand() *cobra.Command {
	return &cobra.Command{Use: "capture [text]", Short: "Capture text into Inbox without prompts or opening an editor", RunE: func(cmd *cobra.Command, args []string) error {
		content := strings.Join(args, " ")
		if len(args) == 0 {
			info, err := os.Stdin.Stat()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("provide text or pipe content into neuron capture")
			}
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return err
			}
			content = string(raw)
		}
		content = strings.TrimSpace(content)
		if content == "" {
			return fmt.Errorf("capture cannot be empty")
		}
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		title := []rune(strings.SplitN(content, "\n", 2)[0])
		if len(title) > 80 {
			title = title[:80]
		}
		// Slashes in captured text are ordinary title text, never folder instructions.
		n, err := s.Create("Inbox", strings.ReplaceAll(strings.ReplaceAll(string(title), "/", "-"), "\\", "-"), nil, content+"\n")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), n.RelPath)
		return nil
	}}
}
func newInboxCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "inbox", Short: "Review captured notes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		all, err := s.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		var inbox []*notes.Note
		for _, n := range all {
			if inFolder(n, "Inbox") {
				inbox = append(inbox, n)
			}
		}
		printNotes(cmd, inbox)
		return nil
	}}
	cmd.AddCommand(&cobra.Command{Use: "file <note> <folder>", Short: "Move an Inbox note into its destination folder", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		n, err := resolveNote(s, args[0])
		if err != nil {
			return err
		}
		if !inFolder(n, "Inbox") {
			return fmt.Errorf("note is not in Inbox")
		}
		if err = s.Move(n.ID, args[1]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Filed:", n.Title)
		return nil
	}})
	return cmd
}
func newTasksCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "tasks", Short: "List Markdown tasks with source paths and line numbers", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		all, err := s.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		showAll, _ := cmd.Flags().GetBool("all")
		folder, _ := cmd.Flags().GetString("folder")
		count := 0
		for _, n := range all {
			if folder != "" && !inFolder(n, folder) {
				continue
			}
			for _, task := range notes.Tasks(n) {
				if task.Done && !showAll {
					continue
				}
				mark := " "
				if task.Done {
					mark = "x"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s:%d  %s\n", mark, n.RelPath, task.Line, task.Text)
				count++
			}
		}
		if count == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No tasks found.")
		}
		return nil
	}}
	cmd.Flags().Bool("all", false, "Include completed tasks")
	cmd.Flags().String("folder", "", "Filter tasks by vault folder")
	for _, action := range []string{"done", "reopen", "open"} {
		action := action
		cmd.AddCommand(&cobra.Command{Use: action + " <note> <line>", Short: action + " a task in its source note", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s, err := productivityStore()
			if err != nil {
				return err
			}
			line, err := strconv.Atoi(args[1])
			if err != nil || line < 1 {
				return fmt.Errorf("line must be a positive integer")
			}
			n, err := resolveNote(s, args[0])
			if err != nil {
				return err
			}
			for _, task := range notes.Tasks(n) {
				if task.Line == line {
					if action == "open" {
						return openProductivityNote(cfg, n)
					}
					if err := notes.SetTaskDone(task, action == "done"); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), "Updated:", n.RelPath+":"+args[1])
					return nil
				}
			}
			return fmt.Errorf("no task at line %d", line)
		}})
	}
	return cmd
}

// Filter syntax accepts shell-like quoted values, e.g. folder:"1. Projects".
func filterQuery(all []*notes.Note, query string) ([]*notes.Note, string, error) {
	words, err := splitQuery(query)
	if err != nil {
		return nil, "", err
	}
	var text []string
	for _, word := range words {
		key, value, ok := strings.Cut(word, ":")
		if !ok || (key != "tag" && key != "folder") {
			text = append(text, word)
			continue
		}
		if value == "" {
			return nil, "", fmt.Errorf("%s filter requires a value", key)
		}
		var matched []*notes.Note
		for _, n := range all {
			yes := false
			if key == "folder" {
				yes = inFolder(n, value)
			} else {
				for _, tag := range n.Tags {
					if strings.EqualFold(tag, strings.TrimPrefix(value, "#")) {
						yes = true
					}
				}
			}
			if yes {
				matched = append(matched, n)
			}
		}
		all = matched
	}
	return all, strings.Join(text, " "), nil
}
func splitQuery(input string) ([]string, error) {
	var words []string
	var b strings.Builder
	var quote rune
	for _, r := range input {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\n' || r == '\t' {
			if b.Len() > 0 {
				words = append(words, b.String())
				b.Reset()
			}
		} else {
			b.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote in query")
	}
	if b.Len() > 0 {
		words = append(words, b.String())
	}
	return words, nil
}
func runSavedSearch(cmd *cobra.Command, s *notes.Store, query string, limit int) error {
	all, err := s.List(notes.ListOptions{})
	if err != nil {
		return err
	}
	all, text, err := filterQuery(all, query)
	if err != nil {
		return err
	}
	if text != "" {
		idx := search.NewIndex()
		idx.Rebuild(all)
		results := idx.Search(text, limit)
		all = nil
		for _, r := range results {
			all = append(all, r.Note)
		}
	} else if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	printNotes(cmd, all)
	return nil
}
func newSearchCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "search [query]", Short: "Search notes with tag: and folder: filters", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		return runSavedSearch(cmd, s, strings.Join(args, " "), limit)
	}}
	cmd.Flags().Int("limit", 50, "Maximum results (0 for all)")
	cmd.AddCommand(&cobra.Command{Use: "save <name> <query>", Short: "Save or replace a named search", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		query := strings.Join(args[1:], " ")
		if _, _, err = filterQuery(nil, query); err != nil {
			return err
		}
		state, err := readWorkspace(s)
		if err != nil {
			return err
		}
		state.Searches[args[0]] = query
		if err = saveWorkspace(s, state); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Saved:", args[0])
		return nil
	}})
	for _, action := range []string{"list", "run", "remove"} {
		action := action
		count := 1
		if action == "list" {
			count = 0
		}
		cmd.AddCommand(&cobra.Command{Use: action + map[bool]string{true: "", false: " <name>"}[count == 0], Short: action + " saved searches", Args: cobra.ExactArgs(count), RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := productivityStore()
			if err != nil {
				return err
			}
			state, err := readWorkspace(s)
			if err != nil {
				return err
			}
			if action == "list" {
				names := make([]string, 0, len(state.Searches))
				for name := range state.Searches {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", name, state.Searches[name])
				}
				if len(names) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No saved searches.")
				}
				return nil
			}
			query, ok := state.Searches[args[0]]
			if !ok {
				return fmt.Errorf("saved search %q not found", args[0])
			}
			if action == "run" {
				return runSavedSearch(cmd, s, query, 50)
			}
			delete(state.Searches, args[0])
			return saveWorkspace(s, state)
		}})
	}
	return cmd
}
func repositoryRoot() (string, error) {
	raw, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("run this command inside a Git repository")
	}
	return filepath.EvalSymlinks(strings.TrimSpace(string(raw)))
}
func newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Open the context note linked to the current Git repository", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, s, err := productivityStore()
		if err != nil {
			return err
		}
		repo, err := repositoryRoot()
		if err != nil {
			return err
		}
		state, err := readWorkspace(s)
		if err != nil {
			return err
		}
		link, ok := state.Projects[repo]
		if !ok {
			return fmt.Errorf("no project linked; run neuron project init or neuron project link <note>")
		}
		n, err := resolveProjectNote(s, link)
		if err != nil {
			return err
		}
		return openProductivityNote(cfg, n)
	}}
	for _, action := range []string{"init", "link", "unlink", "list"} {
		action := action
		c := &cobra.Command{Use: action, Short: action + " project context", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			_, s, err := productivityStore()
			if err != nil {
				return err
			}
			state, err := readWorkspace(s)
			if err != nil {
				return err
			}
			if action == "list" {
				repos := make([]string, 0, len(state.Projects))
				for repo := range state.Projects {
					repos = append(repos, repo)
				}
				sort.Strings(repos)
				for _, repo := range repos {
					link := state.Projects[repo]
					n, e := resolveProjectNote(s, link)
					title := link.NoteID
					if e == nil {
						title = n.Title
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", repo, title)
				}
				return nil
			}
			repo, err := repositoryRoot()
			if err != nil {
				return err
			}
			if action == "unlink" {
				delete(state.Projects, repo)
				return saveWorkspace(s, state)
			}
			var n *notes.Note
			if action == "link" {
				n, err = resolveNote(s, args[0])
			} else {
				if _, exists := state.Projects[repo]; exists {
					return fmt.Errorf("repository already linked; use neuron project to open it")
				}
				title, _ := cmd.Flags().GetString("name")
				if title == "" {
					title = filepath.Base(repo)
				}
				folder := "Projects"
				for _, f := range s.DetectPARAFolders() {
					if strings.Contains(strings.ToLower(f), "project") {
						folder = f
						break
					}
				}
				n, err = s.Create(folder, title, []string{"project"}, "## Objective\n\n## Next steps\n- [ ] Define the next action\n\n## Decisions\n\n## Related notes\n")
			}
			if err != nil {
				return err
			}
			state.Projects[repo] = projectLink{repo, n.ID, n.RelPath}
			if err = saveWorkspace(s, state); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Linked:", n.RelPath)
			return nil
		}}
		if action == "link" {
			c.Use = "link <note>"
			c.Args = cobra.ExactArgs(1)
		}
		if action == "init" {
			c.Flags().String("name", "", "Project title (defaults to repository name)")
		}
		cmd.AddCommand(c)
	}
	return cmd
}
func newDashboardCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "dashboard", Short: "Show today's tasks, Inbox, active projects and recent notes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, s, err := productivityStore()
		if err != nil {
			return err
		}
		all, err := s.List(notes.ListOptions{})
		if err != nil {
			return err
		}
		state, err := readWorkspace(s)
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		if limit < 1 {
			return fmt.Errorf("limit must be positive")
		}
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "TODAY ·", time.Now().Format("2006-01-02"))
		fmt.Fprintln(out, "Open daily note: neuron today")
		fmt.Fprintln(out, "\nPENDING TASKS")
		count, total := 0, 0
		for _, n := range all {
			for _, task := range notes.Tasks(n) {
				if !task.Done {
					total++
					if count < limit {
						fmt.Fprintf(out, "[ ] %s:%d  %s\n", n.RelPath, task.Line, task.Text)
						count++
					}
				}
			}
		}
		fmt.Fprintf(out, "%d pending · neuron tasks\n", total)
		fmt.Fprintln(out, "\nINBOX")
		count, total = 0, 0
		for _, n := range all {
			if inFolder(n, "Inbox") {
				total++
				if count < limit {
					fmt.Fprintln(out, n.RelPath)
					count++
				}
			}
		}
		fmt.Fprintf(out, "%d to process · neuron inbox\n", total)
		fmt.Fprintln(out, "\nACTIVE PROJECTS")
		count = 0
		for _, n := range all {
			active := false
			for _, link := range state.Projects {
				if link.NoteID == n.ID || link.NotePath == n.RelPath {
					active = true
				}
			}
			if active && count < limit {
				fmt.Fprintln(out, n.RelPath)
				count++
			}
		}
		if count == 0 {
			fmt.Fprintln(out, "No linked projects · neuron project init")
		}
		fmt.Fprintln(out, "\nRECENT NOTES")
		if len(all) > limit {
			all = all[:limit]
		}
		printNotes(cmd, all)
		return nil
	}}
	cmd.Flags().Int("limit", 5, "Items per dashboard section")
	return cmd
}
func init() {
	for _, cmd := range []*cobra.Command{newCaptureCommand(), newInboxCommand(), newTasksCommand(), newDashboardCommand(), newProjectCommand(), newSearchCommand()} {
		applyProductivityHelp(cmd, cmd.Name())
		rootCmd.AddCommand(cmd)
	}
}
