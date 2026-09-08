package goldens

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	repoRoot string
	binPath  string
)

// TestMain builds the wrapper binary once; every case replays through the
// real CLI, exactly as the Atlantis pre-workflow hook will run it.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	repoRoot = filepath.Join(wd, "..", "..")

	tmp, err := os.MkdirTemp("", "gen-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binPath = filepath.Join(tmp, "terragrunt-atlantis-gen")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building wrapper: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestGoldens replays every supported case with the wrapper against the
// migrated fixture tree — marks and exclude blocks in place, exactly the
// state a real repo is in when the wrapper runs.
func TestGoldens(t *testing.T) {
	root := migratedRoot(t)
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.Unsupported != "" {
				t.Skip(c.Unsupported)
			}
			want, err := os.ReadFile(filepath.Join(repoRoot, c.ParityGoldenPath()))
			if err != nil {
				t.Fatalf("missing golden (run tools/freeze): %v", err)
			}
			got, err := c.Replay(binPath, root)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from golden %s\n--- want\n%s\n--- got\n%s", c.ParityGoldenPath(), want, got)
			}
		})
	}
}
