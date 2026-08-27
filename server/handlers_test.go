package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"dlx/downloader"
)

func mockValidateURL(rawURL string) (*url.URL, error) {
	if strings.Contains(rawURL, "invalid") {
		return nil, downloader.ErrJobNotFound
	}
	return url.Parse(rawURL)
}

func setupTestRouter() http.Handler {
	jobManager := downloader.NewJobManager("yt-dlp", "ffmpeg", "5G", "/tmp/dlx_test", 10*time.Minute, 2, func(err error, raw string) string { return err.Error() }, func(r, e, f string) string { return r + e })
	handlers := &Handlers{
		JobManager:  jobManager,
		Extractor:   &downloader.YtDlpExtractor{YtDlpPath: "yt-dlp", FFmpegPath: "ffmpeg"},
		ValidateURL: mockValidateURL,
		InfoTimeout: 2 * time.Second,
	}
	return Router(handlers)
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", w.Code, http.StatusOK)
	}

	var res map[string]string
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", res["status"])
	}
}

func TestStaticIndex(t *testing.T) {
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "DLX") {
		t.Errorf("Index page does not contain 'DLX'")
	}
	if !strings.Contains(body, "Video downloader") {
		t.Errorf("Index page does not contain 'Video downloader'")
	}
}

func TestDownloadValidation(t *testing.T) {
	router := setupTestRouter()

	body := []byte(`{"url":"invalid-url"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/download", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /api/download with invalid URL status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
