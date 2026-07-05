package git

import (
	"path/filepath"
	"strings"
)

// MatchSensitivePatterns checks each filename against the given patterns using
// filepath.Match glob semantics. Returns a list of matched filenames with the
// pattern they matched, e.g. ".env matched .env" or "credentials.key matched *.key".
// Patterns come from the config file — there are no hardcoded defaults.
func MatchSensitivePatterns(files []string, patterns []string) []string {
	var matches []string
	for _, file := range files {
		baseName := filepath.Base(file)
		for _, pattern := range patterns {
			// Match against both the full path and the base name
			if matched, _ := filepath.Match(pattern, file); matched {
				matches = append(matches, file+" matched "+pattern)
				break
			}
			if matched, _ := filepath.Match(pattern, baseName); matched {
				matches = append(matches, file+" matched "+pattern)
				break
			}
		}
	}
	return matches
}

// FilterExcluded removes files that match any of the given exclude patterns.
// Uses the same glob semantics as MatchSensitivePatterns (full path + base name).
func FilterExcluded(files []string, excludePatterns []string) []string {
	if len(excludePatterns) == 0 {
		return files
	}
	var result []string
	for _, f := range files {
		matched := false
		baseName := filepath.Base(f)
		for _, pattern := range excludePatterns {
			if m, _ := filepath.Match(pattern, f); m {
				matched = true
				break
			}
			if m, _ := filepath.Match(pattern, baseName); m {
				matched = true
				break
			}
		}
		if !matched {
			result = append(result, f)
		}
	}
	return result
}

// SanitizePatterns cleans and deduplicates a list of glob patterns.
// Empty strings and duplicates are removed.
func SanitizePatterns(patterns []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		result = append(result, p)
	}
	return result
}
