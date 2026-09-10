package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
)

// Pins the contract with terragrunt's debug log: a config that fails to
// parse is reported in the job log with its file and diagnostic, and
// --fail-on-parse-errors turns that into the run's error. A Terragrunt
// upgrade that rewords the suppressed-error message fails this test.
func TestSuppressedParseErrors(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "parse_error"))
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Root: root, TerragruntBin: bin, IgnoreParentTerragrunt: true, CascadeDependencies: true}

	// Opted out: the run goes through and only reports.
	var log bytes.Buffer
	opts.LogWriter = &log
	opts.FailOnParseErrors = false
	out, err := Run(opts)
	if err != nil {
		t.Fatalf("run with --fail-on-parse-errors=false must succeed: %v", err)
	}
	for _, want := range []string{"terragrunt suppressed a parse error: reader/terragrunt.hcl:", "settings/missing.hcl", "1 parse error(s) suppressed"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("job log lacks %q:\n%s", want, log.String())
		}
	}
	var cfg AtlantisConfig
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.Projects {
		if p.Dir == "reader" && len(p.Autoplan.WhenModified) != 4 { // *.hcl, *.tf*, *.tofu*, ../root.hcl: the read is gone
			t.Errorf("reader watches %v, expected only the defaults and the include", p.Autoplan.WhenModified)
		}
	}

	// The default: the run fails and names the config.
	opts.FailOnParseErrors = true
	opts.LogWriter = nil
	if _, err := Run(opts); err == nil || !strings.Contains(err.Error(), "reader/terragrunt.hcl") {
		t.Fatalf("run with --fail-on-parse-errors must name the config, got: %v", err)
	}
}

// With --filter, a parse error in a dropped unit neither fails nor logs;
// one in a kept unit still fails.
func TestSuppressedParseErrorsOutsideFilter(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "parse_error"))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	opts := Options{Root: root, TerragruntBin: bin, IgnoreParentTerragrunt: true, CascadeDependencies: true, FailOnParseErrors: true, LogWriter: &log}

	opts.FilterPaths = []string{filepath.Join(root, "healthy")}
	if _, err := Run(opts); err != nil {
		t.Fatalf("a parse error outside --filter must not fail the run: %v", err)
	}
	if strings.Contains(log.String(), "parse error") {
		t.Errorf("a parse error outside --filter must not be reported:\n%s", log.String())
	}

	opts.FilterPaths = []string{filepath.Join(root, "reader")}
	if _, err := Run(opts); err == nil || !strings.Contains(err.Error(), "reader/terragrunt.hcl") {
		t.Fatalf("the kept unit's parse error must fail the run, got: %v", err)
	}
}

func TestParseErrorConcerns(t *testing.T) {
	units := map[string]*Unit{
		"a": {Path: "a", Include: map[string]string{"root": "root.hcl"}, Reading: []string{"root.hcl", "settings/a.hcl"}},
		"b": {Path: "b", Include: map[string]string{"root": "root.hcl"}},
	}
	keepA := map[string]bool{"a": true}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"a/terragrunt.hcl", true},
		{"a", true},
		{"b/terragrunt.hcl", false},
		{"b", false},
		{"root.hcl", true},        // included by a
		{"settings/a.hcl", true},  // read by a
		{"nobody/owns.hcl", true}, // no unit owns it: counts for all
		{"", true},
	} {
		if got := (parseError{path: tc.path}).concerns(units, keepA); got != tc.want {
			t.Errorf("concerns(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
	// A unit at the root owns every path below it.
	rootUnit := map[string]*Unit{".": {Path: "."}}
	if !(parseError{path: "x/y.hcl"}).concerns(rootUnit, map[string]bool{".": true}) {
		t.Error("a root-level unit must own x/y.hcl")
	}
}
