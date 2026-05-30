// Package search provides full-text search over the neuron vault using an
// in-memory inverted index with BM25-like relevance scoring.
package search

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/steevin/neuron-cli/internal/notes"
)

// stopWords are common English words excluded from the index.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "in": true,
	"of": true, "to": true, "and": true, "or": true, "for": true,
	"with": true, "it": true, "at": true, "by": true, "on": true,
	"as": true, "be": true, "was": true, "are": true, "has": true,
	"that": true, "this": true, "from": true, "not": true, "but": true,
}

// docEntry holds per-document token frequency data.
type docEntry struct {
	note *notes.Note
	tf   map[string]float64 // token → weighted term frequency
}

// Index is a thread-safe in-memory inverted index.
type Index struct {
	mu       sync.RWMutex
	inverted map[string]map[string]bool // token → set of note IDs
	docs     map[string]*docEntry       // note ID → document data
}

// SearchResult pairs a note with its relevance score.
type SearchResult struct {
	Note  *notes.Note
	Score float64
}

// NewIndex creates an empty Index.
func NewIndex() *Index {
	return &Index{
		inverted: make(map[string]map[string]bool),
		docs:     make(map[string]*docEntry),
	}
}

// Rebuild replaces the entire index with the given note list.
func (idx *Index) Rebuild(noteList []*notes.Note) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.inverted = make(map[string]map[string]bool)
	idx.docs = make(map[string]*docEntry)

	for _, n := range noteList {
		idx.indexNote(n)
	}
}

// IndexNote adds or updates a single note in the index.
func (idx *Index) IndexNote(note *notes.Note) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	// Remove stale entry first.
	idx.removeNote(note.ID)
	idx.indexNote(note)
}

// RemoveNote removes a note from the index by ID.
func (idx *Index) RemoveNote(noteID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeNote(noteID)
}

// Search performs a BM25-inspired ranked search and returns up to limit results.
// A limit of 0 returns all matching results.
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

// --- internal helpers (called with lock held) ---

func (idx *Index) indexNote(note *notes.Note) {
	tf := make(map[string]float64)

	// Title tokens weighted 3×.
	for _, tok := range tokenize(note.Title) {
		tf[tok] += 3.0
	}
	// Tag tokens weighted 2×.
	for _, tag := range note.Tags {
		for _, tok := range tokenize(tag) {
			tf[tok] += 2.0
		}
	}
	// Body tokens weighted 1×.
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

// tokenize lowercases s, splits on non-letter/digit runes, and removes stop words.
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
