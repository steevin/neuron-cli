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

package notes

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// ErrNoteNotFound se devuelve cuando no encontramos la nota.
var ErrNoteNotFound = fmt.Errorf("note not found")

// ListOptions controla cómo filtramos y paginamos las notas.
type ListOptions struct {
	Tags   []string // Filter by tags (AND logic — note must have ALL tags)
	Query  string   // Case-insensitive substring match against note title
	Limit  int      // Maximum number of results; 0 means no limit
	SortBy string   // "updated" (default), "created", or "title"
}

// Store maneja las notas en disco a partir de la raíz del vault.
type Store struct {
	VaultPath string         // Absolute path to the vault root
	Vault     *ObsidianVault // Detected Obsidian metadata (may be non-Obsidian)
}

// NewStore inicializa el store y expande el ~ si viene en la ruta.
func NewStore(vaultPath string) (*Store, error) {
	// expandimos el tilde al home del usuario
	if strings.HasPrefix(vaultPath, "~/") || vaultPath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("notes: resolving home directory: %w", err)
		}
		vaultPath = home + vaultPath[1:]
	}

	// creamos el directorio si no existe
	if err := os.MkdirAll(vaultPath, 0o700); err != nil {
		return nil, fmt.Errorf("notes: creating vault directory %q: %w", vaultPath, err)
	}

	vault, err := DetectObsidianVault(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("notes: detecting obsidian vault: %w", err)
	}

	return &Store{
		VaultPath: vaultPath,
		Vault:     vault,
	}, nil
}

// Create genera un UUID, limpia el nombre del archivo y guarda la nota nueva.
func (s *Store) Create(folder string, title string, tags []string, content string) (*Note, error) {
	id := uuid.New().String()
	filename := safeFilename(title) + ".md"

	var dirPath string
	if folder != "" {
		dirPath = filepath.Join(s.VaultPath, folder)
		// aseguramos que la subcarpeta exista
		if err := os.MkdirAll(dirPath, 0o700); err != nil {
			return nil, fmt.Errorf("notes: creating subdirectory %q: %w", folder, err)
		}
	} else {
		dirPath = s.VaultPath
	}

	fullPath := filepath.Join(dirPath, filename)

	// si ya existe, le agregamos un timestamp para no pisarlo
	if _, err := os.Stat(fullPath); err == nil {
		base := strings.TrimSuffix(filename, ".md")
		fullPath = filepath.Join(dirPath, fmt.Sprintf("%s-%d.md", base, time.Now().UnixNano()))
	}

	now := time.Now()
	relPath := filepath.Base(fullPath)
	if rel, err := filepath.Rel(s.VaultPath, fullPath); err == nil {
		relPath = rel
	}

	note := &Note{
		ID:      id,
		Title:   title,
		Path:    fullPath,
		RelPath: relPath,
		Content: content,
		Tags:    tags,
		Created: now,
		Updated: now,
		Extra:   make(map[string]interface{}),
	}

	note.RawContent = ToMarkdown(note)

	if err := os.WriteFile(fullPath, []byte(note.RawContent), 0o600); err != nil {
		return nil, fmt.Errorf("notes: writing note %q: %w", fullPath, err)
	}

	return note, nil
}

// DetectPARAFolders revisa si el usuario usa carpetas numeradas (ej. "1. Projects") o normales ("Projects").
func (s *Store) DetectPARAFolders() []string {
	// revisamos la raíz buscando las palabras clave del método PARA. Si no hay nada, usamos los valores por defecto numerados:
	defaults := []string{"1. Projects", "2. Areas", "3. Resources", "4. Archive"}
	
	entries, err := os.ReadDir(s.VaultPath)
	if err != nil {
		return defaults
	}

	found := make(map[string]string) // mapa "projects" -> nombre real de la carpeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.Contains(lower, "project") {
			found["projects"] = name
		} else if strings.Contains(lower, "area") {
			found["areas"] = name
		} else if strings.Contains(lower, "resource") {
			found["resources"] = name
		} else if strings.Contains(lower, "archive") {
			found["archives"] = name
		}
	}

	result := make([]string, 4)
	keys := []string{"projects", "areas", "resources", "archives"}
	for i, key := range keys {
		if name, ok := found[key]; ok {
			result[i] = name
		} else {
			result[i] = defaults[i]
		}
	}

	return result
}

// ExtraFolders devuelve las carpetas raíz que no son del método PARA ni del sistema.
func (s *Store) ExtraFolders() []string {
	paraFolders := s.DetectPARAFolders()
	paraSet := make(map[string]struct{}, len(paraFolders))
	for _, pf := range paraFolders {
		paraSet[strings.ToLower(pf)] = struct{}{}
	}

	entries, err := os.ReadDir(s.VaultPath)
	if err != nil {
		return nil
	}

	systemDirs := map[string]struct{}{
		".obsidian": {},
		".trash":    {},
	}

	var extra []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if _, isSystem := systemDirs[name]; isSystem {
			continue
		}
		if _, isPARA := paraSet[strings.ToLower(name)]; isPARA {
			continue
		}
		extra = append(extra, name)
	}
	return extra
}

