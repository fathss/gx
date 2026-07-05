# Forge Domain

**Files:** `internal/forge/forge.go`, `internal/forge/url.go`, `internal/forge/github.go`, `internal/forge/gitlab.go`, `internal/forge/bitbucket.go`

## Overview

The forge package detects the remote git host from a remote URL and provides PR/MR creation and discovery URLs. It is consumed by the ship workflow to print a link after a successful push.

Supports three hosts: GitHub, GitLab, and Bitbucket.

---

## Forge (interface)

```go
type Forge interface {
    PRCreateURL(base, branch string) string
    ExistingPRURL(ctx context.Context, base, branch string) (string, error)
}
```

### PRCreateURL

Returns the URL to create a new pull request / merge request from `branch` against `base`. Always succeeds — URL construction is deterministic, no API calls.

### ExistingPRURL

Returns the URL of an already-open PR/MR for this branch, or `""` if none. Returns an error **only** for actual failures (network, API error), not for "no PR found".

| Edge case | Behaviour |
|---|---|
| No open PR for branch | Returns `"", nil` |
| API unreachable/timeout | Returns error — caller falls through to `PRCreateURL` |
| GitHub API returns non-200 | Returns error |

---

## NewForge

```go
func NewForge(remoteURL string) (Forge, error)
```

Detects the host from `remoteURL` via `extractRepoInfo` and returns the matching forge implementation.

| Remote host | Implementation |
|---|---|
| `github.com` | `githubForge` |
| `gitlab.com` | `gitlabForge` |
| `bitbucket.org` | `bitbucketForge` |
| other | Error: `unknown git host` |

---

## URL parsing (extractRepoInfo)

```go
func extractRepoInfo(remoteURL string) (host, owner, repo string, err error)
```

Parses remote URLs in three formats:

| Format | Example |
|---|---|
| SSH (`git@host:owner/repo.git`) | `git@github.com:user/repo.git` |
| SSH (`ssh://git@host/owner/repo.git`) | `ssh://git@github.com/user/repo.git` |
| HTTPS | `https://github.com/user/repo.git` |

Strips `.git` suffix, splits into `host` / `owner` / `repo`.

---

## Host implementations

### GitHub (`githubForge`)

| Method | Behaviour |
|---|---|
| `PRCreateURL` | `https://github.com/{owner}/{repo}/compare/{base}...{branch}?expand=1` |
| `ExistingPRURL` | Calls `GET /repos/{owner}/{repo}/pulls?head={owner}:{branch}&state=open`, parses JSON response for `html_url`. Fully implemented. |

### GitLab (`gitlabForge`)

| Method | Behaviour |
|---|---|
| `PRCreateURL` | `https://gitlab.com/{owner}/{repo}/-/merge_requests/new?merge_request[source_branch]={branch}&merge_request[target_branch]={base}` |
| `ExistingPRURL` | **Stub** — returns `"", nil` (not yet implemented). |

### Bitbucket (`bitbucketForge`)

| Method | Behaviour |
|---|---|
| `PRCreateURL` | `https://bitbucket.org/{owner}/{repo}/pull-requests/new?source={branch}&dest={base}` |
| `ExistingPRURL` | **Stub** — returns `"", nil` (not yet implemented). |

---

## Consumption in workflows

Used in `internal/workflow/ship.go` after a successful push:

1. Fetch the remote URL via `git.RemoteURL()`
2. Call `forge.NewForge(remoteURL)` — if host is unknown, print a warning and exit gracefully (push already succeeded)
3. Try `ExistingPRURL` with a 10-second context timeout
4. If no existing PR found (or API failed), fall back to `PRCreateURL`
5. Print the URL

Errors from the forge layer never fail the ship workflow — the push has already completed.
