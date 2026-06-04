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

// Package notes is the core data layer: Note type, parser, Obsidian vault
// detector, file store, and link graph.
package notes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// wikilinkRe matches [[target]] and [[target|display]] forms.
	wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]*)?\]\]`)
	inlineTagRe = regexp.MustCompile(`(?:^|\s)#([a-zA-Z][a-zA-Z0-9_/-]*)`)
	blockRefRe  = regexp.MustCompile(`\^([a-zA-Z0-9-]+)`)
	h1Re        = regexp.MustCompile(`(?m)^#\s+(.+)$`)
)

// Note represents a single markdown note in the vault.
type Note struct {
	ID         string // UUID (e.g. "a1b2c3d4-...")
	Title      string // From frontmatter or first H1
	Path       string // Absolute path to the .md file
	RelPath    string // Relative path from vault root (set by caller)
	Content    string // Body content (after frontmatter)
	RawContent string // Full raw file content

	Tags      []string // From frontmatter + inline #tags
	Links     []string // [[wikilink]] targets (note titles)
	BlockRefs []string // ^block-id references
	Aliases   []string // Obsidian aliases field

	Created time.Time
	Updated time.Time

	Extra map[string]interface{} // Extra frontmatter fields preserved
}

// ParseFile reads the markdown file at path and returns a populated Note.
func ParseFile(path string) (*Note, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("notes: reading file %q: %w", path, err)
	}
	note, err := ParseContent(string(raw), path)
	if err != nil {
		return nil, fmt.Errorf("notes: parsing %q: %w", path, err)
	}
	return note, nil
}

// ParseContent parses raw markdown (with optional YAML frontmatter) and
// returns a Note. path is used for title fallback and Note.Path.
func ParseContent(raw string, path string) (*Note, error) {
	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	note := &Note{
		Path:       path,
		Content:    body,
		RawContent: raw,
		Extra:      make(map[string]interface{}),
	}

	// ------------------------------------------------------------------ ID
	note.ID = stringField(fm, "id")
	if note.ID == "" {
		note.ID = generatePlaceholderID()
	}
	delete(fm, "id")

	// --------------------------------------------------------------- title
	note.Title = stringField(fm, "title")
	delete(fm, "title")
	if note.Title == "" {
		// Try first H1 in body
		if m := h1Re.FindStringSubmatch(body); m != nil {
			note.Title = strings.TrimSpace(m[1])
		}
	}
	if note.Title == "" {
		// Fall back to filename without extension
		base := filepath.Base(path)
		note.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// ---------------------------------------------------------------- tags
	fmTags := stringSliceField(fm, "tags")
	delete(fm, "tags")
	inlineTags := ExtractInlineTags(body)
	note.Tags = deduplicate(append(fmTags, inlineTags...))

	// --------------------------------------------------------------- links
	note.Links = ExtractWikilinks(body)

	// ---------------------------------------------------------- block refs
	note.BlockRefs = extractBlockRefs(body)

	// --------------------------------------------------------------- aliases
	note.Aliases = stringSliceField(fm, "aliases")
	delete(fm, "aliases")

	// --------------------------------------------------------- created / updated
	note.Created = parseTimeField(fm, "created")
	delete(fm, "created")
	note.Updated = parseTimeField(fm, "updated")
	delete(fm, "updated")

	// Sensible defaults: use file modification time when not set.
	if note.Created.IsZero() || note.Updated.IsZero() {
		if info, statErr := os.Stat(path); statErr == nil {
			if note.Created.IsZero() {
				note.Created = info.ModTime()
			}
			if note.Updated.IsZero() {
				note.Updated = info.ModTime()
			}
		}
	}

	// ------------------------------------------------------------ extra fields
	for k, v := range fm {
		note.Extra[k] = v
	}

	return note, nil
}

// ParseFrontmatter splits raw markdown into a YAML frontmatter map and body.
// Returns an empty map and the full content when no frontmatter is present.
func ParseFrontmatter(raw string) (frontmatter map[string]interface{}, body string, err error) {
	frontmatter = make(map[string]interface{})

	// Frontmatter is only valid when the file starts with "---\n".
	if !strings.HasPrefix(raw, "---") {
		return frontmatter, raw, nil
	}

	// Find the closing delimiter.
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, raw, fmt.Errorf("notes: scanning frontmatter: %w", err)
	}

	if len(lines) == 0 {
		return frontmatter, raw, nil
	}

	// lines[0] must be "---"
	if lines[0] != "---" {
		return frontmatter, raw, nil
	}

	closingIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			closingIdx = i
			break
		}
	}
	if closingIdx == -1 {
		// No closing delimiter found — treat entire content as body.
		return frontmatter, raw, nil
	}

	yamlBlock := strings.Join(lines[1:closingIdx], "\n")
	bodyLines := lines[closingIdx+1:]
	body = strings.Join(bodyLines, "\n")
	// Trim a single leading newline that separates frontmatter from body.
	body = strings.TrimPrefix(body, "\n")

	if err = yaml.Unmarshal([]byte(yamlBlock), &frontmatter); err != nil {
		return nil, raw, fmt.Errorf("notes: parsing YAML frontmatter: %w", err)
	}
	if frontmatter == nil {
		frontmatter = make(map[string]interface{})
	}

	return frontmatter, body, nil
}

