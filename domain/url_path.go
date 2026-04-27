package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildRelativeEscapedURLPath joins path segments and escapes each segment while preserving slashes.
func BuildRelativeEscapedURLPath(prefixPath, objectPath string) string {
	segments := append(splitURLPathSegments(prefixPath), splitURLPathSegments(objectPath)...)
	if len(segments) == 0 {
		return "/"
	}
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return "/" + strings.Join(segments, "/")
}

// BuildAbsoluteEscapedURLPath resolves an escaped relative path against an absolute base URL.
func BuildAbsoluteEscapedURLPath(baseURL, prefixPath, objectPath string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("invalid base url %q: %w", baseURL, err)
	}
	relativePath := BuildRelativeEscapedURLPath(prefixPath, objectPath)
	rel, err := url.Parse(relativePath)
	if err != nil {
		return "", fmt.Errorf("invalid escaped relative path %q: %w", relativePath, err)
	}
	return base.ResolveReference(rel).String(), nil
}

func splitURLPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
