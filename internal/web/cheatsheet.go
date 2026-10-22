package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// The cheat sheet is built from the markdown references in docs/commands.
// Parsing happens once per directory and is cached for the process lifetime.

type cheatEntry struct {
	Section string
	Title   string
	Command string
	Desc    string
	Lang    string
}

type cheatSection struct {
	Title   string
	Entries []cheatEntry
}

type cheatDoc struct {
	Slug     string
	Label    string
	Title    string
	Sections []cheatSection
	Count    int
}

var (
	numberedItem = regexp.MustCompile(`^\d+\.\s+(.+)$`)
	numberedHead = regexp.MustCompile(`^\d+\.\s*(.+)$`)
	boldSpan     = regexp.MustCompile(`\*\*(.+?)\*\*`)
	inlineSpan   = regexp.MustCompile("`([^`]+)`")
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
)

var docLabels = map[string]string{
	"git-command-reference-cheat-sheet":                    "Git",
	"bash-commands-for-backend-dev-and-devops":             "Bash",
	"docker-commands-for-backend-devops":                   "Docker",
	"golang-backend-commands":                              "Go CLI",
	"golang-backend-helper-functions":                      "Go Helpers",
	"golang-backend-utility-functions":                     "Go Utilities",
	"golang-backend-utility-structs":                       "Go Structs",
	"golang-utility-interfaces-for-backend-development":    "Go Interfaces",
	"postgresql-commands-for-backend-devops":               "Postgres",
	"postgresql-tables-and-constraints-for-backend-devops": "PG Schema",
	"rust-backend-commands":                                "Rust",
	"typescript-backend-commands":                          "TypeScript",
	"typescript-backend-api-scaffolds-patterns":            "TS Scaffolds",
	"typescript-backend-helper-functions":                  "TS Helpers",
	"typescript-backend-utility-functions":                 "TS Utilities",
	"golang-backend-api-scaffolds":                         "API Scaffolds",
	"go-fintech-api-scaffolds":                             "Fintech APIs",
	"golang-roadmap":                                       "Go Roadmap",
	"golang-nested-functions":                              "Go Nested",
	"rust-nested-functions":                                "Rust Nested",
	"docker-nested-functions":                              "Docker Nested",
	"typescript-nested-functions":                          "TS Nested",
	"bash-nested-functions":                                "Bash Nested",
}

var docOrder = []string{
	"golang-backend-commands",
	"typescript-backend-commands",
	"rust-backend-commands",
	"bash-commands-for-backend-dev-and-devops",
	"docker-commands-for-backend-devops",
	"postgresql-commands-for-backend-devops",
	"postgresql-tables-and-constraints-for-backend-devops",
	"golang-backend-api-scaffolds",
	"go-fintech-api-scaffolds",
	"golang-roadmap",
	"golang-backend-helper-functions",
	"golang-backend-utility-functions",
	"golang-backend-utility-structs",
	"golang-utility-interfaces-for-backend-development",
	"golang-nested-functions",
	"typescript-nested-functions",
	"rust-nested-functions",
	"bash-nested-functions",
	"docker-nested-functions",
}

// commandsDir resolves where the cheat sheet markdown lives.
func commandsDir() string {
	if dir := os.Getenv("COMMANDS_DIR"); dir != "" {
		return dir
	}
	candidates := []string{"docs/cheatsheet", "../docs/cheatsheet", "../../docs/cheatsheet"}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "docs/cheatsheet"
}

var (
	cheatMu    sync.Mutex
	cheatDir   string
	cheatCache []cheatDoc
	cheatErr   error
	cheatDone  bool
)

func loadCommands(dir string) ([]cheatDoc, error) {
	cheatMu.Lock()
	defer cheatMu.Unlock()
	if cheatDone && cheatDir == dir {
		return cheatCache, cheatErr
	}
	docs, err := parseCommands(dir)
	cheatDir, cheatCache, cheatErr, cheatDone = dir, docs, err, true
	return docs, err
}

func parseCommands(dir string) ([]cheatDoc, error) {
	files, err := collectCheatFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("read commands dir: %w", err)
	}
	sort.Strings(files)

	docs := make([]cheatDoc, 0, len(files))
	for _, name := range files {
		raw, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		doc := parseDoc(filepath.Base(name), string(raw))
		if doc.Count > 0 {
			docs = append(docs, doc)
		}
	}

	rank := func(slug string) int {
		for i, s := range docOrder {
			if s == slug {
				return i
			}
		}
		return len(docOrder)
	}
	sort.SliceStable(docs, func(i, j int) bool {
		ri, rj := rank(docs[i].Slug), rank(docs[j].Slug)
		if ri == rj {
			return docs[i].Slug < docs[j].Slug
		}
		return ri < rj
	})
	return docs, nil
}

