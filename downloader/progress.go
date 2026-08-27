package downloader

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	ansiRegex     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	percentRegex  = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)%`)
	speedRegex    = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?(?:[kKMGT]?i?B/s))`)
	etaRegex      = regexp.MustCompile(`ETA\s+([0-9:]+)`)
)

// ParseProgressLine parses a line from yt-dlp stdout and updates the progress event.
func ParseProgressLine(line, jobID string) (*ProgressEvent, bool) {
	cleanLine := ansiRegex.ReplaceAllString(line, "")
	cleanLine = strings.TrimSpace(cleanLine)
	if cleanLine == "" {
		return nil, false
	}

	// Structured prefix from --progress-template
	if strings.HasPrefix(cleanLine, "DLX_PROG|") {
		parts := strings.Split(cleanLine, "|")
		// DLX_PROG | status | percent_str | speed_str | eta_str | downloaded_bytes | total_bytes | total_estimate
		if len(parts) >= 5 {
			statusStr := parts[1]
			pctStr := strings.Trim(parts[2], " %")
			speedStr := strings.TrimSpace(parts[3])
			etaStr := strings.TrimSpace(parts[4])

			var pct float64
			if p, err := strconv.ParseFloat(pctStr, 64); err == nil {
				pct = p
			}

			if speedStr == "NA" || speedStr == "" {
				speedStr = "-"
			} else {
				speedStr = strings.ReplaceAll(speedStr, "iB/s", "B/s")
			}

			if etaStr == "NA" || etaStr == "" {
				etaStr = "-"
			} else {
				etaStr = formatETA(etaStr)
			}

			status := StatusDownloading
			stage := "Скачивание"
			if statusStr == "finished" || pct >= 100.0 {
				pct = 100.0
				stage = "Обработка потоков"
				status = StatusProcessing
			}

			ev := NewProgressEvent(jobID, status, pct, speedStr, etaStr, stage)
			return &ev, true
		}
	}

	// Postprocess template
	if strings.HasPrefix(cleanLine, "DLX_POST|") {
		ev := NewProgressEvent(jobID, StatusProcessing, 100.0, "-", "-", "Обработка ffmpeg")
		return &ev, true
	}

	// Fallback parsing for standard yt-dlp [download] lines
	if strings.HasPrefix(cleanLine, "[download]") {
		if strings.Contains(cleanLine, "100%") {
			ev := NewProgressEvent(jobID, StatusProcessing, 100.0, "-", "-", "Обработка ffmpeg")
			return &ev, true
		}

		var pct float64
		if match := percentRegex.FindStringSubmatch(cleanLine); len(match) > 1 {
			if p, err := strconv.ParseFloat(match[1], 64); err == nil {
				pct = p
			}
		}

		speed := "-"
		if match := speedRegex.FindStringSubmatch(cleanLine); len(match) > 1 {
			speed = strings.ReplaceAll(match[1], "iB/s", "B/s")
		}

		eta := "-"
		if match := etaRegex.FindStringSubmatch(cleanLine); len(match) > 1 {
			eta = formatETA(match[1])
		}

		if pct > 0 {
			ev := NewProgressEvent(jobID, StatusDownloading, pct, speed, eta, "Скачивание")
			return &ev, true
		}
	}

	if strings.HasPrefix(cleanLine, "[Merger]") ||
		strings.HasPrefix(cleanLine, "[Fixup") ||
		strings.HasPrefix(cleanLine, "[ExtractAudio]") ||
		strings.HasPrefix(cleanLine, "[Embed") {
		ev := NewProgressEvent(jobID, StatusProcessing, 100.0, "-", "-", "Обработка ffmpeg")
		return &ev, true
	}

	return nil, false
}

// formatETA converts HH:MM:SS or MM:SS to Russian friendly string like "18 сек" or "1 мин 20 сек".
func formatETA(etaStr string) string {
	parts := strings.Split(etaStr, ":")
	if len(parts) == 2 {
		min, err1 := strconv.Atoi(parts[0])
		sec, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil {
			if min == 0 {
				return strconv.Itoa(sec) + " сек"
			}
			return strconv.Itoa(min) + " мин " + strconv.Itoa(sec) + " сек"
		}
	} else if len(parts) == 3 {
		hr, err1 := strconv.Atoi(parts[0])
		min, err2 := strconv.Atoi(parts[1])
		sec, err3 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil && err3 == nil {
			if hr == 0 && min == 0 {
				return strconv.Itoa(sec) + " сек"
			}
			if hr == 0 {
				return strconv.Itoa(min) + " мин " + strconv.Itoa(sec) + " сек"
			}
			return strconv.Itoa(hr) + " ч " + strconv.Itoa(min) + " мин"
		}
	}
	return etaStr
}
