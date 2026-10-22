# Nucleaus — Mallard Server Hub

Nucleaus Nu-clea-us is a self-hosted homelab control center written in Go. It renders "Mallard
Server Hub": a single, dark-themed web dashboard that serves as the front door to a
home rack — nav hub, learning content, telemetry, and small utilities, all behind
one binary that talks straight to Prometheus, with a light SQLite vault for
snippets.

## Highlights

- **Single static binary.** Go 1.27, zero-alloc embedded assets (`go:embed` for
  templates, CSS/JS, and logos). No node build step, no CDN dependencies — HTMX and
  highlight.js are vendored.
- **Link hub.** Themed groups of external services (tools, databases, dashboards)
  rendered as navbar dropdowns and card sections.
- **Curriculum.** Long-form learning tracks (Go, TypeScript, Postgres, Docker, Rust,
  Git, Gin, Chi, Hono, Echo, Fiber, Vue, Redis) stored as markdown in
  `docs/curriculum-room`, parsed server-side into searchable, collapsible sections
  with curated official docs/video resources per track. The Go track is complete:
  19 lessons across 14 stages, every lesson with a "Check Your Understanding"
  self-check and worked solutions, current through Go 1.27 (`sync.WaitGroup.Go`,
  `testing/synctest`, `net/http.CrossOriginProtection`, container-aware
  `GOMAXPROCS`, `go vet` cgo checks).
- **Snippets.** A SQLite-backed fragment vault (`internal/snippets`) organized by
  language labels, with add / edit / delete in the UI, label management
  (`/labels`), and a migration dump (`cmd/dumpsnippets` →
  `docs/db/snippets_seed.sql`) for importing the legacy markdown vault into prod.
- **Cheat sheet.** Command cheat sheets (`docs/cheatsheet`) with fuzzy search,
  including the full Git command reference
- **Telemetry dashboard.** Prometheus-backed metrics API (`/api/metrics/*`) with
  fixed, server-side PromQL so the browser only picks a time window. Designed for
  Grafana-style rack graphs at `/dashboard`.
- **TTS read-aloud player.** Curriculum audio at `/tts`, generated with
  `internal/web/tts/generate.py` and streamed from disk with HTTP Range support.
  **WIP — TTS is still under production**; tracks and coverage are incomplete.

## Architecture

```
cmd/server            entrypoint: env loading, route wiring, graceful shutdown
internal/web          all presentation (hub, docs, metrics, TTS player)
  ├─ content.go       curriculum tracks + curated resources
  ├─ doc.go           shared markdown doc view model + per-slug cache
  ├─ cheatsheet.go    cheat sheet rendering
  ├─ metrics.go       Prometheus proxy with server-side PromQL
  ├─ tts.go           TTS manifest + mp3 streaming (Range request support)
  ├─ snippets_page.go snippet vault pages: catalog, add/edit/delete, labels
  ├─ templates/       html/template pages
  ├─ static/          vendored JS/CSS (htmx, highlight.js, hub app js)
  ├─ logos/           service logos + brand assets
  └─ tts/             generated audio + manifest.json
internal/snippets     SQLite store: snippets + enum-like labels (schema.sql)
internal/middleware   recover + request logging chain
cmd/dumpsnippets      migrates the markdown vault to a SQL dump for prod import
cmd/seedsnippets      applies a SQL dump to a snippets SQLite database
docs/                 markdown source content (curriculum, snippets, cheatsheet)
```

Rendering is buffered (`web.Render`) so a template error never sends a half page.
Markdown is parsed once per slug and cached for the process lifetime. Static assets
and logos ship inside the binary. Generated TTS audio stays on disk and is served
immutable.

## Layout map

