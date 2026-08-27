package downloader

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrServerBusy   = errors.New("сервер перегружен, попробуйте позже")
	ErrJobNotFound  = errors.New("загрузка не найдена")
	ErrJobNotReady  = errors.New("файл ещё не готов")
	ErrFileNotFound = errors.New("готовый файл не найден")
)

// DownloadJob represents an active or completed download task.
type DownloadJob struct {
	ID         string
	Request    DownloadRequest
	TempDir    string
	FilePath   string
	FileName   string
	FileSize   int64
	LastEvent  ProgressEvent
	CreatedAt  time.Time
	CancelFunc context.CancelFunc

	mu          sync.RWMutex
	subscribers map[chan ProgressEvent]struct{}
	doneChan    chan struct{}
}

func newJob(req DownloadRequest, tempBaseDir string) (*DownloadJob, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate job ID: %w", err)
	}
	id := hex.EncodeToString(b)

	jobTempDir := filepath.Join(tempBaseDir, id)
	if err := os.MkdirAll(jobTempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	initialEvent := NewProgressEvent(id, StatusPending, 0, "-", "-", "Инициализация")

	return &DownloadJob{
		ID:          id,
		Request:     req,
		TempDir:     jobTempDir,
		LastEvent:   initialEvent,
		CreatedAt:   time.Now(),
		subscribers: make(map[chan ProgressEvent]struct{}),
		doneChan:    make(chan struct{}),
	}, nil
}

// Subscribe registers a listener channel for progress events.
func (j *DownloadJob) Subscribe() chan ProgressEvent {
	j.mu.Lock()
	defer j.mu.Unlock()

	ch := make(chan ProgressEvent, 50)
	j.subscribers[ch] = struct{}{}

	// Send last known event immediately to new subscriber
	ch <- j.LastEvent
	return ch
}

// Unsubscribe removes a listener channel.
func (j *DownloadJob) Unsubscribe(ch chan ProgressEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()

	delete(j.subscribers, ch)
	close(ch)
}

// Broadcast sends progress event to all active subscribers.
func (j *DownloadJob) Broadcast(event ProgressEvent) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.LastEvent = event
	for ch := range j.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber channel full, skip to avoid blocking
		}
	}
}

// GetStatus returns current progress event snapshot.
func (j *DownloadJob) GetStatus() ProgressEvent {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.LastEvent
}
