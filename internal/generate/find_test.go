package generate

import (
	"strings"
	"testing"
)

// The three wordings terragrunt v1.1 logs for a suppressed parse error, plus
// the lines a failing run should surface and the ones it should not.
func TestParseLog(t *testing.T) {
	stderr := strings.Join([]string{
		`{"level":"debug","msg":"Discovery: parsing /repo/u (phase=parse, depth=0)"}`,
		`{"level":"debug","msg":"Suppressed parsing errors /repo/u/terragrunt.hcl:6,14-37: Error in function call; Call failed.\n\nPath: \"/repo/x.hcl\"."}`,
		`{"level":"debug","msg":"Suppressed parse error for /repo/v: some error"}`,
		`{"level":"debug","msg":"Suppressing parse error for /repo/w/terragrunt.hcl: another error"}`,
		`{"level":"warn","msg":"a warning"}`,
		``,
		`{"level":"error","msg":"an error"}`,
		`not a log line at all`,
		`{"level":"info","msg":"Suppressed something unrelated"}`,
	}, "\n")

	suppressed, loud := parseLog([]byte(stderr))
	if len(suppressed) != 3 {
		t.Fatalf("want the three suppression wordings, got %d: %q", len(suppressed), suppressed)
	}
	for _, s := range suppressed {
		if !suppressedParse.MatchString(s) {
			t.Errorf("collected line does not match the suppression pattern: %q", s)
		}
	}
	want := []string{"a warning", "an error", "not a log line at all"}
	if strings.Join(loud, "|") != strings.Join(want, "|") {
		t.Errorf("loud lines: got %q, want %q", loud, want)
	}
}

// Prefix, root and line breaks go; the base label comes; duplicates across
// runs collapse.
func TestParseErrorsNormalize(t *testing.T) {
	var p parseErrors
	p.add([]string{
		"Suppressed parsing errors /repo/u/terragrunt.hcl:6,14-37: Error in function call; Call failed.\n\nPath: \"/repo/x.hcl\".",
		"Suppressing parse error for /repo/w/terragrunt.hcl: another error",
	}, "/repo", "")
	p.add([]string{"Suppressed parsing errors /repo/u/terragrunt.hcl:6,14-37: Error in function call; Call failed.\n\nPath: \"/repo/x.hcl\"."}, "/repo", "")
	p.add([]string{"Suppressed parse error for /wt/v: some error"}, "/wt", "base origin/main")

	want := []string{
		`u/terragrunt.hcl:6,14-37: Error in function call; Call failed. Path: "x.hcl".`,
		`w/terragrunt.hcl: another error`,
		`v: some error (in base origin/main)`,
	}
	if strings.Join(p.lines, "|") != strings.Join(want, "|") {
		t.Errorf("got %q\nwant %q", p.lines, want)
	}
}
