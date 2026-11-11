package feedserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerServesOnlyXMLFiles(t *testing.T) {
	publicDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(publicDir, "sample.xml"), []byte("<rss/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := New(publicDir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	request := httptest.NewRequest(http.MethodGet, "/sample.xml", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "<rss/>" {
		t.Fatalf("response = %d %q, want 200 and RSS body", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/rss+xml; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want RSS media type", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestServerRejectsNonXMLPathsAndDirectoryListing(t *testing.T) {
	publicDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(publicDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	server, err := New(publicDir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	for _, target := range []string{"/", "/nested/", "/nested/sample.xml", "/secret.json", "/../secret.xml"} {
		t.Run(target, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", response.Code)
			}
		})
	}
}

func TestServerRejectsMethodsOtherThanGetAndHead(t *testing.T) {
	server, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/sample.xml", strings.NewReader("ignored")))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.Code)
	}
	if got := response.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q, want GET, HEAD", got)
	}
}

func TestServerDoesNotServeSymlinksOutsidePublicDirectory(t *testing.T) {
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	if err := os.Mkdir(publicDir, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(root, "secret.xml")
	if err := os.WriteFile(secret, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(publicDir, "leak.xml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	server, err := New(publicDir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/leak.xml", nil))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "private") {
		t.Fatalf("response = %d %q, want 404 without file contents", response.Code, response.Body.String())
	}
}
