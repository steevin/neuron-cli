package notes

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ObsidianSettings holds a subset of Obsidian's app.json relevant to note
// creation and attachment handling.
type ObsidianSettings struct {
	NewFileLocation  string `json:"newFileLocation"`      // "root", "folder", "current"
	DefaultLocation  string `json:"newFileFolderPath"`    // folder for new notes
	AttachmentFolder string `json:"attachmentFolderPath"` // attachment storage path
	UseMarkdownLinks bool   `json:"useMarkdownLinks"`     // true → [text](path)
}

// ObsidianVault describes a detected (or absent) Obsidian vault.
type ObsidianVault struct {
	Path       string           // absolute path to vault root
	IsObsidian bool             // true if .obsidian/ was found
	Settings   ObsidianSettings // parsed from .obsidian/app.json
}

// DetectObsidianVault checks vaultPath for a .obsidian/ directory. When found
// it reads app.json for user settings. Missing or malformed settings files are
// silently ignored — the caller always gets a valid struct back.
func DetectObsidianVault(vaultPath string) (*ObsidianVault, error) {
	vault := &ObsidianVault{Path: vaultPath}

	obsidianDir := filepath.Join(vaultPath, ".obsidian")
	info, err := os.Stat(obsidianDir)
	if err != nil || !info.IsDir() {
		return vault, nil
	}

	vault.IsObsidian = true

	data, err := os.ReadFile(filepath.Join(obsidianDir, "app.json"))
	if err != nil {
		return vault, nil
	}

	var settings ObsidianSettings
	if err := json.Unmarshal(data, &settings); err == nil {
		vault.Settings = settings
	}

	return vault, nil
}

// NoteLocation returns the directory where a new note should be created, based
// on Obsidian's newFileLocation setting. Falls back to the vault root when the
// "current" mode is requested (it requires editor context we don't have).
func (v *ObsidianVault) NoteLocation(_ string) string {
	if v.Settings.NewFileLocation == "folder" && v.Settings.DefaultLocation != "" {
		return filepath.Join(v.Path, v.Settings.DefaultLocation)
	}
	return v.Path
}

// IsObsidianFile reports whether path lives inside a .obsidian/ or .trash/
// folder and should be skipped during vault walks.
func IsObsidianFile(path string) bool {
	normalised := filepath.ToSlash(path)
	for _, seg := range []string{"/.obsidian/", "/.trash/"} {
		for i := 0; i <= len(normalised)-len(seg); i++ {
			if normalised[i:i+len(seg)] == seg {
				return true
			}
		}
	}
	base := filepath.Base(path)
	return base == ".obsidian" || base == ".trash"
}
