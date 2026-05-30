package notes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// Obsidian vault detection and settings
// ---------------------------------------------------------------------------

// ObsidianSettings holds a subset of Obsidian's app.json configuration that
// is relevant to note creation and attachment handling.
type ObsidianSettings struct {
	NewFileLocation  string `json:"newFileLocation"`  // "root", "folder", "current"
	DefaultLocation  string `json:"newFileFolderPath"` // default folder for new notes
	AttachmentFolder string `json:"attachmentFolderPath"` // attachment storage path
	UseMarkdownLinks bool   `json:"useMarkdownLinks"` // true → [text](path), false → [[wikilink]]
}

// ObsidianVault describes a detected (or absent) Obsidian vault at a given
// directory path.
type ObsidianVault struct {
	Path       string           // Absolute path to vault root
	IsObsidian bool             // True if a .obsidian/ folder was found
	Settings   ObsidianSettings // Parsed from .obsidian/app.json (if present)
}

// DetectObsidianVault inspects vaultPath for the presence of a .obsidian/
// directory. If found it populates IsObsidian and attempts to read
// .obsidian/app.json for user settings. Missing or unreadable settings files
// are silently ignored — the caller always receives a valid struct.
func DetectObsidianVault(vaultPath string) (*ObsidianVault, error) {
	vault := &ObsidianVault{Path: vaultPath}

	obsidianDir := filepath.Join(vaultPath, ".obsidian")
	info, err := os.Stat(obsidianDir)
	if err != nil {
		// Not an error — vault simply isn't an Obsidian vault.
		return vault, nil
	}
	if !info.IsDir() {
		return vault, nil
	}

	vault.IsObsidian = true

	// Attempt to read app.json; failures are non-fatal.
	appJSON := filepath.Join(obsidianDir, "app.json")
	data, err := os.ReadFile(appJSON)
	if err != nil {
		// app.json is optional — return what we have.
		return vault, nil
	}

	var settings ObsidianSettings
	if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
		// Malformed JSON is non-fatal; use zero-value settings.
		return vault, nil
	}
	vault.Settings = settings

	return vault, nil
}

// NoteLocation returns the directory path where a new note with the given
// title should be created, based on the vault's Obsidian settings.
//
// Supported values of Settings.NewFileLocation:
//   - "folder" → vault root joined with Settings.DefaultLocation
//   - "root" or "" (default) → vault root
//
// The "current" mode cannot be resolved without editor context, so it also
// falls back to the vault root.
func (v *ObsidianVault) NoteLocation(title string) string {
	switch v.Settings.NewFileLocation {
	case "folder":
		if v.Settings.DefaultLocation != "" {
			return filepath.Join(v.Path, v.Settings.DefaultLocation)
		}
	}
	// Default: vault root
	return v.Path
}

// IsObsidianFile reports whether path resides inside a .obsidian/ or .trash/
// folder. These files should be skipped during vault walks.
func IsObsidianFile(path string) bool {
	// Normalise to forward-slashes for consistent matching.
	normalised := filepath.ToSlash(path)
	for _, segment := range []string{"/.obsidian/", "/.trash/"} {
		if contains(normalised, segment) {
			return true
		}
	}
	// Also catch paths that ARE the .obsidian or .trash directory themselves.
	base := filepath.Base(path)
	return base == ".obsidian" || base == ".trash"
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// contains is a thin wrapper kept here to avoid importing strings in this file
// when the only usage is the two-segment check above.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// obsidianVaultError wraps a descriptive error for vault-related failures.
func obsidianVaultError(vaultPath, detail string) error {
	return fmt.Errorf("notes: obsidian vault %q: %s", vaultPath, detail)
}

// Ensure obsidianVaultError is used (compile-time guard).
var _ = obsidianVaultError
