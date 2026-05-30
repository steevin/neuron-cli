package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

// ErrNoteNotFound is returned when a note cannot be located by ID or title.
var ErrNoteNotFound = fmt.Errorf("note not found")

// ---------------------------------------------------------------------------
// ListOptions
// ---------------------------------------------------------------------------

// ListOptions controls filtering, sorting, and pagination for Store.List.
type ListOptions struct {
	Tags   []string // Filter by tags (AND logic — note must have ALL tags)
	Query  string   // Case-insensitive substring match against note title
	Limit  int      // Maximum number of results; 0 means no limit
	SortBy string   // "updated" (default), "created", or "title"
}

// ---------------------------------------------------------------------------
// Store
// ---------------------------------------------------------------------------

// Store is a file-based note store rooted at a vault directory.
type Store struct {
	VaultPath string        // Absolute path to the vault root
	Vault     *ObsidianVault // Detected Obsidian metadata (may be non-Obsidian)
}

// NewStore creates (if necessary) and returns a Store for the given vault path.
// A leading "~" in vaultPath is expanded to the user's home directory.
func NewStore(vaultPath string) (*Store, error) {
	// Expand tilde.
	if strings.HasPrefix(vaultPath, "~/") || vaultPath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("notes: resolving home directory: %w", err)
		}
		vaultPath = home + vaultPath[1:]
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(vaultPath, 0o755); err != nil {
		return nil, fmt.Errorf("notes: creating vault directory %q: %w", vaultPath, err)
	}

	vault, err := DetectObsidianVault(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("notes: detecting obsidian vault: %w", err)
	}

	return &Store{
		VaultPath: vaultPath,
		Vault:     vault,
	}, nil
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create creates a new note with the given title, tags, and body content.
// It generates a UUID, derives a safe filename from the title, writes the file,
// and returns the populated Note.
func (s *Store) Create(title string, tags []string, content string) (*Note, error) {
	id := uuid.New().String()
	filename := safeFilename(title) + ".md"
	fullPath := filepath.Join(s.VaultPath, filename)

	// Avoid overwriting an existing file by appending a suffix.
	if _, err := os.Stat(fullPath); err == nil {
		base := strings.TrimSuffix(filename, ".md")
		fullPath = filepath.Join(s.VaultPath, fmt.Sprintf("%s-%d.md", base, time.Now().UnixNano()))
	}

	now := time.Now()
	note := &Note{
		ID:      id,
		Title:   title,
		Path:    fullPath,
		RelPath: filepath.Base(fullPath),
		Content: content,
		Tags:    tags,
		Created: now,
		Updated: now,
		Extra:   make(map[string]interface{}),
	}

	note.RawContent = ToMarkdown(note)

	if err := os.WriteFile(fullPath, []byte(note.RawContent), 0o644); err != nil {
		return nil, fmt.Errorf("notes: writing note %q: %w", fullPath, err)
	}

	return note, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get retrieves a note by UUID (exact match), then by title (case-insensitive),
// then by filename stem. Returns ErrNoteNotFound when no match exists.
func (s *Store) Get(idOrTitle string) (*Note, error) {
	all, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(idOrTitle)

	// Pass 1: exact UUID match.
	for _, n := range all {
		if n.ID == idOrTitle {
			return n, nil
		}
	}

	// Pass 2: case-insensitive title match.
	for _, n := range all {
		if strings.ToLower(n.Title) == lowerQuery {
			return n, nil
		}
	}

	// Pass 3: filename stem match.
	for _, n := range all {
		stem := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		if strings.ToLower(stem) == lowerQuery {
			return n, nil
		}
	}

	return nil, ErrNoteNotFound
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List walks the vault directory, parses every .md file it finds, applies the
// filters specified in opts, sorts the results, and returns the slice.
func (s *Store) List(opts ListOptions) ([]*Note, error) {
	var notes []*Note

	err := filepath.WalkDir(s.VaultPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable entries rather than aborting the whole walk.
			return nil
		}

		// Skip hidden directories and Obsidian system directories.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ".obsidian" || name == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process markdown files.
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		// Skip Obsidian system files just in case the walk enters them.
		if IsObsidianFile(path) {
			return nil
		}

		note, parseErr := ParseFile(path)
		if parseErr != nil {
			// Malformed files are skipped silently.
			return nil
		}

		// Compute relative path.
		rel, relErr := filepath.Rel(s.VaultPath, path)
		if relErr == nil {
			note.RelPath = rel
		}

		notes = append(notes, note)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("notes: walking vault %q: %w", s.VaultPath, err)
	}

	// ---------------------------------------------------------------- filter
	filtered := make([]*Note, 0, len(notes))
	for _, n := range notes {
		if !matchesListOptions(n, opts) {
			continue
		}
		filtered = append(filtered, n)
	}

	// ------------------------------------------------------------------ sort
	sortNotes(filtered, opts.SortBy)

	// ----------------------------------------------------------------- limit
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}

	return filtered, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update sets note.Updated to the current time and rewrites the note's file.
func (s *Store) Update(note *Note) error {
	note.Updated = time.Now()
	note.RawContent = ToMarkdown(note)

	if err := os.WriteFile(note.Path, []byte(note.RawContent), 0o644); err != nil {
		return fmt.Errorf("notes: writing note %q: %w", note.Path, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// Delete moves the note identified by idOrTitle to the vault's .trash folder
// instead of permanently removing it.
func (s *Store) Delete(idOrTitle string) error {
	note, err := s.Get(idOrTitle)
	if err != nil {
		return err
	}

	trashDir := filepath.Join(s.VaultPath, ".trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return fmt.Errorf("notes: creating .trash directory: %w", err)
	}

	dest := filepath.Join(trashDir, filepath.Base(note.Path))
	// Avoid collisions in .trash by appending a timestamp.
	if _, statErr := os.Stat(dest); statErr == nil {
		stem := strings.TrimSuffix(filepath.Base(note.Path), ".md")
		dest = filepath.Join(trashDir, fmt.Sprintf("%s-%d.md", stem, time.Now().UnixNano()))
	}

	if err := os.Rename(note.Path, dest); err != nil {
		return fmt.Errorf("notes: moving note to trash: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Count
// ---------------------------------------------------------------------------

// Count returns the total number of .md files in the vault (excluding system
// folders).
func (s *Store) Count() (int, error) {
	notes, err := s.List(ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(notes), nil
}

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

// Tags returns a map of tag name → number of notes that carry that tag.
func (s *Store) Tags() (map[string]int, error) {
	notes, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, n := range notes {
		for _, tag := range n.Tags {
			counts[tag]++
		}
	}
	return counts, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// safeFilename converts a note title into a safe, lowercase, hyphen-separated
// filename stem (max 80 characters).
func safeFilename(title string) string {
	var sb strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen && sb.Len() > 0 {
			sb.WriteByte('-')
			prevHyphen = true
		}
	}
	s := strings.TrimRight(sb.String(), "-")
	if len(s) > 80 {
		s = s[:80]
		// Don't end on a hyphen.
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = fmt.Sprintf("note-%d", time.Now().UnixNano())
	}
	return s
}

// matchesListOptions reports whether a note satisfies all ListOptions filters.
func matchesListOptions(n *Note, opts ListOptions) bool {
	// Tag filter (AND logic).
	if len(opts.Tags) > 0 {
		noteTagSet := make(map[string]struct{}, len(n.Tags))
		for _, t := range n.Tags {
			noteTagSet[strings.ToLower(t)] = struct{}{}
		}
		for _, required := range opts.Tags {
			if _, ok := noteTagSet[strings.ToLower(required)]; !ok {
				return false
			}
		}
	}

	// Title substring filter.
	if opts.Query != "" {
		if !strings.Contains(strings.ToLower(n.Title), strings.ToLower(opts.Query)) {
			return false
		}
	}

	return true
}

// sortNotes sorts notes in place according to the requested field.
func sortNotes(notes []*Note, by string) {
	switch by {
	case "created":
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].Created.After(notes[j].Created)
		})
	case "title":
		sort.Slice(notes, func(i, j int) bool {
			return strings.ToLower(notes[i].Title) < strings.ToLower(notes[j].Title)
		})
	default: // "updated" and anything else
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].Updated.After(notes[j].Updated)
		})
	}
}
