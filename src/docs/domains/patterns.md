# Patterns Domain

**File:** `internal/git/patterns.go`

Sensitive file pattern matching and file filtering for `gx save`. Uses
`filepath.Match` glob semantics — patterns like `.env`, `*.pem`.

## DefaultSensitivePatterns

Defined in `internal/config/config.go`, this variable holds the default
sensitive file patterns seeded by `gx init`:

- `.env`
- `*.pem`
- `*secret*`
- `*.key`

These are **not hardcoded** into `MatchSensitivePatterns`. They are written
to `.gx/config` by `gx init` and read from the config file at runtime.
Users can remove them via `gx config` or by editing `.gx/config` directly.

## MatchSensitivePatterns

```
MatchSensitivePatterns(files, patterns) → matches
```

Checks each filename against the given patterns (from config). Does **not**
concatenate any hardcoded defaults — the config file is the single source
of truth. Matches against both the full file path and the base name.

Returns a list of matched filenames with the pattern they matched:
```
.env matched .env
config/keys.pem matched *.pem
```

## FilterExcluded

```
FilterExcluded(files, excludePatterns) → filteredFiles
```

Removes files that match any of the given exclude glob patterns. Uses the
same glob semantics as `MatchSensitivePatterns` (full path + base name).
Applied by `gx save --exclude` before staging and before the sensitive-pattern
check.

## SanitizePatterns

Cleans and deduplicates a list of glob patterns. Removes empty strings and
duplicates. Used by `gx config sensitivePatterns` to normalize input.
