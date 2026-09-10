package cli

import "testing"

// The goldens replay the CLI and cannot capture a failing run, so the
// default of the one flag that makes a run fail is pinned here.
func TestFailOnParseErrorsIsTheDefault(t *testing.T) {
	f := newGenerateCmd("test").PersistentFlags().Lookup("fail-on-parse-errors")
	if f == nil || f.DefValue != "true" {
		t.Fatalf("--fail-on-parse-errors must default to true, got %v", f)
	}
}
