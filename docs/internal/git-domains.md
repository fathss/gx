# Git Domains Reference

The `internal/git` package provides thin, testable Go wrappers around git commands. Each function corresponds to a specific git operation. Git domain functions take a `runner.Executor` instance and return raw errors without constructing `cli.Error` values.

---

## 1. Repository & Ref Vocabulary

### `repository.go`

- **`IsRepository(run runner.Executor) bool`**  
  Runs `git rev-parse --git-dir`. Returns `true` if current directory is inside a git repo. Used by the central pre-flight guard `RequireRepository()`.

### `ref.go`

- **`Qualify(remote, branch string) string`**  
  Returns `branch` qualified with `remote` (e.g., `origin/main`). Idempotent: returns already-qualified refs unchanged. Returns `branch` unchanged if `remote` is empty.
- **`StripRemote(remote, ref string) string`**  
  Removes the `<remote>/` prefix from a remote-tracking ref and returns the bare branch name. Non-matching refs are returned unchanged.

---

## 2. Branch & HEAD State

### `branch.go`

- **`GetHEADState(run runner.Executor) (*HEADState, error)`**  
  Returns `*HEADState` with fields `Branch` (empty if detached), `ShortHash` (abbreviated 7-character hash), and `IsDetached` (boolean).
- **`ShortHeadHash(run runner.Executor) (string, error)`**  
  Returns 7-character abbreviated hash of `HEAD`. Returns empty string on an empty repository without commits.
- **`CurrentBranch(run runner.Executor) (string, error)`**  
  Returns checked-out branch name, or `""` if `HEAD` is detached.
- **`Checkout(run runner.Executor, branch string) error`**  
  Runs `git checkout <branch>`. Fails if uncommitted changes conflict.
- **`LocalBranchExists(run runner.Executor, branch string) bool`**  
  Runs `git rev-parse --verify refs/heads/<branch>`.
- **`MergedBranches(run runner.Executor, base string) ([]string, error)`**  
  Runs `git branch --merged <base>`. Strips the `* ` (current) and `+ ` (worktree) markers and returns local branch names. Worktree policy is applied by the workflow, not hidden in the parser.
- **`WorktreeBranches(run runner.Executor) (map[string]bool, error)`**  
  Runs `git worktree list --porcelain` and returns the set of branches checked out in any worktree (including the main one). `gx clean` excludes these because `git branch -d` refuses to delete them.
- **`MergedRemoteBranches(run runner.Executor, remote, base string) ([]string, error)`**  
  Runs `git branch -r --merged <remote>/<base>`. Returns bare branch names of remote-tracking refs, excluding `origin/HEAD` symbolic refs.
- **`DeleteLocalBranch(run runner.Executor, branch string) error`**  
  Runs `git branch -d <branch>`. Safe delete: refuses unmerged branches.
- **`DeleteRemoteTrackingBranch(run runner.Executor, remote, branch string) error`**  
  Runs `git branch -d -r <remote>/<branch>`. Normalizes qualified and unqualified branch arguments.

### `detect.go`

- **`DetectRemote(run runner.Executor) (string, error)`**  
  Auto-detects remote from current branch tracking config (`branch.<name>.remote`), falling back to first entry in `git remote`.
- **`DetectDefaultBranch(run runner.Executor, remote string) (string, error)`**  
  Resolves remote HEAD via `git rev-parse --abbrev-ref <remote>/HEAD`, falling back to checking existence of local branches (`main`, `master`, `develop`).

---

## 3. Staging, Diffing & Pattern Matching

### `stage.go`

- **`AddAll(run runner.Executor) error`**  
  Runs `git add -A`. Stages modified tracked, new untracked, and deleted files.
- **`Add(run runner.Executor, files ...string) error`**  
  Runs `git add -- <files...>`. If `files` is empty, stages all changes (`git add -A`).
- **`AddPatch(run runner.Executor, files ...string) error`**  
  Runs `git add -p [-- <files...>]` for interactive hunk selection.

### `diff.go`

- **`DiffStatCached(run runner.Executor) (string, error)`**  
  Runs `git diff --stat --cached`. Returns empty string if nothing is staged.
- **`HasStagedChanges(run runner.Executor) (bool, error)`**  
  Runs `git diff --cached --quiet`. Exit code 0 → `false, nil`; exit code 1 → `true, nil`.
