package web

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"nucleaus/internal/snippets"
)

// The snippet vault lives in SQLite. SetSnippetStore wires the store at boot;
// when it is absent (tests, cold checkouts) the pages fall back to the legacy
// markdown sources and pending placeholders.

var snippetStore *snippets.Store

func SetSnippetStore(s *snippets.Store) { snippetStore = s }

// snipRow pairs a snippet with its label name for management views.
type snipRow struct {
	snippets.Snippet
	LabelName string
}

// renderSnippetBody renders a stored snippet's markdown as section HTML and
// counts its copyable code blocks.
func renderSnippetBody(code string) (body template.HTML, blocks int) {
	doc := parseMarkdown(code)
	var b strings.Builder
	for _, sec := range doc.Sections {
		b.WriteString(string(sec.HTML))
		blocks += sec.Count
	}
	return template.HTML(b.String()), blocks
}

// withHeading guarantees the stored markdown opens with the "## title" line
// the section parser needs; user-entered plain code gets one prepended.
func withHeading(title, code string) string {
	for _, line := range strings.Split(code, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			return code
		}
	}
	return "## " + strings.TrimSpace(title) + "\n\n" + strings.Trim(code, "\n")
}

// buildSnippetDoc assembles the per-label doc view straight from the database.
func buildSnippetDoc(ctx context.Context, slug string) (docView, error) {
	label, ok, err := snippetStore.LabelBySlug(ctx, slug)
	if err != nil {
		return docView{}, err
	}
	if !ok {
		return docView{}, fmt.Errorf("no label %q", slug)
	}
	rows, err := snippetStore.ListSnippets(ctx, label.ID)
	if err != nil {
		return docView{}, err
	}
	if len(rows) == 0 {
		return docView{}, fmt.Errorf("label %q has no snippets", slug)
	}

	var tabOrder []string
	tabs := map[string]*docGroupView{}
	seen := map[string]int{}
	sections := 0
	for _, sn := range rows {
		key := sn.Source
		if _, exists := tabs[key]; !exists {
			label := "Custom"
			if key != "custom" {
				label = docLabelFor(key)
			}
			tabs[key] = &docGroupView{Slug: key, Label: label}
			tabOrder = append(tabOrder, key)
		}
		body, count := renderSnippetBody(withHeading(sn.Title, sn.Code))
		tabs[key].Sections = append(tabs[key].Sections, docSectionView{
			ID:        uniqueSlug(slugify(sn.Title), seen),
			Heading:   sn.Title,
			Query:     strings.ToLower(mdPlain(sn.Title) + " " + sn.Code),
			Count:     count,
			Body:      body,
			SnippetID: sn.ID,
			Editable:  true,
		})
		sections++
	}

	view := docView{
		Slug:  label.Slug,
		Label: label.Name,
		Desc:  "Snippet vault stored in the database.",
		Count: sections,
	}
	for _, key := range tabOrder {
		t := tabs[key]
		t.Count = len(t.Sections)
		view.Tabs = append(view.Tabs, *t)
	}
	return view, nil
}

// dbLabels lists labels when a store is wired; otherwise empty.
func dbLabels(ctx context.Context) ([]snippets.Label, error) {
	if snippetStore == nil {
		return nil, nil
	}
	return snippetStore.ListLabels(ctx)
}

func handleSnippetsCatalog(w http.ResponseWriter, r *http.Request) {
	g, _ := findGroup("snippets")

	items := make([]link, 0, len(snippetLanguages)+4)
	for _, l := range snippetLanguages {
		items = append(items, view(l.Name, l.Desc, "/snippets/"+l.Key, l.Logo))
	}
	known := map[string]bool{}
	for _, l := range snippetLanguages {
		known[l.Key] = true
	}
	if snippetStore != nil {
		labels, err := snippetStore.ListLabels(r.Context())
		if err != nil {
			log.Printf("snippets catalog: %v", err)
		}
		for _, lb := range labels {
			if known[lb.Slug] {
				continue
			}
			items = append(items, view(lb.Name, fmt.Sprintf("%d stored snippets.", lb.Count), "/snippets/"+lb.Slug, ""))
		}
	}

	Render(w, "snippets_page", pageData{
		Title:  g.Label,
		Page:   "snippets",
		Nav:    nav(),
		Group:  g,
		Groups: []group{{Key: g.Key, Label: g.Label, Tag: g.Tag, Blurb: g.Blurb, Items: items}},
		HasDB:  snippetStore != nil,
	})
}

