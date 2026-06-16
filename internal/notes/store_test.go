package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreCreateAndMove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "neuron-test-vault-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Create a note in the root
	note1, err := store.Create("", "Test Note 1", []string{"tag1"}, "Hello World 1")
	if err != nil {
		t.Fatalf("failed to create note 1: %v", err)
	}
	if note1.RelPath != "test-note-1.md" {
		t.Errorf("expected RelPath to be test-note-1.md, got %q", note1.RelPath)
	}

	// 2. Create a note in a subdirectory
	note2, err := store.Create("1. Projects", "Test Project Note", []string{"work"}, "Project details")
	if err != nil {
		t.Fatalf("failed to create project note: %v", err)
	}
	expectedRelPath := filepath.Join("1. Projects", "test-project-note.md")
	if note2.RelPath != expectedRelPath {
		t.Errorf("expected RelPath to be %q, got %q", expectedRelPath, note2.RelPath)
	}

	expectedFullPath := filepath.Join(tmpDir, expectedRelPath)
	if _, err := os.Stat(expectedFullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %q", expectedFullPath)
	}

	// 3. Move the note
	err = store.Move(note2.ID, "4. Archive")
	if err != nil {
		t.Fatalf("failed to move note: %v", err)
	}

	if _, err := os.Stat(expectedFullPath); err == nil {
		t.Errorf("file should not exist at old path %q anymore", expectedFullPath)
	}

	newExpectedRelPath := filepath.Join("4. Archive", "test-project-note.md")
	newExpectedFullPath := filepath.Join(tmpDir, newExpectedRelPath)
	if _, err := os.Stat(newExpectedFullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %q after move", newExpectedFullPath)
	}

	fetched, err := store.Get(note2.ID)
	if err != nil {
		t.Fatalf("failed to fetch moved note: %v", err)
	}
	if fetched.RelPath != newExpectedRelPath {
		t.Errorf("expected RelPath after get to be %q, got %q", newExpectedRelPath, fetched.RelPath)
	}
}

func TestStoreDetectPARAFolders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "neuron-test-vault-detect-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	customFolders := []string{"My Projects", "Personal Areas", "Useful Resources", "The Archive"}
	for _, folder := range customFolders {
		err := os.MkdirAll(filepath.Join(tmpDir, folder), 0o700)
		if err != nil {
			t.Fatalf("failed to create folder: %v", err)
		}
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	detected := store.DetectPARAFolders()
	if len(detected) != 4 {
		t.Fatalf("expected 4 detected folders, got %d", len(detected))
	}

	expectedMatches := map[string]string{
		"project":  "My Projects",
		"area":     "Personal Areas",
		"resource": "Useful Resources",
		"archive":  "The Archive",
	}

	for _, det := range detected {
		matched := false
		lower := strings.ToLower(det)
		for key, val := range expectedMatches {
			if strings.Contains(lower, key) {
				if det != val {
					t.Errorf("expected match for %q to be %q, got %q", key, val, det)
				}
				matched = true
			}
		}
		if !matched {
			t.Errorf("detected folder %q didn't match any expected PARA pattern", det)
		}
	}
}

func TestStoreCacheBehavior(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// IsCacheStale returns true when nothing has been cached yet
	if !store.IsCacheStale() {
		t.Error("expected IsCacheStale to be true before any List()")
	}

	// Create a note (invalidates cache internally)
	_, err = store.Create("", "New Note", nil, "content")
	if err != nil {
		t.Fatal(err)
	}

	// List populates cache
	notes, err := store.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}

	// IsCacheStale returns false when nothing changed
	if store.IsCacheStale() {
		t.Error("expected IsCacheStale to be false immediately after List()")
	}

	// Simulate external change by modifying a file's mod time
	note := notes[0]
	os.Chtimes(note.Path, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	if !store.IsCacheStale() {
		t.Error("expected IsCacheStale to be true after external mod")
	}

	// Calling List again re-scans and returns fresh data
	notes, err = store.List(ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Errorf("expected 1 note after re-scan, got %d", len(notes))
	}
	if store.IsCacheStale() {
		t.Error("expected IsCacheStale to be false after fresh List()")
	}
}

func TestStoreNeuronDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	neuronDir := store.NeuronDir()
	if neuronDir != filepath.Join(dir, ".neuron") {
		t.Errorf("NeuronDir() = %q, want %q", neuronDir, filepath.Join(dir, ".neuron"))
	}

	if store.CacheDir() != filepath.Join(dir, ".neuron", "cache") {
		t.Errorf("unexpected CacheDir: %q", store.CacheDir())
	}

	if store.EmbedDir() != filepath.Join(dir, ".neuron", "chromem") {
		t.Errorf("unexpected EmbedDir: %q", store.EmbedDir())
	}
}

