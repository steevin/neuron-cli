package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, "127.0.0.1:7878")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func request(t *testing.T, s *Server, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, "http://"+s.host+target, bytes.NewReader(data))
	r.Header.Set("X-Neuron-Token", s.token)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func decodeNote(t *testing.T, w *httptest.ResponseRecorder) Note {
	t.Helper()
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	var n Note
	if err := json.Unmarshal(w.Body.Bytes(), &n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestAuthenticationAndOrigin(t *testing.T) {
	s, _ := fixture(t)
	for _, tc := range []struct {
		host, token, origin string
		code                int
	}{
		{s.host, "", "", 401}, {"attacker.example", s.token, "", 403}, {s.host, s.token, "https://attacker.example", 403}, {s.host, s.token, "http://" + s.host, 200},
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/api/notes", nil)
		r.Header.Set("X-Neuron-Token", tc.token)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
	}
	w := request(t, s, "GET", "/api/notes", nil)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie not protected")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing CSP")
	}
}
func TestCreateSearchSaveAndConflict(t *testing.T) {
	s, dir := fixture(t)
	n := decodeNote(t, request(t, s, "POST", "/api/notes", map[string]any{"title": "API Plan", "folder": "Projects", "tags": []string{"work"}, "content": "# Plan\n- [ ] Build web\n[[Related]]\n"}))
	if n.Path != "Projects/api-plan.md" || n.Tasks != 1 || n.Version == "" {
		t.Fatalf("%+v", n)
	}
	n2 := decodeNote(t, request(t, s, "POST", "/api/notes", map[string]any{"title": "API Plan", "folder": "Projects"}))
	if n2.Path == n.Path {
		t.Fatal("duplicate title overwrote note")
	}
	w := request(t, s, "GET", "/api/notes?q=build", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), n.Path) || strings.Contains(w.Body.String(), n2.Path) {
		t.Fatal(w.Body.String())
	}
	original, err := os.ReadFile(filepath.Join(dir, n.Path))
	if err != nil {
		t.Fatal(err)
	}
	prefix, _ := splitBody(original)
	updated := decodeNote(t, request(t, s, "PUT", "/api/note", map[string]string{"path": n.Path, "version": n.Version, "content": "# Plan\n- [x] Build web\n"}))
	if updated.Tasks != 0 || updated.Version == n.Version {
		t.Fatal("save did not change content")
	}
	raw, _ := os.ReadFile(filepath.Join(dir, n.Path))
	if !bytes.HasPrefix(raw, prefix) {
		t.Fatal("frontmatter changed")
	}
	w = request(t, s, "PUT", "/api/note", map[string]string{"path": n.Path, "version": n.Version, "content": "Stale overwrite"})
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := os.WriteFile(filepath.Join(dir, n.Path), []byte("External edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w = request(t, s, "PUT", "/api/note", map[string]string{"path": n.Path, "version": updated.Version, "content": "Overwrite external"})
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	raw, _ = os.ReadFile(filepath.Join(dir, n.Path))
	if string(raw) != "External edit\n" {
		t.Fatal("external edit lost")
	}
}
func TestSavePreservesCRLFAndPermissions(t *testing.T) {
	s, dir := fixture(t)
	raw := "---\r\ntitle: Original\r\ncustom: keep\r\n---\r\n\r\nText\r\n"
	if err := os.WriteFile(filepath.Join(dir, "original.md"), []byte(raw), 0640); err != nil {
		t.Fatal(err)
	}
	n := decodeNote(t, request(t, s, "GET", "/api/note?path=original.md", nil))
	decodeNote(t, request(t, s, "PUT", "/api/note", map[string]string{"path": n.Path, "version": n.Version, "content": "Updated\n"}))
	result, _ := os.ReadFile(filepath.Join(dir, "original.md"))
	want := strings.Replace(raw, "Text", "Updated", 1)
	if string(result) != want {
		t.Fatalf("%q != %q", result, want)
	}
	info, _ := os.Stat(filepath.Join(dir, "original.md"))
	if info.Mode().Perm() != 0640 {
		t.Fatal("permissions changed")
	}
}
func TestVaultBoundariesAndAssets(t *testing.T) {
	s, dir := fixture(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.md"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
		t.Skip(err)
	}
	for _, folder := range []string{"../outside", "/absolute", ".neuron", "escape"} {
		w := request(t, s, "POST", "/api/notes", map[string]string{"title": "Forbidden", "folder": folder})
		if w.Code < 400 {
			t.Fatalf("accepted %s", folder)
		}
	}
	for _, name := range []string{"../private.md", "escape/private.md", ".neuron/config.md"} {
		w := request(t, s, "GET", "/api/note?path="+name, nil)
		if w.Code < 400 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "image.svg"), []byte("<svg onload='alert(1)'/>"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := request(t, s, "GET", "/api/asset?path=image.svg", nil); w.Code != 415 {
		t.Fatal(w.Code)
	}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), []byte{137, 80, 78, 71, 13, 10, 26, 10}, 0600); err != nil {
		t.Fatal(err)
	}
	if w := request(t, s, "GET", "/api/asset?path=image.png", nil); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal(w.Code)
	}
}
func TestMarkdownSafetyAndWikiLinks(t *testing.T) {
	s, dir := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	content := "# Heading\n- [x] Done\n[[Related|Read this]]\n\n`[[Not a link]]`\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n![local](photo.png)\n\n![remote](https://example.com/track.png)\n\n| A | B |\n|---|---|\n| 1 | 2 |"
	result := renderMarkdown(content, "note.md", s.root)
	for _, bad := range []string{"<script", "javascript:", `src="https://`} {
		if strings.Contains(result, bad) {
			t.Fatal(result)
		}
	}
	for _, want := range []string{`data-wiki="Related"`, `href="#note"`, `<input`, `checked`, "Read this", "<code>[[Not a link]]</code>", "/api/asset?path=photo.png", "<table>"} {
		if !strings.Contains(result, want) {
			t.Fatalf("missing %s in %s", want, result)
		}
	}
}
func TestMalformedNoteIsReportedAndDoesNotHideGoodNotes(t *testing.T) {
	s, dir := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\ntitle: [\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.md"), []byte("# Good\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request(t, s, "GET", "/api/notes", nil)
	if !strings.Contains(w.Body.String(), "Cannot parse broken.md") || !strings.Contains(w.Body.String(), "good.md") {
		t.Fatal(w.Body.String())
	}
	if w := request(t, s, "GET", "/missing.js", nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
