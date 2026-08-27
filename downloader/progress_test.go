package downloader

import (
	"testing"
)

func TestParseProgressLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		jobID       string
		wantMatched bool
		wantStatus  ProgressStatus
		wantPercent float64
		wantSpeed   string
		wantETA     string
	}{
		{
			name:        "Structured DLX_PROG line",
			line:        "DLX_PROG|downloading| 72.4%|14.2MiB/s|00:18|7589201|10485760|10485760",
			jobID:       "test-job",
			wantMatched: true,
			wantStatus:  StatusDownloading,
			wantPercent: 72.4,
			wantSpeed:   "14.2MB/s",
			wantETA:     "18 сек",
		},
		{
			name:        "Structured finished line",
			line:        "DLX_PROG|finished|100.0%|10.0MiB/s|00:00|10485760|10485760|10485760",
			jobID:       "test-job",
			wantMatched: true,
			wantStatus:  StatusProcessing,
			wantPercent: 100.0,
			wantSpeed:   "10.0MB/s",
		},
		{
			name:        "Standard fallback line",
			line:        "[download]  45.0% of   50.00MiB at   12.4MiB/s ETA 00:05",
			jobID:       "test-job",
			wantMatched: true,
			wantStatus:  StatusDownloading,
			wantPercent: 45.0,
			wantSpeed:   "12.4MB/s",
			wantETA:     "5 сек",
		},
		{
			name:        "Merger line",
			line:        "[Merger] Merging formats into 'video.mp4'",
			jobID:       "test-job",
			wantMatched: true,
			wantStatus:  StatusProcessing,
			wantPercent: 100.0,
		},
		{
			name:        "Irrelevant line",
			line:        "[info] Extracting URL...",
			jobID:       "test-job",
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, matched := ParseProgressLine(tt.line, tt.jobID)
			if matched != tt.wantMatched {
				t.Fatalf("ParseProgressLine(%q) matched = %v, wantMatched %v", tt.line, matched, tt.wantMatched)
			}
			if matched {
				if ev.Status != tt.wantStatus {
					t.Errorf("Status = %v, want %v", ev.Status, tt.wantStatus)
				}
				if ev.Percent != tt.wantPercent {
					t.Errorf("Percent = %v, want %v", ev.Percent, tt.wantPercent)
				}
				if tt.wantSpeed != "" && ev.Speed != tt.wantSpeed {
					t.Errorf("Speed = %v, want %v", ev.Speed, tt.wantSpeed)
				}
				if tt.wantETA != "" && ev.ETA != tt.wantETA {
					t.Errorf("ETA = %v, want %v", ev.ETA, tt.wantETA)
				}
			}
		})
	}
}
