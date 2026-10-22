// Package snippets provides a small SQLite-backed store for the snippet vault:
// user-managed code snippets plus enum-like labels used to categorize them.
package snippets

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL embed.FS

type Label struct {
	ID   int64
	Name string
	Slug string
	// Count of snippets using this label.
	Count int
}

type Snippet struct {
	ID      int64
	Title   string
	Code    string
	Notes   string
	Lang    string
	LabelID sql.NullInt64
	Source  string // "seed" for migrated vault rows, "custom" for user-added
	Created string
	Updated string
}

// SnippetInput carries editable fields for create/update.
type SnippetInput struct {
	Title   string
	Code    string
	Notes   string
	Lang    string
	LabelID int64 // 0 = no label
	Source  string
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Migrate applies the embedded schema. Safe to call on every boot.
func (s *Store) Migrate(ctx context.Context) error {
	raw, err := fs.ReadFile(schemaSQL, "schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, string(raw)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// ApplySeed runs a SQL dump (from cmd/dumpsnippets) once, only when the
// snippets table is still empty, so prod imports are idempotent.
func (s *Store) ApplySeed(ctx context.Context, dump string) error {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM snippets").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, dump); err != nil {
		return fmt.Errorf("apply seed: %w", err)
	}
	return nil
}

func Close(s *Store) error { return s.db.Close() }

func slugifyName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ---------- labels ----------

func (s *Store) ListLabels(ctx context.Context) ([]Label, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.name, l.slug,
		       (SELECT count(*) FROM snippets sn WHERE sn.label_id = l.id) AS n
		FROM labels l
		ORDER BY l.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Label{}
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.Slug, &l.Count); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) LabelBySlug(ctx context.Context, slug string) (Label, bool, error) {
	var l Label
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug FROM labels WHERE slug = ?`, slug).
		Scan(&l.ID, &l.Name, &l.Slug)
	if err == sql.ErrNoRows {
		return Label{}, false, nil
	}
	if err != nil {
		return Label{}, false, err
	}
	return l, true, nil
}

func (s *Store) CreateLabel(ctx context.Context, name string) (Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Label{}, fmt.Errorf("label name is required")
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO labels (name, slug) VALUES (?, ?)`, name, slugifyName(name))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Label{}, fmt.Errorf("label %q already exists", name)
		}
		return Label{}, err
	}
	id, _ := res.LastInsertId()
	return Label{ID: id, Name: name, Slug: slugifyName(name)}, nil
}

func (s *Store) DeleteLabel(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("label %d not found", id)
	}
	return nil
}

// ---------- snippets ----------

func (s *Store) ListSnippets(ctx context.Context, labelID int64) ([]Snippet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, code, notes, lang, label_id, source, created_at, updated_at
		FROM snippets
		WHERE (? = 0 OR label_id = ?)
		ORDER BY source, created_at, id`, labelID, labelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnippets(rows)
}

// SnippetCount reports how many snippets are stored (0 == unseeded).
func (s *Store) SnippetCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM snippets`).Scan(&n)
	return n, err
}

func (s *Store) Snippet(ctx context.Context, id int64) (Snippet, bool, error) {
	var sn Snippet
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, code, notes, lang, label_id, source, created_at, updated_at
		FROM snippets WHERE id = ?`, id).
		Scan(&sn.ID, &sn.Title, &sn.Code, &sn.Notes, &sn.Lang, &sn.LabelID, &sn.Source, &sn.Created, &sn.Updated)
	if err == sql.ErrNoRows {
		return Snippet{}, false, nil
	}
	if err != nil {
		return Snippet{}, false, err
	}
	return sn, true, nil
}

func (s *Store) CreateSnippet(ctx context.Context, in SnippetInput) (int64, error) {
	in, err := normalize(in)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO snippets (title, code, notes, lang, label_id, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, ''), ?, ?)`,
		in.Title, in.Code, in.Notes, in.Lang, in.LabelID, in.Source, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateSnippet(ctx context.Context, id int64, in SnippetInput) error {
	in, err := normalize(in)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE snippets SET title = ?, code = ?, notes = ?, lang = ?, label_id = NULLIF(?, 0), updated_at = ?
		WHERE id = ?`,
		in.Title, in.Code, in.Notes, in.Lang, in.LabelID, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("snippet %d not found", id)
	}
	return nil
}

func (s *Store) DeleteSnippet(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM snippets WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("snippet %d not found", id)
	}
	return nil
}

func normalize(in SnippetInput) (SnippetInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, fmt.Errorf("snippet title is required")
	}
	in.Code = strings.Trim(in.Code, "\n")
	if in.Code == "" {
		return in, fmt.Errorf("snippet code is required")
	}
	in.Lang = strings.TrimSpace(in.Lang)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Source == "" {
		in.Source = "custom"
	}
	return in, nil
}

func scanSnippets(rows *sql.Rows) ([]Snippet, error) {
	out := []Snippet{}
	for rows.Next() {
		var sn Snippet
		if err := rows.Scan(&sn.ID, &sn.Title, &sn.Code, &sn.Notes, &sn.Lang, &sn.LabelID, &sn.Source, &sn.Created, &sn.Updated); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}
