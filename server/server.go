package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"dlx/web"
)

// Router creates the HTTP handler with all registered endpoints.
func Router(h *Handlers) http.Handler {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/api/info", h.InfoHandler)
	mux.HandleFunc("/api/download", h.DownloadHandler)

	// Sub-path routes for jobs
	mux.HandleFunc("/api/download/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/progress") {
			h.ProgressSSEHandler(w, r)
			return
		}
		if strings.HasSuffix(path, "/file") {
			h.FileHandler(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// Static Web Assets (embedded)
	staticHandler := web.Handler()
	mux.Handle("/", staticHandler)

	// Logging & Recovery Middleware
	return loggingMiddleware(recoveryMiddleware(mux))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Do not flood logs with SSE keepalive progress polling
		isSSE := strings.HasSuffix(r.URL.Path, "/progress")

		wrappedWriter := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrappedWriter, r)

		if !isSSE {
			duration := time.Since(start)
			log.Printf("[HTTP] %s %s %d (%s) - %s", r.Method, r.URL.Path, wrappedWriter.status, duration.Round(time.Millisecond), r.RemoteAddr)
		}
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] %v in %s %s", rec, r.Method, r.URL.Path)
				http.Error(w, `{"error":"Внутренняя ошибка сервера"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
