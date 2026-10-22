package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
)

//go:embed templates/*.html static/* logos/*
var files embed.FS

// assetVersion summarises static/ and logos/ into a short hash so asset URLs
// change automatically whenever the embedded files do; long-lived browser
// caches never serve stale CSS or JS after a deploy.
var assetVersion = sync.OnceValue(func() string {
	h := sha256.New()
	var names []string
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == "." || path == "static" || path == "logos" {
				return nil
			}
			return fs.SkipDir
		}
		if strings.HasPrefix(path, "static/") || strings.HasPrefix(path, "logos/") {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		log.Printf("asset hash walk: %v", err)
		return "1"
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := files.ReadFile(name)
		if err != nil {
			log.Printf("asset hash read %s: %v", name, err)
			continue
		}
		h.Write([]byte(name))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
})

// assetURL appends the asset hash as a cache-busting query.
func assetURL(path string) string {
	return path + "?v=" + assetVersion()
}

var tmpl = template.Must(template.New("hub").Funcs(template.FuncMap{
	"asset": assetURL,
}).ParseFS(files, "templates/*.html"))

// Render buffers the output so a template error doesn't send half a page.
func Render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// cachedFileServer serves embedded assets with long-lived cache headers since
// their contents only change with a new binary build.
func cachedFileServer(cache string) http.Handler {
	files := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cache)
		files.ServeHTTP(w, r)
	})
}

// RegisterStatic mounts the embedded assets and the hub page routes. The server
// binary calls this as its single web entrypoint.
//
// NOTE: the root path "/" belongs to the user package's demo page, so the hub
// itself lives at /hub.
func RegisterStatic(mux *http.ServeMux) {
	mux.Handle("GET /static/", cachedFileServer("public, max-age=86400"))
	mux.Handle("GET /logos/", cachedFileServer("public, max-age=86400"))
	registerPages(mux)
	registerMetrics(mux)
}
