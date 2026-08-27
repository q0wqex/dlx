package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Config holds the runtime configuration for DLX.
type Config struct {
	Port                   string
	MaxConcurrentDownloads int
	MaxFileSize            string
	DownloadTimeout        time.Duration
	TempDir                string
	YtDlpPath              string
	FFmpegPath             string
}

// LoadConfig loads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	port := getEnv("PORT", "8080")

	maxConcurrent := 2
	if val := os.Getenv("MAX_CONCURRENT_DOWNLOADS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxConcurrent = parsed
		}
	}

	maxFileSize := getEnv("MAX_FILE_SIZE", "5G")

	downloadTimeout := 30 * time.Minute
	if val := os.Getenv("DOWNLOAD_TIMEOUT"); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil && parsed > 0 {
			downloadTimeout = parsed
		}
	}

	defaultTemp := "/tmp/dlx"
	if runtime.GOOS == "windows" {
		defaultTemp = filepath.Join(os.TempDir(), "dlx")
	}
	tempDir := getEnv("TEMP_DIR", defaultTemp)

	defaultYtDlp := "/usr/local/bin/yt-dlp"
	if runtime.GOOS == "windows" {
		defaultYtDlp = "yt-dlp"
	}
	// Check YTDLP_PATH, fallback to YT_DLP_PATH or default
	ytDlpPath := os.Getenv("YTDLP_PATH")
	if ytDlpPath == "" {
		ytDlpPath = getEnv("YT_DLP_PATH", defaultYtDlp)
	}

	defaultFFmpeg := "/usr/bin/ffmpeg"
	if runtime.GOOS == "windows" {
		defaultFFmpeg = "ffmpeg"
	}
	ffmpegPath := getEnv("FFMPEG_PATH", defaultFFmpeg)

	return &Config{
		Port:                   port,
		MaxConcurrentDownloads: maxConcurrent,
		MaxFileSize:            maxFileSize,
		DownloadTimeout:        downloadTimeout,
		TempDir:                tempDir,
		YtDlpPath:              ytDlpPath,
		FFmpegPath:             ffmpegPath,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
