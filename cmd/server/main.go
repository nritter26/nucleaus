package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"nucleaus/internal/db"
	"nucleaus/internal/middleware"
	"nucleaus/internal/user"
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

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "2602"
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := db.NewClient(sigCtx, dsn)
	if err != nil {
	log.Fatalf("failed to connect to db: %v", err)
	}
	defer client.Close()

	mux := http.NewServeMux()
	web.RegisterStatic(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	if err := client.Pool.Ping(r.Context()); err != nil {
		http.Error(w, "db down", 503)
		return
	}
	w.Write([]byte("ok"))
	})
	user.RegisterRoutes(mux, client.Pool)

	srv := &http.Server{
	Addr:    ":" + port,
	Handler: middleware.Chain(mux, middleware.Recover, middleware.Logger),
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