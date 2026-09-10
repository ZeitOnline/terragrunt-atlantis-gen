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

	var log bytes.Buffer
	opts.LogWriter = &log
	out, err := Run(opts)
	if err != nil {
		t.Fatalf("run without --fail-on-parse-errors must succeed: %v", err)
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

	opts.FailOnParseErrors = true
	opts.LogWriter = nil
	if _, err := Run(opts); err == nil || !strings.Contains(err.Error(), "reader/terragrunt.hcl") {
		t.Fatalf("run with --fail-on-parse-errors must name the config, got: %v", err)
	}
}
