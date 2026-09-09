package goldens

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// -update rewrites the goldens from the wrapper's current output instead of
// comparing against them. Deliberate only: the resulting diff is what a
// reviewer approves.
var update = flag.Bool("update", false, "rewrite testdata/goldens from the current output")

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

// TestGoldens replays every case with the wrapper against its golden, byte
// for byte.
func TestGoldens(t *testing.T) {
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := c.Replay(binPath, repoRoot)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(repoRoot, c.GoldenPath())
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden (run go test ./internal/goldens -update): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from golden %s\n--- want\n%s\n--- got\n%s", c.GoldenPath(), want, got)
			}
		})
	}
}

// TestGoldensBelongToCases fails on golden files no case produces; -update
// never removes them, so a renamed or dropped case would leave one behind.
func TestGoldensBelongToCases(t *testing.T) {
	cases := make(map[string]bool, len(Cases))
	for _, c := range Cases {
		cases[c.Name] = true
	}
	entries, err := os.ReadDir(filepath.Join(repoRoot, "testdata", "goldens"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !cases[strings.TrimSuffix(e.Name(), ".yaml")] {
			t.Errorf("testdata/goldens/%s has no case", e.Name())
		}
	}
}