func TestStoreListOptions(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = store.Create("", "Alpha", []string{"a"}, "")
	_, _ = store.Create("", "Beta", []string{"b"}, "")
	_, _ = store.Create("", "Gamma", []string{"a", "b"}, "")

	tests := []struct {
		name string
		opts ListOptions
		want int
	}{
		{"no filter", ListOptions{}, 3},
		{"tag a", ListOptions{Tags: []string{"a"}}, 2},
		{"tag b", ListOptions{Tags: []string{"b"}}, 2},
		{"tag a and b", ListOptions{Tags: []string{"a", "b"}}, 1},
		{"query alpha", ListOptions{Query: "alpha"}, 1},
		{"limit 1", ListOptions{Limit: 1}, 1},
		{"query nonexistent", ListOptions{Query: "zzz"}, 0},
		{"tag nonexistent", ListOptions{Tags: []string{"zzz"}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notes, err := store.List(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(notes) != tt.want {
				t.Errorf("got %d notes, want %d", len(notes), tt.want)
			}
		})
	}
}

func TestStoreSortBy(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	n1, _ := store.Create("", "C Note", nil, "")
	time.Sleep(10 * time.Millisecond)
	n2, _ := store.Create("", "B Note", nil, "")
	time.Sleep(10 * time.Millisecond)
	n3, _ := store.Create("", "A Note", nil, "")

	// Default sort is by updated (descending): A, B, C
	notes, _ := store.List(ListOptions{SortBy: ""})
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	if notes[0].ID != n3.ID {
		t.Errorf("expected latest note first, got %s", notes[0].Title)
	}

	// Sort by title
	notes, _ = store.List(ListOptions{SortBy: "title"})
	if notes[0].Title != "A Note" {
		t.Errorf("expected 'A Note' first when sorting by title, got %q", notes[0].Title)
	}
	if notes[2].ID != n1.ID {
		t.Errorf("expected 'C Note' last when sorting by title, got %q", notes[2].Title)
	}

	_ = n1
	_ = n2
}

func TestStoreUpdateReload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	note, err := store.Create("", "Original", nil, "Original content")
	if err != nil {
		t.Fatal(err)
	}

	note.Content = "Updated content"
	if err := store.Update(note); err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Reload(note)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Content != "Updated content" {
		t.Errorf("Content = %q, want %q", reloaded.Content, "Updated content")
	}
}

func TestStoreTags(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = store.Create("", "Note1", []string{"go", "test"}, "")
	_, _ = store.Create("", "Note2", []string{"python", "test"}, "")
	_, _ = store.Create("", "Note3", []string{"go"}, "")

	tags, err := store.Tags()
	if err != nil {
		t.Fatal(err)
	}

	if tags["go"] != 2 {
		t.Errorf("expected 'go' count to be 2, got %d", tags["go"])
	}
	if tags["test"] != 2 {
		t.Errorf("expected 'test' count to be 2, got %d", tags["test"])
	}
	if tags["python"] != 1 {
		t.Errorf("expected 'python' count to be 1, got %d", tags["python"])
	}
}

func TestStoreCount(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	_, _ = store.Create("", "N1", nil, "")
	_, _ = store.Create("", "N2", nil, "")

	count, _ = store.Count()
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	note, err := store.Create("", "Delete Me", nil, "bye")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(note.ID); err != nil {
		t.Fatal(err)
	}

	// Should be in trash
	trashDir := filepath.Join(dir, ".trash")
	if _, err := os.Stat(trashDir); os.IsNotExist(err) {
		t.Error("trash directory should exist")
	}

	// Should not show up in list
	notes, _ := store.List(ListOptions{})
	if len(notes) != 0 {
		t.Errorf("expected 0 notes after delete, got %d", len(notes))
	}
}

func TestStoreGetByPath(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	note, err := store.Create("", "Hello World", nil, "content")
	if err != nil {
		t.Fatal(err)
	}

	// Get by ID
	byID, err := store.Get(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if byID.Title != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", byID.Title)
	}

	// Get by title
	byTitle, err := store.Get("Hello World")
	if err != nil {
		t.Fatal(err)
	}
	if byTitle.ID != note.ID {
		t.Errorf("expected ID %q, got %q", note.ID, byTitle.ID)
	}

	// Get by filename
	byFile, err := store.Get("hello-world")
	if err != nil {
		t.Fatal(err)
	}
	if byFile.ID != note.ID {
		t.Errorf("expected ID %q, got %q", note.ID, byFile.ID)
	}

	// Not found
	_, err = store.Get("nonexistent")
	if err != ErrNoteNotFound {
		t.Errorf("expected ErrNoteNotFound, got %v", err)
	}
}
