package web

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"nucleaus/internal/snippets"
)

// link is one clickable destination rendered as a card or a dropdown entry.
type link struct {
	Name     string
	Desc     string
	URL      string
	Logo     string
	Meta     string
	NewTab   bool
	Initials string
}

// group is a themed collection of links: a navbar dropdown plus a card section.
type group struct {
	Key    string
	Label  string
	Tag    string
	Blurb  string
	Items  []link
	Direct bool // render as a plain navbar link instead of a dropdown
}

// pageData is the view model every hub template receives.
type pageData struct {
	Title    string
	Page     string
	Nav      []group
	Groups   []group
	Group    group
	Item     link
	Siblings []link
	Trail    string

	// Linked from the telemetry dashboard.
	GrafanaURL    string
	PrometheusURL string

	// Markdown doc pages (curriculum, snippets).
	Doc *docView

	// Snippet database views: labels, snippet CRUD, manage list.
	HasDB     bool
	Labels    []snippets.Label
	Snip      *snippets.Snippet
	SnipRows  []snipRow
	SnipError string

	// Cheat sheet page.
	CheatDocs  []cheatDoc
	CheatTotal int
	CheatError string

	// TTS read-aloud player.
	Tts *ttsView
}

const grafanaDashboardURL = "http://192.168.1.121:3000/public-dashboards/133997be350646f1b74d2ce4ec72644b"

// app builds an external application card that opens in a new tab.
func app(name, desc, url, logo, meta string) link {
	l := link{Name: name, Desc: desc, URL: url, Logo: logo, Meta: meta, NewTab: true}
	l.Initials = initials(name)
	return l
}

// view builds an internal catalog card that navigates in the same tab.
func view(name, desc, url, logo string) link {
	l := link{Name: name, Desc: desc, URL: url, Logo: logo}
	l.Initials = initials(name)
	return l
}

func initials(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(strings.ToUpper(string([]rune(p)[0])))
		if b.Len() >= 2 {
			break
		}
	}
	return b.String()
}

