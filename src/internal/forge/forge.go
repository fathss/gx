package forge

import (
	"context"
	"fmt"
)

// Forge detects a git host and provides PR/MR URLs.
type Forge interface {
	// PRCreateURL returns the URL to create a new PR/MR from branch against base.
	PRCreateURL(base, branch string) string
	// ExistingPRURL returns the URL of an open PR/MR for this branch, or "" if none.
	// Returns an error only for actual failures (network, API error), not "no PR found".
	ExistingPRURL(ctx context.Context, base, branch string) (string, error)
}

// NewForge detects the host from remoteURL and returns the appropriate Forge.
// Returns a descriptive error for unknown hosts or malformed URLs.
func NewForge(remoteURL string) (Forge, error) {
	host, owner, repo, err := extractRepoInfo(remoteURL)
	if err != nil {
		return nil, err
	}

	switch {
	case host == "github.com":
		return &githubForge{owner: owner, repo: repo}, nil
	case host == "gitlab.com":
		return &gitlabForge{owner: owner, repo: repo}, nil
	case host == "bitbucket.org":
		return &bitbucketForge{owner: owner, repo: repo}, nil
	default:
		return nil, fmt.Errorf("unknown git host: %s", host)
	}
}
