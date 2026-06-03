package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// Verify file physically exists in the subdirectory
	expectedFullPath := filepath.Join(tmpDir, expectedRelPath)
	if _, err := os.Stat(expectedFullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %q", expectedFullPath)
	}

	// 3. Move the note to another subdirectory (e.g. 4. Archive)
	err = store.Move(note2.ID, "4. Archive")
	if err != nil {
		t.Fatalf("failed to move note: %v", err)
	}

	// Verify it moved physically
	oldPath := expectedFullPath
	if _, err := os.Stat(oldPath); err == nil {
		t.Errorf("file should not exist at old path %q anymore", oldPath)
	}

	newExpectedRelPath := filepath.Join("4. Archive", "test-project-note.md")
	newExpectedFullPath := filepath.Join(tmpDir, newExpectedRelPath)
	if _, err := os.Stat(newExpectedFullPath); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %q after move", newExpectedFullPath)
	}

	// Fetch it again to see if we can resolve it and if its path updated
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

	// Create custom PARA folder structure (unnumbered but simple names)
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
