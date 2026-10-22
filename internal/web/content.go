package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The curriculum tab renders the learning tracks stored as markdown under
// docs/curriculum-room. Each track gets its own collapsible, searchable page.

type curriculumTrack struct {
	Slug  string
	Name  string
	Desc  string
	Logo  string
	Dir   string
	File  string
	Blurb string // source doc filename, shown as provenance
}

var curriculumTracks = []curriculumTrack{
	{
		Slug: "go", Name: "Go", Logo: "golang", Dir: "go",
		File: "golang-detailed-curriculum-revised.md",
		Desc: "Concurrency, services, and idiomatic stdlib.",
	},
	{
		Slug: "typescript", Name: "TypeScript", Logo: "ts", Dir: "ts",
		File: "typescript-detailed-curriculum-revised.md",
		Desc: "Types, tooling, and front-end architecture.",
	},
	{
		Slug: "postgres", Name: "Postgres", Logo: "postgres", Dir: "pg",
		File: "pg_curriculum.md",
		Desc: "Queries, schema design, and production reliability.",
	},
	{
		Slug: "docker", Name: "Docker", Logo: "docker", Dir: "docker",
		File: "docker_curriculum.md",
		Desc: "Images, networks, and compose in production.",
	},
	{
		Slug: "rust", Name: "Rust", Logo: "rust", Dir: "rust",
		File: "rust_curriculum_revised.md",
		Desc: "Ownership, traits, and fearless systems code.",
	},
	{
		Slug: "git", Name: "Git", Logo: "git", Dir: "git",
		File: "git_curriculum.md",
		Desc: "Version control from first commit to team workflow.",
	},
	{
		Slug: "gin", Name: "Gin", Logo: "gin", Dir: "gin",
		File: "gin_curriculum.md",
		Desc: "Routing, middleware, and JSON services with Gin.",
	},
	{
		Slug: "chi", Name: "Chi", Logo: "chi", Dir: "chi",
		File: "chi_curriculum.md",
		Desc: "Lightweight, idiomatic routers on net/http.",
	},
	{
		Slug: "hono", Name: "Hono", Logo: "hono", Dir: "hono",
		File: "hono_curriculum.md",
		Desc: "Ultrafast JavaScript apps on any runtime.",
	},
	{
		Slug: "echo", Name: "Echo", Logo: "echo", Dir: "echo",
		File: "echo_curriculum.md",
		Desc: "Minimal but extensible Go web services.",
	},
	{
		Slug: "fiber", Name: "Fiber", Logo: "fiber", Dir: "fiber",
		File: "fiber_curriculum.md",
		Desc: "Express-style routing and middleware in Go.",
	},
	{
		Slug: "vue", Name: "Vue", Logo: "vue", Dir: "vue",
		File: "vue_curriculum.md",
		Desc: "Vue 3 composition API with TypeScript.",
	},
	{
		Slug: "redis", Name: "Redis", Logo: "redis", Dir: "redis",
		File: "redis_curriculum.md",
		Desc: "Redis with Go: caching, streams, and queues.",
	},
}

// docResourceSource is a curated, verified pointer to official
// documentation or a full-length video course for a curriculum track.
type docResourceSource struct {
	Kind string // "DOCS" or "VIDEO"
	Name string
	URL  string
	Desc string
}

