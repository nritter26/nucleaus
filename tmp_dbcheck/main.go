package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

type row struct{ title, source string }

// tokenize walks the dump statement by statement using the same quoting
// grammar q() in cmd/dumpsnippets writes ('...' with '' escapes).
func tokenize(dump string) ([]row, error) {
	var rows []row
	i := 0
	for i < len(dump) {
		if dump[i] != '\'' { // scan to next quoted string
			i++
			continue
		}
		// read the string
		i++
		var fields []string
		var cur []byte
		for {
			if i >= len(dump) {
				return nil, fmt.Errorf("unterminated string at %d", i)
			}
			if dump[i] == '\'' {
				if i+1 < len(dump) && dump[i+1] == '\'' {
					cur = append(cur, '\'')
					i += 2
					continue
				}
				i++
				fields = append(fields, string(cur))
				cur = cur[:0]
				// after a string, decide if another value follows
				for i < len(dump) && (dump[i] == ' ' || dump[i] == '\n' || dump[i] == ',') {
					if dump[i] == ',' {
						// next value may be subselect/keyword; scan to next quote or statement end
						i++
						j := i
						for j < len(dump) && (dump[j] == ' ' || dump[j] == '\n') {
							j++
						}
						if j < len(dump) && dump[j] == '(' {
							// skip subselect to closing paren
							depth := 0
							for j < len(dump) {
								if dump[j] == '(' {
									depth++
								} else if dump[j] == ')' {
									depth--
									if depth == 0 {
										break
									}
								}
								j++
							}
							i = j + 1
						} else if j < len(dump) && (dump[j] == 'd' || dump[j] == 'n') {
							// datetime('now') or NULL-ish keyword: skip to end of statement
							for i < len(dump) && dump[i] != ';' {
								i++
							}
						}
						if i < len(dump) && dump[i] == '\'' {
							i++
							continue // read next string value
						}
					} else {
						i++
					}
				}
				break
			}
			cur = append(cur, dump[i])
			i++
		}
		if len(fields) == 0 {
			continue
		}
		// label INSERTs have 3 fields (name, slug); snippet INSERTs have title, code, '', lang, source
		switch len(fields) {
		case 4: // title, code, "", lang + source follows in fields? no: snippets have title,code,notes,lang,source = 5 strings
			continue
		case 3:
			continue
		case 5:
			rows = append(rows, row{title: fields[0], source: fields[4]})
		}
	}
	return rows, nil
}

func main() {
	dumpPath := os.Args[1]
	raw, err := os.ReadFile(dumpPath)
	if err != nil {
		log.Fatal(err)
	}
	pairs, err := tokenize(string(raw))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("parsed %d snippet rows from dump", len(pairs))

	db, err := sql.Open("sqlite", "data/nucleaus.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	existing := map[string]bool{}
	rows, err := db.QueryContext(ctx, "SELECT title, source FROM snippets")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var t, s string
		rows.Scan(&t, &s)
		existing[s + "\x00" + t] = true
	}
	rows.Close()

	skipped := 0
	for _, p := range pairs {
		if existing[p.source+"\x00"+p.title] {
			skipped++
		}
	}
	log.Printf("%d rows already present, %d to insert", skipped, len(pairs)-skipped)

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tx.Exec(string(raw)); err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	var n int
	db.QueryRowContext(ctx, "SELECT count(*) FROM snippets").Scan(&n)
	log.Printf("vault now holds %d snippets", n)
	for _, s := range []string{"golang", "typescript", "postgres", "bash", "docker", "rust"} {
		var c int
		db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM snippets sn JOIN labels l ON l.id = sn.label_id WHERE l.slug = ?), (SELECT count(*) FROM labels WHERE slug = ?)`, s, s).Scan(&c, &c)
		fmt.Printf("  %s: %d\n", s, c)
	}
}
