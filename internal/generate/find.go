package generate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
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

// findUnits runs `terragrunt find` in root and returns the units. The extra
// args select the detail fields (--dependencies --include --reading ...).
func findUnits(terragruntBin, root string, extraArgs ...string) ([]Unit, error) {
	args := append([]string{"find", "--json"}, extraArgs...)
	cmd := exec.Command(terragruntBin, args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("terragrunt find in %s: %w\nstderr: %s", root, err, stderr.String())
	}

	dec := json.NewDecoder(&stdout)
	dec.DisallowUnknownFields()
	var units []Unit
	if err := dec.Decode(&units); err != nil {
		return nil, fmt.Errorf("decoding terragrunt find output: %w", err)
	}
	return units, nil
}
