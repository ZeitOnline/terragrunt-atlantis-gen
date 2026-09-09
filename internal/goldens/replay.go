package goldens

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const fixturesDir = "testdata/fixtures/"

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

	// Plain concatenation: a trailing slash in Fixture must reach --root.
	root := fixturesDir + c.Fixture
	if c.AbsRoot {
		root, err = filepath.Abs(filepath.Join(repoRoot, root))
		if err != nil {
			return nil, err
		}
	}
	args := append([]string{"generate", "--output", out, "--root", root}, c.Flags...)
	if len(c.Filter) > 0 {
		globs := make([]string, len(c.Filter))
		for i, f := range c.Filter {
			globs[i] = fixturesDir + c.Fixture + "/" + f
		}
		args = append(args, "--filter", strings.Join(globs, ","))
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

// GoldenPath returns the case's golden file, relative to the repo root.
func (c Case) GoldenPath() string {
	return filepath.Join("testdata", "goldens", c.Name+".yaml")
}
