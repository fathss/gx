package forge

import (
	"context"
	"fmt"
)

type bitbucketForge struct {
	owner string
	repo  string
}

func (g *bitbucketForge) PRCreateURL(base, branch string) string {
	return fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/new?source=%s&dest=%s",
		g.owner, g.repo, branch, base)
}

func (g *bitbucketForge) ExistingPRURL(ctx context.Context, base, branch string) (string, error) {
	return "", nil // stub — API lookup not yet implemented
}