func handleSnippetDoc(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if snippetStore != nil {
		view, err := buildSnippetDoc(r.Context(), slug)
		if err == nil {
			Render(w, "doc_page", pageData{
				Title: view.Label + " · Snippet",
				Page:  "snippets",
				Nav:   nav(),
				Trail: "Snippet",
				Doc:   &view,
			})
			return
		}
		if !strings.Contains(err.Error(), "no label") {
			log.Printf("snippets db %s: %v", slug, err)
		}
	}

	// Unknown outside the static language set: neither the database nor the
	// legacy vault knows this slug.
	known := false
	for _, l := range snippetLanguages {
		if l.Key == slug {
			known = true
			break
		}
	}
	if !known {
		notFound(w)
		return
	}

	// Markdown fallback for languages that still have vault files on disk.
	view, err := docStore.load("snippets:"+slug, buildSnippets)
	if err != nil && err == errNoSnippet {
		group, ok := findGroup("snippets")
		if ok {
			if item, ok := findLink(group, "/snippets/"+slug); ok {
				item.Meta = "PENDING"
				renderSoon(w, group, item, "Snippet")
				return
			}
		}
		notFound(w)
		return
	}

	data := pageData{
		Title: view.Label + " · Snippet",
		Page:  "snippets",
		Nav:   nav(),
		Trail: "Snippet",
		Doc:   &view,
	}
	if err != nil && err != errNoSnippet {
		data.Doc.Error = err.Error()
	}
	Render(w, "doc_page", data)
}

// ---------- add / edit / delete ----------

func snippetInput(r *http.Request) snippets.SnippetInput {
	in := snippets.SnippetInput{
		Title:  r.FormValue("title"),
		Code:   r.FormValue("code"),
		Notes:  r.FormValue("notes"),
		Lang:   r.FormValue("lang"),
		Source: "custom",
	}
	in.LabelID, _ = strconv.ParseInt(r.FormValue("label_id"), 10, 64)
	return in
}

func handleSnippetNew(w http.ResponseWriter, r *http.Request) {
	labels, _ := dbLabels(r.Context())
	Render(w, "snippet_form_page", pageData{
		Title:  "New snippet · Snippet",
		Page:   "snippets",
		Nav:    nav(),
		Trail:  "Snippet",
		Labels: labels,
		HasDB:  snippetStore != nil,
	})
}

func handleSnippetCreate(w http.ResponseWriter, r *http.Request) {
	in := snippetInput(r)
	if _, err := snippetStore.CreateSnippet(r.Context(), in); err != nil {
		sn := snippets.Snippet{Title: in.Title, Code: in.Code, Notes: in.Notes, Lang: in.Lang, LabelID: idOrNull(in.LabelID)}
		labels, _ := dbLabels(r.Context())
		Render(w, "snippet_form_page", pageData{
			Title:     "New snippet · Snippet",
			Page:      "snippets",
			Nav:       nav(),
			Trail:     "Snippet",
			Snip:      &sn,
			SnipError: err.Error(),
			Labels:    labels,
		})
		return
	}
	http.Redirect(w, r, "/snippets/manage?saved=1", http.StatusSeeOther)
}