func groups() []group {
	return []group{
		{
			Key:   "curriculum",
			Label: "Curriculum",
			Tag:   "01 / LEARN",
			Blurb: "Guided tracks that take you from syntax to shipped service.",
			Items: []link{
				view("Go", "Concurrency, services, and idiomatic stdlib.", "/curriculum/go", "golang"),
				view("TypeScript", "Types, tooling, and front-end architecture.", "/curriculum/typescript", "ts"),
				view("Postgres", "Queries, schema design, and production reliability.", "/curriculum/postgres", "postgres"),
				view("Docker", "Images, networks, and compose in production.", "/curriculum/docker", "docker"),
				view("Rust", "Ownership, traits, and fearless systems code.", "/curriculum/rust", "rust"),
				view("Git", "Version control mastery from basics to team flow.", "/curriculum/git", "git"),
				view("Git Lab", "Repair broken repos in an animated commit simulator.", "/git-lab", "git"),
				view("Gin", "The Go web framework in production.", "/curriculum/gin", "gin"),
				view("Chi", "Lightweight, composable HTTP routers.", "/curriculum/chi", "chi"),
				view("Hono", "The ultrafast JavaScript web framework.", "/curriculum/hono", "hono"),
				view("Echo", "High performance, extensible Go web framework.", "/curriculum/echo", "echo"),
				view("Fiber", "Express-inspired Go web framework.", "/curriculum/fiber", "fiber"),
				view("Vue", "Vue 3 with TypeScript front-end architecture.", "/curriculum/vue", "vue"),
				view("Redis", "Redis with Go: caching, streaming, and queues.", "/curriculum/redis", "redis"),
				view("Browse curriculum", "All available learning tracks.", "/curriculum", ""),
			},
		},
		{
			Key:   "library",
			Label: "Library",
			Tag:   "02 / READ",
			Blurb: "Reference shelves curated per language.",
			Items: []link{
				view("Golang", "Engineering starters, mirrors, and fintech repos.", "/library/golang", "golang"),
				view("TypeScript", "Typed API layers and edge-first routing.", "/library/typescript", "ts"),
				view("Bash", "Script tooling, CLIs, and test harnesses.", "/library/bash", "bash"),
				view("Rust", "Bindings, blockchain stacks, and wallet crypto.", "/library/rust", "rust"),
				view("Browse library", "Every shelf in the archive.", "/library", ""),
			},
		},
		{
			Key:   "snippets",
			Label: "Snippet",
			Tag:   "03 / REUSE",
			Blurb: "Copy-ready fragments for the jobs that repeat.",
			Items: []link{
				view("Golang", "Handlers, workers, and small utilities.", "/snippets/golang", "golang"),
				view("TypeScript", "Types, hooks, and fetch wrappers.", "/snippets/typescript", "ts"),
				view("Bash", "Ops one-liners and deploy glue.", "/snippets/bash", "bash"),
				view("Docker", "Dockerfiles and compose fragments.", "/snippets/docker", "docker"),
				view("Postgres", "Queries, indexes, and migrations.", "/snippets/postgres", "postgres"),
				view("Browse snippets", "Search the whole fragment vault.", "/snippets", ""),
			},
		},
		{
			Key:   "tools",
			Label: "Tools",
			Tag:   "04 / OPERATE",
			Blurb: "Everything running on the rack, one click away.",
			Items: []link{
				app("Gitea", "Self-hosted git and package registry.", "http://192.168.1.121:3001/", "gitea", "LAN :3001"),
				app("Github", "Repositories, issues, and actions.", "https://github.com/nritter26?tab=repositories", "github-dark", "PUBLIC"),
				app("Portainer", "Docker stacks and container control.", "https://192.168.1.121:9443/#!/3/docker/stacks/newstack", "portainer", "LAN :9443"),
				app("Hoppscotch", "API request workbench.", "http://192.168.1.121:3002/", "hoppscotch", "LAN :3002"),
				app("Mailpit", "Catch and inspect outbound mail.", "http://192.168.1.121:8025/", "mailpit", "LAN :8025"),
				app("Plane", "Issues, cycles, and project boards.", "http://192.168.1.121:8092/ignis/", "plane", "LAN :8092"),
				app("n8n", "Workflow automation canvas.", "http://192.168.1.121:5678/assistant", "n8n", "LAN :5678"),
				app("Supabase", "Postgres, auth, and storage console.", "http://mallard:110200@192.168.1.121:8181/project/default", "supabase", "LAN :8181"),
				app("Pi-Hole", "DNS sinkhole and query log.", "http://192.168.1.121:8088/admin/login", "pi-hole", "LAN :8088"),
				app("Traefik", "Edge router and service discovery.", "http://192.168.1.121:8089/dashboard/", "traefik", "LAN :8089"),
				app("Rabbit-MQ", "Broker queues and exchanges.", "http://192.168.1.121:15672/#/", "rabbitmq", "LAN :15672"),
				app("Keycloak", "Identity provider and realms.", "http://192.168.1.121:8080/admin/master/console/", "keycloak", "LAN :8080"),
				app("Prometheus", "Metrics targets and queries.", "http://192.168.1.121:9090/targets", "prometheus", "LAN :9090"),
				app("Grafana", "Dashboards over every metric.", "http://192.168.1.121:3000/", "grafana", "LAN :3000"),
				app("Libre-Chat", "Team chat and local models.", "http://192.168.1.121:3080/", "librechat", "LAN :3080"),
				app("Jaeger", "Distributed trace explorer.", "http://192.168.1.121:16686/search", "jaeger", "LAN :16686"),
				app("TailScale", "Mesh VPN machine roster.", "https://console.tailscale.com/admin/machines", "tailscale-light", "PUBLIC"),
				app("Swagger", "OpenAPI docs for the services.", "http://192.168.1.121:8082/", "swagger", "LAN :8082"),
				app("K8S", "Cluster overview dashboard.", k8sURL, "k8s", "LAN :3999"),
				app("DeepSeek", "Reasoning chat and code assistant.", "https://chat.deepseek.com/", "deepseek", "PUBLIC"),
				app("Grok", "xAI chat assistant.", "https://grok.com/", "grok", "PUBLIC"),
				app("Gemini", "Google AI assistant.", "https://gemini.google.com/app", "google-gemini", "PUBLIC"),
				app("Claude", "Anthropic AI assistant.", "https://claude.ai/new", "claude", "PUBLIC"),
				app("GLM", "Z.ai GLM chat assistant.", "https://chat.z.ai/", "glm", "PUBLIC"),
				app("Gmail", "Mallard inbox.", "https://mail.google.com/mail/u/1/#inbox", "gmail", "PUBLIC"),
				app("DrawDB", "Database diagram editor.", "https://www.drawdb.app/editor", "drawdb", "PUBLIC"),
			},
		},
		{
			Key:   "cloud",
			Label: "Cloud",
			Tag:   "05 / STORE",
			Blurb: "Object storage and the providers behind it.",
			Items: []link{
				app("File-Stash", "File manager over the storage bucket.", "http://192.168.1.121:8334/admin/storage", "filestash", "LAN :8334"),
				app("SILO", "S3-compatible object browser.", "http://192.168.1.121:9001/browser", "pgsty-silo", "LAN :9001"),
				app("GCP", "Google Cloud console.", "https://console.cloud.google.com/", "gcp", "PUBLIC"),
				app("AWS", "Amazon Web Services console.", "https://ap-southeast-2.console.aws.amazon.com/console/home?nc2=h_si&region=ap-southeast-2&src=header-signin#", "aws", "PUBLIC"),
			},
		},
		{
			Key:   "database",
			Label: "Database",
			Tag:   "06 / QUERY",
			Blurb: "Direct access to the engines holding the data.",
			Items: []link{
				app("Pgadmin", "Postgres browser and query tool.", "http://192.168.1.121:5050/browser/", "pgadmin", "LAN :5050"),
				app("Redis-Insight", "Key browser and memory stats.", "http://192.168.1.121:5540/", "redis", "LAN :5540"),
				app("MongoExpress", "MongoDB collections and documents.", "http://mallard:mallard@192.168.1.121:8081/", "mongo-express", "LAN :8081"),
			},
		},
		{
			Key:   "ai",
			Label: "AI Tools",
			Tag:   "07 / GENERATE",
			Blurb: "Local models wired into the same loop.",
			Items: []link{
				app("Image-Gen", "Text-to-image studio, coming online.", "/imagegen", "image-gen", "INTERNAL"),
				app("TTS", "Listen to the tracks read aloud.", "/tts", "tts", "INTERNAL"),
				app("BG-Remover", "Cut backgrounds out of any shot.", "/bgremover", "bg-remover", "INTERNAL"),
				app("MCP", "Model Context Protocol gateway, coming online.", "/mcp", "mcp", "INTERNAL"),
			},
		},
	}
}

