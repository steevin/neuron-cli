// Package web serves the embedded, local-only Neuron workspace.
package web

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/steevin/neuron-cli/internal/notes"
)

//go:embed assets/*
var assets embed.FS

const maxNoteSize = 4 << 20

type Server struct {
	root                *os.Root
	host, token, cookie string
	mu                  sync.Mutex
}

type Note struct {
	Path    string    `json:"path"`
	Title   string    `json:"title"`
	Folder  string    `json:"folder"`
	Tags    []string  `json:"tags"`
	Links   []string  `json:"links"`
	Updated time.Time `json:"updated"`
	Excerpt string    `json:"excerpt"`
	Tasks   int       `json:"tasks"`
	Version string    `json:"version"`
	Content string    `json:"content,omitempty"`
	HTML    string    `json:"html,omitempty"`
}

// New confines filesystem operations to vault and authenticates each session.
// host must be the address of the actual loopback listener.
func New(vault, host string) (*Server, error) {
	root, err := os.OpenRoot(vault)
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		root.Close()
		return nil, err
	}
	token := hex.EncodeToString(secret)
	return &Server{root: root, host: host, token: token, cookie: "neuron_" + token[:12]}, nil
}
func (s *Server) Close() error { return s.root.Close() }
func (s *Server) URL() string  { return "http://" + s.host + "/#session=" + s.token }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if r.Host != s.host {
		fail(w, http.StatusForbidden, "Invalid local server address")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+s.host {
		fail(w, http.StatusForbidden, "Cross-origin requests are not allowed")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		token := r.Header.Get("X-Neuron-Token")
		if token == "" {
			if c, err := r.Cookie(s.cookie); err == nil {
				token = c.Value
			}
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			fail(w, http.StatusUnauthorized, "Open the session URL printed by neuron web")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: s.token, Path: "/api/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		s.mu.Lock()
		defer s.mu.Unlock()
		s.api(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		fail(w, 405, "Method not allowed")
		return
	}
	name := map[string]string{"/": "index.html", "/app.js": "app.js", "/style.css": "style.css"}[r.URL.Path]
	if name == "" {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("assets/" + name)
	if err != nil {
		fail(w, 500, "Missing application asset")
		return
	}
	types := map[string]string{"index.html": "text/html; charset=utf-8", "app.js": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8"}
	w.Header().Set("Content-Type", types[name])
	if r.Method != http.MethodHead {
		w.Write(data)
	}
}
func fail(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxNoteSize+16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "Invalid or oversized JSON request")
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "Expected one JSON object")
		return false
	}
	return true
}
func cleanPath(name string, allowRoot bool) (string, error) {
	if allowRoot && (name == "" || name == ".") {
		return ".", nil
	}
	if strings.Contains(name, "\\") || !fs.ValidPath(name) {
		return "", fmt.Errorf("use a path relative to the vault")
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("hidden paths are not available in the web workspace")
		}
	}
	return name, nil
}
func (s *Server) read(name string) ([]byte, error) {
	if _, err := cleanPath(name, false); err != nil {
		return nil, err
	}
	if !strings.EqualFold(path.Ext(name), ".md") {
		return nil, fmt.Errorf("expected a Markdown note")
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxNoteSize {
		return nil, fmt.Errorf("note must be a regular file under 4 MiB")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxNoteSize+1))
	if len(raw) > maxNoteSize {
		return nil, fmt.Errorf("note exceeds 4 MiB")
	}
	return raw, err
}
func version(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// splitBody preserves the exact frontmatter bytes, including CRLF and spacing.
func splitBody(raw []byte) (prefix, body []byte) {
	lines := bytes.SplitAfter(raw, []byte("\n"))
	if len(lines) < 2 || strings.TrimRight(string(lines[0]), "\r\n") != "---" {
		return nil, raw
	}
	offset := len(lines[0])
	for _, line := range lines[1:] {
		offset += len(line)
		if strings.TrimRight(string(line), "\r\n") == "---" {
			if bytes.HasPrefix(raw[offset:], []byte("\r\n")) {
				offset += 2
			} else if bytes.HasPrefix(raw[offset:], []byte("\n")) {
				offset++
			}
			return raw[:offset], raw[offset:]
		}
	}
	return nil, raw
}
func (s *Server) describe(name string, raw []byte, full bool) (Note, error) {
	n, err := notes.ParseContent(string(raw), name)
	if err != nil {
		return Note{}, err
	}
	info, err := s.root.Stat(name)
	if err != nil {
		return Note{}, err
	}
	_, body := splitBody(raw)
	excerpt := []rune(strings.Join(strings.Fields(string(body)), " "))
	if len(excerpt) > 150 {
		excerpt = excerpt[:150]
	}
	pending := 0
	for _, t := range notes.Tasks(n) {
		if !t.Done {
			pending++
		}
	}
	folder := path.Dir(name)
	if folder == "." {
		folder = ""
	}
	item := Note{Path: name, Title: n.Title, Folder: folder, Tags: n.Tags, Links: n.Links, Updated: info.ModTime(), Excerpt: string(excerpt), Tasks: pending, Version: version(raw)}
	if full {
		item.Content = string(body)
		item.HTML = renderMarkdown(string(body), name, s.root)
	}
	return item, nil
}
func (s *Server) list() ([]Note, []string, []string, error) {
	list := []Note{}
	folders := []string{}
	warnings := []string{}
	err := fs.WalkDir(s.root.FS(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, "Cannot read "+name)
			return nil
		}
		if name == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			folders = append(folders, name)
			return nil
		}
		if !strings.EqualFold(path.Ext(name), ".md") {
			return nil
		}
		raw, err := s.read(name)
		if err != nil {
			warnings = append(warnings, "Cannot read "+name)
			return nil
		}
		item, err := s.describe(name, raw, false)
		if err != nil {
			warnings = append(warnings, "Cannot parse "+name)
			return nil
		}
		list = append(list, item)
		return nil
	})
	sort.Slice(list, func(i, j int) bool {
		if list[i].Updated.Equal(list[j].Updated) {
			return list[i].Path < list[j].Path
		}
		return list[i].Updated.After(list[j].Updated)
	})
	sort.Strings(folders)
	return list, folders, warnings, err
}
func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/notes" && r.Method == "GET":
		list, folders, warnings, err := s.list()
		if err != nil {
			fail(w, 500, "Could not scan vault")
			return
		}
		if query := strings.TrimSpace(r.URL.Query().Get("q")); query != "" {
			filtered := []Note{}
			for _, n := range list {
				raw, err := s.read(n.Path)
				if err != nil {
					continue
				}
				haystack := strings.ToLower(n.Title + " " + string(raw))
				match := true
				for _, word := range strings.Fields(strings.ToLower(query)) {
					if !strings.Contains(haystack, word) {
						match = false
						break
					}
				}
				if match {
					filtered = append(filtered, n)
				}
			}
			list = filtered
		}
		respond(w, map[string]any{"notes": list, "folders": folders, "warnings": warnings, "vault": path.Base(s.root.Name())})
	case r.URL.Path == "/api/note" && r.Method == "GET":
		name := r.URL.Query().Get("path")
		raw, err := s.read(name)
		if err != nil {
			fail(w, 404, "Note unavailable: "+err.Error())
			return
		}
		n, err := s.describe(name, raw, true)
		if err != nil {
			fail(w, 422, "Could not parse note: "+err.Error())
			return
		}
		respond(w, n)
	case r.URL.Path == "/api/notes" && r.Method == "POST":
		var input struct {
			Title   string   `json:"title"`
			Folder  string   `json:"folder"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		}
		if !decode(w, r, &input) {
			return
		}
		input.Title = strings.TrimSpace(input.Title)
		if input.Title == "" || len([]rune(input.Title)) > 160 || len(input.Content) > maxNoteSize {
			fail(w, 400, "Use a title of 1–160 characters and content under 4 MiB")
			return
		}
		folder, err := cleanPath(input.Folder, true)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if err = s.root.MkdirAll(folder, 0700); err != nil {
			fail(w, 400, "Cannot create folder inside vault")
			return
		}
		var slug strings.Builder
		for _, r := range strings.ToLower(input.Title) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				slug.WriteRune(r)
			} else if slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-") {
				slug.WriteByte('-')
			}
			if slug.Len() > 70 {
				break
			}
		}
		stem := strings.Trim(slug.String(), "-")
		if stem == "" {
			stem = "note"
		}
		name := path.Join(folder, stem+".md")
		now := time.Now()
		n := &notes.Note{ID: uuid.NewString(), Title: input.Title, Tags: input.Tags, Content: input.Content, Created: now, Updated: now}
		raw := []byte(notes.ToMarkdown(n))
		f, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			name = path.Join(folder, stem+"-"+uuid.NewString()[:8]+".md")
			f, err = s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		}
		if err != nil {
			fail(w, 400, "Cannot create note inside vault")
			return
		}
		_, err = f.Write(raw)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			fail(w, 500, "Could not save note")
			return
		}
		item, err := s.describe(name, raw, true)
		if err != nil {
			fail(w, 500, "Note created but could not be read")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		respond(w, item)
	case r.URL.Path == "/api/note" && r.Method == "PUT":
		var input struct {
			Path    string `json:"path"`
			Content string `json:"content"`
			Version string `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		if len(input.Content) > maxNoteSize {
			fail(w, 400, "Note exceeds 4 MiB")
			return
		}
		raw, err := s.read(input.Path)
		if err != nil {
			fail(w, 404, "Note is no longer available")
			return
		}
		if input.Version == "" || input.Version != version(raw) {
			fail(w, 409, "This note changed outside this editor. Your draft has been kept.")
			return
		}
		prefix, _ := splitBody(raw)
		body := strings.ReplaceAll(input.Content, "\r\n", "\n")
		if bytes.Contains(raw, []byte("\r\n")) {
			body = strings.ReplaceAll(body, "\n", "\r\n")
		}
		updated := append(append([]byte{}, prefix...), []byte(body)...)
		if len(updated) > maxNoteSize {
			fail(w, 400, "Note exceeds 4 MiB")
			return
		}
		if _, err := notes.ParseContent(string(updated), input.Path); err != nil {
			fail(w, 422, "Invalid Markdown frontmatter: "+err.Error())
			return
		}
		info, err := s.root.Stat(input.Path)
		if err != nil {
			fail(w, 404, "Note is no longer available")
			return
		}
		temp := path.Join(path.Dir(input.Path), ".neuron-save-"+uuid.NewString())
		f, err := s.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			fail(w, 500, "Cannot prepare note save")
			return
		}
		defer s.root.Remove(temp)
		_, err = f.Write(updated)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			fail(w, 500, "Could not write note")
			return
		}
		latest, err := s.read(input.Path)
		if err != nil || version(latest) != input.Version {
			fail(w, 409, "Note changed during save. Your draft has been kept.")
			return
		}
		if err = s.root.Rename(temp, input.Path); err != nil {
			fail(w, 500, "Could not replace note")
			return
		}
		item, err := s.describe(input.Path, updated, true)
		if err != nil {
			fail(w, 500, "Saved note could not be read")
			return
		}
		respond(w, item)
	case r.URL.Path == "/api/preview" && r.Method == "POST":
		var input struct {
			Content string `json:"content"`
			Path    string `json:"path"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.Path != "" {
			if _, err := cleanPath(input.Path, false); err != nil {
				fail(w, 400, err.Error())
				return
			}
		}
		respond(w, map[string]string{"html": renderMarkdown(input.Content, input.Path, s.root)})
	case r.URL.Path == "/api/asset" && r.Method == "GET":
		name := r.URL.Query().Get("path")
		if _, err := cleanPath(name, false); err != nil {
			fail(w, 400, err.Error())
			return
		}
		types := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif"}
		mime, ok := types[strings.ToLower(path.Ext(name))]
		if !ok {
			fail(w, 415, "Only local raster images are served")
			return
		}
		f, err := s.root.Open(name)
		if err != nil {
			fail(w, 404, "Image unavailable")
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 50<<20 {
			fail(w, 413, "Image unavailable or too large")
			return
		}
		w.Header().Set("Content-Type", mime)
		http.ServeContent(w, r, path.Base(name), info.ModTime(), f)
	default:
		fail(w, 404, "Unknown endpoint or unsupported method")
	}
}
