package git

import "strings"

// Qualify returns branch qualified with remote (e.g. "origin/main").
// It is idempotent: an already-qualified ref is returned unchanged.
func Qualify(remote, branch string) string {
	if remote == "" || strings.HasPrefix(branch, remote+"/") {
		return branch
	}
	return remote + "/" + branch
}

// StripRemote removes the "<remote>/" prefix from a remote-tracking ref and
// returns the bare branch name. Refs not carrying the prefix are returned
// unchanged.
func StripRemote(remote, ref string) string {
	return strings.TrimPrefix(ref, remote+"/")
}
