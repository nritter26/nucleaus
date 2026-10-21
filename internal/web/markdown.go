package web

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
	"unicode"
)

// Minimal markdown support for the curriculum and snippet docs. The sources
// are controlled files in docs/, so the parser only needs the constructs that
// actually appear there: headings, fenced code, bullet and ordered lists
// (one level of nesting), pipe tables, blockquotes and plain paragraphs.

type mdBlock struct {
	Kind  string // "p", "quote", "list", "olist", "table", "code", "head"
	Text  string
	Lang  string
	Body  string
	Items []mdItem
	Rows  [][]string // table rows, cells held raw
}

// mdItem is one list entry with optional nested bullets.
type mdItem struct {
	Text     string
	Ordered  bool
	Children []string
}

type mdSection struct {
	ID      string
	Heading string
	Query   string
	HTML    template.HTML
	Count   int // copyable code blocks

	Blocks []mdBlock
}

type mdDoc struct {
	Title    string
	Sections []mdSection
	Total    int // code blocks across all sections
}

var (
	// Links are rewritten first so later passes can not touch their URLs.
	docLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	docBoldSpan = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	docCodeSpan = regexp.MustCompile("`[^`\n]+`")

	listItem = regexp.MustCompile(`^[-*+] +(.+)$`)
	ordItem  = regexp.MustCompile(`^(\d+)[.)] +(.+)$`)
	tableSep = regexp.MustCompile(`^:?-{3,}:?$`)
)

// mdInlineHTML renders links, bold and code spans on top of an HTML-escaped
// string so no raw markup can leak through from the doc files.
func mdInlineHTML(text string) template.HTML {
	s := html.EscapeString(text)
	s = docLink.ReplaceAllStringFunc(s, func(m string) string {
		parts := docLink.FindStringSubmatch(m)
		// In-page anchors stay in the same tab.
		if strings.HasPrefix(parts[2], "#") {
			return `<a href="` + parts[2] + `">` + parts[1] + `</a>`
		}
		return `<a href="` + parts[2] + `" target="_blank" rel="noopener noreferrer">` + parts[1] + `</a>`
	})
	s = docBoldSpan.ReplaceAllString(s, "<strong>$1</strong>")
	s = docCodeSpan.ReplaceAllStringFunc(s, func(m string) string {
		return "<code>" + strings.Trim(m, "`") + "</code>"
	})
	return template.HTML(s)
}

// mdPlain strips inline markup so blocks can double as search haystacks.
func mdPlain(text string) string {
	text = docLink.ReplaceAllString(text, "$1")
	text = docBoldSpan.ReplaceAllString(text, "$1")
	return strings.Join(strings.Fields(text), " ")
}

