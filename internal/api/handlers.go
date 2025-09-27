package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"csv-search/internal/models"
	"csv-search/internal/search"
)

// Handler handles HTTP requests
type Handler struct {
	indexer  *search.Indexer
	searcher *search.Searcher
	config   *models.Config
}

// NewHandler creates a new handler instance
func NewHandler(indexer *search.Indexer, searcher *search.Searcher, config *models.Config) *Handler {
	return &Handler{
		indexer:  indexer,
		searcher: searcher,
		config:   config,
	}
}

// SearchHandler handles search requests
func (h *Handler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse query parameters
	req := &models.SearchRequest{
		Query:   r.URL.Query().Get("q"),
		Columns: strings.Split(r.URL.Query().Get("columns"), ","),
	}

	// Parse page parameter
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			req.Page = page
		} else {
			req.Page = 1
		}
	} else {
		req.Page = 1
	}

	// Parse limit parameter
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		} else {
			req.Limit = h.config.PageSize
		}
	} else {
		req.Limit = h.config.PageSize
	}

	// Filter empty columns
	filteredColumns := make([]string, 0)
	for _, col := range req.Columns {
		if col != "" {
			filteredColumns = append(filteredColumns, col)
		}
	}
	req.Columns = filteredColumns

	// Check if index is ready
	if !h.indexer.IsIndexed() {
		h.writeError(w, http.StatusServiceUnavailable, "Index is not ready", "Index is still being built")
		return
	}

	// Perform search
	response, err := h.searcher.Search(ctx, req)
	if err != nil {
		log.Printf("Search error: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Search failed", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// ColumnsHandler returns available columns
func (h *Handler) ColumnsHandler(w http.ResponseWriter, r *http.Request) {
	columns := h.indexer.GetColumns()
	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"columns": columns,
	})
}

// StatsHandler returns file statistics
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.GetFileStats()
	if stats == nil {
		h.writeError(w, http.StatusServiceUnavailable, "Stats not available", "Index is not ready")
		return
	}
	h.writeJSON(w, http.StatusOK, stats)
}

// ProgressHandler returns indexing progress
func (h *Handler) ProgressHandler(w http.ResponseWriter, r *http.Request) {
	progress := h.indexer.GetProgress()
	h.writeJSON(w, http.StatusOK, progress)
}

// SuggestionsHandler returns search suggestions
func (h *Handler) SuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")

	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	suggestions, err := h.searcher.GetSuggestions(ctx, query, limit)
	if err != nil {
		log.Printf("Suggestions error: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to get suggestions", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"suggestions": suggestions,
	})
}

// HealthHandler returns health status
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	status := "healthy"
	code := http.StatusOK

	if !h.indexer.IsIndexed() {
		status = "indexing"
		code = http.StatusOK
	}

	h.writeJSON(w, code, map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"indexed":   h.indexer.IsIndexed(),
	})
}

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// writeError writes an error response
func (h *Handler) writeError(w http.ResponseWriter, status int, message, details string) {
	error := models.APIError{
		Error:   message,
		Code:    status,
		Message: details,
	}
	h.writeJSON(w, status, error)
}

// CORSHandler handles CORS preflight requests
func (h *Handler) CORSHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingHandler logs HTTP requests
func (h *Handler) LoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

// RecoveryHandler recovers from panics
func (h *Handler) RecoveryHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				h.writeError(w, http.StatusInternalServerError, "Internal server error", "An unexpected error occurred")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
