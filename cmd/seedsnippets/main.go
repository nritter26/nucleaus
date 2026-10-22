// Command seedsnippets applies a SQL dump (from cmd/dumpsnippets) to the
// snippets SQLite database. It only imports when the vault is empty, so
// retries are safe.
//
// Usage:
//
//	go run ./cmd/seedsnippets -db data/nucleaus.db -dump docs/db/snippets_seed.sql
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"nucleaus/internal/snippets"
)

func main() {
	dbPath := flag.String("db", os.Getenv("SNIPPETS_DB"), "sqlite database path")
	dump := flag.String("dump", "docs/db/snippets_seed.sql", "SQL dump to apply")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("db path required (-db or SNIPPETS_DB)")
	}

	store, err := snippets.Open(*dbPath)
	if err != nil {
		log.Fatalf("open %s: %v", *dbPath, err)
	}
	defer snippets.Close(store)

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	raw, err := os.ReadFile(*dump)
	if err != nil {
		log.Fatalf("read dump: %v", err)
	}
	if err := store.ApplySeed(ctx, string(raw)); err != nil {
		log.Fatalf("seed: %v", err)
	}

	n, err := store.SnippetCount(ctx)
	if err != nil {
		log.Fatalf("count: %v", err)
	}
	log.Printf("snippets vault holds %d rows", n)
}