// Move mueve la nota a otra carpeta del vault.
func (s *Store) Move(idOrTitle string, targetFolder string) error {
	note, err := s.Get(idOrTitle)
	if err != nil {
		return err
	}

	targetDir := s.VaultPath
	if targetFolder != "" {
		targetDir = filepath.Join(s.VaultPath, targetFolder)
	}

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("notes: creating target directory: %w", err)
	}

	dest := filepath.Join(targetDir, filepath.Base(note.Path))
	// evitamos colisiones de nombres
	if _, statErr := os.Stat(dest); statErr == nil {
		stem := strings.TrimSuffix(filepath.Base(note.Path), ".md")
		dest = filepath.Join(targetDir, fmt.Sprintf("%s-%d.md", stem, time.Now().UnixNano()))
	}

	if err := os.Rename(note.Path, dest); err != nil {
		return fmt.Errorf("notes: moving note: %w", err)
	}

	// actualizamos la ruta en memoria por si el caller la necesita
	note.Path = dest
	rel, relErr := filepath.Rel(s.VaultPath, dest)
	if relErr == nil {
		note.RelPath = rel
	}
	return nil
}


// Get busca primero por UUID, luego por título y por último por nombre de archivo.
func (s *Store) Get(idOrTitle string) (*Note, error) {
	all, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(idOrTitle)

	// buscamos por UUID exacto
	for _, n := range all {
		if n.ID == idOrTitle {
			return n, nil
		}
	}

	// si no, por título (ignorando mayúsculas)
	for _, n := range all {
		if strings.ToLower(n.Title) == lowerQuery {
			return n, nil
		}
	}

	// por último, por nombre de archivo
	for _, n := range all {
		stem := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		if strings.ToLower(stem) == lowerQuery {
			return n, nil
		}
	}

	return nil, ErrNoteNotFound
}

// List recorre el vault, lee los .md y devuelve los resultados filtrados y ordenados.
func (s *Store) List(opts ListOptions) ([]*Note, error) {
	var notes []*Note

	err := filepath.WalkDir(s.VaultPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// si no podemos leer algo, lo saltamos para no romper todo
			return nil
		}

		// ignoramos carpetas ocultas y de sistema de Obsidian
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ".obsidian" || name == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}

		// solo nos importan los markdown
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		// por si acaso nos metemos en alguna carpeta de Obsidian, evitamos sus archivos
		if IsObsidianFile(path) {
			return nil
		}

		note, parseErr := ParseFile(path)
		if parseErr != nil {
			// los archivos mal formados los ignoramos en silencio
			return nil
		}

		// calculamos la ruta relativa
		rel, relErr := filepath.Rel(s.VaultPath, path)
		if relErr == nil {
			note.RelPath = rel
		}

		notes = append(notes, note)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("notes: walking vault %q: %w", s.VaultPath, err)
	}

	filtered := make([]*Note, 0, len(notes))
	for _, n := range notes {
		if !matchesListOptions(n, opts) {
			continue
		}
		filtered = append(filtered, n)
	}

	SortNotes(filtered, opts.SortBy)

	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}

	return filtered, nil
}

// Update actualiza la fecha de modificación y reescribe el archivo.
func (s *Store) Update(note *Note) error {
	note.Updated = time.Now()
	note.RawContent = ToMarkdown(note)

	if err := os.WriteFile(note.Path, []byte(note.RawContent), 0o600); err != nil {
		return fmt.Errorf("notes: writing note %q: %w", note.Path, err)
	}
	return nil
}

// Reload vuelve a leer la nota desde el disco.
func (s *Store) Reload(note *Note) (*Note, error) {
	fresh, err := ParseFile(note.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(s.VaultPath, note.Path)
	if err == nil {
		fresh.RelPath = rel
	}
	return fresh, nil
}


// Delete mueve la nota a la papelera en lugar de borrarla definitivamente.
func (s *Store) Delete(idOrTitle string) error {
	note, err := s.Get(idOrTitle)
	if err != nil {
		return err
	}

	trashDir := filepath.Join(s.VaultPath, ".trash")
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		return fmt.Errorf("notes: creating .trash directory: %w", err)
	}

	dest := filepath.Join(trashDir, filepath.Base(note.Path))
	// si ya hay algo en la papelera con ese nombre, le ponemos timestamp
	if _, statErr := os.Stat(dest); statErr == nil {
		stem := strings.TrimSuffix(filepath.Base(note.Path), ".md")
		dest = filepath.Join(trashDir, fmt.Sprintf("%s-%d.md", stem, time.Now().UnixNano()))
	}

	if err := os.Rename(note.Path, dest); err != nil {
		return fmt.Errorf("notes: moving note to trash: %w", err)
	}
	return nil
}

