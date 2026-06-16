package search

import (
	"os"
	"testing"
	"time"

	"github.com/steevin/neuron-cli/internal/notes"
)

func makeNote(id, title, content string, tags []string) *notes.Note {
	return &notes.Note{
		ID:      id,
		Title:   title,
		Content: content,
		Tags:    tags,
		Created: time.Now(),
		Updated: time.Now(),
	}
}

func TestIndexSearch(t *testing.T) {
	idx := NewIndex()
	idx.Rebuild([]*notes.Note{
		makeNote("1", "Go Programming", "Go is a statically typed language", []string{"go", "programming"}),
		makeNote("2", "Python Scripting", "Python is great for scripting", []string{"python", "scripting"}),
		makeNote("3", "Rust Systems", "Rust is a systems language", []string{"rust", "systems"}),
	})

	tests := []struct {
		query string
		want  int // minimum expected results
	}{
		{query: "go", want: 1},
		{query: "python", want: 1},
		{query: "language", want: 2},
		{query: "nonexistent", want: 0},
		{query: "", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := idx.Search(tt.query, 10)
			if len(results) < tt.want {
				t.Errorf("Search(%q) returned %d results, want >= %d", tt.query, len(results), tt.want)
			}
		})
	}
}

func TestIndexSearchRanking(t *testing.T) {
	idx := NewIndex()
	idx.Rebuild([]*notes.Note{
		makeNote("1", "Go Guide", "A comprehensive guide to Go programming language and its ecosystem", []string{}),
		makeNote("2", "Random Notes", "Nothing about go here, just random thoughts", []string{}),
	})

	results := idx.Search("go", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// "Go Guide" should rank higher since "go" is in the title (weighted 3x)
	if results[0].Note.ID != "1" {
		t.Errorf("expected 'Go Guide' to rank first, got %s (score=%.2f)", results[0].Note.Title, results[0].Score)
	}
}

func TestIndexTagBoost(t *testing.T) {
	idx := NewIndex()
	idx.Rebuild([]*notes.Note{
		makeNote("1", "Meeting Notes", "Discussed project timeline", []string{"work", "meeting"}),
		makeNote("2", "Random Thoughts", "Just some meeting ideas", []string{}),
	})

	results := idx.Search("meeting", 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// "Meeting Notes" should rank higher because "meeting" is in title (3x) and as a tag (2x)
	if results[0].Note.ID != "1" {
		t.Errorf("expected 'Meeting Notes' to rank first, got %s", results[0].Note.Title)
	}
}

func TestIndexSaveLoad(t *testing.T) {
	dir := t.TempDir()

	idx := NewIndex()
	idx.Rebuild([]*notes.Note{
		makeNote("1", "Test Save", "This is a test note for save/load", []string{"test"}),
		makeNote("2", "Another", "Another note for testing", []string{"example"}),
	})

	if err := idx.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := NewIndex()
	if err := loaded.Load(dir); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	results := loaded.Search("test", 10)
	if len(results) == 0 {
		t.Fatal("expected results from loaded index")
	}
	if results[0].Note.ID != "1" {
		t.Errorf("expected note '1', got %s", results[0].Note.ID)
	}
}

func TestCacheKeyConsistency(t *testing.T) {
	now := time.Now()
	notes1 := []*notes.Note{
		{Path: "/vault/a.md", Updated: now},
		{Path: "/vault/b.md", Updated: now},
	}
	notes2 := []*notes.Note{
		{Path: "/vault/a.md", Updated: now},
		{Path: "/vault/b.md", Updated: now},
	}
	notes3 := []*notes.Note{
		{Path: "/vault/a.md", Updated: now.Add(time.Hour)}, // different mod time
		{Path: "/vault/b.md", Updated: now},
	}

	key1 := CacheKey(notes1)
	key2 := CacheKey(notes2)
	key3 := CacheKey(notes3)

	if key1 != key2 {
		t.Errorf("same notes should produce same key: %q vs %q", key1, key2)
	}
	if key1 == key3 {
		t.Errorf("different mod times should produce different keys")
	}
}

func TestRebuildWithCache(t *testing.T) {
	dir := t.TempDir()
	noteList := []*notes.Note{
		makeNote("1", "Cached Note", "Content for cache test", []string{"cache"}),
	}

	// First call: rebuilds and saves
	idx1, err := RebuildWithCache(dir, noteList)
	if err != nil {
		t.Fatalf("first RebuildWithCache failed: %v", err)
	}
	r1 := idx1.Search("cached", 10)
	if len(r1) != 1 {
		t.Fatalf("expected 1 result, got %d", len(r1))
	}

	// Verify cache file was created
	if _, err := os.Stat(dir + "/index.json"); os.IsNotExist(err) {
		t.Error("cache file not created")
	}
	if _, err := os.Stat(dir + "/manifest"); os.IsNotExist(err) {
		t.Error("manifest file not created")
	}

	// Second call: should load from cache
	idx2, err := RebuildWithCache(dir, noteList)
	if err != nil {
		t.Fatalf("second RebuildWithCache failed: %v", err)
	}
	r2 := idx2.Search("cached", 10)
	if len(r2) != 1 {
		t.Fatalf("expected 1 result from cached index, got %d", len(r2))
	}
}

func TestIndexRemoveNote(t *testing.T) {
	idx := NewIndex()
	idx.Rebuild([]*notes.Note{
		makeNote("1", "First", "Content one", nil),
		makeNote("2", "Second", "Content two", nil),
	})

	idx.RemoveNote("1")

	results := idx.Search("content", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 result after removal, got %d", len(results))
	}
	if results[0].Note.ID != "2" {
		t.Errorf("expected note '2', got %s", results[0].Note.ID)
	}
}

func TestIndexIndexNote(t *testing.T) {
	idx := NewIndex()
	note := makeNote("1", "Dynamic", "Dynamic content added later", []string{"late"})
	idx.IndexNote(note)

	results := idx.Search("dynamic", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
