package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dlx/downloader"
)

// URLValidatorFunc validates and resolves a URL to prevent SSRF.
type URLValidatorFunc func(rawURL string) (*url.URL, error)

// Handlers holds dependencies for HTTP request handlers.
type Handlers struct {
	JobManager   *downloader.JobManager
	Extractor    *downloader.YtDlpExtractor
	ValidateURL  URLValidatorFunc
	InfoTimeout  time.Duration
}

// HealthHandler returns basic server health status.
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// InfoRequest represents the JSON body for /api/info.
type InfoRequest struct {
	URL string `json:"url"`
}

// InfoHandler retrieves media metadata for preview.
func (h *Handlers) InfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON запрос")
		return
	}

	validatedURL, err := h.ValidateURL(req.URL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.InfoTimeout)
	defer cancel()

	info, err := h.Extractor.FetchMediaInfo(ctx, validatedURL.String())
	if err != nil {
		log.Printf("[INFO ERROR] URL=%s error=%v", validatedURL.String(), err)
		writeJSONError(w, http.StatusUnprocessableEntity, "Не удалось извлечь информацию о видео")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(info)
}

// DownloadHandler creates and starts a download task.
func (h *Handlers) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req downloader.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Некорректный JSON запрос")
		return
	}

	validatedURL, err := h.ValidateURL(req.URL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.URL = validatedURL.String()

	job, err := h.JobManager.StartJob(req)
	if err != nil {
		if err == downloader.ErrServerBusy {
			writeJSONError(w, http.StatusTooManyRequests, "Сервер сейчас перегружен. Пожалуйста, повторите попытку позже.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Не удалось создать задачу на скачивание")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": job.ID,
	})
}

// ProgressSSEHandler streams real-time progress events using Server-Sent Events.
func (h *Handlers) ProgressSSEHandler(w http.ResponseWriter, r *http.Request) {
	// Extract {id} from path /api/download/{id}/progress
	path := strings.TrimPrefix(r.URL.Path, "/api/download/")
	id := strings.TrimSuffix(path, "/progress")
	id = strings.Trim(id, "/")

	if id == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, ok := h.JobManager.GetJob(id)
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Essential for Nginx proxying

	subCh := job.Subscribe()
	defer job.Unsubscribe(subCh)

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case event, ok := <-subCh:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if event.Status == downloader.StatusCompleted || event.Status == downloader.StatusError {
				return
			}
		}
	}
}

// FileHandler streams the completed video/audio file to the client.
func (h *Handlers) FileHandler(w http.ResponseWriter, r *http.Request) {
	// Extract {id} from path /api/download/{id}/file
	path := strings.TrimPrefix(r.URL.Path, "/api/download/")
	id := strings.TrimSuffix(path, "/file")
	id = strings.Trim(id, "/")

	if id == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, ok := h.JobManager.GetJob(id)
	if !ok {
		http.Error(w, "Download job not found or expired", http.StatusNotFound)
		return
	}

	status := job.GetStatus()
	if status.Status != downloader.StatusCompleted || job.FilePath == "" {
		http.Error(w, "File is not ready", http.StatusConflict)
		return
	}

	file, err := os.Open(job.FilePath)
	if err != nil {
		http.Error(w, "File error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "File error", http.StatusInternalServerError)
		return
	}

	ext := filepath.Ext(job.FileName)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		if strings.ToLower(ext) == ".mp4" {
			mimeType = "video/mp4"
		} else if strings.ToLower(ext) == ".mp3" {
			mimeType = "audio/mpeg"
		} else {
			mimeType = "application/octet-stream"
		}
	}

	// Content-Disposition with UTF-8 filename encoding (RFC 6266 / RFC 5987)
	asciiName := sanitizeASCII(job.FileName)
	utf8Name := url.PathEscape(job.FileName)
	disposition := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiName, utf8Name)

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("X-Accel-Buffering", "no")

	// Serve the content via http.ServeContent (supports range requests, zero in-memory buffer overhead)
	http.ServeContent(w, r, job.FileName, fileInfo.ModTime(), file)

	// Clean up after 1 minute of serving to ensure partial chunks/resumes work if client requests
	go func() {
		time.Sleep(1 * time.Minute)
		h.JobManager.DeleteJob(id)
	}()
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func sanitizeASCII(input string) string {
	var sb strings.Builder
	for _, r := range input {
		if r > 31 && r < 127 && r != '"' && r != '\\' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := strings.TrimSpace(sb.String())
	if res == "" {
		return "file.bin"
	}
	return res
}
