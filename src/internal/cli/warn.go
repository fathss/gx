package cli

// Warn represents a non-fatal warning message to display to the user.
// Unlike Error, Warn does not stop execution — it surfaces unusual
// conditions (sensitive files, stale config, etc.) before or during
// an operation.
type Warn struct {
	Message string
	Hint    string
}
