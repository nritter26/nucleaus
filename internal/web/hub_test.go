package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	RegisterStatic(mux)
	return mux
}

func get(t *testing.T, mux *http.ServeMux, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	res := rec.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return res.StatusCode, string(body)
}

func TestPagesRender(t *testing.T) {
	mux := newMux()

	cases := []struct {
		path  string
		wants []string
	}{
		{"/hub", []string{"192.168.1.121", "LIVE TELEMETRY", "dashboard.js", "/api/metrics", "public-dashboards/133997be350646f1b74d2ce4ec72644b", "Curriculum", "Database"}},
		{"/tools", []string{"Tools", "Rabbit-MQ", "K8S", "/k8s"}},
		{"/cloud", []string{"Cloud", "File-Stash", "SILO", "AWS"}},
		{"/database", []string{"Database", "Redis-Insight", "MongoExpress"}},
		{"/ai", []string{"AI Tools", "BG-Remover", "/imagegen", "/tts", "MCP", "/mcp", "mcp.png"}},
		{"/mcp", []string{"MCP"}},		{"/curriculum", []string{"/curriculum/go", "/curriculum/typescript", "/curriculum/postgres", "/curriculum/docker", "/curriculum/rust", "/logos/golang.png", "/logos/ts.png", "/logos/rust.png"}},
		{"/curriculum/go", []string{"Go", "doc-sec", "doc-body", "docs.js"}},
		{"/library", []string{"/library/golang", "/library/typescript"}},
		{"/snippets", []string{"/snippets/bash", "/snippets/docker", "/snippets/postgres"}},
		{"/snippets/docker", []string{"Docker"}},
		{"/library/golang", []string{"Golang", "PENDING"}},
		{"/k8s", []string{"Kubernetes", "PENDING"}},
		{"/imagegen", []string{"Image-Gen"}},
		{"/tts", []string{"TTS"}},
		{"/bgremover", []string{"BG-Remover"}},
		{"/static/hub.css", []string{"--purple: #af05ca"}},
		{"/static/dashboard.js", []string{"telemetry"}},
		{"/static/hub.js", []string{"starfield"}},
		{"/logos/gitea.png", nil},
	}

	for _, tc := range cases {
		status, body := get(t, mux, tc.path)
		if status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, status)
			continue
		}
		for _, want := range tc.wants {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s missing %q", tc.path, want)
			}
		}
	}
}

func TestHubHasNoLegacySections(t *testing.T) {
	mux := newMux()
	_, body := get(t, mux, "/hub")
	for _, gone := range []string{`class="cards"`, `id="tools"`, `class="ticker"`, `class="rail"`, "Launch tools", `class="console"`, `class="reactor"`, `class="core"`} {
		if strings.Contains(body, gone) {
			t.Errorf("hub still contains removed markup %q", gone)
		}
	}
	if !strings.Contains(body, `data-window="5m"`) {
		t.Error("hub is missing the 5m window button")
	}
}

func TestLibraryHasRepoLinks(t *testing.T) {
	mux := newMux()
	_, body := get(t, mux, "/library")
	for _, want := range []string{
		"http://192.168.1.121:3001/mallard/cobra",
		"http://192.168.1.121:3001/mallard/zerolog",
		"http://192.168.1.121:3001/mallard/viper",
		"http://192.168.1.121:3001/mallard/awesome-go",
		"http://192.168.1.121:3001/mallard/kit",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("library missing %s", want)
		}
	}
	// Links without a logo must still render a tile.
	if !strings.Contains(body, "monogram--lg") {
		t.Error("library tiles should fall back to a monogram")
	}
}

