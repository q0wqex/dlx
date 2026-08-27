package main

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"Empty URL", "", true},
		{"File scheme", "file:///etc/passwd", true},
		{"Ftp scheme", "ftp://example.com/file", true},
		{"Localhost", "http://localhost:8080/test", true},
		{"127.0.0.1", "http://127.0.0.1/test", true},
		{"Valid public URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"Valid TikTok", "https://www.tiktok.com/@user/video/123456789", false},
		{"Valid Instagram", "https://www.instagram.com/reel/C0example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