- **`DiffUnstagedFiles(run runner.Executor) (string, error)`**  
  Runs `git diff --name-only`. Returns modified tracked files that have unstaged changes.
- **`CheckCachedDiff(run runner.Executor) (string, error)`**  
  Runs `git diff --cached --check` to detect conflict markers in staged files.

### `patterns.go`

- **`MatchSensitivePatterns(files, patterns []string) []string`**  
  Matches file paths against glob patterns using `filepath.Match` against full path and base name. Single source of truth is `.gx/config`.
- **`FilterExcluded(files, excludePatterns []string) []string`**  
  Filters out files matching glob exclude patterns.
- **`SanitizePatterns(patterns []string) []string`**  
  Trims whitespace, drops empty strings, and deduplicates pattern lists.

---

## 4. Commit & History Log

### `commit.go`

- **`Commit(run runner.Executor, message string) error`**  
  Runs `git commit -m <message>`.
- **`CommitEditor(run runner.Executor) error`**  
  Runs `git commit` to launch `$EDITOR`.
- **`CommitAllowEmpty(run runner.Executor, message string) error`**  
  Runs `git commit --allow-empty -m <message>`.
- **`CommitAllowEmptyEditor(run runner.Executor) error`**  
  Runs `git commit --allow-empty` launching `$EDITOR`.

### `log.go`

- **`RecentCommits(run runner.Executor, n int) ([]CommitInfo, error)`**  
  Queries `git log -n <n> --format=%h|||%ar|||%s`. Splits using `|||` delimiter to preserve commit subjects containing whitespace. Returns `[]CommitInfo{ShortHash, RelativeDate, Subject}`.

---

## 5. Remote Operations & Divergence

### `remote.go`

- **`Fetch(run runner.Executor, remote string) error`**  
  Runs `git fetch --prune <remote>`. Pruning removes remote-tracking refs whose remote branch is gone, so merged-branch decisions never use stale refs.
- **`FastForward(run runner.Executor, remote, branch string) error`**  
  Runs `git merge --ff-only <remote>/<branch>`.
- **`RemoteExists(run runner.Executor, name string) bool`**  
  Runs `git remote get-url <name>` to check existence.
- **`RemoteBranchExists(run runner.Executor, remote, branch string) bool`**  
  Runs `git rev-parse --verify <remote>/<branch>`.
- **`RemoteHEAD(run runner.Executor, remote string) (string, error)`**  
  Resolves `git symbolic-ref refs/remotes/<remote>/HEAD`.
- **`Push(run runner.Executor, remote, branch string) error`**  
  Runs `git push <remote> <branch>`.
- **`PushSetUpstream(run runner.Executor, remote, branch string) error`**  
  Runs `git push -u <remote> <branch>` on first push.
- **`PushForceWithLease(run runner.Executor, remote, branch string) error`**  
  Runs `git push --force-with-lease <remote> <branch>`. Safe force push.
- **`RemoteURL(run runner.Executor, remote string) (string, error)`**  
  Runs `git remote get-url <remote>`.
- **`DeleteRemoteBranch(run runner.Executor, remote, branch string) error`**  
  Runs `git push <remote> --delete <branch>`.
- **`IsRemoteRefMissing(err error) bool`**  
  Inspects `runner.CommandError.Stderr` for `remote ref does not exist`. Used by clean's remote deletion loop so losing the race to another collaborator is treated as already pruned.
- **`IsNonFastForward(err error) bool`**  
  Inspects `runner.CommandError.Stderr` for `non-fast-forward` marker.

### `divergence.go`

- **`CheckDivergence(run runner.Executor, remote, branch string) (Divergence, error)`**  
  Compares local branch with `<remote>/<branch>`. Runs `git rev-list --count --left-right <remote>/<branch>...HEAD`.
  Returns `Divergence{Ahead, Behind, Tracking}`. If `<remote>/<branch>` does not exist, returns `Divergence{Tracking: false}` without error.

### `internal/forge`

- Detects host provider (GitHub, GitLab, Bitbucket) from `RemoteURL`.
- Provides `PRCreateURL(base, branch string) string` and `ExistingPRURL(ctx, base, branch) (string, error)`.

---

## 6. Rebase & Merge Operations

### `rebase.go`

- **`Rebase(run runner.Executor, branch string) error`**  
  Runs `git rebase -- <branch>`.
- **`RebaseAbort(run runner.Executor) error`**  
  Runs `git rebase --abort`.
