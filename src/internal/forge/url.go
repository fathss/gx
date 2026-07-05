package forge

import (
	"fmt"
	"path"
	"strings"
)

// extractRepoInfo parses a git remote URL into (host, owner, repo).
// Supports SSH (git@host:owner/repo.git) and HTTPS (https://host/owner/repo.git) formats.
func extractRepoInfo(remoteURL string) (host string, owner string, repo string, err error) {
	url := remoteURL

	// Handle ssh://git@host/owner/repo.git
	if strings.HasPrefix(url, "ssh://") {
		url = strings.TrimPrefix(url, "ssh://")
		// Strip user@ part
		if atIdx := strings.Index(url, "@"); atIdx != -1 {
			url = url[atIdx+1:]
		}
		slashIdx := strings.Index(url, "/")
		if slashIdx == -1 {
			return "", "", "", fmt.Errorf("cannot parse SSH URL: %s", remoteURL)
		}
		host = url[:slashIdx]
		url = url[slashIdx+1:]
	} else if strings.Contains(url, "@") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		// Handle git@host:owner/repo.git (SSH)
		parts := strings.SplitN(url, "@", 2)
		afterAt := parts[1]
		// Split on first colon to get host
		colonIdx := strings.Index(afterAt, ":")
		if colonIdx == -1 {
			return "", "", "", fmt.Errorf("cannot parse SSH URL: %s", remoteURL)
		}
		host = afterAt[:colonIdx]
		url = afterAt[colonIdx+1:]
	} else {
		// HTTPS: https://github.com/owner/repo.git
		for _, prefix := range []string{"https://", "http://"} {
			url = strings.TrimPrefix(url, prefix)
		}
		slashIdx := strings.Index(url, "/")
		if slashIdx == -1 {
			return "", "", "", fmt.Errorf("cannot parse HTTPS URL: %s", remoteURL)
		}
		host = url[:slashIdx]
		url = url[slashIdx+1:]
	}

	// Strip .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Split owner/repo
	owner = path.Dir(url)
	repo = path.Base(url)

	if owner == "." || repo == "" {
		return "", "", "", fmt.Errorf("cannot extract owner/repo from: %s", remoteURL)
	}

	return host, owner, repo, nil
}