// collectCheatFiles walks the command reference root recursively (archived
// markdown may live in nested folders such as backup/) and adds the archived
// reference shelves kept beside the snippet vault, de-duplicated by filename.
func collectCheatFiles(dir string) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	collect := func(root string) error {
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return nil
		}
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			if seen[d.Name()] {
				return nil
			}
			seen[d.Name()] = true
			files = append(files, path)
			return nil
		})
	}
	if err := collect(dir); err != nil {
		return nil, err
	}
	for _, extra := range cheatExtraDirs() {
		if err := collect(extra); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// cheatExtraDirs lists supplementary reference dirs: archived reference
// markdown kept alongside the snippet vault, override via CHEATSHEET_EXTRA_DIR.
func cheatExtraDirs() []string {
	if dir := os.Getenv("CHEATSHEET_EXTRA_DIR"); dir != "" {
		return []string{dir}
	}
	return []string{filepath.Join(snippetsRoot(), "backup")}
}

func docSlug(filename string) string {
	base := strings.TrimSuffix(filename, ".md")
	base = strings.TrimPrefix(base, "100_")
	return strings.ReplaceAll(base, "_", "-")
}

func docLabel(filename string) string {
	slug := docSlug(filename)
	if label, ok := docLabels[slug]; ok {
		return label
	}
	return strings.ToUpper(strings.ReplaceAll(slug, "-", " "))
}

type draftEntry struct {
	title string
	lines []string
}

func parseDoc(filename, content string) cheatDoc {
	doc := cheatDoc{Slug: docSlug(filename), Label: docLabel(filename)}

	var section *cheatSection
	var draft *draftEntry
	var inFence bool
	skipSection := false
	sectionLines := [][]string{}

	flush := func() {
		if draft == nil {
			return
		}
		if section != nil {
			if entry, ok := buildEntry(*draft); ok {
				section.Entries = append(section.Entries, entry)
				doc.Count++
			}
		}
		draft = nil
	}

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)

		if section != nil && !skipSection {
			sectionLines[len(sectionLines)-1] = append(sectionLines[len(sectionLines)-1], line)
		}

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			if draft != nil {
				draft.lines = append(draft.lines, line)
			}
			continue
		}
		if inFence {
			if draft != nil {
				draft.lines = append(draft.lines, line)
			}
			continue
		}

		switch {
		case !inFence && strings.HasPrefix(trimmed, "# "):
			flush()
			section = nil
			if doc.Title == "" {
				doc.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			}
		case strings.HasPrefix(trimmed, "## "):
			flush()
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if m := numberedHead.FindStringSubmatch(title); m != nil {
				title = m[1]
			}
			if strings.EqualFold(title, "Table of Contents") {
				section = nil
				skipSection = true
				continue
			}
			skipSection = false
			doc.Sections = append(doc.Sections, cheatSection{Title: title})
			section = &doc.Sections[len(doc.Sections)-1]
			sectionLines = append(sectionLines, []string{})
		case strings.HasPrefix(trimmed, "### "):
			flush()
			if section == nil || skipSection {
				continue
			}
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			if m := numberedHead.FindStringSubmatch(title); m != nil {
				title = m[1]
			}
			draft = &draftEntry{title: cleanup(title)}
		default:
			if section == nil || skipSection {
				continue
			}
			if m := numberedItem.FindStringSubmatch(trimmed); m != nil {
				flush()
				draft = &draftEntry{lines: []string{m[1]}}
				continue
			}
			if draft != nil {
				draft.lines = append(draft.lines, line)
			}
		}
	}
	flush()

	flush()

	// Sections that hold nothing but annotated fences (comment lines inside a
	// fenced block, one command per comment) fall back to a comment-driven
	// split, like the Git command reference.
	for i := range doc.Sections {
		sec := &doc.Sections[i]
		if len(sec.Entries) > 0 {
			continue
		}
		if i < len(sectionLines) {
			for _, entry := range commentedFenceEntries(sectionLines[i]) {
				sec.Entries = append(sec.Entries, entry)
				doc.Count++
			}
		}
	}

	return doc
}

// commentedFenceEntries splits fenced blocks whose lines carry "# comment" +
// command pairs into individual entries.
func commentedFenceEntries(lines []string) []cheatEntry {
	var out []cheatEntry
	var draft *draftEntry
	inFence := false

	flush := func(command string) {
		if draft == nil {
			return
		}
		cmd := strings.TrimSpace(strings.Join(append(draft.lines, command), "\n"))
		if cmd != "" && draft.title != "" {
			out = append(out, cheatEntry{Title: draft.title, Command: strings.Trim(cmd, "\n")})
		}
		draft = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			flush("")
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		if title, ok := fenceCommentTitle(line); ok {
			flush("")
			if title != "" {
				draft = &draftEntry{title: title, lines: []string{}}
			}
			continue
		}
		if trimmed == "" {
			continue
		}
		if draft == nil {
			draft = &draftEntry{title: "", lines: []string{}}
		}
		draft.lines = append(draft.lines, line)
	}
	flush("")
	return out
}