// slugify makes an anchor id out of a heading.
func slugify(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func uniqueSlug(prefix string, seen map[string]int) string {
	if prefix == "" {
		prefix = "section"
	}
	n := seen[prefix] + 1
	seen[prefix] = n
	if n > 1 {
		return fmt.Sprintf("%s-%d", prefix, n)
	}
	return prefix
}

func splitRow(line string) []string {
	line = strings.TrimSpace(strings.Trim(line, "|"))
	cells := strings.Split(line, "|")
	out := make([]string, 0, len(cells))
	for _, c := range cells {
		out = append(out, strings.TrimSpace(c))
	}
	return out
}

func isSeparatorRow(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, c := range row {
		if !tableSep.MatchString(strings.Trim(strings.TrimSpace(c), ":")) {
			return false
		}
	}
	return true
}

type mdParser struct {
	doc   mdDoc
	sec   *mdSection
	para  []string
	items []mdItem
	table [][]string
	quote []string
	fence bool
	lang  string
	body  []string
	seen  map[string]int
}

// parseMarkdown turns a doc file into blocks grouped under "## " sections.
func parseMarkdown(content string) mdDoc {
	p := &mdParser{seen: map[string]int{}}

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(line)

		if p.fence {
			if strings.HasPrefix(trimmed, "```") {
				p.fence = false
				p.append(mdBlock{Kind: "code", Lang: p.lang, Body: strings.Trim(strings.Join(p.body, "\n"), "\n")})
				p.body, p.lang = nil, ""
				continue
			}
			p.body = append(p.body, line)
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			p.flush()
			p.fence = true
			p.lang = strings.TrimPrefix(trimmed, "```")
			continue
		}

		// Nested list items must be checked before flat bullets, since their
		// trimmed form is identical.
		if line != trimmed && (listItem.MatchString(trimmed) || ordItem.MatchString(trimmed)) {
			var text string
			if m := listItem.FindStringSubmatch(trimmed); m != nil {
				text = m[1]
			} else {
				text = ordItem.FindStringSubmatch(trimmed)[2]
			}
			if text != "" && len(p.items) > 0 {
				p.items[len(p.items)-1].Children = append(
					p.items[len(p.items)-1].Children, text)
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "## "):
			p.flush()
			p.startSection(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
		case strings.HasPrefix(trimmed, "# "):
			p.flush()
			if text := strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")); p.doc.Title == "" {
				p.doc.Title = text
			} else {
				p.startSection(text)
			}
		case strings.HasPrefix(trimmed, "### "):
			p.flush()
			p.append(mdBlock{Kind: "head", Text: strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))})
		case strings.HasPrefix(trimmed, "#### "), strings.HasPrefix(trimmed, "##### "):
			p.flush()
			p.append(mdBlock{Kind: "head", Text: strings.TrimSpace(strings.TrimLeft(trimmed, "# "))})
		case strings.HasPrefix(trimmed, "> "):
			p.flush()
			p.quote = append(p.quote, strings.TrimSpace(strings.TrimPrefix(trimmed, "> ")))
		case listItem.MatchString(trimmed):
			if len(p.items) == 0 {
				p.flush()
			}
			p.items = append(p.items, mdItem{Text: listItem.FindStringSubmatch(trimmed)[1]})
		case ordItem.MatchString(trimmed):
			if len(p.items) == 0 {
				p.flush()
			}
			p.items = append(p.items, mdItem{
				Text:    ordItem.FindStringSubmatch(trimmed)[2],
				Ordered: true,
			})
		case strings.HasPrefix(trimmed, "|"):
			if len(p.table) == 0 {
				p.flush()
			}
			if row := splitRow(trimmed); !isSeparatorRow(row) {
				p.table = append(p.table, row)
			}
		case trimmed == "---" || trimmed == "***" || trimmed == "___" || trimmed == "":
			p.flush()
		default:
			if len(p.table) > 0 || len(p.items) > 0 || len(p.quote) > 0 {
				p.flush()
			}
			p.para = append(p.para, trimmed)
		}
	}
	p.flush()

	for i := range p.doc.Sections {
		s := &p.doc.Sections[i]
		s.HTML = renderBlocks(s.Blocks)
		for _, b := range s.Blocks {
			if b.Kind == "code" {
				s.Count++
			}
		}
		s.Query = sectionQuery(s.Heading, s.Blocks)
		p.doc.Total += s.Count
	}
	return p.doc
}

// startSection opens a "## " heading. A table of contents is skipped, along
// with everything under it, to keep pages lean.
func (p *mdParser) startSection(heading string) {
	switch strings.ToLower(heading) {
	case "table of contents", "contents":
		p.sec = nil
		return
	}
	p.doc.Sections = append(p.doc.Sections, mdSection{Heading: heading})
	p.sec = &p.doc.Sections[len(p.doc.Sections)-1]
}

// flush commits any paragraph, list, table or quote being collected. It runs
// before a different construct takes over the stream and at EOF, keeping
// blocks in document order.
func (p *mdParser) flush() {
	if len(p.para) > 0 {
		p.append(mdBlock{Kind: "p", Text: strings.Join(p.para, " ")})
		p.para = nil
	}
	if len(p.quote) > 0 {
		p.append(mdBlock{Kind: "quote", Text: strings.Join(p.quote, " ")})
		p.quote = nil
	}
	if len(p.table) > 0 {
		p.append(mdBlock{Kind: "table", Rows: p.table})
		p.table = nil
	}
	if len(p.items) > 0 {
		ordered := p.items[0].Ordered
		kind := "list"
		if ordered {
			kind = "olist"
		}
		p.append(mdBlock{Kind: kind, Items: p.items})
		p.items = nil
	}
}

func (p *mdParser) append(b mdBlock) {
	if p.sec == nil {
		return
	}
	p.sec.Blocks = append(p.sec.Blocks, b)
}

const sectionQueryLimit = 600