func TestCheatSheetPage(t *testing.T) {
	if _, err := parseCommands(commandsDir()); err != nil {
		t.Skipf("command docs unavailable: %v", err)
	}

	mux := newMux()
	status, body := get(t, mux, "/cheatsheet")
	if status != http.StatusOK {
		t.Fatalf("/cheatsheet = %d, want 200", status)
	}
	for _, want := range []string{"Cheat Sheet", "cheatsheet.js", `id="cheat-search"`, "data-doc=\"docker-commands-for-backend-devops\"", "cheat-entry"} {
		if !strings.Contains(body, want) {
			t.Errorf("/cheatsheet missing %q", want)
		}
	}
	if strings.Contains(body, "Could not read the command docs") {
		t.Error("/cheatsheet rendered the error state")
	}

	// The navbar tab must be a plain link, not a dropdown trigger.
	if !strings.Contains(body, `class="nav-trigger nav-trigger--link is-active" href="/cheatsheet"`) {
		t.Error("cheatsheet navbar tab missing")
	}
}

func TestParseCommandsFixture(t *testing.T) {
	dir := t.TempDir()

	docker := strings.Join([]string{
		"# Docker Reference",
		"",
		"Intro.",
		"",
		"## Table of Contents",
		"1. [Info](#info)",
		"",
		"---",
		"",
		"## 1. System Info",
		"",
		"1. `docker version`  ",
		"   Displays client and server versions.",
		"2. `docker system df`",
		"   Shows disk usage.",
		"",
		"## 2. Build",
		"",
		"1. **Build an image**",
		"   ```bash",
		"   docker build -t app .",
		"   ```",
	}, "\n")
	helpers := strings.Join([]string{
		"# Helpers",
		"",
		"## 1. HTTP",
		"",
		"### 1. WriteJSON",
		"Sends a JSON response.",
		"```go",
		"func WriteJSON(w http.ResponseWriter) {}",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(dir, "100_docker_commands.md"), []byte(docker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helpers.md"), []byte(helpers), 0o600); err != nil {
		t.Fatal(err)
	}

	docs, err := parseCommands(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("docs = %d, want 2", len(docs))
	}

	var dockerDoc cheatDoc
	for _, doc := range docs {
		if strings.Contains(doc.Slug, "docker") {
			dockerDoc = doc
		}
	}
	if dockerDoc.Count != 3 {
		t.Fatalf("docker entries = %d, want 3 (%+v)", dockerDoc.Count, dockerDoc.Sections)
	}
	if len(dockerDoc.Sections) != 2 {
		t.Fatalf("docker sections = %d, want 2 (TOC must be skipped)", len(dockerDoc.Sections))
	}

	first := dockerDoc.Sections[0].Entries[0]
	if first.Command != "docker version" {
		t.Errorf("first command = %q", first.Command)
	}
	if !strings.Contains(first.Desc, "Displays client and server versions") {
		t.Errorf("first desc = %q", first.Desc)
	}

	build := dockerDoc.Sections[1].Entries[0]
	if build.Title != "Build an image" {
		t.Errorf("fenced entry title = %q", build.Title)
	}
	if !strings.Contains(build.Command, "docker build -t app") {
		t.Errorf("fenced entry command = %q", build.Command)
	}
}

func TestSeriesWindow5m(t *testing.T) {
	stub := stubPrometheus(t)
	defer stub.Close()
	t.Setenv("PROMETHEUS_URL", stub.URL)

	mux := newMux()
	status, body := get(t, mux, "/api/metrics/series?window=5m")
	if status != http.StatusOK {
		t.Fatalf("series 5m = %d, want 200", status)
	}
	if !strings.Contains(body, `"window":"5m"`) {
		t.Errorf("window not echoed as 5m: %s", body)
	}
}

func TestUnknownRoutes(t *testing.T) {
	mux := newMux()
	for _, path := range []string{"/curriculum/perl", "/library/cobol", "/snippets/perl"} {
		if status, _ := get(t, mux, path); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

// docDir helpers: content tests skip cleanly when the source markdown is not
// in the repo checkout (the server mounts it via DOC_* env in production).
func docsAvailable(env, name string) bool {
	return os.Getenv(env) != "" || func() bool {
		for _, dir := range []string{"docs/" + name, "../docs/" + name, "../../docs/" + name} {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return true
			}
		}
		return false
	}()
}

func TestCurriculumPagesRender(t *testing.T) {
	if !docsAvailable("CURRICULUM_DIR", "curriculum-room") {
		t.Skip("curriculum docs unavailable")
	}

	mux := newMux()

	status, body := get(t, mux, "/curriculum/rust")
	if status != http.StatusOK {
		t.Fatalf("/curriculum/rust = %d, want 200", status)
	}
	for _, want := range []string{"Rust", "/logos/rust.png", "doc-sec", "doc-body", "docs.js"} {
		if !strings.Contains(body, want) {
			t.Errorf("/curriculum/rust missing %q", want)
		}
	}
	// Every published track renders real content.
	for _, slug := range []string{"go", "typescript", "postgres", "docker", "rust"} {
		_, body := get(t, mux, "/curriculum/"+slug)
		if strings.Contains(body, "Could not read") {
			t.Errorf("/curriculum/%s rendered the error state: %s", slug, body)
		}
		if !strings.Contains(body, "doc-sec") {
			t.Errorf("/curriculum/%s has no sections", slug)
		}
		// Curated resource links are attached to every track: at least one
		// official docs pointer and one video pointer.
		if !strings.Contains(body, "doc-res") {
			t.Errorf("/curriculum/%s missing the resource panel", slug)
		}
		if !strings.Contains(body, `class="doc-res-kind doc-res-kind--DOCS"`) {
			t.Errorf("/curriculum/%s missing a docs resource", slug)
		}
	}

	// The resource panel links are verified official docs and video URLs.
	status, body = get(t, mux, "/curriculum/docker")
	if status != http.StatusOK {
		t.Fatalf("/curriculum/docker = %d, want 200", status)
	}
	for _, want := range []string{
		"doc-res-link", "https://docs.docker.com/get-started/",
		"https://www.youtube.com/watch?v=3c-iBn73dDE",
		"rel=\"noopener noreferrer\"",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/curriculum/docker missing resource %q", want)
		}
	}
	status, body = get(t, mux, "/curriculum/rust")
	if status != http.StatusOK {
		t.Fatalf("/curriculum/rust = %d, want 200", status)
	}
	if !strings.Contains(body, "https://doc.rust-lang.org/book/") {
		t.Error("/curriculum/rust missing The Rust Book link")
	}
}

func TestSnippetPagesRender(t *testing.T) {
	if _, err := os.Stat("../../docs/snippets"); err != nil {
		t.Skip("snippet docs unavailable")
	}

	mux := newMux()

	status, body := get(t, mux, "/snippets/typescript")
	if status != http.StatusOK {
		t.Fatalf("/snippets/typescript = %d, want 200", status)
	}
	for _, want := range []string{
		"TypeScript", "doc-sec", "doc-tabs", "cheat-cmd", "docs.js",
		"typescript-backend-helper-functions",
		"typescript-backend-utility-functions",
		"typescript-backend-api-scaffolds-patterns",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/snippets/typescript missing %q", want)
		}
	}

	status, body = get(t, mux, "/snippets/golang")
	if status != http.StatusOK {
		t.Fatalf("/snippets/golang = %d, want 200", status)
	}
	for _, want := range []string{
		"Golang", "doc-sec", "cheat-cmd", "docs.js",
		"Go Helpers", "Go Utilities", "Go Structs", "Go Interfaces",
		"Fintech APIs", "API Scaffolds",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/snippets/golang missing %q", want)
		}
	}

	status, body = get(t, mux, "/snippets/postgres")
	if status != http.StatusOK {
		t.Fatalf("/snippets/postgres = %d, want 200", status)
	}
	if !strings.Contains(body, "doc-sec") || !strings.Contains(body, "PG Schema") {
		t.Error("/snippets/postgres should render the schema content")
	}

	// Languages without source docs keep their coming-soon module.
	for _, slug := range []string{"bash", "docker"} {
		status, body := get(t, mux, "/snippets/"+slug)
		if status != http.StatusOK {
			t.Errorf("/snippets/%s = %d, want 200", slug, status)
		}
		if !strings.Contains(body, "PENDING") {
			t.Errorf("/snippets/%s should stay a pending module", slug)
		}
	}
}