// fenceCommentTitle recognises the comment styles used to annotate fenced
// cheat sheet entries: "# description" (Git reference) and
// "// 1. description" (TypeScript helper shelves). The bool reports whether the
// line is a comment at all; an empty title marks a bare separator comment.
func fenceCommentTitle(line string) (string, bool) {
	// Indented comments annotate the code inside an entry; only column-zero
	// comments introduce a new one.
	if line != strings.TrimLeft(line, " \t") {
		return "", false
	}
	var body string
	switch {
	case strings.HasPrefix(line, "//"):
		body = strings.TrimSpace(strings.TrimPrefix(line, "//"))
	case strings.HasPrefix(line, "# "):
		body = strings.TrimSpace(strings.TrimPrefix(line, "# "))
	default:
		return "", false
	}
	if m := numberedHead.FindStringSubmatch(body); m != nil {
		body = strings.TrimSpace(m[1])
	}
	if strings.Trim(body, "-=_* #") == "" {
		return "", true
	}
	return body, true
}

func buildEntry(d draftEntry) (cheatEntry, bool) {
	text := strings.Join(d.lines, "\n")
	code, lang, rest := splitFence(text)

	title := cleanup(d.title)
	if title == "" {
		if m := boldSpan.FindStringSubmatch(rest); m != nil {
			title = cleanup(m[1])
			rest = strings.Replace(rest, m[0], "", 1)
		}
	}

	command := strings.TrimSpace(code)
	if command == "" {
		command, rest = takeCommand(rest)
	}
	if command == "" && title == "" {
		return cheatEntry{}, false
	}

	desc := cleanup(rest)

	// Drop a leading repeat of the command plus any dash/colon separator.
	cleanCommand := cleanup(command)
	if cleanCommand != "" {
		switch {
		case desc == cleanCommand:
			desc = ""
		case strings.HasPrefix(desc, cleanCommand):
			desc = strings.TrimSpace(strings.TrimPrefix(desc, cleanCommand))
		default:
			// Docs often repeat the tool name ("ls – List ...") while the fenced
			// example adds flags ("ls -la"). Strip the bare token in that case.
			if token, rest, ok := splitLeadToken(desc); ok && strings.HasPrefix(cleanCommand, token) {
				desc = rest
			}
		}
	}
	desc = strings.TrimLeft(desc, "–—:-|, ")
	desc = strings.TrimRight(desc, "–—:- ")
	desc = strings.TrimSpace(desc)

	if title == "" {
		title = command
	}

	return cheatEntry{
		Title:   title,
		Command: command,
		Desc:    desc,
		Lang:    lang,
	}, true
}

// splitFence pulls the first fenced code block out of an entry block.
func splitFence(text string) (code, lang, rest string) {
	lines := strings.Split(text, "\n")
	var out []string
	var fence []string
	inFence := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				fence = nil
				if code == "" {
					lang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				}
				continue
			}
			inFence = false
			if code == "" {
				code = strings.Join(fence, "\n")
			}
			continue
		}
		if inFence {
			fence = append(fence, line)
			continue
		}
		out = append(out, line)
	}
	if inFence && code == "" {
		code = strings.Join(fence, "\n")
	}
	return code, lang, strings.Join(out, "\n")
}

// takeCommand finds the first inline code span and returns the command plus the
// text that is left over for the description.
func takeCommand(text string) (command, rest string) {
	spans := inlineSpan.FindAllStringSubmatch(text, -1)
	if len(spans) == 0 {
		return "", text
	}

	line := firstNonEmptyLine(text)
	lineSpans := inlineSpan.FindAllStringSubmatch(line, -1)
	if len(lineSpans) >= 2 {
		remainder := inlineSpan.ReplaceAllString(line, "")
		remainder = strings.TrimSpace(strings.NewReplacer(",", "", "or", "", "and", "", "/", "", "|", "", "–", "", "-", "").Replace(remainder))
		if remainder == "" {
			parts := make([]string, 0, len(lineSpans))
			for _, span := range lineSpans {
				parts = append(parts, strings.TrimSpace(span[1]))
			}
			command = strings.Join(parts, " or ")
			return command, strings.Replace(text, line, "", 1)
		}
	}

	command = strings.TrimSpace(spans[0][1])
	rest = strings.Replace(text, spans[0][0], "", 1)
	return command, rest
}

// splitLeadToken splits "ls – List files" into ("ls", "List files").
func splitLeadToken(text string) (token, rest string, ok bool) {
	for i, r := range text {
		if !strings.ContainsRune("–—:-", r) {
			continue
		}
		token = strings.TrimSpace(text[:i])
		if token == "" || strings.ContainsAny(token, " ,") {
			return "", "", false
		}
		rest = strings.TrimLeft(text[i:], "–—:- ")
		return token, strings.TrimSpace(rest), true
	}
	return "", "", false
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func cleanup(text string) string {
	text = mdLink.ReplaceAllString(text, "$1")
	text = boldSpan.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func handleCheatSheet(w http.ResponseWriter, r *http.Request) {
	docs, err := loadCommands(commandsDir())

	data := pageData{
		Title:      "Cheat Sheet",
		Page:       "cheatsheet",
		Nav:        nav(),
		CheatDocs:  docs,
		CheatError: "",
	}
	if err != nil {
		data.CheatError = err.Error()
	}

	total := 0
	for _, doc := range docs {
		total += doc.Count
	}
	data.CheatTotal = total

	Render(w, "cheatsheet_page", data)
}