func handleSnippetManage(w http.ResponseWriter, r *http.Request) {
	rows, labels, err := snippetRows(r.Context())
	if err != nil {
		log.Printf("snippets manage: %v", err)
	}
	Render(w, "snippet_manage_page", pageData{
		Title:    "Manage snippets · Snippet",
		Page:     "snippets",
		Nav:      nav(),
		Trail:    "Snippet",
		SnipRows: rows,
		Labels:   labels,
		HasDB:    snippetStore != nil,
	})
}

func handleSnippetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	sn, ok, err := snippetStore.Snippet(r.Context(), id)
	if err != nil || !ok {
		notFound(w)
		return
	}
	labels, _ := dbLabels(r.Context())
	Render(w, "snippet_form_page", pageData{
		Title:  "Edit " + sn.Title + " · Snippet",
		Page:   "snippets",
		Nav:    nav(),
		Trail:  "Snippet",
		Snip:   &sn,
		Labels: labels,
		HasDB:  true,
	})
}

func handleSnippetUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	if err := snippetStore.UpdateSnippet(r.Context(), id, snippetInput(r)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/snippets/manage?saved=1", http.StatusSeeOther)
}

func handleSnippetDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	if err := snippetStore.DeleteSnippet(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/snippets/manage?deleted=1", http.StatusSeeOther)
}

func idOrNull(id int64) sql.NullInt64 {
	if id != 0 {
		return sql.NullInt64{Int64: id, Valid: true}
	}
	return sql.NullInt64{}
}

// ---------- labels ----------

func handleLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := dbLabels(r.Context())
	if err != nil {
		log.Printf("labels: %v", err)
	}
	Render(w, "labels_page", pageData{
		Title:  "Snippet labels · Snippet",
		Page:   "snippets",
		Nav:    nav(),
		Trail:  "Snippet",
		Labels: labels,
		HasDB:  snippetStore != nil,
	})
}

func handleLabelCreate(w http.ResponseWriter, r *http.Request) {
	if snippetStore == nil {
		http.Error(w, "database not wired", http.StatusServiceUnavailable)
		return
	}
	if _, err := snippetStore.CreateLabel(r.Context(), r.FormValue("name")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/labels?saved=1", http.StatusSeeOther)
}

func handleLabelDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		notFound(w)
		return
	}
	if err := snippetStore.DeleteLabel(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/labels?deleted=1", http.StatusSeeOther)
}

func snippetRows(ctx context.Context) ([]snipRow, []snippets.Label, error) {
	if snippetStore == nil {
		return nil, nil, nil
	}
	rows, err := snippetStore.ListSnippets(ctx, 0)
	if err != nil {
		return nil, nil, err
	}
	labels, err := snippetStore.ListLabels(ctx)
	if err != nil {
		return nil, nil, err
	}
	names := map[int64]string{}
	for _, l := range labels {
		names[l.ID] = l.Name
	}
	out := make([]snipRow, 0, len(rows))
	for _, sn := range rows {
		out = append(out, snipRow{Snippet: sn, LabelName: names[sn.LabelID.Int64]})
	}
	return out, labels, nil
}

func registerSnippets(mux *http.ServeMux) {
	mux.HandleFunc("GET /snippets", handleSnippetsCatalog)
	mux.HandleFunc("GET /snippets/new", handleSnippetNew)
	mux.HandleFunc("POST /snippets", handleSnippetCreate)
	mux.HandleFunc("GET /snippets/manage", handleSnippetManage)
	mux.HandleFunc("GET /snippets/manage/{id}/edit", handleSnippetEdit)
	mux.HandleFunc("POST /snippets/manage/{id}", handleSnippetUpdate)
	mux.HandleFunc("POST /snippets/manage/{id}/delete", handleSnippetDelete)
	mux.HandleFunc("GET /snippets/{slug}", handleSnippetDoc)
	mux.HandleFunc("GET /labels", handleLabels)
	mux.HandleFunc("POST /labels", handleLabelCreate)
	mux.HandleFunc("POST /labels/{id}/delete", handleLabelDelete)
}
