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

	ctx := context.Background()

	pool, err := db.New(ctx)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	if os.Getenv("MIGRATE_ONLY") == "true" {
		log.Println("migrations complete, exiting")
		return
	}

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
