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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/steevin/neuron-cli/internal/notes"
)

func (s *NeuronMCPServer) registerResources() {
	// Vault Resource: lists all notes
	vaultRes := mcp.NewResource("neuron://vault/", "Vault Notes",
		mcp.WithResourceDescription("List of all notes in the vault"),
		mcp.WithMIMEType("application/json"),
	)
	s.server.AddResource(vaultRes, s.handleReadVault)

	// Note Resource Template: read a specific note
	noteResTpl := mcp.NewResourceTemplate("neuron://notes/{id}", "Note Content",
		mcp.WithTemplateDescription("Read the raw markdown content of a specific note by ID"),
		mcp.WithTemplateMIMEType("text/markdown"),
	)
	s.server.AddResourceTemplate(noteResTpl, s.handleReadNote)
}

func (s *NeuronMCPServer) handleReadVault(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	nList, err := s.store.List(notes.ListOptions{})
	if err != nil {
		return nil, err
	}

	type vaultEntry struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		RelPath string `json:"path"`
	}

	var entries []vaultEntry
	for _, n := range nList {
		entries = append(entries, vaultEntry{
			ID:      n.ID,
			Title:   n.Title,
			RelPath: n.RelPath,
		})
	}

	b, _ := json.MarshalIndent(entries, "", "  ")

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "neuron://vault/",
			MIMEType: "application/json",
			Text:     string(b),
		},
	}, nil
}

func (s *NeuronMCPServer) handleReadNote(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	id := strings.TrimPrefix(uri, "neuron://notes/")

	note, err := s.store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("note not found: %v", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     note.RawContent,
		},
	}, nil
}
