package goldens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTACGoldens replays every case with the frozen terragrunt-atlantis-config
// against the same fixture tree: the goldens are TAC's output, not the
// wrapper's reading of it, and the read marks in the fixtures stay invisible
// to TAC. Gated on TAC_BIN because it needs the v2.25.1 binary.
func TestTACGoldens(t *testing.T) {
	tac := os.Getenv("TAC_BIN")
	if tac == "" {
		t.Skip("TAC_BIN not set; point it at the terragrunt-atlantis-config v2.25.1 binary")
	}
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(repoRoot, c.TACGoldenPath()))
			if err != nil {
				t.Fatalf("missing golden (run tools/freeze): %v", err)
			}
			got, err := c.Replay(tac, repoRoot)
			if err != nil {
				// TAC's go-getter BitBucket detector performs a live API
				// lookup; its transient failures are TAC's, not the tree's.
				if strings.Contains(err.Error(), "BitBucket URL") {
					t.Skipf("TAC needs network for the BitBucket detector: %v", err)
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("TAC output differs from golden %s\n--- want\n%s\n--- got\n%s", c.TACGoldenPath(), want, got)
			}
		})
	}
}