// ExtractWikilinks returns deduplicated [[wikilink]] targets from content.
// For [[target|display]] only the target part is returned.
func ExtractWikilinks(content string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]struct{}, len(matches))
	var out []string
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		if target == "" {
			continue
		}
		if _, ok := seen[target]; !ok {
			seen[target] = struct{}{}
			out = append(out, target)
		}
	}
	return out
}

// ExtractInlineTags finds #tag patterns outside fenced code blocks and URLs.
// Returns deduplicated tag names without the leading '#'.
func ExtractInlineTags(content string) []string {
	// Strip fenced code blocks first so we don't match tags inside them.
	stripped := removeCodeBlocks(content)

	matches := inlineTagRe.FindAllStringSubmatch(stripped, -1)
	seen := make(map[string]struct{}, len(matches))
	var out []string
	for _, m := range matches {
		tag := m[1]
		if _, ok := seen[tag]; !ok {
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}

// ToMarkdown serializes a Note to markdown with YAML frontmatter.
// Always includes id, title, tags, created, updated plus any Extra fields.
func ToMarkdown(note *Note) string {
	fm := make(map[string]interface{})

	// Merge extra fields first so explicit fields overwrite them.
	for k, v := range note.Extra {
		fm[k] = v
	}

	fm["id"] = note.ID
	fm["title"] = note.Title

	if len(note.Tags) > 0 {
		fm["tags"] = note.Tags
	} else {
		fm["tags"] = []string{}
	}

	if len(note.Aliases) > 0 {
		fm["aliases"] = note.Aliases
	}

	fm["created"] = note.Created.UTC().Format(time.RFC3339)
	fm["updated"] = note.Updated.UTC().Format(time.RFC3339)

	yamlBytes, err := yaml.Marshal(fm)
	var yamlStr string
	if err != nil {
		// Fallback: write minimal frontmatter manually.
		yamlStr = fmt.Sprintf("id: %s\ntitle: %q\n", note.ID, note.Title)
	} else {
		yamlStr = string(yamlBytes)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(yamlStr)
	sb.WriteString("---\n\n")
	sb.WriteString(note.Content)

	return sb.String()
}

// generatePlaceholderID returns a time-stamped fallback ID.
// The real UUID is assigned in store.go.
func generatePlaceholderID() string {
	return fmt.Sprintf("note-%d", time.Now().UnixNano())
}

// stringField safely retrieves a string value from a frontmatter map.
func stringField(fm map[string]interface{}, key string) string {
	v, ok := fm[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// stringSliceField retrieves a []string from a frontmatter map.
// It handles both []interface{} (common from yaml.v3) and []string.
func stringSliceField(fm map[string]interface{}, key string) []string {
	v, ok := fm[key]
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out
	case string:
		// Single-value written as a plain string.
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return nil
}

// parseTimeField attempts to parse a frontmatter time value using RFC3339 then
// the date-only format "2006-01-02". Returns the zero time on failure.
func parseTimeField(fm map[string]interface{}, key string) time.Time {
	s := stringField(fm, key)
	if s == "" {
		return time.Time{}
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// extractBlockRefs returns all ^blockid references found in content.
func extractBlockRefs(content string) []string {
	matches := blockRefRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]struct{}, len(matches))
	var out []string
	for _, m := range matches {
		ref := m[1]
		if _, ok := seen[ref]; !ok {
			seen[ref] = struct{}{}
			out = append(out, ref)
		}
	}
	return out
}

// removeCodeBlocks strips fenced code blocks (``` or ~~~) from content so that
// inline-tag extraction does not match tags inside code.
func removeCodeBlocks(content string) string {
	var sb strings.Builder
	inFence := false
	fenceChar := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !inFence {
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = true
				fenceChar = trimmed[:3]
				sb.WriteByte('\n')
				continue
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
		} else {
			if strings.HasPrefix(trimmed, fenceChar) {
				inFence = false
			}
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// deduplicate returns a new slice with duplicates removed, preserving order.
func deduplicate(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
