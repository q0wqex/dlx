package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// Invalid characters on Windows/Linux/macOS filesystems: / \ : * ? " < > |
	invalidFileChars = regexp.MustCompile(`[/\\:*?"<>|]+`)
	// Whitespace normalization
	multiSpace = regexp.MustCompile(`\s+`)
)

// SanitizeFilename creates a clean, cross-platform filesystem safe filename with appropriate extension.
func SanitizeFilename(rawName, defaultExt, fallbackName string) string {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return fallbackName
	}

	// Replace invalid characters with underscore or remove them
	name = invalidFileChars.ReplaceAllString(name, "_")

	// Strip control characters (ASCII 0-31, 127)
	var sb strings.Builder
	for _, r := range name {
		if r >= 32 && r != 127 {
			sb.WriteRune(r)
		}
	}
	name = sb.String()

	// Normalize spaces
	name = multiSpace.ReplaceAllString(name, " ")
	name = strings.Trim(name, " ._")

	// Max safe length (in characters, around 150)
	if utf8.RuneCountInString(name) > 150 {
		runes := []rune(name)
		name = string(runes[:150])
		name = strings.Trim(name, " ._")
	}

	if name == "" {
		return fallbackName
	}

	// Ensure extension
	ext := filepath.Ext(name)
	if ext == "" {
		if !strings.HasPrefix(defaultExt, ".") && defaultExt != "" {
			defaultExt = "." + defaultExt
		}
		name = name + defaultExt
	}

	return name
}
