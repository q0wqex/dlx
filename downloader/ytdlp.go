package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// YtDlpExtractor handles interaction with yt-dlp binary.
type YtDlpExtractor struct {
	YtDlpPath  string
	FFmpegPath string
}

// rawYtDlpJSON is a minimal mapping of yt-dlp JSON dump output.
type rawYtDlpJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Duration  any    `json:"duration"`
	Extractor string `json:"extractor"`
	Formats   []struct {
		FormatID   string `json:"format_id"`
		Height     int    `json:"height"`
		Ext        string `json:"ext"`
		Vcodec     string `json:"vcodec"`
		Acodec     string `json:"acodec"`
		Resolution string `json:"resolution"`
	} `json:"formats"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

// FetchMediaInfo retrieves video metadata without downloading the content.
func (e *YtDlpExtractor) FetchMediaInfo(ctx context.Context, targetURL string) (*MediaInfo, error) {
	args := []string{
		"-J",
		"--no-playlist",
		"--no-cache-dir",
		"--skip-download",
		"--no-warnings",
		targetURL,
	}

	cmd := exec.CommandContext(ctx, e.YtDlpPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %w: %s", err, stderr.String())
	}

	var raw rawYtDlpJSON
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp JSON: %w", err)
	}

	// Calculate duration in seconds
	var durationSec int
	switch v := raw.Duration.(type) {
	case float64:
		durationSec = int(v)
	case int:
		durationSec = v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			durationSec = int(f)
		}
	}

	// Pick best thumbnail
	thumbnail := raw.Thumbnail
	if thumbnail == "" && len(raw.Thumbnails) > 0 {
		thumbnail = raw.Thumbnails[len(raw.Thumbnails)-1].URL
	}

	// Extract unique available video heights
	heightMap := make(map[int]bool)
	for _, f := range raw.Formats {
		if f.Height > 0 && f.Vcodec != "none" && f.Vcodec != "" {
			heightMap[f.Height] = true
		}
	}

	var heights []int
	for h := range heightMap {
		heights = append(heights, h)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(heights)))

	var formats []string
	for _, h := range heights {
		formats = append(formats, strconv.Itoa(h)+"p")
	}

	siteName := raw.Extractor
	if siteName == "" {
		siteName = "web"
	}

	return &MediaInfo{
		Title:     raw.Title,
		Thumbnail: thumbnail,
		Duration:  durationSec,
		Site:      siteName,
		Formats:   formats,
		Extractor: raw.Extractor,
	}, nil
}

// BuildDownloadArgs creates the command line arguments for yt-dlp execution.
func BuildDownloadArgs(ytdlpPath, ffmpegPath, maxFileSize, outputDir string, req DownloadRequest) []string {
	args := []string{
		"--no-playlist",
		"--no-cache-dir",
		"--no-mtime",
		"--newline",
		"--progress-template", "download:DLX_PROG|%(progress.status)s|%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s",
		"--progress-template", "postprocess:DLX_POST|%(progress.status)s",
	}

	if maxFileSize != "" {
		args = append(args, "--max-filesize", maxFileSize)
	}

	if ffmpegPath != "" {
		args = append(args, "--ffmpeg-location", ffmpegPath)
	}

	reqFormat := strings.ToLower(strings.TrimSpace(req.Format))
	if reqFormat == "mp3" {
		args = append(args,
			"-x",
			"--audio-format", "mp3",
			"--audio-quality", "0",
		)
	} else {
		// MP4 default
		quality := strings.ToLower(strings.TrimSpace(req.Quality))
		quality = strings.TrimSuffix(quality, "p")

		if quality == "" || quality == "best" {
			args = append(args,
				"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best",
				"--merge-output-format", "mp4",
			)
		} else {
			formatSelector := fmt.Sprintf(
				"bestvideo[height<=?%s][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=?%s]+bestaudio/best[height<=?%s]/best",
				quality, quality, quality,
			)
			args = append(args,
				"-f", formatSelector,
				"--merge-output-format", "mp4",
			)
		}
	}

	if req.Subtitles {
		args = append(args, "--write-subs", "--embed-subs")
		if req.SubtitleLanguage != "" {
			args = append(args, "--sub-langs", req.SubtitleLanguage)
		} else {
			args = append(args, "--sub-langs", "all")
		}
	}

	if req.Thumbnail {
		args = append(args, "--embed-thumbnail")
	}

	// Safe output template inside job temporary directory
	outputTemplate := filepath.Join(outputDir, "%(title).200B.%(ext)s")
	args = append(args, "-o", outputTemplate, "--", req.URL)

	return args
}