var curriculumResources = map[string][]docResourceSource{
	"go": {
		{Kind: "DOCS", Name: "Official Go Docs", URL: "https://go.dev/doc/", Desc: "The Go Programming Language Specification, tutorials, and release notes."},
		{Kind: "DOCS", Name: "A Tour of Go", URL: "https://go.dev/tour/", Desc: "Interactive tour covering syntax, methods, interfaces, and concurrency."},
		{Kind: "DOCS", Name: "Effective Go", URL: "https://go.dev/doc/effective_go", Desc: "Idiomatic Go: formatting, naming, error handling, and concurrency patterns."},
		{Kind: "VIDEO", Name: "Go Programming – Full Course", URL: "https://www.youtube.com/watch?v=un6ZyFkqFKo", Desc: "freeCodeCamp · Golang course with bonus projects (7h)."},
		{Kind: "VIDEO", Name: "Learn Go Programming", URL: "https://www.youtube.com/watch?v=YS4e4q9oBaU", Desc: "freeCodeCamp · Golang tutorial for beginners (7h)."},
	},
	"typescript": {
		{Kind: "DOCS", Name: "TS Handbook", URL: "https://www.typescriptlang.org/docs/handbook/intro.html", Desc: "The official TypeScript handbook: everyday types, narrowing, generics, and inference."},
		{Kind: "DOCS", Name: "TypeScript Docs", URL: "https://www.typescriptlang.org/docs/", Desc: "Official reference: compiler options, declarations, and project configuration."},
		{Kind: "VIDEO", Name: "Learn TypeScript – Full Course", URL: "https://www.youtube.com/watch?v=30LWjhZzg50", Desc: "freeCodeCamp · TypeScript tutorial for beginners (4h)."},
		{Kind: "VIDEO", Name: "TypeScript Full Course", URL: "https://www.youtube.com/watch?v=gieEQFIfgYc", Desc: "Dave Gray · TypeScript for beginners, 8h guided series."},
	},
	"postgres": {
		{Kind: "DOCS", Name: "Official PG Docs", URL: "https://www.postgresql.org/docs/", Desc: "PostgreSQL documentation: SQL reference, server admin, and internals."},
		{Kind: "DOCS", Name: "PG Tutorial", URL: "https://www.postgresql.org/docs/current/tutorial.html", Desc: "Official Part I tutorial: psql basics, SQL language, and advanced features."},
		{Kind: "VIDEO", Name: "Learn PostgreSQL – Full Course", URL: "https://www.youtube.com/watch?v=qw--VYLpxG4", Desc: "freeCodeCamp · Amigoscode, 4h course for beginners."},
		{Kind: "VIDEO", Name: "PostgreSQL Tutorial for Beginners", URL: "https://www.youtube.com/watch?v=SpfIwlAYaKk", Desc: "freeCodeCamp · comprehensive Postgres course (4h)."},
	},
	"docker": {
		{Kind: "DOCS", Name: "Official Docker Docs", URL: "https://docs.docker.com/", Desc: "Docker documentation: engine, builds, compose, and swarm."},
		{Kind: "DOCS", Name: "Docker Get Started", URL: "https://docs.docker.com/get-started/", Desc: "Official guided walkthrough: containers, images, volumes, and compose."},
		{Kind: "VIDEO", Name: "Docker Tutorial for Beginners", URL: "https://www.youtube.com/watch?v=3c-iBn73dDE", Desc: "TechWorld with Nana · full Docker course (3h)."},
		{Kind: "VIDEO", Name: "Docker Crash Course", URL: "https://www.youtube.com/watch?v=pg19Z8LL06w", Desc: "TechWorld with Nana · hands-on crash course (2h)."},
	},
	"rust": {
		{Kind: "DOCS", Name: "The Rust Book", URL: "https://doc.rust-lang.org/book/", Desc: "The official Rust Programming Language book: ownership, traits, lifetimes."},
		{Kind: "DOCS", Name: "Official Rust Docs", URL: "https://doc.rust-lang.org/", Desc: "Standard library reference, edition guides, and cargo book."},
		{Kind: "DOCS", Name: "Rustlings", URL: "https://github.com/rust-lang/rustlings", Desc: "Official small exercises to get you reading and writing Rust."},
		{Kind: "VIDEO", Name: "Rust Programming – Full Course", URL: "https://www.youtube.com/watch?v=BpPEoZW5IiY", Desc: "freeCodeCamp · complete Rust course for beginners (6h)."},
	},
	"git": {
		{Kind: "DOCS", Name: "Pro Git Book", URL: "https://git-scm.com/book/en/v2", Desc: "The complete official Git book, free online, every edition."},
		{Kind: "DOCS", Name: "Official Git Reference", URL: "https://git-scm.com/docs", Desc: "Every command, option, and plumbing detail from git-scm."},
		{Kind: "VIDEO", Name: "Git and GitHub for Beginners", URL: "https://www.youtube.com/watch?v=RGOj5yH7evk", Desc: "freeCodeCamp · crash course covering the full workflow (1h)."},
	},
	"gin": {
		{Kind: "DOCS", Name: "Gin Web Framework Docs", URL: "https://gin-gonic.com/docs/", Desc: "Official documentation: routing, middleware, and deployment."},
		{Kind: "DOCS", Name: "Gin GitHub Repository", URL: "https://github.com/gin-gonic/gin", Desc: "Source, examples, benchmarks, and the issue tracker."},
	},
	"chi": {
		{Kind: "DOCS", Name: "Chi Router Docs", URL: "https://go-chi.io/", Desc: "Official site: middleware, routing, and design philosophy."},
		{Kind: "DOCS", Name: "Chi GitHub Repository", URL: "https://github.com/go-chi/chi", Desc: "Examples for REST APIs, middlewares, and graceful shutdown."},
	},
	"hono": {
		{Kind: "DOCS", Name: "Hono Documentation", URL: "https://hono.dev/docs/", Desc: "Official docs: routers, middleware, and runtime adapters."},
		{Kind: "DOCS", Name: "Hono GitHub Repository", URL: "https://github.com/honojs/hono", Desc: "Source, examples, and middleware ecosystem."},
	},
	"echo": {
		{Kind: "DOCS", Name: "Echo Docs", URL: "https://echo.labstack.com/docs", Desc: "Official guide: routing, middleware, request and response handling."},
		{Kind: "DOCS", Name: "Echo Recipes", URL: "https://echo.labstack.com/docs/recipes", Desc: "Official recipes: CRUD APIs, websockets, file uploads, auth."},
	},
	"fiber": {
		{Kind: "DOCS", Name: "Fiber Docs", URL: "https://docs.gofiber.io/", Desc: "Official documentation: routing, middleware, and optimizations."},
		{Kind: "DOCS", Name: "Fiber GitHub Repository", URL: "https://github.com/gofiber/fiber", Desc: "Source, examples, and the middleware ecosystem."},
	},
	"vue": {
		{Kind: "DOCS", Name: "Vue 3 Guide", URL: "https://vuejs.org/guide/introduction.html", Desc: "The official guide: reactivity, components, and composition API."},
		{Kind: "DOCS", Name: "Vue TypeScript Docs", URL: "https://vuejs.org/guide/typescript/overview.html", Desc: "Official TypeScript support, typing props, and composables."},
	},
	"redis": {
		{Kind: "DOCS", Name: "Redis Docs", URL: "https://redis.io/docs/latest/", Desc: "Official documentation: data types, persistence, and clients."},
		{Kind: "DOCS", Name: "go-redis Client", URL: "https://redis.io/docs/latest/develop/clients/go/", Desc: "Official Go client guide and connection patterns."},
	},
}