| Route | Purpose |
|---|---|
| `/hub` | Landing page with grouped service cards |
| `/tools`, `/database`, ... | Per-group pages from the navbar config |
| `/curriculum/{slug}` | Learning tracks: go, typescript, postgres, docker, rust, git, gin, chi, hono, echo, fiber, vue, redis |
| `/snippets/{slug}` | DB-backed snippet vault grouped by label |
| `/snippets/new`, `/snippets/manage` | Add / edit / delete snippets |
| `/labels` | Manage snippet label enums |
| `/cheatsheet` | Command cheat sheets (incl. Git reference) |
| `/dashboard` | Rack telemetry (Prometheus-backed, Plane + Grafana quick links) |
| `/tts` | Curriculum read-aloud player (**WIP/under production**) |
| `/api/metrics/*` | JSON bridge over Prometheus |
| `/healthz` | Liveness probe |

## Configuration

Environment (via `.env` or process environment):

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `2602` (compose default `9088`) | HTTP listen port |
| `PROMETHEUS_URL` | `http://192.168.1.121:9090` | Prometheus endpoint for metrics |
| `SNIPPETS_DB` | — | Path to the snippets SQLite file (wires the vault when set) |
| `SNIPPETS_SEED` | — | SQL dump auto-applied when the vault is still empty |
| `CURRICULUM_DIR` / `SNIPPETS_DIR` | `docs/...` | Override markdown source dirs |

## Running

### Docker Compose (recommended)

```sh
cp .env.example .env   # or edit .env directly
docker compose up -d --build
```

The stack runs the hub container on the `nucleus` bridge network plus your
external `monitoring` network so it reaches the Prometheus container directly.
The app listens on `${PORT:-9088}` by default. SQLite lives on the `hub-data`
volume; `sqlite-web` (admin browser) is published at
`${SQLITE_WEB_PORT:-1297}` over the same file.

### Migrating the legacy snippet vault

The original markdown vault was captured to a dump for import into the prod
database:

```sh
go run ./cmd/dumpsnippets -dir docs/snippets/backup -out docs/db/snippets_seed.sql
go run ./cmd/seedsnippets -db data/nucleaus.db -dump docs/db/snippets_seed.sql
```

Both steps are idempotent: labels use INSERT OR IGNORE and seeding is skipped
when rows already exist. Set `SNIPPETS_SEED` to have the server import the dump
on first boot.

### Plain Go

```sh
go run ./cmd/server
```

Needs `PORT` and optionally `PROMETHEUS_URL`; set `SNIPPETS_DB` to enable the
SQLite snippet vault and `SNIPPETS_SEED` to import the migration dump on first
boot.

## TTS pipeline

`internal/web/tts/generate.py` renders curriculum markdown to spoken audio (edge
TTS voice) and writes `internal/web/tts/<track>/*.mp3` with a `manifest.json`.
The `/tts` page reads the manifest, and mp3s are streamed with immutable cache
headers. More tracks are being generated — the feature is a work in progress.

## Go curriculum structure

`docs/curriculum-room/backup/go/golang-detailed-curriculum-revised.md` is the
source of truth for the Go track rendered at `/curriculum/go`:

| | |
|---|---|
| Stages 1–8 | Language fundamentals, types, control flow, structs/methods/interfaces, errors, packages, concurrency basics |
| Lessons 1–11 | One lesson per stage plus projects; Lesson 9 covers goroutines, channels, and context |
| Lessons 12–16 | HTTP services, testing/profiling, API design & auth, concurrency projects, final service project |
| Lessons 17–19 | Observability (slog, metrics, pprof), CLI apps (flag, cobra, viper), packaging & delivery (Docker, releases, CI) |
| Post-course | roadmap.sh Go path mapping, continuing-education checklist, course assessment |

Lesson numbering is asserted by `TestGoCurriculumLearningAids`, which fails if a
lesson is added without updating the count — content changes can't silently
regress the sequence again.

## Development notes

- `go test ./...` covers the hub rendering path.
- Regenerate the favicon from `internal/web/logos/mallard.png` with ImageMagick
  (`convert` at 16/32/48 px into `internal/web/logos/favicon.ico`).
- Keep content-only workflows in `docs/`: markdown you write there appears as
  sections rendered by the hub without touching Go code.
