package web

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed templates/*.html static/* logos/*
var files embed.FS

var tmpl = template.Must(template.ParseFS(files, "templates/*.html"))

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

// RegisterStatic mounts the embedded assets and the hub page routes. The server
// binary calls this as its single web entrypoint.
//
// NOTE: the root path "/" belongs to the user package's demo page, so the hub
// itself lives at /hub.
func RegisterStatic(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.FileServerFS(files))
	mux.Handle("GET /logos/", http.FileServerFS(files))
	registerPages(mux)
	registerMetrics(mux)
}
