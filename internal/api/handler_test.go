package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"urlshortener/internal/events"
	"urlshortener/internal/shortener"
	"urlshortener/internal/store"
)

type recordingPublisher struct {
	events []events.ClickEvent
}

func (r *recordingPublisher) PublishClick(_ context.Context, e events.ClickEvent) error {
	r.events = append(r.events, e)
	return nil
}

func newTestHandler(pub events.Publisher) *Handler {
	svc := shortener.NewService(store.NewMemory())
	return NewHandler(svc, pub, "http://short.test",
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newTestRouter() *http.ServeMux {
	return NewRouter(newTestHandler(events.Noop{}))
}

func TestHandler_ShortenThenRedirect(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("shorten status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var resp shortenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(resp.ShortURL, "http://short.test/") {
		t.Errorf("short_url = %q, missing base prefix", resp.ShortURL)
	}

	req = httptest.NewRequest(http.MethodGet, "/"+resp.Code, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("redirect status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://example.com" {
		t.Errorf("Location = %q, want original URL", loc)
	}
}

func TestHandler_Redirect_PublishesClick(t *testing.T) {
	rec := &recordingPublisher{}
	h := newTestHandler(rec)
	router := NewRouter(h)

	post := httptest.NewRecorder()
	router.ServeHTTP(post, httptest.NewRequest(
		http.MethodPost,
		"/shorten",
		strings.NewReader(`{"url":"https://example.com"}`)))

	var resp shortenResponse
	_ = json.NewDecoder(post.Body).Decode(&resp)

	req := httptest.NewRequest(http.MethodGet, "/"+resp.Code, nil)
	req.Header.Set("User-Agent", "test-agent")
	router.ServeHTTP(httptest.NewRecorder(), req)
	if len(rec.events) != 1 {
		t.Fatalf("published %d click events, want 1", len(rec.events))
	}
	if rec.events[0].Code != resp.Code {
		t.Errorf("click code = %q, want %q", rec.events[0].Code, resp.Code)
	}
	if rec.events[0].UserAgent != "test-agent" {
		t.Errorf("click user-agent = %q, want test-agent", rec.events[0].UserAgent)
	}
}

func TestHandler_Redirect_NotFound_DoesNotPublish(t *testing.T) {
	rec := &recordingPublisher{}
	handler := newTestHandler(rec)
	router := NewRouter(handler)
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))
	if len(rec.events) != 0 {
		t.Errorf("published %d events on 404, want 0", len(rec.events))
	}
}

func TestHandler_Shorten_BadJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost,
			"/shorten",
			strings.NewReader("{bad")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_Shorten_InvalidURL(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"ftp://x"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_Redirect_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandler_Health(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
