package main

import "testing"

func TestSplitCommandSupportsEditorArgs(t *testing.T) {
	parts, err := splitCommand(`code -w "My Note.md"`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code", "-w", "My Note.md"}
	if len(parts) != len(want) {
		t.Fatalf("expected %d parts, got %d: %#v", len(want), len(parts), parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("part %d: expected %q, got %q", i, want[i], parts[i])
		}
	}
}

func TestSplitCommandRejectsShellMetacharacters(t *testing.T) {
	if _, err := splitCommand("vi; rm -rf /"); err == nil {
		t.Fatal("expected shell metacharacter to be rejected")
	}
}