// nav returns the navbar entries sorted alphabetically, tags renumbered in
// the new order. Social has no destinations yet and is hidden.
func nav() []group {
	all := groups()
	out := make([]group, 0, len(all)+1)
	for _, g := range all {
		if g.Key == "social" || (len(g.Items) == 0 && !g.Direct) {
			continue
		}
		out = append(out, g)
	}
	out = append(out, group{
		Key:    "cheatsheet",
		Label:  "Cheat Sheet",
		Tag:    "99 / REFERENCE",
		Blurb:  "Every command from the reference library, one search away.",
		Direct: true,
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	for i := range out {
		parts := strings.SplitN(out[i].Tag, " / ", 2)
		suffix := out[i].Tag
		if len(parts) == 2 {
			suffix = parts[1]
		}
		out[i].Tag = fmt.Sprintf("%02d / %s", i+1, suffix)
	}
	return out
}

func findGroup(key string) (group, bool) {
	for _, g := range groups() {
		if g.Key == key {
			return g, true
		}
	}
	return group{}, false
}

func findLink(g group, url string) (link, bool) {
	for _, l := range g.Items {
		if l.URL == url {
			return l, true
		}
	}
	return link{}, false
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	Render(w, "soon_page", pageData{
		Title: "Lost",
		Page:  "soon",
		Nav:   nav(),
		Item:  link{Name: "404", Desc: "This corridor does not exist.", Meta: "VOID", Initials: "404"},
	})
}

func registerPages(mux *http.ServeMux) {
	mux.HandleFunc("GET /cheatsheet", handleCheatSheet)
	mux.HandleFunc("GET /git-lab", handleGitLab)

	mux.HandleFunc("GET /hub", func(w http.ResponseWriter, r *http.Request) {
		Render(w, "hub_page", pageData{
			Title:         "Mallard Server Hub",
			Page:          "hub",
			Nav:           nav(),
			GrafanaURL:    grafanaDashboardURL,
			PrometheusURL: defaultPrometheusURL,
		})
	})

	for _, key := range []string{"tools", "cloud", "database", "ai"} {
		mux.HandleFunc("GET /"+key, func(w http.ResponseWriter, r *http.Request) {
			g, ok := findGroup(key)
			if !ok {
				notFound(w)
				return
			}
			Render(w, "section_page", pageData{
				Title: g.Label,
				Page:  key,
				Nav:   nav(),
				Group: g,
			})
		})
	}

	// Library: language picker first, then per-language shelves.
	mux.HandleFunc("GET /library", handleLibrary)
	mux.HandleFunc("GET /library/{slug}", handleLibraryShelf)

	// Curriculum renders markdown docs when they exist.
	registerDocPages(mux, "curriculum", handleCurriculumDoc)

	// Snippets: SQLite-backed vault with add/edit/delete plus labels.
	registerSnippets(mux)

	// TTS read-aloud player plus disk-backed generated audio.
	mux.HandleFunc("GET /tts", handleTTS)
	mux.HandleFunc("GET /tts/full/{slug}/{file}", handleTTSAudio)

	for path, title := range map[string]string{
		"/imagegen":  "Image-Gen",
		"/bgremover": "BG-Remover",
		"/mcp":       "MCP",
	} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			Render(w, "soon_page", pageData{
				Title: title,
				Page:  "soon",
				Nav:   nav(),
				Item: link{
					Name:     title,
					Desc:     "This module is provisioned but not wired up yet.",
					Meta:     "PENDING",
					URL:      path,
					Initials: initials(title),
				},
			})
		})
	}

	// The Kubernetes surface lives outside the hub; forward visitors to it.
	mux.HandleFunc("GET /k8s", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, k8sURL, http.StatusTemporaryRedirect)
	})
}

// registerDocTitles mounts the tile catalog at /<key> and hands
// /<key>/{slug} pages to the given doc handler.
func registerDocPages(mux *http.ServeMux, key string, doc http.HandlerFunc) {
	mux.HandleFunc("GET /"+key, func(w http.ResponseWriter, r *http.Request) {
		g, ok := findGroup(key)
		if !ok {
			notFound(w)
			return
		}
		items := make([]link, 0, len(g.Items))
		for _, l := range g.Items {
			if l.URL == "/"+g.Key {
				continue
			}
			items = append(items, l)
		}
		Render(w, "catalog_page", pageData{
			Title:  g.Label,
			Page:   key,
			Nav:    nav(),
			Group:  g,
			Groups: []group{{Key: g.Key, Label: g.Label, Tag: g.Tag, Blurb: g.Blurb, Items: items}},
		})
	})

	mux.HandleFunc("GET /"+key+"/{slug}", doc)
}

// k8sURL is where the Kubernetes surface lives on the rack.
const k8sURL = "http://192.168.1.121:3999/"
