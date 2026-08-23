package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"knowdo/api/internal/ai"
	"knowdo/api/internal/httpserver"
	"knowdo/api/internal/queue"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("pinging database: %v", err)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL is required")
	}

	jobs, err := queue.NewRedis(redisURL)
	if err != nil {
		log.Fatalf("configuring redis: %v", err)
	}

	if err := jobs.Ping(ctx); err != nil {
		log.Fatalf("pinging redis: %v", err)
	}

	aiURL := os.Getenv("AI_URL")
	if aiURL == "" {
		log.Fatal("AI_URL is required")
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := httpserver.NewRouter(pool, jobs, ai.NewClient(aiURL))

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
