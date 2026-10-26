package runner

import "strings"

// OutputResult is a canned Output/CombinedOutput result for Fake.
type OutputResult struct {
	Out string
	Err error
}

// Fake is the test double for Executor — the seam's second adapter. Keys in
// the results maps are the git arguments joined by single spaces
// (e.g. "push origin feat"). Unknown commands succeed with empty output;
// set RunErr/OutputErr to change that fallback. Every call is recorded in
// Calls, warnings in Warnings.
type Fake struct {
	RunResults      map[string]error
	OutputResults   map[string]OutputResult
	CombinedResults map[string]OutputResult
	RunErr          error
	OutputErr       error
	Warnings        []string
	Calls           [][]string
}

var _ Executor = (*Fake)(nil)

func (f *Fake) record(args []string) string {
	f.Calls = append(f.Calls, append([]string(nil), args...))
	return strings.Join(args, " ")
}

func (f *Fake) Run(args ...string) error {
	key := f.record(args)
	if err, ok := f.RunResults[key]; ok {
		return err
	}
	return f.RunErr
}

func (f *Fake) RunWithEnv(extraEnv []string, args ...string) error {
	return f.Run(args...)
}

func (f *Fake) Output(args ...string) (string, error) {
	key := f.record(args)
	if result, ok := f.OutputResults[key]; ok {
		return result.Out, result.Err
	}
	return "", f.OutputErr
}

func (f *Fake) CombinedOutput(args ...string) (string, error) {
	key := f.record(args)
	if result, ok := f.CombinedResults[key]; ok {
		return result.Out, result.Err
	}
	return "", f.OutputErr
}

func (f *Fake) Warn(msg string) {
	f.Warnings = append(f.Warnings, msg)
}

func (f *Fake) Warnf(msg, hint string) {
	f.Warnings = append(f.Warnings, msg)
}
