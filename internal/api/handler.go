package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"urlshortener/internal/shortener"
	"urlshortener/internal/store"
)

type Shortener interface {
	Shorten(ctx context.Context, longURL string) (string, error)
	Resolve(ctx context.Context, code string) (string, error)
}

type Handler struct {
	svc     Shortener
	baseUrl string
	log     *slog.Logger
}

func NewHandler(svc Shortener, baseUrl string, log *slog.Logger) *Handler {
	return &Handler{svc: svc, baseUrl: strings.TrimRight(baseUrl, "/"), log: log}
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	var requestBody shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSON(w, http.StatusBadRequest, &errorResponse{Error: "invalid JSON body"})
		return
	}
	code, err := h.svc.Shorten(r.Context(), requestBody.URL)
	if err != nil {
		if errors.Is(err, shortener.ErrInvalidURL) {
			writeJSON(w, http.StatusBadRequest, &errorResponse{Error: err.Error()})
			return
		}
		h.log.Error("shorten failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, &errorResponse{Error: "internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, &shortenResponse{
		Code:     code,
		ShortURL: h.baseUrl + "/" + code,
	})
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	longUrl, err := h.svc.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, &errorResponse{Error: err.Error()})
			return
		}
		h.log.Error("resolve failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, &errorResponse{Error: "internal server error"})
		return
	}
	http.Redirect(w, r, longUrl, http.StatusMovedPermanently)
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
