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

// Package config handles loading, saving, and defaulting of NeuronCLI configuration.
// Configuration is stored as TOML at ~/.config/neuron/config.toml and is created
// automatically with sensible defaults on first run.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AIConfig holds AI/embedding provider settings.
type AIConfig struct {
	Enabled   bool   `toml:"enabled"`
	Provider  string `toml:"provider"`   // "ollama", "openai", or "none"
	Model     string `toml:"model"`
	OpenAIKey string `toml:"openai_key"` // leave empty when using Ollama
	OllamaURL string `toml:"ollama_url"`
}

// Config is the top-level NeuronCLI configuration.
type Config struct {
	VaultPath string    `toml:"vault_path"`
	Editor    string    `toml:"editor"`     // command used to open notes (e.g. "nvim", "code")
	Theme     string    `toml:"theme"`      // "dark" or "light"
	GitRemote string    `toml:"git_remote"`
	AI        AIConfig  `toml:"ai"`
}

// DefaultConfig returns a Config populated with sensible out-of-the-box values.
// The editor is taken from the $EDITOR environment variable, falling back to "vi".
func DefaultConfig() Config {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	return Config{
		VaultPath: "~/Documents/neuron-vault",
		Editor:    editor,
		Theme:     "dark",
		GitRemote: "",
		AI: AIConfig{
			Enabled:   false,
			Provider:  "ollama",
			Model:     "nomic-embed-text",
			OpenAIKey: "",
			OllamaURL: "http://localhost:11434",
		},
	}
}

// ConfigDir returns the path to the NeuronCLI configuration directory
// (~/.config/neuron). The directory is not guaranteed to exist.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "neuron"), nil
}

// ConfigPath returns the full path to the TOML configuration file
// (~/.config/neuron/config.toml).
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the configuration from disk. If the file does not yet exist the
// config directory is created, defaults are written, and the defaults are
// returned. Any ~ in VaultPath is expanded to the user home directory.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// First-run: create directory and write defaults.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		cfg := DefaultConfig()
		if saveErr := Save(&cfg); saveErr != nil {
			return nil, fmt.Errorf("config: failed to write defaults: %w", saveErr)
		}
		cfg.VaultPath = expandHome(cfg.VaultPath)
		return &cfg, nil
	}

	var cfg Config
	if _, err = toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: failed to parse %s: %w", path, err)
	}

	cfg.VaultPath = expandHome(cfg.VaultPath)
	return &cfg, nil
}

// Save writes cfg to disk as TOML, creating the config directory if needed.
func Save(cfg *Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: cannot create directory %s: %w", dir, err)
	}

	path := filepath.Join(dir, "config.toml")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("config: cannot create %s: %w", path, err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err = enc.Encode(cfg); err != nil {
		return fmt.Errorf("config: failed to encode TOML: %w", err)
	}
	return nil
}

// expandHome replaces a leading ~ with the current user's home directory.
// If the home directory cannot be determined the path is returned unchanged.
func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