func findTrack(slug string) (curriculumTrack, bool) {
	for _, t := range curriculumTracks {
		if t.Slug == slug {
			return t, true
		}
	}
	return curriculumTrack{}, false
}

func curriculumRoot() string {
	return resolveDir("CURRICULUM_DIR", "curriculum-room")
}

func buildCurriculum(slug string) (docView, error) {
	track, ok := findTrack(strings.TrimPrefix(slug, "curriculum:"))
	if !ok {
		return docView{}, fmt.Errorf("unknown curriculum %q", slug)
	}
	doc, err := readDocFile(filepath.Join(curriculumRoot(), track.Dir, track.File))
	if err != nil {
		// Archived tracks keep rendering from the docs/backup archive.
		doc, err = readDocFile(filepath.Join(curriculumRoot(), "backup", track.Dir, track.File))
		if err != nil {
			return docView{}, fmt.Errorf("read curriculum %s: %w", track.Slug, err)
		}
	}
	group := docGroupView{
		Slug:     track.Slug,
		Label:    track.Name,
		Title:    doc.Title,
		Count:    0,
		Sections: sectionsFromDoc(doc),
	}
	group.Count = len(group.Sections)
	lessons, minutes := 0, 0
	for _, sec := range group.Sections {
		minutes += sec.Minutes
		if sec.Lesson {
			lessons++
		}
	}
	resources := make([]docResource, 0, len(curriculumResources[track.Slug]))
	for _, r := range curriculumResources[track.Slug] {
		resources = append(resources, docResource{Kind: r.Kind, Name: r.Name, URL: r.URL, Desc: r.Desc})
	}
	return docView{
		Slug:      track.Slug,
		Label:     track.Name,
		Desc:      track.Desc,
		Logo:      track.Logo,
		Kind:      "curriculum",
		Count:     group.Count,
		Lessons:   lessons,
		Minutes:   minutes,
		Tabs:      []docGroupView{group},
		Resources: resources,
	}, nil
}

func handleCurriculumDoc(w http.ResponseWriter, r *http.Request) {
	track, ok := findTrack(r.PathValue("slug"))
	if !ok {
		notFound(w)
		return
	}
	view, err := docStore.load("curriculum:"+track.Slug, buildCurriculum)

	data := pageData{
		Title: track.Name + " · Curriculum",
		Page:  "curriculum",
		Nav:   nav(),
		Trail: "Curriculum",
		Doc:   &view,
	}
	if err != nil {
		data.Doc.Error = err.Error()
	}
	Render(w, "doc_page", data)
}

