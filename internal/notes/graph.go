package notes

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Graph types
// ---------------------------------------------------------------------------

// GraphNode represents a single note in the link graph.
type GraphNode struct {
	Title     string   // Note title
	Path      string   // Absolute path to the note file
	Tags      []string // Tags on this note
	Links     []string // Titles of notes this note links to
	Backlinks []string // Titles of notes that link to this note
}

// Graph is an in-memory bidirectional link graph built from a slice of Notes.
type Graph struct {
	Nodes map[string]*GraphNode // keyed by note title
}

// ---------------------------------------------------------------------------
// BuildGraph
// ---------------------------------------------------------------------------

// BuildGraph constructs a bidirectional link graph from a slice of parsed
// notes. Each note becomes a node; forward links are copied from Note.Links,
// and backlinks are computed as the reverse.
func BuildGraph(notes []*Note) *Graph {
	g := &Graph{
		Nodes: make(map[string]*GraphNode, len(notes)),
	}

	// First pass: create all nodes.
	for _, n := range notes {
		g.Nodes[n.Title] = &GraphNode{
			Title: n.Title,
			Path:  n.Path,
			Tags:  n.Tags,
			Links: n.Links,
		}
	}

	// Second pass: populate backlinks.
	for _, n := range notes {
		for _, target := range n.Links {
			if targetNode, ok := g.Nodes[target]; ok {
				targetNode.Backlinks = append(targetNode.Backlinks, n.Title)
			}
			// If target doesn't exist as a node we still record the forward
			// link on the source node, but we can't add a backlink.
		}
	}

	// Sort backlinks for deterministic output.
	for _, node := range g.Nodes {
		sort.Strings(node.Backlinks)
	}

	return g
}

// ---------------------------------------------------------------------------
// Orphans
// ---------------------------------------------------------------------------

// Orphans returns all nodes that have no outgoing links AND no backlinks —
// i.e. notes that are completely isolated in the graph.
func (g *Graph) Orphans() []*GraphNode {
	var orphans []*GraphNode
	for _, node := range g.Nodes {
		if len(node.Links) == 0 && len(node.Backlinks) == 0 {
			orphans = append(orphans, node)
		}
	}
	// Sort for deterministic output.
	sort.Slice(orphans, func(i, j int) bool {
		return strings.ToLower(orphans[i].Title) < strings.ToLower(orphans[j].Title)
	})
	return orphans
}

// ---------------------------------------------------------------------------
// MostConnected
// ---------------------------------------------------------------------------

// MostConnected returns the top `limit` nodes ranked by total connections
// (len(Links) + len(Backlinks)). If limit ≤ 0 all nodes are returned.
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

// ---------------------------------------------------------------------------
// RenderASCII
// ---------------------------------------------------------------------------

// RenderASCII renders a Unicode tree rooted at rootTitle, expanding outgoing
// links up to depth levels deep. Cycles are detected and annotated with
// "(↺ cycle)" rather than recursing infinitely.
//
// If rootTitle is not found in the graph, a summary of the whole graph is
// rendered instead (total nodes, top connected nodes).
//
// Example output:
//
//	[Root Note]
//	├──▶ [Linked Note 1]
//	│    └──▶ [Deep Note]
//	└──▶ [Linked Note 2]
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

		// Choose connector characters.
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
			// Link target not in vault — render as dead link.
			sb.WriteString(fmt.Sprintf("%s%s[%s] (not in vault)\n", prefix, connector, linkTitle))
			continue
		}

		sb.WriteString(fmt.Sprintf("%s%s[%s]\n", prefix, connector, childNode.Title))
		g.renderChildren(sb, childNode, maxDepth, currentDepth+1, visited, childPrefix)
	}
}

// renderSummary produces a brief overview when the requested root note is not
// found in the graph.
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
