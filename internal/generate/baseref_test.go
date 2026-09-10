package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A --root that the pull request introduced is not a directory in the base:
// the base pass has nothing to merge and must not fail the run.
func TestMergeBaseStateRootMissingInBase(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=test", "-c", "user.email=test@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=base",
		}, args...)...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel string) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	write("existing/terragrunt.hcl")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "head")
	write("added/unit/terragrunt.hcl")
	git("add", "-A")
	git("commit", "-q", "-m", "head")

	root, err := filepath.EvalSymlinks(filepath.Join(repo, "added"))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	units := map[string]*Unit{"unit": {Type: "unit", Path: "unit", Reading: []string{"x.hcl"}}}
	if err := mergeBaseState(Options{BaseRef: "base", LogWriter: &log}, root, units); err != nil {
		t.Fatalf("base pass must skip a root missing in the base, got: %v", err)
	}
	if got := units["unit"].Reading; len(got) != 1 || got[0] != "x.hcl" {
		t.Errorf("head unit changed: %v", got)
	}
	if !strings.Contains(log.String(), "added is not a directory there") {
		t.Errorf("log does not say why the base pass was skipped:\n%s", log.String())
	}
	out, _ := exec.Command("git", "-C", repo, "worktree", "list").Output()
	if lines := strings.Count(strings.TrimSpace(string(out)), "\n"); lines != 0 {
		t.Errorf("worktree left behind:\n%s", out)
	}
}
