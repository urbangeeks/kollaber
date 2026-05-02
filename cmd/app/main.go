package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/urbangeeks/kollaber/internal/api"
	"github.com/urbangeeks/kollaber/internal/db"
	"github.com/urbangeeks/kollaber/internal/store"
)

func main() {
	_ = godotenv.Load()

	pool, err := db.New(context.Background())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	q := store.New(pool)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := api.NewRouter(q)
	log.Printf("starting on :%s", port)
	if err := r.Start(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