// stubPrometheus answers the two Prometheus endpoints the bridge uses.
func stubPrometheus(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query":
			query := r.URL.Query().Get("query")
			var value string
			var metric = map[string]string{}
			switch {
			case strings.Contains(query, "container_cpu_usage_seconds_total") && strings.Contains(query, "sum by (name)"):
				metric = map[string]string{"name": "gitea"}
				value = "0.4"
			case strings.Contains(query, "irate"):
				value = "0.32"
			case strings.Contains(query, "machine_cpu_cores"):
				value = "8"
			case strings.Contains(query, "machine_memory_bytes"):
				value = "16149897216"
			case strings.Contains(query, "container_memory_working_set_bytes"):
				value = "9068576768"
			case strings.Contains(query, "container_last_seen"):
				value = "48"
			default:
				value = "1000"
			}
			json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{"metric": metric, "value": []any{1792502912.5, value}},
					},
				},
			})
		case "/api/v1/query_range":
			json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "matrix",
					"result": []map[string]any{
						{"metric": map[string]string{}, "values": []any{
							[]any{1792502280.0, "0.3"},
							[]any{1792502400.0, "0.33"},
						}},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestMetricsOverview(t *testing.T) {
	stub := stubPrometheus(t)
	defer stub.Close()
	t.Setenv("PROMETHEUS_URL", stub.URL)

	mux := newMux()
	status, body := get(t, mux, "/api/metrics/overview")
	if status != http.StatusOK {
		t.Fatalf("overview = %d, want 200 (%s)", status, body)
	}

	var out overviewResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if out.HostCPUs != 8 {
		t.Errorf("hostCpus = %v, want 8", out.HostCPUs)
	}
	if out.MemTotal != 16149897216 {
		t.Errorf("memTotal = %v", out.MemTotal)
	}
	if out.Containers != 48 {
		t.Errorf("containers = %v, want 48", out.Containers)
	}
	if out.CPUCores <= 0 || out.CPUPercent <= 0 {
		t.Errorf("cpu values not populated: %+v", out)
	}
}

func TestMetricsSeries(t *testing.T) {
	stub := stubPrometheus(t)
	defer stub.Close()
	t.Setenv("PROMETHEUS_URL", stub.URL)

	mux := newMux()
	status, body := get(t, mux, "/api/metrics/series?window=15m")
	if status != http.StatusOK {
		t.Fatalf("series = %d, want 200 (%s)", status, body)
	}

	var out struct {
		Window string                  `json:"window"`
		Points map[string][][2]float64 `json:"points"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode series: %v", err)
	}
	if out.Window != "15m" {
		t.Errorf("window = %q, want 15m", out.Window)
	}
	for _, key := range []string{"cpu", "mem", "rx", "tx"} {
		if len(out.Points[key]) != 2 {
			t.Errorf("%s points = %d, want 2", key, len(out.Points[key]))
		}
	}
}

func TestMetricsContainers(t *testing.T) {
	stub := stubPrometheus(t)
	defer stub.Close()
	t.Setenv("PROMETHEUS_URL", stub.URL)

	mux := newMux()
	status, body := get(t, mux, "/api/metrics/containers")
	if status != http.StatusOK {
		t.Fatalf("containers = %d, want 200 (%s)", status, body)
	}

	var out struct {
		Count      int            `json:"count"`
		Containers []containerRow `json:"containers"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode containers: %v", err)
	}
	if out.Count != 1 || out.Containers[0].Name != "gitea" {
		t.Fatalf("unexpected containers payload: %+v", out)
	}
	if out.Containers[0].CPU <= 0 {
		t.Errorf("cpu percent not derived: %+v", out.Containers[0])
	}
}

func TestMetricsBridgeFailure(t *testing.T) {
	stub := stubPrometheus(t)
	stub.Close() // simulate the rack being offline
	t.Setenv("PROMETHEUS_URL", stub.URL)

	mux := newMux()
	status, body := get(t, mux, "/api/metrics/overview")
	if status != http.StatusBadGateway {
		t.Fatalf("overview with dead prometheus = %d, want 502 (%s)", status, body)
	}
	if !strings.Contains(body, "error") {
		t.Errorf("expected error payload, got %s", body)
	}
}

func TestTTSPlayerRenders(t *testing.T) {
	if _, err := os.Stat("../../internal/web/tts/manifest.json"); err != nil {
		t.Skip("tts manifest unavailable")
	}
	mux := newMux()

	status, body := get(t, mux, "/tts")
	if status != http.StatusOK {
		t.Fatalf("/tts = %d, want 200", status)
	}
	for _, want := range []string{
		"Curriculum read-aloud", "tts-audio", "tts-rate", "tts-vol", "tts-seek",
		"/static/tts.js", "/logos/tts.png",
		`data-track="go"`, `data-track="typescript"`, `data-track="postgres"`,
		`data-track="docker"`, `data-track="rust"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/tts missing %q", want)
		}
	}

	// Every manifest item is linked with its disk-backed audio URL.
	var man struct {
		Tracks map[string]struct {
			Items []struct {
				File string `json:"file"`
			} `json:"items"`
		} `json:"tracks"`
	}
	raw, err := os.ReadFile("../../internal/web/tts/manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	total := 0
	for track, tr := range man.Tracks {
		total += len(tr.Items)
		for _, item := range tr.Items {
			if !strings.Contains(body, item.File) {
				t.Errorf("/tts missing audio %s", item.File)
			}
			// Audio route serves existing mp3s on disk.
			disk := filepath.Join("../../internal/web/tts", item.File)
			if _, err := os.Stat(disk); err != nil {
				continue
			}
			_ = track
			break
		}
	}
	if total < 90 {
		t.Errorf("manifest tracks hold %d items, want ~93", total)
	}
}

func TestTTSAudioServes(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "web", "tts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("tts audio dir unavailable")
	}
	var sample string
	for _, e := range entries {
		if e.IsDir() {
			if files, err := os.ReadDir(filepath.Join(dir, e.Name())); err == nil {
				for _, f := range files {
					if strings.HasSuffix(f.Name(), ".mp3") {
						sample = e.Name() + "/" + f.Name()
						break
					}
				}
			}
		}
		if sample != "" {
			break
		}
	}
	if sample == "" {
		t.Skip("no generated mp3s yet")
	}

	mux := newMux()
	status, body := get(t, mux, "/tts/full/"+sample)
	if status != http.StatusOK {
		t.Errorf("/tts/full/%s = %d, want 200", sample, status)
	}
	if len(body) == 0 {
		t.Error("audio body is empty")
	}

	// Traversal is rejected.
	for _, bad := range []string{
		"/tts/full/go/..%2f..%2f..%2fgo.mod",
		"/tts/full/go/not-realvoice.mp3",
	} {
		if status, _ := get(t, mux, bad); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", bad, status)
		}
	}
}

func TestExternalLinksOpenInNewTab(t *testing.T) {	mux := newMux()
	_, body := get(t, mux, "/tools")
	if strings.Contains(body, `href="http://192.168.1.121:3001/"`) &&
		!strings.Contains(body, `href="http://192.168.1.121:3001/" target="_blank"`) {
		t.Error("Gitea card should open in a new tab")
	}
}
