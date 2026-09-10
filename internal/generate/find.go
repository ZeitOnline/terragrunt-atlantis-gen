package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Unit is one entry of `terragrunt find --json`. Decoding is strict: an
// unknown field means Terragrunt changed its output shape, and silently
// dropping data on an upgrade is the one failure mode this design forbids.
type Unit struct {
	Type         string            `json:"type"`
	Path         string            `json:"path"`
	Include      map[string]string `json:"include,omitempty"`
	Reading      []string          `json:"reading,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Exclude      *ExcludeConfig    `json:"exclude,omitempty"`
}

// ExcludeConfig is a unit's evaluated exclude block as find reports it —
// merged across the include chain, so a child's `if = false` override of an
// inherited block is already applied.
type ExcludeConfig struct {
	ExcludeDependencies *bool    `json:"exclude_dependencies"`
	NoRun               *bool    `json:"no_run"`
	Actions             []string `json:"actions"`
	If                  bool     `json:"if"`
}

// ExcludesPlan reports whether the exclude block is active and covers plan.
func (e *ExcludeConfig) ExcludesPlan() bool {
	if e == nil || !e.If {
		return false
	}
	for _, a := range e.Actions {
		if a == "all" || a == "plan" {
			return true
		}
	}
	return false
}

// findUnits runs `terragrunt find` and returns the units of the tree at
// root, every path relative to root, plus the parse errors discovery
// suppressed. The process runs in cwd — a version-manager shim resolves the
// binary by working directory — and is pointed at root with --working-dir
// when the two differ. The extra args select the detail fields
// (--dependencies --include --reading ...).
//
// Discovery swallows a config that fails to parse: the unit is still listed,
// with whatever includes, reads and dependencies survived, exit 0, and a
// DEBUG line (gruntwork-io/terragrunt#6856). So the log runs at debug level
// in JSON form and those lines are picked out of it; the result on stdout is
// unaffected.
func findUnits(terragruntBin, cwd, root string, extraArgs ...string) ([]Unit, []string, error) {
	args := append([]string{"find", "--json", "--log-level", "debug", "--log-format", "json"}, extraArgs...)
	if root != cwd {
		args = append(args, "--working-dir", root)
	}
	cmd := exec.Command(terragruntBin, args...)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	suppressed, loud := parseLog(stderr.Bytes())
	if runErr != nil {
		return nil, nil, fmt.Errorf("terragrunt find in %s: %w\n%s", root, runErr, strings.Join(loud, "\n"))
	}

	dec := json.NewDecoder(&stdout)
	dec.DisallowUnknownFields()
	var units []Unit
	if err := dec.Decode(&units); err != nil {
		return nil, nil, fmt.Errorf("decoding terragrunt find output: %w", err)
	}
	return units, suppressed, nil
}

type logLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// suppressedParsePrefix matches both wordings terragrunt uses ("Suppressed
// parsing errors <file>:<pos>: ..." while parsing, "Suppressed parse error
// for <dir>: ..." while resolving dependencies). TestSuppressedParseErrors
// fails when a Terragrunt upgrade rewords them.
const suppressedParsePrefix = "Suppressed pars"

// parseLog splits terragrunt's JSON log into the suppressed parse errors and
// the lines worth showing when the command failed: errors, warnings, and
// anything that is not a log line at all.
func parseLog(stderr []byte) (suppressed, loud []string) {
	for _, raw := range bytes.Split(stderr, []byte("\n")) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var l logLine
		if json.Unmarshal(raw, &l) != nil {
			loud = append(loud, string(raw))
			continue
		}
		switch {
		case strings.HasPrefix(l.Msg, suppressedParsePrefix):
			suppressed = append(suppressed, l.Msg)
		case l.Level == "error" || l.Level == "warn":
			loud = append(loud, l.Msg)
		}
	}
	return suppressed, loud
}
