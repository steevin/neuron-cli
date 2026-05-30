package notes

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// TemplateData holds the variables that can be used inside a template.
type TemplateData struct {
	Title string
	Date  string
}

// RenderTemplate looks for a template file by name (either in .obsidian/templates/ or templates/),
// and executes it with the provided title and current date.
func (s *Store) RenderTemplate(templateName string, title string) (string, error) {
	if templateName == "" {
		return "", nil
	}

	// Determine template path. We check `.obsidian/templates` first, then `templates`
	// Ensure template ends in .md
	if filepath.Ext(templateName) == "" {
		templateName += ".md"
	}

	var tplPath string

	pathsToTry := []string{
		filepath.Join(s.VaultPath, ".obsidian", "templates", templateName),
		filepath.Join(s.VaultPath, "templates", templateName),
	}

	for _, p := range pathsToTry {
		if _, err := os.Stat(p); err == nil {
			tplPath = p
			break
		}
	}

	if tplPath == "" {
		return "", fmt.Errorf("template %q not found in vault", templateName)
	}

	contentBytes, err := os.ReadFile(tplPath)
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}

	t, err := template.New(templateName).Parse(string(contentBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := TemplateData{
		Title: title,
		Date:  time.Now().Format("2006-01-02"),
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
