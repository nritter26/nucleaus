package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"nucleaus/internal/middleware"
	"nucleaus/internal/snippets"
	"nucleaus/internal/web"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file found, using process environment")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "2602"
	}

	if path := os.Getenv("SNIPPETS_DB"); path != "" {
		store, err := snippets.Open(path)
		if err != nil {
			log.Fatalf("open snippets db: %v", err)
		}
		defer snippets.Close(store)
		if err := store.Migrate(context.Background()); err != nil {
			log.Fatalf("migrate snippets db: %v", err)
		}
		if path := os.Getenv("SNIPPETS_SEED"); path != "" {
			raw, err := os.ReadFile(path)
			if err != nil {
				log.Fatalf("read snippets seed %s: %v", path, err)
			}
			if err := store.ApplySeed(context.Background(), string(raw)); err != nil {
				log.Fatalf("seed snippets: %v", err)
			}
			log.Printf("snippets seed applied from %s", path)
		}
		web.SetSnippetStore(store)
		log.Printf("snippets vault wired at %s", path)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	web.RegisterStatic(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.Chain(mux, middleware.Recover, middleware.Logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	<-sigCtx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
