package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/urbangeeks/kollaber/internal/api"
	"github.com/urbangeeks/kollaber/internal/db"
	"github.com/urbangeeks/kollaber/internal/digest"
	"github.com/urbangeeks/kollaber/internal/resend"
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

	// The weekly digest schedules itself in-process so it works on an install
	// with no cron. Running it on every replica is safe: the send is claimed in
	// digest_sends, so exactly one of them mails each org.
	digestScheduler := digest.NewScheduler(q, func(recipients []string, w digest.Weekly) error {
		return resend.SendHTML(recipients, w.Subject(), w.HTML(), w.DevLine())
	})
	go digestScheduler.Start(ctx)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := api.NewRouter(q, pool)
	log.Printf("starting on :%s", port)
	if err := r.Start(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
