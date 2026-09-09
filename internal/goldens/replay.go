package goldens

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Replay runs bin's generate command for the case from the repo root and
// returns the produced output file.
func (c Case) Replay(bin, repoRoot string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "goldens-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	out := filepath.Join(tmp, "atlantis.yaml")
	if c.PreSeed != "" {
		if err := os.WriteFile(out, []byte(c.PreSeed), 0o644); err != nil {
			return nil, err
		}
	}

	args := append([]string{"generate", "--output", out}, c.Args...)
	if c.AbsRoot {
		for i, a := range args {
			if a == "--root" && i+1 < len(args) {
				args[i+1], err = filepath.Abs(filepath.Join(repoRoot, args[i+1]))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s %v: %w\nstderr: %s", bin, args, err, stderr.String())
	}
	return os.ReadFile(out)
}

// GoldenPath returns the golden the wrapper is gated by, relative to the
// repo root: TAC's frozen output, or the reviewed wrapper output for cases
// that deliberately diverge (see Case.Diverges).
func (c Case) GoldenPath() string {
	return filepath.Join("testdata", "goldens", c.Name+".yaml")
}

// TACGoldenPath returns the golden TAC is gated by: the same file as the
// wrapper's, except for diverging cases, whose TAC output lives apart.
func (c Case) TACGoldenPath() string {
	if c.Diverges != "" {
		return filepath.Join("testdata", "goldens-tac", c.Name+".yaml")
	}
	return c.GoldenPath()
}
