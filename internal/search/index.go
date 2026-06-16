package search

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/steevin/neuron-cli/internal/notes"
)

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "in": true,
	"of": true, "to": true, "and": true, "or": true, "for": true,
	"with": true, "it": true, "at": true, "by": true, "on": true,
	"as": true, "be": true, "was": true, "are": true, "has": true,
	"that": true, "this": true, "from": true, "not": true, "but": true,
}

type docEntry struct {
	note *notes.Note
	tf   map[string]float64
}

type Index struct {
	mu       sync.RWMutex
	inverted map[string]map[string]bool
	docs     map[string]*docEntry
}

type SearchResult struct {
	Note  *notes.Note
	Score float64
}

func NewIndex() *Index {
	return &Index{
		inverted: make(map[string]map[string]bool),
		docs:     make(map[string]*docEntry),
	}
}

func (idx *Index) Rebuild(noteList []*notes.Note) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.inverted = make(map[string]map[string]bool)
	idx.docs = make(map[string]*docEntry)

	for _, n := range noteList {
		idx.indexNote(n)
	}
}

func (idx *Index) IndexNote(note *notes.Note) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeNote(note.ID)
	idx.indexNote(note)
}

func (idx *Index) RemoveNote(noteID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeNote(noteID)
}

func (idx *Index) Search(query string, limit int) []*SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	N := float64(len(idx.docs))
	if N == 0 {
		return nil
	}

	scores := make(map[string]float64)

	for _, tok := range tokens {
		postings, ok := idx.inverted[tok]
		if !ok {
			continue
		}
		df := float64(len(postings))
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)

		for id := range postings {
			entry, ok := idx.docs[id]
			if !ok {
				continue
			}
			tf := entry.tf[tok]
			scores[id] += tf * idf
		}
	}

	results := make([]*SearchResult, 0, len(scores))
	for id, score := range scores {
		entry, ok := idx.docs[id]
		if !ok {
			continue
		}
		results = append(results, &SearchResult{Note: entry.note, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func (idx *Index) indexNote(note *notes.Note) {
	tf := make(map[string]float64)

	for _, tok := range tokenize(note.Title) {
		tf[tok] += 3.0
	}
	for _, tag := range note.Tags {
		for _, tok := range tokenize(tag) {
			tf[tok] += 2.0
		}
	}
	for _, tok := range tokenize(note.Content) {
		tf[tok] += 1.0
	}

	idx.docs[note.ID] = &docEntry{note: note, tf: tf}

	for tok := range tf {
		if idx.inverted[tok] == nil {
			idx.inverted[tok] = make(map[string]bool)
		}
		idx.inverted[tok][note.ID] = true
	}
}

func (idx *Index) removeNote(noteID string) {
	entry, ok := idx.docs[noteID]
	if !ok {
		return
	}
	for tok := range entry.tf {
		delete(idx.inverted[tok], noteID)
		if len(idx.inverted[tok]) == 0 {
			delete(idx.inverted, tok)
		}
	}
	delete(idx.docs, noteID)
}

// ─── Persistencia a disco ─────────────────────────────────────────────────────

type cacheNoteMeta struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags"`
	Content   string    `json:"body"`
	Filename  string    `json:"filename"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

type cacheEntry struct {
	Inverted map[string][]string          `json:"inverted"`
	DocTF    map[string]map[string]float64 `json:"tf"`
	Meta     map[string]cacheNoteMeta     `json:"meta"`
}

func (idx *Index) Save(cacheDir string) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entry := cacheEntry{
		Inverted: make(map[string][]string, len(idx.inverted)),
		DocTF:    make(map[string]map[string]float64, len(idx.docs)),
		Meta:     make(map[string]cacheNoteMeta, len(idx.docs)),
	}

	for tok, postings := range idx.inverted {
		ids := make([]string, 0, len(postings))
		for id := range postings {
			ids = append(ids, id)
		}
		entry.Inverted[tok] = ids
	}

	for id, doc := range idx.docs {
		entry.DocTF[id] = doc.tf
		entry.Meta[id] = cacheNoteMeta{
			ID:       doc.note.ID,
			Title:    doc.note.Title,
			Tags:     doc.note.Tags,
			Content:  doc.note.Content,
			Filename: doc.note.RelPath,
			Created:  doc.note.Created,
			Updated:  doc.note.Updated,
		}
	}

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return fmt.Errorf("search: creating cache dir: %w", err)
	}

	path := filepath.Join(cacheDir, "index.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("search: creating cache file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("search: encoding cache: %w", err)
	}
	return nil
}

func (idx *Index) Load(cacheDir string) error {
	path := filepath.Join(cacheDir, "index.json")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var entry cacheEntry
	dec := json.NewDecoder(f)
	if err := dec.Decode(&entry); err != nil {
		return fmt.Errorf("search: decoding cache: %w", err)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.inverted = make(map[string]map[string]bool, len(entry.Inverted))
	for tok, ids := range entry.Inverted {
		postings := make(map[string]bool, len(ids))
		for _, id := range ids {
			postings[id] = true
		}
		idx.inverted[tok] = postings
	}

	idx.docs = make(map[string]*docEntry, len(entry.DocTF))
	for id, tf := range entry.DocTF {
		meta := entry.Meta[id]
		note := &notes.Note{
			ID:      meta.ID,
			Title:   meta.Title,
			Tags:    meta.Tags,
			Content: meta.Content,
			RelPath: meta.Filename,
			Created: meta.Created,
			Updated: meta.Updated,
		}
		idx.docs[id] = &docEntry{note: note, tf: tf}
	}

	return nil
}

func CacheKey(noteList []*notes.Note) string {
	if len(noteList) == 0 {
		return "empty"
	}
	h := sha256.New()
	for _, n := range noteList {
		fmt.Fprintf(h, "%s:%d\n", n.Path, n.Updated.UnixNano())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, f := range fields {
		if len(f) < 2 || stopWords[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// RebuildWithCache rebuilds the index from the note list, using a disk cache
// when the note contents haven't changed. The cache key is derived from each
// note's path and modification time.
func RebuildWithCache(cacheDir string, noteList []*notes.Note) (*Index, error) {
	idx := NewIndex()

	key := CacheKey(noteList)
	manifestPath := filepath.Join(cacheDir, "manifest")

	// Check if we have a valid cache
	loadCache := false
	if data, err := os.ReadFile(manifestPath); err == nil {
		if string(data) == key {
			if err := idx.Load(cacheDir); err == nil {
				loadCache = true
			}
		}
	}

	if !loadCache {
		idx.Rebuild(noteList)
		if err := idx.Save(cacheDir); err != nil {
			// Non-fatal — index works in-memory either way
			_ = err
		}
		_ = os.WriteFile(manifestPath, []byte(key), 0o600)
	}

	return idx, nil
}
