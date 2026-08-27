package downloader

import (
	"time"
)

// MediaInfo contains metadata extracted from a media URL.
type MediaInfo struct {
	Title     string   `json:"title"`
	Thumbnail string   `json:"thumbnail"`
	Duration  int      `json:"duration"`
	Site      string   `json:"site"`
	Formats   []string `json:"formats,omitempty"`
	Extractor string   `json:"extractor,omitempty"`
}

// DownloadRequest represents a user request to download media.
type DownloadRequest struct {
	URL              string `json:"url"`
	Format           string `json:"format"`           // "mp4" (default) or "mp3"
	Quality          string `json:"quality"`          // "best", "2160", "1440", "1080", "720", "480", "360"
	Subtitles        bool   `json:"subtitles"`        // embed subtitles if available
	SubtitleLanguage string `json:"subtitleLanguage"` // e.g. "ru,en"
	Thumbnail        bool   `json:"thumbnail"`        // embed thumbnail into file
}

// ProgressStatus represents the state of a download job.
type ProgressStatus string

const (
	StatusPending    ProgressStatus = "pending"
	StatusDownloading ProgressStatus = "downloading"
	StatusProcessing  ProgressStatus = "processing"
	StatusCompleted   ProgressStatus = "completed"
	StatusError       ProgressStatus = "error"
)

// ProgressEvent is sent to the client via SSE or polling.
type ProgressEvent struct {
	ID        string         `json:"id"`
	Status    ProgressStatus `json:"status"`
	Percent   float64        `json:"percent"`
	Speed     string         `json:"speed"`
	ETA       string         `json:"eta"`
	Stage     string         `json:"stage"`
	Error     string         `json:"error,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	FileSize  int64          `json:"fileSize,omitempty"`
	Timestamp int64          `json:"timestamp"`
}

// NewProgressEvent creates a ProgressEvent with current timestamp.
func NewProgressEvent(id string, status ProgressStatus, percent float64, speed, eta, stage string) ProgressEvent {
	return ProgressEvent{
		ID:        id,
		Status:    status,
		Percent:   percent,
		Speed:     speed,
		ETA:       eta,
		Stage:     stage,
		Timestamp: time.Now().UnixMilli(),
	}
}
