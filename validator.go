package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrInvalidURL   = errors.New("некорректный формат ссылки")
	ErrInvalidScheme = errors.New("поддерживаются только ссылки http:// и https://")
	ErrBlockedHost  = errors.New("доступ к локальным и приватным адресам запрещён")
)

// ValidateURL performs structural validation and SSRF checks on the provided URL.
func ValidateURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, errors.New("ссылка не может быть пустой")
	}
	if len(trimmed) > 4096 {
		return nil, errors.New("ссылка слишком длинная")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, ErrInvalidURL
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrInvalidScheme
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return nil, ErrInvalidURL
	}

	lowerHost := strings.ToLower(hostname)
	if isBlockedHostname(lowerHost) {
		return nil, ErrBlockedHost
	}

	// Resolve host IPs to prevent SSRF against loopback or private networks
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// If DNS resolution fails, reject to prevent blind exploitation
		return nil, fmt.Errorf("не удалось разрешить адрес %s: %w", hostname, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("адрес %s не найден", hostname)
	}

	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return nil, ErrBlockedHost
		}
	}

	return parsed, nil
}

func isBlockedHostname(host string) bool {
	if host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".lan") ||
		strings.HasSuffix(host, ".localdomain") {
		return true
	}
	return false
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified() {
		return true
	}

	// Additional check for 0.0.0.0/8 and 169.254.0.0/16
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 || (ip4[0] == 169 && ip4[1] == 254) {
			return true
		}
	}

	return false
}
