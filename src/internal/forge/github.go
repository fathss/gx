package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type githubForge struct {
	owner string
	repo  string
}

func (g *githubForge) PRCreateURL(base, branch string) string {
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s?expand=1", g.owner, g.repo, base, branch)
}

func (g *githubForge) ExistingPRURL(ctx context.Context, base, branch string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open", g.owner, g.repo, g.owner, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var pulls []struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pulls); err != nil {
		return "", err
	}

	if len(pulls) > 0 {
		return pulls[0].HTMLURL, nil
	}
	return "", nil
}