// Count cuenta cuántos archivos .md tenemos.
func (s *Store) Count() (int, error) {
	notes, err := s.List(ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(notes), nil
}

// Tags devuelve un mapa con la frecuencia de cada etiqueta.
func (s *Store) Tags() (map[string]int, error) {
	notes, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, n := range notes {
		for _, tag := range n.Tags {
			counts[tag]++
		}
	}
	return counts, nil
}

// safeFilename limpia el título para usarlo como nombre de archivo (solo letras, números y guiones).
func safeFilename(title string) string {
	var sb strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen && sb.Len() > 0 {
			sb.WriteByte('-')
			prevHyphen = true
		}
	}
	s := strings.TrimRight(sb.String(), "-")
	if len(s) > 80 {
		s = s[:80]
		// que no termine en guion
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = fmt.Sprintf("note-%d", time.Now().UnixNano())
	}
	return s
}

// matchesListOptions verifica si la nota cumple con los filtros.
func matchesListOptions(n *Note, opts ListOptions) bool {
	// filtro por etiquetas (tienen que estar todas)
	if len(opts.Tags) > 0 {
		noteTagSet := make(map[string]struct{}, len(n.Tags))
		for _, t := range n.Tags {
			noteTagSet[strings.ToLower(t)] = struct{}{}
		}
		for _, required := range opts.Tags {
			if _, ok := noteTagSet[strings.ToLower(required)]; !ok {
				return false
			}
		}
	}

	// filtro por parte del título
	if opts.Query != "" {
		if !strings.Contains(strings.ToLower(n.Title), strings.ToLower(opts.Query)) {
			return false
		}
	}

	return true
}

// SortNotes ordena las notas según lo que pidan.
func SortNotes(notes []*Note, by string) {
	switch by {
	case "created":
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].Created.After(notes[j].Created)
		})
	case "title":
		sort.Slice(notes, func(i, j int) bool {
			return strings.ToLower(notes[i].Title) < strings.ToLower(notes[j].Title)
		})
	default: // "updated" and anything else
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].Updated.After(notes[j].Updated)
		})
	}
}

// AttachAsset copies a local file or downloads a URL into the "assets/" folder
// and appends an image/link markdown tag to the note.
func (s *Store) AttachAsset(noteID string, pathOrURL string) error {
	note, err := s.Get(noteID)
	if err != nil {
		return err
	}

	assetsDir := filepath.Join(s.VaultPath, "assets")
	if err := os.MkdirAll(assetsDir, 0o700); err != nil {
		return fmt.Errorf("notes: creating assets directory: %w", err)
	}

	var filename string
	var localDest string

	isURL := strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://")

	if isURL {
		parsed, err := url.Parse(pathOrURL)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		filename = filepath.Base(parsed.Path)
		if filename == "" || filename == "/" {
			filename = fmt.Sprintf("download-%d.jpg", time.Now().UnixNano())
		}
		
		// Ensure unique filename
		localDest = filepath.Join(assetsDir, filename)
		if _, err := os.Stat(localDest); err == nil {
			filename = fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)
			localDest = filepath.Join(assetsDir, filename)
		}

		resp, err := http.Get(pathOrURL)
		if err != nil {
			return fmt.Errorf("downloading asset: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %s", resp.Status)
		}

		out, err := os.Create(localDest)
		if err != nil {
			return fmt.Errorf("creating asset file: %w", err)
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return fmt.Errorf("saving asset: %w", err)
		}
	} else {
		// Local file path
		cleanPath := strings.TrimPrefix(pathOrURL, "file://")
		
		// If path has spaces but is passed directly from D&D, it might have quotes or escaped spaces. 
		cleanPath = strings.Trim(cleanPath, "'\" ")
		cleanPath = strings.ReplaceAll(cleanPath, "\\ ", " ") // Fix macOS Terminal drag and drop
		
		_, err := os.Stat(cleanPath)
		if err != nil {
			return fmt.Errorf("local file not found: %w", err)
		}

		filename = filepath.Base(cleanPath)
		localDest = filepath.Join(assetsDir, filename)
		if _, err := os.Stat(localDest); err == nil {
			filename = fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)
			localDest = filepath.Join(assetsDir, filename)
		}

		in, err := os.Open(cleanPath)
		if err != nil {
			return fmt.Errorf("opening local file: %w", err)
		}
		defer in.Close()

		out, err := os.Create(localDest)
		if err != nil {
			return fmt.Errorf("creating asset file: %w", err)
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		if err != nil {
			return fmt.Errorf("copying asset: %w", err)
		}
	}

	// Append to note
	ext := strings.ToLower(filepath.Ext(filename))
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".svg" || ext == ".avif"
	
	mdPath := filepath.ToSlash(filepath.Join("assets", filename))
	var appendText string
	if isImage {
		appendText = fmt.Sprintf("\n\n![%s](%s)", filename, mdPath)
	} else {
		appendText = fmt.Sprintf("\n\n[%s](%s)", filename, mdPath)
	}

	note.Content += appendText
	return s.Update(note)
}
