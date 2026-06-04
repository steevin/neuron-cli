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

package notes

import (
	"fmt"
	"sort"
	"strings"
)

// GraphNode represents a single note in the link graph.
type GraphNode struct {
	Title     string
	Path      string
	Tags      []string
	Links     []string // outgoing [[wikilinks]]
	Backlinks []string // notes that link to this one
}

// Graph is an in-memory bidirectional link graph built from a slice of Notes.
type Graph struct {
	Nodes map[string]*GraphNode // keyed by note title
}

// BuildGraph constructs a bidirectional link graph from a slice of notes.
func BuildGraph(notes []*Note) *Graph {
	g := &Graph{
		Nodes: make(map[string]*GraphNode, len(notes)),
	}

	for _, n := range notes {
		g.Nodes[n.Title] = &GraphNode{
			Title: n.Title,
			Path:  n.Path,
			Tags:  n.Tags,
			Links: n.Links,
		}
	}

	for _, n := range notes {
		for _, target := range n.Links {
			if targetNode, ok := g.Nodes[target]; ok {
				targetNode.Backlinks = append(targetNode.Backlinks, n.Title)
			}
		}
	}

	for _, node := range g.Nodes {
		sort.Strings(node.Backlinks)
	}

	return g
}

// Orphans returns nodes with no outgoing links and no backlinks.
func (g *Graph) Orphans() []*GraphNode {
	var orphans []*GraphNode
	for _, node := range g.Nodes {
		if len(node.Links) == 0 && len(node.Backlinks) == 0 {
			orphans = append(orphans, node)
		}
	}
	sort.Slice(orphans, func(i, j int) bool {
		return strings.ToLower(orphans[i].Title) < strings.ToLower(orphans[j].Title)
	})
	return orphans
}

// MostConnected returns the top limit nodes by total connections (links + backlinks).
// A limit ≤ 0 returns all nodes.
func (g *Graph) MostConnected(limit int) []*GraphNode {
	all := make([]*GraphNode, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		all = append(all, node)
	}

	sort.Slice(all, func(i, j int) bool {
		ci := len(all[i].Links) + len(all[i].Backlinks)
		cj := len(all[j].Links) + len(all[j].Backlinks)
		if ci != cj {
			return ci > cj // descending by connection count
		}
		return strings.ToLower(all[i].Title) < strings.ToLower(all[j].Title)
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

// RenderASCII renders a Unicode tree rooted at rootTitle up to depth levels deep.
// Cycles are annotated with "(↺ cycle)". Falls back to a graph summary when
// rootTitle is not found.
func (g *Graph) RenderASCII(rootTitle string, depth int) string {
	rootNode, exists := g.Nodes[rootTitle]
	if !exists {
		return g.renderSummary()
	}

	var sb strings.Builder
	visited := make(map[string]bool)
	sb.WriteString(fmt.Sprintf("[%s]\n", rootNode.Title))
	g.renderChildren(&sb, rootNode, depth, 0, visited, "")
	return sb.String()
}

// renderChildren recursively writes child nodes to sb.
// prefix is the accumulated indent string for the current level.
func (g *Graph) renderChildren(
	sb *strings.Builder,
	node *GraphNode,
	maxDepth, currentDepth int,
	visited map[string]bool,
	prefix string,
) {
	if currentDepth >= maxDepth {
		return
	}

	visited[node.Title] = true
	defer func() { visited[node.Title] = false }()

	for i, linkTitle := range node.Links {
		isLast := i == len(node.Links)-1

		connector := "├──▶ "
		childPrefix := prefix + "│    "
		if isLast {
			connector = "└──▶ "
			childPrefix = prefix + "     "
		}

		if visited[linkTitle] {
			sb.WriteString(fmt.Sprintf("%s%s[%s] (↺ cycle)\n", prefix, connector, linkTitle))
			continue
		}

		childNode, exists := g.Nodes[linkTitle]
		if !exists {
			sb.WriteString(fmt.Sprintf("%s%s[%s] (not in vault)\n", prefix, connector, linkTitle))
			continue
		}

		sb.WriteString(fmt.Sprintf("%s%s[%s]\n", prefix, connector, childNode.Title))
		g.renderChildren(sb, childNode, maxDepth, currentDepth+1, visited, childPrefix)
	}
}

func (g *Graph) renderSummary() string {
	var sb strings.Builder
	sb.WriteString("(root note not found — graph summary)\n")
	sb.WriteString(fmt.Sprintf("Total nodes: %d\n", len(g.Nodes)))

	top := g.MostConnected(5)
	if len(top) > 0 {
		sb.WriteString("Most connected:\n")
		for _, node := range top {
			connections := len(node.Links) + len(node.Backlinks)
			sb.WriteString(fmt.Sprintf("  [%s] — %d connection(s)\n", node.Title, connections))
		}
	}
	return sb.String()
}
