package goldens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigratedFixturesMatchTACGoldens proves testdata/migrated stays
// additive: TAC's output over the migrated tree is byte-identical to the
// goldens frozen from the pristine fixtures — for every case, including the
// ones the wrapper does not support. Gated on TAC_BIN because it needs the
// frozen terragrunt-atlantis-config v2.25.1 binary.
func TestMigratedFixturesMatchTACGoldens(t *testing.T) {
	tac := os.Getenv("TAC_BIN")
	if tac == "" {
		t.Skip("TAC_BIN not set; point it at the terragrunt-atlantis-config v2.25.1 binary")
	}
	root := migratedRoot(t)
	for _, c := range Cases {
		t.Run(c.Name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(repoRoot, c.GoldenPath()))
			if err != nil {
				t.Fatalf("missing golden (run tools/freeze): %v", err)
			}
			got, err := c.Replay(tac, root)
			if err != nil {
				// TAC's go-getter BitBucket detector performs a live API
				// lookup; its transient failures are TAC's, not the tree's.
				if strings.Contains(err.Error(), "BitBucket URL") {
					t.Skipf("TAC needs network for the BitBucket detector: %v", err)
				}
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("TAC output changed on the migrated tree\n--- golden\n%s\n--- migrated\n%s", want, got)
			}
		})
	}
}

// migratedRoot returns testdata/migrated prepared as a replay root: the
// chdir-based case needs the internal/goldens working directory to exist.
func migratedRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(repoRoot, "testdata", "migrated")
	if err := os.MkdirAll(filepath.Join(root, "internal", "goldens"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
