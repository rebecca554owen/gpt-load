package utils

import (
	"path/filepath"
	"strings"
)

// HasWildcard checks if the pattern contains wildcard characters
func HasWildcard(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

// MatchWildcard checks if the text matches the wildcard pattern
// Supports * and ? wildcards, case-insensitive
func MatchWildcard(pattern, text string) bool {
	pattern = strings.ToLower(pattern)
	text = strings.ToLower(text)

	matched, err := filepath.Match(pattern, text)
	if err != nil {
		return false
	}
	return matched
}
