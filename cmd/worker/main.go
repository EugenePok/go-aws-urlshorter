package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"urlshortener/internal/events"
	"urlshortener/internal/stats"
	"urlshortener/internal/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- MySQL ---
	dsn := getenv("MYSQL_DSN", "root:root@tcp(localhost:3306)/urlshortener?parseTime=true&loc=UTC")
	db, err := stats.Open(dsn)
	if err != nil {
		log.Error("open mysql", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := waitForDB(ctx, db, log); err != nil {
		log.Error("mysql not reachable", "err", err)
		os.Exit(1)
	}
	if err := stats.EnsureSchema(ctx, db); err != nil {
		log.Error("ensure schema", "err", err)
		os.Exit(1)
	}

	// --- SQS ---
	endpoint := os.Getenv("AWS_ENDPOINT_URL") // empty => real AWS
	client, err := events.NewSQSClient(ctx, events.SQSClientConfig{
		Region:   getenv("AWS_REGION", "us-east-1"),
		Endpoint: endpoint,
	})
	if err != nil {
		log.Error("sqs client", "err", err)
		os.Exit(1)
	}
	queueName := getenv("SQS_QUEUE_NAME", "url_shortener_clicks")
	var queueURL string
	if endpoint != "" {
		queueURL, err = events.EnsureQueue(ctx, client, queueName)
	} else {
		queueURL, err = events.QueueURL(ctx, client, queueName)
	}
	if err != nil {
		log.Error("resolve queue", "err", err)
		os.Exit(1)
	}

	w := worker.New(client, queueURL, stats.NewMySQL(db), log)
	if err := w.Run(ctx); err != nil {
		log.Error("worker exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("worker stopped cleanly")
}

func waitForDB(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Info("waiting for mysql", "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
