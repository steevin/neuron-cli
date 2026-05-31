package main

import (
	"fmt"

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

		fmt.Println("NeuronCLI initialized successfully!")
		return nil
	},
}
