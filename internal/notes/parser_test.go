package notes

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantTag string
		wantOK  bool
	}{
		{
			name: "simple frontmatter",
			input: `---
title: My Note
tags: [test, go]
---

Hello world`,
			wantID:  "",
			wantTag: "test",
			wantOK:  true,
		},
		{
			name: "no frontmatter",
			input: `# Just a title

Some content`,
			wantID:  "",
			wantTag: "",
			wantOK:  true,
		},
		{
			name: "malformed frontmatter (no close)",
			input: `---
title: Broken
no close

body`,
			wantID:  "",
			wantTag: "",
			wantOK:  true,
		},
		{
			name: "with id and aliases",
			input: `---
id: abc-123
title: Test Note
aliases: [alias1, alias2]
tags: [work]
---

Content`,
			wantID:  "abc-123",
			wantTag: "work",
			wantOK:  true,
		},
		{
			name: "inline tags in body",
			input: `---
title: Tagged
---

This is about #golang and #coding`,
			wantID:  "",
			wantTag: "golang",
			wantOK:  true,
		},
		{
			name: "wikilinks extracted",
			input: `---
title: Linked
---

See [[Other Note]] and [[Target|display]]`,
			wantID:  "",
			wantTag: "",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "note.md")
			if err := os.WriteFile(path, []byte(tt.input), 0o600); err != nil {
				t.Fatal(err)
			}
			note, err := ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}
			if tt.wantOK && tt.wantID != "" && note.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", note.ID, tt.wantID)
			}
			if tt.wantTag != "" {
				found := false
				for _, tag := range note.Tags {
					if tag == tt.wantTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("tag %q not found in %v", tt.wantTag, note.Tags)
				}
			}
		})
	}
}

func TestParseContent(t *testing.T) {
	t.Run("title from H1 fallback", func(t *testing.T) {
		raw := "# My Document Title\n\nSome body"
		note, err := ParseContent(raw, "/tmp/test.md")
		if err != nil {
			t.Fatal(err)
		}
		if note.Title != "My Document Title" {
			t.Errorf("Title = %q, want %q", note.Title, "My Document Title")
		}
	})

	t.Run("title from filename fallback", func(t *testing.T) {
		raw := "No title here"
		note, err := ParseContent(raw, "/tmp/hello-world.md")
		if err != nil {
			t.Fatal(err)
		}
		if note.Title != "hello-world" {
			t.Errorf("Title = %q, want %q", note.Title, "hello-world")
		}
	})

	t.Run("tags deduplicated", func(t *testing.T) {
		raw := `---
tags: [go, test]
---

This has #go and #test again`
		note, err := ParseContent(raw, "/tmp/test.md")
		if err != nil {
			t.Fatal(err)
		}
		if len(note.Tags) != 2 {
			t.Errorf("expected 2 unique tags, got %d: %v", len(note.Tags), note.Tags)
		}
	})
}

func TestExtractWikilinks(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "[[Simple]]", want: []string{"Simple"}},
		{input: "[[Target|Display]]", want: []string{"Target"}},
		{input: "[[A]] and [[B]] and [[A]]", want: []string{"A", "B"}},
		{input: "No links here", want: nil},
		{input: "[[Multiple]]\n[[links]]\n[[here]]", want: []string{"Multiple", "links", "here"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractWikilinks(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractInlineTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "a #tag here", want: []string{"tag"}},
		{input: "#multi-word-tag/with-slash", want: []string{"multi-word-tag/with-slash"}},
		{input: "no tags", want: nil},
		{input: "```\n#not-a-tag\n```", want: nil},
		{input: "#dup and #dup", want: []string{"dup"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractInlineTags(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestToMarkdownRoundtrip(t *testing.T) {
	note := &Note{
		ID:      "test-uuid",
		Title:   "Round Trip",
		Tags:    []string{"test", "go"},
		Content: "Hello\n\nWorld",
		Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		Extra:   make(map[string]interface{}),
	}
	md := ToMarkdown(note)

	parsed, err := ParseContent(md, "/tmp/roundtrip.md")
	if err != nil {
		t.Fatalf("ParseContent failed: %v", err)
	}
	if parsed.Title != note.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, note.Title)
	}
	if len(parsed.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(parsed.Tags))
	}
	if parsed.Content != note.Content {
		t.Errorf("Content = %q, want %q", parsed.Content, note.Content)
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "Hello World", want: "hello-world"},
		{input: "Special!!!Chars___Here", want: "special-chars-here"},
		{input: "  spaces  ", want: "spaces"},
		{input: "ALLCAPS", want: "allcaps"},
		{input: "a-b-c-d-e", want: "a-b-c-d-e"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeFilename(tt.input)
			if got != tt.want {
				t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
