package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/config"
)

var initAppCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize NeuronCLI interactively",
	Long:  "Interactively prompt to set up vault path, preferred editor, and theme.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil || cfg == nil {
			cfg = &config.Config{
				Theme: "dark", // default
			}
		}

		var vaultPath, editor, theme string
		var setupPARA bool
		vaultPath = cfg.VaultPath
		editor = cfg.Editor
		theme = cfg.Theme

		if theme == "" {
			theme = "dark"
		}

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Vault Path").
					Description("Absolute path to your Markdown vault").
					Value(&vaultPath),
				huh.NewInput().
					Title("Preferred Editor").
					Description("Command used to open notes (e.g., vim, code, nano)").
					Value(&editor),
				huh.NewSelect[string]().
					Title("Theme").
					Options(
						huh.NewOption("Dark", "dark"),
						huh.NewOption("Light", "light"),
					).
					Value(&theme),
				huh.NewConfirm().
					Title("PARA Method").
					Description("Would you like to initialize the PARA folder structure? (1. Projects, 2. Areas, 3. Resources, 4. Archive)").
					Value(&setupPARA),
			),
		).Run()

		if err != nil {
			return err
		}

		cfg.VaultPath = vaultPath
		cfg.Editor = editor
		cfg.Theme = theme

		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %v", err)
		}

		if setupPARA {
			// Resolve tilde if needed
			resolvedPath := vaultPath
			if strings.HasPrefix(resolvedPath, "~/") || resolvedPath == "~" {
				home, err := os.UserHomeDir()
				if err == nil {
					resolvedPath = home + resolvedPath[1:]
				}
			}
			folders := []string{"1. Projects", "2. Areas", "3. Resources", "4. Archive"}
			for _, folder := range folders {
				path := filepath.Join(resolvedPath, folder)
				if err := os.MkdirAll(path, 0o700); err != nil {
					fmt.Printf("Warning: failed to create folder %q: %v\n", folder, err)
				}
			}
			fmt.Println("✓ PARA folders initialized!")
		}

		fmt.Println("NeuronCLI initialized successfully!")
		return nil
	},
}

