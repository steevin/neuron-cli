// Copyright (C) 2025 Daniel Steevin
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package search

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/philippgille/chromem-go"
	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/notes"
)

// SemanticIndex provides vector-based semantic search over notes using local embeddings.
type SemanticIndex struct {
	db   *chromem.DB
	coll *chromem.Collection
	docs map[string]*notes.Note
}

// NewSemanticIndex initializes a new semantic index using the provided AI configuration.
func NewSemanticIndex(cfg *config.Config) (*SemanticIndex, error) {
	if !cfg.AI.Enabled {
		return nil, fmt.Errorf("AI features are disabled in config")
	}

	var embedFunc chromem.EmbeddingFunc
	if cfg.AI.Provider == "ollama" {
		embedFunc = chromem.NewEmbeddingFuncOllama(cfg.AI.Model, cfg.AI.OllamaURL)
	} else if cfg.AI.Provider == "openai" {
		embedFunc = chromem.NewEmbeddingFuncOpenAI(cfg.AI.OpenAIKey, chromem.EmbeddingModelOpenAI(cfg.AI.Model))
	} else {
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.AI.Provider)
	}

	db := chromem.NewDB()
	coll, err := db.GetOrCreateCollection("notes", nil, embedFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	return &SemanticIndex{
		db:   db,
		coll: coll,
		docs: make(map[string]*notes.Note),
	}, nil
}

// Rebuild clears and rebuilds the entire semantic index. This may take time if there are many notes
// and embeddings must be generated.
func (idx *SemanticIndex) Rebuild(ctx context.Context, noteList []*notes.Note) error {
	docs := make([]chromem.Document, 0, len(noteList))

	for _, n := range noteList {
		idx.docs[n.ID] = n

		// Create a chunk of text that represents the note semantics well
		content := fmt.Sprintf("Title: %s\nTags: %s\n\n%s",
			n.Title,
			strings.Join(n.Tags, ", "),
			n.Content,
		)

		docs = append(docs, chromem.Document{
			ID:      n.ID,
			Content: content,
			Metadata: map[string]string{
				"title": n.Title,
			},
		})
	}

	// AddDocuments will generate embeddings in parallel
	return idx.coll.AddDocuments(ctx, docs, runtime.NumCPU())
}

// Search queries the semantic index and returns the top results.
func (idx *SemanticIndex) Search(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	res, err := idx.coll.Query(ctx, query, limit, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	var results []*SearchResult
	for _, r := range res {
		if note, ok := idx.docs[r.ID]; ok {
			results = append(results, &SearchResult{
				Note:  note,
				Score: float64(r.Similarity),
			})
		}
	}
	return results, nil
}
