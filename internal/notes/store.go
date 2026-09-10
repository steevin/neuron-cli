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
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const maxAssetDownloadBytes = 50 << 20 // 50 MiB

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

	mu            sync.RWMutex
	noteCache     []*Note              // in-memory cache of parsed notes (nil = dirty)
	cacheModTimes map[string]time.Time // file path → last mod time when cached
	cachedFileCnt int                  // total .md files at last scan; 0 means unknown
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
	absVaultPath, err := filepath.Abs(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("notes: resolving vault path %q: %w", vaultPath, err)
	}
	vaultPath = absVaultPath

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

// NeuronDir returns the path to the .neuron metadata directory inside the vault.
// Creates it if it doesn't exist.
func (s *Store) NeuronDir() string {
	return filepath.Join(s.VaultPath, ".neuron")
}

// CacheDir returns the path where search indices are cached.
func (s *Store) CacheDir() string {
	return filepath.Join(s.NeuronDir(), "cache")
}

// EmbedDir returns the path where chromem-go persists the vector DB.
func (s *Store) EmbedDir() string {
	return filepath.Join(s.NeuronDir(), "chromem")
}

// InvalidateCache marks the in-memory note cache as stale so the next
// List call re-scans the vault from disk.
func (s *Store) InvalidateCache() {
	s.mu.Lock()
	s.noteCache = nil
	s.cacheModTimes = nil
	s.cachedFileCnt = 0
	s.mu.Unlock()
}

func (s *Store) vaultSubdir(folder string) (string, error) {
	if folder == "" || folder == "." {
		return s.VaultPath, nil
	}
	if filepath.IsAbs(folder) {
		return "", fmt.Errorf("notes: folder %q must be relative to the vault", folder)
	}
	clean := filepath.Clean(folder)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("notes: folder %q escapes the vault", folder)
	}

	fullPath := filepath.Join(s.VaultPath, clean)
	rel, err := filepath.Rel(s.VaultPath, fullPath)
	if err != nil {
		return "", fmt.Errorf("notes: resolving folder %q: %w", folder, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("notes: folder %q escapes the vault", folder)
	}
	return fullPath, nil
}

func (s *Store) assetsDir() (string, error) {
	folder := "assets"
	if s.Vault != nil && s.Vault.Settings.AttachmentFolder != "" {
		folder = s.Vault.Settings.AttachmentFolder
	}
	return s.vaultSubdir(folder)
}

