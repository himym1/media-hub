package webui

import (
	"embed"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var embedded embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return &handler{root: root}
}

type handler struct {
	root fs.FS
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if requestPath == "." || requestPath == "" {
		h.serveIndex(w, r)
		return
	}
	if strings.HasPrefix(requestPath, "api/") {
		http.NotFound(w, r)
		return
	}

	content, err := fs.ReadFile(h.root, requestPath)
	if err == nil {
		if contentType := mime.TypeByExtension(path.Ext(requestPath)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if strings.HasPrefix(requestPath, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		_, _ = w.Write(content)
		return
	}
	if !errors.Is(err, fs.ErrNotExist) || path.Ext(requestPath) != "" {
		http.NotFound(w, r)
		return
	}
	h.serveIndex(w, r)
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	content, err := fs.ReadFile(h.root, "index.html")
	if err != nil {
		http.Error(w, "Web application is unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}
