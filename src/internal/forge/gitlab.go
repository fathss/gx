package forge

import (
	"context"
	"fmt"
)

type gitlabForge struct {
	owner string
	repo  string
}

func (g *gitlabForge) PRCreateURL(base, branch string) string {
	return fmt.Sprintf("https://gitlab.com/%s/%s/-/merge_requests/new?merge_request[source_branch]=%s&merge_request[target_branch]=%s",
		g.owner, g.repo, branch, base)
}

func (g *gitlabForge) ExistingPRURL(ctx context.Context, base, branch string) (string, error) {
	return "", nil // stub — API lookup not yet implemented
}
