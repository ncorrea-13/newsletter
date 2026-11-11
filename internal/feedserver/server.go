package feedserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	root *os.Root
}

func New(directory string) (*Server, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	return &Server{root: root}, nil
}

func (s *Server) Close() error {
	return s.root.Close()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" || strings.Contains(name, "/") || filepath.Ext(name) != ".xml" {
		http.NotFound(w, r)
		return
	}

	file, err := s.root.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, info.ModTime(), file)
}
