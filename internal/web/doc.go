package web

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"sync"
)

// Shared view model and loader for the two content tabs rendered from
// markdown: /curriculum/{slug} (docs/curriculum-room) and /snippets/{slug}
// (docs/snippets). Both reuse the cheat sheet's design language: a search
// box, optional per-source tabs and collapsible sections.

type docSectionView struct {
	ID      string
	Heading string
	Query   string
	Open    bool
	Count   int
	Body    template.HTML
}

type docGroupView struct {
	Slug     string
	Label    string
	Title    string
	Count    int
	Sections []docSectionView
}

type docResource struct {
	Kind string // "DOCS" or "VIDEO"
	Name string
	URL  string
	Desc string
}

type docView struct {
	Slug  string
	Label string
	Desc  string
	Logo  string
	Count int
	Tabs  []docGroupView // one group per source file
	Error string         // loader failure, rendered as an error panel

	// Curated pointers to official documentation and video courses.
	Resources []docResource
}

// resolveDir mirrors cheatsheet.go: fall back to docs/<name> in the parent
// directories so the server binary can run from cmd/server as well.
func resolveDir(env string, name string) string {
	if dir := os.Getenv(env); dir != "" {
		return dir
	}
	for _, dir := range []string{"docs/" + name, "../docs/" + name, "../../docs/" + name} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "docs/" + name
}

type docCache struct {
	mu   sync.Mutex
	docs map[string]docView
	errs map[string]error
}

var docStore = &docCache{docs: map[string]docView{}, errs: map[string]error{}}

// load caches view models per tab slug. build runs once per slug for the
// process lifetime.
func (c *docCache) load(slug string, build func(string) (docView, error)) (docView, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if doc, ok := c.docs[slug]; ok {
		return doc, nil
	}
	if err, ok := c.errs[slug]; ok {
		return docView{}, err
	}
	doc, err := build(slug)
	if err != nil {
		c.errs[slug] = err
		return docView{}, err
	}
	c.docs[slug] = doc
	return doc, nil
}

// sectionsFromDoc converts parsed markdown into collapsible sections. The
// first section opens by default; the rest stay closed.
func sectionsFromDoc(doc mdDoc) []docSectionView {
	seen := map[string]int{}
	out := make([]docSectionView, 0, len(doc.Sections))
	for i, sec := range doc.Sections {
		if sec.HTML == "" {
			continue
		}
		out = append(out, docSectionView{
			ID:      uniqueSlug(slugify(sec.Heading), seen),
			Heading: sec.Heading,
			Query:   sec.Query,
			Count:   sec.Count,
			Body:    sec.HTML,
			Open:    i == 0,
		})
	}
	return out
}

// groupView packages one source file under its tab.
func groupView(slug, label string, doc mdDoc) docGroupView {
	secs := sectionsFromDoc(doc)
	return docGroupView{
		Slug:     slug,
		Label:    label,
		Title:    doc.Title,
		Count:    len(secs),
		Sections: secs,
	}
}

// docLabelFor picks a friendly label for a file slug.
func docLabelFor(slug string) string {
	if label, ok := docLabels[slug]; ok {
		return label
	}
	words := strings.Split(strings.ReplaceAll(slug, "-", " "), " ")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func readDocFile(path string) (mdDoc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return mdDoc{}, fmt.Errorf("read %s: %w", path, err)
	}
	return parseMarkdown(string(raw)), nil
}

// sortDocFiles orders source file names by the cheat sheet's canonical order
// and falls back to alphabetical.
func sortDocFiles(names []string) []string {
	rank := func(slug string) int {
		for i, s := range docOrder {
			if s == slug {
				return i
			}
		}
		return len(docOrder)
	}
	sort.Slice(names, func(i, j int) bool {
		si, sj := docSlug(names[i]), docSlug(names[j])
		ri, rj := rank(si), rank(sj)
		if ri == rj {
			return si < sj
		}
		return ri < rj
	})
	return names
}