var snippetLanguages = []struct {
	Key  string
	Name string
	Desc string
	Logo string
}{
	{Key: "golang", Name: "Golang", Desc: "Handlers, workers, and small utilities.", Logo: "golang"},
	{Key: "typescript", Name: "TypeScript", Desc: "Types, hooks, and fetch wrappers.", Logo: "ts"},
	{Key: "bash", Name: "Bash", Desc: "Ops one-liners and deploy glue.", Logo: "bash"},
	{Key: "docker", Name: "Docker", Desc: "Dockerfiles and compose fragments.", Logo: "docker"},
	{Key: "postgres", Name: "Postgres", Desc: "Queries, indexes, and migrations.", Logo: "postgres"},
	{Key: "rust", Name: "Rust", Desc: "Crate scaffolds and unsafe-free helpers.", Logo: "rust"},
}

// snippetKey buckets a source file under its language tab.
func snippetKey(filename string) string {
	base := strings.ToLower(docSlug(filename))
	switch {
	case strings.Contains(base, "postgres"), strings.Contains(base, "pg-"):
		return "postgres"
	case strings.HasPrefix(base, "golang"), strings.HasPrefix(base, "go-"), strings.HasPrefix(base, "go_"), base == "go":
		return "golang"
	case strings.HasPrefix(base, "typescript"), strings.HasPrefix(base, "ts-"):
		return "typescript"
	case strings.HasPrefix(base, "bash"), strings.HasPrefix(base, "shell"):
		return "bash"
	case strings.HasPrefix(base, "docker"), strings.HasPrefix(base, "compose"):
		return "docker"
	case strings.HasPrefix(base, "rust"):
		return "rust"
	}
	return ""
}

func snippetsRoot() string {
	return resolveDir("SNIPPETS_DIR", "snippets")
}

func buildSnippets(lang string) (docView, error) {
	lang = strings.TrimPrefix(lang, "snippets:")
	var meta struct {
		Key, Name, Desc, Logo string
	}
	found := false
	for _, l := range snippetLanguages {
		if l.Key == lang {
			meta = struct {
				Key, Name, Desc, Logo string
			}{l.Key, l.Name, l.Desc, l.Logo}
			found = true
			break
		}
	}
	if !found {
		return docView{}, fmt.Errorf("unknown snippet language %q", lang)
	}

	entries, err := os.ReadDir(snippetsRoot())
	if err != nil {
		return docView{}, fmt.Errorf("read snippets dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if snippetKey(e.Name()) != lang {
			continue
		}
		files = append(files, e.Name())
	}
	if len(files) == 0 {
		return docView{}, errNoSnippet
	}

	tabs := make([]docGroupView, 0, len(files))
	sections, minutes := 0, 0
	for _, name := range sortDocFiles(files) {
		doc, err := readDocFile(filepath.Join(snippetsRoot(), name))
		if err != nil {
			continue
		}
		group := groupView(docSlug(name), docLabelFor(docSlug(name)), doc)
		tab := group
		tab.Count = len(group.Sections)
		sections += len(group.Sections)
		for _, sec := range group.Sections {
			minutes += sec.Minutes
		}
		tabs = append(tabs, tab)
	}
	if len(tabs) == 0 {
		return docView{}, errNoSnippet
	}

	return docView{
		Slug:    meta.Key,
		Label:   meta.Name,
		Desc:    meta.Desc,
		Logo:    meta.Logo,
		Kind:    "snippets",
		Count:   sections,
		Minutes: minutes,
		Tabs:    tabs,
	}, nil
}

type errNoSnippets struct{ kind string }

func (e errNoSnippets) Error() string { return "no snippet docs for " + e.kind }

var errNoSnippet = errNoSnippets{}

// renderSoon draws the "module pending" page used while a track has no docs.
func renderSoon(w http.ResponseWriter, g group, item link, trail string) {
	siblings := make([]link, 0, len(g.Items))
	for _, l := range g.Items {
		if l.URL != item.URL && l.URL != "/"+g.Key {
			siblings = append(siblings, l)
		}
	}
	Render(w, "soon_page", pageData{
		Title:    item.Name + " · " + g.Label,
		Page:     g.Key,
		Nav:      nav(),
		Group:    g,
		Item:     item,
		Siblings: siblings,
		Trail:    trail,
	})
}