// Create genera un UUID, limpia el nombre del archivo y guarda la nota nueva.
func (s *Store) Create(folder string, title string, tags []string, content string) (*Note, error) {
	id := uuid.New().String()
	filename := safeFilename(title) + ".md"

	dirPath, err := s.vaultSubdir(folder)
	if err != nil {
		return nil, err
	}
	// aseguramos que la subcarpeta exista
	if err := os.MkdirAll(dirPath, 0o700); err != nil {
		return nil, fmt.Errorf("notes: creating subdirectory %q: %w", folder, err)
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

	s.InvalidateCache()
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

	targetDir, err := s.vaultSubdir(targetFolder)
	if err != nil {
		return err
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
	s.InvalidateCache()
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
// Usa un caché en memoria para evitar escanear el disco en cada llamada.
func (s *Store) List(opts ListOptions) ([]*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.noteCache == nil || s.isCacheStaleLocked() {
		if err := s.scanAllLocked(); err != nil {
			return nil, err
		}
	}

	filtered := make([]*Note, 0, len(s.noteCache))
	for _, n := range s.noteCache {
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

// scanAllLocked walks the vault and parses every .md file, populating the cache.
// The caller must hold s.mu write lock.
func (s *Store) scanAllLocked() error {
	var notes []*Note
	modTimes := make(map[string]time.Time)
	var mdCount int

	err := filepath.WalkDir(s.VaultPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, walkErr)
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ".obsidian" || name == ".trash" || name == ".neuron" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		if IsObsidianFile(path) {
			return nil
		}

		mdCount++

		note, parseErr := ParseFile(path)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to parse %s: %v\n", path, parseErr)
			return nil
		}

		rel, relErr := filepath.Rel(s.VaultPath, path)
		if relErr == nil {
			note.RelPath = rel
		}

		notes = append(notes, note)

		if info, err := os.Stat(path); err == nil {
			modTimes[path] = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("notes: walking vault %q: %w", s.VaultPath, err)
	}

	s.noteCache = notes
	s.cacheModTimes = modTimes
	s.cachedFileCnt = mdCount
	return nil
}

// IsCacheStale checks if any .md file has been modified since the last scan.
// Also detects new or deleted files by comparing the total .md count.
func (s *Store) IsCacheStale() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isCacheStaleLocked()
}

// isCacheStaleLocked is the lock-free version for callers that already hold the lock.
func (s *Store) isCacheStaleLocked() bool {
	if s.cacheModTimes == nil {
		return true
	}
	for path, cachedMod := range s.cacheModTimes {
		info, err := os.Stat(path)
		if err != nil || !info.ModTime().Equal(cachedMod) {
			return true
		}
	}
	return s.walkVaultMdCountLocked() != s.cachedFileCnt
}

// walkVaultMdCountLocked counts .md files in the vault (excluding dotdirs and
// Obsidian files). Only touches the filesystem, not Store state — safe to call
// with or without a lock.
func (s *Store) walkVaultMdCountLocked() int {
	var cnt int
	err := filepath.WalkDir(s.VaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") && !IsObsidianFile(path) {
			cnt++
		}
		return nil
	})
	// An incomplete scan cannot validate the cached count.
	if err != nil {
		return -1
	}
	return cnt
}

// Update actualiza la fecha de modificación y reescribe el archivo.
func (s *Store) Update(note *Note) error {
	note.Updated = time.Now()
	note.RawContent = ToMarkdown(note)

	if err := os.WriteFile(note.Path, []byte(note.RawContent), 0o600); err != nil {
		return fmt.Errorf("notes: writing note %q: %w", note.Path, err)
	}
	s.InvalidateCache()
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
	s.InvalidateCache()
	return nil
}

// ListTrash returns Markdown notes currently stored in the vault trash.
func (s *Store) ListTrash() ([]*Note, error) {
	trashDir := filepath.Join(s.VaultPath, ".trash")
	var trashed []*Note

	err := filepath.WalkDir(trashDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		note, err := ParseFile(path)
		if err != nil {
			return nil
		}
		if rel, err := filepath.Rel(s.VaultPath, path); err == nil {
			note.RelPath = rel
		}
		trashed = append(trashed, note)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("notes: walking trash: %w", err)
	}

	SortNotes(trashed, "updated")
	return trashed, nil
}

// Restore moves a note from .trash back into the vault root or target folder.
func (s *Store) Restore(idOrTitle string, targetFolder string) (*Note, error) {
	trashed, err := s.ListTrash()
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(idOrTitle)
	var note *Note
	for _, n := range trashed {
		stem := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		if n.ID == idOrTitle || strings.ToLower(n.Title) == lowerQuery || strings.ToLower(stem) == lowerQuery {
			note = n
			break
		}
	}
	if note == nil {
		return nil, ErrNoteNotFound
	}

	targetDir, err := s.vaultSubdir(targetFolder)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return nil, fmt.Errorf("notes: creating restore target: %w", err)
	}

	dest := filepath.Join(targetDir, filepath.Base(note.Path))
	if _, statErr := os.Stat(dest); statErr == nil {
		stem := strings.TrimSuffix(filepath.Base(note.Path), ".md")
		dest = filepath.Join(targetDir, fmt.Sprintf("%s-%d.md", stem, time.Now().UnixNano()))
	}
	if err := os.Rename(note.Path, dest); err != nil {
		return nil, fmt.Errorf("notes: restoring note: %w", err)
	}

	restored, err := ParseFile(dest)
	if err != nil {
		return nil, err
	}
	if rel, err := filepath.Rel(s.VaultPath, dest); err == nil {
		restored.RelPath = rel
	}
	s.InvalidateCache()
	return restored, nil
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

	assetsDir, err := s.assetsDir()
	if err != nil {
		return err
	}
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
		if filename == "" || filename == "/" || filename == "." {
			filename = fmt.Sprintf("download-%d.jpg", time.Now().UnixNano())
		}

		// Ensure unique filename
		localDest = filepath.Join(assetsDir, filename)
		if _, err := os.Stat(localDest); err == nil {
			filename = fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)
			localDest = filepath.Join(assetsDir, filename)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(pathOrURL)
		if err != nil {
			return fmt.Errorf("downloading asset: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %s", resp.Status)
		}
		if resp.ContentLength > maxAssetDownloadBytes {
			return fmt.Errorf("asset too large: %d bytes exceeds %d byte limit", resp.ContentLength, maxAssetDownloadBytes)
		}

		out, err := os.Create(localDest)
		if err != nil {
			return fmt.Errorf("creating asset file: %w", err)
		}
		defer out.Close()

		written, err := io.Copy(out, io.LimitReader(resp.Body, maxAssetDownloadBytes+1))
		if err != nil {
			return fmt.Errorf("saving asset: %w", err)
		}
		if written > maxAssetDownloadBytes {
			_ = os.Remove(localDest)
			return fmt.Errorf("asset too large: exceeds %d byte limit", maxAssetDownloadBytes)
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

	// Append to note with path relative to note's directory
	ext := strings.ToLower(filepath.Ext(filename))
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".svg" || ext == ".avif"

	noteDir := filepath.Dir(note.Path)
	relPath, err := filepath.Rel(noteDir, filepath.Join(assetsDir, filename))
	if err != nil {
		relPath = filepath.Join("assets", filename)
	}
	mdPath := filepath.ToSlash(relPath)
	var appendText string
	if isImage {
		appendText = fmt.Sprintf("\n\n![%s](%s)", filename, mdPath)
	} else {
		appendText = fmt.Sprintf("\n\n[%s](%s)", filename, mdPath)
	}

	note.Content += appendText
	return s.Update(note)
}
