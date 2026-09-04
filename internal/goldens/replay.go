package goldens

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Replay runs bin's generate command for the case and returns the golden
// bytes: the produced output file, or stderr when WantErr is set.
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
	cmd.Dir = filepath.Join(repoRoot, c.Chdir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if c.WantErr {
		if runErr == nil {
			return nil, fmt.Errorf("expected non-zero exit, got success (stderr: %q)", stderr.String())
		}
		if _, ok := runErr.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("running %s: %w", bin, runErr)
		}
		return errorLines(stderr.Bytes()), nil
	}
	if runErr != nil {
		return nil, fmt.Errorf("running %s %v: %w\nstderr: %s", bin, args, runErr, stderr.String())
	}
	return os.ReadFile(out)
}

// errorLines keeps only the "Error: ..." lines of stderr. The rest is
// timestamped logging, which would make the golden unstable.
func errorLines(stderr []byte) []byte {
	var out []byte
	for _, line := range bytes.Split(stderr, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("Error:")) {
			out = append(out, line...)
			out = append(out, '\n')
		}
	}
	return out
}

// GoldenPath returns the case's golden file path relative to the repo root.
func (c Case) GoldenPath() string {
	ext := ".yaml"
	if c.WantErr {
		ext = ".err"
	}
	return filepath.Join("testdata", "goldens", c.Name+ext)
}
