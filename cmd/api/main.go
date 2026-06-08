package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"urlshortener/internal/api"
	"urlshortener/internal/cache"
	"urlshortener/internal/events"
	"urlshortener/internal/shortener"
	"urlshortener/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		runHealthcheck()
		return
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := getenv("ADDR", ":8080")
	baseUrl := getenv("BASE_URL", "http://localhost:8080")

	st, err := buildStore(context.Background(), log)
	if err != nil {
		log.Error("failed to build store", "err", err)
		os.Exit(1)
	}
	st = maybeWrapCache(st, log)
	svc := shortener.NewService(st)
	publisher, closePublisher, err := buildPublisher(context.Background(), log)
	if err != nil {
		log.Error("failed to build publisher", "err", err)
		os.Exit(1)
	}
	h := api.NewHandler(svc, publisher, baseUrl, log)

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(h),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("server starting", "addr", addr, "base_url", baseUrl)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("server shutting down")
	shutdownctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownctx); err != nil {
		log.Error("shutdown error", "err", err)
	}
	closePublisher()
}

func maybeWrapCache(base store.Store, log *slog.Logger) store.Store {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		log.Info("cache disabled (REDIS_ADDR unset)")
		return base
	}
	ttl := getDuration("CACHE_TTL", 5*time.Minute)
	redisCache := cache.NewRedis(addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := redisCache.Ping(ctx); err != nil {
		log.Warn("redis unreachable at startup; will serve from store and retry lazily", "err", err, "addr", addr)
	} else {
		log.Info("redis cache enabled", "addr", addr, "ttl", ttl.String())
	}
	return store.NewCached(base, redisCache, ttl, log)
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func buildStore(ctx context.Context, log *slog.Logger) (store.Store, error) {
	switch getenv("STORE", "memory") {
	case "dynamodb":
		endpoint := getenv("AWS_ENDPOINT_URL", "")
		client, err := store.NewDynamoClient(ctx, store.DynamoClientConfig{
			Region:   getenv("AWS_REGION", "us-east-1"),
			Endpoint: endpoint,
		})
		if err != nil {
			return nil, err
		}
		table := getenv("DYNAMODB_TABLE", "url_shortener_links")
		//only auto-create on localstack not AWS.
		if endpoint != "" {
			if err := store.EnsureTable(ctx, client, table); err != nil {
				return nil, err
			}
		}
		log.Info("using dynamodb store", "table", table, "endpoint", endpoint)
		return store.NewDynamo(client, table), nil
	default:
		log.Info("using in-memory store")
		return store.NewMemory(), nil
	}
}

func buildPublisher(ctx context.Context, log *slog.Logger) (events.Publisher, func(), error) {
	if getenv("EVENTS", "noop") != "sqs" {
		log.Info("click events disabled (EVENTS != sqs)")
		return events.Noop{}, func() {}, nil
	}

	endpoint := os.Getenv("AWS_ENDPOINT_URL") // empty means real AWS
	client, err := events.NewSQSClient(ctx, events.SQSClientConfig{
		Region:   getenv("AWS_REGION", "us-east-1"),
		Endpoint: endpoint,
	})
	if err != nil {
		return nil, nil, err
	}
	queueName := getenv("SQS_QUEUE_NAME", "url_shortener_clicks")
	var queueURL string
	if endpoint != "" {
		queueURL, err = events.EnsureQueue(ctx, client, queueName)
	} else {
		queueURL, err = events.QueueURL(ctx, client, queueName)
	}
	if err != nil {
		return nil, nil, err
	}

	async := events.NewAsync(events.NewSQS(client, queueURL), 1024, 2, log)
	log.Info("click events -> SQS", "queue", queueName, "url", queueURL)
	return async, async.Close, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