// sectionQuery builds a searchable haystack for one collapsible section.
func sectionQuery(heading string, blocks []mdBlock) string {
	var sb strings.Builder
	sb.WriteString(heading)
	sb.WriteByte(' ')
	for _, b := range blocks {
		switch b.Kind {
		case "p", "quote", "head":
			sb.WriteString(b.Text)
			sb.WriteByte(' ')
		case "list", "olist":
			for _, it := range b.Items {
				sb.WriteString(it.Text)
				sb.WriteByte(' ')
				for _, c := range it.Children {
					sb.WriteString(c)
					sb.WriteByte(' ')
				}
			}
		case "table":
			for _, row := range b.Rows {
				sb.WriteString(strings.Join(row, " "))
				sb.WriteByte(' ')
			}
		case "code":
			runes := []rune(b.Body)
			if len(runes) > 300 {
				runes = runes[:300]
			}
			sb.WriteString(string(runes))
			sb.WriteByte(' ')
		}
	}
	q := mdPlain(sb.String())
	runes := []rune(q)
	if len(runes) > sectionQueryLimit {
		runes = append(runes[:sectionQueryLimit-1], '…')
	}
	return string(runes)
}

// renderBlocks turns parsed blocks into section HTML.
func renderBlocks(blocks []mdBlock) template.HTML {
	var b strings.Builder
	for _, bl := range blocks {
		switch bl.Kind {
		case "head":
			b.WriteString(`<h4 class="doc-sub">` + string(mdInlineHTML(bl.Text)) + `</h4>`)
		case "p":
			b.WriteString(`<p class="doc-text">` + string(mdInlineHTML(bl.Text)) + `</p>`)
		case "quote":
			b.WriteString(`<blockquote class="doc-quote">` + string(mdInlineHTML(bl.Text)) + `</blockquote>`)
		case "list", "olist":
			b.WriteString(renderList(bl))
		case "table":
			b.WriteString(renderTable(bl.Rows))
		case "code":
			b.WriteString(renderCode(bl))
		}
	}
	return template.HTML(b.String())
}

func renderList(bl mdBlock) string {
	tag := "ul"
	if bl.Kind == "olist" {
		tag = "ol"
	}
	var b strings.Builder
	b.WriteString("<" + tag + ` class="doc-list">`)
	for _, it := range bl.Items {
		b.WriteString(`<li class="doc-li">`)
		if box := checkbox(it.Text); box != "" {
			b.WriteString(box)
		} else {
			b.WriteString(string(mdInlineHTML(it.Text)))
		}
		if len(it.Children) > 0 {
			b.WriteString(`<` + tag + ` class="doc-list doc-list--sub">`)
			for _, c := range it.Children {
				b.WriteString(`<li class="doc-li">`)
				if box := checkbox(c); box != "" {
					b.WriteString(box)
				} else {
					b.WriteString(string(mdInlineHTML(c)))
				}
				b.WriteString(`</li>`)
			}
			b.WriteString(`</` + tag + `>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</` + tag + `>`)
	return b.String()
}

// checkbox renders GitHub task-list markers as styled boxes.
func checkbox(text string) string {
	switch {
	case strings.HasPrefix(text, "[ ] "), strings.HasPrefix(text, "[] "):
		return `<span class="doc-box" aria-hidden="true"></span>` +
			string(mdInlineHTML(text[4:]))
	case strings.HasPrefix(text, "[x] "), strings.HasPrefix(text, "[X] "):
		return `<span class="doc-box doc-box--done" aria-hidden="true"></span>` +
			string(mdInlineHTML(text[4:]))
	}
	return ""
}

func renderTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="doc-tablewrap"><table class="doc-table"><thead><tr>`)
	for _, c := range rows[0] {
		b.WriteString(`<th>` + string(mdInlineHTML(c)) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows[1:] {
		b.WriteString(`<tr>`)
		for _, c := range row {
			b.WriteString(`<td>` + string(mdInlineHTML(c)) + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func renderCode(bl mdBlock) string {
	var b strings.Builder
	b.WriteString(`<figure class="cheat-cmd doc-code"`)
	if bl.Lang != "" {
		b.WriteString(` data-lang="` + html.EscapeString(bl.Lang) + `"`)
	}
	b.WriteString(`><pre><code>` + html.EscapeString(bl.Body) + `</code></pre>`)
	if bl.Lang != "" {
		b.WriteString(`<span class="doc-lang">` + html.EscapeString(bl.Lang) + `</span>`)
	}
	b.WriteString(`<button class="copy" type="button" data-copy aria-label="Copy code">copy</button>`)
	b.WriteString(`</figure>`)
	return b.String()
}
