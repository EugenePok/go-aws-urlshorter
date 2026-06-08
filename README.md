# go-aws-urlshorter

A small URL shortener written in Go using only the standard library (`net/http`).

## Features

- Create short codes for long URLs and redirect on lookup
- In-memory store behind a `Store` interface (easy to swap for a real backend)
- Structured JSON logging via `log/slog`
- Graceful shutdown on `SIGINT` / `SIGTERM`

## API

| Method | Path       | Description                   |
| ------ | ---------- | ----------------------------- |
| `GET`  | `/healthz` | Health check                  |
| `POST` | `/shorten` | Create a short link for a URL |
| `GET`  | `/{code}`  | Redirect to the original URL  |

## Running

```sh
go run ./cmd/api       # in-memory store, starts on :8080
```

Configuration via environment variables:

| Variable   | Default                 | Description                      |
| ---------- | ----------------------- | -------------------------------- |
| `ADDR`     | `:8080`                 | Address the server listens on    |
| `BASE_URL` | `http://localhost:8080` | Base URL used in shortened links |

## Testing

```sh
go test ./...
```

## Layout

```
cmd/api/            HTTP server entrypoint (wiring + graceful shutdown)
internal/api/       HTTP transport: handlers, router, status mapping
internal/shortener/ Core logic: code generation, URL validation
internal/store/     Link persistence: Store interface, memory + DynamoDB, cache decorator
internal/cache/     Cache interface + Redis implementation
internal/events/    Click-event Publisher: SQS, Noop, Async buffer
internal/worker/    SQS poll loop + batch aggregation
```
