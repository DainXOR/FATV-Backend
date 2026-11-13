package configs

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type frontendNS struct{}

// Frontend exposes configuration used when linking back to the web application.
var Frontend frontendNS

// BaseURL returns the absolute public URL of the frontend.
// It is intentionally required instead of guessing a host in production.
func (frontendNS) BaseURL() (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL")), "/")
	if baseURL == "" {
		return "", fmt.Errorf("FRONTEND_BASE_URL is not configured")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("FRONTEND_BASE_URL must be an absolute http or https URL")
	}
	return baseURL, nil
}

// URL builds an absolute frontend URL for emails and shareable form links.
func (frontendNS) URL(path string) (string, error) {
	baseURL, err := Frontend.BaseURL()
	if err != nil {
		return "", err
	}
	if path == "" || path[0] != '/' {
		return "", fmt.Errorf("frontend URL path must begin with /")
	}
	return baseURL + path, nil
}