- **`RebaseContinue(run runner.Executor) error`**  
  Runs `GIT_EDITOR=true git rebase --continue`.
- **`RebaseSkip(run runner.Executor) error`**  
  Runs `GIT_EDITOR=true git rebase --skip`.
- **`IsRebaseInProgress() bool`**  
  Checks existence of `.git/rebase-merge` or `.git/rebase-apply`.
- **`CurrentRebasePatchInfo(run runner.Executor) string`**  
  Extracts patch author/commit descriptor from `git rebase --show-current-patch`.

### `merge.go`

- **`Merge(run runner.Executor, branch string) error`**  
  Runs `git merge --no-edit <branch>`.
- **`MergeContinue(run runner.Executor) error`**  
  Runs `GIT_EDITOR=true git merge --continue`.
- **`MergeAbort(run runner.Executor) error`**  
  Runs `git merge --abort`.
- **`IsMergeInProgress(run runner.Executor) bool`**  
  Runs `git rev-parse -q --verify MERGE_HEAD`.
- **`ConflictedFiles(run runner.Executor) ([]string, error)`**  
  Runs `git diff --name-only --diff-filter=U`.

---

## 7. Stash Management

### `stash.go`

- **`StashList(run runner.Executor) ([]string, error)`**  
  Runs `git stash list`.
- **`StashPush(run runner.Executor, message string) error`**  
  Runs `git stash push -m <message>`.
- **`StashPop(run runner.Executor, branch ...string) error`**  
  Finds the stash matching the exact `gx-sync/<branch>/<timestamp>` label (any `gx-sync/` entry when branch is omitted) and pops it via `git stash pop <ref>`. Exact matching keeps `feature/a` from matching `feature/a/b`.
- **`DropGXStash(run runner.Executor, branch ...string) error`**  
  Iteratively drops matching `gx-sync/` stash entries, scoped to the exact branch when provided.
- **`IsClean(run runner.Executor) bool`**  
  Runs `git status --porcelain --untracked-files=no` to check if tracked files are clean.
- **`HasGXStash(run runner.Executor, branch ...string) bool`**  
  Checks if any stash matches the exact `gx-sync/<branch>/<timestamp>` label (or any `gx-sync/` entry when branch is omitted).
- **`IsIndexLocked(err error) bool`**  
  Inspects `runner.CommandError.Stderr` for git's index lock so sync can report lock contention instead of a misleading "commit or discard your changes" hint.

---

## 8. Sync Ownership State

### `syncstate.go`

- **`GitDir(run runner.Executor) (string, error)`**  
  Runs `git rev-parse --absolute-git-dir`. Returns the worktree-specific git directory.
- **`WriteGXSyncState(run runner.Executor, branch string) error`**  
  Writes `.git/gx-sync-state` containing the owning branch immediately before gx starts a rebase or merge.
- **`ReadGXSyncState(run runner.Executor) (string, bool)`**  
  Reads the ownership marker. Returns `("", false)` when absent or empty.
- **`ClearGXSyncState(run runner.Executor) error`**  
  Removes the marker. A missing marker is not an error.
- **`RebaseBranch(run runner.Executor) string`**  
  Reads `.git/rebase-merge/head-name` (or `rebase-apply/head-name`) and strips `refs/heads/`. Supplies the owning branch while HEAD is detached during a rebase.

---

## 9. Status Aggregation

### `status.go`

- **`StatusLabels`**  
  Map translating porcelain letters (`M`, `A`, `D`, `R`) to display strings (`modified`, `new file`, `deleted`, `renamed`).
- **`GetParsedStatus(run runner.Executor) (*ParsedStatus, error)`**  
  Executes `git status --porcelain --untracked-files=all` and parses entries into:
  - `StagedFiles map[string][]string`
  - `UnstagedFiles []string`
  - `UntrackedFiles []string`
  - `ConflictedFiles []string`
  - `ModifiedFiles []string`
- **`FormatStagedLabel(letter string) string`**  
  Returns human-readable label or `status <letter>`.
- **`Status(run runner.Executor) (modified, untracked []string, err error)`**  
  Returns categorized modified tracked and untracked file paths.
- **`NonEmptyStatus(run runner.Executor) bool`**  
  Checks whether tracked files have staged or unstaged modifications.
- **`SubmodulePaths(run runner.Executor) (map[string]bool, error)`**  
  Parses `git ls-files --stage` for mode `160000` (gitlink) entries to identify submodules.
