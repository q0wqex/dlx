package main

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		ext      string
		fallback string
		want     string
	}{
		{
			name:     "Empty input",
			input:    "",
			ext:      ".mp4",
			fallback: "video.mp4",
			want:     "video.mp4",
		},
		{
			name:     "Standard title",
			input:    "My Awesome Video",
			ext:      ".mp4",
			fallback: "video.mp4",
			want:     "My Awesome Video.mp4",
		},
		{
			name:     "Title with invalid chars",
			input:    `Video: Part 1/2? "Best" <Review> | *Special*`,
			ext:      ".mp4",
			fallback: "video.mp4",
			want:     "Video_ Part 1_2_ _Best_ _Review_ _ _Special.mp4",
		},
		{
			name:     "Russian unicode title",
			input:    "Очень смешной ролик (2026)",
			ext:      ".mp4",
			fallback: "video.mp4",
			want:     "Очень смешной ролик (2026).mp4",
		},
		{
			name:     "Only invalid characters",
			input:    `//:::***???`,
			ext:      ".mp4",
			fallback: "video.mp4",
			want:     "video.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input, tt.ext, tt.fallback)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
